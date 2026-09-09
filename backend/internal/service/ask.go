package service

import (
	"context"
	"strings"

	"github.com/kaskol10/org-cost-api/backend/internal/analysis"
	"github.com/kaskol10/org-cost-api/backend/internal/aws/costexplorer"
	"github.com/kaskol10/org-cost-api/backend/internal/aws/ec2volumes"
	appconfig "github.com/kaskol10/org-cost-api/backend/internal/config"
)

// AskResponse is the answer to a natural-language cost question.
type AskResponse struct {
	Answer      string   `json:"answer"`
	Intent      string   `json:"intent"`
	CECallsUsed int      `json:"ce_calls_used"`
	Sources     []string `json:"sources"`
}

// AskRequest is the JSON body for POST /api/ask.
type AskRequest struct {
	Question string `json:"question"`
	Refresh  bool   `json:"refresh"`
}

// AccountCostsResponse is a single-account slice from the dashboard cache.
type AccountCostsResponse struct {
	AccountID   string                      `json:"account_id"`
	AccountName string                      `json:"account_name"`
	Period      analysis.PeriodSummary      `json:"period"`
	Costs       any                         `json:"costs"`
	Volumes     any                         `json:"volumes,omitempty"`
	Snapshots   any                         `json:"snapshots,omitempty"`
}

// AccountCosts returns one account's costs from the cached dashboard.
func (a *Aggregator) AccountCosts(ctx context.Context, account string, force bool) (*AccountCostsResponse, error) {
	dash, err := a.dashboard(ctx, force, appconfig.PeriodLookback)
	if err != nil {
		return nil, err
	}
	key := strings.TrimSpace(account)
	keyLower := strings.ToLower(key)
	for _, acct := range dash.Accounts {
		if acct.AccountID == key || strings.EqualFold(acct.AccountName, keyLower) {
			return &AccountCostsResponse{
				AccountID:   acct.AccountID,
				AccountName: acct.AccountName,
				Period: analysis.PeriodSummary{
					Start: dash.Start,
					End:   dash.End,
				},
				Costs:     acct.Costs,
				Volumes:   acct.Volumes,
				Snapshots: acct.Snapshots,
			}, nil
		}
	}
	return nil, NewUnknownAccountError(account)
}

// Ask answers a plain-language question using cached report data.
func (a *Aggregator) Ask(ctx context.Context, question string, force bool) (*AskResponse, error) {
	accountNames := make([]string, 0, len(a.clients))
	for _, c := range a.clients {
		accountNames = append(accountNames, c.Account.Name)
	}
	route := analysis.RouteQuestion(question, accountNames)

	resp := &AskResponse{
		Intent:  string(route.Intent),
		Sources: []string{"dashboard_cache"},
	}

	switch route.Intent {
	case analysis.AskIntentOrgSummary, analysis.AskIntentWaste:
		dash, err := a.dashboard(ctx, force, appconfig.PeriodLookback)
		if err != nil {
			return nil, err
		}
		view := toDashboardView(dash)
		if route.Intent == analysis.AskIntentOrgSummary {
			resp.Answer = analysis.FormatOrgSummaryAnswer(view)
		} else {
			resp.Answer = analysis.FormatWasteAnswer(view)
		}

	case analysis.AskIntentTrends:
		trends, err := a.Trends(ctx, force, "")
		if err != nil {
			return nil, err
		}
		resp.CECallsUsed = trends.CECallsUsed
		resp.Answer = analysis.FormatTrendsAnswer(trends, route.Service)
		if trends.PriorSource == "history_snapshot" {
			resp.Sources = append(resp.Sources, "history_snapshot")
		}

	case analysis.AskIntentSuggestions:
		suggestions, err := a.Suggestions(ctx, force, "")
		if err != nil {
			return nil, err
		}
		resp.CECallsUsed = suggestions.CECallsUsed
		resp.Answer = analysis.FormatSuggestionsAnswer(suggestions)
		resp.Sources = append(resp.Sources, "trends_cache")

	case analysis.AskIntentAccount:
		acctName := route.Account
		if acctName == "" {
			// Try to extract from question.
			for _, name := range accountNames {
				if analysis.ContainsWord(strings.ToLower(question), strings.ToLower(name)) {
					acctName = name
					break
				}
			}
		}
		if acctName == "" {
			resp.Intent = string(analysis.AskIntentFallback)
			resp.Answer = analysis.FormatFallbackAnswer(accountNames)
			return resp, nil
		}
		costs, err := a.AccountCosts(ctx, acctName, force)
		if err != nil {
			return nil, err
		}
		acctView := accountViewFromCosts(costs)
		resp.Answer = analysis.FormatAccountAnswer(acctView, costs.Period.Start, costs.Period.End)

	default:
		resp.Answer = analysis.FormatFallbackAnswer(accountNames)
	}

	return resp, nil
}

func accountViewFromCosts(resp *AccountCostsResponse) analysis.AccountView {
	av := analysis.AccountView{
		AccountID:   resp.AccountID,
		AccountName: resp.AccountName,
	}
	if cs, ok := resp.Costs.(*costexplorer.CostSummary); ok && cs != nil {
		av.AllTotal = cs.AllTotal
		av.OtherServicesTotal = cs.OtherServicesTotal
		for _, s := range cs.ByService {
			av.ByService = append(av.ByService, analysis.ServiceDriverView{
				Service: s.Service,
				Amount:  s.Amount,
			})
		}
	}
	if vol, ok := resp.Volumes.(*ec2volumes.Inventory); ok && vol != nil {
		av.Volumes = &analysis.VolumeView{
			Count:          vol.Count,
			AvailableCount: vol.AvailableCount,
			AvailableGiB:   vol.AvailableGiB,
			TotalGiB:       vol.TotalGiB,
		}
	}
	return av
}
