package handlers

import (
	"time"

	"github.com/kaskol10/org-cost-api/backend/internal/analysis"
	"github.com/kaskol10/org-cost-api/backend/internal/service"
)

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func stampDashboardGeneratedAt(d *service.DashboardResponse) {
	if d != nil {
		d.GeneratedAt = nowRFC3339()
	}
}

func stampTrendsGeneratedAt(t *analysis.TrendsResponse) {
	if t != nil {
		t.GeneratedAt = nowRFC3339()
	}
}

func stampSuggestionsGeneratedAt(s *analysis.SuggestionsResponse) {
	if s != nil {
		s.GeneratedAt = nowRFC3339()
	}
}

func stampTagDeltaGeneratedAt(t *analysis.ServiceTagDeltaResponse) {
	if t != nil {
		t.GeneratedAt = nowRFC3339()
	}
}

func stampTagTotalsGeneratedAt(t *analysis.ServiceTagTotalsResponse) {
	if t != nil {
		t.GeneratedAt = nowRFC3339()
	}
}

func stampReportGeneratedAt(r *service.ReportResponse) {
	if r == nil {
		return
	}
	now := nowRFC3339()
	if r.Dashboard != nil {
		r.Dashboard.GeneratedAt = now
	}
	if r.Trends != nil {
		r.Trends.GeneratedAt = now
	}
	if r.Suggestions != nil {
		r.Suggestions.GeneratedAt = now
	}
}
