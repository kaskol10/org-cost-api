package analysis

import "testing"

func TestEBSPricePerGiBMonth(t *testing.T) {
	cases := map[string]float64{"gp3": 0.08, "GP2": 0.10, "io2": 0.125, "sc1": 0.015, "weird": DefaultEBSPricePerGiBMonth, "": DefaultEBSPricePerGiBMonth}
	for vt, want := range cases {
		if got := EBSPricePerGiBMonth(vt); got != want {
			t.Errorf("EBSPricePerGiBMonth(%q) = %v, want %v", vt, got, want)
		}
	}
}

func TestEstimateUnattachedEBS_PricesByType(t *testing.T) {
	vols := []*VolumeView{
		{AvailableCount: 3, AvailableGiB: 300, AvailableByType: []VolumeTypeView{
			{VolumeType: "io2", Count: 1, SizeGiB: 100},
			{VolumeType: "sc1", Count: 2, SizeGiB: 200},
		}},
		{AvailableCount: 1, AvailableGiB: 50}, // no breakdown → gp3 default
		{AvailableCount: 0, AvailableGiB: 0},
		nil,
	}
	total, byType := EstimateUnattachedEBS(vols)
	// io2 100*0.125=12.5, sc1 200*0.015=3, unknown 50*0.08=4
	if total != 19.5 {
		t.Fatalf("total = %v, want 19.5", total)
	}
	if len(byType) != 3 || byType[0].VolumeType != "io2" || byType[0].EstimatedMonthlyUSD != 12.5 {
		t.Fatalf("unexpected breakdown: %+v", byType)
	}
	last := byType[2]
	if last.VolumeType != "sc1" || last.Count != 2 {
		t.Fatalf("expected sc1 last (cheapest), got %+v", last)
	}
}

func TestEstimateUnattachedEBS_PartialBreakdown(t *testing.T) {
	// Breakdown covers 40 of 100 GiB; the remainder is priced as unknown.
	vols := []*VolumeView{{AvailableCount: 2, AvailableGiB: 100, AvailableByType: []VolumeTypeView{{VolumeType: "gp2", Count: 1, SizeGiB: 40}}}}
	total, byType := EstimateUnattachedEBS(vols)
	if total != 8.8 { // 40*0.10 + 60*0.08
		t.Fatalf("total = %v, want 8.8", total)
	}
	if len(byType) != 2 {
		t.Fatalf("expected gp2 + unknown, got %+v", byType)
	}
}
