package costexplorer

import "testing"

func TestReconcileTaxTotal(t *testing.T) {
	tests := []struct {
		name      string
		inclTax   float64
		usage     float64
		want      float64
	}{
		{"normal", 72400, 70300, 2100},
		{"zero usage", 100, 0, 100},
		{"zero tax", 500, 500, 0},
		{"rounding negative floors to zero", 100.001, 100.002, 0},
		{"both zero", 0, 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ReconcileTaxTotal(tc.inclTax, tc.usage); got != tc.want {
				t.Errorf("ReconcileTaxTotal(%v, %v) = %v, want %v", tc.inclTax, tc.usage, got, tc.want)
			}
		})
	}
}

func TestTaxBreakdownHas(t *testing.T) {
	if (*TaxBreakdown)(nil).Has() {
		t.Error("nil TaxBreakdown must not have data")
	}
	if (&TaxBreakdown{TotalUSD: 0}).Has() {
		t.Error("zero total must not have data")
	}
	if !(&TaxBreakdown{TotalUSD: 50}).Has() {
		t.Error("non-zero total must have data")
	}
}
