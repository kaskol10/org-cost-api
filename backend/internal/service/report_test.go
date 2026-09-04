package service

import (
	"testing"
	"time"

	"github.com/kaskol10/org-cost-api/backend/internal/aws/costexplorer"
)

func TestPriorPeriod(t *testing.T) {
	start, end := priorPeriod("2026-07-11", "2026-08-10")
	if start != "2026-06-11" || end != "2026-07-11" {
		t.Fatalf("got %q–%q, want 2026-06-11–2026-07-11", start, end)
	}
}

func TestCacheFresh(t *testing.T) {
	recent := time.Now().UTC().Format(time.RFC3339)
	if !cacheFresh(recent, 24) {
		t.Fatal("expected fresh cache")
	}
	old := time.Now().UTC().Add(-48 * time.Hour).Format(time.RFC3339)
	if cacheFresh(old, 24) {
		t.Fatal("expected stale cache")
	}
}

func TestToDashboardViewOtherServicesTotal(t *testing.T) {
	dash := &DashboardResponse{
		Start: "2026-07-11",
		End:   "2026-08-10",
		Totals: ConsolidatedTotals{
			OrgTotal: 1000,
		},
		Accounts: []AccountDashboard{
			{
				AccountID:   "111",
				AccountName: "prod",
				Costs: &costexplorer.CostSummary{
					AllTotal:           500,
					OtherServicesTotal: 450,
				},
			},
		},
	}
	view := toDashboardView(dash)
	if view.Accounts[0].OtherServicesTotal != 450 {
		t.Fatalf("got %.0f, want 450", view.Accounts[0].OtherServicesTotal)
	}
}
