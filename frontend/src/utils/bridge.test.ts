import { describe, expect, it } from "vitest";
import type { TrendsResponse } from "../types";
import { bridgeDomain, buildBridge } from "./bridge";

function trends(prior: number, current: number, services: [string, number][]): TrendsResponse {
  return {
    generated_at: "",
    current_period: { start: "", end: "" },
    prior_period: { start: "", end: "" },
    prior_source: "history_snapshot",
    ce_calls_used: 0,
    org_total: { prior_usd: prior, current_usd: current, change_usd: current - prior, change_percent: 0 },
    service_trends: services.map(([name, change]) => ({
      service: name,
      display_name: name,
      current_usd: 100 + change,
      prior_usd: 100,
      change_usd: change,
      change_percent: change,
      direction: change > 0 ? "up" : "down",
    })),
    top_increases: [],
    top_decreases: [],
    account_trends: [],
    top_account_increases: [],
    top_account_decreases: [],
  };
}

describe("buildBridge", () => {
  it("returns null without a prior period", () => {
    expect(buildBridge(null)).toBeNull();
    expect(buildBridge(trends(0, 100, []))).toBeNull();
  });

  it("orders by size of change and closes the gap with Other services", () => {
    // Org moved +5760 but listed services only explain +4800 (small ones are filtered by the API).
    const steps = buildBridge(trends(66240, 72000, [["S3", 960], ["EC2", 1936], ["RDS", -736], ["Glue", 2640]]))!;
    expect(steps.map((s) => s.label)).toEqual(["Prior period", "Glue", "EC2", "S3", "RDS", "Other services", "This period"]);
    const other = steps.find((s) => s.key === "other")!;
    expect(other.value).toBeCloseTo(960);
    const lastDelta = steps[steps.length - 2];
    expect(lastDelta.end).toBeCloseTo(72000);
    const rds = steps.find((s) => s.label === "RDS")!;
    expect(rds.end).toBeLessThan(rds.start);
  });

  it("folds services beyond maxSteps into Other services", () => {
    const steps = buildBridge(trends(1000, 1060, [["a", 10], ["b", 20], ["c", 30]]), 2)!;
    expect(steps.map((s) => s.label)).toEqual(["Prior period", "c", "b", "Other services", "This period"]);
    expect(steps[3].value).toBeCloseTo(10);
  });

  it("omits Other services when the listed services explain everything", () => {
    const steps = buildBridge(trends(1000, 1030, [["a", 10], ["b", 20]]))!;
    expect(steps.some((s) => s.key === "other")).toBe(false);
  });
});

describe("bridgeDomain", () => {
  it("pads the running-total range", () => {
    const steps = buildBridge(trends(1000, 1100, [["a", 100]]))!;
    const [lo, hi] = bridgeDomain(steps);
    expect(lo).toBeLessThan(1000);
    expect(hi).toBeGreaterThan(1100);
  });
});
