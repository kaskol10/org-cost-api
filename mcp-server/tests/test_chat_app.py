"""Tests for chat HTTP API."""

from __future__ import annotations

from unittest.mock import patch

from fastapi.testclient import TestClient

from org_cost_mcp.agent import AgentResult
from org_cost_mcp.chat_app import app

client = TestClient(app)


def test_chat_health():
    res = client.get("/v1/chat/health")
    assert res.status_code == 200
    body = res.json()
    assert body["status"] == "ok"
    assert "llm_provider" in body
    assert "llm_model" in body
    assert "suggestions_llm" in body


@patch("org_cost_mcp.chat_app.run_agent")
def test_chat_non_stream(mock_run):
    mock_run.return_value = AgentResult(
        content="Hello from agent.",
        tools_called=["get_org_summary"],
        ce_calls_used=0,
    )
    res = client.post(
        "/v1/chat?stream=false",
        json={"message": "What is our spend?"},
    )
    assert res.status_code == 200
    body = res.json()
    assert body["answer"] == "Hello from agent."
    assert body["session_id"]
    assert body["tools_called"] == ["get_org_summary"]


@patch("org_cost_mcp.chat_app.run_agent")
def test_chat_stream(mock_run):
    from org_cost_mcp.agent import AgentEvent

    def fake_stream(*_args, **_kwargs):
        yield AgentEvent("delta", {"text": "Streaming "})
        yield AgentEvent("delta", {"text": "answer."})
        yield AgentEvent("done", {"tools_called": [], "ce_calls_used": 0})

    mock_run.return_value = fake_stream()
    res = client.post("/v1/chat", json={"message": "Hi"})
    assert res.status_code == 200
    assert "text/event-stream" in res.headers.get("content-type", "")
    assert "event: delta" in res.text
    assert "Streaming " in res.text
    assert "answer." in res.text


@patch("org_cost_mcp.suggestions_enrich.enrich_suggestions")
@patch("org_cost_mcp.chat_app.llm_configured", return_value=True)
def test_explain_suggestions(mock_configured, mock_enrich):
    mock_enrich.return_value = {
        "summary": "1 opportunity",
        "llm_enriched": True,
        "suggestions": [{"id": "unattached-ebs", "explanation": "Because…"}],
    }
    res = client.post("/v1/suggestions/explain", json={"refresh": False})
    assert res.status_code == 200
    assert res.json()["llm_enriched"] is True


@patch("org_cost_mcp.chat_app.llm_configured", return_value=False)
def test_explain_suggestions_no_llm(mock_configured):
    res = client.post("/v1/suggestions/explain", json={"refresh": False})
    assert res.status_code == 503


@patch("org_cost_mcp.suggestions_enrich.enrich_suggestions")
@patch("org_cost_mcp.chat_app.llm_configured", return_value=True)
def test_explain_suggestions_fallback_on_llm_error(mock_configured, mock_enrich):
    mock_enrich.side_effect = RuntimeError("model unavailable")
    report = {
        "dashboard": {"totals": {"org_total": 100}},
        "trends": {},
        "suggestions": {
            "summary": "1 item",
            "suggestions": [{"id": "waste", "title": "EBS", "detail": "x", "actions": []}],
            "ce_calls_used": 0,
            "data_sources": [],
        },
    }
    res = client.post("/v1/suggestions/explain", json={"refresh": False, "report": report})
    assert res.status_code == 200
    body = res.json()
    assert body["llm_enriched"] is False
    assert "model unavailable" in body.get("enrichment_error", "")
