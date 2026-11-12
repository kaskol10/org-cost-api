import { describe, expect, it } from "vitest";
import type { AccountDashboard, AccountTrend } from "../types";
import { buildAccountRows, sortAccountRows } from "./accountsTable";

const acct = (id: string, name: string, total: number, extra: Partial<AccountDashboard> = {}): AccountDashboard =>
  ({ account_id: id, account_name: name, costs: total ? ({ all_total: total } as AccountDashboard["costs"]) : undefined, ...extra }) as AccountDashboard;

const trend = (id: string, prior: number, current: number): AccountTrend => ({
  account_id: id,
  account_name: id,
  prior_usd: prior,
  current_usd: current,
  change_usd: current - prior,
  change_percent: prior ? ((current - prior) / prior) * 100 : 0,
  direction: current >= prior ? "up" : "down",
});

describe("accounts table", () => {
  const rows = buildAccountRows(
    [
      acct("1", "production", 600, { unattached_ebs_est_monthly_usd: 12.5 }),
      acct("2", "staging", 300),
      acct("3", "broken", 0, { error: "AccessDenied" }),
      acct("4", "new-acct", 100),
    ],
    [trend("1", 500, 600), trend("2", 400, 300)],
    1000
  );

  it("joins dashboard spend, trend change and waste", () => {
    expect(rows[0]).toMatchObject({ name: "production", spend: 600, change: 100, share: 60, wasteMonthly: 12.5 });
    expect(rows[3]).toMatchObject({ name: "new-acct", change: null, changePct: null });
  });

  it("sorts by change, missing values and errors last", () => {
    const sorted = sortAccountRows(rows, "change", "desc").map((r) => r.name);
    expect(sorted).toEqual(["production", "staging", "new-acct", "broken"]);
    expect(sortAccountRows(rows, "change", "asc").map((r) => r.name)).toEqual(["staging", "production", "new-acct", "broken"]);
  });

  it("sorts by name", () => {
    expect(sortAccountRows(rows, "name", "asc").map((r) => r.name)).toEqual(["new-acct", "production", "staging", "broken"]);
  });
});
