package service

import (
	"context"
	"time"

	awsclient "github.com/kaskol10/org-cost-api/backend/internal/aws"
	"github.com/kaskol10/org-cost-api/backend/internal/analysis"
	"github.com/kaskol10/org-cost-api/backend/internal/aws/costexplorer"
	appconfig "github.com/kaskol10/org-cost-api/backend/internal/config"
	ceapi "github.com/aws/aws-sdk-go-v2/service/costexplorer"
)

// NewTestAggregator builds an Aggregator with a warm dashboard cache for httptest.
// No AWS calls are made while the cache matches the current config date range.
func NewTestAggregator(cfg *appconfig.Config, dash *DashboardResponse) *Aggregator {
	clients := make([]*awsclient.AccountClients, 0, len(cfg.Accounts))
	for _, acct := range cfg.Accounts {
		clients = append(clients, &awsclient.AccountClients{
			Account:   acct,
			AccountID: acct.ID,
		})
	}

	start, end := cfg.CostDateRange()
	if dash != nil {
		if dash.Start == "" {
			dash.Start = start
		}
		if dash.End == "" {
			dash.End = end
		}
	}

	agg := &Aggregator{
		cfg:                      cfg,
		clients:                  clients,
		billingByProfile:         make(map[string]*ceapi.Client),
		cacheTTL:                 20 * time.Minute,
		cachedServiceTagDelta:    make(map[string]*analysis.ServiceTagDeltaResponse),
		cachedServiceTagDeltaAt:  make(map[string]time.Time),
		// Avoid live AWS STS in Ready() for httptest / unit fixtures.
		accountProbe:             func(context.Context, appconfig.Account) error { return nil },
	}
	if dash != nil {
		agg.cachedDash = dash
		agg.cachedDashAt = time.Now()
		agg.cachedDashKey = string(appconfig.PeriodLookback) + ":" + dash.Start + ":" + dash.End
	}
	return agg
}

// TestDashboard builds a minimal dashboard response for integration tests.
func TestDashboard(cfg *appconfig.Config) *DashboardResponse {
	start, end := cfg.CostDateRange()
	accounts := make([]AccountDashboard, 0, len(cfg.Accounts))
	for _, acct := range cfg.Accounts {
		accounts = append(accounts, AccountDashboard{
			AccountID:   acct.ID,
			AccountName: acct.Name,
			Costs: &costexplorer.CostSummary{
				AllTotal:           1000,
				OtherServicesTotal: 800,
				ByService: []costexplorer.ServiceCost{
					{Service: "Amazon Simple Storage Service", Amount: 500},
				},
			},
		})
	}
	return &DashboardResponse{
		Start:       start,
		End:         end,
		Accounts:    accounts,
		Totals: ConsolidatedTotals{
			OrgTotal:     float64(len(accounts)) * 1000,
			AccountCount: len(accounts),
			Unit:         "USD",
		},
		TopServices: []OrgServiceDriver{
			{Service: "Amazon Simple Storage Service", Amount: 500, Unit: "USD"},
		},
	}
}
