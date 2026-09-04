"""Tests for remaining MCP tools and visualize helpers."""

from __future__ import annotations

import json
import tempfile
from pathlib import Path
from unittest.mock import patch

from org_cost_mcp import server
from org_cost_mcp.visualize import build_markdown_report, write_html_report

SAMPLE_DASHBOARD = {
    "start": "2026-08-01",
    "end": "2026-08-31",
    "generated_at": "2026-08-31T12:00:00Z",
    "totals": {
        "org_total": 5000,
        "ec2_other_cost": 800,
        "account_count": 2,
        "volume_available_count": 3,
        "volume_count": 10,
        "snapshot_count": 5,
    },
    "top_services": [
        {"service": "Amazon Simple Storage Service", "amount": 2000, "percent": 40},
    ],
    "accounts": [
        {
            "account_name": "production",
            "account_id": "123",
            "volumes": {"available_count": 2, "available_gib": 100, "count": 5},
            "snapshots": {"count": 3, "total_size_gib": 50},
        }
    ],
}

SAMPLE_TRENDS = {
    "current_period": {"start": "2026-08-01", "end": "2026-08-31"},
    "prior_period": {"start": "2026-07-01", "end": "2026-07-31"},
    "prior_source": "history_snapshot",
    "ce_calls_used": 0,
    "org_total": {"current_usd": 5000, "prior_usd": 4500, "change_usd": 500, "change_percent": 11.1},
    "top_increases": [
        {
            "service": "Amazon Redshift",
            "display_name": "Redshift",
            "current_usd": 300,
            "prior_usd": 100,
            "change_usd": 200,
            "change_percent": 200,
        }
    ],
    "top_decreases": [],
}

SAMPLE_SUGGESTIONS = {
    "summary": "2 suggestions based on trends and inventory.",
    "suggestions": [
        {
            "priority": 1,
            "category": "waste",
            "title": "Unattached EBS volumes",
            "detail": "Delete or attach 3 volumes.",
            "estimated_monthly_usd": 24,
        }
    ],
}

SAMPLE_REPORT = {
    "dashboard": SAMPLE_DASHBOARD,
    "trends": SAMPLE_TRENDS,
    "suggestions": SAMPLE_SUGGESTIONS,
    "ce_calls_used": 0,
    "refresh_allowed": True,
}


@patch.object(
    server,
    "health_status",
    return_value={
        "api_base": "http://localhost:8080",
        "api_reachable": True,
        "healthy": True,
        "ready": True,
        "demo": False,
        "auth_ok": True,
    },
)
def test_check_api_health(mock_health):
    out = json.loads(server.check_api_health())
    assert out["healthy"] is True
    assert "api_base" in out
    mock_health.assert_called_once()


@patch("org_cost_mcp.llm_config.llm_configured", return_value=True)
@patch("org_cost_mcp.llm_config.resolve_llm_settings")
def test_check_chat_health(mock_settings, mock_llm_configured):
    mock_settings.return_value.as_health_dict.return_value = {
        "llm_provider": "vllm",
        "llm_model": "hosted_vllm/test",
        "llm_base_url": "http://localhost:8000/v1",
    }
    out = json.loads(server.check_chat_health())
    assert out["llm_configured"] is True
    assert out["llm_provider"] == "vllm"


@patch.object(
    server,
    "fetch_accounts",
    return_value=[{"id": "123", "name": "production"}],
)
def test_list_accounts(mock_fetch):
    out = json.loads(server.list_accounts())
    assert out["count"] == 1
    assert out["accounts"][0]["name"] == "production"
    mock_fetch.assert_called_once()


@patch.object(server, "fetch_dashboard", return_value=SAMPLE_DASHBOARD)
def test_get_org_summary(mock_fetch):
    out = json.loads(server.get_org_summary())
    assert out["period"]["start"] == "2026-08-01"
    assert out["totals"]["org_total_usd"] == 5000
    assert out["top_services"]
    assert "note" in out
    mock_fetch.assert_called_once_with(refresh=False)


@patch.object(server, "fetch_dashboard", return_value=SAMPLE_DASHBOARD)
def test_get_waste_signals(mock_fetch):
    out = json.loads(server.get_waste_signals())
    assert out["organization"]["unattached_ebs_disks"] == 3
    assert len(out["accounts_with_signals"]) == 1
    assert out["guidance"]
    mock_fetch.assert_called_once_with(refresh=False)


@patch.object(server, "fetch_suggestions", return_value=SAMPLE_SUGGESTIONS)
def test_get_cost_suggestions(mock_fetch):
    out = json.loads(server.get_cost_suggestions())
    assert out["summary"]
    assert len(out["suggestions"]) == 1
    assert out["next_steps"]
    mock_fetch.assert_called_once_with(refresh=False)


@patch.object(
    server,
    "fetch_ask",
    return_value={
        "answer": "Org total is $5,000",
        "intent": "org_summary",
        "ce_calls_used": 0,
        "sources": ["dashboard"],
    },
)
def test_ask_cost_question(mock_fetch):
    out = json.loads(server.ask_cost_question("What is our total spend?"))
    assert out["intent"] == "org_summary"
    assert "answer" in out
    mock_fetch.assert_called_once_with("What is our total spend?", refresh=False)


def test_ask_cost_question_requires_question():
    out = json.loads(server.ask_cost_question("   "))
    assert out["error"] == "question is required"


@patch.object(server, "write_html_report", return_value="/tmp/report.html")
@patch.object(server, "build_markdown_report", return_value="# Report\n")
@patch.object(server, "fetch_report", return_value=SAMPLE_REPORT)
def test_get_cost_report(mock_fetch, mock_md, mock_html):
    report = server.get_cost_report(html=True)
    assert report == "# Report\n"
    mock_fetch.assert_called_once_with(refresh=False)
    mock_md.assert_called_once()
    mock_html.assert_called_once()


@patch.object(server, "write_html_report")
@patch.object(server, "build_markdown_report", return_value="# Report\n")
@patch.object(server, "fetch_report", return_value=SAMPLE_REPORT)
def test_get_cost_report_no_html(mock_fetch, mock_md, mock_html):
    report = server.get_cost_report(html=False)
    assert report == "# Report\n"
    mock_html.assert_not_called()


def test_build_markdown_report_includes_sections():
    md = build_markdown_report(SAMPLE_DASHBOARD, SAMPLE_TRENDS, SAMPLE_SUGGESTIONS)
    assert "# AWS organization cost report" in md
    assert "Biggest increases" in md
    assert "Savings opportunities" in md


def test_build_markdown_report_html_path():
    md = build_markdown_report(
        SAMPLE_DASHBOARD,
        SAMPLE_TRENDS,
        SAMPLE_SUGGESTIONS,
        html_path="/tmp/cost-report.html",
    )
    assert "/tmp/cost-report.html" in md


def test_write_html_report_creates_file():
    with tempfile.TemporaryDirectory() as tmp:
        path = write_html_report(
            SAMPLE_DASHBOARD,
            SAMPLE_TRENDS,
            SAMPLE_SUGGESTIONS,
            out_dir=tmp,
        )
        assert Path(path).exists()
        assert Path(tmp, "latest.html").exists()
        content = Path(path).read_text(encoding="utf-8")
        assert "AWS organization cost report" in content
