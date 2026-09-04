"""HTTP chat API for conversational Ask (LiteLLM + org-cost tools)."""

from __future__ import annotations

import json
import os
from collections.abc import AsyncIterator
from typing import Any

from fastapi import FastAPI, HTTPException, Query
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse, StreamingResponse
from pydantic import BaseModel, Field

from org_cost_mcp.agent import AgentEvent, run_agent
from org_cost_mcp.config_discovery import apply_discovered_config
from org_cost_mcp.llm_config import llm_configured, resolve_llm_settings
from org_cost_mcp.sessions import SessionStore

apply_discovered_config()

app = FastAPI(title="org-cost-chat", version="0.1.0")
_store = SessionStore()


def _cors_origins() -> list[str]:
    raw = os.environ.get("CHAT_CORS_ORIGINS", "").strip()
    if raw:
        return [o.strip() for o in raw.split(",") if o.strip()]
    return ["http://localhost:5173", "http://localhost:8080", "http://127.0.0.1:8080"]


app.add_middleware(
    CORSMiddleware,
    allow_origins=_cors_origins(),
    allow_credentials=True,
    allow_methods=["GET", "POST", "OPTIONS"],
    allow_headers=["*"],
)


class ChatRequest(BaseModel):
    session_id: str | None = None
    message: str = Field(..., min_length=1, max_length=8000)
    refresh: bool = False


class ChatResponse(BaseModel):
    session_id: str
    answer: str
    tools_called: list[str] = Field(default_factory=list)
    ce_calls_used: int = 0


@app.get("/v1/chat/health")
def chat_health() -> dict[str, str]:
    apply_discovered_config()
    settings = resolve_llm_settings()
    body = {"status": "ok", **settings.as_health_dict()}
    cfg = os.environ.get("ORG_COST_CONFIG_PATH", "").strip()
    if cfg:
        body["config_path"] = cfg
    configured = llm_configured()
    body["suggestions_llm"] = "true" if configured else "false"
    body["llm_configured"] = "true" if configured else "false"
    if not configured:
        body["hint"] = (
            "Set chat: in config.yaml or LLM_BASE_URL + LLM_API_KEY. "
            "Env vars override YAML; restart org-cost-chat after changes."
        )
    return body


class SuggestionsExplainRequest(BaseModel):
    refresh: bool = False
    report: dict[str, Any] | None = None


def _base_suggestions_payload(report: dict[str, Any] | None) -> dict[str, Any] | None:
    if not isinstance(report, dict):
        return None
    suggestions = report.get("suggestions")
    if isinstance(suggestions, dict) and suggestions.get("suggestions"):
        return dict(suggestions)
    return None


@app.post("/v1/suggestions/explain", response_model=None)
async def explain_suggestions(body: SuggestionsExplainRequest) -> JSONResponse:
    import asyncio

    from org_cost_mcp.suggestions_enrich import enrich_suggestions

    if not llm_configured():
        raise HTTPException(
            status_code=503,
            detail="LLM not configured — set LLM_BASE_URL or chat: in config.yaml",
        )
    try:
        # Run sync LiteLLM off the event loop so /v1/chat/health stays responsive.
        data = await asyncio.to_thread(
            enrich_suggestions,
            refresh=body.refresh,
            report=body.report,
        )
    except Exception as exc:  # noqa: BLE001
        fallback = _base_suggestions_payload(body.report)
        if fallback is not None:
            fallback["llm_enriched"] = False
            fallback["enrichment_error"] = str(exc)[:500]
            return JSONResponse(fallback)
        raise HTTPException(status_code=500, detail=str(exc)) from exc
    return JSONResponse(data)


def _sse_event(event: str, data: dict[str, Any]) -> str:
    return f"event: {event}\ndata: {json.dumps(data)}\n\n"


async def _stream_chat(
    session_id: str,
    user_message: str,
    refresh: bool,
) -> AsyncIterator[str]:
    import asyncio
    from queue import Queue

    sess = _store.get_or_create(session_id)
    _store.append_message(sess.session_id, {"role": "user", "content": user_message})
    yield _sse_event("session", {"session_id": sess.session_id})

    q: Queue[AgentEvent | BaseException | None] = Queue()

    def _produce() -> None:
        try:
            events = run_agent(
                sess.messages,
                default_refresh=refresh,
                stream=True,
            )
            assert not isinstance(events, type(None))  # stream=True returns iterator
            for ev in events:
                q.put(ev)
            q.put(None)
        except BaseException as exc:  # noqa: BLE001
            q.put(exc)

    producer = asyncio.create_task(asyncio.to_thread(_produce))

    try:
        answer_parts: list[str] = []
        tools_called: list[str] = []
        ce_calls_used = 0

        while True:
            item = await asyncio.to_thread(q.get)
            if item is None:
                break
            if isinstance(item, BaseException):
                raise item
            if not isinstance(item, AgentEvent):
                continue
            ev = item
            if ev.kind == "tool_start":
                yield _sse_event("tool_start", ev.data)
            elif ev.kind == "tool_end":
                name = ev.data.get("name", "")
                if name:
                    tools_called.append(name)
                yield _sse_event("tool_end", ev.data)
            elif ev.kind == "delta":
                text = ev.data.get("text", "")
                if text:
                    answer_parts.append(text)
                    yield _sse_event("delta", {"text": text})
            elif ev.kind == "done":
                tools_called = ev.data.get("tools_called", tools_called)
                ce_calls_used = int(ev.data.get("ce_calls_used", 0))
                yield _sse_event(
                    "done",
                    {
                        "tools_called": tools_called,
                        "ce_calls_used": ce_calls_used,
                    },
                )
            elif ev.kind == "error":
                yield _sse_event("error", ev.data)

        answer = "".join(answer_parts)
        if answer:
            _store.append_message(
                sess.session_id,
                {"role": "assistant", "content": answer},
            )
    except Exception as exc:  # noqa: BLE001
        yield _sse_event("error", {"message": str(exc)})
    finally:
        await producer


@app.post("/v1/chat", response_model=None)
async def chat(
    body: ChatRequest,
    stream: bool = Query(default=True),
) -> StreamingResponse | JSONResponse:
    message = body.message.strip()
    if not message:
        raise HTTPException(status_code=400, detail="message is required")

    if stream:
        sess = _store.get_or_create(body.session_id)
        return StreamingResponse(
            _stream_chat(sess.session_id, message, body.refresh),
            media_type="text/event-stream",
            headers={
                "Cache-Control": "no-cache",
                "Connection": "keep-alive",
                "X-Accel-Buffering": "no",
            },
        )

    import asyncio

    sess = _store.get_or_create(body.session_id)
    _store.append_message(sess.session_id, {"role": "user", "content": message})
    result = await asyncio.to_thread(
        run_agent,
        sess.messages,
        default_refresh=body.refresh,
        stream=False,
    )
    if not hasattr(result, "content"):
        raise HTTPException(status_code=500, detail="agent error")
    _store.append_message(
        sess.session_id,
        {"role": "assistant", "content": result.content},
    )
    return JSONResponse(
        ChatResponse(
            session_id=sess.session_id,
            answer=result.content,
            tools_called=result.tools_called,
            ce_calls_used=result.ce_calls_used,
        ).model_dump()
    )


def main() -> None:
    import uvicorn

    host = os.environ.get("CHAT_HOST", "0.0.0.0").strip()
    port = int(os.environ.get("CHAT_PORT", "8090"))
    uvicorn.run(
        "org_cost_mcp.chat_app:app",
        host=host,
        port=port,
        reload=os.environ.get("CHAT_RELOAD", "").strip() == "1",
    )


if __name__ == "__main__":
    main()
