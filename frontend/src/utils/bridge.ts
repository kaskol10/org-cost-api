import type { TrendsResponse } from "../types";

export interface BridgeStep {
  key: string;
  label: string;
  /** "total" rows are the prior / current period totals; "delta" rows move the running total. */
  kind: "total" | "delta";
  /** Total for "total" rows; signed change for "delta" rows. */
  value: number;
  /** Running total before / after this row (equal for totals). */
  start: number;
  end: number;
  /** Raw service name (Cost Explorer SERVICE) for delta rows backed by a service. */
  service?: string;
}

/**
 * Builds a cost bridge (waterfall) from the prior period total to the
 * current one: the `maxSteps` services that moved most, then "Other
 * services" for the remainder (smaller services the API leaves out of
 * service_trends), so the steps always add up to the real org change.
 */
export function buildBridge(trends: TrendsResponse | null | undefined, maxSteps = 6): BridgeStep[] | null {
  const org = trends?.org_total;
  if (!trends || !org || !(org.prior_usd > 0)) return null;

  const moved = [...(trends.service_trends ?? [])]
    .filter((s) => Math.abs(s.change_usd) >= 0.5)
    .sort((a, b) => Math.abs(b.change_usd) - Math.abs(a.change_usd));
  const shown = moved.slice(0, maxSteps);
  const shownChange = shown.reduce((sum, s) => sum + s.change_usd, 0);
  const rest = org.change_usd - shownChange;

  const steps: BridgeStep[] = [
    { key: "prior", label: "Prior period", kind: "total", value: org.prior_usd, start: org.prior_usd, end: org.prior_usd },
  ];
  let running = org.prior_usd;
  for (const s of shown) {
    steps.push({
      key: `svc-${s.service}`,
      label: s.display_name || s.service,
      kind: "delta",
      value: s.change_usd,
      start: running,
      end: running + s.change_usd,
      service: s.service,
    });
    running += s.change_usd;
  }
  if (Math.abs(rest) >= 0.5) {
    steps.push({
      key: "other",
      label: "Other services",
      kind: "delta",
      value: rest,
      start: running,
      end: running + rest,
    });
    running += rest;
  }
  steps.push({ key: "current", label: "This period", kind: "total", value: org.current_usd, start: org.current_usd, end: org.current_usd });
  return steps;
}

/** Domain for the bridge's value axis: the range the running total spans, padded. */
export function bridgeDomain(steps: BridgeStep[]): [number, number] {
  const values = steps.flatMap((s) => [s.start, s.end]);
  const lo = Math.min(...values);
  const hi = Math.max(...values);
  const pad = Math.max((hi - lo) * 0.12, Math.abs(hi) * 0.002, 1);
  return [lo - pad, hi + pad];
}
