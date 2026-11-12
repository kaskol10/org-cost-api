package analysis

import (
	"strings"
	"testing"
)

func TestFormatUSD(t *testing.T) {
	cases := map[float64]string{0: "$0.00", 8: "$8.00", 1234.5: "$1,234.50", 184220: "$184,220.00", -1600: "-$1,600.00", 1234567.891: "$1,234,567.89"}
	for v, want := range cases {
		if got := formatUSD(v); got != want {
			t.Errorf("formatUSD(%v) = %q, want %q", v, got, want)
		}
	}
	if got := formatSignedUSD(12410); got != "+$12,410.00" {
		t.Errorf("formatSignedUSD = %q", got)
	}
	if got := formatSignedPercent(-7.4); got != "-7%" {
		t.Errorf("formatSignedPercent = %q", got)
	}
}

func sampleInput() HighlightInput {
	est := 41.26
	return HighlightInput{
		Period:                "30d",
		OrgTotal:              184220,
		AccountCount:          5,
		TopService:            &ServiceDriverView{Service: "Amazon Elastic Compute Cloud - Compute", Amount: 92110},
		VolumeAvailableCount:  11,
		VolumeAvailableGiB:    458,
		VolumeWasteMonthlyUSD: est,
		Trends: &TrendsResponse{
			CurrentPeriod: PeriodSummary{Days: 30},
			OrgTotal:      ChangeSummary{CurrentUSD: 184220, PriorUSD: 171810, ChangeUSD: 12410, ChangePercent: 7.2},
			TopIncreases:  []ServiceTrend{{Service: "NAT", DisplayName: "NAT Gateway", CurrentUSD: 20000, PriorUSD: 12100, ChangeUSD: 7900, ChangePercent: 65}},
		},
		Suggestions: &SuggestionsResponse{Suggestions: []Suggestion{{ID: "ebs", Title: "Delete unattached EBS", Category: "waste", EstimatedMonthlyUSD: &est}}},
	}
}

func TestBuildHighlights(t *testing.T) {
	got := BuildHighlights(sampleInput())
	ids := make([]string, 0, len(got))
	for _, h := range got {
		ids = append(ids, h.ID)
	}
	want := "org-spend,org-trend,top-service,ebs-waste,spike-NAT,saving-ebs"
	if strings.Join(ids, ",") != want {
		t.Fatalf("ids = %v, want %s", ids, want)
	}
	if got[0].Value != "$184,220.00" || got[0].Detail != "Last 30 days · 5 accounts · usage (excl. credits)" {
		t.Errorf("org-spend = %+v", got[0])
	}
	if got[1].Title != "Higher spend" || got[1].Value != "+$12,410.00 · +7%" {
		t.Errorf("org-trend = %+v", got[1])
	}
	if got[2].Value != "EC2" || got[2].Detail != "$92,110.00 · 50% of org" {
		t.Errorf("top-service = %+v", got[2])
	}
	if got[3].Detail != "458 GiB · ~$41.26/mo potential" {
		t.Errorf("ebs-waste uses the per-type estimate, got %+v", got[3])
	}
	if got[4].Detail != "drove +$7,900.00 (+65%)" {
		t.Errorf("spike = %+v", got[4])
	}
}

func TestBuildHighlights_BudgetFirstAndCapped(t *testing.T) {
	in := sampleInput()
	in.Forecast = &ForecastResult{ProjectedUSD: 210000}
	in.Budgets = []*BudgetStatus{{Name: "team-a", Account: "111", Status: "ok"}, {Name: "org", MonthlyUSD: 200000, PercentOfBudget: 105, Status: "over", OverByUSD: 10000}}
	got := BuildHighlights(in)
	if len(got) != MaxHighlights {
		t.Fatalf("expected cap of %d, got %d", MaxHighlights, len(got))
	}
	if got[0].Kind != "budget" || got[0].Title != "Over budget" || got[0].Value != "105% of org" {
		t.Fatalf("budget card = %+v", got[0])
	}
}

func TestBuildHeadline(t *testing.T) {
	got := BuildHeadline(sampleInput())
	want := "Last 30 days: $184,220.00 across 5 accounts, +$12,410.00 (+7%) vs prior period. Biggest increase: NAT Gateway (+$7,900.00)."
	if got != want {
		t.Fatalf("headline =\n%q\nwant\n%q", got, want)
	}
	in := sampleInput()
	in.Period, in.AccountCount, in.Trends = "mtd", 1, nil
	if got := BuildHeadline(in); got != "Month to date: $184,220.00 across 1 account." {
		t.Fatalf("minimal headline = %q", got)
	}
}

func TestBuildHeadlineBiggestIncreaseIsInDollars(t *testing.T) {
	in := sampleInput()
	small := ServiceTrend{Service: "RDS", DisplayName: "RDS", PriorUSD: 7820, CurrentUSD: 9200, ChangeUSD: 1380, ChangePercent: 17.6}
	big := ServiceTrend{Service: "EC2", DisplayName: "EC2 compute", PriorUSD: 21970, CurrentUSD: 24200, ChangeUSD: 2230, ChangePercent: 10.2}
	in.Trends.ServiceTrends = []ServiceTrend{small, big}
	in.Trends.TopIncreases = []ServiceTrend{small, big} // ranked by % like the API
	got := BuildHeadline(in)
	if !strings.HasSuffix(got, "Biggest increase: EC2 compute (+$2,230.00).") {
		t.Fatalf("headline should name the largest dollar increase, got %q", got)
	}
}
