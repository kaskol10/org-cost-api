package analysis

import (
	"fmt"
	"math"
	"sort"
)

// Suggestion is an actionable cost optimization item for Hermes.
type Suggestion struct {
	ID                  string   `json:"id"`
	Priority            int      `json:"priority"` // 1 = highest
	Category            string   `json:"category"`
	Title               string   `json:"title"`
	Detail              string   `json:"detail"`
	EstimatedMonthlyUSD *float64 `json:"estimated_monthly_usd,omitempty"`
	Actions             []string `json:"actions"`
	Account             string   `json:"account,omitempty"`
	Service             string   `json:"service,omitempty"`
}

// SuggestionsResponse is ranked optimization guidance.
type SuggestionsResponse struct {
	GeneratedAt  string       `json:"generated_at"`
	Period       string       `json:"period"`
	Suggestions  []Suggestion `json:"suggestions"`
	Summary      string       `json:"summary"`
	CECallsUsed  int          `json:"ce_calls_used"`
	DataSources  []string     `json:"data_sources"`
}

// BuildSuggestions combines trends, dashboard waste signals, and spend patterns.
func BuildSuggestions(dash DashboardView, trends *TrendsResponse) *SuggestionsResponse {
	var items []Suggestion
	priority := 1

	// Unattached EBS — strong dollar estimate from provisioned GiB.
	if dash.Totals.VolumeAvailable > 0 {
		est := estimateUnattachedEBSMonthly(dash)
		items = append(items, Suggestion{
			ID:                  "unattached-ebs",
			Priority:            priority,
			Category:            "waste",
			Title:               fmt.Sprintf("Delete or attach %d unattached EBS disks", dash.Totals.VolumeAvailable),
			Detail:              "Available (unattached) volumes still bill for full provisioned capacity.",
			EstimatedMonthlyUSD: &est,
			Actions: []string{
				"Run get_waste_signals to see which accounts have the most unattached disks.",
				"Verify volumes are not needed, then delete or attach in the AWS console.",
			},
		})
		priority++
	}

	// Low EBS filesystem utilization.
	if dash.Totals.VolumeUtilizationPercent != nil && *dash.Totals.VolumeUtilizationPercent < 40 &&
		dash.Totals.VolumeFilesystemUsedGiB != nil {
		pct := *dash.Totals.VolumeUtilizationPercent
		items = append(items, Suggestion{
			ID:       "ebs-underutilized",
			Priority: priority,
			Category: "rightsizing",
			Title:    fmt.Sprintf("EBS filesystem only %.0f%% utilized (where measured)", pct),
			Detail: fmt.Sprintf(
				"%.0f GiB used of %.0f GiB measured provisioned. Downsize volumes or remove unused disks.",
				*dash.Totals.VolumeFilesystemUsedGiB, dash.Totals.VolumeSizeGiB,
			),
			Actions: []string{
				"Review get_account_costs for EC2-Other / EBS volume spend.",
				"Install CloudWatch agent on more instances for fuller coverage.",
			},
		})
		priority++
	}

	// Service spend spikes from trends.
	if trends != nil {
		for _, t := range trends.TopIncreases {
			if t.ChangePercent < 50 || t.CurrentUSD < 100 {
				continue
			}
			items = append(items, Suggestion{
				ID:       fmt.Sprintf("spike-%s", slug(t.Service)),
				Priority: priority,
				Category: "trend",
				Title: fmt.Sprintf(
					"%s spend up %.0f%% ($%.0f → $%.0f)",
					t.DisplayName, t.ChangePercent, t.PriorUSD, t.CurrentUSD,
				),
				Detail: fmt.Sprintf(
					"Prior period %s–%s vs current %s–%s. Investigate what changed.",
					trends.PriorPeriod.Start, trends.PriorPeriod.End,
					trends.CurrentPeriod.Start, trends.CurrentPeriod.End,
				),
				Service: t.Service,
				Actions: []string{
					fmt.Sprintf("Run get_service_detail for account + service %q.", t.DisplayName),
					"Check new clusters, larger instance types, or data growth.",
				},
			})
			priority++
			if priority > 8 {
				break
			}
		}

		// Org total spike.
		if trends.OrgTotal.ChangePercent > 25 && trends.OrgTotal.CurrentUSD > 500 {
			items = append(items, Suggestion{
				ID:       "org-spend-up",
				Priority: 2,
				Category: "trend",
				Title: fmt.Sprintf(
					"Organization spend up %.0f%% vs prior period",
					trends.OrgTotal.ChangePercent,
				),
				Detail: fmt.Sprintf(
					"$%.0f now vs $%.0f prior (source: %s).",
					trends.OrgTotal.CurrentUSD, trends.OrgTotal.PriorUSD, trends.PriorSource,
				),
				Actions: []string{
					"Run get_cost_trends for full service breakdown.",
					"Focus on top_increases before refresh=true (saves CE API calls).",
				},
			})
		}
	}

	// Spend concentration — top service dominates.
	if len(dash.TopServices) > 0 && dash.Totals.OrgTotal > 0 {
		top := dash.TopServices[0]
		share := (top.Amount / dash.Totals.OrgTotal) * 100
		if share > 30 {
			name := friendlyService(top.Service)
			items = append(items, Suggestion{
				ID:       "concentration-top-service",
				Priority: priority,
				Category: "concentration",
				Title:    fmt.Sprintf("%s is %.0f%% of org spend ($%.0f)", name, share, top.Amount),
				Detail:   "High concentration means small optimizations here have outsized impact.",
				Service:  top.Service,
				Actions: []string{
					fmt.Sprintf("Drill into %s with get_service_detail per top account.", name),
					"Review reserved capacity / savings plans for steady workloads.",
				},
			})
			priority++
		}
	}

	// Non-EC2-Other services per account outliers.
	for _, acct := range dash.Accounts {
		if acct.OtherServicesTotal < 500 {
			continue
		}
		amt := acct.OtherServicesTotal
		items = append(items, Suggestion{
			ID:       fmt.Sprintf("other-services-%s", acct.AccountName),
			Priority: priority,
			Category: "visibility",
			Title:    fmt.Sprintf("%s: $%.0f outside EC2-Other", acct.AccountName, amt),
			Detail:   "Most spend is S3, RDS, data transfer, etc. — review top services for this account.",
			Account:  acct.AccountName,
			Actions: []string{
				fmt.Sprintf("Run get_account_costs account=%q.", acct.AccountName),
			},
		})
		priority++
		if priority > 12 {
			break
		}
	}

	sort.Slice(items, func(i, j int) bool { return items[i].Priority < items[j].Priority })
	if len(items) > 15 {
		items = items[:15]
	}

	sources := []string{"dashboard_cache"}
	if trends != nil {
		sources = append(sources, "trends:"+trends.PriorSource)
	}

	summary := "No major issues detected — keep monitoring trends weekly."
	if len(items) > 0 {
		summary = fmt.Sprintf("%d optimization opportunities ranked by impact.", len(items))
	}

	return &SuggestionsResponse{
		GeneratedAt: dash.GeneratedAt,
		Period:      dash.Start + " to " + dash.End,
		Suggestions: items,
		Summary:     summary,
		DataSources: sources,
	}
}

func estimateUnattachedEBSMonthly(dash DashboardView) float64 {
	var gib float64
	for _, acct := range dash.Accounts {
		if acct.Volumes == nil || acct.Volumes.AvailableCount == 0 {
			continue
		}
		gib += acct.Volumes.AvailableGiB
	}
	return math.Round(gib*0.08*100) / 100
}

func slug(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			out = append(out, c)
		} else if c == ' ' || c == '-' {
			out = append(out, '-')
		}
	}
	if len(out) > 40 {
		out = out[:40]
	}
	return string(out)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
