"""LiteLLM tool-calling agent loop for conversational Ask."""

from __future__ import annotations

import json
import os
from collections.abc import Iterator
from dataclasses import dataclass, field
from typing import Any

import litellm

from org_cost_mcp.llm_config import llm_completion_kwargs
from org_cost_mcp.tools import TOOL_SCHEMAS, ce_calls_in_result, execute_tool

SYSTEM_PROMPT = """You are a read-only AWS organization cost assistant.

Rules:
- Always call tools before stating dollar amounts or trends.
- Default refresh=false; only pass refresh=true when the user explicitly asks for live AWS data.
- Cite the reporting period and data source when answering.
- Be concise; use markdown lists for breakdowns.
- For follow-up questions, use conversation context but re-fetch data when amounts may have changed.
"""


def _env_int(name: str, default: int) -> int:
    raw = os.environ.get(name, "").strip()
    if raw:
        try:
            return int(raw)
        except ValueError:
            pass
    return default


@dataclass
class AgentResult:
    content: str = ""
    tools_called: list[str] = field(default_factory=list)
    ce_calls_used: int = 0


@dataclass
class AgentEvent:
    kind: str
    data: dict[str, Any] = field(default_factory=dict)


def _message_content(msg: dict[str, Any]) -> str:
    content = msg.get("content")
    if content is None:
        return ""
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        parts: list[str] = []
        for block in content:
            if isinstance(block, dict) and block.get("type") == "text":
                parts.append(str(block.get("text", "")))
        return "".join(parts)
    return str(content)


def _tool_calls(msg: dict[str, Any]) -> list[dict[str, Any]]:
    raw = msg.get("tool_calls")
    if not raw:
        return []
    return list(raw)


def run_agent(
    messages: list[dict[str, Any]],
    *,
    default_refresh: bool = False,
    stream: bool = False,
) -> AgentResult | Iterator[AgentEvent]:
    if stream:
        return _run_agent_stream(messages, default_refresh=default_refresh)
    return _run_agent_sync(messages, default_refresh=default_refresh)


def _apply_default_refresh(args: dict[str, Any], default_refresh: bool) -> dict[str, Any]:
    out = dict(args)
    if "refresh" not in out and default_refresh:
        out["refresh"] = True
    return out


def _run_agent_sync(
    messages: list[dict[str, Any]],
    *,
    default_refresh: bool,
) -> AgentResult:
    working = [{"role": "system", "content": SYSTEM_PROMPT}, *messages]
    max_rounds = _env_int("CHAT_MAX_TOOL_ROUNDS", 8)
    tools_called: list[str] = []
    ce_total = 0
    llm_kwargs = llm_completion_kwargs()

    for _ in range(max_rounds):
        response = litellm.completion(
            messages=working,
            tools=TOOL_SCHEMAS,
            tool_choice="auto",
            stream=False,
            **llm_kwargs,
        )
        choice = response.choices[0].message
        assistant_msg = choice.model_dump(exclude_none=True)
        working.append(assistant_msg)

        calls = _tool_calls(assistant_msg)
        if not calls:
            return AgentResult(
                content=_message_content(assistant_msg),
                tools_called=tools_called,
                ce_calls_used=ce_total,
            )

        for call in calls:
            fn = call.get("function") or {}
            name = fn.get("name", "")
            raw_args = fn.get("arguments") or "{}"
            try:
                args = json.loads(raw_args) if isinstance(raw_args, str) else raw_args
            except json.JSONDecodeError:
                args = {}
            if not isinstance(args, dict):
                args = {}
            args = _apply_default_refresh(args, default_refresh)
            tools_called.append(name)
            result = execute_tool(name, args)
            ce_total += ce_calls_in_result(result)
            working.append(
                {
                    "role": "tool",
                    "tool_call_id": call.get("id"),
                    "name": name,
                    "content": result,
                }
            )

    return AgentResult(
        content="I reached the tool-call limit. Please try a narrower question.",
        tools_called=tools_called,
        ce_calls_used=ce_total,
    )


def _run_agent_stream(
    messages: list[dict[str, Any]],
    *,
    default_refresh: bool,
) -> Iterator[AgentEvent]:
    result = _run_agent_sync(messages, default_refresh=default_refresh)
    if result.tools_called:
        for name in result.tools_called:
            yield AgentEvent("tool_end", {"name": name})
    # v1: emit full answer as one delta (LiteLLM streaming + tools is complex)
    if result.content:
        yield AgentEvent("delta", {"text": result.content})
    yield AgentEvent(
        "done",
        {
            "tools_called": result.tools_called,
            "ce_calls_used": result.ce_calls_used,
        },
    )
