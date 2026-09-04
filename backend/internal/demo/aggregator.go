package demo

import (
	"fmt"
	"os"
	"path/filepath"

	appconfig "github.com/kaskol10/org-cost-api/backend/internal/config"
	"github.com/kaskol10/org-cost-api/backend/internal/history"
	"github.com/kaskol10/org-cost-api/backend/internal/service"
)

// NewAggregator builds a demo aggregator with fixture data and no AWS calls.
func NewAggregator(cfg *appconfig.Config) (*service.Aggregator, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	cfg.Demo = true

	dash := Dashboard(cfg)
	agg := service.NewTestAggregator(cfg, dash)
	agg.SetDemoMode(true)
	agg.SetDemoRefresh(func() *service.DashboardResponse {
		return Dashboard(cfg)
	})

	histDir := cfg.HistoryDir
	if histDir == "" {
		histDir = filepath.Join(os.TempDir(), "org-cost-demo-history")
	}
	store, err := history.NewStore(histDir)
	if err != nil {
		return nil, fmt.Errorf("demo history: %w", err)
	}
	if err := SeedHistory(store, dash); err != nil {
		return nil, err
	}
	agg.SetHistoryStore(store)
	return agg, nil
}
