package handlers

import (
	"encoding/json"
	"testing"

	"github.com/kaskol10/org-cost-api/backend/internal/analysis"
	"github.com/kaskol10/org-cost-api/backend/internal/service"
)

func TestReportResponseJSONShape(t *testing.T) {
	resp := service.ReportResponse{
		Dashboard: &service.DashboardResponse{
			Start: "2026-07-11",
			End:   "2026-08-10",
			Totals: service.ConsolidatedTotals{
				OrgTotal: 1000,
				Unit:     "USD",
			},
		},
		Trends: &analysis.TrendsResponse{
			PriorSource: "history_snapshot",
		},
		Suggestions: &analysis.SuggestionsResponse{
			Summary: "2 optimization opportunities ranked by impact.",
		},
		CECallsUsed:    0,
		RefreshAllowed: true,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"dashboard", "trends", "suggestions", "ce_calls_used", "refresh_allowed"} {
		if _, ok := m[key]; !ok {
			t.Fatalf("missing key %q in report JSON", key)
		}
	}
}
