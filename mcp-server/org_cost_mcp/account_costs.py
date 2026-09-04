"""Account cost JSON shaping for MCP tools."""

from __future__ import annotations

from typing import Any


def shape_account_costs(acct: dict[str, Any]) -> dict[str, Any]:
    """Map /api/account-costs (or dashboard account) JSON to MCP output."""
    costs = acct.get("costs") or {}
    period = acct.get("period") or {"start": costs.get("start"), "end": costs.get("end")}
    return {
        "account_id": acct.get("account_id"),
        "account_name": acct.get("account_name"),
        "period": period,
        "all_services_total_usd": costs.get("all_total"),
        "ec2_other_usd": costs.get("total"),
        "other_services_usd": costs.get("other_services_total"),
        "top_services": costs.get("by_service") or [],
        "usage_categories": [
            {
                "id": c.get("id"),
                "label": c.get("label"),
                "amount": c.get("amount"),
            }
            for c in (costs.get("usage_categories") or [])[:8]
        ],
    }
