package demo

import (
	"testing"

	appconfig "github.com/kaskol10/org-cost-api/backend/internal/config"
)

func TestDemoDashboard(t *testing.T) {
	cfg := DefaultConfig()
	dash := Dashboard(cfg)
	if dash.Totals.OrgTotal <= 0 {
		t.Fatalf("org total=%v", dash.Totals.OrgTotal)
	}
	if len(dash.Accounts) != 5 {
		t.Fatalf("accounts=%d", len(dash.Accounts))
	}
	if dash.Totals.VolumeAvailable < 1 {
		t.Fatal("expected unattached volume waste signal")
	}
}

func TestNewAggregator(t *testing.T) {
	agg, err := NewAggregator(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if !agg.DemoMode() {
		t.Fatal("expected demo mode")
	}
	if agg.SnapshotCount() < 1 {
		t.Fatal("expected seeded history snapshot")
	}
}

func TestMTDTrendsCompareSameDaysLastMonth(t *testing.T) {
	agg, err := NewAggregator(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	trends, err := agg.Trends(t.Context(), false, "mtd")
	if err != nil {
		t.Fatal(err)
	}
	if trends.Period != "mtd" {
		t.Fatalf("period=%q", trends.Period)
	}
	wantStart, wantEnd := appconfig.PriorPeriodFor(
		appconfig.PeriodMTD,
		trends.CurrentPeriod.Start,
		trends.CurrentPeriod.End,
	)
	if trends.PriorPeriod.Start != wantStart || trends.PriorPeriod.End != wantEnd {
		t.Fatalf("prior=%s–%s, want %s–%s",
			trends.PriorPeriod.Start, trends.PriorPeriod.End, wantStart, wantEnd)
	}
	if trends.OrgTotal.PriorUSD <= 0 {
		t.Fatal("expected prior MTD comparison total")
	}
}


func TestDemoEnabledEnv(t *testing.T) {
	t.Setenv("ORG_COST_DEMO", "1")
	if !appconfig.DemoEnabled() {
		t.Fatal("expected demo enabled")
	}
}
