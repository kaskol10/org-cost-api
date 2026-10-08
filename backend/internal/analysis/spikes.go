package analysis

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// Spike is a single-day anomaly in org spend vs a rolling baseline.
type Spike struct {
	Date         string   `json:"date"`
	AmountUSD    float64  `json:"amount_usd"`
	BaselineUSD  float64  `json:"baseline_usd"`
	DeviationPct float64  `json:"deviation_pct"`
	Direction    string   `json:"direction"` // up | down
	AccountIDs   []string `json:"account_ids,omitempty"`
	// Incomplete is true when the day is inside the CE-lag window.
	Incomplete bool `json:"incomplete,omitempty"`
}

// SpikeParams tunes spike detection.
type SpikeParams struct {
	// BaselineDays is the trailing window for the baseline (default 14).
	BaselineDays int
	// MinDeviationPct: flag when |amount - baseline| / baseline >= this (default 50).
	MinDeviationPct float64
	// MinDeviationUSD: also require absolute deviation >= this (default 50).
	MinDeviationUSD float64
	// SkipLast is the number of most recent days to treat as CE-lagging.
	SkipLast int
	// MaxSpikes caps the result (default 10).
	MaxSpikes int
}

func (p SpikeParams) defaults() SpikeParams {
	if p.BaselineDays < 7 {
		p.BaselineDays = 14
	}
	if p.MinDeviationPct <= 0 {
		p.MinDeviationPct = 50
	}
	if p.MinDeviationUSD <= 0 {
		p.MinDeviationUSD = 50
	}
	if p.MaxSpikes <= 0 {
		p.MaxSpikes = 10
	}
	return p
}

// DetectSpikes flags days whose spend deviates from the trailing median
// baseline. Median (not mean) keeps the baseline robust to the spikes
// themselves and to monthly seasonality.
func DetectSpikes(daily []DailyPoint, params SpikeParams) []Spike {
	return DetectSpikesWithAccounts(daily, nil, params)
}

// DetectSpikesWithAccounts is DetectSpikes plus per-account attribution for
// each flagged day (top accounts contributing that day's spend).
func DetectSpikesWithAccounts(daily []DailyPoint, accounts []AccountDailyView, params SpikeParams) []Spike {
	params = params.defaults()
	if len(daily) < params.BaselineDays+2 {
		return nil
	}

	acctByDate := make(map[string]map[string]float64)
	for _, ad := range accounts {
		if ad.AccountID == "" {
			continue
		}
		for _, d := range ad.Daily {
			m := acctByDate[d.Date]
			if m == nil {
				m = make(map[string]float64)
				acctByDate[d.Date] = m
			}
			m[ad.AccountID] += d.Amount
		}
	}

	sorted := make([]DailyPoint, len(daily))
	copy(sorted, daily)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Date < sorted[j].Date })

	var out []Spike
	// Walk days oldest→newest so the trailing baseline never includes the
	// candidate day itself.
	for i := params.BaselineDays; i < len(sorted); i++ {
		day := sorted[i]
		window := sorted[i-params.BaselineDays : i]
		base := median(windowAmounts(window))

		deviation := day.Amount - base
		var devPct float64
		if base > 0.01 {
			devPct = (deviation / base) * 100
		} else if day.Amount >= params.MinDeviationUSD {
			devPct = 100
		} else {
			continue // baseline and amount are both ~zero
		}
		if math.Abs(devPct) < params.MinDeviationPct {
			continue
		}
		if math.Abs(deviation) < params.MinDeviationUSD {
			continue
		}
		// A near-zero recent day is usually CE lag, not a real drop.
		if deviation < 0 && i >= len(sorted)-params.SkipLast {
			continue
		}
		spike := Spike{
			Date:         day.Date,
			AmountUSD:    day.Amount,
			BaselineUSD:  base,
			DeviationPct: devPct,
			Direction:    directionFor(deviation),
		}
		if i >= len(sorted)-params.SkipLast {
			spike.Incomplete = true
		}
		if amounts, ok := acctByDate[day.Date]; ok {
			spike.AccountIDs = topAccountIDs(amounts, params.MinDeviationUSD*0.5)
		}
		out = append(out, spike)
	}

	sort.Slice(out, func(i, j int) bool {
		return math.Abs(out[i].DeviationPct) > math.Abs(out[j].DeviationPct)
	})
	if len(out) > params.MaxSpikes {
		out = out[:params.MaxSpikes]
	}
	return out
}

func windowAmounts(window []DailyPoint) []float64 {
	out := make([]float64, len(window))
	for i, d := range window {
		out[i] = d.Amount
	}
	return out
}

func median(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sorted := make([]float64, len(vals))
	copy(sorted, vals)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2
}

func directionFor(deviation float64) string {
	if deviation > 0 {
		return "up"
	}
	return "down"
}

func topAccountIDs(amounts map[string]float64, minUSD float64) []string {
	type kv struct {
		id  string
		amt float64
	}
	var entries []kv
	for id, amt := range amounts {
		if amt >= minUSD {
			entries = append(entries, kv{id, amt})
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].amt > entries[j].amt })
	var out []string
	for i, e := range entries {
		if i >= 3 {
			break
		}
		out = append(out, e.id)
	}
	return out
}

// SpikeDateLabel renders a spike date for copy (e.g. "Sep 3").
func SpikeDateLabel(date string) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	return t.Format("Jan 2")
}

// SpikeSummaryLine is a one-line human description of a spike.
func SpikeSummaryLine(s Spike) string {
	pct := fmt.Sprintf("%.0f%%", math.Abs(s.DeviationPct))
	if s.Direction == "up" {
		return fmt.Sprintf("%s spent %s (+%s vs ~%s baseline)",
			SpikeDateLabel(s.Date), fmtMoney(s.AmountUSD), pct, fmtMoney(s.BaselineUSD))
	}
	return fmt.Sprintf("%s spend dropped to %s (-%s vs ~%s baseline)",
		SpikeDateLabel(s.Date), fmtMoney(s.AmountUSD), pct, fmtMoney(s.BaselineUSD))
}

func fmtMoney(v float64) string {
	return fmt.Sprintf("$%.0f", v)
}
