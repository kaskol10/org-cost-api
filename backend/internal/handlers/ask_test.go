package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAskOversizedBody413(t *testing.T) {
	api := testAPI(t)
	// MaxBytesReader wraps the body; exceed maxAskBodyBytes.
	payload := `{"question":"` + strings.Repeat("x", maxAskBodyBytes+1024) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ask", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	api.Routes("").ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d, want 413: %s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "request body too large" {
		t.Fatalf("error %q, want request body too large", body["error"])
	}
}

func TestAskInvalidJSON400(t *testing.T) {
	api := testAPI(t)
	req := httptest.NewRequest(http.MethodPost, "/api/ask", strings.NewReader(`{not-json`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	api.Routes("").ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400: %s", rec.Code, rec.Body.String())
	}
}
