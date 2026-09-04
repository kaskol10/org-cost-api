package service

import (
	"context"
	"fmt"
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

// ServiceTagDelta computes "why did <service> change" by showing the top tag value buckets
// (grouped by Cost Explorer cost allocation tag key, default "Name") for the selected period.
//
// For org scope (accountID empty), it aggregates tag deltas across the top N accounts by abs delta
// (from history snapshot when available; otherwise by current spend).
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
		topAccounts = 5
	}
	if start == "" || end == "" {
		start, end = a.cfg.CostDateRange()
	}

	if start == "" || end == "" {
		return nil, fmt.Errorf("invalid date range")
	}

	priorStart, priorEnd := priorPeriod(start, end)
	priorEndTime, _ := time.Parse("2006-01-02", start) // prior period ends at current start

	cacheKey := fmt.Sprintf(
		"%s|%s|%s|%s|%s|%d",
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
				return &out, nil
			}
		}
	}
	a.cacheMu.Unlock()

	// Selected accounts:
	// - account scope: only one account (accountID)
	// - org scope: top N accounts by abs(delta) using history snapshot if available, else by current.
	type accountClient struct {
		accountID   string
		accountName string
		client      *awsclient.AccountClients
	}
	var selected []accountClient

	// Org scope ranking relies on history snapshots only (no extra dashboard/EC2 inventory calls).
	// This avoids hammering the dashboard endpoint while users click around in the UI.
	var curByAccount map[string]float64
	var priorByAccount map[string]float64
	haveHistory := a.history != nil

	findClosestSnapshot := func(targetEnd time.Time, maxSkewDays int) (*history.Snapshot, error) {
		snaps, err := a.history.ListSnapshots()
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

	if haveHistory {
		// Prior period (already used in earlier trends code).
		if snap, err := a.history.FindPriorSnapshot(priorEndTime, 5); err == nil && snap != nil {
			priorByAccount = make(map[string]float64)
			for _, acct := range snap.Accounts {
				for _, svc := range acct.Services {
					if svc.Service == service {
						priorByAccount[acct.AccountID] = svc.Amount
						break
					}
				}
			}
		}

		// Current period.
		curEndTime, _ := time.Parse("2006-01-02", end)
		if snap, err := findClosestSnapshot(curEndTime, 5); err == nil && snap != nil {
			curByAccount = make(map[string]float64)
			for _, acct := range snap.Accounts {
				for _, svc := range acct.Services {
					if svc.Service == service {
						curByAccount[acct.AccountID] = svc.Amount
						break
					}
				}
			}
		}
	}

	// Choose accounts.
	if accountID != "" {
		found := false
		for _, c := range a.clients {
			if c.AccountID == accountID {
				selected = []accountClient{{
					accountID:   accountID,
					accountName: c.Account.Name,
					client:      c,
				}}
				found = true
				break
			}
		}
		if !found {
			return nil, NewUnknownAccountError(accountID)
		}
	} else {
		// Rank by abs(delta) when we have history; otherwise pick first N accounts.
		var ranks []serviceTagDeltaAccountRank
		if curByAccount != nil || priorByAccount != nil {
			ranks = make([]serviceTagDeltaAccountRank, 0, len(a.clients))
			for _, c := range a.clients {
				curAmt := 0.0
				if curByAccount != nil {
					curAmt = curByAccount[c.AccountID]
				}
				priorAmt := 0.0
				if priorByAccount != nil {
					priorAmt = priorByAccount[c.AccountID]
				}
				if curAmt <= 0 && priorAmt <= 0 {
					continue
				}
				deltaUSD := curAmt - priorAmt
				ranks = append(ranks, serviceTagDeltaAccountRank{
					accountID:   c.AccountID,
					accountName: c.Account.Name,
					currentUSD:  curAmt,
					priorUSD:    priorAmt,
					deltaUSD:    deltaUSD,
					scoreUSD:    math.Abs(deltaUSD),
				})
			}
		}

		if len(ranks) == 0 {
			for i := 0; i < len(a.clients) && len(selected) < topAccounts; i++ {
				c := a.clients[i]
				selected = append(selected, accountClient{
					accountID:   c.AccountID,
					accountName: c.Account.Name,
					client:      c,
				})
			}
		} else {
			sort.Slice(ranks, func(i, j int) bool { return ranks[i].scoreUSD > ranks[j].scoreUSD })
			for _, r := range ranks {
				for _, c := range a.clients {
					if c.AccountID == r.accountID {
						selected = append(selected, accountClient{
							accountID:   r.accountID,
							accountName: r.accountName,
							client:      c,
						})
						break
					}
				}
				if len(selected) >= topAccounts {
					break
				}
			}
		}
	}

	if len(selected) == 0 {
		return &analysis.ServiceTagDeltaResponse{
			Service:        service,
			DisplayName:    analysis.FriendlyService(service),
			TagKey:         tagKey,
			CurrentPeriod:  analysis.PeriodSummary{Start: start, End: end},
			PriorPeriod:    analysis.PeriodSummary{Start: priorStart, End: priorEnd},
			TotalCurrentUSD: 0,
			TotalPriorUSD:   0,
			DeltaUSD:        0,
			DeltaPercent:    0,
			TopIncreases:    nil,
			TopDecreases:    nil,
			Accounts:        nil,
			CECallsUsed:     0,
	}, nil
	}

	orgCur := make(map[string]float64)
	orgPrior := make(map[string]float64)

	var accounts []analysis.AccountTagDelta
	ceCalls := 0

	for _, ac := range selected {
		costCE, fromPayer := a.resolveCostClient(ac.client)

		curTagRaw, err := costexplorer.GetServiceTagTotals(ctx, costCE, ac.accountID, service, start, end, fromPayer, tagKey)
		if err != nil {
			return nil, fmt.Errorf("tag totals (%s current): %w", ac.accountID, err)
		}
		priorTagRaw, err := costexplorer.GetServiceTagTotals(ctx, costCE, ac.accountID, service, priorStart, priorEnd, fromPayer, tagKey)
		if err != nil {
			return nil, fmt.Errorf("tag totals (%s prior): %w", ac.accountID, err)
		}
		ceCalls += 2

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

		accounts = append(accounts, analysis.AccountTagDelta{
			AccountID:         ac.accountID,
			AccountName:       ac.accountName,
			TotalCurrentUSD:  totalCur,
			TotalPriorUSD:    totalPrior,
			DeltaUSD:         deltaUSD,
			DeltaPercent:     deltaPct,
			TopIncreases:     topInc,
			TopDecreases:     topDec,
		})

		for k, v := range curTag {
			orgCur[k] += v
		}
		for k, v := range priorTag {
			orgPrior[k] += v
		}
	}

	var orgCurTotal, orgPriorTotal float64
	for _, v := range orgCur {
		orgCurTotal += v
	}
	for _, v := range orgPrior {
		orgPriorTotal += v
	}

	orgDeltaUSD := orgCurTotal - orgPriorTotal
	orgDeltaPct := 0.0
	if orgPriorTotal > 0 {
		orgDeltaPct = (orgDeltaUSD / orgPriorTotal) * 100
	}

	orgTopInc, orgTopDec := analysis.BuildTagDeltaDrivers(orgCur, orgPrior, orgCurTotal, 5)

	note := fmt.Sprintf("Aggregated from %d top account(s) by ", len(selected))
	if priorByAccount != nil {
		note += "abs delta (history snapshot)"
	} else {
		note += "current spend"
	}

	out := &analysis.ServiceTagDeltaResponse{
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
	}

	a.cacheMu.Lock()
	if a.cachedServiceTagDelta != nil {
		a.cachedServiceTagDelta[cacheKey] = out
		a.cachedServiceTagDeltaAt[cacheKey] = time.Now()
	}
	a.cacheMu.Unlock()

	return out, nil
}

