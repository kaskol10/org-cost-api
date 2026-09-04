package service

import (
	"context"
	"fmt"
	"time"

	"github.com/kaskol10/org-cost-api/backend/internal/analysis"
	"github.com/kaskol10/org-cost-api/backend/internal/aws/costexplorer"
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
func (a *Aggregator) Trends(ctx context.Context, force bool) (*analysis.TrendsResponse, error) {
	dash, view, _, err := a.dashboardBundle(ctx, force)
	if err != nil {
		return nil, err
	}
	resp, err := a.trendsFromView(ctx, dash, view)
	if err != nil {
		return nil, err
	}
	resp.RefreshAllowed = a.cacheWarm()
	return resp, nil
}

// Suggestions returns ranked cost optimization items for Hermes.
func (a *Aggregator) Suggestions(ctx context.Context, force bool) (*analysis.SuggestionsResponse, error) {
	dash, view, _, err := a.dashboardBundle(ctx, force)
	if err != nil {
		return nil, err
	}
	trends, err := a.trendsFromView(ctx, dash, view)
	if err != nil {
		trends = nil
	}
	out := analysis.BuildSuggestions(view, trends)
	if trends != nil {
		out.CECallsUsed = trends.CECallsUsed
	}
	return out, nil
}

// Report returns dashboard, trends, and suggestions from one dashboard fetch.
func (a *Aggregator) Report(ctx context.Context, force bool) (*ReportResponse, error) {
	dash, view, refreshAllowed, err := a.dashboardBundle(ctx, force)
	if err != nil {
		return nil, err
	}

	trends, err := a.trendsFromView(ctx, dash, view)
	if err != nil {
		return nil, err
	}
	trends.RefreshAllowed = refreshAllowed

	suggestions := analysis.BuildSuggestions(view, trends)
	suggestions.CECallsUsed = trends.CECallsUsed

	return &ReportResponse{
		Dashboard:      dash,
		Trends:         trends,
		Suggestions:    suggestions,
		CECallsUsed:    trends.CECallsUsed,
		RefreshAllowed: refreshAllowed,
	}, nil
}

func (a *Aggregator) dashboardBundle(ctx context.Context, force bool) (*DashboardResponse, analysis.DashboardView, bool, error) {
	dash, err := a.dashboard(ctx, force)
	if err != nil {
		return nil, analysis.DashboardView{}, false, err
	}
	view := toDashboardView(dash)
	a.recordSnapshot(dash)
	return dash, view, a.cacheWarm() && !force, nil
}

func (a *Aggregator) cacheWarm() bool {
	a.cacheMu.Lock()
	defer a.cacheMu.Unlock()
	if a.cachedDash == nil {
		return false
	}
	start, end := a.cfg.CostDateRange()
	key := start + ":" + end
	return a.cachedDashKey == key && time.Since(a.cachedDashAt) < a.cacheTTL
}

func (a *Aggregator) trendsFromView(ctx context.Context, dash *DashboardResponse, view analysis.DashboardView) (*analysis.TrendsResponse, error) {
	priorStart, priorEnd := priorPeriod(dash.Start, dash.End)
	priorEndTime, _ := time.Parse("2006-01-02", dash.Start)

	var (
		priorMap    map[string]float64
		priorTotal  float64
		source      string
		ceCalls     int
		historyNote string
		snapCount   int
	)

	if a.history != nil {
		snaps, _ := a.history.ListSnapshots()
		snapCount = len(snaps)
		if snapCount == 0 {
			historyNote = "No cost history yet — run the dashboard once daily (or call get_org_summary) to enable free prior-period trends."
		}
		if snap, err := a.history.FindPriorSnapshot(priorEndTime, 5); err == nil && snap != nil {
			priorMap, priorTotal = analysis.PriorMapFromSnapshot(snap)
			source = "history_snapshot"
			historyNote = fmt.Sprintf(
				"Prior period from snapshot saved %s (period %s to %s). No Cost Explorer calls.",
				snap.SavedAt, snap.PeriodStart, snap.PeriodEnd,
			)
			resp := analysis.BuildTrends(view, priorMap, priorTotal, source, ceCalls, historyNote, snapCount)
			resp.PriorPeriod = analysis.PeriodSummary{Start: snap.PeriodStart, End: snap.PeriodEnd}
			return resp, nil
		}
		if cache, err := a.history.LoadPriorCache(priorStart, priorEnd); err == nil && cacheFresh(cache.FetchedAt, a.cfg.PriorPeriodCacheHours) {
			priorMap, priorTotal = analysis.PriorMapFromCache(cache)
			source = cache.Source
			historyNote = fmt.Sprintf("Prior period from CE cache (%s). No new CE calls.", cache.FetchedAt)
			resp := analysis.BuildTrends(view, priorMap, priorTotal, source, ceCalls, historyNote, snapCount)
			resp.PriorPeriod = analysis.PeriodSummary{Start: priorStart, End: priorEnd}
			return resp, nil
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
			historyNote = "Prior period from one payer Cost Explorer call (cached 24h). Linked accounts only; member-billed accounts may differ."
			if a.history != nil {
				_ = a.history.SavePriorCache(history.PriorCache{
					PeriodStart: priorStart,
					PeriodEnd:   priorEnd,
					FetchedAt:   time.Now().UTC().Format(time.RFC3339),
					Services:    toHistoryServices(priorMap),
					OrgTotal:    priorTotal,
					Source:      source,
				})
			}
		}
	} else {
		if historyNote == "" {
			historyNote = "No billing profile — prior period comparison needs daily snapshots."
		}
		priorMap = map[string]float64{}
	}

	resp := analysis.BuildTrends(view, priorMap, priorTotal, source, ceCalls, historyNote, snapCount)
	resp.PriorPeriod = analysis.PeriodSummary{Start: priorStart, End: priorEnd}
	return resp, nil
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
	start, err1 := time.Parse("2006-01-02", currentStart)
	end, err2 := time.Parse("2006-01-02", currentEnd)
	if err1 != nil || err2 != nil {
		return "", ""
	}
	days := int(end.Sub(start).Hours() / 24)
	if days < 1 {
		days = 30
	}
	priorEnd := start
	priorStart := start.AddDate(0, 0, -days)
	return priorStart.Format("2006-01-02"), priorEnd.Format("2006-01-02")
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
