package service

import (
	"sort"

	"github.com/kaskol10/org-cost-api/backend/internal/analysis"
	"github.com/kaskol10/org-cost-api/backend/internal/aws/costexplorer"
)

// withDerived returns a shallow copy of dash with fields that used to be
// computed by the web UI: merged org daily series and the unattached-EBS
// estimate priced by volume type. Computing them here means every API
// consumer (UI, MCP, third-party apps) gets the same numbers.
//
// A copy is returned because dash may be a shared cached value; derived
// fields are recomputed per response so older on-disk caches gain them too.
func withDerived(dash *DashboardResponse) *DashboardResponse {
	if dash == nil {
		return nil
	}
	out := *dash
	out.OrgDaily = mergeDaily(dash.Accounts, func(c *costexplorer.CostSummary) []costexplorer.DailyCost { return c.AllDaily })
	out.OrgEC2OtherDaily = mergeDaily(dash.Accounts, func(c *costexplorer.CostSummary) []costexplorer.DailyCost { return c.Daily })

	// Copy the accounts slice so per-account derived fields don't touch the cache.
	out.Accounts = append([]AccountDashboard(nil), dash.Accounts...)
	vols := make([]*analysis.VolumeView, 0, len(out.Accounts))
	for i := range out.Accounts {
		acct := &out.Accounts[i]
		acct.UnattachedEBSEstMonthlyUSD = 0
		if acct.Error != "" || acct.Volumes == nil {
			continue
		}
		v := &analysis.VolumeView{
			AvailableCount: acct.Volumes.AvailableCount,
			AvailableGiB:   acct.Volumes.AvailableGiB,
		}
		for _, t := range acct.Volumes.AvailableByType {
			v.AvailableByType = append(v.AvailableByType, analysis.VolumeTypeView{
				VolumeType: t.VolumeType,
				Count:      t.Count,
				SizeGiB:    t.SizeGiB,
			})
		}
		acct.UnattachedEBSEstMonthlyUSD, _ = analysis.EstimateUnattachedEBS([]*analysis.VolumeView{v})
		vols = append(vols, v)
	}
	est, byType := analysis.EstimateUnattachedEBS(vols)
	out.Totals.VolumeAvailableEstMonthlyUSD = est
	out.Totals.VolumeAvailableByType = byType
	if len(byType) > 0 {
		out.Totals.VolumeAvailablePricingBasis = analysis.EBSPricingBasis
	} else {
		out.Totals.VolumeAvailablePricingBasis = ""
	}
	return &out
}

func mergeDaily(accounts []AccountDashboard, series func(*costexplorer.CostSummary) []costexplorer.DailyCost) []costexplorer.DailyCost {
	byDate := make(map[string]float64)
	unit := "USD"
	for _, acct := range accounts {
		if acct.Error != "" || acct.Costs == nil {
			continue
		}
		for _, d := range series(acct.Costs) {
			byDate[d.Date] += d.Amount
			if d.Unit != "" {
				unit = d.Unit
			}
		}
	}
	if len(byDate) == 0 {
		return nil
	}
	out := make([]costexplorer.DailyCost, 0, len(byDate))
	for date, amount := range byDate {
		out = append(out, costexplorer.DailyCost{Date: date, Amount: amount, Unit: unit})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out
}
