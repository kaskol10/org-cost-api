package service

import (
	"context"

	awsclient "github.com/kaskol10/org-cost-api/backend/internal/aws"
	appconfig "github.com/kaskol10/org-cost-api/backend/internal/config"
	"github.com/kaskol10/org-cost-api/backend/internal/errmsg"
)

// ReadyStatus is returned by GET /api/ready for Kubernetes readiness probes.
//
// billing_check values:
//   - "ok": payer/billing credentials probed successfully (profile STS or static STS)
//   - "failed": billing configured but probe failed — Ready is false
//   - "skipped": no billing profile/static keys configured — Ready stays true when
//     accounts_check is ok (valid for member-only setups). K8s should still probe /api/ready;
//     "skipped" means config OK, no payer probe.
//
// accounts_check values:
//   - "ok": member account credentials usable (boot-loaded clients, or re-STS of account[0]
//     when billing is skipped)
//   - "failed": member account STS failed — Ready is false
//   - "skipped": no accounts configured
type ReadyStatus struct {
	Ready              bool   `json:"ready"`
	AccountsConfigured int    `json:"accounts_configured"`
	SnapshotCount      int    `json:"snapshot_count"`
	BillingCheck       string `json:"billing_check"`  // ok, skipped, failed
	BillingError       string `json:"billing_error,omitempty"`
	AccountsCheck      string `json:"accounts_check"` // ok, skipped, failed
	AccountsError      string `json:"accounts_error,omitempty"`
}

// Ready reports whether the service can serve traffic.
func (a *Aggregator) Ready(ctx context.Context) ReadyStatus {
	if a.demoMode {
		return ReadyStatus{
			Ready:              true,
			AccountsConfigured: len(a.clients),
			SnapshotCount:      a.SnapshotCount(),
			BillingCheck:       "skipped",
			AccountsCheck:      "ok",
		}
	}
	status := ReadyStatus{
		Ready:              len(a.clients) > 0,
		AccountsConfigured: len(a.clients),
		SnapshotCount:      a.SnapshotCount(),
		BillingCheck:       "skipped",
		AccountsCheck:      "skipped",
	}
	if !status.Ready {
		return status
	}
	status.AccountsCheck = "ok"

	if a.cfg.BillingProfile != "" {
		_, err := awsclient.LoadAccountClients(ctx, appconfig.Account{
			Profile: a.cfg.BillingProfile,
			Region:  "us-east-1",
		})
		if err != nil {
			status.Ready = false
			status.BillingCheck = "failed"
			status.BillingError = errmsg.Code(err)
		} else {
			status.BillingCheck = "ok"
		}
		return status
	}

	if a.cfg.HasBillingStaticCredentials() {
		creds, err := a.cfg.ResolveBillingStaticCredentials()
		if err != nil {
			status.Ready = false
			status.BillingCheck = "failed"
			status.BillingError = errmsg.Code(err)
			return status
		}
		if _, err = awsclient.LoadBillingCostClientWithCredentials(ctx, creds); err != nil {
			status.Ready = false
			status.BillingCheck = "failed"
			status.BillingError = errmsg.Code(err)
			return status
		}
		// Client construction alone is not enough — require a live STS call.
		if err = awsclient.ProbeCallerIdentity(ctx, creds); err != nil {
			status.Ready = false
			status.BillingCheck = "failed"
			status.BillingError = errmsg.Code(err)
			return status
		}
		status.BillingCheck = "ok"
		return status
	}

	// Member-only: re-STS account[0] so credential death after boot fails readiness.
	if err := a.probeFirstAccount(ctx); err != nil {
		status.Ready = false
		status.AccountsCheck = "failed"
		status.AccountsError = errmsg.Code(err)
	}
	return status
}

func (a *Aggregator) probeFirstAccount(ctx context.Context) error {
	if len(a.cfg.Accounts) == 0 {
		return nil
	}
	acct := a.cfg.Accounts[0]
	if a.accountProbe != nil {
		return a.accountProbe(ctx, acct)
	}
	_, err := awsclient.LoadAccountClients(ctx, acct)
	return err
}
