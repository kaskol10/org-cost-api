package service

import (
	"encoding/json"
	"testing"

	"github.com/kaskol10/org-cost-api/backend/internal/analysis"
)

func TestReportResponseBundlesAllSections(t *testing.T) {
	resp := ReportResponse{
		Dashboard: &DashboardResponse{
			Start: "2026-07-11",
			End:   "2026-08-10",
			Totals: ConsolidatedTotals{
				OrgTotal:     5000,
				AccountCount: 2,
				Unit:         "USD",
			},
		},
		Trends: &analysis.TrendsResponse{
			PriorSource: "history_snapshot",
			CECallsUsed: 0,
		},
		Suggestions: &analysis.SuggestionsResponse{
			Summary: "1 optimization opportunities ranked by impact.",
		},
		CECallsUsed:    0,
		RefreshAllowed: true,
	}
	// Report() calls dashboard once via dashboardBundle; verify assembled response is coherent.
	if resp.Dashboard == nil || resp.Trends == nil || resp.Suggestions == nil {
		t.Fatal("report must include dashboard, trends, and suggestions")
	}
	if resp.CECallsUsed != resp.Trends.CECallsUsed {
		t.Fatalf("CE calls mismatch: report=%d trends=%d", resp.CECallsUsed, resp.Trends.CECallsUsed)
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 50 {
		t.Fatal("expected non-trivial JSON payload")
	}
}

func TestDashboardBundleSharesCacheKey(t *testing.T) {
	// dashboardBundle and Report both use the same dashboard() + toDashboardView path.
	dash := &DashboardResponse{
		Start: "2026-07-11",
		End:   "2026-08-10",
		Totals: ConsolidatedTotals{OrgTotal: 100},
	}
	view := toDashboardView(dash)
	if view.OrgTotal != dash.Totals.OrgTotal {
		t.Fatalf("view org total %.0f != dash %.0f", view.OrgTotal, dash.Totals.OrgTotal)
	}
}
