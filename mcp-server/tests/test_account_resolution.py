"""Account resolution tests for MCP tools that accept an account parameter."""

from __future__ import annotations

import json
from unittest.mock import patch

import pytest

from org_cost_mcp import server

SAMPLE_ACCOUNTS = [
    {"id": "123456789012", "name": "production"},
    {"id": "210987654321", "name": "partner-staging"},
]


@pytest.fixture
def mock_accounts():
    with patch.object(server, "fetch_accounts", return_value=SAMPLE_ACCOUNTS):
        yield


@pytest.mark.parametrize(
    "account,expected_id",
    [
        ("production", "123456789012"),
        ("123456789012", "123456789012"),
        ("partner-staging", "210987654321"),
        ("Partner-Staging", "210987654321"),
    ],
)
def test_account_from_api_list_resolves(account, expected_id):
    resolved = server._account_from_api_list(SAMPLE_ACCOUNTS, account)
    assert resolved is not None
    assert resolved["id"] == expected_id


def test_account_from_api_list_unknown():
    assert server._account_from_api_list(SAMPLE_ACCOUNTS, "nonexistent") is None


def test_account_hint_format():
    hints = server._account_hint(SAMPLE_ACCOUNTS)
    assert hints == ["production (123456789012)", "partner-staging (210987654321)"]


@patch.object(server, "fetch_dashboard", return_value={"start": "2026-08-01", "end": "2026-08-31"})
@patch.object(server, "fetch_service_detail", return_value={"service": "S3"})
def test_get_service_detail_resolves_by_name(mock_detail, _mock_dash, mock_accounts):
    out = json.loads(server.get_service_detail("production", "Amazon S3"))
    assert "error" not in out
    mock_detail.assert_called_once_with(
        "123456789012", "Amazon S3", start="2026-08-01", end="2026-08-31"
    )


@patch.object(server, "fetch_service_detail")
def test_get_service_detail_unknown_account(mock_detail, mock_accounts):
    out = json.loads(server.get_service_detail("unknown", "Amazon S3"))
    assert out["error"] == "unknown account: unknown"
    assert out["hint"] == "use list_accounts"
    mock_detail.assert_not_called()


@patch.object(server, "fetch_account_costs", return_value={"account_id": "123456789012", "costs": {}})
def test_get_account_costs_resolves_by_name(mock_fetch, mock_accounts):
    server.get_account_costs("production")
    mock_fetch.assert_called_once_with("123456789012", refresh=False)


@patch.object(server, "fetch_account_costs")
def test_get_account_costs_unknown_account(mock_fetch, mock_accounts):
    out = json.loads(server.get_account_costs("unknown"))
    assert out["error"] == "unknown account: unknown"
    assert out["hint"] == "use list_accounts"
    mock_fetch.assert_not_called()


@patch.object(server, "fetch_service_tag_delta", return_value={"top_increases": []})
def test_get_service_tag_delta_resolves_by_id(mock_delta, mock_accounts):
    out = json.loads(
        server.get_service_tag_delta("Amazon Simple Storage Service", account="123456789012")
    )
    assert "error" not in out
    mock_delta.assert_called_once()
    assert mock_delta.call_args.kwargs["account_id"] == "123456789012"


@patch.object(server, "fetch_service_tag_delta")
def test_get_service_tag_delta_unknown_account(mock_delta, mock_accounts):
    out = json.loads(
        server.get_service_tag_delta("Amazon Simple Storage Service", account="missing")
    )
    assert out["error"] == "unknown account: missing"
    assert "partner-staging (210987654321)" in out["available"]
    mock_delta.assert_not_called()


@patch.object(server, "fetch_service_tag_totals", return_value={"buckets": []})
def test_get_service_tag_totals_resolves_by_name(mock_totals, mock_accounts):
    out = json.loads(
        server.get_service_tag_totals("Amazon Simple Storage Service", account="partner-staging")
    )
    assert "error" not in out
    mock_totals.assert_called_once()
    assert mock_totals.call_args.kwargs["account_id"] == "210987654321"


@patch.object(server, "fetch_service_tag_totals")
def test_get_service_tag_totals_unknown_account(mock_totals, mock_accounts):
    out = json.loads(
        server.get_service_tag_totals("Amazon Simple Storage Service", account="nope")
    )
    assert out["error"] == "unknown account: nope"
    mock_totals.assert_not_called()
