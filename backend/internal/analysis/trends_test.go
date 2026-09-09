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
	resp := BuildTrends(dash, prior, nil, 1000, "history_snapshot", 0, "", 3)

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
	resp := BuildTrends(dash, prior, nil, 0.001, "test", 0, "", 0)
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
	resp := BuildTrends(dash, prior, nil, 50, "test", 0, "", 0)
	for _, inc := range resp.TopIncreases {
		if inc.ChangeUSD < minTrendUSD {
			t.Fatalf("increase below min USD threshold: %+v", inc)
		}
	}
}

func TestBuildTrendsAccountMovers(t *testing.T) {
	dash := DashboardView{
		Start:  "2026-07-11",
		End:    "2026-08-10",
		Totals: TotalsView{OrgTotal: 5000},
		Accounts: []AccountView{
			{AccountID: "111", AccountName: "prod", AllTotal: 3000},
			{AccountID: "222", AccountName: "dev", AllTotal: 200},
			{AccountID: "333", AccountName: "staging", AllTotal: 800},
		},
	}
	priorAccounts := map[string]float64{
		"111": 1000, // +2000 up
		"222": 800,  // -600 down
		"333": 780,  // ~stable (~2.5%)
	}
	resp := BuildTrends(dash, nil, priorAccounts, 2580, "history_snapshot", 0, "", 1)

	if len(resp.TopAccountIncreases) == 0 {
		t.Fatal("expected account increases")
	}
	inc := resp.TopAccountIncreases[0]
	if inc.AccountID != "111" {
		t.Fatalf("top increase account: got %q, want 111", inc.AccountID)
	}
	if inc.AccountName != "prod" {
		t.Fatalf("account name: got %q", inc.AccountName)
	}
	if inc.ChangeUSD < minTrendUSD {
		t.Fatalf("increase below min USD: %+v", inc)
	}

	if len(resp.TopAccountDecreases) == 0 {
		t.Fatal("expected account decreases")
	}
	dec := resp.TopAccountDecreases[0]
	if dec.AccountID != "222" {
		t.Fatalf("top decrease account: got %q, want 222", dec.AccountID)
	}

	for _, tnd := range resp.TopAccountIncreases {
		if tnd.ChangeUSD < minTrendUSD {
			t.Fatalf("account increase below min USD: %+v", tnd)
		}
	}
	if len(resp.TopAccountIncreases) > 5 {
		t.Fatalf("expected at most 5 account increases, got %d", len(resp.TopAccountIncreases))
	}
}

func TestBuildTrendsAccountMinUSDThreshold(t *testing.T) {
	dash := DashboardView{
		Totals: TotalsView{OrgTotal: 100},
		Accounts: []AccountView{
			{AccountID: "tiny", AccountName: "tiny", AllTotal: 30},
		},
	}
	priorAccounts := map[string]float64{"tiny": 20} // +10 below minTrendUSD
	resp := BuildTrends(dash, nil, priorAccounts, 20, "test", 0, "", 0)
	for _, inc := range resp.TopAccountIncreases {
		if inc.ChangeUSD < minTrendUSD {
			t.Fatalf("account increase below min USD threshold: %+v", inc)
		}
	}
	if len(resp.TopAccountIncreases) != 0 {
		t.Fatalf("expected no top account increases for +$10, got %+v", resp.TopAccountIncreases)
	}
}
