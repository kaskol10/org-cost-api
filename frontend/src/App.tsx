import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { fetchMeta, fetchReport, fetchReportFresh, fetchExplainedSuggestions, fetchChatHealth, chatEnabled, ApiError, type PeriodPreset } from "./api";
import AccountCard from "./components/AccountCard";
import ChatStatusBanner from "./components/ChatStatusBanner";
import AskPanel, { type AskRequest } from "./components/AskPanel";
import BillingHighlights from "./components/BillingHighlights";
import AskDrawer from "./components/AskDrawer";
import BudgetBar from "./components/BudgetBar";
import CostBridge from "./components/CostBridge";
import DailyCostChart from "./components/DailyCostChart";
import OrgTopServicesPanel from "./components/OrgTopServicesPanel";
import SpikesPanel from "./components/SpikesPanel";
import SuggestionsPanel from "./components/SuggestionsPanel";
import SummaryDashboard from "./components/SummaryDashboard";
import { buildBillingHighlights, fromServerHighlights } from "./utils/highlights";
import { formatCurrency, formatSignedCurrency } from "./utils/format";
import { periodLabel } from "./utils/periodLabels";
import type { BudgetStatus, DashboardResponse, DailyCost, ForecastResult, ReportResponse, ServerHighlight, SuggestionsResponse, TrendsResponse } from "./types";

function mergeDaily(
  accounts: DashboardResponse["accounts"] | undefined,
  field: "daily" | "all_daily"
): DailyCost[] {
  const byDate = new Map<string, number>();
  if (!Array.isArray(accounts)) return [];
  for (const acct of accounts) {
    if (!acct.costs) continue;
    const series = acct.costs[field];
    if (!Array.isArray(series)) continue;
    for (const d of series) {
      byDate.set(d.date, (byDate.get(d.date) ?? 0) + d.amount);
    }
  }
  return [...byDate.entries()]
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([date, amount]) => ({ date, amount, unit: "USD" }));
}

export default function App() {
  const [askOpen, setAskOpen] = useState(false);
  const [askRequest, setAskRequest] = useState<AskRequest | null>(null);
  const askIdRef = useRef(0);
  const [data, setData] = useState<DashboardResponse | null>(null);
  const [trends, setTrends] = useState<TrendsResponse | null>(null);
  const [suggestions, setSuggestions] = useState<SuggestionsResponse | null>(null);
  const [forecast, setForecast] = useState<ForecastResult | null>(null);
  const [budgets, setBudgets] = useState<BudgetStatus[] | null>(null);
  const [serverHighlights, setServerHighlights] = useState<ServerHighlight[] | null>(null);
  const [headline, setHeadline] = useState<string | null>(null);
  const [enrichingSuggestions, setEnrichingSuggestions] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [authError, setAuthError] = useState(false);
  const [loading, setLoading] = useState(true);
  const [showDetails, setShowDetails] = useState(false);
  const [demoMode, setDemoMode] = useState(false);
  const [demoBannerDismissed, setDemoBannerDismissed] = useState(false);
  const [period, setPeriod] = useState<PeriodPreset>("30d");

  useEffect(() => {
    fetchMeta()
      .then((m) => setDemoMode(m.demo))
      .catch(() => setDemoMode(false));
  }, []);

  const applySuggestions = useCallback(async (report: ReportResponse, refresh: boolean) => {
    setSuggestions(report.suggestions);
    if (!chatEnabled()) return;
    try {
      const health = await fetchChatHealth();
      if (health.suggestions_llm !== "true") return;
    } catch {
      return;
    }
    setEnrichingSuggestions(true);
    try {
      const enriched = await fetchExplainedSuggestions(refresh, report);
      setSuggestions(enriched);
    } catch {
      // Keep rule-based suggestions on LLM failure or misconfiguration.
    } finally {
      setEnrichingSuggestions(false);
    }
  }, []);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    setAuthError(false);
    try {
      const report = await fetchReport(period);
      setData(report.dashboard);
      setTrends(report.trends);
      setForecast(report.forecast ?? null);
      setBudgets(report.budgets ?? null);
      setServerHighlights(report.highlights ?? null);
      setHeadline(report.headline ?? null);
      setLoading(false);
      await applySuggestions(report, false);
    } catch (e) {
      const msg = e instanceof Error ? e.message : "Failed to load report";
      if (e instanceof ApiError && e.status === 401) {
        setAuthError(true);
      }
      setError(msg);
      setLoading(false);
    }
  }, [applySuggestions, period]);

  const refresh = useCallback(async () => {
    setLoading(true);
    setError(null);
    setAuthError(false);
    try {
      const report = await fetchReportFresh(period);
      setData(report.dashboard);
      setTrends(report.trends);
      setForecast(report.forecast ?? null);
      setBudgets(report.budgets ?? null);
      setServerHighlights(report.highlights ?? null);
      setHeadline(report.headline ?? null);
      setLoading(false);
      await applySuggestions(report, true);
    } catch (e) {
      const msg = e instanceof Error ? e.message : "Failed to load report";
      if (e instanceof ApiError && e.status === 401) {
        setAuthError(true);
      }
      setError(msg);
      setLoading(false);
    }
  }, [applySuggestions, period]);

  useEffect(() => {
    load();
  }, [load]);

  // Prefer server-computed series; fall back to merging client-side for older APIs.
  const consolidatedOrgDaily = useMemo(() => {
    if (Array.isArray(data?.org_daily)) return data.org_daily;
    return Array.isArray(data?.accounts) && data.accounts.length ? mergeDaily(data.accounts, "all_daily") : [];
  }, [data]);

  const consolidatedEC2Daily = useMemo(() => {
    if (Array.isArray(data?.org_ec2_other_daily)) return data.org_ec2_other_daily;
    return Array.isArray(data?.accounts) && data.accounts.length ? mergeDaily(data.accounts, "daily") : [];
  }, [data]);

  const dateRange =
    data &&
    `${new Date(data.start + "T00:00:00").toLocaleDateString()} – ${new Date(data.end + "T00:00:00").toLocaleDateString()}`;

  // Server-computed highlights keep the UI consistent with MCP and other API
  // consumers; the client-side builder is only a fallback for older APIs.
  const highlights = useMemo(
    () =>
      serverHighlights && serverHighlights.length > 0
        ? fromServerHighlights(serverHighlights)
        : buildBillingHighlights(data, trends, suggestions, period, forecast, budgets),
    [serverHighlights, data, trends, suggestions, period, forecast, budgets]
  );

  const topAccountsBySpend = useMemo(() => {
    if (!Array.isArray(data?.accounts) || !data.accounts.length) return [];
    return [...data.accounts]
      .filter((a) => a.costs && !a.error)
      .sort((a, b) => (b.costs?.all_total ?? 0) - (a.costs?.all_total ?? 0))
      .slice(0, 5);
  }, [data?.accounts]);

  const accountStrip = useMemo(() => {
    const increases = Array.isArray(trends?.top_account_increases)
      ? trends.top_account_increases
      : [];
    const decreases = Array.isArray(trends?.top_account_decreases)
      ? trends.top_account_decreases
      : [];
    const hasMovers =
      !!trends?.prior_source && (increases.length > 0 || decreases.length > 0);
    if (hasMovers) {
      return { mode: "movers" as const, increases, decreases };
    }
    return { mode: "spend" as const, accounts: topAccountsBySpend };
  }, [trends, topAccountsBySpend]);

  const handleAskAbout = useCallback((question: string) => {
    setAskOpen(true);
    askIdRef.current += 1;
    setAskRequest({ id: askIdRef.current, question });
  }, []);

  const clearAskRequest = useCallback(() => {
    setAskRequest(null);
  }, []);

  const closeAsk = useCallback(() => setAskOpen(false), []);

  const openAccountDetails = useCallback((accountId: string) => {
    setShowDetails(true);
    window.setTimeout(() => {
      document
        .getElementById(`account-${accountId}`)
        ?.scrollIntoView({ behavior: "smooth", block: "start" });
    }, 50);
  }, []);

  const headlineText =
    headline ??
    (data
      ? `${periodLabel(period, trends?.current_period?.days)}: ${formatCurrency(data.totals.org_total)} across ${data.totals.account_count} accounts.`
      : null);

  return (
    <>
      <header className="topbar">
        <div className="topbar-inner">
          <div className="topbar-brand">
            <span className="topbar-title">AWS Org Cost</span>
            {demoMode && <span className="cost-context-demo-chip">Demo data</span>}
          </div>
          <div className="topbar-actions">
            <div className="period-toggle" role="group" aria-label="Cost period">
              {(["30d", "mtd"] as const).map((p) => (
                <button
                  key={p}
                  type="button"
                  className={period === p ? "period-toggle-btn active" : "period-toggle-btn"}
                  aria-pressed={period === p}
                  onClick={() => setPeriod(p)}
                  disabled={loading}
                >
                  <span className="label-long">{p === "30d" ? "Last 30 days" : "Month to date"}</span>
                  <span className="label-short" aria-hidden="true">{p === "30d" ? "30d" : "MTD"}</span>
                </button>
              ))}
            </div>
            <button
              type="button"
              className="btn btn-ghost"
              onClick={refresh}
              disabled={loading}
              title="Fetch fresh numbers from AWS Cost Explorer (limited to once every 5 minutes)"
            >
              {loading ? "Refreshing…" : "Refresh"}
            </button>
            <button
              type="button"
              className="btn topbar-ask"
              onClick={() => setAskOpen((v) => !v)}
              aria-expanded={askOpen}
            >
              Ask
            </button>
          </div>
        </div>
      </header>

      {demoMode && !demoBannerDismissed && (
        <div className="demo-banner" role="status">
          <span>Demo data — not connected to AWS. Explore the UI with synthetic fixture costs.</span>
          <button type="button" className="demo-banner-dismiss" onClick={() => setDemoBannerDismissed(true)}>
            Dismiss
          </button>
        </div>
      )}

      <main className="app">
        {authError && (
          <div className="error">
            The API rejected the request (401): this deployment requires an API token. For local
            development set <code>VITE_API_TOKEN</code> to the server&apos;s token and restart Vite.
          </div>
        )}
        {error && !authError && <div className="error">{error}</div>}

        <section className="headline" aria-live="polite">
          {headlineText ? (
            <p className={`headline-text${loading ? " is-updating" : ""}`}>{headlineText}</p>
          ) : (
            <p className="headline-text muted">{loading ? "Loading your AWS spend…" : "No cost data yet."}</p>
          )}
          {data && (
            <p className="headline-meta">
              {dateRange}
              <span className="summary-dot">·</span>
              {data.totals.account_count} accounts
              <span className="summary-dot">·</span>
              updated {new Date(data.generated_at).toLocaleString()}
              {loading && <span className="headline-updating"> · updating…</span>}
            </p>
          )}
          <BudgetBar forecast={forecast} budgets={budgets} />
        </section>

        {data && (
          <>
            {data.cur_note && <p className="page-note">{data.cur_note}</p>}

            <CostBridge trends={trends} period={period} onAskAbout={handleAskAbout} />

            {highlights.length > 0 && (
              <BillingHighlights highlights={highlights} onAskAbout={handleAskAbout} compact />
            )}

            <section className="panel">
              <DailyCostChart
                data={consolidatedOrgDaily}
                title="Daily organization spend (all services, usage)"
                seriesName="Org spend"
                spikes={trends?.spikes}
                incompleteDays={data.incomplete_days ?? 0}
              />
            </section>

            {trends?.spikes && trends.spikes.length > 0 && (
              <SpikesPanel
                spikes={trends.spikes}
                accounts={data.accounts}
                spikeMeta={trends.spike_meta}
                onAskAbout={handleAskAbout}
              />
            )}

            {(accountStrip.mode === "movers" ||
              (accountStrip.mode === "spend" && accountStrip.accounts.length > 0)) && (
              <section
                className="top-accounts-strip"
                aria-label={
                  accountStrip.mode === "movers"
                    ? "Accounts with biggest spend changes"
                    : "Top accounts by spend"
                }
              >
                <h2 className="top-accounts-strip-title">
                  {accountStrip.mode === "movers" ? "Account movers" : "Top accounts by spend"}
                </h2>
                <ul className="top-accounts-strip-list">
                  {accountStrip.mode === "movers"
                    ? [
                        ...accountStrip.increases.map((t) => ({ t, up: true })),
                        ...accountStrip.decreases.map((t) => ({ t, up: false })),
                      ].map(({ t, up }) => (
                        <li key={`${up ? "up" : "down"}-${t.account_id}`}>
                          <button
                            type="button"
                            className={`top-accounts-chip ${up ? "top-accounts-chip-up" : "top-accounts-chip-down"}`}
                            onClick={() => openAccountDetails(t.account_id)}
                            title={`Show details for ${t.account_name}`}
                          >
                            <span className="top-accounts-chip-name">{t.account_name}</span>
                            <span className={`top-accounts-chip-amount ${up ? "up" : "down"}`}>
                              {formatSignedCurrency(t.change_usd)}
                            </span>
                          </button>
                        </li>
                      ))
                    : accountStrip.accounts.map((acct) => (
                        <li key={acct.account_id}>
                          <button
                            type="button"
                            className="top-accounts-chip"
                            onClick={() => openAccountDetails(acct.account_id)}
                            title={`Show details for ${acct.account_name}`}
                          >
                            <span className="top-accounts-chip-name">{acct.account_name}</span>
                            <span className="top-accounts-chip-amount">
                              {formatCurrency(acct.costs?.all_total ?? 0)}
                            </span>
                          </button>
                        </li>
                      ))}
                </ul>
              </section>
            )}

            <SummaryDashboard
              totals={data.totals}
              topServices={data.top_services ?? []}
              accounts={data.accounts}
              orgDaily={consolidatedOrgDaily}
              dateRange={dateRange ?? ""}
              trends={trends}
              trendsLoading={loading && !trends}
              period={period}
              commitments={data.commitments ?? null}
              tax={data.tax ?? null}
              hideHero
            />

            <SuggestionsPanel
              suggestions={suggestions}
              loading={loading && !suggestions}
              enriching={enrichingSuggestions}
              onAskAbout={handleAskAbout}
            />

            <OrgTopServicesPanel drivers={data.top_services ?? []} orgTotal={data.totals.org_total} />

            {consolidatedEC2Daily.length > 0 && (
              <section className="panel">
                <DailyCostChart
                  data={consolidatedEC2Daily}
                  title="Daily EC2-Other spend (all accounts)"
                  incompleteDays={data.incomplete_days ?? 0}
                />
              </section>
            )}

            <div className="details-toggle-row">
              <button
                type="button"
                className="btn btn-ghost"
                aria-expanded={showDetails}
                onClick={() => setShowDetails((v) => !v)}
              >
                {showDetails ? "Hide account details" : "Show account details"}
                <span className="details-toggle-hint">
                  {data.accounts.length} accounts · services, EC2-Other usage, storage inventory
                </span>
              </button>
            </div>

            {showDetails && (
              <section className="account-grid">
                {data.accounts.map((acct) => (
                  <AccountCard key={acct.account_id} account={acct} />
                ))}
              </section>
            )}
          </>
        )}
      </main>

      <AskDrawer open={askOpen} onClose={closeAsk}>
        <ChatStatusBanner apiOk={!!data && !error} apiError={error} />
        <AskPanel askRequest={askRequest} onAskRequestHandled={clearAskRequest} />
      </AskDrawer>
    </>
  );
}
