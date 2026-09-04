"""Tests for org-cost-api HTTP client."""

from __future__ import annotations

from unittest.mock import MagicMock, patch

import httpx

from org_cost_mcp.client import _get, _reset_http_client, _with_http_retry, health_status


@patch("org_cost_mcp.client._get")
def test_health_status_includes_ready_detail_when_not_ready(mock_get):
    mock_get.side_effect = [
        {"status": "ok"},
        {
            "ready": False,
            "billing_check": "failed",
            "billing_error": "sso_expired",
            "accounts_check": "ok",
        },
        {"demo": False},
    ]
    out = health_status()
    assert out["api_reachable"] is True
    assert out["healthy"] is False
    assert out["ready"] is False
    assert out["ready_detail"]["billing_error"] == "sso_expired"
    assert "hint" in out


@patch("org_cost_mcp.client._get")
def test_health_status_connection_error(mock_get):
    mock_get.side_effect = ConnectionError("Connection refused")
    out = health_status()
    assert out["healthy"] is False
    assert out["api_reachable"] is False
    assert "Connection refused" in out["error"]


def test_with_http_retry_recovers_after_transient_error():
    calls = {"n": 0}

    def flaky() -> str:
        calls["n"] += 1
        if calls["n"] == 1:
            raise httpx.RemoteProtocolError("Server disconnected")
        return "ok"

    assert _with_http_retry(flaky) == "ok"
    assert calls["n"] == 2


@patch("org_cost_mcp.client._http_client")
def test_get_retries_on_stale_connection(mock_client_factory):
    _reset_http_client()
    client = MagicMock()
    mock_client_factory.return_value = client
    ok = MagicMock()
    ok.raise_for_status.return_value = None
    ok.json.return_value = {"status": "ok"}
    client.get.side_effect = [httpx.ReadError("stale"), ok]

    out = _get("/api/health")
    assert out == {"status": "ok"}
    assert client.get.call_count == 2
