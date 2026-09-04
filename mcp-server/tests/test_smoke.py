"""Smoke tests for MCP tool cross-layer behavior."""

from __future__ import annotations

import json
from unittest.mock import patch

from org_cost_mcp import server

SAMPLE_TRENDS = {
    "current_period": {"start": "2026-08-01", "end": "2026-08-31"},
    "prior_period": {"start": "2026-07-01", "end": "2026-07-31"},
    "prior_source": "history_snapshot",
    "ce_calls_used": 0,
    "org_total": {"current_usd": 1000, "prior_usd": 900},
    "service_trends": [
        {
            "service": "Amazon Redshift",
            "display_name": "Redshift",
            "current_usd": 300,
            "prior_usd": 100,
        },
        {
            "service": "Amazon Simple Storage Service",
            "display_name": "S3",
            "current_usd": 200,
            "prior_usd": 150,
        },
    ],
    "top_increases": [
        {"service": "Amazon Redshift", "display_name": "Redshift"},
        {"service": "Amazon Simple Storage Service", "display_name": "S3"},
    ],
    "top_decreases": [
        {"service": "Amazon Redshift", "display_name": "Redshift"},
        {"service": "AWS Lambda", "display_name": "Lambda"},
    ],
}


@patch.object(server, "fetch_trends", return_value=SAMPLE_TRENDS)
def test_get_cost_trends_filters_top_decreases(mock_fetch):
    out = json.loads(server.get_cost_trends(service="redshift"))
    assert out["filter"] == "redshift"
    assert len(out["top_increases"]) == 1
    assert out["top_increases"][0]["display_name"] == "Redshift"
    assert len(out["top_decreases"]) == 1
    assert out["top_decreases"][0]["display_name"] == "Redshift"
    assert len(out["service_trends"]) == 1
    mock_fetch.assert_called_once_with(refresh=False)


@patch.object(server, "fetch_accounts", return_value=[{"id": "1", "name": "production"}])
@patch.object(server, "fetch_account_costs")
def test_get_account_costs_smoke_unknown_account(mock_fetch, _mock_accounts):
    out = json.loads(server.get_account_costs("typo"))
    assert "error" in out
    assert out["hint"] == "use list_accounts"
    mock_fetch.assert_not_called()
