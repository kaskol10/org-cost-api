package analysis

import (
	"math"
	"testing"
	"time"
)

// currentMonthDaily builds N days of the current calendar month (from the 1st).
func currentMonthDaily(n int, perDay float64) []DailyPoint {
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	out := make([]DailyPoint, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, DailyPoint{
			Date:   start.AddDate(0, 0, i).Format("2006-01-02"),
			Amount: perDay,
		})
	}
	return out
}

func TestProjectEOMRoughlyLinear(t *testing.T) {
	now := time.Now().UTC()
	days := now.Day()
	perDay := 100.0
	daily := currentMonthDaily(days, perDay)

	res, err := ProjectEOM(daily)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	daysInMonth := daysInMonth(now.Year(), now.Month())
	want := perDay * float64(daysInMonth)
	if math.Abs(res.ProjectedUSD-want) > want*0.01 {
		t.Fatalf("projected %.2f, want ~%.2f", res.ProjectedUSD, want)
	}
	if res.DaysElapsed != days {
		t.Fatalf("days elapsed: got %d, want %d", res.DaysElapsed, days)
	}
	if res.DaysInMonth != daysInMonth {
		t.Fatalf("days in month: got %d, want %d", res.DaysInMonth, daysInMonth)
	}
}

func TestProjectEOMFiltersOtherMonths(t *testing.T) {
	now := time.Now().UTC()
	days := now.Day()
	perDay := 100.0
	daily := currentMonthDaily(days, perDay)
	// Add a prior-month point that must be ignored.
	prev := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)
	daily = append(daily, DailyPoint{Date: prev.Format("2006-01-02"), Amount: 9999})

	res, err := ProjectEOM(daily)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.MTDUSD > perDay*float64(days)+0.01 {
		t.Fatalf("MTD %.2f includes prior month (9999)", res.MTDUSD)
	}
}

func TestProjectEOMEmpty(t *testing.T) {
	if _, err := ProjectEOM(nil); err == nil {
		t.Fatal("expected error for empty series")
	}
}

func TestBudgetStatusThresholds(t *testing.T) {
	// Under budget.
	b := BudgetStatusFor("org", 10000, "", 8000)
	if b == nil || b.Status != "ok" {
		t.Fatalf("expected ok, got %+v", b)
	}
	// Warning (>90%).
	b = BudgetStatusFor("org", 10000, "", 9500)
	if b == nil || b.Status != "warning" {
		t.Fatalf("expected warning, got %+v", b)
	}
	// Over (>100%).
	b = BudgetStatusFor("org", 10000, "", 11000)
	if b == nil || b.Status != "over" || b.OverByUSD != 1000 {
		t.Fatalf("expected over with 1000 over, got %+v", b)
	}
	// Zero budget -> nil.
	if b := BudgetStatusFor("org", 0, "", 100); b != nil {
		t.Fatalf("expected nil for zero budget, got %+v", b)
	}
}

func TestBudgetStatusPercent(t *testing.T) {
	b := BudgetStatusFor("org", 2000, "", 1500)
	if b == nil || math.Abs(b.PercentOfBudget-75) > 0.001 {
		t.Fatalf("expected 75%%, got %+v", b)
	}
}
