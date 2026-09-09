import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { fetchMeta, fetchReport, fetchReportFresh, fetchExplainedSuggestions, fetchChatHealth, chatEnabled, ApiError, type PeriodPreset } from "./api";
import AccountCard from "./components/AccountCard";
import ChatStatusBanner from "./components/ChatStatusBanner";
import AskPanel, { type AskRequest } from "./components/AskPanel";
import BillingHighlights from "./components/BillingHighlights";
import CostContextBar from "./components/CostContextBar";
import DailyCostChart from "./components/DailyCostChart";
import OrgTopServicesPanel from "./components/OrgTopServicesPanel";
import SuggestionsPanel from "./components/SuggestionsPanel";
import SummaryDashboard from "./components/SummaryDashboard";
import VgpuBackground from "./components/VgpuBackground";
import { buildBillingHighlights } from "./utils/highlights";
import { formatCurrency, formatSignedCurrency } from "./utils/format";
import type { DashboardResponse, DailyCost, ReportResponse, SuggestionsResponse, TrendsResponse } from "./types";

type Tab = "ask" | "explore";

function mergeDaily(
  accounts: DashboardResponse["accounts"],
  field: "daily" | "all_daily"
): DailyCost[] {
  const byDate = new Map<string, number>();
  for (const acct of accounts) {
    if (!acct.costs) continue;
    for (const d of acct.costs[field] ?? []) {
      byDate.set(d.date, (byDate.get(d.date) ?? 0) + d.amount);
    }
  }
  return [...byDate.entries()]
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([date, amount]) => ({ date, amount, unit: "USD" }));
}

export default function App() {
  const [tab, setTab] = useState<Tab>("ask");
  const [askRequest, setAskRequest] = useState<AskRequest | null>(null);
  const askIdRef = useRef(0);
  const [data, setData] = useState<DashboardResponse | null>(null);
  const [trends, setTrends] = useState<TrendsResponse | null>(null);
  const [suggestions, setSuggestions] = useState<SuggestionsResponse | null>(null);
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

  const consolidatedOrgDaily = useMemo(
    () => (data?.accounts.length ? mergeDaily(data.accounts, "all_daily") : []),
    [data]
  );

  const consolidatedEC2Daily = useMemo(
    () => (data?.accounts.length ? mergeDaily(data.accounts, "daily") : []),
    [data]
  );

  const dateRange =
    data &&
    `${new Date(data.start + "T00:00:00").toLocaleDateString()} – ${new Date(data.end + "T00:00:00").toLocaleDateString()}`;

  const highlights = useMemo(
    () => buildBillingHighlights(data, trends, suggestions, period),
    [data, trends, suggestions, period]
  );

  const topAccountsBySpend = useMemo(() => {
    if (!data?.accounts.length) return [];
    return [...data.accounts]
      .filter((a) => a.costs && !a.error)
      .sort((a, b) => (b.costs?.all_total ?? 0) - (a.costs?.all_total ?? 0))
      .slice(0, 5);
  }, [data?.accounts]);

  const accountStrip = useMemo(() => {
    const increases = trends?.top_account_increases ?? [];
    const decreases = trends?.top_account_decreases ?? [];
    const hasMovers =
      !!trends?.prior_source && (increases.length > 0 || decreases.length > 0);
    if (hasMovers) {
      return { mode: "movers" as const, increases, decreases };
    }
    return { mode: "spend" as const, accounts: topAccountsBySpend };
  }, [trends, topAccountsBySpend]);

  const handleAskAbout = useCallback((question: string) => {
    setTab("ask");
    askIdRef.current += 1;
    setAskRequest({ id: askIdRef.current, question });
  }, []);

  const clearAskRequest = useCallback(() => {
    setAskRequest(null);
  }, []);

  const openAccountDetails = useCallback((accountId: string) => {
    setShowDetails(true);
    window.setTimeout(() => {
      document
        .getElementById(`account-${accountId}`)
        ?.scrollIntoView({ behavior: "smooth", block: "start" });
    }, 50);
  }, []);

  return (
    <>
      <VgpuBackground />
      {demoMode && !demoBannerDismissed && (
        <div className="demo-banner" role="status">
          <span>Demo data — not connected to AWS. Explore the UI with synthetic fixture costs.</span>
          <button type="button" className="demo-banner-dismiss" onClick={() => setDemoBannerDismissed(true)}>
            Dismiss
          </button>
        </div>
      )}
      <div className="app">
      <header className="header">
        <div>
          <h1>AWS Org Cost Explorer</h1>
          <p>
            {dateRange
              ? `Usage spend for ${dateRange}`
              : "Org-wide AWS usage spend, trends, and savings opportunities"}
          </p>
        </div>
      </header>

      <CostContextBar
        orgTotal={data?.totals.org_total ?? null}
        accountCount={data?.totals.account_count}
        dateRange={dateRange ?? undefined}
        trends={trends}
        trendsLoading={loading && !trends}
        period={period}
        onPeriodChange={setPeriod}
        onRefresh={refresh}
        loading={loading}
        demoMode={demoMode}
      />

      <nav className="tab-nav" role="tablist">
        <button
          type="button"
          role="tab"
          aria-selected={tab === "ask"}
          className={tab === "ask" ? "tab active" : "tab"}
          onClick={() => setTab("ask")}
        >
          Chat
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={tab === "explore"}
          className={tab === "explore" ? "tab active" : "tab"}
          onClick={() => setTab("explore")}
        >
          Explore
        </button>
      </nav>

      {authError && (
        <div className="error">
          API returned 401 — set <code>VITE_API_TOKEN</code> to match{" "}
          <code>ORG_COST_API_TOKEN</code> and restart the dev server.
        </div>
      )}

      {error && !authError && <div className="error">{error}</div>}

      <div
        className={tab === "ask" ? undefined : "tab-panel-hidden"}
        aria-hidden={tab !== "ask"}
      >
        <ChatStatusBanner apiOk={!!data && !error} apiError={error} />
        {loading && !data && <p className="loading">Loading billing context…</p>}
        {highlights.length > 0 && (
          <BillingHighlights highlights={highlights} onAskAbout={handleAskAbout} />
        )}
        <AskPanel
          askRequest={askRequest}
          onAskRequestHandled={clearAskRequest}
        />
      </div>

      <div
        className={tab === "explore" ? undefined : "tab-panel-hidden"}
        aria-hidden={tab !== "explore"}
      >
        {loading && !data && <p className="loading">Loading AWS data…</p>}

        {data && (
          <>
            {data.cur_note && (
              <div className="error" style={{ marginBottom: "1rem" }}>
                {data.cur_note}
              </div>
            )}
            {data.cur_enabled && (
              <p className="category-desc" style={{ marginBottom: "1rem" }}>
                EBS daily disk/snapshot counts use your Cost and Usage Report (Athena).
              </p>
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
            />

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
                  {accountStrip.mode === "movers"
                    ? "Account movers"
                    : "Top accounts by spend"}
                </h2>
                {accountStrip.mode === "movers" ? (
                  <ul className="top-accounts-strip-list">
                    {[
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
                          <span
                            className={`top-accounts-chip-amount ${up ? "up" : "down"}`}
                          >
                            {formatSignedCurrency(t.change_usd)}
                          </span>
                        </button>
                      </li>
                    ))}
                  </ul>
                ) : (
                  <ul className="top-accounts-strip-list">
                    {accountStrip.accounts.map((acct) => (
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
                )}
              </section>
            )}

            {highlights.length > 0 && (
              <BillingHighlights
                highlights={highlights}
                onAskAbout={handleAskAbout}
                compact
              />
            )}

            <SuggestionsPanel
              suggestions={suggestions}
              loading={loading && !suggestions}
              enriching={enrichingSuggestions}
              onAskAbout={handleAskAbout}
            />

            <div className="details-toggle-row">
              <button
                type="button"
                className="btn btn-ghost"
                aria-expanded={showDetails}
                onClick={() => setShowDetails((v) => !v)}
              >
                {showDetails ? "Hide details" : "Show details"}
                <span className="details-toggle-hint">
                  {showDetails
                    ? "full service breakdown, charts, accounts"
                    : `${data.top_services?.length ?? 0} services · ${data.accounts.length} accounts`}
                </span>
              </button>
            </div>

            {showDetails && (
              <div className="details-section">
                <OrgTopServicesPanel
                  drivers={data.top_services ?? []}
                  orgTotal={data.totals.org_total}
                />

                <section className="panel">
                  <DailyCostChart
                    data={consolidatedOrgDaily}
                    title="Daily organization spend (all services, usage)"
                  />
                </section>

                <section className="panel">
                  <DailyCostChart
                    data={consolidatedEC2Daily}
                    title="Daily EC2-Other spend (all accounts)"
                  />
                </section>

                <p className="category-desc" style={{ marginBottom: "1rem" }}>
                  Per account: top services by cost, EC2-Other usage breakdown, live storage
                  inventory, and service drill-down. Use this to find where money goes and
                  what to rightsize or delete.
                </p>

                <section className="account-grid">
                  {data.accounts.map((acct) => (
                    <AccountCard key={acct.account_id} account={acct} />
                  ))}
                </section>
              </div>
            )}

            <p className="empty" style={{ marginTop: "1.5rem" }}>
              Last updated {new Date(data.generated_at).toLocaleString()}
            </p>
          </>
        )}
      </div>
    </div>
    </>
  );
}
