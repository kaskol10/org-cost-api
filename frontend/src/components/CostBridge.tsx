import { useEffect, useMemo, useRef, useState } from "react";
import type { TrendsResponse } from "../types";
import { bridgeDomain, buildBridge, type BridgeStep } from "../utils/bridge";
import { formatCurrency, formatSignedCurrency } from "../utils/format";
import { priorCompareLabel } from "../utils/periodLabels";
import type { PeriodPreset } from "../api";

interface Props {
  trends: TrendsResponse | null;
  period: PeriodPreset;
  onAskAbout?: (question: string) => void;
}

// Geometry in CSS pixels: the SVG is drawn at its container's real width so
// text stays 13px at any screen size instead of scaling with the chart.
const ROW_H = 30;
const BAR_H = 14;
const TOP = 4;

function useContainerWidth<T extends HTMLElement>(fallback: number) {
  const ref = useRef<T>(null);
  const [width, setWidth] = useState(fallback);
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    setWidth(el.clientWidth || fallback);
    if (typeof ResizeObserver === "undefined") return;
    const ro = new ResizeObserver((entries) => {
      const w = entries[0]?.contentRect.width;
      if (w) setWidth(w);
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, [fallback]);
  return [ref, width] as const;
}

function compactUSD(v: number): string {
  const abs = Math.abs(v);
  if (abs >= 1_000_000) return `$${(v / 1_000_000).toFixed(1)}M`;
  if (abs >= 10_000) return `$${(v / 1000).toFixed(1)}k`;
  if (abs >= 1000) return `$${(v / 1000).toFixed(2)}k`;
  return formatCurrency(v);
}

function askFor(step: BridgeStep, compare: string): string {
  const moved = formatSignedCurrency(step.value);
  if (step.key === "other") {
    return `Which smaller services account for the ${moved} change in "other services" ${compare}?`;
  }
  return `What drove the ${moved} change in ${step.label} spend ${compare}? Break it down by account and usage type.`;
}

/**
 * Cost bridge: how spend got from the prior period total to this one, one
 * row per service that moved. The value axis is zoomed to the range the
 * running total spans (it does not start at zero), so the period totals are
 * drawn as markers rather than bars and the scale start is labelled.
 */
export default function CostBridge({ trends, period, onAskAbout }: Props) {
  const steps = useMemo(() => buildBridge(trends), [trends]);
  const [hover, setHover] = useState<string | null>(null);
  const [wrapRef, measured] = useContainerWidth<HTMLDivElement>(760);
  if (!steps) return null;

  const W = Math.max(320, Math.round(measured));
  const LABEL_W = Math.min(200, Math.round(W * 0.3));
  const VALUE_W = W < 480 ? 84 : 110;
  const PLOT_X0 = LABEL_W;
  const PLOT_X1 = W - VALUE_W - 8;
  const maxLabel = Math.max(10, Math.floor(LABEL_W / 7.5));

  const [lo, hi] = bridgeDomain(steps);
  const x = (v: number) => PLOT_X0 + ((v - lo) / (hi - lo)) * (PLOT_X1 - PLOT_X0);
  const height = TOP + steps.length * ROW_H + 8;
  const prior = steps[0];
  const current = steps[steps.length - 1];
  const change = current.value - prior.value;
  const compare = priorCompareLabel(period);
  const increases = steps.filter((s) => s.kind === "delta" && s.value > 0).length;
  const decreases = steps.filter((s) => s.kind === "delta" && s.value < 0).length;

  return (
    <section className="panel cost-bridge" aria-labelledby="cost-bridge-title">
      <div className="cost-bridge-head">
        <h2 id="cost-bridge-title">Where the change came from</h2>
        <p className="cost-bridge-sub">
          {formatCurrency(prior.value)} → {formatCurrency(current.value)} ·{" "}
          <span className={change > 0 ? "delta-up" : change < 0 ? "delta-down" : undefined}>
            {formatSignedCurrency(change)}
          </span>{" "}
          {compare}
        </p>
      </div>

      <div ref={wrapRef} className="cost-bridge-plot">
      <svg
        className="cost-bridge-svg"
        width={W}
        height={height}
        viewBox={`0 0 ${W} ${height}`}
        role="img"
        aria-label={`Cost bridge from ${formatCurrency(prior.value)} to ${formatCurrency(current.value)}: ${increases} increases and ${decreases} decreases by service`}
      >
        {/* Reference lines at the two period totals. */}
        {[prior, current].map((t) => (
          <line
            key={`ref-${t.key}`}
            x1={x(t.value)}
            x2={x(t.value)}
            y1={TOP}
            y2={height - 6}
            className="cost-bridge-ref"
          />
        ))}
        {steps.map((s, i) => {
          const y = TOP + i * ROW_H;
          const cy = y + ROW_H / 2;
          const interactive = s.kind === "delta" && !!onAskAbout;
          const isUp = s.value > 0;
          const x0 = x(Math.min(s.start, s.end));
          const x1 = x(Math.max(s.start, s.end));
          const tip =
            s.kind === "total"
              ? `${s.label}: ${formatCurrency(s.value)}`
              : `${s.label}: ${formatSignedCurrency(s.value)} (running total ${formatCurrency(s.end)})`;
          return (
            <g
              key={s.key}
              className={`cost-bridge-row${hover === s.key ? " is-hover" : ""}${interactive ? " is-interactive" : ""}`}
              onMouseEnter={() => setHover(s.key)}
              onMouseLeave={() => setHover(null)}
              onClick={interactive ? () => onAskAbout!(askFor(s, compare)) : undefined}
              onKeyDown={
                interactive
                  ? (e) => {
                      if (e.key === "Enter" || e.key === " ") {
                        e.preventDefault();
                        onAskAbout!(askFor(s, compare));
                      }
                    }
                  : undefined
              }
              tabIndex={interactive ? 0 : undefined}
              role={interactive ? "button" : undefined}
              aria-label={interactive ? `${tip}. Ask why.` : undefined}
            >
              <title>{interactive ? `${tip} — click to ask why` : tip}</title>
              {/* Full-row hit target, wider than the mark. */}
              <rect x={0} y={y} width={W} height={ROW_H} className="cost-bridge-hit" />
              <text x={0} y={cy} dy="0.35em" className={s.kind === "total" ? "cost-bridge-label is-total" : "cost-bridge-label"}>
                {s.label.length > maxLabel ? `${s.label.slice(0, maxLabel - 1)}…` : s.label}
              </text>
              {s.kind === "total" ? (
                <rect x={x(s.value) - 2} y={cy - BAR_H / 2 - 3} width={4} height={BAR_H + 6} rx={2} className="cost-bridge-total" />
              ) : (
                <rect
                  x={x0}
                  y={cy - BAR_H / 2}
                  width={Math.max(2, x1 - x0)}
                  height={BAR_H}
                  rx={3}
                  className={isUp ? "cost-bridge-up" : "cost-bridge-down"}
                />
              )}
              {i < steps.length - 1 && (
                <line
                  x1={x(s.end)}
                  x2={x(s.end)}
                  y1={cy + BAR_H / 2}
                  y2={cy + ROW_H - BAR_H / 2}
                  className="cost-bridge-connector"
                />
              )}
              <text x={W} y={cy} dy="0.35em" textAnchor="end" className={s.kind === "total" ? "cost-bridge-value is-total" : "cost-bridge-value"}>
                {s.kind === "total" ? formatCurrency(s.value) : formatSignedCurrency(s.value)}
              </text>
            </g>
          );
        })}
      </svg>
      </div>

      <div className="cost-bridge-foot">
        <span className="cost-bridge-key">
          <span className="cost-bridge-swatch is-up" aria-hidden="true" /> Increase
        </span>
        <span className="cost-bridge-key">
          <span className="cost-bridge-swatch is-down" aria-hidden="true" /> Decrease
        </span>
        <span className="cost-bridge-note">
          Scale runs {compactUSD(lo)}–{compactUSD(hi)}, not from $0.
          {onAskAbout ? " Click a row to ask why it moved." : ""}
        </span>
      </div>

      <table className="sr-only">
        <caption>Cost change by service {compare}</caption>
        <thead>
          <tr>
            <th scope="col">Step</th>
            <th scope="col">Change</th>
            <th scope="col">Running total</th>
          </tr>
        </thead>
        <tbody>
          {steps.map((s) => (
            <tr key={s.key}>
              <th scope="row">{s.label}</th>
              <td>{s.kind === "total" ? "—" : formatSignedCurrency(s.value)}</td>
              <td>{formatCurrency(s.end)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}
