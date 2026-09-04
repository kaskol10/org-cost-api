import type {
  DashboardResponse,
  SuggestionsResponse,
  TrendsResponse,
} from "../types";
import { formatCurrency, formatPercent, formatServiceName } from "./format";

export type HighlightKind = "spend" | "trend" | "waste" | "savings" | "concentration";

export interface BillingHighlight {
  id: string;
  kind: HighlightKind;
  title: string;
  value: string;
  detail: string;
  askQuestion: string;
}

export function buildBillingHighlights(
  dashboard: DashboardResponse | null,
  trends: TrendsResponse | null,
  suggestions: SuggestionsResponse | null
): BillingHighlight[] {
  if (!dashboard) return [];

  const items: BillingHighlight[] = [];
  const totals = dashboard.totals;
  const top = dashboard.top_services?.[0];

  items.push({
    id: "org-spend",
    kind: "spend",
    title: "Org spend",
    value: formatCurrency(totals.org_total),
    detail: `${totals.account_count} accounts · usage (excl. credits)`,
    askQuestion:
      "What is our organization spend this period, and what are the top cost drivers?",
  });

  const orgChange = trends?.org_total;
  if (orgChange && orgChange.prior_usd > 0) {
    const up = orgChange.change_percent > 0;
    items.push({
      id: "org-trend",
      kind: "trend",
      title: up ? "Spend increased" : "Spend decreased",
      value: formatPercent(orgChange.change_percent),
      detail: `${formatCurrency(orgChange.prior_usd)} → ${formatCurrency(orgChange.current_usd)}`,
      askQuestion: up
        ? "How did our organization spend change vs the prior period? Which services increased the most?"
        : "What decreased in our AWS spend vs the prior period?",
    });
  }

  if (top && totals.org_total > 0) {
    const share = (top.amount / totals.org_total) * 100;
    items.push({
      id: "top-service",
      kind: "concentration",
      title: "Top service",
      value: formatServiceName(top.service),
      detail: `${formatCurrency(top.amount)} · ${share.toFixed(0)}% of org`,
      askQuestion: `Why is ${formatServiceName(top.service)} a top cost driver? Break it down by account and region.`,
    });
  }

  if ((totals.volume_available_count ?? 0) > 0) {
    const gib = totals.volume_available_gib ?? 0;
    const est = gib * 0.08;
    items.push({
      id: "ebs-waste",
      kind: "waste",
      title: "Unattached EBS",
      value: `${totals.volume_available_count} disks`,
      detail: `${gib.toFixed(0)} GiB · ~${formatCurrency(est)}/mo est.`,
      askQuestion:
        "Which accounts have unattached EBS volumes, how much are we wasting, and what should we delete or attach first?",
    });
  }

  const topSpike = trends?.top_increases?.[0];
  if (topSpike && topSpike.change_percent >= 25 && topSpike.current_usd >= 50) {
    const name = topSpike.display_name || formatServiceName(topSpike.service);
    items.push({
      id: `spike-${topSpike.service}`,
      kind: "trend",
      title: "Biggest spike",
      value: name,
      detail: `${formatPercent(topSpike.change_percent)} · ${formatCurrency(topSpike.change_usd)}`,
      askQuestion: `What caused ${name} spend to increase ${formatPercent(topSpike.change_percent)}? Which accounts or resources drove it?`,
    });
  }

  const topSuggestion = suggestions?.suggestions?.[0];
  if (topSuggestion) {
    items.push({
      id: `saving-${topSuggestion.id}`,
      kind: "savings",
      title: "Top savings opportunity",
      value: topSuggestion.title,
      detail:
        topSuggestion.estimated_monthly_usd != null && topSuggestion.estimated_monthly_usd > 0
          ? `~${formatCurrency(topSuggestion.estimated_monthly_usd)}/mo potential`
          : topSuggestion.category,
      askQuestion: `Tell me more about this savings opportunity: "${topSuggestion.title}". What should we do first and what's the impact?`,
    });
  }

  return items.slice(0, 6);
}
