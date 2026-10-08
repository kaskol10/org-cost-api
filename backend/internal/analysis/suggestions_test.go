package analysis

import "testing"

func TestBuildSuggestionsUnattachedEBS(t *testing.T) {
	gib := 100.0
	dash := DashboardView{
		Start: "2026-07-11",
		End:   "2026-08-10",
		Totals: TotalsView{
			OrgTotal:        1000,
			VolumeAvailable: 5,
			VolumeSizeGiB:   gib,
		},
		Accounts: []AccountView{
			{
				AccountName: "production",
				Volumes: &VolumeView{
					AvailableCount: 5,
					AvailableGiB:   gib,
				},
			},
		},
	}
	out := BuildSuggestions(dash, nil, nil)
	if len(out.Suggestions) == 0 {
		t.Fatal("expected suggestions")
	}
	if out.Suggestions[0].ID != "unattached-ebs" {
		t.Fatalf("first suggestion: %+v", out.Suggestions[0])
	}
	if out.Suggestions[0].EstimatedMonthlyUSD == nil || *out.Suggestions[0].EstimatedMonthlyUSD != 8 {
		t.Fatalf("expected $8/mo estimate, got %+v", out.Suggestions[0].EstimatedMonthlyUSD)
	}
}

func TestBuildSuggestionsSpendSpike(t *testing.T) {
	dash := DashboardView{
		Start: "2026-07-11",
		End:   "2026-08-10",
		Totals: TotalsView{OrgTotal: 5000},
	}
	trends := &TrendsResponse{
		CurrentPeriod: PeriodSummary{Start: "2026-07-11", End: "2026-08-10"},
		PriorPeriod:   PeriodSummary{Start: "2026-06-11", End: "2026-07-10"},
		TopIncreases: []ServiceTrend{
			{
				Service:       "Amazon Redshift",
				DisplayName:   "Redshift",
				CurrentUSD:    400,
				PriorUSD:      100,
				ChangePercent: 300,
				Direction:     "up",
			},
		},
	}
	out := BuildSuggestions(dash, trends, nil)
	found := false
	for _, s := range out.Suggestions {
		if s.Category == "trend" && s.Service == "Amazon Redshift" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected Redshift spike suggestion, got %+v", out.Suggestions)
	}
}

func TestBuildSuggestionsOtherServicesID(t *testing.T) {
	dash := DashboardView{
		Start: "2026-07-11",
		End:   "2026-08-10",
		Totals: TotalsView{OrgTotal: 10000},
		Accounts: []AccountView{
			{
				AccountName:        "production",
				OtherServicesTotal: 1200,
			},
		},
	}
	out := BuildSuggestions(dash, nil, nil)
	found := false
	for _, s := range out.Suggestions {
		if s.ID == "other-services-production" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected other-services-production suggestion, got %+v", out.Suggestions)
	}
}

func TestBuildSuggestionsLowCommitmentCoverage(t *testing.T) {
	util := 72.0
	uncommitted := 9400.0
	dash := DashboardView{
		Start: "2026-07-11",
		End:   "2026-08-10",
		Totals: TotalsView{OrgTotal: 50000},
		Commitments: &CommitmentView{
			SPUtilizationPct: &util,
			UncommittedUSD:   &uncommitted,
			HasCommitments:   true,
		},
	}
	out := BuildSuggestions(dash, nil, nil)
	found := false
	for _, s := range out.Suggestions {
		if s.ID == "low-commitment-coverage" && s.Category == "commitment" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected low-commitment-coverage suggestion, got %+v", out.Suggestions)
	}
}

func TestBuildSuggestionsNoCommitmentWhenHighUtilization(t *testing.T) {
	util := 95.0
	dash := DashboardView{
		Start: "2026-07-11",
		End:   "2026-08-10",
		Totals: TotalsView{OrgTotal: 50000},
		Commitments: &CommitmentView{
			SPUtilizationPct: &util,
			HasCommitments:   true,
		},
	}
	out := BuildSuggestions(dash, nil, nil)
	for _, s := range out.Suggestions {
		if s.ID == "low-commitment-coverage" {
			t.Fatalf("did not expect low-commitment-coverage at 95%% utilization: %+v", s)
		}
	}
}

func TestBuildSuggestionsDailySpike(t *testing.T) {
	dash := DashboardView{
		Start: "2026-07-11",
		End:   "2026-08-10",
		Totals: TotalsView{OrgTotal: 5000},
	}
	trends := &TrendsResponse{
		CurrentPeriod: PeriodSummary{Start: "2026-07-11", End: "2026-08-10"},
		PriorPeriod:   PeriodSummary{Start: "2026-06-11", End: "2026-07-10"},
		Spikes: []Spike{
			{
				Date:         "2026-08-05",
				AmountUSD:    3000,
				BaselineUSD:  1000,
				DeviationPct: 200,
				Direction:    "up",
				AccountIDs:   []string{"111111111111"},
			},
		},
	}
	out := BuildSuggestions(dash, trends, nil)
	found := false
	for _, s := range out.Suggestions {
		if s.ID == "daily-spike-2026-08-05" && s.Category == "spike" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected daily-spike suggestion, got %+v", out.Suggestions)
	}
}

func TestBuildSuggestionsOverBudget(t *testing.T) {
	dash := DashboardView{
		Start: "2026-07-11",
		End:   "2026-08-10",
		Totals: TotalsView{OrgTotal: 5000},
	}
	budgets := []*BudgetStatus{
		BudgetStatusFor("org", 10000, "", 11000),
	}
	out := BuildSuggestions(dash, nil, budgets)
	found := false
	for _, s := range out.Suggestions {
		if s.ID == "over-budget-org" && s.Category == "budget" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected over-budget-org suggestion, got %+v", out.Suggestions)
	}
}

func TestBuildSuggestionsNoBudgetWhenOK(t *testing.T) {
	dash := DashboardView{
		Start: "2026-07-11",
		End:   "2026-08-10",
		Totals: TotalsView{OrgTotal: 5000},
	}
	budgets := []*BudgetStatus{
		BudgetStatusFor("org", 10000, "", 5000),
	}
	out := BuildSuggestions(dash, nil, budgets)
	for _, s := range out.Suggestions {
		if s.Category == "budget" {
			t.Fatalf("did not expect a budget suggestion when under budget: %+v", s)
		}
	}
}
