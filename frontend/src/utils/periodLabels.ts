import type { PeriodPreset } from "../api";

export function periodLabel(period: PeriodPreset, days?: number): string {
  if (period === "mtd") return "Month to date";
  if (days && days > 0) return `Last ${days} days`;
  return "Last 30 days";
}

export function priorCompareLabel(period: PeriodPreset): string {
  return period === "mtd" ? "vs same days last month" : "vs prior period";
}
