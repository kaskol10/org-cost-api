package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"

	"github.com/kaskol10/org-cost-api/backend/internal/demo"
)

func demoAPI(t *testing.T, token string) http.Handler {
	t.Helper()
	cfg := demo.DefaultConfig()
	agg, err := demo.NewAggregator(cfg)
	if err != nil {
		t.Fatalf("demo aggregator: %v", err)
	}
	return New(agg, cfg.CORSOrigin, token).Routes("")
}

// Every endpoint must answer under /api/v1 and the legacy /api alias.
func TestEndpointsServedUnderBothPrefixes(t *testing.T) {
	h := demoAPI(t, "")
	api := New(nil, "", "")
	for _, r := range api.Endpoints() {
		for _, prefix := range []string{APIVersionPrefix, LegacyPrefix} {
			var body *strings.Reader
			if r.Method == http.MethodPost {
				body = strings.NewReader(`{"question":"What is our total spend?"}`)
			} else {
				body = strings.NewReader("")
			}
			req := httptest.NewRequest(r.Method, prefix+r.Path, body)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code == http.StatusNotFound || rec.Code == http.StatusMethodNotAllowed {
				t.Errorf("%s %s%s: status %d", r.Method, prefix, r.Path, rec.Code)
			}
		}
	}
}

func TestPublicPathsExemptUnderBothPrefixes(t *testing.T) {
	h := demoAPI(t, "secret-token")
	for _, p := range []string{"/api/health", "/api/v1/health", "/api/v1/ready", "/api/v1/meta"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code == http.StatusUnauthorized {
			t.Errorf("%s should be auth-exempt, got 401", p)
		}
	}
	for _, p := range []string{"/api/report", "/api/v1/report", "/api/v1/accounts"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s without token: status %d, want 401", p, rec.Code)
		}
	}
	if isPublicPath("/api/v1healthz") || isPublicPath("/api/v2/health") {
		t.Error("isPublicPath matched a non-public path")
	}
}

// openapi.yaml is the contract third-party clients code against: it must
// list exactly the routes the server registers, under /api/v1.
func TestOpenAPIMatchesRegisteredRoutes(t *testing.T) {
	raw, err := os.ReadFile("../../../openapi.yaml")
	if err != nil {
		t.Fatalf("read openapi.yaml: %v", err)
	}
	var spec struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}
	var inSpec []string
	for path, ops := range spec.Paths {
		for method := range ops {
			switch method {
			case "get", "post", "put", "patch", "delete":
				inSpec = append(inSpec, strings.ToUpper(method)+" "+path)
			}
		}
	}
	var inServer []string
	for _, r := range New(nil, "", "").Endpoints() {
		inServer = append(inServer, r.Method+" "+APIVersionPrefix+r.Path)
	}
	sort.Strings(inSpec)
	sort.Strings(inServer)
	if strings.Join(inSpec, "\n") != strings.Join(inServer, "\n") {
		t.Fatalf("openapi.yaml paths and registered routes differ\nspec:\n  %s\nserver:\n  %s",
			strings.Join(inSpec, "\n  "), strings.Join(inServer, "\n  "))
	}
}
