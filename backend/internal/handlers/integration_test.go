package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	appconfig "github.com/kaskol10/org-cost-api/backend/internal/config"
	"github.com/kaskol10/org-cost-api/backend/internal/service"
)

func testAPI(t *testing.T) *API {
	t.Helper()
	cfg := &appconfig.Config{
		Accounts: []appconfig.Account{
			{ID: "123456789012", Name: "production"},
			{ID: "210987654321", Name: "staging"},
		},
		CostLookbackDays: 30,
	}
	agg := service.NewTestAggregator(cfg, service.TestDashboard(cfg))
	return New(agg, "http://localhost:5173", "")
}

func TestAccountCostsUnknownAccount404(t *testing.T) {
	api := testAPI(t)
	req := httptest.NewRequest(http.MethodGet, "/api/account-costs?account=missing", nil)
	rec := httptest.NewRecorder()
	api.Routes("").ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "unknown account" {
		t.Fatalf("error %q, want unknown account", body["error"])
	}
	if strings.Contains(body["error"], "123456789012") {
		t.Fatal("error leaked account list")
	}
}

func TestServiceDetailUnknownAccount404(t *testing.T) {
	api := testAPI(t)
	req := httptest.NewRequest(http.MethodGet, "/api/service-detail?account_id=999&service=S3", nil)
	rec := httptest.NewRecorder()
	api.Routes("").ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "unknown account" {
		t.Fatalf("error %q", body["error"])
	}
}

func TestAskRoutesTrendsBeforeAccount(t *testing.T) {
	api := testAPI(t)
	req := httptest.NewRequest(http.MethodPost, "/api/ask", strings.NewReader(`{"question":"How did production change?"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	api.Routes("").ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var body service.AskResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Intent != "trends" {
		t.Fatalf("intent %q, want trends", body.Intent)
	}
	if body.Answer == "" {
		t.Fatal("expected non-empty answer")
	}
}

func TestAskAccountNameOnly(t *testing.T) {
	api := testAPI(t)
	req := httptest.NewRequest(http.MethodPost, "/api/ask", strings.NewReader(`{"question":"Tell me about production"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	api.Routes("").ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var body service.AskResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Intent != "account" {
		t.Fatalf("intent %q, want account", body.Intent)
	}
}

func TestAccountCostsKnownAccount(t *testing.T) {
	api := testAPI(t)
	req := httptest.NewRequest(http.MethodGet, "/api/account-costs?account=production", nil)
	rec := httptest.NewRecorder()
	api.Routes("").ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["account_name"] != "production" {
		t.Fatalf("account_name %v", body["account_name"])
	}
}

func TestReportEndpoint(t *testing.T) {
	api := testAPI(t)
	req := httptest.NewRequest(http.MethodGet, "/api/report", nil)
	rec := httptest.NewRecorder()
	api.Routes("").ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"dashboard", "trends", "suggestions", "ce_calls_used", "refresh_allowed"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("missing key %q in report response", key)
		}
	}
	dash, ok := body["dashboard"].(map[string]any)
	if !ok {
		t.Fatal("dashboard is not an object")
	}
	ga, ok := dash["generated_at"].(string)
	if !ok || ga == "" {
		t.Fatal("dashboard.generated_at missing")
	}
	if _, err := time.Parse(time.RFC3339, ga); err != nil {
		t.Fatalf("dashboard.generated_at %q is not RFC3339: %v", ga, err)
	}
}

func TestServiceTagDeltaUnknownAccount404(t *testing.T) {
	api := testAPI(t)
	req := httptest.NewRequest(http.MethodGet, "/api/service-tag-delta?service=S3&account_id=999", nil)
	rec := httptest.NewRecorder()
	api.Routes("").ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "unknown account" {
		t.Fatalf("error %q, want unknown account", body["error"])
	}
}

func TestServiceTagTotalsUnknownAccount404(t *testing.T) {
	api := testAPI(t)
	req := httptest.NewRequest(http.MethodGet, "/api/service-tag-totals?service=S3&account_id=999", nil)
	rec := httptest.NewRecorder()
	api.Routes("").ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "unknown account" {
		t.Fatalf("error %q, want unknown account", body["error"])
	}
}
