package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kaskol10/org-cost-api/backend/internal/analysis"
	awsclient "github.com/kaskol10/org-cost-api/backend/internal/aws"
	"github.com/kaskol10/org-cost-api/backend/internal/aws/cur"
	"github.com/kaskol10/org-cost-api/backend/internal/aws/costexplorer"
	"github.com/kaskol10/org-cost-api/backend/internal/aws/ec2snapshots"
	"github.com/kaskol10/org-cost-api/backend/internal/aws/ec2volumes"
	appconfig "github.com/kaskol10/org-cost-api/backend/internal/config"
	"github.com/kaskol10/org-cost-api/backend/internal/errmsg"
	"github.com/kaskol10/org-cost-api/backend/internal/history"
	ceapi "github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"golang.org/x/sync/singleflight"
)

type Aggregator struct {
	cfg         *appconfig.Config
	clients     []*awsclient.AccountClients
	billingCost *ceapi.Client
	billingByProfile map[string]*ceapi.Client
	curClient   *cur.Client
	history     *history.Store

	cacheMu        sync.Mutex
	cachedDash     *DashboardResponse
	cachedDashAt   time.Time
	cachedDashKey  string
	cacheTTL       time.Duration
	dashSF         singleflight.Group

	// cachedServiceTagDelta stores tag-delta explanations for "why did service change" (cheap reuse).
	cachedServiceTagDelta   map[string]*analysis.ServiceTagDeltaResponse
	cachedServiceTagDeltaAt map[string]time.Time

	// accountProbe overrides LoadAccountClients for Ready() member-account STS (tests).
	accountProbe func(ctx context.Context, acct appconfig.Account) error

	demoMode bool

	// demoRefresh rebuilds fixture dashboard data for a date range (demo mode only).
	demoRefresh func(start, end string) *DashboardResponse
}

// SetDemoMode marks the aggregator as serving fixture data only.
func (a *Aggregator) SetDemoMode(v bool) {
	a.demoMode = v
}

// DemoMode reports whether this aggregator serves fixture data only.
func (a *Aggregator) DemoMode() bool {
	return a.demoMode
}

// SetDemoRefresh sets the fixture rebuild function for demo mode.
func (a *Aggregator) SetDemoRefresh(fn func(start, end string) *DashboardResponse) {
	a.demoRefresh = fn
}

// SetHistoryStore attaches a history store (used by demo mode).
func (a *Aggregator) SetHistoryStore(store *history.Store) {
	a.history = store
}

func NewAggregator(cfg *appconfig.Config) (*Aggregator, error) {
	ctx := context.Background()
	clients := make([]*awsclient.AccountClients, 0, len(cfg.Accounts))
	for _, acct := range cfg.Accounts {
		c, err := awsclient.LoadAccountClients(ctx, acct)
		if err != nil {
			return nil, err
		}
		clients = append(clients, c)
	}

	agg := &Aggregator{cfg: cfg, clients: clients, billingByProfile: make(map[string]*ceapi.Client)}
	agg.cacheTTL = 20 * time.Minute
	agg.cachedServiceTagDelta = make(map[string]*analysis.ServiceTagDeltaResponse)
	agg.cachedServiceTagDeltaAt = make(map[string]time.Time)

	profiles := make(map[string]struct{})
	if cfg.BillingProfile != "" {
		profiles[cfg.BillingProfile] = struct{}{}
	}
	for _, acct := range cfg.Accounts {
		bp := acct.BillingProfile
		if bp != "" && bp != "account" {
			profiles[bp] = struct{}{}
		}
	}
	for profile := range profiles {
		billing, err := awsclient.LoadBillingCostClient(ctx, profile)
		if err != nil {
			return nil, fmt.Errorf("billing profile %q: %w", profile, err)
		}
		agg.billingByProfile[profile] = billing
		if cfg.BillingProfile == profile {
			agg.billingCost = billing
		}
	}
	if agg.billingCost == nil && cfg.HasBillingStaticCredentials() {
		static, err := cfg.ResolveBillingStaticCredentials()
		if err != nil {
			return nil, fmt.Errorf("billing credentials: %w", err)
		}
		billing, err := awsclient.LoadBillingCostClientWithCredentials(ctx, static)
		if err != nil {
			return nil, fmt.Errorf("billing credentials: %w", err)
		}
		agg.billingCost = billing
	}
	if agg.billingCost == nil && cfg.BillingRoleARN != "" {
		billing, err := awsclient.LoadBillingCostClientWithRole(ctx, cfg.BillingRoleARN)
		if err != nil {
			return nil, fmt.Errorf("billing role %q: %w", cfg.BillingRoleARN, err)
		}
		agg.billingCost = billing
	}
	if agg.billingCost == nil && cfg.BillingProfile == "" && !cfg.HasBillingStaticCredentials() {
		// IRSA / default chain as payer CE when no explicit billing identity is set.
		billing, err := awsclient.LoadBillingCostClient(ctx, "")
		if err == nil {
			agg.billingCost = billing
		}
	}
	if cfg.CUR != nil && cfg.CUR.Enabled {
		profile := cfg.CUR.Profile
		if profile == "" {
			profile = cfg.BillingProfile
		}
		curClient, err := cur.NewClient(ctx, *cfg.CUR, profile)
		if err != nil {
			return nil, fmt.Errorf("cur/athena: %w", err)
		}
		agg.curClient = curClient
	}
	hist, err := history.NewStore(cfg.HistoryDir)
	if err != nil {
		return nil, fmt.Errorf("history store: %w", err)
	}
	agg.history = hist
	return agg, nil
}

type DashboardResponse struct {
	GeneratedAt  string              `json:"generated_at"`
	Start        string              `json:"start"`
	End          string              `json:"end"`
	Accounts     []AccountDashboard  `json:"accounts"`
	Totals       ConsolidatedTotals  `json:"totals"`
	TopServices  []OrgServiceDriver  `json:"top_services,omitempty"`
	CUREnabled   bool                `json:"cur_enabled"`
	CURNote      string              `json:"cur_note,omitempty"`
}

type AccountDashboard struct {
	AccountID   string                           `json:"account_id"`
	AccountName string                           `json:"account_name"`
	Error       string                           `json:"error,omitempty"`
	Costs       *costexplorer.CostSummary        `json:"costs,omitempty"`
	Volumes     *ec2volumes.Inventory            `json:"volumes,omitempty"`
	Snapshots   *ec2snapshots.SnapshotSummary    `json:"snapshots,omitempty"`
}

type ConsolidatedTotals struct {
	OrgTotal          float64 `json:"org_total"`
	EC2OtherCost      float64 `json:"ec2_other_cost"`
	Unit              string  `json:"unit"`
	VolumeCount              int      `json:"volume_count"`
	VolumeAvailable          int      `json:"volume_available_count"`
	VolumeAvailableGiB       float64  `json:"volume_available_gib"`
	VolumeSizeGiB            float64  `json:"volume_size_gib"`
	VolumeFilesystemUsedGiB  *float64 `json:"volume_filesystem_used_gib,omitempty"`
	VolumeUtilizationPercent *float64 `json:"volume_utilization_percent,omitempty"`
	VolumeUsageCoveragePct   float64  `json:"volume_usage_coverage_percent"`
	SnapshotCount     int     `json:"snapshot_count"`
	SnapshotSizeGiB   float64 `json:"snapshot_size_gib"`
	AccountCount      int     `json:"account_count"`
}

type ServiceDetailRequest struct {
	AccountID string `json:"account_id"`
	Service   string `json:"service"`
	Start     string `json:"start"`
	End       string `json:"end"`
}

func (a *Aggregator) ServiceDetail(ctx context.Context, accountID, service, start, end string) (*costexplorer.ServiceDetail, error) {
	if start == "" || end == "" {
		start, end = a.cfg.CostDateRange()
	}
	var client *awsclient.AccountClients
	for _, c := range a.clients {
		if c.AccountID == accountID {
			client = c
			break
		}
	}
	if client == nil {
		return nil, NewUnknownAccountError(accountID)
	}
	costCE, fromPayer := a.resolveCostClient(client)
	if costCE == nil {
		return nil, fmt.Errorf("no cost explorer client available for account %s", accountID)
	}
	return costexplorer.GetServiceDetail(ctx, costCE, accountID, service, start, end, fromPayer)
}

func (a *Aggregator) resolveCostClient(client *awsclient.AccountClients) (*ceapi.Client, bool) {
	bp := client.Account.BillingProfile
	switch bp {
	case "account":
		return client.Cost, false
	case "":
		if a.billingCost != nil {
			return a.billingCost, true
		}
		return client.Cost, false
	default:
		if c, ok := a.billingByProfile[bp]; ok {
			return c, true
		}
		return client.Cost, false
	}
}

func (a *Aggregator) Dashboard(ctx context.Context) (*DashboardResponse, error) {
	return a.dashboard(ctx, false, appconfig.PeriodLookback)
}

func (a *Aggregator) DashboardFresh(ctx context.Context) (*DashboardResponse, error) {
	return a.dashboard(ctx, true, appconfig.PeriodLookback)
}

// DashboardForPeriod returns the org dashboard for a period mode (30d or mtd).
func (a *Aggregator) DashboardForPeriod(ctx context.Context, force bool, period string) (*DashboardResponse, error) {
	mode, err := parsePeriodMode(period)
	if err != nil {
		return nil, err
	}
	return a.dashboard(ctx, force, mode)
}

func parsePeriodMode(period string) (appconfig.PeriodMode, error) {
	mode, err := appconfig.ParsePeriod(period)
	if err != nil {
		return "", NewInvalidRequestError(err.Error())
	}
	return mode, nil
}

func (a *Aggregator) dashboard(ctx context.Context, force bool, mode appconfig.PeriodMode) (*DashboardResponse, error) {
	if a.demoMode && a.demoRefresh != nil {
		return a.demoDashboard(ctx, force, mode)
	}
	start, end := a.cfg.CostDateRangeFor(mode)
	key := string(mode) + ":" + start + ":" + end

	a.cacheMu.Lock()
	if !force && a.cachedDash != nil && a.cachedDashKey == key && time.Since(a.cachedDashAt) < a.cacheTTL {
		out := *a.cachedDash
		a.cacheMu.Unlock()
		return &out, nil
	}
	if force {
		// Force refresh invalidates tag-delta explanations built against the prior dash.
		a.cachedServiceTagDelta = make(map[string]*analysis.ServiceTagDeltaResponse)
		a.cachedServiceTagDeltaAt = make(map[string]time.Time)
	}
	a.cacheMu.Unlock()

	sfKey := key
	if force {
		sfKey = key + ":refresh"
	}
	v, err, _ := a.dashSF.Do(sfKey, func() (interface{}, error) {
		// Re-check cache inside singleflight so waiters share a warm result.
		a.cacheMu.Lock()
		if !force && a.cachedDash != nil && a.cachedDashKey == key && time.Since(a.cachedDashAt) < a.cacheTTL {
			cached := a.cachedDash
			a.cacheMu.Unlock()
			return cached, nil
		}
		a.cacheMu.Unlock()
		// Detach from the leader's cancel so a disconnected caller does not poison waiters.
		return a.buildDashboard(context.WithoutCancel(ctx), start, end, key)
	})
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Shallow copy so handlers can stamp GeneratedAt without racing other callers.
	resp := v.(*DashboardResponse)
	out := *resp
	return &out, nil
}

func (a *Aggregator) demoDashboard(ctx context.Context, force bool, mode appconfig.PeriodMode) (*DashboardResponse, error) {
	start, end := a.cfg.CostDateRangeFor(mode)
	key := string(mode) + ":" + start + ":" + end

	a.cacheMu.Lock()
	if !force && a.cachedDash != nil && a.cachedDashKey == key && time.Since(a.cachedDashAt) < a.cacheTTL {
		out := *a.cachedDash
		a.cacheMu.Unlock()
		return &out, nil
	}
	a.cacheMu.Unlock()

	if force {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}

	dash := a.demoRefresh(start, end)
	if dash == nil {
		return nil, fmt.Errorf("demo fixture unavailable")
	}
	dash.GeneratedAt = time.Now().UTC().Format(time.RFC3339)

	a.cacheMu.Lock()
	a.cachedDash = dash
	a.cachedDashAt = time.Now()
	a.cachedDashKey = key
	a.cacheMu.Unlock()

	out := *dash
	return &out, nil
}

func (a *Aggregator) buildDashboard(ctx context.Context, start, end, key string) (*DashboardResponse, error) {
	var curInv *cur.AccountDailyInventory
	var curNote string
	if a.curClient != nil {
		ids := make([]string, len(a.clients))
		for i, c := range a.clients {
			ids[i] = c.AccountID
		}
		inv, err := a.curClient.FetchEBSInventory(ctx, ids, start, end)
		if err != nil {
			curNote = errmsg.CURNote(err)
		} else {
			curInv = inv
		}
	}

	type result struct {
		idx int
		ad  AccountDashboard
		err error
	}

	ch := make(chan result, len(a.clients))
	var wg sync.WaitGroup

	for i, c := range a.clients {
		wg.Add(1)
		go func(idx int, client *awsclient.AccountClients) {
			defer wg.Done()
			ad := AccountDashboard{
				AccountID:   client.AccountID,
				AccountName: client.Account.Name,
			}

			costCE, fromPayer := a.resolveCostClient(client)
			region := client.Account.Region
			if region == "" {
				region = "eu-west-1"
			}
			vols, volErr := ec2volumes.ListInventory(ctx, client.EC2, client.CloudWatch)
			if volErr != nil {
				ad.Error = errmsg.AccountComponent("volumes", volErr)
				ch <- result{idx: idx, ad: ad}
				return
			}
			ad.Volumes = vols

			snaps, err := ec2snapshots.GetSnapshotSummary(ctx, client.EC2, client.AccountID, client.Account.Name, region, 10)
			if err != nil {
				ad.Error = errmsg.AccountComponent("snapshots", err)
				ch <- result{idx: idx, ad: ad}
				return
			}
			ad.Snapshots = snaps

			storage := costexplorer.NewStorageContext(vols, snaps, curInv, client.AccountID)
			costs, err := costexplorer.GetCostSummary(ctx, costCE, client.EC2, client.AccountID, client.Account.Name, region, start, end, fromPayer, storage)
			if err != nil {
				ad.Error = errmsg.AccountComponent("costs", err)
				ch <- result{idx: idx, ad: ad}
				return
			}
			ad.Costs = costs
			ch <- result{idx: idx, ad: ad}
		}(i, c)
	}

	wg.Wait()
	close(ch)

	accounts := make([]AccountDashboard, len(a.clients))
	var totals ConsolidatedTotals
	totals.Unit = "USD"
	var measuredProvGiB, measuredUsedGiB float64
	successCount := 0

	for r := range ch {
		accounts[r.idx] = r.ad
		if r.ad.Error != "" {
			continue
		}
		successCount++
		if r.ad.Costs != nil {
			totals.OrgTotal += r.ad.Costs.AllTotal
			totals.EC2OtherCost += r.ad.Costs.Total
			if totals.Unit == "" {
				totals.Unit = r.ad.Costs.Unit
			}
		}
		if r.ad.Volumes != nil {
			totals.VolumeCount += r.ad.Volumes.Count
			totals.VolumeAvailable += r.ad.Volumes.AvailableCount
			totals.VolumeAvailableGiB += r.ad.Volumes.AvailableGiB
			totals.VolumeSizeGiB += r.ad.Volumes.TotalGiB
			if u := r.ad.Volumes.Usage; u != nil {
				measuredProvGiB += u.MeasuredProvisionedGiB
				if u.FilesystemUsedGiB != nil {
					measuredUsedGiB += *u.FilesystemUsedGiB
				}
			}
		}
		if r.ad.Snapshots != nil {
			totals.SnapshotCount += r.ad.Snapshots.Count
			totals.SnapshotSizeGiB += r.ad.Snapshots.TotalSizeGiB
		}
	}

	if totals.VolumeSizeGiB > 0 {
		totals.VolumeUsageCoveragePct = measuredProvGiB / totals.VolumeSizeGiB * 100
	}
	if measuredProvGiB > 0 && measuredUsedGiB > 0 {
		totals.VolumeFilesystemUsedGiB = &measuredUsedGiB
		util := measuredUsedGiB / measuredProvGiB * 100
		totals.VolumeUtilizationPercent = &util
	}
	totals.AccountCount = successCount

	resp := &DashboardResponse{
		Start:       start,
		End:         end,
		Accounts:    accounts,
		Totals:      totals,
		TopServices: buildOrgTopServices(accounts, totals.OrgTotal, totals.Unit),
		CUREnabled:  a.curClient != nil && curInv != nil,
		CURNote:     curNote,
	}

	a.cacheMu.Lock()
	a.cachedDash = resp
	a.cachedDashAt = time.Now()
	a.cachedDashKey = key
	a.cacheMu.Unlock()

	a.recordSnapshot(resp)

	return resp, nil
}

func (a *Aggregator) Accounts() []map[string]string {
	out := make([]map[string]string, 0, len(a.clients))
	for _, c := range a.clients {
		out = append(out, map[string]string{
			"id":   c.AccountID,
			"name": c.Account.Name,
		})
	}
	return out
}
