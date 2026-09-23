package service

import (
	"context"
	"fmt"
	"time"

	"github.com/kaskol10/org-cost-api/backend/internal/analysis"
	"github.com/kaskol10/org-cost-api/backend/internal/aws/costexplorer"
	appconfig "github.com/kaskol10/org-cost-api/backend/internal/config"
	"github.com/kaskol10/org-cost-api/backend/internal/errmsg"
	"github.com/kaskol10/org-cost-api/backend/internal/history"
)

// ReportResponse bundles dashboard, trends, and suggestions from a single dashboard fetch.
type ReportResponse struct {
	Dashboard      *DashboardResponse              `json:"dashboard"`
	Trends         *analysis.TrendsResponse        `json:"trends"`
	Suggestions    *analysis.SuggestionsResponse   `json:"suggestions"`
	CECallsUsed    int                             `json:"ce_calls_used"`
	RefreshAllowed bool                            `json:"refresh_allowed"`
}

// Trends returns period-over-period service spend changes with minimal CE usage.
func (a *Aggregator) Trends(ctx context.Context, force bool, period string) (*analysis.TrendsResponse, error) {
	mode, err := parsePeriodMode(period)
	if err != nil {
		return nil, err
	}
	dash, view, _, err := a.dashboardBundle(ctx, force, mode)
	if err != nil {
		return nil, err
	}
	resp, err := a.trendsFromView(ctx, dash, view, mode)
	if err != nil {
		return nil, err
	}
	resp.RefreshAllowed = a.cacheWarm(mode)
	return resp, nil
}

// Suggestions returns ranked cost optimization items for Hermes.
func (a *Aggregator) Suggestions(ctx context.Context, force bool, period string) (*analysis.SuggestionsResponse, error) {
	mode, err := parsePeriodMode(period)
	if err != nil {
		return nil, err
	}
	dash, view, _, err := a.dashboardBundle(ctx, force, mode)
	if err != nil {
		return nil, err
	}
	trends, err := a.trendsFromView(ctx, dash, view, mode)
	if err != nil {
		trends = nil
	}
	out := analysis.BuildSuggestions(view, trends)
	if trends != nil {
		out.CECallsUsed = trends.CECallsUsed
	}
	return out, nil
}

func (a *Aggregator) dashboardBundle(ctx context.Context, force bool, mode appconfig.PeriodMode) (*DashboardResponse, analysis.DashboardView, bool, error) {
	dash, err := a.dashboard(ctx, force, mode)
	if err != nil {
		return nil, analysis.DashboardView{}, false, err
	}
	view := toDashboardView(dash)
	a.recordSnapshot(dash)
	return dash, view, a.cacheWarm(mode) && !force, nil
}

func (a *Aggregator) cacheWarm(mode appconfig.PeriodMode) bool {
	a.cacheMu.Lock()
	defer a.cacheMu.Unlock()
	if a.cachedDash == nil {
		return false
	}
	start, end := a.cfg.CostDateRangeFor(mode)
	key := string(mode) + ":" + start + ":" + end
	return a.cachedDashKey == key && time.Since(a.cachedDashAt) < a.cacheTTL
}

func (a *Aggregator) trendsFromView(ctx context.Context, dash *DashboardResponse, view analysis.DashboardView, mode appconfig.PeriodMode) (*analysis.TrendsResponse, error) {
	priorStart, priorEnd := appconfig.PriorPeriodFor(mode, dash.Start, dash.End)
	priorEndTime, _ := time.Parse("2006-01-02", dash.Start)

	var (
		priorMap      map[string]float64
		priorAccounts map[string]float64
		priorTotal    float64
		source        string
		ceCalls       int
		historyNote   string
		snapCount     int
	)

	finish := func() *analysis.TrendsResponse {
		resp := analysis.BuildTrends(view, priorMap, priorAccounts, priorTotal, source, ceCalls, historyNote, snapCount)
		resp.Period = string(mode)
		resp.PriorPeriod = analysis.PeriodSummary{
			Start: priorStart,
			End:   priorEnd,
			Days:  appconfig.PeriodDayCount(priorStart, priorEnd),
		}
		return resp
	}

	if a.history != nil {
		snaps, _ := a.history.ListSnapshots()
		snapCount = len(snaps)
		if snapCount == 0 {
			historyNote = "No cost history yet — run the dashboard once daily (or call get_org_summary) to enable free prior-period trends."
		}
		// Daily snapshots match trailing lookback windows, not calendar MTD.
		// Only reuse them for 30d so MTD always compares same days last month.
		if mode == appconfig.PeriodLookback {
			if snap, err := a.history.FindPriorSnapshot(priorEndTime, 5); err == nil && snap != nil {
				priorMap, priorTotal = analysis.PriorMapFromSnapshot(snap)
				priorAccounts = analysis.PriorAccountMapFromSnapshot(snap)
				source = "history_snapshot"
				historyNote = fmt.Sprintf(
					"Prior period from snapshot saved %s (period %s to %s). No Cost Explorer calls.",
					snap.SavedAt, snap.PeriodStart, snap.PeriodEnd,
				)
				resp := finish()
				resp.PriorPeriod = analysis.PeriodSummary{
					Start: snap.PeriodStart,
					End:   snap.PeriodEnd,
					Days:  appconfig.PeriodDayCount(snap.PeriodStart, snap.PeriodEnd),
				}
				return resp, nil
			}
		}
		if cache, err := a.history.LoadPriorCache(priorStart, priorEnd); err == nil && cacheFresh(cache.FetchedAt, a.cfg.PriorPeriodCacheHours) {
			priorMap, priorTotal = analysis.PriorMapFromCache(cache)
			priorAccounts = analysis.PriorAccountMapFromCache(cache)
			source = cache.Source
			// Legacy cache may lack account totals — fill with one CE call when possible.
			if len(priorAccounts) == 0 && a.billingCost != nil {
				if accounts, _, acctErr := costexplorer.GetOrgAccountTotals(ctx, a.billingCost, priorStart, priorEnd); acctErr == nil {
					ceCalls = 1
					priorAccounts = make(map[string]float64, len(accounts))
					for _, ac := range accounts {
						priorAccounts[ac.AccountID] = ac.Amount
					}
					_ = a.history.SavePriorCache(history.PriorCache{
						PeriodStart: priorStart,
						PeriodEnd:   priorEnd,
						FetchedAt:   time.Now().UTC().Format(time.RFC3339),
						Services:    cache.Services,
						Accounts:    toHistoryAccounts(priorAccounts, view),
						OrgTotal:    priorTotal,
						Source:      cache.Source,
					})
					historyNote = fmt.Sprintf(
						"Prior services from CE cache (%s); account totals from one payer CE call (cached).",
						cache.FetchedAt,
					)
					return finish(), nil
				}
			}
			historyNote = fmt.Sprintf("Prior period from CE cache (%s). No new CE calls.", cache.FetchedAt)
			return finish(), nil
		}
	}

	if a.billingCost != nil {
		services, total, err := costexplorer.GetOrgServiceTotals(ctx, a.billingCost, priorStart, priorEnd)
		if err != nil {
			if historyNote == "" {
				historyNote = errmsg.HistoryPriorNote(err)
			}
			priorMap = map[string]float64{}
		} else {
			ceCalls = 1
			priorMap = make(map[string]float64, len(services))
			for _, s := range services {
				priorMap[s.Service] = s.Amount
			}
			priorTotal = total
			source = "cost_explorer_payer"
			historyNote = "Prior period from payer Cost Explorer (services + linked accounts; cached 24h). Member-billed accounts may differ."

			if accounts, _, acctErr := costexplorer.GetOrgAccountTotals(ctx, a.billingCost, priorStart, priorEnd); acctErr == nil {
				ceCalls = 2
				priorAccounts = make(map[string]float64, len(accounts))
				for _, ac := range accounts {
					priorAccounts[ac.AccountID] = ac.Amount
				}
			} else {
				priorAccounts = map[string]float64{}
				historyNote = "Prior services from one payer Cost Explorer call (cached 24h). Account movers unavailable (linked-account CE failed)."
			}

			if a.history != nil {
				_ = a.history.SavePriorCache(history.PriorCache{
					PeriodStart: priorStart,
					PeriodEnd:   priorEnd,
					FetchedAt:   time.Now().UTC().Format(time.RFC3339),
					Services:    toHistoryServices(priorMap),
					Accounts:    toHistoryAccounts(priorAccounts, view),
					OrgTotal:    priorTotal,
					Source:      source,
				})
			}
		}
	} else if a.demoMode {
		priorMap, priorAccounts, priorTotal = demoPriorFromView(view)
		source = "demo_fixture"
		if mode == appconfig.PeriodMTD {
			historyNote = fmt.Sprintf(
				"Demo comparison vs same days last month (%s to %s).",
				priorStart, priorEnd,
			)
		} else {
			historyNote = fmt.Sprintf("Demo comparison vs prior period (%s to %s).", priorStart, priorEnd)
		}
	} else {
		if historyNote == "" {
			historyNote = "No billing profile — prior period comparison needs daily snapshots."
		}
		priorMap = map[string]float64{}
		priorAccounts = map[string]float64{}
	}

	return finish(), nil
}

// demoPriorFromView builds a synthetic prior (~8% lower) so MTD/demo trends stay comparable.
func demoPriorFromView(view analysis.DashboardView) (map[string]float64, map[string]float64, float64) {
	const scale = 0.92
	out := make(map[string]float64, len(view.TopServices))
	var total float64
	for _, s := range view.TopServices {
		prior := s.Amount * scale
		out[s.Service] = prior
		total += prior
	}
	accounts := make(map[string]float64, len(view.Accounts))
	for _, acct := range view.Accounts {
		if acct.AccountID == "" {
			continue
		}
		accounts[acct.AccountID] = acct.AllTotal * scale
	}
	if total < 0.01 && view.OrgTotal > 0 {
		total = view.OrgTotal * scale
	}
	return out, accounts, total
}

func toDashboardView(dash *DashboardResponse) analysis.DashboardView {
	view := analysis.DashboardView{
		GeneratedAt: dash.GeneratedAt,
		Start:       dash.Start,
		End:         dash.End,
		OrgTotal:    dash.Totals.OrgTotal,
		Totals: analysis.TotalsView{
			OrgTotal:                 dash.Totals.OrgTotal,
			VolumeAvailable:          dash.Totals.VolumeAvailable,
			VolumeSizeGiB:            dash.Totals.VolumeSizeGiB,
			VolumeFilesystemUsedGiB:  dash.Totals.VolumeFilesystemUsedGiB,
			VolumeUtilizationPercent: dash.Totals.VolumeUtilizationPercent,
		},
	}
	for _, s := range dash.TopServices {
		view.TopServices = append(view.TopServices, analysis.ServiceDriverView{
			Service: s.Service,
			Amount:  s.Amount,
		})
	}
	for _, acct := range dash.Accounts {
		if acct.Error != "" {
			continue
		}
		av := analysis.AccountView{
			AccountID:   acct.AccountID,
			AccountName: acct.AccountName,
		}
		if acct.Costs != nil {
			av.AllTotal = acct.Costs.AllTotal
			av.OtherServicesTotal = acct.Costs.OtherServicesTotal
			for _, s := range acct.Costs.ByService {
				av.ByService = append(av.ByService, analysis.ServiceDriverView{
					Service: s.Service,
					Amount:  s.Amount,
				})
			}
		}
		if acct.Volumes != nil {
			av.Volumes = &analysis.VolumeView{
				Count:          acct.Volumes.Count,
				AvailableCount: acct.Volumes.AvailableCount,
				AvailableGiB:   acct.Volumes.AvailableGiB,
				TotalGiB:       acct.Volumes.TotalGiB,
			}
		}
		view.Accounts = append(view.Accounts, av)
	}
	return view
}

func (a *Aggregator) recordSnapshot(dash *DashboardResponse) {
	if a.history == nil || dash == nil {
		return
	}
	snap := history.Snapshot{
		SavedAt:     time.Now().UTC().Format(time.RFC3339),
		PeriodStart: dash.Start,
		PeriodEnd:   dash.End,
		OrgTotal:    dash.Totals.OrgTotal,
	}
	for _, s := range dash.TopServices {
		snap.Services = append(snap.Services, history.ServiceAmount{
			Service: s.Service,
			Amount:  s.Amount,
		})
	}
	for _, acct := range dash.Accounts {
		if acct.Error != "" {
			continue
		}
		as := history.AccountAmount{
			AccountID:   acct.AccountID,
			AccountName: acct.AccountName,
		}
		if acct.Costs != nil {
			as.AllTotal = acct.Costs.AllTotal
			for _, s := range acct.Costs.ByService {
				as.Services = append(as.Services, history.ServiceAmount{
					Service: s.Service,
					Amount:  s.Amount,
				})
			}
		}
		snap.Accounts = append(snap.Accounts, as)
	}
	_ = a.history.SaveSnapshot(snap)
}

func priorPeriod(currentStart, currentEnd string) (string, string) {
	return appconfig.PriorPeriodFor(appconfig.PeriodLookback, currentStart, currentEnd)
}

func cacheFresh(fetchedAt string, maxHours int) bool {
	t, err := time.Parse(time.RFC3339, fetchedAt)
	if err != nil {
		return false
	}
	return time.Since(t) < time.Duration(maxHours)*time.Hour
}

func toHistoryServices(m map[string]float64) []history.ServiceAmount {
	out := make([]history.ServiceAmount, 0, len(m))
	for svc, amt := range m {
		out = append(out, history.ServiceAmount{Service: svc, Amount: amt})
	}
	return out
}

func toHistoryAccounts(m map[string]float64, view analysis.DashboardView) []history.AccountAmount {
	names := make(map[string]string, len(view.Accounts))
	for _, acct := range view.Accounts {
		names[acct.AccountID] = acct.AccountName
	}
	out := make([]history.AccountAmount, 0, len(m))
	for id, amt := range m {
		name := names[id]
		if name == "" {
			name = id
		}
		out = append(out, history.AccountAmount{
			AccountID:   id,
			AccountName: name,
			AllTotal:    amt,
		})
	}
	return out
}

// SnapshotCount returns the number of saved history snapshots.
func (a *Aggregator) SnapshotCount() int {
	if a.history == nil {
		return 0
	}
	snaps, err := a.history.ListSnapshots()
	if err != nil {
		return 0
	}
	return len(snaps)
}
