package service

import (
	"testing"

	"github.com/kaskol10/org-cost-api/backend/internal/aws/costexplorer"
	"github.com/kaskol10/org-cost-api/backend/internal/aws/ec2volumes"
)

func TestWithDerived(t *testing.T) {
	orig := &DashboardResponse{
		Accounts: []AccountDashboard{
			{
				AccountID: "1",
				Costs: &costexplorer.CostSummary{
					AllDaily: []costexplorer.DailyCost{{Date: "2026-11-02", Amount: 10, Unit: "USD"}, {Date: "2026-11-01", Amount: 5, Unit: "USD"}},
					Daily:    []costexplorer.DailyCost{{Date: "2026-11-01", Amount: 1, Unit: "USD"}},
				},
				Volumes: &ec2volumes.Inventory{AvailableCount: 1, AvailableGiB: 100, AvailableByType: []ec2volumes.TypeBreakdown{{VolumeType: "io1", Count: 1, SizeGiB: 100}}},
			},
			{
				AccountID: "2",
				Costs: &costexplorer.CostSummary{
					AllDaily: []costexplorer.DailyCost{{Date: "2026-11-01", Amount: 2, Unit: "USD"}},
				},
			},
			{AccountID: "3", Error: "boom", Costs: &costexplorer.CostSummary{AllDaily: []costexplorer.DailyCost{{Date: "2026-11-01", Amount: 999}}}},
		},
	}
	got := withDerived(orig)

	if got == orig {
		t.Fatal("withDerived must return a copy, not mutate the cached value")
	}
	if orig.OrgDaily != nil || orig.Totals.VolumeAvailableEstMonthlyUSD != 0 {
		t.Fatal("original dashboard was mutated")
	}
	if len(got.OrgDaily) != 2 || got.OrgDaily[0].Date != "2026-11-01" || got.OrgDaily[0].Amount != 7 || got.OrgDaily[1].Amount != 10 {
		t.Fatalf("org daily = %+v (errored accounts must be skipped, dates sorted)", got.OrgDaily)
	}
	if len(got.OrgEC2OtherDaily) != 1 || got.OrgEC2OtherDaily[0].Amount != 1 {
		t.Fatalf("ec2-other daily = %+v", got.OrgEC2OtherDaily)
	}
	if got.Totals.VolumeAvailableEstMonthlyUSD != 12.5 {
		t.Fatalf("waste estimate = %v, want 12.5 (100 GiB io1)", got.Totals.VolumeAvailableEstMonthlyUSD)
	}
	if got.Totals.VolumeAvailablePricingBasis == "" || len(got.Totals.VolumeAvailableByType) != 1 {
		t.Fatalf("expected pricing basis + breakdown, got %+v", got.Totals)
	}
	if got.Accounts[0].UnattachedEBSEstMonthlyUSD != 12.5 || orig.Accounts[0].UnattachedEBSEstMonthlyUSD != 0 {
		t.Fatalf("per-account estimate = %v (original must stay untouched)", got.Accounts[0].UnattachedEBSEstMonthlyUSD)
	}
	if withDerived(nil) != nil {
		t.Fatal("nil in, nil out")
	}
}
