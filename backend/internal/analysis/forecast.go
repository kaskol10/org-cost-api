package analysis

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// ForecastResult is a projected end-of-month spend estimate.
type ForecastResult struct {
	// ProjectedUSD is the projected full-month spend.
	ProjectedUSD float64 `json:"projected_usd"`
	// MTDUSD is spend so far in the month.
	MTDUSD float64 `json:"mtd_usd"`
	// DaysElapsed is the number of MTD days with data.
	DaysElapsed int `json:"days_elapsed"`
	// DaysInMonth is the total days in the current month.
	DaysInMonth int `json:"days_in_month"`
	// DailyAverage is the MTD daily average used for the projection.
	DailyAverage float64 `json:"daily_average"`
	// Method is a short human description of the projection.
	Method string `json:"method"`
}

// BudgetStatus compares a projection to a configured budget.
type BudgetStatus struct {
	Name       string  `json:"name"`
	MonthlyUSD float64 `json:"monthly_usd"`
	Account    string  `json:"account,omitempty"`
	// ProjectedUSD is the projected spend for the month (org unless scoped to an account).
	ProjectedUSD float64 `json:"projected_usd"`
	// PercentOfBudget is projected / budget * 100.
	PercentOfBudget float64 `json:"percent_of_budget"`
	// Status is ok | warning | over.
	Status string `json:"status"`
	// OverByUSD is the projected overrun (positive when over budget).
	OverByUSD float64 `json:"over_by_usd,omitempty"`
}

// BudgetStatus thresholds.
const (
	BudgetWarningPct = 90.0
	BudgetOverPct    = 100.0
)

// BudgetStatusFor evaluates a single budget against a projected amount.
func BudgetStatusFor(name string, monthlyUSD float64, account string, projectedUSD float64) *BudgetStatus {
	if monthlyUSD <= 0 {
		return nil
	}
	pct := projectedUSD / monthlyUSD * 100
	status := "ok"
	if pct > BudgetOverPct {
		status = "over"
	} else if pct > BudgetWarningPct {
		status = "warning"
	}
	bs := &BudgetStatus{
		Name:            name,
		MonthlyUSD:      monthlyUSD,
		Account:         account,
		ProjectedUSD:    projectedUSD,
		PercentOfBudget: pct,
		Status:          status,
	}
	if projectedUSD > monthlyUSD {
		bs.OverByUSD = projectedUSD - monthlyUSD
	}
	return bs
}

// ProjectEOM projects full-month spend from the daily series so far using a
// seasonal-naive daily average (today's run-rate). No extra CE calls needed —
// it operates on the org daily series already fetched for the dashboard.
//
// Daily points inside the CE-lag window are still included in the MTD total
// (they are real spend, just understated); the projection simply extrapolates
// the observed daily average.
func ProjectEOM(daily []DailyPoint) (*ForecastResult, error) {
	if len(daily) == 0 {
		return nil, fmt.Errorf("no daily data to project from")
	}
	now := time.Now().UTC()
	monthPrefix := now.Format("2006-01")

	// Keep only the current calendar month, sorted by date.
	var mtd []DailyPoint
	for _, d := range daily {
		if d.Date[:7] == monthPrefix {
			mtd = append(mtd, d)
		}
	}
	if len(mtd) == 0 {
		return nil, fmt.Errorf("no daily data for the current month")
	}
	sort.Slice(mtd, func(i, j int) bool { return mtd[i].Date < mtd[j].Date })

	var total float64
	for _, d := range mtd {
		total += d.Amount
	}
	daysElapsed := len(mtd)
	// Cap at today: the series can include "today" which is incomplete.
	if daysElapsed > now.Day() {
		daysElapsed = now.Day()
	}
	if daysElapsed < 1 {
		daysElapsed = 1
	}
	daysInMonth := daysInMonth(now.Year(), now.Month())
	dailyAvg := total / float64(daysElapsed)
	projected := dailyAvg * float64(daysInMonth)

	return &ForecastResult{
		ProjectedUSD:  math.Round(projected*100) / 100,
		MTDUSD:        math.Round(total*100) / 100,
		DaysElapsed:   daysElapsed,
		DaysInMonth:   daysInMonth,
		DailyAverage:  math.Round(dailyAvg*100) / 100,
		Method:        fmt.Sprintf("daily-average over %d MTD days", daysElapsed),
	}, nil
}

// ProjectEOMAccount is ProjectEOM scoped to one account's daily series.
func ProjectEOMAccount(daily []DailyPoint) (*ForecastResult, error) {
	return ProjectEOM(daily)
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// MonthLabel returns e.g. "2026-09" for a date string.
func MonthLabel(date string) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	return t.Format("2006-01")
}
