"""MCP server exposing read-only AWS org cost tools for Hermes Agent."""

from __future__ import annotations

import json
import os
from typing import Any

from mcp.server.fastmcp import FastMCP

from org_cost_mcp.client import (
    base_url,
    fetch_account_costs,
    fetch_accounts,
    fetch_ask,
    fetch_dashboard,
    fetch_report,
    fetch_service_detail,
    fetch_suggestions,
    fetch_trends,
    fetch_service_tag_delta,
    fetch_service_tag_totals,
    health_status,
)
from org_cost_mcp.config_discovery import apply_discovered_config
from org_cost_mcp.account_costs import shape_account_costs
from org_cost_mcp.visualize import build_markdown_report, write_html_report

apply_discovered_config()


def _allowed_hosts_from_env() -> list[str]:
    """Parse MCP_ALLOWED_HOSTS (comma-separated public Host headers)."""
    raw = os.environ.get("MCP_ALLOWED_HOSTS", "").strip()
    if not raw:
        return []
    return [item.strip() for item in raw.split(",") if item.strip()]


def _expand_allowed_hosts(hosts: list[str]) -> list[str]:
    """SDK matches Host exactly; allow bare hostname and hostname:*."""
    expanded: list[str] = []
    seen: set[str] = set()
    for host in hosts:
        variants = [host]
        if ":" not in host:
            variants.append(f"{host}:*")
        for item in variants:
            if item not in seen:
                seen.add(item)
                expanded.append(item)
    return expanded


def _origins_for_hosts(hosts: list[str]) -> list[str]:
    origins: list[str] = []
    seen: set[str] = set()
    for host in hosts:
        name = host.split(":")[0]
        if not name or name == "*":
            continue
        for origin in (f"https://{name}", f"https://{name}:*", f"http://{name}", f"http://{name}:*"):
            if origin not in seen:
                seen.add(origin)
                origins.append(origin)
    return origins


def _transport_security_settings() -> Any:
    """DNS-rebinding allowlist for Streamable HTTP behind a Gateway.

    Without this, FastMCP defaults to localhost-only Host headers and returns
    421 Misdirected Request for public names like costs.internal.resiz.es.
    """
    try:
        from mcp.server.transport_security import TransportSecuritySettings
    except ImportError:
        return None

    hosts = _expand_allowed_hosts(_allowed_hosts_from_env())
    bind = os.environ.get("MCP_HOST", "127.0.0.1").strip() or "127.0.0.1"
    kwargs: dict[str, Any] = {}
    if hosts:
        kwargs["enable_dns_rebinding_protection"] = True
        kwargs["allowed_hosts"] = hosts
        origins = _origins_for_hosts(hosts)
        if origins:
            kwargs["allowed_origins"] = origins
    elif bind in {"0.0.0.0", "::", "[::]"}:
        # Bind-all HTTP (container / Helm): do not keep the localhost-only guard.
        kwargs["enable_dns_rebinding_protection"] = False
    else:
        return None

    try:
        return TransportSecuritySettings(**kwargs)
    except TypeError:
        kwargs.pop("allowed_origins", None)
        try:
            return TransportSecuritySettings(**kwargs)
        except TypeError:
            return None


def _fastmcp(name: str, **kwargs: Any) -> FastMCP:
    security = _transport_security_settings()
    if security is not None:
        kwargs["transport_security"] = security
    try:
        return FastMCP(name, **kwargs)
    except TypeError:
        kwargs.pop("transport_security", None)
        return FastMCP(name, **kwargs)


mcp = _fastmcp(
    "org-cost-api",
    instructions=(
        "Read-only access to AWS Org Cost Explorer. "
        "Always call a tool before stating dollar amounts. "
        "Default refresh=false to avoid Cost Explorer API charges. "
        "Costs are usage spend (credits/refunds excluded) for the dashboard date range."
    ),
)


def _account_from_api_list(
    entries: list[dict[str, str]], account: str
) -> dict[str, str] | None:
    key = account.strip().lower()
    for e in entries:
        if e.get("id") == account:
            return e
        if (e.get("name") or "").lower() == key:
            return e
    return None


def _account_hint(entries: list[dict[str, str]]) -> list[str]:
    return [f"{e.get('name')} ({e.get('id')})" for e in entries or []]


@mcp.tool()
def check_mcp_alive() -> str:
    """
    Lightweight ping — confirms this MCP server process is running.
    Use when Cursor shows MCP as disconnected but you want to verify the process responds.
    Does not call the Go API or AWS.
    """
    import time

    return json.dumps(
        {
            "status": "ok",
            "server": "org-cost-api",
            "api_base": base_url(),
            "ts": int(time.time()),
        },
        indent=2,
    )


@mcp.tool()
def check_api_health() -> str:
    """Verify the org-cost-api backend is running, ready, and authenticated (when token configured)."""
    apply_discovered_config()
    status = health_status()
    return json.dumps(status, indent=2)


@mcp.tool()
def check_chat_health() -> str:
    """
    Report conversational Ask / LLM configuration (from config.yaml chat: block or LLM_* env).
    Does not call the LLM — only shows whether org-cost-chat can reach a configured endpoint.
    """
    apply_discovered_config()
    from org_cost_mcp.llm_config import llm_configured, resolve_llm_settings

    settings = resolve_llm_settings()
    out: dict[str, Any] = {
        "api_base": base_url(),
        "llm_configured": llm_configured(),
        **settings.as_health_dict(),
    }
    cfg = os.environ.get("ORG_COST_CONFIG_PATH", "").strip()
    if cfg:
        out["config_path"] = cfg
    if not out["llm_configured"]:
        out["hint"] = (
            "Set chat: in config.yaml or LLM_BASE_URL + LLM_API_KEY (or LLM_PROVIDER=vllm + base URL). "
            "Start org-cost-chat on port 8090 and set VITE_CHAT_URL in the frontend."
        )
    return json.dumps(out, indent=2)


@mcp.tool()
def list_accounts() -> str:
    """
    List configured AWS accounts (id and short name).
    Use account name (e.g. production) or account_id with other tools.
    """
    accounts = fetch_accounts()
    return json.dumps({"accounts": accounts, "count": len(accounts)}, indent=2)


@mcp.tool()
def get_org_summary(refresh: bool = False) -> str:
    """
    Organization-wide AWS usage spend: totals, top cost-driver services,
    and period. Set refresh=true to bypass the 20-minute cache (slower).
    """
    data = fetch_dashboard(refresh=refresh)
    totals = data.get("totals") or {}
    out = {
        "period": {"start": data.get("start"), "end": data.get("end")},
        "generated_at": data.get("generated_at"),
        "totals": {
            "org_total_usd": totals.get("org_total"),
            "ec2_other_usd": totals.get("ec2_other_cost"),
            "account_count": totals.get("account_count"),
            "unattached_ebs_disks": totals.get("volume_available_count"),
            "ebs_disk_count": totals.get("volume_count"),
            "ebs_snapshot_count": totals.get("snapshot_count"),
        },
        "top_services": data.get("top_services") or [],
        "cur_enabled": data.get("cur_enabled", False),
        "note": (
            "org_total is all AWS services (usage). "
            "ec2_other is EC2-Other only. "
            "Investigate savings via get_waste_signals and per-account breakdowns."
        ),
    }
    return json.dumps(out, indent=2)


@mcp.tool()
def get_account_costs(account: str, refresh: bool = False) -> str:
    """
    Per-account cost summary. Pass account name (e.g. production, partner-staging)
    or numeric account_id. Includes top services and EC2-Other total.
    """
    entries = fetch_accounts()
    resolved = _account_from_api_list(entries, account)
    if resolved is None:
        return json.dumps(
            {"error": f"unknown account: {account}", "hint": "use list_accounts"},
            indent=2,
        )
    data = fetch_account_costs(resolved["id"], refresh=refresh)
    return json.dumps(shape_account_costs(data), indent=2)


@mcp.tool()
def get_waste_signals(refresh: bool = False) -> str:
    """
    Waste / cleanup candidates: unattached EBS volumes (status=available),
    snapshot counts, and accounts ranked by unattached disk count.
    Storage (GiB) is provisioned size of unattached volumes only — not total account EBS.
    """
    data = fetch_dashboard(refresh=refresh)
    totals = data.get("totals") or {}
    by_account: list[dict[str, Any]] = []
    account_distribution: list[dict[str, Any]] = []
    gib_per_month = 0.08  # gp3 ballpark USD/GiB-month

    for acct in data.get("accounts") or []:
        vols = acct.get("volumes") or {}
        snaps = acct.get("snapshots") or {}
        avail = int(vols.get("available_count") or 0)
        avail_gib = float(vols.get("available_gib") or 0)
        if avail > 0 or (snaps.get("count") or 0) > 0:
            est_monthly = round(avail_gib * gib_per_month, 2)
            row = {
                "account_name": acct.get("account_name"),
                "account_id": acct.get("account_id"),
                "unattached_ebs_disks": avail,
                "unattached_ebs_gib": avail_gib,
                "estimated_unattached_monthly_usd": est_monthly,
                "total_ebs_disks": vols.get("count"),
                "total_ebs_gib": vols.get("total_gib"),
                "snapshot_count": snaps.get("count"),
                "snapshot_gib": snaps.get("total_size_gib"),
            }
            by_account.append(row)
            if avail > 0:
                account_distribution.append(
                    {
                        "account": acct.get("account_name"),
                        "unattached_volumes": avail,
                        "storage_gib": avail_gib,
                        "estimated_monthly_usd": est_monthly,
                    }
                )

    by_account.sort(key=lambda x: x.get("unattached_ebs_disks", 0), reverse=True)
    account_distribution.sort(
        key=lambda x: x.get("estimated_monthly_usd", 0), reverse=True
    )
    org_avail_gib = float(totals.get("volume_available_gib") or 0)
    out = {
        "period": {"start": data.get("start"), "end": data.get("end")},
        "organization": {
            "unattached_ebs_disks": totals.get("volume_available_count"),
            "unattached_ebs_gib": org_avail_gib,
            "estimated_unattached_monthly_usd": round(org_avail_gib * gib_per_month, 2),
            "ebs_disk_count": totals.get("volume_count"),
            "ebs_disk_gib": totals.get("volume_size_gib"),
            "snapshot_count": totals.get("snapshot_count"),
        },
        "account_distribution": account_distribution,
        "accounts_with_signals": by_account,
        "guidance": [
            "storage_gib / unattached_ebs_gib = provisioned size of available (unattached) volumes only.",
            "estimated_monthly_usd uses ~$0.08/GiB-month (gp3 ballpark); actual varies by volume type/region.",
            "Unattached EBS (available) are strong delete/attach candidates — verify in console.",
            "Use get_account_costs + get_service_detail for spend drivers, not just inventory.",
        ],
    }
    return json.dumps(out, indent=2)


@mcp.tool()
def get_service_detail(
    account: str,
    service: str,
    start: str = "",
    end: str = "",
    refresh: bool = False,
) -> str:
    """
    Drill-down for one AWS service in one account (Cost Explorer usage).
    service must be the Cost Explorer SERVICE name (e.g. 'Amazon Simple Storage Service'
    or 'EC2 - Other'); copy exact name from get_account_costs top_services.
    account: name or id. Optional start/end YYYY-MM-DD (defaults to dashboard period).
    """
    entries = fetch_accounts()
    resolved = _account_from_api_list(entries, account)
    if resolved is None:
        return json.dumps(
            {"error": f"unknown account: {account}", "hint": "use list_accounts"},
            indent=2,
        )
    account_id = resolved["id"]
    if not start or not end:
        dash = fetch_dashboard(refresh=refresh)
        start = start or dash.get("start", "")
        end = end or dash.get("end", "")
    detail = fetch_service_detail(account_id, service, start=start, end=end)
    # Trim large daily arrays for LLM context
    daily = detail.get("daily") or []
    if len(daily) > 14:
        detail["daily"] = daily[-14:]
        detail["daily_note"] = "last 14 days shown"
    for key in ("by_usage_type", "by_region", "by_operation", "by_name_tag"):
        items = detail.get(key) or []
        if len(items) > 15:
            detail[key] = items[:15]
            detail[key + "_note"] = "top 15 shown"
    return json.dumps(detail, indent=2)


@mcp.tool()
def get_cost_trends(refresh: bool = False, service: str = "") -> str:
    """
    Compare current vs prior period AWS spend by service (e.g. Redshift +300%).
    Uses saved daily snapshots when available — avoids Cost Explorer API calls.
    Set refresh=true only when you need live AWS data (slower, more CE usage).
    Optional service: filter to one service name (partial match, e.g. Redshift).
    """
    data = fetch_trends(refresh=refresh)
    if service.strip():
        key = service.strip().lower()
        trends = data.get("service_trends") or []
        filtered = [
            t
            for t in trends
            if key in (t.get("service") or "").lower()
            or key in (t.get("display_name") or "").lower()
        ]
        data["service_trends"] = filtered
        data["top_increases"] = [
            t
            for t in (data.get("top_increases") or [])
            if key in (t.get("service") or "").lower()
            or key in (t.get("display_name") or "").lower()
        ]
        data["top_decreases"] = [
            t
            for t in (data.get("top_decreases") or [])
            if key in (t.get("service") or "").lower()
            or key in (t.get("display_name") or "").lower()
        ]
        data["filter"] = service
    out = {
        "current_period": data.get("current_period"),
        "prior_period": data.get("prior_period"),
        "prior_source": data.get("prior_source"),
        "ce_calls_used": data.get("ce_calls_used", 0),
        "history_note": data.get("history_note"),
        "snapshot_count": data.get("snapshot_count"),
        "org_total_change": data.get("org_total"),
        "top_increases": data.get("top_increases") or [],
        "top_decreases": data.get("top_decreases") or [],
        "service_trends": (data.get("service_trends") or [])[:25],
        "tip": (
            "Run the API daily (or ask for org summary once per day) to build history "
            "and avoid prior-period CE calls. Do not use refresh=true unless the user asks."
        ),
    }
    if data.get("filter"):
        out["filter"] = data["filter"]
    return json.dumps(out, indent=2)


@mcp.tool()
def get_service_tag_delta(
    service: str,
    account: str = "",
    tag_key: str = "Name",
    top_accounts: int = 5,
    start: str = "",
    end: str = "",
) -> str:
    """
    Explain why a service spend changed using cost allocation tag buckets.

    This is an "enrichment" / cached explainability tool:
    - Default tag_key is "Name" (your activated Cost Explorer cost allocation tag key).
    - Uses the backend cache when available (fast/cheap).
    - If cache is cold, it may trigger extra Cost Explorer tag-grouping calls.

    account is optional (account name or id). If omitted, aggregates across top contributing accounts.
    start/end are optional YYYY-MM-DD (defaults to the backend dashboard period).
    """
    if not service.strip():
        return json.dumps({"error": "service is required"}, indent=2)

    account_id = ""
    if account.strip():
        entries = fetch_accounts()
        resolved = _account_from_api_list(entries, account)
        if resolved is None:
            return json.dumps(
                {
                    "error": f"unknown account: {account}",
                    "hint": "use list_accounts",
                    "available": _account_hint(entries)[:30],
                },
                indent=2,
            )
        account_id = resolved["id"]

    data = fetch_service_tag_delta(
        service=service.strip(),
        account_id=account_id,
        start=start.strip(),
        end=end.strip(),
        tag_key=tag_key.strip() or "Name",
        top_accounts=top_accounts,
    )

    # Null-safety for Hermes.
    data["top_increases"] = data.get("top_increases") or []
    data["top_decreases"] = data.get("top_decreases") or []
    for acct in data.get("accounts") or []:
        acct["top_increases"] = acct.get("top_increases") or []
        acct["top_decreases"] = acct.get("top_decreases") or []

    # Trim to keep the tool response small for LLM context.
    data["top_increases"] = (data["top_increases"] or [])[:10]
    data["top_decreases"] = (data["top_decreases"] or [])[:10]
    for acct in data.get("accounts") or []:
        acct["top_increases"] = (acct.get("top_increases") or [])[:10]
        acct["top_decreases"] = (acct.get("top_decreases") or [])[:10]

    return json.dumps(data, indent=2)


@mcp.tool()
def get_service_tag_totals(
    service: str,
    account: str = "",
    tag_key: str = "Name",
    top_buckets: int = 10,
    start: str = "",
    end: str = "",
) -> str:
    """
    "Most expensive buckets" for one service, grouped by a cost allocation tag key.

    Use this for questions like:
    - "What are the most expensive S3 buckets?"
    - "Which Name-tagged resources are driving S3 spend?"

    This is *not* a delta explanation; it reports current spend by tag buckets.
    """
    if not service.strip():
        return json.dumps({"error": "service is required"}, indent=2)

    account_id = ""
    if account.strip():
        entries = fetch_accounts()
        resolved = _account_from_api_list(entries, account)
        if resolved is None:
            return json.dumps(
                {
                    "error": f"unknown account: {account}",
                    "hint": "use list_accounts",
                    "available": _account_hint(entries)[:30],
                },
                indent=2,
            )
        account_id = resolved["id"]

    data = fetch_service_tag_totals(
        service=service.strip(),
        account_id=account_id,
        start=start.strip(),
        end=end.strip(),
        tag_key=tag_key.strip() or "Name",
        top_buckets=top_buckets,
    )

    data["buckets"] = data.get("buckets") or []
    return json.dumps(data, indent=2)


@mcp.tool()
def get_cost_suggestions(refresh: bool = False) -> str:
    """
    Ranked cost optimization suggestions: waste (unattached EBS), spend spikes,
    concentration risk, and rightsizing hints. Uses dashboard cache + trends history.
    Set refresh=true only when the user needs live AWS data.
    """
    data = fetch_suggestions(refresh=refresh)
    suggestions = data.get("suggestions") or []
    out = {
        "summary": data.get("summary"),
        "period": data.get("period"),
        "ce_calls_used": data.get("ce_calls_used", 0),
        "data_sources": data.get("data_sources"),
        "suggestions": suggestions[:12],
        "next_steps": [
            "Pick the highest-priority item and run get_service_detail or get_waste_signals.",
            "Use get_cost_trends service=Redshift (or other) to quantify a spike.",
            "Avoid refresh=true unless the user explicitly wants live AWS pulls.",
        ],
    }
    return json.dumps(out, indent=2)


@mcp.tool()
def ask_cost_question(question: str, refresh: bool = False) -> str:
    """
    Ask a natural-language question about org AWS costs (same as the browser Ask tab).

    Examples: "What is our total spend?", "Which account has the most waste?",
    "How did Redshift change vs last period?"

    Set refresh=true only when the user needs live AWS data (slower, more CE usage).
    """
    question = question.strip()
    if not question:
        return json.dumps({"error": "question is required"}, indent=2)
    data = fetch_ask(question, refresh=refresh)
    return json.dumps(data, indent=2)


@mcp.tool()
def get_cost_report(refresh: bool = False, html: bool = True) -> str:
    """
    Visual cost report for Hermes: markdown with Mermaid charts (org spend, trends,
    savings) plus an optional HTML file you can open in a browser.
    Uses cached data by default — no refresh unless the user asks for live AWS data.
    Prefer this when the user wants charts, a dashboard, or a visual summary.
    """
    report = fetch_report(refresh=refresh)
    dashboard = report.get("dashboard") or {}
    trends = report.get("trends") or {}
    suggestions = report.get("suggestions") or {}

    html_path: str | None = None
    if html:
        html_path = write_html_report(dashboard, trends, suggestions)

    report = build_markdown_report(
        dashboard, trends, suggestions, html_path=html_path
    )
    return report


def main() -> None:
    apply_discovered_config()
    transport = os.environ.get("MCP_TRANSPORT", "stdio").strip().lower()
    if transport not in ("stdio", "sse", "streamable-http"):
        raise SystemExit(
            f"Invalid MCP_TRANSPORT={transport!r} — use stdio, sse, or streamable-http"
        )

    if transport in ("sse", "streamable-http"):
        mcp.settings.host = os.environ.get("MCP_HOST", "0.0.0.0")
        mcp.settings.port = int(os.environ.get("MCP_PORT", "8000"))
        security = _transport_security_settings()
        if security is not None:
            for attr in ("transport_security", "_transport_security"):
                if hasattr(mcp, attr):
                    setattr(mcp, attr, security)

    mcp.run(transport=transport)  # type: ignore[arg-type]


if __name__ == "__main__":
    main()
