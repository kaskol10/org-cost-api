package config

import (
	"fmt"
	"strings"
	"time"
)

// PeriodMode selects the dashboard/trends date window.
type PeriodMode string

const (
	// PeriodLookback is trailing CostLookbackDays (query value "30d" or empty).
	PeriodLookback PeriodMode = "30d"
	// PeriodMTD is calendar month-to-date (1st of month → today UTC).
	PeriodMTD PeriodMode = "mtd"
)

// ParsePeriod maps a query/API value to a PeriodMode.
// Empty, "30d", and "lookback" all mean trailing lookback.
func ParsePeriod(raw string) (PeriodMode, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "30d", "lookback", "trailing":
		return PeriodLookback, nil
	case "mtd", "month", "month_to_date":
		return PeriodMTD, nil
	default:
		return "", fmt.Errorf("invalid period %q (want 30d or mtd)", raw)
	}
}

// CostDateRangeFor returns [start, end] (UTC dates) for the given period mode.
func (c *Config) CostDateRangeFor(mode PeriodMode) (start, end string) {
	endTime := time.Now().UTC().Truncate(24 * time.Hour)
	switch mode {
	case PeriodMTD:
		startTime := time.Date(endTime.Year(), endTime.Month(), 1, 0, 0, 0, 0, time.UTC)
		return startTime.Format("2006-01-02"), endTime.Format("2006-01-02")
	default:
		days := c.CostLookbackDays
		if days < 1 {
			days = 30
		}
		startTime := endTime.AddDate(0, 0, -days)
		return startTime.Format("2006-01-02"), endTime.Format("2006-01-02")
	}
}

// CostDateRange is CostDateRangeFor(PeriodLookback).
func (c *Config) CostDateRange() (start, end string) {
	return c.CostDateRangeFor(PeriodLookback)
}

// PeriodDayCount returns the whole-day span between start and end (end - start).
func PeriodDayCount(start, end string) int {
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

// PriorPeriodFor returns the comparison window for the current range.
// Lookback: equal-length window immediately before current start.
// MTD: same calendar day-of-month range in the previous month (clamped).
func PriorPeriodFor(mode PeriodMode, currentStart, currentEnd string) (string, string) {
	start, err1 := time.Parse("2006-01-02", currentStart)
	end, err2 := time.Parse("2006-01-02", currentEnd)
	if err1 != nil || err2 != nil {
		return "", ""
	}
	days := int(end.Sub(start).Hours() / 24)
	if days < 1 {
		days = 30
	}

	if mode == PeriodMTD {
		prevMonth := start.AddDate(0, -1, 0)
		priorStart := time.Date(prevMonth.Year(), prevMonth.Month(), 1, 0, 0, 0, 0, time.UTC)
		// Same day-of-month as current end, clamped to last day of prior month.
		wantDay := end.Day()
		lastDay := time.Date(priorStart.Year(), priorStart.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
		if wantDay > lastDay {
			wantDay = lastDay
		}
		priorEnd := time.Date(priorStart.Year(), priorStart.Month(), wantDay, 0, 0, 0, 0, time.UTC)
		return priorStart.Format("2006-01-02"), priorEnd.Format("2006-01-02")
	}

	priorEnd := start
	priorStart := start.AddDate(0, 0, -days)
	return priorStart.Format("2006-01-02"), priorEnd.Format("2006-01-02")
}
