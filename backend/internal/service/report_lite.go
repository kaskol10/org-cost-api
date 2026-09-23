package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/kaskol10/org-cost-api/backend/internal/analysis"
	"github.com/kaskol10/org-cost-api/backend/internal/aws/costexplorer"
	appconfig "github.com/kaskol10/org-cost-api/backend/internal/config"
)

const (
	ReportViewFull = "full"
	ReportViewLite = "lite"
)

// NormalizeReportView returns full|lite (default full for SPA compatibility).
func NormalizeReportView(view string) string {
	switch view {
	case ReportViewLite:
		return ReportViewLite
	default:
		return ReportViewFull
	}
}

// Report returns dashboard, trends, and suggestions.
// view=lite builds from payer org totals (~2 CE calls) without per-account GetCostSummary.
func (a *Aggregator) Report(ctx context.Context, force bool, period, view string) (*ReportResponse, error) {
	view = NormalizeReportView(view)
	mode, err := parsePeriodMode(period)
	if err != nil {
		return nil, err
	}

	var (
		dash           *DashboardResponse
		cacheStatus    string
		refreshAllowed bool
		ceCurrent      int
	)

	if view == ReportViewLite {
		dash, cacheStatus, ceCurrent, err = a.dashboardLite(ctx, force, mode)
		if err != nil {
			return nil, err
		}
		refreshAllowed = a.liteCacheWarm(mode) && !force
	} else {
		beforeWarm := a.cacheWarm(mode)
		dash, err = a.dashboard(ctx, force, mode)
		if err != nil {
			return nil, err
		}
		if beforeWarm && !force {
			cacheStatus = "hit"
		} else {
			cacheStatus = "miss"
		}
		refreshAllowed = a.cacheWarm(mode) && !force
		a.recordSnapshot(dash)
	}

	viewDash := toDashboardView(dash)
	trends, err := a.trendsFromView(ctx, dash, viewDash, mode)
	if err != nil {
		return nil, err
	}
	trends.RefreshAllowed = refreshAllowed

	suggestions := analysis.BuildSuggestions(viewDash, trends)
	ceTotal := ceCurrent + trends.CECallsUsed
	suggestions.CECallsUsed = ceTotal
	trends.CECallsUsed = ceTotal

	log.Printf(
		"endpoint=report ce_calls_used=%d cache=%s view=%s period=%s force=%v",
		ceTotal, cacheStatus, view, mode, force,
	)

	return &ReportResponse{
		Dashboard:      dash,
		Trends:         trends,
		Suggestions:    suggestions,
		CECallsUsed:    ceTotal,
		RefreshAllowed: refreshAllowed,
	}, nil
}

func (a *Aggregator) liteCacheKey(mode appconfig.PeriodMode) string {
	start, end := a.cfg.CostDateRangeFor(mode)
	return "lite:" + string(mode) + ":" + start + ":" + end
}

func (a *Aggregator) liteCacheWarm(mode appconfig.PeriodMode) bool {
	if a.history == nil {
		return false
	}
	_, _, err := a.history.LoadDashboardCache(a.liteCacheKey(mode), a.cacheTTL)
	return err == nil
}

// dashboardLite builds a ReportResponse-compatible dashboard from payer org totals.
func (a *Aggregator) dashboardLite(ctx context.Context, force bool, mode appconfig.PeriodMode) (*DashboardResponse, string, int, error) {
	start, end := a.cfg.CostDateRangeFor(mode)
	key := a.liteCacheKey(mode)

	if force {
		if err := a.allowForceRefresh(); err != nil {
			return nil, "miss", 0, err
		}
		if a.history != nil {
			_ = a.history.DeleteDashboardCache(key)
		}
	}

	if !force {
		if a.history != nil {
			if raw, _, err := a.history.LoadDashboardCache(key, a.cacheTTL); err == nil {
				var dash DashboardResponse
				if json.Unmarshal(raw, &dash) == nil {
					return &dash, "hit", 0, nil
				}
			}
		}
	}

	if a.demoMode && a.demoRefresh != nil {
		dash := a.demoRefresh(start, end)
		if dash == nil {
			return nil, "miss", 0, fmt.Errorf("demo fixture unavailable")
		}
		out := *dash
		out.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
		a.saveDiskDashboard(key, &out)
		a.recordSnapshot(&out)
		return &out, "miss", 0, nil
	}

	if a.billingCost == nil {
		return nil, "miss", 0, fmt.Errorf("lite report requires payer Cost Explorer (billing profile/role)")
	}

	accountsRaw, orgTotal, err := costexplorer.GetOrgAccountTotals(ctx, a.billingCost, start, end)
	if err != nil {
		return nil, "miss", 0, fmt.Errorf("org account totals: %w", err)
	}
	servicesRaw, _, err := costexplorer.GetOrgServiceTotals(ctx, a.billingCost, start, end)
	if err != nil {
		return nil, "miss", 2, fmt.Errorf("org service totals: %w", err)
	}
	ceCalls := 2

	names := make(map[string]string, len(a.clients))
	for _, c := range a.clients {
		names[c.AccountID] = c.Account.Name
	}

	sort.Slice(accountsRaw, func(i, j int) bool {
		return accountsRaw[i].Amount > accountsRaw[j].Amount
	})

	accounts := make([]AccountDashboard, 0, len(accountsRaw))
	for _, ac := range accountsRaw {
		name := names[ac.AccountID]
		if name == "" {
			name = ac.AccountID
		}
		accounts = append(accounts, AccountDashboard{
			AccountID:   ac.AccountID,
			AccountName: name,
			Costs: &costexplorer.CostSummary{
				AccountID:   ac.AccountID,
				AccountName: name,
				Start:       start,
				End:         end,
				AllTotal:    ac.Amount,
				Unit:        "USD",
			},
		})
	}

	topServices := make([]OrgServiceDriver, 0, len(servicesRaw))
	for _, s := range servicesRaw {
		if s.Amount <= 0 {
			continue
		}
		driver := OrgServiceDriver{
			Service: s.Service,
			Amount:  s.Amount,
			Unit:    "USD",
		}
		if orgTotal > 0 {
			driver.Percent = (s.Amount / orgTotal) * 100
		}
		topServices = append(topServices, driver)
	}
	sort.Slice(topServices, func(i, j int) bool {
		return topServices[i].Amount > topServices[j].Amount
	})
	const maxTop = 15
	if len(topServices) > maxTop {
		topServices = topServices[:maxTop]
	}

	dash := &DashboardResponse{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Start:       start,
		End:         end,
		Accounts:    accounts,
		Totals: ConsolidatedTotals{
			OrgTotal:     orgTotal,
			Unit:         "USD",
			AccountCount: len(accounts),
		},
		TopServices: topServices,
	}

	a.saveDiskDashboard(key, dash)
	a.recordSnapshot(dash)
	return dash, "miss", ceCalls, nil
}
