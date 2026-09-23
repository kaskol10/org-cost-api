package service

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"time"

	"github.com/kaskol10/org-cost-api/backend/internal/analysis"
	awsclient "github.com/kaskol10/org-cost-api/backend/internal/aws"
	"github.com/kaskol10/org-cost-api/backend/internal/aws/costexplorer"
	"github.com/kaskol10/org-cost-api/backend/internal/history"
)

type serviceTagDeltaAccountRank struct {
	accountID   string
	accountName string
	currentUSD  float64
	priorUSD    float64
	deltaUSD    float64
	scoreUSD    float64
}

// ServiceTagDelta computes "why did <service> change" by Name-tag (or other tag) movers.
//
// Lazy org scope (accountID empty): 2 payer CE calls for org-wide tag deltas; accounts are
// ranked from history only (tag lists empty until an account is selected).
// Account scope: 2 CE calls for that account only.
func (a *Aggregator) ServiceTagDelta(
	ctx context.Context,
	accountID string,
	service string,
	start string,
	end string,
	tagKey string,
	topAccounts int,
) (*analysis.ServiceTagDeltaResponse, error) {
	if service == "" {
		return nil, fmt.Errorf("service is required")
	}
	if tagKey == "" {
		tagKey = "Name"
	}
	if topAccounts <= 0 {
		topAccounts = 3
	}
	if start == "" || end == "" {
		start, end = a.cfg.CostDateRange()
	}

	if start == "" || end == "" {
		return nil, fmt.Errorf("invalid date range")
	}

	priorStart, priorEnd := priorPeriod(start, end)
	priorEndTime, _ := time.Parse("2006-01-02", start)

	scope := "org"
	if accountID != "" {
		scope = "account"
	}
	cacheKey := fmt.Sprintf(
		"%s|%s|%s|%s|%s|%s|%d",
		scope,
		accountID,
		service,
		start,
		end,
		tagKey,
		topAccounts,
	)

	a.cacheMu.Lock()
	if a.cachedServiceTagDelta != nil {
		if cached := a.cachedServiceTagDelta[cacheKey]; cached != nil {
			at := a.cachedServiceTagDeltaAt[cacheKey]
			if time.Since(at) < a.cacheTTL {
				out := *cached
				a.cacheMu.Unlock()
				log.Printf(
					"endpoint=service-tag-delta ce_calls_used=0 cache=hit view=%s service=%s",
					scope, service,
				)
				return &out, nil
			}
		}
	}
	a.cacheMu.Unlock()

	var out *analysis.ServiceTagDeltaResponse
	var err error
	if accountID != "" {
		out, err = a.serviceTagDeltaAccount(ctx, accountID, service, start, end, priorStart, priorEnd, tagKey)
	} else {
		out, err = a.serviceTagDeltaOrgLazy(ctx, service, start, end, priorStart, priorEnd, priorEndTime, tagKey, topAccounts)
	}
	if err != nil {
		return nil, err
	}

	a.cacheMu.Lock()
	if a.cachedServiceTagDelta != nil {
		a.cachedServiceTagDelta[cacheKey] = out
		a.cachedServiceTagDeltaAt[cacheKey] = time.Now()
	}
	a.cacheMu.Unlock()

	log.Printf(
		"endpoint=service-tag-delta ce_calls_used=%d cache=miss view=%s service=%s",
		out.CECallsUsed, scope, service,
	)
	return out, nil
}

func (a *Aggregator) serviceTagDeltaAccount(
	ctx context.Context,
	accountID, service, start, end, priorStart, priorEnd, tagKey string,
) (*analysis.ServiceTagDeltaResponse, error) {
	var client *awsclient.AccountClients
	for _, c := range a.clients {
		if c.AccountID == accountID {
			client = c
			break
		}
	}
	if client == nil {
		return nil, NewUnknownAccountError(accountID)
	}

	if a.demoMode {
		return &analysis.ServiceTagDeltaResponse{
			Service:       service,
			DisplayName:   analysis.FriendlyService(service),
			TagKey:        tagKey,
			CurrentPeriod: analysis.PeriodSummary{Start: start, End: end},
			PriorPeriod:   analysis.PeriodSummary{Start: priorStart, End: priorEnd},
			Accounts: []analysis.AccountTagDelta{{
				AccountID:   accountID,
				AccountName: client.Account.Name,
			}},
			TopAccountNote: "Demo account-scoped tag-delta (no CE)",
			CECallsUsed:    0,
		}, nil
	}

	costCE, fromPayer := a.resolveCostClient(client)
	curTagRaw, err := costexplorer.GetServiceTagTotals(ctx, costCE, accountID, service, start, end, fromPayer, tagKey)
	if err != nil {
		return nil, fmt.Errorf("tag totals (%s current): %w", accountID, err)
	}
	priorTagRaw, err := costexplorer.GetServiceTagTotals(ctx, costCE, accountID, service, priorStart, priorEnd, fromPayer, tagKey)
	if err != nil {
		return nil, fmt.Errorf("tag totals (%s prior): %w", accountID, err)
	}

	curTag := costexplorer.NormalizeTagMapKeys(tagKey, curTagRaw)
	priorTag := costexplorer.NormalizeTagMapKeys(tagKey, priorTagRaw)

	var totalCur, totalPrior float64
	for _, v := range curTag {
		totalCur += v
	}
	for _, v := range priorTag {
		totalPrior += v
	}

	topInc, topDec := analysis.BuildTagDeltaDrivers(curTag, priorTag, totalCur, 5)
	deltaUSD := totalCur - totalPrior
	deltaPct := 0.0
	if totalPrior > 0 {
		deltaPct = (deltaUSD / totalPrior) * 100
	}

	account := analysis.AccountTagDelta{
		AccountID:       accountID,
		AccountName:     client.Account.Name,
		TotalCurrentUSD: totalCur,
		TotalPriorUSD:   totalPrior,
		DeltaUSD:        deltaUSD,
		DeltaPercent:    deltaPct,
		TopIncreases:    topInc,
		TopDecreases:    topDec,
	}

	return &analysis.ServiceTagDeltaResponse{
		Service:         service,
		DisplayName:     analysis.FriendlyService(service),
		TagKey:          tagKey,
		CurrentPeriod:   analysis.PeriodSummary{Start: start, End: end},
		PriorPeriod:     analysis.PeriodSummary{Start: priorStart, End: priorEnd},
		TotalCurrentUSD: totalCur,
		TotalPriorUSD:   totalPrior,
		DeltaUSD:        deltaUSD,
		DeltaPercent:    deltaPct,
		TopIncreases:    topInc,
		TopDecreases:    topDec,
		Accounts:        []analysis.AccountTagDelta{account},
		TopAccountNote:  "Account-scoped Name-tag drivers",
		CECallsUsed:     2,
	}, nil
}

func (a *Aggregator) serviceTagDeltaOrgLazy(
	ctx context.Context,
	service, start, end, priorStart, priorEnd string,
	priorEndTime time.Time,
	tagKey string,
	topAccounts int,
) (*analysis.ServiceTagDeltaResponse, error) {
	accounts := a.rankAccountsFromHistory(service, priorEndTime, end, topAccounts)

	if a.demoMode {
		note := "Demo org-wide tag-delta (no CE). Select an account for per-account tags."
		return &analysis.ServiceTagDeltaResponse{
			Service:         service,
			DisplayName:     analysis.FriendlyService(service),
			TagKey:          tagKey,
			CurrentPeriod:   analysis.PeriodSummary{Start: start, End: end},
			PriorPeriod:     analysis.PeriodSummary{Start: priorStart, End: priorEnd},
			Accounts:        accounts,
			TopAccountNote:  note,
			CECallsUsed:     0,
		}, nil
	}

	if a.billingCost == nil {
		return nil, fmt.Errorf("org tag-delta requires payer Cost Explorer (billing profile/role)")
	}

	// Org-wide Name tags at payer scope (no LINKED_ACCOUNT) — 2 CE calls.
	curTagRaw, err := costexplorer.GetServiceTagTotals(ctx, a.billingCost, "", service, start, end, true, tagKey)
	if err != nil {
		return nil, fmt.Errorf("org tag totals (current): %w", err)
	}
	priorTagRaw, err := costexplorer.GetServiceTagTotals(ctx, a.billingCost, "", service, priorStart, priorEnd, true, tagKey)
	if err != nil {
		return nil, fmt.Errorf("org tag totals (prior): %w", err)
	}
	ceCalls := 2

	curTag := costexplorer.NormalizeTagMapKeys(tagKey, curTagRaw)
	priorTag := costexplorer.NormalizeTagMapKeys(tagKey, priorTagRaw)

	var orgCurTotal, orgPriorTotal float64
	for _, v := range curTag {
		orgCurTotal += v
	}
	for _, v := range priorTag {
		orgPriorTotal += v
	}
	orgDeltaUSD := orgCurTotal - orgPriorTotal
	orgDeltaPct := 0.0
	if orgPriorTotal > 0 {
		orgDeltaPct = (orgDeltaUSD / orgPriorTotal) * 100
	}
	orgTopInc, orgTopDec := analysis.BuildTagDeltaDrivers(curTag, priorTag, orgCurTotal, 5)

	note := fmt.Sprintf(
		"Org-wide %s tags (2 CE). Account list from history; select an account for per-account tags.",
		tagKey,
	)
	if len(accounts) == 0 {
		note = fmt.Sprintf(
			"Org-wide %s tags (2 CE). No history yet for account ranking — select an account or warm daily snapshots.",
			tagKey,
		)
	}

	return &analysis.ServiceTagDeltaResponse{
		Service:         service,
		DisplayName:     analysis.FriendlyService(service),
		TagKey:          tagKey,
		CurrentPeriod:   analysis.PeriodSummary{Start: start, End: end},
		PriorPeriod:     analysis.PeriodSummary{Start: priorStart, End: priorEnd},
		TotalCurrentUSD: orgCurTotal,
		TotalPriorUSD:   orgPriorTotal,
		DeltaUSD:        orgDeltaUSD,
		DeltaPercent:    orgDeltaPct,
		TopIncreases:    orgTopInc,
		TopDecreases:    orgTopDec,
		Accounts:        accounts,
		TopAccountNote:  note,
		CECallsUsed:     ceCalls,
	}, nil
}

func (a *Aggregator) rankAccountsFromHistory(
	service string,
	priorEndTime time.Time,
	currentEnd string,
	topAccounts int,
) []analysis.AccountTagDelta {
	if a.history == nil || topAccounts <= 0 {
		return nil
	}

	var priorByAccount, curByAccount map[string]float64
	if snap, err := a.history.FindPriorSnapshot(priorEndTime, 5); err == nil && snap != nil {
		priorByAccount = serviceAmountsByAccount(snap, service)
	}
	curEndTime, _ := time.Parse("2006-01-02", currentEnd)
	if snap, err := findClosestSnapshot(a.history, curEndTime, 5); err == nil && snap != nil {
		curByAccount = serviceAmountsByAccount(snap, service)
	}
	if curByAccount == nil && priorByAccount == nil {
		return nil
	}

	names := make(map[string]string, len(a.clients))
	for _, c := range a.clients {
		names[c.AccountID] = c.Account.Name
	}

	ids := make(map[string]struct{})
	for id := range curByAccount {
		ids[id] = struct{}{}
	}
	for id := range priorByAccount {
		ids[id] = struct{}{}
	}

	ranks := make([]serviceTagDeltaAccountRank, 0, len(ids))
	for id := range ids {
		curAmt := 0.0
		if curByAccount != nil {
			curAmt = curByAccount[id]
		}
		priorAmt := 0.0
		if priorByAccount != nil {
			priorAmt = priorByAccount[id]
		}
		if curAmt <= 0 && priorAmt <= 0 {
			continue
		}
		deltaUSD := curAmt - priorAmt
		name := names[id]
		if name == "" {
			name = id
		}
		ranks = append(ranks, serviceTagDeltaAccountRank{
			accountID:   id,
			accountName: name,
			currentUSD:  curAmt,
			priorUSD:    priorAmt,
			deltaUSD:    deltaUSD,
			scoreUSD:    math.Abs(deltaUSD),
		})
	}
	sort.Slice(ranks, func(i, j int) bool { return ranks[i].scoreUSD > ranks[j].scoreUSD })
	if len(ranks) > topAccounts {
		ranks = ranks[:topAccounts]
	}

	out := make([]analysis.AccountTagDelta, 0, len(ranks))
	for _, r := range ranks {
		deltaPct := 0.0
		if r.priorUSD > 0 {
			deltaPct = (r.deltaUSD / r.priorUSD) * 100
		}
		// Tag lists intentionally empty — filled when account_id is selected.
		out = append(out, analysis.AccountTagDelta{
			AccountID:       r.accountID,
			AccountName:     r.accountName,
			TotalCurrentUSD: r.currentUSD,
			TotalPriorUSD:   r.priorUSD,
			DeltaUSD:        r.deltaUSD,
			DeltaPercent:    deltaPct,
		})
	}
	return out
}

func serviceAmountsByAccount(snap *history.Snapshot, service string) map[string]float64 {
	out := make(map[string]float64)
	for _, acct := range snap.Accounts {
		for _, svc := range acct.Services {
			if svc.Service == service {
				out[acct.AccountID] = svc.Amount
				break
			}
		}
	}
	return out
}

func findClosestSnapshot(store *history.Store, targetEnd time.Time, maxSkewDays int) (*history.Snapshot, error) {
	snaps, err := store.ListSnapshots()
	if err != nil {
		return nil, err
	}
	var best *history.Snapshot
	var bestDelta time.Duration
	for i := range snaps {
		end, err := time.Parse("2006-01-02", snaps[i].PeriodEnd)
		if err != nil {
			continue
		}
		delta := targetEnd.Sub(end)
		if delta < 0 {
			delta = -delta
		}
		if delta > time.Duration(maxSkewDays)*24*time.Hour {
			continue
		}
		if best == nil || delta < bestDelta {
			best = &snaps[i]
			bestDelta = delta
		}
	}
	return best, nil
}
