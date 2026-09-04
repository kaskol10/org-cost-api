import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { fetchMeta, fetchReport, fetchReportFresh, fetchExplainedSuggestions, fetchChatHealth, chatEnabled, ApiError } from "./api";
import AccountCard from "./components/AccountCard";
import ChatStatusBanner from "./components/ChatStatusBanner";
import AskPanel, { type AskRequest } from "./components/AskPanel";
import BillingHighlights from "./components/BillingHighlights";
import DailyCostChart from "./components/DailyCostChart";
import OrgTopServicesPanel from "./components/OrgTopServicesPanel";
import SuggestionsPanel from "./components/SuggestionsPanel";
import SummaryDashboard from "./components/SummaryDashboard";
import VgpuBackground from "./components/VgpuBackground";
import { buildBillingHighlights } from "./utils/highlights";
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
      const report = await fetchReport();
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
  }, [applySuggestions]);

  const refresh = useCallback(async () => {
    setLoading(true);
    setError(null);
    setAuthError(false);
    try {
      const report = await fetchReportFresh();
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
  }, [applySuggestions]);

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
    () => buildBillingHighlights(data, trends, suggestions),
    [data, trends, suggestions]
  );

  const handleAskAbout = useCallback((question: string) => {
    setTab("ask");
    askIdRef.current += 1;
    setAskRequest({ id: askIdRef.current, question });
  }, []);

  const clearAskRequest = useCallback(() => {
    setAskRequest(null);
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
            Conversational billing analysis — explore highlights, then ask follow-ups
            about spend, waste, and savings
            {dateRange ? ` · ${dateRange}` : ""}
          </p>
        </div>
        <button className="btn" type="button" onClick={refresh} disabled={loading}>
          {loading ? "Refreshing…" : "Refresh"}
        </button>
      </header>

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

      {tab === "ask" && (
        <>
          <ChatStatusBanner apiOk={!!data && !error} apiError={error} />
          {loading && !data && <p className="loading">Loading billing context…</p>}
          {highlights.length > 0 && (
            <BillingHighlights highlights={highlights} onAskAbout={handleAskAbout} />
          )}
          <AskPanel
            askRequest={askRequest}
            onAskRequestHandled={clearAskRequest}
          />
        </>
      )}

      {tab === "explore" && (
        <>
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
              />

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
        </>
      )}
    </div>
    </>
  );
}
