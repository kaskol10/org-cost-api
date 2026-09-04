"""Tests for chat agent and tools."""

from __future__ import annotations

import json
from unittest.mock import MagicMock, patch

from org_cost_mcp import tools
from org_cost_mcp.agent import run_agent
from org_cost_mcp.sessions import SessionStore


@patch("org_cost_mcp.agent.execute_tool")
@patch("org_cost_mcp.agent.litellm.completion")
def test_agent_answers_without_tools(mock_completion, mock_execute):
    msg = MagicMock()
    msg.model_dump.return_value = {
        "role": "assistant",
        "content": "Org spend is **$5000**.",
    }
    mock_completion.return_value = MagicMock(choices=[MagicMock(message=msg)])

    result = run_agent(
        [{"role": "user", "content": "What is our spend?"}],
        stream=False,
    )

    assert result.content == "Org spend is **$5000**."
    assert result.tools_called == []
    mock_execute.assert_not_called()


@patch("org_cost_mcp.agent.execute_tool")
@patch("org_cost_mcp.agent.litellm.completion")
def test_agent_calls_tool_then_responds(mock_completion, mock_execute):
    tool_msg = MagicMock()
    tool_msg.model_dump.return_value = {
        "role": "assistant",
        "content": None,
        "tool_calls": [
            {
                "id": "call_1",
                "type": "function",
                "function": {"name": "get_org_summary", "arguments": "{}"},
            }
        ],
    }
    final_msg = MagicMock()
    final_msg.model_dump.return_value = {
        "role": "assistant",
        "content": "Total org spend is $5000.",
    }
    mock_completion.side_effect = [
        MagicMock(choices=[MagicMock(message=tool_msg)]),
        MagicMock(choices=[MagicMock(message=final_msg)]),
    ]
    mock_execute.return_value = json.dumps(
        {"totals": {"org_total_usd": 5000}, "ce_calls_used": 0}
    )

    result = run_agent(
        [{"role": "user", "content": "What is our total spend?"}],
        stream=False,
    )

    assert result.content == "Total org spend is $5000."
    assert result.tools_called == ["get_org_summary"]
    mock_execute.assert_called_once_with("get_org_summary", {})


def test_execute_tool_unknown():
    out = tools.execute_tool("not_a_tool", {})
    data = json.loads(out)
    assert "error" in data


def test_session_store_create_and_append():
    store = SessionStore()
    sess = store.get_or_create(None)
    assert sess.session_id
    store.append_message(sess.session_id, {"role": "user", "content": "hi"})
    again = store.get_or_create(sess.session_id)
    assert len(again.messages) == 1
