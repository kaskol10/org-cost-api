"""Build visual cost reports for Hermes (markdown + HTML)."""

from __future__ import annotations

import html
import os
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


def _default_reports_dir() -> str:
    new = os.path.expanduser("~/.org-cost/reports")
    legacy = os.path.expanduser("~/.ec2-other/reports")
    if Path(new).exists() or not Path(legacy).exists():
        return new
    return legacy


def _usd(v: float | int | None) -> str:
    if v is None:
        return "—"
    return f"${v:,.2f}"


def _pct(v: float | int | None) -> str:
    if v is None:
        return "—"
    sign = "+" if v > 0 else ""
    return f"{sign}{v:.1f}%"


def _bar(value: float, max_abs: float, width: int = 24) -> str:
    if max_abs <= 0:
        return "░" * width
    n = int(min(width, max(1, abs(value) / max_abs * width)))
    ch = "█" if value >= 0 else "▓"
    return ch * n + "░" * (width - n)


def _friendly_service(name: str) -> str:
    s = name or "Unknown"
    if "Redshift" in s:
        return "Redshift"
    if "Simple Storage Service" in s:
        return "S3"
    if "Relational Database" in s:
        return "RDS"
    if "Elastic Compute Cloud - Compute" in s:
        return "EC2"
    if s == "EC2 - Other":
        return "EC2-Other"
    if "Data Transfer" in s:
        return "Data transfer"
    if "CloudWatch" in s:
        return "CloudWatch"
    if "Kinesis" in s:
        return "Kinesis"
    return s[:28] + ("…" if len(s) > 28 else "")


def _mermaid_bar_chart(items: list[dict[str, Any]], value_key: str, label_key: str) -> str:
    if not items:
        return ""
    labels = [(_friendly_service(str(i.get(label_key, ""))) or "?").replace('"', "'") for i in items[:8]]
    values = [round(float(i.get(value_key) or 0), 2) for i in items[:8]]
    if not values:
        return ""
    lo = min(values + [0])
    hi = max(values + [0])
    pad = max(abs(lo), abs(hi)) * 0.15 or 10
    ymin = int(lo - pad)
    ymax = int(hi + pad)
    lines = [
        "```mermaid",
        "xychart-beta",
        '    title "Spend change by service (USD)"',
        f"    x-axis [{', '.join(json_quote(l) for l in labels)}]",
        f'    y-axis "USD" {ymin} --> {ymax}',
        f"    bar [{', '.join(str(v) for v in values)}]",
        "```",
    ]
    return "\n".join(lines)


def json_quote(s: str) -> str:
    return '"' + s.replace('"', "'") + '"'


def _mermaid_pie(services: list[dict[str, Any]], total: float) -> str:
    if not services or total <= 0:
        return ""
    lines = ["```mermaid", "pie showData", "    title Current org spend mix"]
    for s in services[:8]:
        amt = float(s.get("amount") or 0)
        if amt < 1:
            continue
        label = _friendly_service(str(s.get("service", "")))
        lines.append(f'    "{label}" : {amt:.2f}')
    lines.append("```")
    return "\n".join(lines)


def build_markdown_report(
    dashboard: dict[str, Any],
    trends: dict[str, Any],
    suggestions: dict[str, Any],
    *,
    html_path: str | None = None,
) -> str:
    totals = dashboard.get("totals") or {}
    org = trends.get("org_total_change") or {}
    cur_p = trends.get("current_period") or {}
    pri_p = trends.get("prior_period") or {}
    increases = trends.get("top_increases") or []
    decreases = trends.get("top_decreases") or []
    top_services = dashboard.get("top_services") or []
    sug_list = suggestions.get("suggestions") or []

    org_cur = float(org.get("current_usd") or totals.get("org_total") or 0)
    org_pri = float(org.get("prior_usd") or 0)
    org_chg = float(org.get("change_percent") or 0)
    org_delta = float(org.get("change_usd") or 0)

    lines: list[str] = [
        "# AWS organization cost report",
        "",
        f"**Current period:** {cur_p.get('start', '?')} → {cur_p.get('end', '?')}  ",
        f"**Prior period:** {pri_p.get('start', '?')} → {pri_p.get('end', '?')}  ",
        f"**Data source (prior):** {trends.get('prior_source', 'n/a')} · CE calls: {trends.get('ce_calls_used', 0)}",
        "",
        "## Organization spend",
        "",
        f"| | Amount |",
        f"|---|--:|",
        f"| **Current** | **{_usd(org_cur)}** |",
        f"| Prior | {_usd(org_pri)} |",
        f"| Change | {_usd(org_delta)} ({_pct(org_chg)}) |",
        "",
    ]

    # Mermaid: org comparison
    if org_pri > 0 or org_cur > 0:
        lines.extend(
            [
                "```mermaid",
                "xychart-beta",
                '    title "Organization spend (USD)"',
                '    x-axis ["Prior period", "Current period"]',
                f"    y-axis \"USD\" 0 --> {int(max(org_pri, org_cur) * 1.15) or 100}",
                f"    bar [{round(org_pri, 2)}, {round(org_cur, 2)}]",
                "```",
                "",
            ]
        )

    if increases:
        lines.append("## Biggest increases")
        lines.append("")
        max_abs = max(abs(float(i.get("change_usd") or 0)) for i in increases) or 1
        for item in increases[:6]:
            name = _friendly_service(str(item.get("display_name") or item.get("service", "")))
            chg = float(item.get("change_usd") or 0)
            pct = float(item.get("change_percent") or 0)
            lines.append(
                f"- **{name}** {_usd(chg)} ({_pct(pct)}) — "
                f"{_usd(item.get('prior_usd'))} → {_usd(item.get('current_usd'))}  "
                f"`{_bar(chg, max_abs)}`"
            )
        lines.append("")
        lines.append(_mermaid_bar_chart(increases, "change_usd", "display_name"))
        lines.append("")

    if decreases:
        lines.append("## Biggest decreases")
        lines.append("")
        max_abs = max(abs(float(i.get("change_usd") or 0)) for i in decreases) or 1
        for item in decreases[:6]:
            name = _friendly_service(str(item.get("display_name") or item.get("service", "")))
            chg = float(item.get("change_usd") or 0)
            pct = float(item.get("change_percent") or 0)
            lines.append(
                f"- **{name}** {_usd(chg)} ({_pct(pct)}) — "
                f"{_usd(item.get('prior_usd'))} → {_usd(item.get('current_usd'))}  "
                f"`{_bar(chg, max_abs)}`"
            )
        lines.append("")

    if top_services:
        lines.extend(["## Current spend mix (top services)", ""])
        for s in top_services[:8]:
            amt = float(s.get("amount") or 0)
            pct = float(s.get("percent") or 0)
            name = _friendly_service(str(s.get("service", "")))
            lines.append(f"- **{name}** {_usd(amt)} ({pct:.1f}% of org)")
        lines.append("")
        lines.append(_mermaid_pie(top_services, org_cur))
        lines.append("")

    if sug_list:
        lines.extend(["## Savings opportunities", ""])
        for s in sug_list[:9]:
            pri = s.get("priority", "?")
            title = s.get("title", "")
            detail = s.get("detail", "")
            est = s.get("estimated_monthly_usd")
            est_s = f" · ~{_usd(est)}/mo" if est is not None else ""
            lines.append(f"### {pri}. {title}{est_s}")
            if detail:
                lines.append(f"{detail}")
            actions = s.get("actions") or []
            for a in actions[:2]:
                lines.append(f"- {a}")
            lines.append("")

    lines.append("---")
    lines.append(f"*{suggestions.get('summary', '')}*")
    if html_path:
        lines.append("")
        lines.append(f"**Interactive report:** `{html_path}` (open in browser)")
    return "\n".join(lines)


def write_html_report(
    dashboard: dict[str, Any],
    trends: dict[str, Any],
    suggestions: dict[str, Any],
    out_dir: str | None = None,
) -> str:
    out = Path(out_dir or _default_reports_dir())
    out.mkdir(parents=True, exist_ok=True)
    ts = datetime.now(timezone.utc).strftime("%Y-%m-%d-%H%M")
    path = out / f"cost-report-{ts}.html"
    latest = out / "latest.html"

    totals = dashboard.get("totals") or {}
    org = trends.get("org_total_change") or {}
    increases = trends.get("top_increases") or []
    decreases = trends.get("top_decreases") or []
    top_services = dashboard.get("top_services") or []
    sug_list = suggestions.get("suggestions") or []
    cur_p = trends.get("current_period") or {}
    pri_p = trends.get("prior_period") or {}

    org_cur = float(org.get("current_usd") or totals.get("org_total") or 0)
    org_pri = float(org.get("prior_usd") or 0)
    org_chg = float(org.get("change_percent") or 0)

    def rows_trend(items: list[dict[str, Any]], positive: bool) -> str:
        out_rows = []
        max_abs = max((abs(float(i.get("change_usd") or 0)) for i in items), default=1) or 1
        for i in items[:8]:
            chg = float(i.get("change_usd") or 0)
            if positive and chg < 0:
                continue
            if not positive and chg > 0:
                continue
            pct = float(i.get("change_percent") or 0)
            w = min(100, abs(chg) / max_abs * 100)
            color = "#e63946" if chg > 0 else "#2a9d8f"
            name = html.escape(_friendly_service(str(i.get("display_name") or i.get("service", ""))))
            out_rows.append(
                f'<tr><td>{name}</td><td class="num">{_usd(i.get("prior_usd"))}</td>'
                f'<td class="num">{_usd(i.get("current_usd"))}</td>'
                f'<td class="num">{_usd(chg)} ({_pct(pct)})</td>'
                f'<td><div class="bar"><div class="fill" style="width:{w:.0f}%;background:{color}"></div></div></td></tr>'
            )
        return "\n".join(out_rows)

    def rows_pie() -> str:
        rows = []
        max_amt = max((float(s.get("amount") or 0) for s in top_services), default=1) or 1
        for s in top_services[:10]:
            amt = float(s.get("amount") or 0)
            pct = float(s.get("percent") or 0)
            w = amt / max_amt * 100
            name = html.escape(_friendly_service(str(s.get("service", ""))))
            rows.append(
                f'<tr><td>{name}</td><td class="num">{_usd(amt)}</td>'
                f'<td class="num">{pct:.1f}%</td>'
                f'<td><div class="bar"><div class="fill" style="width:{w:.0f}%;background:#4cc9f0"></div></div></td></tr>'
            )
        return "\n".join(rows)

    def cards_suggestions() -> str:
        cards = []
        for s in sug_list[:12]:
            pri = s.get("priority", "")
            cat = html.escape(str(s.get("category", "")))
            title = html.escape(str(s.get("title", "")))
            detail = html.escape(str(s.get("detail", "")))
            est = s.get("estimated_monthly_usd")
            est_badge = f'<span class="badge">~{_usd(est)}/mo</span>' if est else ""
            cards.append(
                f'<article class="card"><div class="card-head"><span class="pri">#{pri}</span>'
                f'<span class="cat">{cat}</span>{est_badge}</div>'
                f"<h3>{title}</h3><p>{detail}</p></article>"
            )
        return "\n".join(cards)

    chg_class = "down" if org_chg < 0 else "up"
    doc = f"""<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8"/>
<meta name="viewport" content="width=device-width, initial-scale=1"/>
<title>AWS Org Cost Report</title>
<style>
  :root {{ --bg:#0f1117; --panel:#1a1d27; --text:#e8eaed; --muted:#9aa0a6; --border:#2a2f3a; }}
  * {{ box-sizing:border-box; }}
  body {{ font-family: system-ui, -apple-system, sans-serif; background:var(--bg); color:var(--text); margin:0; padding:1.5rem; line-height:1.45; }}
  h1 {{ margin:0 0 .25rem; font-size:1.6rem; }}
  .sub {{ color:var(--muted); margin-bottom:1.5rem; }}
  .grid {{ display:grid; gap:1rem; grid-template-columns:repeat(auto-fit, minmax(280px, 1fr)); }}
  .panel {{ background:var(--panel); border:1px solid var(--border); border-radius:12px; padding:1rem 1.1rem; }}
  .kpi {{ font-size:2rem; font-weight:700; }}
  .kpi.up {{ color:#e63946; }}
  .kpi.down {{ color:#2a9d8f; }}
  table {{ width:100%; border-collapse:collapse; font-size:.9rem; }}
  th, td {{ padding:.45rem .35rem; border-bottom:1px solid var(--border); text-align:left; }}
  th.num, td.num {{ text-align:right; font-variant-numeric:tabular-nums; }}
  .bar {{ height:8px; background:var(--border); border-radius:4px; overflow:hidden; min-width:80px; }}
  .fill {{ height:100%; border-radius:4px; }}
  .card {{ background:var(--panel); border:1px solid var(--border); border-radius:10px; padding:.9rem; margin-bottom:.75rem; }}
  .card-head {{ display:flex; gap:.5rem; align-items:center; margin-bottom:.35rem; font-size:.8rem; color:var(--muted); }}
  .pri {{ font-weight:700; color:#f4a261; }}
  .cat {{ text-transform:uppercase; letter-spacing:.04em; }}
  .badge {{ background:#2a3f5f; color:#4cc9f0; padding:.15rem .45rem; border-radius:4px; margin-left:auto; }}
  h2 {{ font-size:1.05rem; margin:0 0 .75rem; }}
  h3 {{ font-size:.95rem; margin:.25rem 0; }}
</style>
</head>
<body>
<h1>AWS organization cost report</h1>
<p class="sub">Current: {html.escape(str(cur_p.get("start","")))} → {html.escape(str(cur_p.get("end","")))} · Prior: {html.escape(str(pri_p.get("start","")))} → {html.escape(str(pri_p.get("end","")))}</p>
<div class="grid">
  <div class="panel"><h2>Current spend</h2><div class="kpi">{_usd(org_cur)}</div></div>
  <div class="panel"><h2>Prior spend</h2><div class="kpi">{_usd(org_pri)}</div></div>
  <div class="panel"><h2>Change</h2><div class="kpi {chg_class}">{_pct(org_chg)}</div><div class="sub">{_usd(org.get("change_usd"))}</div></div>
</div>
<div class="panel" style="margin-top:1rem">
  <h2>Top increases</h2>
  <table><thead><tr><th>Service</th><th class="num">Prior</th><th class="num">Current</th><th class="num">Change</th><th></th></tr></thead>
  <tbody>{rows_trend(increases, True)}</tbody></table>
</div>
<div class="panel" style="margin-top:1rem">
  <h2>Top decreases</h2>
  <table><thead><tr><th>Service</th><th class="num">Prior</th><th class="num">Current</th><th class="num">Change</th><th></th></tr></thead>
  <tbody>{rows_trend(decreases, False)}</tbody></table>
</div>
<div class="panel" style="margin-top:1rem">
  <h2>Spend mix (current)</h2>
  <table><thead><tr><th>Service</th><th class="num">Amount</th><th class="num">Share</th><th></th></tr></thead>
  <tbody>{rows_pie()}</tbody></table>
</div>
<div style="margin-top:1rem">
  <h2>Savings opportunities</h2>
  {cards_suggestions()}
</div>
<p class="sub" style="margin-top:2rem">Generated {html.escape(datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M UTC"))} · org-cost-api MCP</p>
</body>
</html>"""

    path.write_text(doc, encoding="utf-8")
    latest.write_text(doc, encoding="utf-8")
    return str(path)
