package service

import (
	"context"
	"errors"
	"testing"

	appconfig "github.com/kaskol10/org-cost-api/backend/internal/config"
)

func TestReadyNoClientsNotReady(t *testing.T) {
	agg := &Aggregator{
		cfg:     &appconfig.Config{},
		clients: nil,
	}
	status := agg.Ready(context.Background())
	if status.Ready {
		t.Fatal("expected ready=false when no account clients")
	}
	if status.AccountsConfigured != 0 {
		t.Fatalf("accounts_configured=%d, want 0", status.AccountsConfigured)
	}
	if status.BillingCheck != "skipped" {
		t.Fatalf("billing_check=%q, want skipped", status.BillingCheck)
	}
	if status.AccountsCheck != "skipped" {
		t.Fatalf("accounts_check=%q, want skipped", status.AccountsCheck)
	}
}

func TestReadyNoBillingSkipped(t *testing.T) {
	cfg := &appconfig.Config{
		Accounts: []appconfig.Account{
			{ID: "123456789012", Name: "production"},
		},
	}
	agg := NewTestAggregator(cfg, TestDashboard(cfg))
	status := agg.Ready(context.Background())
	if !status.Ready {
		t.Fatal("expected ready=true for member-only setup with accounts loaded")
	}
	if status.BillingCheck != "skipped" {
		t.Fatalf("billing_check=%q, want skipped (no payer configured)", status.BillingCheck)
	}
	if status.AccountsCheck != "ok" {
		t.Fatalf("accounts_check=%q, want ok", status.AccountsCheck)
	}
	if status.AccountsConfigured != 1 {
		t.Fatalf("accounts_configured=%d, want 1", status.AccountsConfigured)
	}
}

func TestReadyMemberAccountProbeFailed(t *testing.T) {
	cfg := &appconfig.Config{
		Accounts: []appconfig.Account{
			{ID: "123456789012", Name: "production"},
		},
	}
	agg := NewTestAggregator(cfg, TestDashboard(cfg))
	agg.accountProbe = func(context.Context, appconfig.Account) error {
		return errors.New("sts: expired credentials")
	}
	status := agg.Ready(context.Background())
	if status.Ready {
		t.Fatal("expected ready=false when member account STS fails")
	}
	if status.BillingCheck != "skipped" {
		t.Fatalf("billing_check=%q, want skipped", status.BillingCheck)
	}
	if status.AccountsCheck != "failed" {
		t.Fatalf("accounts_check=%q, want failed", status.AccountsCheck)
	}
	if status.AccountsError == "" {
		t.Fatal("expected accounts_error to be set")
	}
}
