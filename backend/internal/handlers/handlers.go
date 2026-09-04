package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/kaskol10/org-cost-api/backend/internal/service"
)

type API struct {
	agg        *service.Aggregator
	corsOrigin string
	apiToken   string
}

func New(agg *service.Aggregator, corsOrigin, apiToken string) *API {
	return &API{agg: agg, corsOrigin: corsOrigin, apiToken: apiToken}
}

func (a *API) Routes(staticDir string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", a.health)
	mux.HandleFunc("GET /api/ready", a.ready)
	mux.HandleFunc("GET /api/meta", a.meta)
	mux.HandleFunc("GET /api/accounts", a.accounts)
	mux.HandleFunc("GET /api/dashboard", a.dashboard)
	mux.HandleFunc("GET /api/trends", a.trends)
	mux.HandleFunc("GET /api/suggestions", a.suggestions)
	mux.HandleFunc("GET /api/report", a.report)
	mux.HandleFunc("GET /api/account-costs", a.accountCosts)
	mux.HandleFunc("POST /api/ask", a.ask)
	mux.HandleFunc("GET /api/service-detail", a.serviceDetail)
	mux.HandleFunc("GET /api/service-tag-delta", a.serviceTagDelta)
	mux.HandleFunc("GET /api/service-tag-totals", a.serviceTagTotals)
	handler := a.withCORS(withBearerAuth(a.apiToken, mux))
	if staticDir != "" {
		return withStaticSPA(handler, staticDir)
	}
	return handler
}

func (a *API) meta(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"demo": a.agg.DemoMode(),
	})
}

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) accounts(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.agg.Accounts())
}

func (a *API) dashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	refresh := strings.TrimSpace(r.URL.Query().Get("refresh"))
	var (
		data *service.DashboardResponse
		err  error
	)
	if refresh != "" && refresh != "0" && refresh != "false" {
		data, err = a.agg.DashboardFresh(ctx)
	} else {
		data, err = a.agg.Dashboard(ctx)
	}
	if err != nil {
		writeAPIError(w, err)
		return
	}
	stampDashboardGeneratedAt(data)
	writeJSON(w, http.StatusOK, data)
}

func (a *API) trends(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	refresh := strings.TrimSpace(r.URL.Query().Get("refresh"))
	force := refresh != "" && refresh != "0" && refresh != "false"
	data, err := a.agg.Trends(ctx, force)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	stampTrendsGeneratedAt(data)
	writeJSON(w, http.StatusOK, data)
}

func (a *API) suggestions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	refresh := strings.TrimSpace(r.URL.Query().Get("refresh"))
	force := refresh != "" && refresh != "0" && refresh != "false"
	data, err := a.agg.Suggestions(ctx, force)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	stampSuggestionsGeneratedAt(data)
	writeJSON(w, http.StatusOK, data)
}

func (a *API) report(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	refresh := strings.TrimSpace(r.URL.Query().Get("refresh"))
	force := refresh != "" && refresh != "0" && refresh != "false"
	data, err := a.agg.Report(ctx, force)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	stampReportGeneratedAt(data)
	writeJSON(w, http.StatusOK, data)
}

func (a *API) serviceDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	accountID := strings.TrimSpace(q.Get("account_id"))
	serviceName := strings.TrimSpace(q.Get("service"))
	start := strings.TrimSpace(q.Get("start"))
	end := strings.TrimSpace(q.Get("end"))
	if accountID == "" || serviceName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "account_id and service are required"})
		return
	}
	data, err := a.agg.ServiceDetail(ctx, accountID, serviceName, start, end)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (a *API) serviceTagDelta(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()

	accountID := strings.TrimSpace(q.Get("account_id"))
	serviceName := strings.TrimSpace(q.Get("service"))
	start := strings.TrimSpace(q.Get("start"))
	end := strings.TrimSpace(q.Get("end"))
	tagKey := strings.TrimSpace(q.Get("tag_key"))
	if tagKey == "" {
		tagKey = "Name"
	}

	topAccounts := 5
	if v := strings.TrimSpace(q.Get("top_accounts")); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			topAccounts = parsed
			if topAccounts > 25 {
				topAccounts = 25
			}
		}
	}

	if serviceName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "service is required"})
		return
	}

	data, err := a.agg.ServiceTagDelta(ctx, accountID, serviceName, start, end, tagKey, topAccounts)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	stampTagDeltaGeneratedAt(data)
	writeJSON(w, http.StatusOK, data)
}

func (a *API) serviceTagTotals(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()

	accountID := strings.TrimSpace(q.Get("account_id"))
	serviceName := strings.TrimSpace(q.Get("service"))
	start := strings.TrimSpace(q.Get("start"))
	end := strings.TrimSpace(q.Get("end"))
	tagKey := strings.TrimSpace(q.Get("tag_key"))
	if tagKey == "" {
		tagKey = "Name"
	}

	topBuckets := 10
	if v := strings.TrimSpace(q.Get("top_buckets")); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			topBuckets = parsed
			if topBuckets > 50 {
				topBuckets = 50
			}
		}
	}

	if serviceName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "service is required"})
		return
	}

	data, err := a.agg.ServiceTagTotals(ctx, accountID, serviceName, start, end, tagKey, topBuckets)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	stampTagTotalsGeneratedAt(data)
	writeJSON(w, http.StatusOK, data)
}

func (a *API) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", a.corsOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		log.Printf("encode json: %v", err)
	}
}
