package service

import (
	"context"
	"errors"
	"testing"
	"time"

	appconfig "github.com/kaskol10/org-cost-api/backend/internal/config"
)

func newLimiterAgg(t *testing.T) *Aggregator {
	t.Helper()
	cfg := &appconfig.Config{
		Accounts:         []appconfig.Account{{ID: "111", Name: "a"}},
		CostLookbackDays: 30,
	}
	agg := NewTestAggregator(cfg, TestDashboard(cfg))
	agg.refreshMinInterval = 5 * time.Minute
	return agg
}

func TestAllowForceRefreshLimiter(t *testing.T) {
	agg := newLimiterAgg(t)
	ctx := context.Background() // anonymous client, as with auth disabled

	if coalesce, err := agg.allowForceRefresh(ctx, "k"); err != nil || coalesce {
		t.Fatalf("first refresh: coalesce=%v err=%v", coalesce, err)
	}
	_, err := agg.allowForceRefresh(ctx, "k")
	var rl *RateLimitedError
	if !errors.As(err, &rl) {
		t.Fatalf("same client again: want RateLimitedError, got %T %v", err, err)
	}
	if rl.RetryAfter <= 0 {
		t.Fatalf("retry after %v", rl.RetryAfter)
	}
	// Same client, other key: still its own budget → 429.
	if _, err := agg.allowForceRefresh(ctx, "other"); !errors.As(err, &rl) {
		t.Fatalf("same client other key: want RateLimitedError, got %v", err)
	}

	agg.lastForceByClient[AnonymousClient] = time.Now().Add(-6 * time.Minute)
	agg.lastForceByKey["k"] = time.Now().Add(-6 * time.Minute)
	if coalesce, err := agg.allowForceRefresh(ctx, "k"); err != nil || coalesce {
		t.Fatalf("after interval: coalesce=%v err=%v", coalesce, err)
	}
}

func TestAllowForceRefreshCoalescesAcrossClients(t *testing.T) {
	agg := newLimiterAgg(t)
	backstage := WithClient(context.Background(), "backstage")
	ui, info := WithRefreshInfo(WithClient(context.Background(), "web-ui"))

	if _, err := agg.allowForceRefresh(backstage, "30d"); err != nil {
		t.Fatalf("backstage refresh: %v", err)
	}
	coalesce, err := agg.allowForceRefresh(ui, "30d")
	if err != nil {
		t.Fatalf("web-ui must not be locked out by backstage: %v", err)
	}
	if !coalesce || !info.Coalesced {
		t.Fatal("web-ui refresh should be coalesced onto backstage's fresh data")
	}
	// Coalescing doesn't spend web-ui's own budget: a refresh of another key goes ahead.
	if coalesce, err := agg.allowForceRefresh(ui, "mtd"); err != nil || coalesce {
		t.Fatalf("web-ui mtd refresh: coalesce=%v err=%v", coalesce, err)
	}
	// …and now web-ui has used its budget.
	if _, err := agg.allowForceRefresh(ui, "lite"); err == nil {
		t.Fatal("expected web-ui to be rate limited after its own refresh")
	}
}
