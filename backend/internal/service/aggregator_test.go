package service

import (
	"context"
	"testing"

	appconfig "github.com/kaskol10/org-cost-api/backend/internal/config"
)

func TestDashboardCopyOnReturnStampIsolation(t *testing.T) {
	cfg := &appconfig.Config{
		Accounts: []appconfig.Account{
			{ID: "111", Name: "good"},
		},
		CostLookbackDays: 30,
	}
	agg := NewTestAggregator(cfg, TestDashboard(cfg))

	a, err := agg.Dashboard(context.Background())
	if err != nil {
		t.Fatalf("Dashboard: %v", err)
	}
	b, err := agg.Dashboard(context.Background())
	if err != nil {
		t.Fatalf("Dashboard: %v", err)
	}
	if a == b {
		t.Fatal("expected distinct DashboardResponse pointers")
	}
	a.GeneratedAt = "stamp-a"
	if b.GeneratedAt == "stamp-a" {
		t.Fatal("stamping one response mutated the other")
	}
}

func TestPartialDashboardAccountError(t *testing.T) {
	cfg := &appconfig.Config{
		Accounts: []appconfig.Account{
			{ID: "111", Name: "good"},
			{ID: "222", Name: "bad"},
		},
		CostLookbackDays: 30,
	}
	dash := TestDashboard(cfg)
	dash.Accounts[1].Error = "volumes: access denied"
	dash.Accounts[1].Costs = nil
	dash.Totals.OrgTotal = 1000
	dash.Totals.AccountCount = 1

	view := toDashboardView(dash)
	if len(view.Accounts) != 1 {
		t.Fatalf("expected 1 account in view, got %d", len(view.Accounts))
	}
	if view.Accounts[0].AccountName != "good" {
		t.Fatalf("account %q, want good", view.Accounts[0].AccountName)
	}
}
