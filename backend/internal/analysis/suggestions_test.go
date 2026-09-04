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
	out := BuildSuggestions(dash, nil)
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
	out := BuildSuggestions(dash, trends)
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
	out := BuildSuggestions(dash, nil)
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
