package handlers

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appconfig "github.com/kaskol10/org-cost-api/backend/internal/config"
	"github.com/kaskol10/org-cost-api/backend/internal/service"
)

func clientsTestAPI(t *testing.T) (*service.Aggregator, http.Handler) {
	t.Helper()
	cfg := &appconfig.Config{
		Accounts:         []appconfig.Account{{ID: "111", Name: "a"}},
		CostLookbackDays: 30,
	}
	agg := service.NewTestAggregator(cfg, service.TestDashboard(cfg))
	h := NewWithClients(agg, "*", []APIClient{
		{Name: "web-ui", Token: "tok-ui"},
		{Name: "backstage", Token: "tok-backstage"},
	}).Routes("")
	return agg, h
}

func get(h http.Handler, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })
	return &buf
}

func TestPerClientTokensAuthenticateAndAttribute(t *testing.T) {
	_, h := clientsTestAPI(t)
	logs := captureLog(t)

	for _, tok := range []string{"tok-ui", "tok-backstage"} {
		if rec := get(h, "/api/v1/accounts", tok); rec.Code != http.StatusOK {
			t.Fatalf("token %s: status %d", tok, rec.Code)
		}
	}
	for _, tok := range []string{"", "wrong", "tok-ui-extra"} {
		if rec := get(h, "/api/v1/accounts", tok); rec.Code != http.StatusUnauthorized {
			t.Fatalf("token %q: status %d, want 401", tok, rec.Code)
		}
	}
	out := logs.String()
	for _, want := range []string{"client=web-ui method=GET path=/api/v1/accounts status=200", "client=backstage method=GET"} {
		if !strings.Contains(out, want) {
			t.Fatalf("access log missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "tok-") {
		t.Fatal("access log must never contain tokens")
	}
}

// One integration refreshing must not lock the others out.
func TestRefreshCoalescedAcrossClients(t *testing.T) {
	agg, h := clientsTestAPI(t)
	backstage := service.WithClient(context.Background(), "backstage")
	if err := agg.RecordForceRefreshForTest(backstage, appconfig.PeriodLookback); err != nil {
		t.Fatal(err)
	}

	rec := get(h, "/api/v1/dashboard?refresh=1", "tok-ui")
	if rec.Code != http.StatusOK {
		t.Fatalf("web-ui refresh after backstage refresh: status %d, want 200 (coalesced) body %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("X-Refresh-Coalesced") != "true" {
		t.Fatal("expected X-Refresh-Coalesced: true")
	}

	rec = get(h, "/api/v1/dashboard?refresh=1", "tok-backstage")
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("backstage refreshing again: status %d, want 429 with Retry-After", rec.Code)
	}

	rec = get(h, "/api/v1/dashboard", "tok-ui")
	if rec.Header().Get("X-Refresh-Coalesced") != "" {
		t.Fatal("plain requests must not carry X-Refresh-Coalesced")
	}
}
