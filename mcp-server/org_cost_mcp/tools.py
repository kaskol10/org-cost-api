"""OpenAI-style tool schemas and dispatch for the chat agent."""

from __future__ import annotations

import json
from typing import Any, Callable

from org_cost_mcp import server

TOOL_SCHEMAS: list[dict[str, Any]] = [
    {
        "type": "function",
        "function": {
            "name": "get_org_summary",
            "description": (
                "Organization-wide AWS usage spend: totals, top cost-driver services, "
                "and period. Default refresh=false."
            ),
            "parameters": {
                "type": "object",
                "properties": {
                    "refresh": {
                        "type": "boolean",
                        "description": "Bypass cache and pull live AWS data (slower).",
                    }
                },
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "get_cost_trends",
            "description": (
                "Compare current vs prior period spend by service. "
                "Uses history snapshots when available."
            ),
            "parameters": {
                "type": "object",
                "properties": {
                    "refresh": {"type": "boolean"},
                    "service": {
                        "type": "string",
                        "description": "Optional service filter (partial match, e.g. Redshift).",
                    },
                },
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "get_cost_suggestions",
            "description": "Ranked cost optimization suggestions from dashboard cache and trends.",
            "parameters": {
                "type": "object",
                "properties": {"refresh": {"type": "boolean"}},
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "get_waste_signals",
            "description": "Unattached EBS volumes, snapshot counts, accounts ranked by waste.",
            "parameters": {
                "type": "object",
                "properties": {"refresh": {"type": "boolean"}},
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "get_account_costs",
            "description": "Per-account cost summary. Pass account name or account_id.",
            "parameters": {
                "type": "object",
                "properties": {
                    "account": {
                        "type": "string",
                        "description": "Account name (e.g. production) or numeric id.",
                    },
                    "refresh": {"type": "boolean"},
                },
                "required": ["account"],
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "get_service_detail",
            "description": (
                "Drill-down for one AWS service in one account. "
                "Use exact Cost Explorer service name from get_account_costs."
            ),
            "parameters": {
                "type": "object",
                "properties": {
                    "account": {"type": "string"},
                    "service": {"type": "string"},
                    "start": {"type": "string", "description": "YYYY-MM-DD"},
                    "end": {"type": "string", "description": "YYYY-MM-DD"},
                    "refresh": {"type": "boolean"},
                },
                "required": ["account", "service"],
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "ask_cost_question",
            "description": (
                "Quick rule-based answer for simple questions (same as browser Ask tab). "
                "Prefer specific tools for follow-ups."
            ),
            "parameters": {
                "type": "object",
                "properties": {
                    "question": {"type": "string"},
                    "refresh": {"type": "boolean"},
                },
                "required": ["question"],
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "list_accounts",
            "description": "List configured AWS accounts (id and short name).",
            "parameters": {"type": "object", "properties": {}},
        },
    },
]

_TOOL_HANDLERS: dict[str, Callable[..., str]] = {
    "get_org_summary": server.get_org_summary,
    "get_cost_trends": server.get_cost_trends,
    "get_cost_suggestions": server.get_cost_suggestions,
    "get_waste_signals": server.get_waste_signals,
    "get_account_costs": server.get_account_costs,
    "get_service_detail": server.get_service_detail,
    "ask_cost_question": server.ask_cost_question,
    "list_accounts": server.list_accounts,
}


def execute_tool(name: str, arguments: dict[str, Any] | None) -> str:
    """Run a tool by name; returns JSON string for the LLM."""
    handler = _TOOL_HANDLERS.get(name)
    if handler is None:
        return json.dumps({"error": f"unknown tool: {name}"})
    args = arguments or {}
    try:
        return handler(**args)
    except TypeError as exc:
        return json.dumps({"error": f"invalid arguments for {name}: {exc}"})
    except Exception as exc:  # noqa: BLE001 — surface API errors to the model
        return json.dumps({"error": str(exc)})


def ce_calls_in_result(tool_result: str) -> int:
    """Extract ce_calls_used from a tool JSON payload if present."""
    try:
        data = json.loads(tool_result)
    except json.JSONDecodeError:
        return 0
    if not isinstance(data, dict):
        return 0
    raw = data.get("ce_calls_used")
    if isinstance(raw, (int, float)):
        return int(raw)
    return 0
