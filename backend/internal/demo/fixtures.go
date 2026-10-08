package demo

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/kaskol10/org-cost-api/backend/internal/aws/costexplorer"
	"github.com/kaskol10/org-cost-api/backend/internal/aws/ec2snapshots"
	"github.com/kaskol10/org-cost-api/backend/internal/aws/ec2volumes"
	appconfig "github.com/kaskol10/org-cost-api/backend/internal/config"
	"github.com/kaskol10/org-cost-api/backend/internal/history"
	"github.com/kaskol10/org-cost-api/backend/internal/service"
)

// DefaultConfig returns a minimal config for demo mode when no file is provided.
func DefaultConfig() *appconfig.Config {
	return &appconfig.Config{
		Demo:             true,
		ListenAddr:       ":8080",
		CORSOrigin:       "*",
		CostLookbackDays: 30,
		CostLag:          2,
		Accounts: []appconfig.Account{
			{ID: "111111111111", Name: "production"},
			{ID: "222222222222", Name: "staging"},
			{ID: "333333333333", Name: "analytics"},
			{ID: "444444444444", Name: "sandbox"},
			{ID: "555555555555", Name: "legacy"},
		},
		// Demo org runs ~$72k/month; a $65k budget puts the projection over.
		Budgets: []appconfig.Budget{
			{Name: "org", MonthlyUSD: 65000},
		},
	}
}

type accountFixture struct {
	id      string
	name    string
	total   float64
	services []costexplorer.ServiceCost
	volumes *ec2volumes.Inventory
	snaps   *ec2snapshots.SnapshotSummary
}

// Dashboard builds a rich fixture dashboard for demo mode.
func Dashboard(cfg *appconfig.Config) *service.DashboardResponse {
	start, end := cfg.CostDateRange()
	return DashboardForRange(cfg, start, end)
}

// DashboardForRange builds a fixture dashboard for an explicit date window.
func DashboardForRange(cfg *appconfig.Config, start, end string) *service.DashboardResponse {
	if start == "" || end == "" {
		start, end = cfg.CostDateRange()
	}
	fixtures := []accountFixture{
		{
			id: "111111111111", name: "production", total: 48200,
			services: []costexplorer.ServiceCost{
				{Service: "Amazon Elastic Compute Cloud - Compute", Amount: 18500},
				{Service: "Amazon Relational Database Service", Amount: 9200},
				{Service: "Amazon Simple Storage Service", Amount: 6100},
				{Service: "EC2 - Other", Amount: 4800},
			},
			volumes: &ec2volumes.Inventory{Count: 240, AvailableCount: 2, AvailableGiB: 80, TotalGiB: 4800},
			snaps:   &ec2snapshots.SnapshotSummary{Count: 890, TotalSizeGiB: 12000},
		},
		{
			id: "222222222222", name: "staging", total: 11800,
			services: []costexplorer.ServiceCost{
				{Service: "Amazon Elastic Compute Cloud - Compute", Amount: 4200},
				{Service: "Amazon Simple Storage Service", Amount: 2800},
				{Service: "AmazonCloudWatch", Amount: 1900},
				{Service: "EC2 - Other", Amount: 1500},
			},
			volumes: &ec2volumes.Inventory{Count: 68, AvailableCount: 5, AvailableGiB: 250, TotalGiB: 920},
			snaps:   &ec2snapshots.SnapshotSummary{Count: 120, TotalSizeGiB: 1800},
		},
		{
			id: "333333333333", name: "analytics", total: 8400,
			services: []costexplorer.ServiceCost{
				{Service: "Amazon Redshift", Amount: 3200},
				{Service: "Amazon Simple Storage Service", Amount: 2100},
				{Service: "AWS Glue", Amount: 1400},
				{Service: "EC2 - Other", Amount: 900},
			},
			volumes: &ec2volumes.Inventory{Count: 42, TotalGiB: 2100},
			snaps:   &ec2snapshots.SnapshotSummary{Count: 45, TotalSizeGiB: 600},
		},
		{
			id: "444444444444", name: "sandbox", total: 2100,
			services: []costexplorer.ServiceCost{
				{Service: "Amazon Elastic Compute Cloud - Compute", Amount: 900},
				{Service: "Amazon Simple Storage Service", Amount: 600},
				{Service: "EC2 - Other", Amount: 400},
			},
			volumes: &ec2volumes.Inventory{Count: 18, AvailableCount: 1, AvailableGiB: 32, TotalGiB: 180},
		},
		{
			id: "555555555555", name: "legacy", total: 1500,
			services: []costexplorer.ServiceCost{
				{Service: "Amazon Elastic Compute Cloud - Compute", Amount: 600},
				{Service: "EC2 - Other", Amount: 500},
				{Service: "Amazon Simple Storage Service", Amount: 400},
			},
			volumes: &ec2volumes.Inventory{Count: 12, AvailableCount: 3, AvailableGiB: 96, TotalGiB: 240},
			snaps:   &ec2snapshots.SnapshotSummary{Count: 200, TotalSizeGiB: 3200},
		},
	}

	accounts := make([]service.AccountDashboard, 0, len(fixtures))
	var totals service.ConsolidatedTotals
	totals.Unit = "USD"
	topByService := make(map[string]float64)

	for _, f := range fixtures {
		ec2Other := 0.0
		for _, s := range f.services {
			if s.Service == "EC2 - Other" {
				ec2Other = s.Amount
			}
			topByService[s.Service] += s.Amount
		}
		accounts = append(accounts, service.AccountDashboard{
			AccountID:   f.id,
			AccountName: f.name,
			Costs: &costexplorer.CostSummary{
				AllTotal:           f.total,
				OtherServicesTotal: ec2Other,
				ByService:          f.services,
				Unit:               "USD",
				AllDaily:           dailyForAccount(f.id, f.total, start, end),
			},
			Volumes:   f.volumes,
			Snapshots: f.snaps,
		})
		totals.OrgTotal += f.total
		totals.EC2OtherCost += ec2Other
		if f.volumes != nil {
			totals.VolumeCount += f.volumes.Count
			totals.VolumeAvailable += f.volumes.AvailableCount
			totals.VolumeAvailableGiB += f.volumes.AvailableGiB
			totals.VolumeSizeGiB += f.volumes.TotalGiB
		}
		if f.snaps != nil {
			totals.SnapshotCount += f.snaps.Count
			totals.SnapshotSizeGiB += f.snaps.TotalSizeGiB
		}
	}
	totals.AccountCount = len(accounts)
	util := 42.5
	totals.VolumeUtilizationPercent = &util
	usedGiB := totals.VolumeSizeGiB * util / 100
	totals.VolumeFilesystemUsedGiB = &usedGiB
	if totals.VolumeSizeGiB > 0 {
		totals.VolumeUsageCoveragePct = 68
	}

	topServices := make([]service.OrgServiceDriver, 0, len(topByService))
	for svc, amt := range topByService {
		topServices = append(topServices, service.OrgServiceDriver{
			Service: svc,
			Amount:  amt,
			Unit:    "USD",
		})
	}
	// Sort by amount desc (simple bubble for small n)
	for i := 0; i < len(topServices); i++ {
		for j := i + 1; j < len(topServices); j++ {
			if topServices[j].Amount > topServices[i].Amount {
				topServices[i], topServices[j] = topServices[j], topServices[i]
			}
		}
	}

	return &service.DashboardResponse{
		Start:          start,
		End:            end,
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		Accounts:       accounts,
		Totals:         totals,
		TopServices:    topServices,
		CUREnabled:     false,
		CURNote:        "CUR/Athena disabled in demo mode",
		IncompleteDays: 2,
		TaxExcluded:    true,
		Tax:            demoTaxBreakdown(start, end, totals.OrgTotal),
		Commitments:    demoCommitments(),
	}
}

// demoTaxBreakdown models how AWS posts tax: a single lump on the 1st of the
// month, not spread daily. It posts the previous month's tax on the 1st of the
// range's month so the "faked day-1 spike" and the excluded-amount are visible.
// usageTotal is the org usage (tax-free) total; incl_tax_total_usd is usage+tax.
func demoTaxBreakdown(start, end string, usageTotal float64) *costexplorer.TaxBreakdown {
	s, err1 := time.Parse("2006-01-02", start)
	e, err2 := time.Parse("2006-01-02", end)
	if err1 != nil || err2 != nil {
		return &costexplorer.TaxBreakdown{HasData: false}
	}
	// Collect the 1st-of-month dates that fall inside [start, end].
	var firsts []time.Time
	d := s
	for !d.After(e) {
		if d.Day() == 1 {
			firsts = append(firsts, d)
		}
		d = d.AddDate(0, 0, 1)
	}
	if len(firsts) == 0 {
		// Range doesn't cross a month boundary; use the 1st of end's month.
		firsts = []time.Time{time.Date(e.Year(), e.Month(), 1, 0, 0, 0, 0, time.UTC)}
	}
	// The most recent 1st carries the largest (previous month's) lump.
	base := 4200.0
	const taxFmt = "2006-01-02"
	var daily []costexplorer.DailyCost
	var total float64
	for i, first := range firsts {
		// Earlier lumps are smaller (partial overlap with the range).
		amt := base * float64(i+1) / float64(len(firsts)+1)
		daily = append(daily, costexplorer.DailyCost{Date: first.Format(taxFmt), Amount: amt, Unit: "USD"})
		total += amt
	}
	sort.Slice(daily, func(i, j int) bool { return daily[i].Date < daily[j].Date })
	return &costexplorer.TaxBreakdown{
		TotalUSD:        total,
		InclTaxTotalUSD: usageTotal + total,
		ByService:       []costexplorer.ServiceCost{{Service: "Tax", Amount: total, Unit: "USD", Percent: 100}},
		Daily:           daily,
		PostedDays:      len(daily),
		HasData:         total > 0,
	}
}

// demoCommitments returns fixture commitment data with a low SP utilization
// (72%) so the low-commitment-coverage suggestion is visible in the demo.
func demoCommitments() *costexplorer.CommitmentCoverage {
	spCov := 55.0
	spUtil := 72.0
	riCov := 12.0
	uncommitted := 9400.0
	return &costexplorer.CommitmentCoverage{
		SPCoveragePct:  &spCov,
		SPUtilizationPct: &spUtil,
		RICoveragePct:  &riCov,
		UncommittedUSD: &uncommitted,
		HasCommitments: true,
	}
}

// dailyForAccount spreads an account's period total across the date range
// with a deterministic wiggle. The most recent full day (end - 3) gets a
// synthetic 3x spike so spike detection is visible in the demo.
func dailyForAccount(id string, total float64, start, end string) []costexplorer.DailyCost {
	s, err1 := time.Parse("2006-01-02", start)
	e, err2 := time.Parse("2006-01-02", end)
	if err1 != nil || err2 != nil {
		return nil
	}
	days := int(e.Sub(s).Hours()/24) + 1
	if days < 7 {
		daily := total / float64(days)
		out := make([]costexplorer.DailyCost, 0, days)
		for d := 0; d < days; d++ {
			out = append(out, costexplorer.DailyCost{
				Date:  s.AddDate(0, 0, d).Format("2006-01-02"),
				Amount: daily,
				Unit:  "USD",
			})
		}
		return out
	}
	base := total / float64(days)
	out := make([]costexplorer.DailyCost, 0, days)
	spikeIndex := days - 4 // 3 full days after the spike, 2 lag days before
	// Only the largest account drives the spike so org-level detection fires.
	isDriver := id == "111111111111"
	var sum float64
	raw := make([]float64, days)
	for d := 0; d < days; d++ {
		v := base * (1 + 0.08*math.Sin(float64(d)*0.9))
		if isDriver && d == spikeIndex {
			v = base * 3
		}
		raw[d] = v
		sum += v
	}
	for d := 0; d < days; d++ {
		amount := raw[d] / sum * total
		out = append(out, costexplorer.DailyCost{
			Date:   s.AddDate(0, 0, d).Format("2006-01-02"),
			Amount: math.Round(amount*100) / 100,
			Unit:   "USD",
		})
	}
	return out
}

// PriorSnapshot builds a history snapshot for trend comparison (~8% lower than current).
func PriorSnapshot(dash *service.DashboardResponse) history.Snapshot {
	services := make([]history.ServiceAmount, 0, len(dash.TopServices))
	var priorTotal float64
	for _, s := range dash.TopServices {
		prior := s.Amount * 0.92
		services = append(services, history.ServiceAmount{Service: s.Service, Amount: prior})
		priorTotal += prior
	}
	priorStart, priorEnd := priorPeriod(dash.Start, dash.End)
	return history.Snapshot{
		SavedAt:     time.Now().UTC().AddDate(0, 0, -35).Format(time.RFC3339),
		PeriodStart: priorStart,
		PeriodEnd:   priorEnd,
		OrgTotal:    dash.Totals.OrgTotal * 0.92,
		Services:    services,
	}
}

func priorPeriod(start, end string) (string, string) {
	return appconfig.PriorPeriodFor(appconfig.PeriodLookback, start, end)
}

// SeedHistory writes a prior-period snapshot so trends work without AWS.
func SeedHistory(store *history.Store, dash *service.DashboardResponse) error {
	if store == nil {
		return nil
	}
	snap := PriorSnapshot(dash)
	if err := store.SaveSnapshot(snap); err != nil {
		return fmt.Errorf("seed demo snapshot: %w", err)
	}
	return nil
}
