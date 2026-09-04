package analysis

import "testing"

func TestBuildTrendsSpikeDetection(t *testing.T) {
	dash := DashboardView{
		Start: "2026-07-11",
		End:   "2026-08-10",
		Totals: TotalsView{
			OrgTotal: 4000,
		},
		TopServices: []ServiceDriverView{
			{Service: "Amazon Redshift", Amount: 400},
		},
	}
	prior := map[string]float64{
		"Amazon Redshift": 100,
	}
	resp := BuildTrends(dash, prior, 1000, "history_snapshot", 0, "", 3)

	if len(resp.TopIncreases) == 0 {
		t.Fatal("expected top increases")
	}
	inc := resp.TopIncreases[0]
	if inc.Service != "Amazon Redshift" {
		t.Fatalf("got service %q", inc.Service)
	}
	if inc.ChangePercent < 200 {
		t.Fatalf("expected large increase, got %.1f%%", inc.ChangePercent)
	}
	if resp.OrgTotal.ChangeUSD != 3000 {
		t.Fatalf("org change USD: got %.0f, want 3000", resp.OrgTotal.ChangeUSD)
	}
}

func TestBuildTrendsIgnoresTinyAmounts(t *testing.T) {
	dash := DashboardView{
		Totals: TotalsView{OrgTotal: 100},
		TopServices: []ServiceDriverView{
			{Service: "Amazon S3", Amount: 10},
		},
	}
	prior := map[string]float64{"Amazon S3": 0.001}
	resp := BuildTrends(dash, prior, 0.001, "test", 0, "", 0)
	if len(resp.ServiceTrends) != 0 && resp.ServiceTrends[0].CurrentUSD < 0.01 {
		t.Fatalf("unexpected tiny trend: %+v", resp.ServiceTrends)
	}
}

func TestBuildTrendsMinUSDThresholdForIncreases(t *testing.T) {
	dash := DashboardView{
		Totals: TotalsView{OrgTotal: 200},
		TopServices: []ServiceDriverView{
			{Service: "Amazon RDS", Amount: 30},
		},
	}
	prior := map[string]float64{"Amazon RDS": 10}
	resp := BuildTrends(dash, prior, 50, "test", 0, "", 0)
	for _, inc := range resp.TopIncreases {
		if inc.ChangeUSD < minTrendUSD {
			t.Fatalf("increase below min USD threshold: %+v", inc)
		}
	}
}
