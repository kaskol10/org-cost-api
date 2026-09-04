import type { Suggestion } from "../types";

export type SuggestionCategory =
  | "waste"
  | "rightsizing"
  | "trend"
  | "concentration"
  | "visibility";

export const CATEGORY_META: Record<
  SuggestionCategory,
  { label: string; hint: string; cssClass: string }
> = {
  waste: {
    label: "Waste",
    hint: "Removable spend — disks, idle resources",
    cssClass: "suggestion-cat-waste",
  },
  rightsizing: {
    label: "Rightsizing",
    hint: "Under-used provisioned capacity",
    cssClass: "suggestion-cat-rightsizing",
  },
  trend: {
    label: "Trend",
    hint: "Spend increased vs prior period",
    cssClass: "suggestion-cat-trend",
  },
  concentration: {
    label: "Concentration",
    hint: "One service dominates org spend",
    cssClass: "suggestion-cat-concentration",
  },
  visibility: {
    label: "Visibility",
    hint: "Review non-EC2 spend drivers",
    cssClass: "suggestion-cat-visibility",
  },
};

const CATEGORY_ORDER: SuggestionCategory[] = [
  "waste",
  "rightsizing",
  "trend",
  "concentration",
  "visibility",
];

export function normalizeCategory(raw: string): SuggestionCategory {
  const key = raw.toLowerCase() as SuggestionCategory;
  return key in CATEGORY_META ? key : "visibility";
}

export interface CategoryRollup {
  category: SuggestionCategory;
  label: string;
  hint: string;
  cssClass: string;
  count: number;
  quantifiedSavingsUsd: number;
}

export interface SuggestionsSummary {
  totalCount: number;
  quantifiedCount: number;
  totalQuantifiedSavingsUsd: number;
  byCategory: CategoryRollup[];
}

export function summarizeSuggestions(items: Suggestion[]): SuggestionsSummary {
  const buckets = new Map<SuggestionCategory, CategoryRollup>();

  for (const cat of CATEGORY_ORDER) {
    const meta = CATEGORY_META[cat];
    buckets.set(cat, {
      category: cat,
      label: meta.label,
      hint: meta.hint,
      cssClass: meta.cssClass,
      count: 0,
      quantifiedSavingsUsd: 0,
    });
  }

  let quantifiedCount = 0;
  let totalQuantifiedSavingsUsd = 0;

  for (const item of items) {
    const cat = normalizeCategory(item.category);
    const bucket = buckets.get(cat)!;
    bucket.count += 1;
    if (item.estimated_monthly_usd != null && item.estimated_monthly_usd > 0) {
      bucket.quantifiedSavingsUsd += item.estimated_monthly_usd;
      quantifiedCount += 1;
      totalQuantifiedSavingsUsd += item.estimated_monthly_usd;
    }
  }

  const byCategory = CATEGORY_ORDER.map((c) => buckets.get(c)!).filter((b) => b.count > 0);

  return {
    totalCount: items.length,
    quantifiedCount,
    totalQuantifiedSavingsUsd,
    byCategory,
  };
}

export function groupSuggestionsByCategory(
  items: Suggestion[]
): { category: CategoryRollup; items: Suggestion[] }[] {
  const summary = summarizeSuggestions(items);
  const groups: { category: CategoryRollup; items: Suggestion[] }[] = [];

  for (const rollup of summary.byCategory) {
    const catItems = items.filter((i) => normalizeCategory(i.category) === rollup.category);
    catItems.sort((a, b) => a.priority - b.priority);
    groups.push({ category: rollup, items: catItems });
  }

  return groups;
}
