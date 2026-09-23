package service

import (
	"errors"
	"testing"
	"time"

	appconfig "github.com/kaskol10/org-cost-api/backend/internal/config"
)

func TestAllowForceRefreshLimiter(t *testing.T) {
	cfg := &appconfig.Config{
		Accounts:         []appconfig.Account{{ID: "111", Name: "a"}},
		CostLookbackDays: 30,
	}
	agg := NewTestAggregator(cfg, TestDashboard(cfg))
	agg.refreshMinInterval = 5 * time.Minute

	if err := agg.allowForceRefresh(); err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	err := agg.allowForceRefresh()
	if err == nil {
		t.Fatal("expected rate limit on second refresh")
	}
	var rl *RateLimitedError
	if !errors.As(err, &rl) {
		t.Fatalf("want RateLimitedError, got %T %v", err, err)
	}
	if rl.RetryAfter <= 0 {
		t.Fatalf("retry after %v", rl.RetryAfter)
	}

	agg.lastForceRefresh = time.Now().Add(-6 * time.Minute)
	if err := agg.allowForceRefresh(); err != nil {
		t.Fatalf("after interval: %v", err)
	}
}

func TestNormalizeReportView(t *testing.T) {
	if NormalizeReportView("lite") != ReportViewLite {
		t.Fatal("lite")
	}
	if NormalizeReportView("") != ReportViewFull {
		t.Fatal("default full")
	}
	if NormalizeReportView("full") != ReportViewFull {
		t.Fatal("full")
	}
}
