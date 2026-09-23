package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kaskol10/org-cost-api/backend/internal/service"
)

func TestWriteAPIErrorInvalidRequest(t *testing.T) {
	rec := httptest.NewRecorder()
	writeAPIError(rec, service.NewInvalidRequestError("invalid period"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "invalid period") {
		t.Fatalf("body %q", rec.Body.String())
	}
}

func TestWriteAPIErrorUnknownAccount(t *testing.T) {
	rec := httptest.NewRecorder()
	writeAPIError(rec, service.NewUnknownAccountError("missing"))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404", rec.Code)
	}
}

func TestWriteAPIErrorRateLimited(t *testing.T) {
	rec := httptest.NewRecorder()
	writeAPIError(rec, service.NewRateLimitedError(90*time.Second))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") != "90" {
		t.Fatalf("Retry-After %q", rec.Header().Get("Retry-After"))
	}
	if !strings.Contains(rec.Body.String(), "refresh limited") {
		t.Fatalf("body %q", rec.Body.String())
	}
}

func TestWriteAPIErrorInternal(t *testing.T) {
	rec := httptest.NewRecorder()
	writeAPIError(rec, errors.New("aws profile acme-admin failed"))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status %d, want 500", rec.Code)
	}
	body := rec.Body.String()
	if body == "" {
		t.Fatal("empty body")
	}
	if strings.Contains(body, "acme-admin") {
		t.Fatal("internal error leaked to client")
	}
}
