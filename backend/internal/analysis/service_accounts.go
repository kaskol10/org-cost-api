package analysis

import (
	"math"
	"sort"

	"github.com/kaskol10/org-cost-api/backend/internal/history"
)

// ServiceAccountChange is how one account's spend on a service moved.
type ServiceAccountChange struct {
	AccountID   string  `json:"account_id"`
	AccountName string  `json:"account_name"`
	CurrentUSD  float64 `json:"current_usd"`
	PriorUSD    float64 `json:"prior_usd"`
	ChangeUSD   float64 `json:"change_usd"`
}

// MaxServiceAccounts caps the per-service account breakdown.
const MaxServiceAccounts = 5

// PriorAccountServices maps account ID → service → prior spend.
type PriorAccountServices map[string]map[string]float64

// PriorAccountServicesFromSnapshot reads per-account service spend from a snapshot.
func PriorAccountServicesFromSnapshot(snap *history.Snapshot) PriorAccountServices {
	out := make(PriorAccountServices, len(snap.Accounts))
	for _, a := range snap.Accounts {
		if len(a.Services) == 0 {
			continue
		}
		m := make(map[string]float64, len(a.Services))
		for _, s := range a.Services {
			m[s.Service] += s.Amount
		}
		out[a.AccountID] = m
	}
	return out
}

// AttachServiceAccounts fills ServiceTrend.Accounts — which accounts drove
// each service's change — from the current dashboard and prior per-account
// service spend. It costs no Cost Explorer calls. With no prior breakdown
// (e.g. a Cost Explorer prior that is not split by account) it does nothing,
// and clients fall back to /service-tag-delta.
func AttachServiceAccounts(resp *TrendsResponse, view DashboardView, prior PriorAccountServices) {
	if resp == nil || len(prior) == 0 {
		return
	}
	names := make(map[string]string, len(view.Accounts))
	current := make(PriorAccountServices, len(view.Accounts))
	for _, a := range view.Accounts {
		names[a.AccountID] = a.AccountName
		m := make(map[string]float64, len(a.ByService))
		for _, s := range a.ByService {
			m[s.Service] += s.Amount
		}
		current[a.AccountID] = m
	}

	breakdown := func(service string) []ServiceAccountChange {
		ids := map[string]struct{}{}
		for id, m := range current {
			if _, ok := m[service]; ok {
				ids[id] = struct{}{}
			}
		}
		for id, m := range prior {
			if _, ok := m[service]; ok {
				ids[id] = struct{}{}
			}
		}
		var out []ServiceAccountChange
		for id := range ids {
			cur, pri := current[id][service], prior[id][service]
			if math.Abs(cur-pri) < 0.005 && cur < 0.005 {
				continue
			}
			name := names[id]
			if name == "" {
				name = id
			}
			out = append(out, ServiceAccountChange{AccountID: id, AccountName: name, CurrentUSD: cur, PriorUSD: pri, ChangeUSD: cur - pri})
		}
		sort.Slice(out, func(i, j int) bool {
			ai, aj := math.Abs(out[i].ChangeUSD), math.Abs(out[j].ChangeUSD)
			if ai != aj {
				return ai > aj
			}
			return out[i].AccountName < out[j].AccountName
		})
		if len(out) > MaxServiceAccounts {
			out = out[:MaxServiceAccounts]
		}
		return out
	}

	cache := map[string][]ServiceAccountChange{}
	fill := func(list []ServiceTrend) {
		for i := range list {
			svc := list[i].Service
			if _, ok := cache[svc]; !ok {
				cache[svc] = breakdown(svc)
			}
			list[i].Accounts = cache[svc]
		}
	}
	fill(resp.ServiceTrends)
	fill(resp.TopIncreases)
	fill(resp.TopDecreases)
}

// DemoPriorScale is the synthetic prior/current ratio demo data uses for
// account i's j-th service: mostly −8%, with some bigger moves and a few
// decreases so drill-downs show movers in both directions.
func DemoPriorScale(accountIdx, serviceIdx int) float64 {
	switch (accountIdx + serviceIdx) % 4 {
	case 1:
		return 0.85
	case 2:
		return 1.04
	default:
		return 0.92
	}
}
