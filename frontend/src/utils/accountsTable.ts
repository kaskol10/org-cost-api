import type { AccountDashboard, AccountTrend } from "../types";

export interface AccountRow {
  id: string;
  name: string;
  error?: string;
  spend: number;
  prior: number | null;
  change: number | null;
  changePct: number | null;
  share: number;
  wasteMonthly: number;
}

export type SortKey = "name" | "spend" | "change" | "changePct" | "share" | "waste";
export type SortDir = "asc" | "desc";

/** One row per account: spend and waste from the dashboard, change from trends. */
export function buildAccountRows(
  accounts: AccountDashboard[],
  accountTrends: AccountTrend[] | undefined,
  orgTotal: number
): AccountRow[] {
  const trendById = new Map((accountTrends ?? []).map((t) => [t.account_id, t]));
  return accounts.map((a) => {
    const t = trendById.get(a.account_id);
    const spend = a.costs?.all_total ?? 0;
    const hasPrior = !!t && t.prior_usd > 0;
    return {
      id: a.account_id,
      name: a.account_name,
      error: a.error || undefined,
      spend,
      prior: t ? t.prior_usd : null,
      change: t ? t.change_usd : null,
      changePct: hasPrior ? t!.change_percent : null,
      share: orgTotal > 0 ? (spend / orgTotal) * 100 : 0,
      wasteMonthly: a.unattached_ebs_est_monthly_usd ?? 0,
    };
  });
}

function value(row: AccountRow, key: SortKey): number | string | null {
  switch (key) {
    case "name":
      return row.name.toLowerCase();
    case "spend":
      return row.spend;
    case "change":
      return row.change;
    case "changePct":
      return row.changePct;
    case "share":
      return row.share;
    case "waste":
      return row.wasteMonthly;
  }
}

/** Sorts rows; accounts with errors and missing values always go last. */
export function sortAccountRows(rows: AccountRow[], key: SortKey, dir: SortDir): AccountRow[] {
  const sign = dir === "asc" ? 1 : -1;
  return [...rows].sort((a, b) => {
    if (!!a.error !== !!b.error) return a.error ? 1 : -1;
    const va = value(a, key);
    const vb = value(b, key);
    if (va === null && vb === null) return a.name.localeCompare(b.name);
    if (va === null) return 1;
    if (vb === null) return -1;
    if (va < vb) return -1 * sign;
    if (va > vb) return 1 * sign;
    return a.name.localeCompare(b.name);
  });
}
