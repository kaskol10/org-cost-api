package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/kaskol10/org-cost-api/backend/internal/demo"
)

func TestDemoModeReport(t *testing.T) {
	cfg := demo.DefaultConfig()
	agg, err := demo.NewAggregator(cfg)
	if err != nil {
		t.Fatalf("demo aggregator: %v", err)
	}

	api := New(agg, cfg.CORSOrigin, "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/report", nil)
	api.Routes("").ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v", err)
	}
	if body["dashboard"] == nil {
		t.Fatal("expected dashboard in report")
	}
}

func TestDemoModeMeta(t *testing.T) {
	cfg := demo.DefaultConfig()
	agg, err := demo.NewAggregator(cfg)
	if err != nil {
		t.Fatalf("demo aggregator: %v", err)
	}
	api := New(agg, "*", "")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/meta", nil)
	api.Routes("").ServeHTTP(rec, req)

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v", err)
	}
	if body["demo"] != true {
		t.Fatalf("demo=%v want true", body["demo"])
	}
}

func TestMain(m *testing.M) {
	os.Unsetenv("ORG_COST_DEMO")
	os.Exit(m.Run())
}
