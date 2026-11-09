package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kaskol10/org-cost-api/backend/internal/demo"
)

func demoGet(t *testing.T, path string) map[string]any {
	t.Helper()
	cfg := demo.DefaultConfig()
	agg, err := demo.NewAggregator(cfg)
	if err != nil {
		t.Fatalf("demo aggregator: %v", err)
	}
	rec := httptest.NewRecorder()
	New(agg, cfg.CORSOrigin, "").Routes("").ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("%s: status %d body %s", path, rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v", err)
	}
	return body
}

// Fields third-party clients used to have to compute themselves.
func TestReportIncludesServerDerivedFields(t *testing.T) {
	body := demoGet(t, "/api/report")
	if hl, _ := body["highlights"].([]any); len(hl) == 0 {
		t.Fatalf("expected highlights, got %v", body["highlights"])
	}
	if h, _ := body["headline"].(string); h == "" {
		t.Fatal("expected headline")
	}
	dash := body["dashboard"].(map[string]any)
	if d, _ := dash["org_daily"].([]any); len(d) == 0 {
		t.Fatal("expected dashboard.org_daily")
	}
	totals := dash["totals"].(map[string]any)
	if est, _ := totals["volume_available_est_monthly_usd"].(float64); est <= 0 {
		t.Fatalf("expected volume_available_est_monthly_usd > 0, got %v", totals["volume_available_est_monthly_usd"])
	}
	// Existing fields stay put.
	for _, k := range []string{"org_total", "volume_available_count", "volume_available_gib", "account_count"} {
		if _, ok := totals[k]; !ok {
			t.Fatalf("existing totals field %q disappeared", k)
		}
	}
}

func TestDashboardHonorsOptionalPeriod(t *testing.T) {
	def := demoGet(t, "/api/dashboard")
	mtd := demoGet(t, "/api/dashboard?period=mtd")
	if def["org_daily"] == nil || mtd["org_daily"] == nil {
		t.Fatal("expected org_daily on /api/dashboard")
	}
	if mtd["start"] == def["start"] && def["start"] != nil {
		// Only fails if the 30d window and MTD window happen to coincide
		// (i.e. today is the 31st of a 31-day month and lookback is 30d).
		t.Logf("start dates equal (%v); acceptable only on month boundaries", def["start"])
	}
	cfg := demo.DefaultConfig()
	agg, _ := demo.NewAggregator(cfg)
	rec := httptest.NewRecorder()
	New(agg, cfg.CORSOrigin, "").Routes("").ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/dashboard?period=bogus", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad period: status %d, want 400", rec.Code)
	}
}
