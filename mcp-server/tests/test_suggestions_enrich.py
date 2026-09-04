"""Tests for LLM suggestion enrichment."""

from __future__ import annotations

import json
from unittest.mock import MagicMock, patch

from org_cost_mcp.suggestions_enrich import (
    build_report_context,
    enrich_suggestions,
    merge_enrichment,
)


SAMPLE_REPORT = {
    "dashboard": {
        "start": "2026-08-01",
        "end": "2026-08-31",
        "totals": {"org_total": 5000, "volume_available_count": 2},
        "top_services": [{"service": "Amazon S3", "amount": 2000}],
        "accounts": [
            {
                "account_name": "production",
                "account_id": "123",
                "costs": {"all_total": 3000, "by_service": []},
                "volumes": {"available_count": 2, "available_gib": 50},
            }
        ],
    },
    "trends": {
        "org_total": {"current_usd": 5000, "prior_usd": 4500, "change_percent": 11},
        "top_increases": [],
    },
    "suggestions": {
        "summary": "1 optimization opportunities ranked by impact.",
        "suggestions": [
            {
                "id": "unattached-ebs",
                "priority": 1,
                "category": "waste",
                "title": "Delete or attach 2 unattached EBS disks",
                "detail": "Available volumes still bill.",
                "estimated_monthly_usd": 4,
                "actions": ["Check console"],
            }
        ],
        "ce_calls_used": 0,
        "data_sources": ["dashboard_cache"],
    },
}


def test_build_report_context_trims_accounts():
    ctx = build_report_context(SAMPLE_REPORT)
    assert ctx["dashboard"]["totals"]["org_total"] == 5000
    assert len(ctx["dashboard"]["accounts"]) == 1


def test_merge_enrichment_by_id():
    base = SAMPLE_REPORT["suggestions"]
    enrichment = {
        "narrative_summary": "Org spend is rising slightly.",
        "suggestions": [
            {
                "id": "unattached-ebs",
                "explanation": "Two disks in **production** cost ~$4/mo.",
                "confidence": "high",
            }
        ],
        "additional_insights": ["Review S3 lifecycle policies."],
    }
    out = merge_enrichment(base, enrichment)
    assert out["llm_enriched"] is True
    assert out["narrative_summary"] == "Org spend is rising slightly."
    assert out["suggestions"][0]["explanation"].startswith("Two disks")
    assert out["additional_insights"] == ["Review S3 lifecycle policies."]


@patch("org_cost_mcp.suggestions_enrich.litellm.completion")
@patch("org_cost_mcp.suggestions_enrich.fetch_report")
@patch("org_cost_mcp.llm_config.llm_configured", return_value=True)
def test_enrich_suggestions_calls_llm(mock_configured, mock_report, mock_completion):
    mock_report.return_value = SAMPLE_REPORT
    msg = MagicMock()
    msg.content = json.dumps(
        {
            "narrative_summary": "Focus on EBS waste first.",
            "suggestions": [
                {
                    "id": "unattached-ebs",
                    "explanation": "Production has 50 GiB unattached.",
                    "confidence": "high",
                }
            ],
            "additional_insights": [],
        }
    )
    mock_completion.return_value = MagicMock(choices=[MagicMock(message=msg)])

    out = enrich_suggestions(refresh=False)
    assert out["llm_enriched"] is True
    assert "Production" in out["suggestions"][0]["explanation"]
    mock_completion.assert_called_once()
    mock_report.assert_called_once()


@patch("org_cost_mcp.suggestions_enrich.litellm.completion")
@patch("org_cost_mcp.suggestions_enrich.fetch_report")
@patch("org_cost_mcp.llm_config.llm_configured", return_value=True)
def test_enrich_suggestions_uses_passed_report(mock_configured, mock_report, mock_completion):
    mock_report.return_value = SAMPLE_REPORT
    msg = MagicMock()
    msg.content = json.dumps(
        {
            "narrative_summary": "Focus on EBS waste first.",
            "suggestions": [
                {
                    "id": "unattached-ebs",
                    "explanation": "Production has 50 GiB unattached.",
                    "confidence": "high",
                }
            ],
            "additional_insights": [],
        }
    )
    mock_completion.return_value = MagicMock(choices=[MagicMock(message=msg)])

    out = enrich_suggestions(refresh=False, report=SAMPLE_REPORT)
    assert out["llm_enriched"] is True
    mock_report.assert_not_called()
