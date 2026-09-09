import type { PeriodPreset } from "../api";
import type {
  DashboardResponse,
  SuggestionsResponse,
  TrendsResponse,
} from "../types";
import {
  formatCurrency,
  formatServiceSpendDelta,
  formatSignedCurrency,
  formatSignedPercent,
  formatSpendChangeDetail,
  formatServiceName,
  spendDirection,
} from "./format";
import { periodLabel, priorCompareLabel } from "./periodLabels";

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
  suggestions: SuggestionsResponse | null,
  period: PeriodPreset = "30d"
): BillingHighlight[] {
  if (!dashboard) return [];

  const items: BillingHighlight[] = [];
  const totals = dashboard.totals;
  const top = dashboard.top_services?.[0];
  const compareLabel = priorCompareLabel(period);
  const activePeriodLabel = periodLabel(period, trends?.current_period?.days);

  items.push({
    id: "org-spend",
    kind: "spend",
    title: "Org spend",
    value: formatCurrency(totals.org_total),
    detail: `${activePeriodLabel} · ${totals.account_count} accounts · usage (excl. credits)`,
    askQuestion:
      "What is our organization spend this period, and what are the top cost drivers?",
  });

  const orgChange = trends?.org_total;
  if (orgChange && orgChange.prior_usd > 0) {
    const dir = spendDirection(orgChange.change_usd, orgChange.change_percent);
    const title =
      dir === "flat"
        ? "Spend about flat"
        : dir === "up"
          ? "Higher spend"
          : "Lower spend";
    items.push({
      id: "org-trend",
      kind: "trend",
      title,
      value:
        dir === "flat"
          ? "About flat"
          : `${formatSignedCurrency(orgChange.change_usd)} · ${formatSignedPercent(orgChange.change_percent)}`,
      detail: `${formatCurrency(orgChange.prior_usd)} → ${formatCurrency(orgChange.current_usd)} · ${formatSpendChangeDetail(orgChange.change_usd, orgChange.change_percent)} ${compareLabel}`,
      askQuestion:
        dir === "down"
          ? "What decreased in our AWS spend vs the prior period?"
          : "How did our organization spend change vs the prior period? Which services increased the most?",
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
      detail: `${gib.toFixed(0)} GiB · ~${formatCurrency(est)}/mo potential`,
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
      detail: formatServiceSpendDelta(topSpike.change_usd, topSpike.change_percent),
      askQuestion: `What caused ${name} spend to increase ${formatSignedPercent(topSpike.change_percent)}? Which accounts or resources drove it?`,
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
          ? `Est. ~${formatCurrency(topSuggestion.estimated_monthly_usd)}/mo potential · not yet realized`
          : `${topSuggestion.category} · investigate (no $ estimate)`,
      askQuestion: `Tell me more about this savings opportunity: "${topSuggestion.title}". What should we do first and what's the impact?`,
    });
  }

  return items.slice(0, 6);
}
