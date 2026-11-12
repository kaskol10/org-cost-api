import { useEffect, useRef, useState } from "react";
import { fetchServiceTagDelta } from "../api";
import type { ServiceTagDeltaResponse, ServiceTrend } from "../types";
import { formatCurrency, formatSignedCurrency } from "../utils/format";

interface Props {
  /** The selected service, or null for the "Other services" row. */
  trend: ServiceTrend | null;
  /** For "Other services": the listed services not shown as their own bridge rows. */
  otherTrends?: ServiceTrend[];
  /** For "Other services": change from services below the API's trend threshold. */
  unlistedChange?: number;
  compareLabel: string;
  onClose: () => void;
  onAskAbout: (question: string) => void;
  onSelectAccount: (accountId: string) => void;
  onSelectService: (service: string) => void;
}

function DivergingRows({
  rows,
  onSelect,
}: {
  rows: { key: string; label: string; change: number; detail: string; onClick?: () => void }[];
  onSelect?: boolean;
}) {
  const max = Math.max(1, ...rows.map((r) => Math.abs(r.change)));
  // Diverging around a centre line only when there are moves both ways;
  // otherwise bars start at the left so they use the full width.
  const diverging = rows.some((r) => r.change > 0) && rows.some((r) => r.change < 0);
  return (
    <ul className={`drill-rows${diverging ? " is-diverging" : ""}`}>
      {rows.map((r) => {
        const span = diverging ? 50 : 100;
        const pct = (Math.abs(r.change) / max) * span;
        const up = r.change > 0;
        const barStyle = !diverging
          ? { left: 0, width: `${pct}%` }
          : up
            ? { left: "50%", width: `${pct}%` }
            : { right: "50%", width: `${pct}%` };
        const content = (
          <>
            <span className="drill-row-label">{r.label}</span>
            <span className="drill-row-bar" aria-hidden="true">
              <span className="drill-row-axis" />
              <span className={`drill-row-fill ${up ? "is-up" : "is-down"}`} style={barStyle} />
            </span>
            <span className="drill-row-value">
              {formatSignedCurrency(r.change)}
              <span className="drill-row-detail">{r.detail}</span>
            </span>
          </>
        );
        return (
          <li key={r.key}>
            {r.onClick && onSelect ? (
              <button type="button" className="drill-row is-button" onClick={r.onClick}>
                {content}
              </button>
            ) : (
              <div className="drill-row">{content}</div>
            )}
          </li>
        );
      })}
    </ul>
  );
}

/**
 * Drill-down for one cost-bridge row: which accounts moved this service
 * (from the trends payload, no Cost Explorer calls), and on request the
 * tag-level breakdown from /service-tag-delta (uses Cost Explorer).
 */
export default function ServiceDrilldown({
  trend,
  otherTrends = [],
  unlistedChange = 0,
  compareLabel,
  onClose,
  onAskAbout,
  onSelectAccount,
  onSelectService,
}: Props) {
  const [tags, setTags] = useState<ServiceTagDeltaResponse | null>(null);
  const [tagsLoading, setTagsLoading] = useState(false);
  const [tagsError, setTagsError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const abortRef = useRef<AbortController | null>(null);
  const rootRef = useRef<HTMLElement>(null);
  const service = trend?.service ?? null;

  useEffect(() => {
    setTags(null);
    setTagsError(null);
    setTagsLoading(false);
    abortRef.current?.abort();
    rootRef.current?.scrollIntoView({ behavior: "smooth", block: "nearest" });
    return () => abortRef.current?.abort();
  }, [service]);

  async function loadTags() {
    if (!service) return;
    abortRef.current?.abort();
    const ac = new AbortController();
    abortRef.current = ac;
    setTagsLoading(true);
    setTagsError(null);
    try {
      setTags(await fetchServiceTagDelta({ service, signal: ac.signal }));
    } catch (e) {
      if (!ac.signal.aborted) setTagsError(e instanceof Error ? e.message : "Could not load tag breakdown");
    } finally {
      if (!ac.signal.aborted) setTagsLoading(false);
    }
  }

  async function copyLink() {
    try {
      await navigator.clipboard.writeText(window.location.href);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1800);
    } catch {
      setCopied(false);
    }
  }

  const title = trend ? trend.display_name || trend.service : "Other services";
  const change = trend ? trend.change_usd : otherTrends.reduce((s, t) => s + t.change_usd, 0) + unlistedChange;
  const accounts = trend?.accounts ?? [];
  const tagRows = tags ? [...tags.top_increases, ...tags.top_decreases].sort((a, b) => Math.abs(b.change_usd) - Math.abs(a.change_usd)) : [];

  return (
    <section ref={rootRef} className="panel drilldown" aria-labelledby="drilldown-title">
      <div className="drilldown-head">
        <div>
          <h2 id="drilldown-title">{title}</h2>
          <p className="drilldown-sub">
            <span className={change > 0 ? "delta-up" : change < 0 ? "delta-down" : undefined}>{formatSignedCurrency(change)}</span>{" "}
            {compareLabel}
            {trend && ` · ${formatCurrency(trend.prior_usd)} → ${formatCurrency(trend.current_usd)}`}
          </p>
        </div>
        <div className="drilldown-actions">
          {trend && (
            <button
              type="button"
              className="btn btn-ghost"
              onClick={() =>
                onAskAbout(
                  `What drove the ${formatSignedCurrency(trend.change_usd)} change in ${title} spend ${compareLabel}? Break it down by account and usage type.`
                )
              }
            >
              Ask why
            </button>
          )}
          <button type="button" className="btn btn-ghost" onClick={copyLink}>
            {copied ? "Link copied" : "Copy link"}
          </button>
          <button type="button" className="btn btn-ghost" onClick={onClose} aria-label="Close drill-down">
            ✕
          </button>
        </div>
      </div>

      {trend ? (
        <>
          <h3 className="drilldown-h3">By account</h3>
          {accounts.length > 0 ? (
            <DivergingRows
              onSelect
              rows={accounts.map((a) => ({
                key: a.account_id,
                label: a.account_name,
                change: a.change_usd,
                detail: `${formatCurrency(a.prior_usd)} → ${formatCurrency(a.current_usd)}`,
                onClick: () => onSelectAccount(a.account_id),
              }))}
            />
          ) : (
            <p className="drilldown-empty">
              No per-account breakdown for this comparison (the prior period came from Cost Explorer totals, not a saved
              snapshot). Break it down by tag instead.
            </p>
          )}

          <h3 className="drilldown-h3">By tag</h3>
          {!tags && !tagsLoading && (
            <p className="drilldown-empty">
              <button type="button" className="btn btn-ghost" onClick={loadTags}>
                Break down by Name tag
              </button>{" "}
              <span className="drilldown-hint">Queries Cost Explorer (a few calls, cached).</span>
            </p>
          )}
          {tagsLoading && <p className="drilldown-empty">Loading tag breakdown…</p>}
          {tagsError && <p className="error">{tagsError}</p>}
          {tags &&
            (tagRows.length > 0 ? (
              <DivergingRows
                rows={tagRows.map((t) => ({
                  key: t.key,
                  label: t.key || "(untagged)",
                  change: t.change_usd,
                  detail: `${formatCurrency(t.prior_usd)} → ${formatCurrency(t.current_usd)}`,
                }))}
              />
            ) : (
              <p className="drilldown-empty">
                No tag-level changes found for the <code>{tags.tag_key}</code> tag (resources may be untagged, or this is
                demo data).
              </p>
            ))}
        </>
      ) : (
        <>
          <h3 className="drilldown-h3">Smaller services</h3>
          {otherTrends.length > 0 && (
            <DivergingRows
              onSelect
              rows={otherTrends.map((t) => ({
                key: t.service,
                label: t.display_name || t.service,
                change: t.change_usd,
                detail: `${formatCurrency(t.prior_usd)} → ${formatCurrency(t.current_usd)}`,
                onClick: () => onSelectService(t.service),
              }))}
            />
          )}
          {Math.abs(unlistedChange) >= 0.5 && (
            <p className="drilldown-empty">
              {formatSignedCurrency(unlistedChange)} comes from services that each moved less than the trend threshold, or
              from spend not attributed to a service in the comparison period.
            </p>
          )}
        </>
      )}
    </section>
  );
}
