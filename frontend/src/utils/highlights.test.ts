import { describe, expect, it } from "vitest";
import type { DashboardResponse, SuggestionsResponse, TrendsResponse } from "../types";
import { buildBillingHighlights } from "./highlights";

const DASHBOARD: DashboardResponse = {
  generated_at: "2026-01-01T00:00:00Z",
  start: "2026-01-01",
  end: "2026-01-31",
  accounts: [],
  totals: {
    org_total: 10000,
    ec2_other_cost: 2000,
    unit: "USD",
    volume_count: 10,
    volume_available_count: 3,
    volume_available_gib: 120,
    volume_size_gib: 500,
    snapshot_count: 2,
    snapshot_size_gib: 50,
    account_count: 4,
    volume_usage_coverage_percent: 0,
    volume_available_est_monthly_usd: 9.6,
  },
  top_services: [{ service: "AmazonEC2", amount: 4000, unit: "USD" }],
  cur_enabled: false,
};

const TRENDS: TrendsResponse = {
  generated_at: "2026-01-01T00:00:00Z",
  current_period: { start: "2026-01-01", end: "2026-01-31" },
  prior_period: { start: "2025-12-01", end: "2025-12-31" },
  prior_source: "ce",
  ce_calls_used: 1,
  org_total: {
    current_usd: 10000,
    prior_usd: 8000,
    change_usd: 2000,
    change_percent: 25,
  },
  top_increases: [
    {
      service: "AmazonRedshift",
      display_name: "Redshift",
      current_usd: 500,
      prior_usd: 200,
      change_usd: 300,
      change_percent: 150,
      direction: "up",
    },
  ],
  top_decreases: [],
  service_trends: [],
  account_trends: [],
  top_account_increases: [],
  top_account_decreases: [],
};

const SUGGESTIONS: SuggestionsResponse = {
  generated_at: "2026-01-01T00:00:00Z",
  period: "2026-01",
  suggestions: [
    {
      id: "ebs-waste",
      priority: 1,
      category: "waste",
      title: "Delete unattached volumes",
      detail: "3 volumes",
      estimated_monthly_usd: 50,
      actions: ["Review in console"],
    },
  ],
  summary: "Some savings",
  ce_calls_used: 0,
  data_sources: ["dashboard"],
};

describe("buildBillingHighlights", () => {
  it("returns empty when dashboard is null", () => {
    expect(buildBillingHighlights(null, TRENDS, SUGGESTIONS)).toEqual([]);
  });

  it("builds spend, trend, concentration, waste, spike, and savings highlights", () => {
    const items = buildBillingHighlights(DASHBOARD, TRENDS, SUGGESTIONS, "mtd");
    const ids = items.map((h) => h.id);
    expect(ids).toContain("org-spend");
    expect(ids).toContain("org-trend");
    expect(ids).toContain("top-service");
    expect(ids).toContain("ebs-waste");
    expect(ids).toContain("spike-AmazonRedshift");
    expect(ids).toContain("saving-ebs-waste");
    expect(items.length).toBeLessThanOrEqual(6);

    const trend = items.find((h) => h.id === "org-trend")!;
    expect(trend.title).toBe("Higher spend");
    expect(trend.value).toContain("+$");
    expect(trend.detail).toContain("vs same days last month");

    const savings = items.find((h) => h.id === "saving-ebs-waste")!;
    expect(savings.detail).toMatch(/potential/i);
    expect(savings.detail).toMatch(/not yet realized/i);

    for (const item of items) {
      expect(item.askQuestion.length).toBeGreaterThan(10);
    }
  });

  it("omits trend when prior spend is zero", () => {
    const trends: TrendsResponse = {
      ...TRENDS,
      org_total: { current_usd: 100, prior_usd: 0, change_usd: 100, change_percent: 100 },
    };
    const items = buildBillingHighlights(DASHBOARD, trends, null);
    expect(items.some((h) => h.id === "org-trend")).toBe(false);
  });
});

describe("fromServerHighlights", () => {
  it("maps snake_case API highlights to the UI shape", async () => {
    const { fromServerHighlights } = await import("./highlights");
    const items = fromServerHighlights([
      { id: "org-spend", kind: "spend", title: "Org spend", value: "$1.00", detail: "d", ask_question: "q?" },
    ]);
    expect(items).toEqual([
      { id: "org-spend", kind: "spend", title: "Org spend", value: "$1.00", detail: "d", askQuestion: "q?" },
    ]);
  });

  it("fallback builder prefers the server per-type EBS estimate", () => {
    const dash = {
      ...DASHBOARD,
      totals: { ...DASHBOARD.totals, volume_available_count: 2, volume_available_gib: 100, volume_available_est_monthly_usd: 12.5 },
    };
    const waste = buildBillingHighlights(dash, null, null).find((h) => h.id === "ebs-waste");
    expect(waste?.detail).toContain("$12.50");
  });
});
