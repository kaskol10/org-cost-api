package analysis

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Highlight is one headline billing fact with a suggested follow-up question.
// This used to be computed in the web UI (frontend/src/utils/highlights.ts);
// computing it here gives every API consumer the same cards.
type Highlight struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"` // spend | trend | waste | savings | concentration | budget
	Title       string `json:"title"`
	Value       string `json:"value"`
	Detail      string `json:"detail"`
	AskQuestion string `json:"ask_question"`
}

// MaxHighlights caps the number of highlight cards returned.
const MaxHighlights = 6

// HighlightInput is everything BuildHighlights and BuildHeadline read.
type HighlightInput struct {
	Period                string // "30d" | "mtd"
	OrgTotal              float64
	AccountCount          int
	TopService            *ServiceDriverView
	VolumeAvailableCount  int
	VolumeAvailableGiB    float64
	VolumeWasteMonthlyUSD float64
	Trends                *TrendsResponse
	Suggestions           *SuggestionsResponse
	Forecast              *ForecastResult
	Budgets               []*BudgetStatus
}

// BuildHighlights returns up to MaxHighlights billing highlights.
func BuildHighlights(in HighlightInput) []Highlight {
	items := make([]Highlight, 0, MaxHighlights+1)
	compare := priorCompareLabel(in.Period)
	days := 0
	if in.Trends != nil {
		days = in.Trends.CurrentPeriod.Days
	}
	active := periodLabel(in.Period, days)

	if b := primaryBudget(in.Budgets); in.Forecast != nil && b != nil && b.Status != "ok" {
		title := "Near budget"
		if b.Status == "over" {
			title = "Over budget"
		}
		detail := fmt.Sprintf("Projected %s EOM vs %s budget", formatUSD(in.Forecast.ProjectedUSD), formatUSD(b.MonthlyUSD))
		if b.Status == "over" && b.OverByUSD > 0 {
			detail += fmt.Sprintf(" · %s over", formatUSD(b.OverByUSD))
		}
		items = append(items, Highlight{
			ID:          "budget-" + b.Name,
			Kind:        "budget",
			Title:       title,
			Value:       fmt.Sprintf("%d%% of %s", int(math.Round(b.PercentOfBudget)), b.Name),
			Detail:      detail,
			AskQuestion: "We're over budget for the month. What's driving the projected overrun and what should we cut or defer?",
		})
	}

	items = append(items, Highlight{
		ID:          "org-spend",
		Kind:        "spend",
		Title:       "Org spend",
		Value:       formatUSD(in.OrgTotal),
		Detail:      fmt.Sprintf("%s · %d accounts · usage (excl. credits)", active, in.AccountCount),
		AskQuestion: "What is our organization spend this period, and what are the top cost drivers?",
	})

	if in.Trends != nil && in.Trends.OrgTotal.PriorUSD > 0 {
		c := in.Trends.OrgTotal
		dir := spendDirection(c.ChangeUSD, c.ChangePercent)
		h := Highlight{ID: "org-trend", Kind: "trend"}
		switch dir {
		case "flat":
			h.Title, h.Value = "Spend about flat", "About flat"
		case "up":
			h.Title = "Higher spend"
		default:
			h.Title = "Lower spend"
		}
		if dir != "flat" {
			h.Value = formatSignedUSD(c.ChangeUSD) + " · " + formatSignedPercent(c.ChangePercent)
		}
		h.Detail = fmt.Sprintf("%s → %s · %s %s", formatUSD(c.PriorUSD), formatUSD(c.CurrentUSD), spendChangeDetail(c.ChangeUSD, c.ChangePercent), compare)
		if dir == "down" {
			h.AskQuestion = "What decreased in our AWS spend vs the prior period?"
		} else {
			h.AskQuestion = "How did our organization spend change vs the prior period? Which services increased the most?"
		}
		items = append(items, h)
	}

	if in.TopService != nil && in.OrgTotal > 0 {
		name := ServiceShortName(in.TopService.Service)
		share := in.TopService.Amount / in.OrgTotal * 100
		items = append(items, Highlight{
			ID:          "top-service",
			Kind:        "concentration",
			Title:       "Top service",
			Value:       name,
			Detail:      fmt.Sprintf("%s · %.0f%% of org", formatUSD(in.TopService.Amount), share),
			AskQuestion: fmt.Sprintf("Why is %s a top cost driver? Break it down by account and region.", name),
		})
	}

	if in.VolumeAvailableCount > 0 {
		items = append(items, Highlight{
			ID:          "ebs-waste",
			Kind:        "waste",
			Title:       "Unattached EBS",
			Value:       fmt.Sprintf("%d disks", in.VolumeAvailableCount),
			Detail:      fmt.Sprintf("%.0f GiB · ~%s/mo potential", in.VolumeAvailableGiB, formatUSD(in.VolumeWasteMonthlyUSD)),
			AskQuestion: "Which accounts have unattached EBS volumes, how much are we wasting, and what should we delete or attach first?",
		})
	}

	if s := topSpike(in.Trends); s != nil {
		name := s.DisplayName
		if name == "" {
			name = ServiceShortName(s.Service)
		}
		items = append(items, Highlight{
			ID:          "spike-" + s.Service,
			Kind:        "trend",
			Title:       "Biggest spike",
			Value:       name,
			Detail:      serviceSpendDelta(s.ChangeUSD, s.ChangePercent),
			AskQuestion: fmt.Sprintf("What caused %s spend to increase %s? Which accounts or resources drove it?", name, formatSignedPercent(s.ChangePercent)),
		})
	}

	if in.Suggestions != nil && len(in.Suggestions.Suggestions) > 0 {
		top := in.Suggestions.Suggestions[0]
		detail := top.Category + " · investigate (no $ estimate)"
		if top.EstimatedMonthlyUSD != nil && *top.EstimatedMonthlyUSD > 0 {
			detail = fmt.Sprintf("Est. ~%s/mo potential · not yet realized", formatUSD(*top.EstimatedMonthlyUSD))
		}
		items = append(items, Highlight{
			ID:          "saving-" + top.ID,
			Kind:        "savings",
			Title:       "Top savings opportunity",
			Value:       top.Title,
			Detail:      detail,
			AskQuestion: fmt.Sprintf("Tell me more about this savings opportunity: \"%s\". What should we do first and what's the impact?", top.Title),
		})
	}

	if len(items) > MaxHighlights {
		items = items[:MaxHighlights]
	}
	return items
}

// BuildHeadline returns a one-sentence, rule-based summary of the period, e.g.
// "Last 30 days: $184,220.00 across 5 accounts, +$12,410.00 (+7%) vs prior
// period. Biggest increase: NAT Gateway (+$7,900.00)." No LLM involved, so
// Slack bots, MCP answers and email digests can all reuse it verbatim.
func BuildHeadline(in HighlightInput) string {
	days := 0
	if in.Trends != nil {
		days = in.Trends.CurrentPeriod.Days
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %s across %d account", periodLabel(in.Period, days), formatUSD(in.OrgTotal), in.AccountCount)
	if in.AccountCount != 1 {
		b.WriteString("s")
	}
	if in.Trends != nil && in.Trends.OrgTotal.PriorUSD > 0 {
		c := in.Trends.OrgTotal
		if spendDirection(c.ChangeUSD, c.ChangePercent) == "flat" {
			fmt.Fprintf(&b, ", about flat %s", priorCompareLabel(in.Period))
		} else {
			fmt.Fprintf(&b, ", %s (%s) %s", formatSignedUSD(c.ChangeUSD), formatSignedPercent(c.ChangePercent), priorCompareLabel(in.Period))
		}
	}
	b.WriteString(".")
	if in.Trends != nil && len(in.Trends.TopIncreases) > 0 && in.Trends.TopIncreases[0].ChangeUSD > 0 {
		t := in.Trends.TopIncreases[0]
		name := t.DisplayName
		if name == "" {
			name = ServiceShortName(t.Service)
		}
		fmt.Fprintf(&b, " Biggest increase: %s (%s).", name, formatSignedUSD(t.ChangeUSD))
	}
	return b.String()
}

func primaryBudget(budgets []*BudgetStatus) *BudgetStatus {
	for _, b := range budgets {
		if b != nil && b.Account == "" {
			return b
		}
	}
	if len(budgets) > 0 {
		return budgets[0]
	}
	return nil
}

func topSpike(t *TrendsResponse) *ServiceTrend {
	if t == nil || len(t.TopIncreases) == 0 {
		return nil
	}
	s := t.TopIncreases[0]
	if s.ChangePercent >= 25 && s.CurrentUSD >= 50 {
		return &s
	}
	return nil
}

func periodLabel(period string, days int) string {
	if period == "mtd" {
		return "Month to date"
	}
	if days > 0 {
		return fmt.Sprintf("Last %d days", days)
	}
	return "Last 30 days"
}

func priorCompareLabel(period string) string {
	if period == "mtd" {
		return "vs same days last month"
	}
	return "vs prior period"
}

// spendDirection classifies a move; tiny moves count as flat.
func spendDirection(changeUSD, changePct float64) string {
	if math.Abs(changeUSD) < 1 && math.Abs(changePct) < 0.5 {
		return "flat"
	}
	if changeUSD > 0 || (changeUSD == 0 && changePct > 0) {
		return "up"
	}
	if changeUSD < 0 || changePct < 0 {
		return "down"
	}
	return "flat"
}

func spendChangeDetail(changeUSD, changePct float64) string {
	switch spendDirection(changeUSD, changePct) {
	case "flat":
		return "about flat vs prior"
	case "up":
		return fmt.Sprintf("%s higher spend (%s)", formatSignedUSD(changeUSD), formatSignedPercent(changePct))
	default:
		return fmt.Sprintf("%s lower spend (%s)", formatSignedUSD(changeUSD), formatSignedPercent(changePct))
	}
}

func serviceSpendDelta(changeUSD, changePct float64) string {
	switch spendDirection(changeUSD, changePct) {
	case "flat":
		return fmt.Sprintf("flat (%s)", formatSignedPercent(changePct))
	case "up":
		return fmt.Sprintf("drove %s (%s)", formatSignedUSD(changeUSD), formatSignedPercent(changePct))
	default:
		return fmt.Sprintf("saved %s (%s)", formatUSD(math.Abs(changeUSD)), formatSignedPercent(changePct))
	}
}

// formatUSD matches the UI's Intl.NumberFormat en-US currency output: $1,234.50.
func formatUSD(v float64) string {
	neg := v < 0
	s := strconv.FormatFloat(math.Abs(v), 'f', 2, 64)
	intPart, frac := s, ""
	if i := strings.IndexByte(s, '.'); i >= 0 {
		intPart, frac = s[:i], s[i:]
	}
	out := "$" + groupThousands(intPart) + frac
	if neg {
		return "-" + out
	}
	return out
}

func formatSignedUSD(v float64) string {
	switch {
	case v > 0:
		return "+" + formatUSD(v)
	case v < 0:
		return "-" + formatUSD(-v)
	default:
		return formatUSD(0)
	}
}

func formatSignedPercent(v float64) string {
	abs := groupThousands(strconv.FormatFloat(math.Round(math.Abs(v)), 'f', 0, 64)) + "%"
	switch {
	case v > 0:
		return "+" + abs
	case v < 0:
		return "-" + abs
	default:
		return abs
	}
}

func groupThousands(digits string) string {
	if len(digits) <= 3 {
		return digits
	}
	var b strings.Builder
	pre := len(digits) % 3
	if pre > 0 {
		b.WriteString(digits[:pre])
	}
	for i := pre; i < len(digits); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(digits[i : i+3])
	}
	return b.String()
}

// ServiceShortName mirrors the UI's formatServiceName for Cost Explorer
// SERVICE dimension values.
func ServiceShortName(service string) string {
	s := strings.TrimSpace(service)
	if s == "" {
		return "Unknown"
	}
	exact := map[string]string{
		"Amazon Simple Storage Service":          "S3",
		"AWS Data Transfer":                      "Data transfer",
		"Amazon Redshift":                        "Redshift",
		"Amazon Relational Database Service":     "RDS",
		"Amazon Elastic Compute Cloud - Compute": "EC2",
		"Amazon Elastic Kubernetes Service":      "EKS",
		"Elastic Load Balancing":                 "ELB",
		"Amazon CloudWatch":                      "CloudWatch",
		"AWS Lambda":                             "Lambda",
		"Amazon Kinesis":                         "Kinesis",
	}
	if v, ok := exact[s]; ok {
		return v
	}
	contains := []struct{ sub, name string }{
		{"Amazon Simple Storage Service", "S3"},
		{"Amazon Kinesis", "Kinesis"},
		{"Amazon Redshift", "Redshift"},
		{"Relational Database Service", "RDS"},
		{"CloudWatch", "CloudWatch"},
		{"Elastic Load Balancing", "ELB"},
		{"Kubernetes Service", "EKS"},
		{"Data Transfer", "Data transfer"},
	}
	for _, c := range contains {
		if strings.Contains(s, c.sub) {
			return c.name
		}
	}
	return s
}
