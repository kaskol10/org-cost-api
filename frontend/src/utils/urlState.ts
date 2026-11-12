import { useCallback, useEffect, useState } from "react";
import type { PeriodPreset } from "../api";

/**
 * Page state that belongs in the URL so any view can be shared:
 *   ?period=mtd&service=Amazon%20RDS&account=111111111111
 * Defaults (30d, nothing selected) are left out to keep links short.
 */
export interface UrlState {
  period: PeriodPreset;
  /** Selected cost-bridge row: a Cost Explorer service name, or OTHER_SERVICES. */
  service: string | null;
  /** Expanded account in the accounts table (account ID). */
  account: string | null;
}

export const OTHER_SERVICES = "__other__";

export function readUrlState(search: string): UrlState {
  const q = new URLSearchParams(search);
  const period = q.get("period") === "mtd" ? "mtd" : "30d";
  const clean = (v: string | null) => (v && v.trim() ? v.trim() : null);
  return { period, service: clean(q.get("service")), account: clean(q.get("account")) };
}

/** Returns the query string (with leading "?", or "" when everything is default). */
export function writeUrlState(state: UrlState, existing = ""): string {
  const q = new URLSearchParams(existing);
  const set = (key: string, value: string | null) => (value ? q.set(key, value) : q.delete(key));
  set("period", state.period === "30d" ? null : state.period);
  set("service", state.service);
  set("account", state.account);
  const s = q.toString();
  return s ? `?${s}` : "";
}

/**
 * URL-backed state. Updates push a history entry so Back undoes a drill-down;
 * Back/Forward (popstate) flow back into React state.
 */
export function useUrlState(): [UrlState, (patch: Partial<UrlState>) => void] {
  const [state, setState] = useState<UrlState>(() => readUrlState(window.location.search));

  useEffect(() => {
    const onPop = () => setState(readUrlState(window.location.search));
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, []);

  const update = useCallback((patch: Partial<UrlState>) => {
    setState((prev) => {
      const next = { ...prev, ...patch };
      const search = writeUrlState(next, window.location.search);
      if (search !== window.location.search) {
        window.history.pushState(null, "", `${window.location.pathname}${search}${window.location.hash}`);
      }
      return next;
    });
  }, []);

  return [state, update];
}
