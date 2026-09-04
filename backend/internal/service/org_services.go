package service

import "sort"

const orgTopServicesLimit = 20
const orgTopAccountsPerService = 3

// OrgServiceDriver is a service ranked by org-wide usage cost.
type OrgServiceDriver struct {
	Service     string                   `json:"service"`
	Amount      float64                  `json:"amount"`
	Unit        string                   `json:"unit"`
	Percent     float64                  `json:"percent,omitempty"`
	TopAccounts []OrgServiceAccountShare `json:"top_accounts,omitempty"`
}

// OrgServiceAccountShare is one account's share of a service's org cost.
type OrgServiceAccountShare struct {
	AccountID   string  `json:"account_id"`
	AccountName string  `json:"account_name"`
	Amount      float64 `json:"amount"`
	Percent     float64 `json:"percent,omitempty"`
}

func buildOrgTopServices(accounts []AccountDashboard, orgTotal float64, unit string) []OrgServiceDriver {
	byService := make(map[string]float64)
	byServiceAcct := make(map[string]map[string]OrgServiceAccountShare)

	for _, ad := range accounts {
		if ad.Costs == nil {
			continue
		}
		for _, s := range ad.Costs.ByService {
			if s.Amount <= 0 {
				continue
			}
			svc := s.Service
			byService[svc] += s.Amount
			if byServiceAcct[svc] == nil {
				byServiceAcct[svc] = make(map[string]OrgServiceAccountShare)
			}
			prev := byServiceAcct[svc][ad.AccountID]
			prev.AccountID = ad.AccountID
			prev.AccountName = ad.AccountName
			prev.Amount += s.Amount
			byServiceAcct[svc][ad.AccountID] = prev
		}
	}

	out := make([]OrgServiceDriver, 0, len(byService))
	for svc, amt := range byService {
		pct := 0.0
		if orgTotal > 0 {
			pct = (amt / orgTotal) * 100
		}
		driver := OrgServiceDriver{
			Service: svc,
			Amount:  amt,
			Unit:    unit,
			Percent: pct,
		}
		if accts, ok := byServiceAcct[svc]; ok && len(accts) > 0 {
			driver.TopAccounts = topAccountShares(accts, amt, orgTopAccountsPerService)
		}
		out = append(out, driver)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Amount > out[j].Amount })
	if len(out) > orgTopServicesLimit {
		out = out[:orgTopServicesLimit]
	}
	return out
}

func topAccountShares(accts map[string]OrgServiceAccountShare, serviceTotal float64, limit int) []OrgServiceAccountShare {
	list := make([]OrgServiceAccountShare, 0, len(accts))
	for _, a := range accts {
		pct := 0.0
		if serviceTotal > 0 {
			pct = (a.Amount / serviceTotal) * 100
		}
		a.Percent = pct
		list = append(list, a)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Amount > list[j].Amount })
	if limit > 0 && len(list) > limit {
		list = list[:limit]
	}
	return list
}
