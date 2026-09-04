package demo

import (
	"fmt"
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
		Accounts: []appconfig.Account{
			{ID: "111111111111", Name: "production"},
			{ID: "222222222222", Name: "staging"},
			{ID: "333333333333", Name: "analytics"},
			{ID: "444444444444", Name: "sandbox"},
			{ID: "555555555555", Name: "legacy"},
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
		Start:       start,
		End:         end,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Accounts:    accounts,
		Totals:      totals,
		TopServices: topServices,
		CUREnabled:  false,
		CURNote:     "CUR/Athena disabled in demo mode",
	}
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
	startT, err := time.Parse("2006-01-02", start)
	if err != nil {
		return start, end
	}
	endT, err := time.Parse("2006-01-02", end)
	if err != nil {
		return start, end
	}
	days := int(endT.Sub(startT).Hours()/24) + 1
	priorEnd := startT.AddDate(0, 0, -1)
	priorStart := priorEnd.AddDate(0, 0, -(days - 1))
	return priorStart.Format("2006-01-02"), priorEnd.Format("2006-01-02")
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
