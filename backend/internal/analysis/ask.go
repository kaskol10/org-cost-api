package analysis

import (
	"fmt"
	"regexp"
	"strings"
)

// AskIntent identifies how a natural-language question should be answered.
type AskIntent string

const (
	AskIntentOrgSummary  AskIntent = "org_summary"
	AskIntentTrends      AskIntent = "trends"
	AskIntentSuggestions AskIntent = "suggestions"
	AskIntentWaste       AskIntent = "waste"
	AskIntentAccount     AskIntent = "account"
	AskIntentFallback    AskIntent = "fallback"
)

// AskRoute is the result of routing a user question.
type AskRoute struct {
	Intent  AskIntent
	Params  map[string]string
	Service string // optional service filter for trends
	Account string // optional account name/id for account intent
}

// RouteQuestion maps a plain-language question to an intent (rule-based, no LLM).
func RouteQuestion(question string, accountNames []string) AskRoute {
	q := strings.ToLower(strings.TrimSpace(question))
	if q == "" {
		return AskRoute{Intent: AskIntentFallback}
	}

	// Waste signals — require EBS/storage cleanup context (not bare "snapshot").
	if strings.Contains(q, "unattached") ||
		(strings.Contains(q, "ebs") && strings.Contains(q, "volume")) ||
		(strings.Contains(q, "ebs") && strings.Contains(q, "snapshot")) ||
		strings.Contains(q, "old snapshot") || strings.Contains(q, "unused snapshot") ||
		strings.Contains(q, "delete snapshot") {
		return AskRoute{Intent: AskIntentWaste}
	}

	// Savings / optimization.
	if strings.Contains(q, "save") || strings.Contains(q, "optim") || strings.Contains(q, "waste") ||
		strings.Contains(q, "reduce") {
		return AskRoute{Intent: AskIntentSuggestions}
	}

	// Trends / changes (whole-word matching avoids "backup", "update", etc.).
	trendWords := []string{
		"change", "changed", "increase", "decrease", "vs last", "compared",
		"trend", "spike", "went up", "going up", "went down", "going down",
	}
	for _, w := range trendWords {
		if containsPhrase(q, w) {
			svc := extractService(q)
			return AskRoute{Intent: AskIntentTrends, Service: svc}
		}
	}
	if ContainsWord(q, "up") || ContainsWord(q, "down") {
		svc := extractService(q)
		return AskRoute{Intent: AskIntentTrends, Service: svc}
	}

	// Account-specific: match configured account names (no extra keywords required).
	for _, name := range accountNames {
		if ContainsWord(q, strings.ToLower(name)) {
			return AskRoute{Intent: AskIntentAccount, Account: name}
		}
	}
	if ContainsWord(q, "account") && ContainsWord(q, "breakdown") {
		return AskRoute{Intent: AskIntentAccount}
	}

	// Org summary.
	summaryWords := []string{"total spend", "top service", "cost driver", "how much", "org spend", "organization spend", "top cost"}
	for _, w := range summaryWords {
		if strings.Contains(q, w) {
			return AskRoute{Intent: AskIntentOrgSummary}
		}
	}

	return AskRoute{Intent: AskIntentFallback}
}

var wordToken = regexp.MustCompile(`[\w-]+`)

// ContainsWord reports whether text contains word as a whole token (letters, digits, hyphens).
func ContainsWord(text, word string) bool {
	if word == "" {
		return false
	}
	for _, tok := range wordToken.FindAllString(text, -1) {
		if strings.EqualFold(tok, word) {
			return true
		}
	}
	return false
}

func containsPhrase(text, phrase string) bool {
	if phrase == "" {
		return false
	}
	if strings.Contains(phrase, " ") {
		return strings.Contains(text, phrase)
	}
	return ContainsWord(text, phrase)
}

func extractService(q string) string {
	known := []struct {
		key  string
		name string
	}{
		{"redshift", "Amazon Redshift"},
		{"s3", "Amazon Simple Storage Service"},
		{"rds", "Amazon Relational Database Service"},
		{"ec2", "Amazon Elastic Compute Cloud - Compute"},
		{"lambda", "AWS Lambda"},
		{"kinesis", "Amazon Kinesis"},
		{"cloudwatch", "AmazonCloudWatch"},
		{"data transfer", "AWS Data Transfer"},
	}
	for _, k := range known {
		if strings.Contains(q, k.key) {
			return k.name
		}
	}
	return ""
}

// FormatOrgSummaryAnswer builds markdown from dashboard view data.
func FormatOrgSummaryAnswer(dash DashboardView) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Organization spend: **$%.0f** (%s to %s)\n\n", dash.Totals.OrgTotal, dash.Start, dash.End)
	if len(dash.TopServices) > 0 {
		b.WriteString("Top cost drivers:\n")
		for i, s := range dash.TopServices {
			if i >= 8 {
				break
			}
			name := friendlyService(s.Service)
			pct := 0.0
			if dash.Totals.OrgTotal > 0 {
				pct = (s.Amount / dash.Totals.OrgTotal) * 100
			}
			fmt.Fprintf(&b, "- %s: $%.0f (%.0f%%)\n", name, s.Amount, pct)
		}
	}
	if dash.Totals.VolumeAvailable > 0 {
		fmt.Fprintf(&b, "\nWaste signal: **%d unattached EBS volumes** across the org.\n", dash.Totals.VolumeAvailable)
	}
	return b.String()
}

// FormatTrendsAnswer builds markdown from trends response.
func FormatTrendsAnswer(trends *TrendsResponse, serviceFilter string) string {
	if trends == nil {
		return "Trend data is not available yet. Run the dashboard daily to build history snapshots."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Period: %s to %s vs %s to %s\n",
		trends.CurrentPeriod.Start, trends.CurrentPeriod.End,
		trends.PriorPeriod.Start, trends.PriorPeriod.End)
	fmt.Fprintf(&b, "Org total: $%.0f → $%.0f (%+.0f%%, source: %s)\n\n",
		trends.OrgTotal.PriorUSD, trends.OrgTotal.CurrentUSD,
		trends.OrgTotal.ChangePercent, trends.PriorSource)

	list := trends.TopIncreases
	label := "Top increases"
	if serviceFilter != "" {
		key := strings.ToLower(serviceFilter)
		var filtered []ServiceTrend
		for _, t := range trends.ServiceTrends {
			if strings.Contains(strings.ToLower(t.Service), key) ||
				strings.Contains(strings.ToLower(t.DisplayName), key) {
				filtered = append(filtered, t)
			}
		}
		if len(filtered) > 0 {
			list = filtered
			label = "Matching services"
		}
	}
	if len(list) > 0 {
		b.WriteString(label + ":\n")
		for i, t := range list {
			if i >= 8 {
				break
			}
			fmt.Fprintf(&b, "- %s: $%.0f → $%.0f (%+.0f%%)\n",
				t.DisplayName, t.PriorUSD, t.CurrentUSD, t.ChangePercent)
		}
	}
	if trends.HistoryNote != "" {
		fmt.Fprintf(&b, "\n_%s_\n", trends.HistoryNote)
	}
	return b.String()
}

// FormatSuggestionsAnswer builds markdown from suggestions response.
func FormatSuggestionsAnswer(sug *SuggestionsResponse) string {
	if sug == nil || len(sug.Suggestions) == 0 {
		return "No optimization suggestions right now — keep monitoring trends weekly."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", sug.Summary)
	for i, s := range sug.Suggestions {
		if i >= 8 {
			break
		}
		line := fmt.Sprintf("%d. [%s] %s", s.Priority, s.Category, s.Title)
		if s.EstimatedMonthlyUSD != nil {
			line += fmt.Sprintf(" (~$%.0f/mo potential)", *s.EstimatedMonthlyUSD)
		}
		b.WriteString(line + "\n")
		if s.Detail != "" {
			fmt.Fprintf(&b, "   %s\n", s.Detail)
		}
	}
	return b.String()
}

// FormatWasteAnswer builds markdown from dashboard waste signals.
func FormatWasteAnswer(dash DashboardView) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Unattached EBS volumes: **%d** disks", dash.Totals.VolumeAvailable)
	if dash.Totals.VolumeSizeGiB > 0 {
		fmt.Fprintf(&b, " (%.0f GiB total provisioned across org)", dash.Totals.VolumeSizeGiB)
	}
	b.WriteString("\n\n")
	if dash.Totals.VolumeAvailable == 0 {
		b.WriteString("No unattached EBS volumes detected in live inventory.\n")
		return b.String()
	}
	b.WriteString("Accounts with unattached disks:\n")
	type row struct {
		name  string
		count int
		gib   float64
	}
	var rows []row
	for _, acct := range dash.Accounts {
		if acct.Volumes == nil || acct.Volumes.AvailableCount == 0 {
			continue
		}
		rows = append(rows, row{acct.AccountName, acct.Volumes.AvailableCount, acct.Volumes.AvailableGiB})
	}
	for i, r := range rows {
		if i >= 10 {
			break
		}
		fmt.Fprintf(&b, "- %s: %d volumes (%.0f GiB unattached)\n", r.name, r.count, r.gib)
	}
	est := estimateUnattachedEBSMonthly(dash)
	fmt.Fprintf(&b, "\nEstimated unattached EBS cost: ~$%.0f/month (gp3 ballpark).\n", est)
	return b.String()
}

// FormatAccountAnswer builds markdown for one account.
func FormatAccountAnswer(acct AccountView, periodStart, periodEnd string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "**%s** (%s)\n", acct.AccountName, acct.AccountID)
	fmt.Fprintf(&b, "Period: %s to %s\n", periodStart, periodEnd)
	fmt.Fprintf(&b, "All services: $%.0f | EC2-Other: $%.0f | Other services: $%.0f\n\n",
		acct.AllTotal, acct.AllTotal-acct.OtherServicesTotal, acct.OtherServicesTotal)
	if len(acct.ByService) > 0 {
		b.WriteString("Top services:\n")
		for i, s := range acct.ByService {
			if i >= 8 {
				break
			}
			fmt.Fprintf(&b, "- %s: $%.0f\n", friendlyService(s.Service), s.Amount)
		}
	}
	if acct.Volumes != nil && acct.Volumes.AvailableCount > 0 {
		fmt.Fprintf(&b, "\nUnattached EBS: %d volumes (%.0f GiB)\n",
			acct.Volumes.AvailableCount, acct.Volumes.AvailableGiB)
	}
	return b.String()
}

// FormatFallbackAnswer lists supported question types.
func FormatFallbackAnswer(accountNames []string) string {
	var b strings.Builder
	b.WriteString("I can answer questions about:\n")
	b.WriteString("- Organization spend and top cost drivers\n")
	b.WriteString("- Spend changes vs prior period (trends)\n")
	b.WriteString("- Cost optimization suggestions\n")
	b.WriteString("- Unattached EBS volumes and snapshots\n")
	if len(accountNames) > 0 {
		b.WriteString("- Per-account breakdowns (try: \"")
		b.WriteString(accountNames[0])
		b.WriteString(" account costs\")\n")
	}
	b.WriteString("\nExample: \"What are our top cost drivers?\" or \"Where can we save money?\"\n")
	return b.String()
}
