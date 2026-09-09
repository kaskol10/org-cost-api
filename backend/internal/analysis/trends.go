package analysis

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/kaskol10/org-cost-api/backend/internal/history"
)

const minTrendUSD = 25.0

// ServiceTrend is period-over-period change for one AWS service.
type ServiceTrend struct {
	Service         string  `json:"service"`
	DisplayName     string  `json:"display_name"`
	CurrentUSD      float64 `json:"current_usd"`
	PriorUSD        float64 `json:"prior_usd"`
	ChangeUSD       float64 `json:"change_usd"`
	ChangePercent   float64 `json:"change_percent"`
	Direction       string  `json:"direction"` // up, down, new, stable
	CurrentSharePct float64 `json:"current_share_pct,omitempty"`
}

// AccountTrend is period-over-period change for one linked account.
type AccountTrend struct {
	AccountID       string  `json:"account_id"`
	AccountName     string  `json:"account_name"`
	CurrentUSD      float64 `json:"current_usd"`
	PriorUSD        float64 `json:"prior_usd"`
	ChangeUSD       float64 `json:"change_usd"`
	ChangePercent   float64 `json:"change_percent"`
	Direction       string  `json:"direction"` // up, down, new, stable
	CurrentSharePct float64 `json:"current_share_pct,omitempty"`
}

// PeriodSummary describes a comparison window.
type PeriodSummary struct {
	Start string `json:"start"`
	End   string `json:"end"`
	Days  int    `json:"days,omitempty"`
}

// TrendsResponse compares current vs prior period using dashboard + history.
type TrendsResponse struct {
	GeneratedAt         string         `json:"generated_at"`
	Period              string         `json:"period,omitempty"` // 30d | mtd
	CurrentPeriod       PeriodSummary  `json:"current_period"`
	PriorPeriod         PeriodSummary  `json:"prior_period"`
	PriorSource         string         `json:"prior_source"`
	CECallsUsed         int            `json:"ce_calls_used"`
	OrgTotal            ChangeSummary  `json:"org_total"`
	ServiceTrends       []ServiceTrend `json:"service_trends"`
	TopIncreases        []ServiceTrend `json:"top_increases"`
	TopDecreases        []ServiceTrend `json:"top_decreases"`
	AccountTrends       []AccountTrend `json:"account_trends"`
	TopAccountIncreases []AccountTrend `json:"top_account_increases"`
	TopAccountDecreases []AccountTrend `json:"top_account_decreases"`
	HistoryNote         string         `json:"history_note,omitempty"`
	SnapshotCount       int            `json:"snapshot_count,omitempty"`
	RefreshAllowed      bool           `json:"refresh_allowed,omitempty"`
}

type ChangeSummary struct {
	CurrentUSD    float64 `json:"current_usd"`
	PriorUSD      float64 `json:"prior_usd"`
	ChangeUSD     float64 `json:"change_usd"`
	ChangePercent float64 `json:"change_percent"`
}

// BuildTrends compares current dashboard services/accounts to prior period maps.
func BuildTrends(
	dash DashboardView,
	priorServices map[string]float64,
	priorAccounts map[string]float64,
	priorOrgTotal float64,
	priorSource string,
	ceCalls int,
	historyNote string,
	snapshotCount int,
) *TrendsResponse {
	if priorServices == nil {
		priorServices = map[string]float64{}
	}
	if priorAccounts == nil {
		priorAccounts = map[string]float64{}
	}

	current := serviceMapFromDashboard(dash)
	currentAccounts := accountMapFromDashboard(dash)
	currentTotal := dash.Totals.OrgTotal

	resp := &TrendsResponse{
		GeneratedAt:   dash.GeneratedAt,
		CurrentPeriod: PeriodSummary{Start: dash.Start, End: dash.End, Days: periodDays(dash.Start, dash.End)},
		PriorSource:   priorSource,
		CECallsUsed:   ceCalls,
		HistoryNote:   historyNote,
		SnapshotCount: snapshotCount,
		OrgTotal: ChangeSummary{
			CurrentUSD: currentTotal,
			PriorUSD:   priorOrgTotal,
		},
	}
	resp.OrgTotal.ChangeUSD = currentTotal - priorOrgTotal
	if priorOrgTotal > 0 {
		resp.OrgTotal.ChangePercent = (resp.OrgTotal.ChangeUSD / priorOrgTotal) * 100
	}

	allServices := make(map[string]struct{})
	for svc := range current {
		allServices[svc] = struct{}{}
	}
	for svc := range priorServices {
		allServices[svc] = struct{}{}
	}

	var trends []ServiceTrend
	for svc := range allServices {
		cur := current[svc]
		prior := priorServices[svc]
		if cur < 0.01 && prior < 0.01 {
			continue
		}
		t := ServiceTrend{
			Service:     svc,
			DisplayName: friendlyService(svc),
			CurrentUSD:  cur,
			PriorUSD:    prior,
			ChangeUSD:   cur - prior,
		}
		if currentTotal > 0 {
			t.CurrentSharePct = (cur / currentTotal) * 100
		}
		t.ChangePercent, t.Direction = trendDirection(cur, prior, t.ChangeUSD)
		trends = append(trends, t)
	}

	sort.Slice(trends, func(i, j int) bool {
		return math.Abs(trends[i].ChangeUSD) > math.Abs(trends[j].ChangeUSD)
	})
	resp.ServiceTrends = trends
	resp.TopIncreases = topByDirection(trends, "up", 10)
	resp.TopDecreases = topByDirection(trends, "down", 10)

	accountNames := accountNamesFromDashboard(dash)
	allAccounts := make(map[string]struct{})
	for id := range currentAccounts {
		allAccounts[id] = struct{}{}
	}
	for id := range priorAccounts {
		allAccounts[id] = struct{}{}
	}

	var accountTrends []AccountTrend
	for id := range allAccounts {
		cur := currentAccounts[id]
		prior := priorAccounts[id]
		if cur < 0.01 && prior < 0.01 {
			continue
		}
		name := accountNames[id]
		if name == "" {
			name = id
		}
		t := AccountTrend{
			AccountID:   id,
			AccountName: name,
			CurrentUSD:  cur,
			PriorUSD:    prior,
			ChangeUSD:   cur - prior,
		}
		if currentTotal > 0 {
			t.CurrentSharePct = (cur / currentTotal) * 100
		}
		t.ChangePercent, t.Direction = trendDirection(cur, prior, t.ChangeUSD)
		accountTrends = append(accountTrends, t)
	}

	sort.Slice(accountTrends, func(i, j int) bool {
		return math.Abs(accountTrends[i].ChangeUSD) > math.Abs(accountTrends[j].ChangeUSD)
	})
	resp.AccountTrends = accountTrends
	resp.TopAccountIncreases = topAccountByDirection(accountTrends, "up", 5)
	resp.TopAccountDecreases = topAccountByDirection(accountTrends, "down", 5)
	return resp
}

func trendDirection(cur, prior, changeUSD float64) (changePercent float64, direction string) {
	switch {
	case prior < 0.01 && cur >= minTrendUSD:
		return 100, "new"
	case prior >= minTrendUSD:
		changePercent = ((cur - prior) / prior) * 100
		if math.Abs(changePercent) < 5 {
			return changePercent, "stable"
		} else if changeUSD > 0 {
			return changePercent, "up"
		}
		return changePercent, "down"
	default:
		return 0, "stable"
	}
}

func serviceMapFromDashboard(dash DashboardView) map[string]float64 {
	out := make(map[string]float64)
	for _, s := range dash.TopServices {
		out[s.Service] = s.Amount
	}
	agg := make(map[string]float64)
	for _, acct := range dash.Accounts {
		for _, s := range acct.ByService {
			if s.Amount > 0 {
				agg[s.Service] += s.Amount
			}
		}
	}
	for svc, amt := range agg {
		out[svc] = amt
	}
	return out
}

func accountMapFromDashboard(dash DashboardView) map[string]float64 {
	out := make(map[string]float64, len(dash.Accounts))
	for _, acct := range dash.Accounts {
		if acct.AccountID == "" {
			continue
		}
		out[acct.AccountID] = acct.AllTotal
	}
	return out
}

func accountNamesFromDashboard(dash DashboardView) map[string]string {
	out := make(map[string]string, len(dash.Accounts))
	for _, acct := range dash.Accounts {
		if acct.AccountID == "" {
			continue
		}
		name := strings.TrimSpace(acct.AccountName)
		if name == "" {
			name = acct.AccountID
		}
		out[acct.AccountID] = name
	}
	return out
}

func PriorMapFromSnapshot(snap *history.Snapshot) (map[string]float64, float64) {
	out := make(map[string]float64, len(snap.Services))
	for _, s := range snap.Services {
		out[s.Service] = s.Amount
	}
	return out, snap.OrgTotal
}

func PriorAccountMapFromSnapshot(snap *history.Snapshot) map[string]float64 {
	out := make(map[string]float64, len(snap.Accounts))
	for _, a := range snap.Accounts {
		out[a.AccountID] = a.AllTotal
	}
	return out
}

func PriorMapFromCache(c *history.PriorCache) (map[string]float64, float64) {
	out := make(map[string]float64, len(c.Services))
	for _, s := range c.Services {
		out[s.Service] = s.Amount
	}
	return out, c.OrgTotal
}

func PriorAccountMapFromCache(c *history.PriorCache) map[string]float64 {
	out := make(map[string]float64, len(c.Accounts))
	for _, a := range c.Accounts {
		out[a.AccountID] = a.AllTotal
	}
	return out
}

func topByDirection(trends []ServiceTrend, dir string, limit int) []ServiceTrend {
	var out []ServiceTrend
	for _, t := range trends {
		if t.Direction != dir {
			continue
		}
		if dir == "up" && t.ChangeUSD < minTrendUSD {
			continue
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		if dir == "up" {
			return out[i].ChangePercent > out[j].ChangePercent
		}
		return out[i].ChangePercent < out[j].ChangePercent
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func topAccountByDirection(trends []AccountTrend, dir string, limit int) []AccountTrend {
	var out []AccountTrend
	for _, t := range trends {
		if t.Direction != dir {
			continue
		}
		if dir == "up" && t.ChangeUSD < minTrendUSD {
			continue
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		if dir == "up" {
			return out[i].ChangePercent > out[j].ChangePercent
		}
		return out[i].ChangePercent < out[j].ChangePercent
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func friendlyService(service string) string {
	s := strings.TrimSpace(service)
	switch {
	case strings.Contains(s, "Redshift"):
		return "Redshift"
	case strings.Contains(s, "Simple Storage Service"):
		return "S3"
	case strings.Contains(s, "Relational Database Service"):
		return "RDS"
	case strings.Contains(s, "Elastic Compute Cloud - Compute"):
		return "EC2 compute"
	case s == "EC2 - Other":
		return "EC2-Other"
	case strings.Contains(s, "Data Transfer"):
		return "Data transfer"
	case strings.Contains(s, "Kinesis"):
		return "Kinesis"
	default:
		return s
	}
}

func periodDays(start, end string) int {
	s, err1 := time.Parse("2006-01-02", start)
	e, err2 := time.Parse("2006-01-02", end)
	if err1 != nil || err2 != nil {
		return 0
	}
	days := int(e.Sub(s).Hours() / 24)
	if days < 1 {
		return 1
	}
	return days
}
