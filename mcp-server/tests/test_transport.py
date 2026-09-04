"""Tests for MCP transport configuration."""

from __future__ import annotations

import os
from unittest.mock import MagicMock, patch

import pytest

from org_cost_mcp import server


def test_main_rejects_invalid_transport(monkeypatch):
    monkeypatch.setenv("MCP_TRANSPORT", "websocket")
    with pytest.raises(SystemExit):
        server.main()


@patch.object(server.mcp, "run")
def test_main_defaults_to_stdio(mock_run, monkeypatch):
    monkeypatch.delenv("MCP_TRANSPORT", raising=False)
    server.main()
    mock_run.assert_called_once_with(transport="stdio")


@patch.object(server.mcp, "run")
def test_main_sse_sets_host_port(mock_run, monkeypatch):
    monkeypatch.setenv("MCP_TRANSPORT", "sse")
    monkeypatch.setenv("MCP_HOST", "0.0.0.0")
    monkeypatch.setenv("MCP_PORT", "9000")
    server.main()
    assert server.mcp.settings.host == "0.0.0.0"
    assert server.mcp.settings.port == 9000
    mock_run.assert_called_once_with(transport="sse")


@patch.object(server.mcp, "run")
def test_main_streamable_http(mock_run, monkeypatch):
    monkeypatch.setenv("MCP_TRANSPORT", "streamable-http")
    server.main()
    mock_run.assert_called_once_with(transport="streamable-http")
