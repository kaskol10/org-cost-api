package errmsg

import (
	"errors"
	"strings"
	"testing"
)

func TestCodeAccessDenied(t *testing.T) {
	err := errors.New("AccessDenied: User is not authorized to perform: ce:GetCostAndUsage")
	if got := Code(err); got != "access_denied" {
		t.Fatalf("Code() = %q, want access_denied", got)
	}
}

func TestCodeRateLimited(t *testing.T) {
	err := errors.New("Throttling: Rate exceeded")
	if got := Code(err); got != "rate_limited" {
		t.Fatalf("Code() = %q, want rate_limited", got)
	}
}

func TestAccountComponent(t *testing.T) {
	err := errors.New("ExpiredToken: token expired")
	got := AccountComponent("costs", err)
	if got != "costs: credentials_expired" {
		t.Fatalf("AccountComponent() = %q", got)
	}
}

func TestNoteDoesNotLeakInternals(t *testing.T) {
	err := errors.New("internal aws error with account 123456789012 and role arn")
	got := Note("Could not list EBS volumes", err)
	if strings.Contains(got, "123456789012") || strings.Contains(got, "arn") {
		t.Fatalf("Note leaked internals: %q", got)
	}
}
