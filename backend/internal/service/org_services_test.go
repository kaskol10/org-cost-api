package service

import (
	"testing"

	"github.com/kaskol10/org-cost-api/backend/internal/aws/costexplorer"
)

func TestBuildOrgTopServices(t *testing.T) {
	accounts := []AccountDashboard{
		{
			AccountID: "111", AccountName: "prod",
			Costs: &costexplorer.CostSummary{
				ByService: []costexplorer.ServiceCost{
					{Service: "Amazon S3", Amount: 100},
					{Service: "EC2 - Other", Amount: 50},
				},
			},
		},
		{
			AccountID: "222", AccountName: "staging",
			Costs: &costexplorer.CostSummary{
				ByService: []costexplorer.ServiceCost{
					{Service: "Amazon S3", Amount: 40},
					{Service: "Amazon RDS", Amount: 30},
				},
			},
		},
	}

	out := buildOrgTopServices(accounts, 220, "USD")
	if len(out) != 3 {
		t.Fatalf("got %d drivers, want 3", len(out))
	}
	if out[0].Service != "Amazon S3" || out[0].Amount != 140 {
		t.Fatalf("first driver: %+v", out[0])
	}
	if len(out[0].TopAccounts) != 2 || out[0].TopAccounts[0].AccountName != "prod" {
		t.Fatalf("top accounts: %+v", out[0].TopAccounts)
	}
}
