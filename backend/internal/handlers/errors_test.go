package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
