"""In-memory chat session store (v1)."""

from __future__ import annotations

import os
import time
import uuid
from dataclasses import dataclass, field
from typing import Any


def _ttl_seconds() -> int:
    raw = os.environ.get("CHAT_SESSION_TTL_SECONDS", "").strip()
    if raw:
        try:
            return max(60, int(raw))
        except ValueError:
            pass
    return 3600


@dataclass
class ChatSession:
    session_id: str
    messages: list[dict[str, Any]] = field(default_factory=list)
    created_at: float = field(default_factory=time.time)
    updated_at: float = field(default_factory=time.time)

    def touch(self) -> None:
        self.updated_at = time.time()


class SessionStore:
    def __init__(self) -> None:
        self._sessions: dict[str, ChatSession] = {}
        self._ttl = _ttl_seconds()

    def _purge_expired(self) -> None:
        now = time.time()
        expired = [
            sid
            for sid, sess in self._sessions.items()
            if now - sess.updated_at > self._ttl
        ]
        for sid in expired:
            del self._sessions[sid]

    def get_or_create(self, session_id: str | None) -> ChatSession:
        self._purge_expired()
        if session_id and session_id in self._sessions:
            sess = self._sessions[session_id]
            sess.touch()
            return sess
        new_id = session_id or str(uuid.uuid4())
        sess = ChatSession(session_id=new_id)
        self._sessions[new_id] = sess
        return sess

    def append_message(self, session_id: str, message: dict[str, Any]) -> None:
        sess = self._sessions.get(session_id)
        if sess is None:
            return
        sess.messages.append(message)
        sess.touch()
