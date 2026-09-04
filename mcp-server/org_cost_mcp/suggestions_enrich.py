"""LLM-powered explanations for rule-based cost savings suggestions."""

from __future__ import annotations

import json
import re
from typing import Any

import litellm

from org_cost_mcp.client import fetch_report
from org_cost_mcp.llm_config import llm_completion_kwargs, llm_configured

ENRICH_SYSTEM = """You are a FinOps analyst explaining AWS cost savings opportunities.

You receive:
1) Rule-based suggestions (deterministic — do NOT change their titles, priorities, or dollar estimates)
2) Trimmed org cost data (dashboard, trends, waste signals)

Your job:
- Write a clear `narrative_summary` (2-4 sentences) for leadership.
- For each rule-based suggestion id, add an `explanation` (markdown, 2-5 sentences) citing specific numbers from the data.
- Optionally add `additional_insights` (max 3 bullets) ONLY if supported by the data — not duplicates of existing suggestions.
- Set `confidence` per suggestion: high | medium | low.

Rules:
- Never invent dollar amounts or account names not in the data.
- If data is insufficient to explain a suggestion, say what is missing.
- Do not recommend destructive actions without "verify in console" caveats.
- Return valid JSON only, matching the schema exactly.
"""


def _trim_dashboard(dashboard: dict[str, Any]) -> dict[str, Any]:
    totals = dashboard.get("totals") or {}
    accounts_out: list[dict[str, Any]] = []
    for acct in (dashboard.get("accounts") or [])[:12]:
        if acct.get("error"):
            continue
        row: dict[str, Any] = {
            "account_name": acct.get("account_name"),
            "account_id": acct.get("account_id"),
        }
        costs = acct.get("costs") or {}
        if costs:
            row["all_total"] = costs.get("all_total")
            row["other_services_total"] = costs.get("other_services_total")
            by_svc = costs.get("by_service") or []
            row["top_services"] = by_svc[:6]
        vols = acct.get("volumes") or {}
        if vols:
            row["volumes"] = {
                "available_count": vols.get("available_count"),
                "available_gib": vols.get("available_gib"),
                "count": vols.get("count"),
            }
        snaps = acct.get("snapshots") or {}
        if snaps.get("count"):
            row["snapshots"] = {"count": snaps.get("count")}
        accounts_out.append(row)

    top_services = (dashboard.get("top_services") or [])[:10]
    return {
        "period": {"start": dashboard.get("start"), "end": dashboard.get("end")},
        "totals": {
            "org_total": totals.get("org_total"),
            "volume_available_count": totals.get("volume_available_count"),
            "volume_available_gib": totals.get("volume_available_gib"),
            "volume_utilization_percent": totals.get("volume_utilization_percent"),
            "snapshot_count": totals.get("snapshot_count"),
        },
        "top_services": top_services,
        "accounts": accounts_out,
    }


def _trim_trends(trends: dict[str, Any]) -> dict[str, Any]:
    return {
        "current_period": trends.get("current_period"),
        "prior_period": trends.get("prior_period"),
        "prior_source": trends.get("prior_source"),
        "org_total": trends.get("org_total"),
        "top_increases": (trends.get("top_increases") or [])[:8],
        "top_decreases": (trends.get("top_decreases") or [])[:5],
        "history_note": trends.get("history_note"),
    }


def build_report_context(report: dict[str, Any]) -> dict[str, Any]:
    return {
        "dashboard": _trim_dashboard(report.get("dashboard") or {}),
        "trends": _trim_trends(report.get("trends") or {}),
        "rule_suggestions": report.get("suggestions") or {},
    }


def _parse_llm_json(content: str) -> dict[str, Any]:
    text = content.strip()
    if text.startswith("```"):
        text = re.sub(r"^```(?:json)?\s*", "", text)
        text = re.sub(r"\s*```$", "", text)
    data = json.loads(text)
    if not isinstance(data, dict):
        raise ValueError("LLM response is not a JSON object")
    return data


def merge_enrichment(
    base: dict[str, Any],
    enrichment: dict[str, Any],
) -> dict[str, Any]:
    """Merge LLM explanations into rule-based suggestions by id."""
    out = dict(base)
    by_id = {
        str(item.get("id", "")): item
        for item in enrichment.get("suggestions") or []
        if isinstance(item, dict)
    }
    merged: list[dict[str, Any]] = []
    for item in out.get("suggestions") or []:
        if not isinstance(item, dict):
            continue
        row = dict(item)
        extra = by_id.get(str(row.get("id", "")))
        if extra:
            if extra.get("explanation"):
                row["explanation"] = str(extra["explanation"])
            if extra.get("confidence"):
                row["confidence"] = str(extra["confidence"])
        merged.append(row)
    out["suggestions"] = merged
    if enrichment.get("narrative_summary"):
        out["narrative_summary"] = str(enrichment["narrative_summary"])
    insights = enrichment.get("additional_insights")
    if isinstance(insights, list):
        out["additional_insights"] = [str(x) for x in insights[:3]]
    out["llm_enriched"] = True
    return out


def _call_enrichment_llm(user_payload: str, schema_hint: str) -> str:
    messages = [
        {"role": "system", "content": ENRICH_SYSTEM},
        {
            "role": "user",
            "content": (
                "Explain these cost savings opportunities using ONLY the data below.\n\n"
                f"JSON schema:\n{schema_hint}\n\n"
                f"Data:\n{user_payload}"
            ),
        },
    ]
    kwargs = llm_completion_kwargs()
    try:
        response = litellm.completion(
            messages=messages,
            response_format={"type": "json_object"},
            stream=False,
            **kwargs,
        )
    except Exception:
        response = litellm.completion(
            messages=messages,
            stream=False,
            **kwargs,
        )
    choice = response.choices[0].message
    content = choice.content if isinstance(choice.content, str) else ""
    if not content:
        raise RuntimeError("LLM returned empty response")
    return content


def enrich_suggestions(
    *,
    refresh: bool = False,
    report: dict[str, Any] | None = None,
) -> dict[str, Any]:
    """
    Add LLM explanations to rule-based suggestions.

    When `report` is provided (e.g. from the browser), the chat agent does not
    need to re-fetch the Go API — avoids 401s when only the frontend has a token.

    Raises RuntimeError if LLM is not configured or the model call fails.
    """
    if not llm_configured():
        raise RuntimeError("LLM is not configured (set LLM_BASE_URL or chat: in config.yaml)")

    if report is None or refresh:
        report = fetch_report(refresh=refresh)
    base = report.get("suggestions") or {}
    if not isinstance(base, dict):
        base = {}

    context = build_report_context(report)
    user_payload = json.dumps(context, indent=2)

    schema_hint = """{
  "narrative_summary": "string",
  "suggestions": [
    {"id": "<rule suggestion id>", "explanation": "markdown", "confidence": "high|medium|low"}
  ],
  "additional_insights": ["optional bullet strings"]
}"""

    content = _call_enrichment_llm(user_payload, schema_hint)
    enrichment = _parse_llm_json(content)
    return merge_enrichment(base, enrichment)
