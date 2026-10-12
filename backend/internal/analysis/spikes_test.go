package analysis

import (
	"fmt"
	"testing"
	"time"
)

func dailySeries(n int, base float64) []DailyPoint {
	out := make([]DailyPoint, 0, n)
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		out = append(out, DailyPoint{
			Date:   start.AddDate(0, 0, i).Format("2006-01-02"),
			Amount: base,
		})
	}
	return out
}

func TestDetectSpikesFindsUpSpike(t *testing.T) {
	daily := dailySeries(21, 1000)
	// 3x spike on day index 18 (3 full days after it).
	daily[18].Amount = 3000

	spikes := DetectSpikes(daily, SpikeParams{})
	if len(spikes) == 0 {
		t.Fatal("expected a spike")
	}
	top := spikes[0]
	if top.Date != daily[18].Date {
		t.Fatalf("top spike date: got %s, want %s", top.Date, daily[18].Date)
	}
	if top.Direction != "up" {
		t.Fatalf("direction: got %s", top.Direction)
	}
	if top.DeviationPct < 100 {
		t.Fatalf("deviation: got %.1f%%, want >= 100%%", top.DeviationPct)
	}
	if top.BaselineUSD < 900 || top.BaselineUSD > 1100 {
		t.Fatalf("baseline: got %.0f, want ~1000", top.BaselineUSD)
	}
}

func TestDetectSpikesIgnoresLaggingDips(t *testing.T) {
	daily := dailySeries(21, 1000)
	// Most recent day is ~0 (CE lag) — must NOT be flagged as a dip.
	daily[20].Amount = 10

	spikes := DetectSpikes(daily, SpikeParams{SkipLast: 2})
	for _, s := range spikes {
		if s.Date == daily[20].Date {
			t.Fatalf("CE-lagging dip was flagged: %+v", s)
		}
	}
}

func TestDetectSpikesSkipsUnsettledDaysBothDirections(t *testing.T) {
	// A big move on a day that is still inside the CE-lag window must not be
	// analyzed at all (up or down) — it is a partial day, not a real anomaly.
	daily := dailySeries(21, 1000)
	daily[19].Amount = 5000 // second-to-last day, within SkipLast=2
	daily[20].Amount = 10   // last day, within SkipLast=2

	spikes := DetectSpikes(daily, SpikeParams{SkipLast: 2})
	for _, s := range spikes {
		if s.Date == daily[19].Date || s.Date == daily[20].Date {
			t.Fatalf("unsettled day was flagged: %+v", s)
		}
	}

	// Same data with SkipLast=0: the moves are real and get flagged.
	spikes = DetectSpikes(daily, SpikeParams{SkipLast: 0})
	if len(spikes) == 0 {
		t.Fatal("expected spikes when no days are skipped")
	}
}

func TestDetectSpikesAdaptiveFloor(t *testing.T) {
	// With a low percentage threshold (to catch subtle drift on a stable
	// org), the adaptive floor (15% of baseline by default) still stops pure
	// noise: +10% on a $1000 baseline is below the $150 floor.
	daily := dailySeries(21, 1000)
	daily[18].Amount = 1100 // +10%

	params := SpikeParams{MinDeviationPct: 10}
	for _, s := range DetectSpikes(daily, params) {
		if s.Date == daily[18].Date {
			t.Fatalf("noise below adaptive floor was flagged: %+v", s)
		}
	}

	// Lowering the share floor makes the same move flag.
	params.MinBaselineSharePct = 5
	if spikes := DetectSpikes(daily, params); len(spikes) == 0 || spikes[0].Date != daily[18].Date {
		t.Fatalf("expected spike with lowered share floor, got %+v", spikes)
	}

	// Default pct (50%): the same +10% move is ignored by the pct test.
	if spikes := DetectSpikes(daily, SpikeParams{}); len(spikes) != 0 {
		t.Fatalf("expected no spikes at default thresholds, got %+v", spikes)
	}
}

func TestDetectSpikesRespectsUSDThreshold(t *testing.T) {
	daily := dailySeries(21, 20)
	// +200% but only $40 above baseline of $20 — absolute deviation below $50.
	daily[18].Amount = 60

	spikes := DetectSpikes(daily, SpikeParams{})
	for _, s := range spikes {
		if s.Date == daily[18].Date {
			t.Fatalf("spike below MinDeviationUSD was flagged: %+v", s)
		}
	}
}

func TestDetectSpikesAccountAttribution(t *testing.T) {
	daily := dailySeries(21, 1000)
	daily[18].Amount = 3000

	accounts := []AccountDailyView{
		{AccountID: "111", AccountName: "prod"},
		{AccountID: "222", AccountName: "staging"},
	}
	// prod carries most of day 18's extra spend.
	for i := range daily {
		accounts[0].Daily = append(accounts[0].Daily, DailyPoint{Date: daily[i].Date, Amount: 700})
		accounts[1].Daily = append(accounts[1].Daily, DailyPoint{Date: daily[i].Date, Amount: 300})
	}
	accounts[0].Daily[18].Amount = 2200
	accounts[1].Daily[18].Amount = 800

	spikes := DetectSpikesWithAccounts(daily, accounts, SpikeParams{})
	if len(spikes) == 0 {
		t.Fatal("expected a spike")
	}
	if len(spikes[0].AccountIDs) == 0 {
		t.Fatal("expected account attribution")
	}
	if spikes[0].AccountIDs[0] != "111" {
		t.Fatalf("top account: got %v, want 111 first", spikes[0].AccountIDs)
	}
}

func TestDetectSpikesNeedsEnoughData(t *testing.T) {
	daily := dailySeries(10, 1000)
	daily[9].Amount = 5000
	if spikes := DetectSpikes(daily, SpikeParams{}); len(spikes) != 0 {
		t.Fatalf("expected no spikes with insufficient data, got %+v", spikes)
	}
}

func TestDetectSpikesMaxCap(t *testing.T) {
	daily := dailySeries(40, 100)
	for i := 15; i < 38; i += 2 {
		daily[i].Amount = 500
	}
	spikes := DetectSpikes(daily, SpikeParams{MaxSpikes: 3})
	if len(spikes) > 3 {
		t.Fatalf("expected at most 3 spikes, got %d", len(spikes))
	}
	for i, s := range spikes {
		fmt.Printf("spike %d: %s %.0f%%\n", i, s.Date, s.DeviationPct)
	}
}
