package config

import (
	"testing"
	"time"
)

func TestParsePeriod(t *testing.T) {
	cases := []struct {
		in   string
		want PeriodMode
		ok   bool
	}{
		{"", PeriodLookback, true},
		{"30d", PeriodLookback, true},
		{"lookback", PeriodLookback, true},
		{"mtd", PeriodMTD, true},
		{"month_to_date", PeriodMTD, true},
		{"year", "", false},
	}
	for _, tc := range cases {
		got, err := ParsePeriod(tc.in)
		if tc.ok {
			if err != nil {
				t.Fatalf("ParsePeriod(%q): %v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("ParsePeriod(%q)=%q, want %q", tc.in, got, tc.want)
			}
		} else if err == nil {
			t.Fatalf("ParsePeriod(%q): expected error", tc.in)
		}
	}
}

func TestPriorPeriodForLookback(t *testing.T) {
	start, end := PriorPeriodFor(PeriodLookback, "2026-07-11", "2026-08-10")
	if start != "2026-06-11" || end != "2026-07-11" {
		t.Fatalf("got %q–%q", start, end)
	}
}

func TestPriorPeriodForMTD(t *testing.T) {
	start, end := PriorPeriodFor(PeriodMTD, "2026-03-01", "2026-03-09")
	if start != "2026-02-01" || end != "2026-02-09" {
		t.Fatalf("got %q–%q, want 2026-02-01–2026-02-09", start, end)
	}
	// Clamp end day when prior month is shorter (Mar 31 → Feb 28/29).
	start, end = PriorPeriodFor(PeriodMTD, "2026-03-01", "2026-03-31")
	if start != "2026-02-01" || end != "2026-02-28" {
		t.Fatalf("got %q–%q, want 2026-02-01–2026-02-28", start, end)
	}
}

func TestCostDateRangeForMTD(t *testing.T) {
	cfg := &Config{CostLookbackDays: 30}
	start, end := cfg.CostDateRangeFor(PeriodMTD)
	now := time.Now().UTC()
	wantStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	wantEnd := now.Truncate(24 * time.Hour).Format("2006-01-02")
	if start != wantStart || end != wantEnd {
		t.Fatalf("got %q–%q, want %q–%q", start, end, wantStart, wantEnd)
	}
}
