"""HTTP client for the org-cost-api backend."""

from __future__ import annotations

import os
import warnings
from collections.abc import Callable
from typing import Any, TypeVar

import httpx

DEFAULT_BASE = "http://localhost:8080"
DEFAULT_READ_TIMEOUT = 120.0

_client: httpx.Client | None = None

# Transient network errors — often stale keep-alive sockets after idle periods.
_TRANSIENT_HTTP_ERRORS = (
    httpx.ConnectError,
    httpx.ReadError,
    httpx.RemoteProtocolError,
    httpx.WriteError,
    httpx.PoolTimeout,
)

T = TypeVar("T")


def base_url() -> str:
    url = os.environ.get("ORG_COST_API_URL", "").strip()
    if not url:
        legacy = os.environ.get("EC2_OTHER_API_URL", "").strip()
        if legacy:
            warnings.warn(
                "EC2_OTHER_API_URL is deprecated; use ORG_COST_API_URL",
                DeprecationWarning,
                stacklevel=2,
            )
            url = legacy
    if not url:
        url = DEFAULT_BASE
    return url.rstrip("/")


def _api_token() -> str:
    return os.environ.get("ORG_COST_API_TOKEN", "").strip()


def _read_timeout() -> float:
    raw = os.environ.get("ORG_COST_API_TIMEOUT_SECONDS", "").strip()
    if raw:
        try:
            return float(raw)
        except ValueError:
            pass
    return DEFAULT_READ_TIMEOUT


def _reset_http_client() -> None:
    global _client
    if _client is not None:
        try:
            _client.close()
        except Exception:  # noqa: BLE001
            pass
    _client = None


def _http_client() -> httpx.Client:
    global _client
    if _client is None:
        read = _read_timeout()
        _client = httpx.Client(
            timeout=httpx.Timeout(connect=10.0, read=read, write=30.0, pool=10.0),
            limits=httpx.Limits(max_keepalive_connections=10, keepalive_expiry=30.0),
        )
    return _client


def _with_http_retry(fn: Callable[[], T]) -> T:
    """Retry once after resetting the client on transient connection errors."""
    last_exc: Exception | None = None
    for attempt in range(2):
        try:
            return fn()
        except _TRANSIENT_HTTP_ERRORS as exc:
            last_exc = exc
            _reset_http_client()
            if attempt == 0:
                continue
            raise
    if last_exc is not None:
        raise last_exc
    raise RuntimeError("HTTP retry failed without exception")


def _headers() -> dict[str, str]:
    token = _api_token()
    if token:
        return {"Authorization": f"Bearer {token}"}
    return {}


def _get(path: str, *, refresh: bool = False, params: dict[str, str] | None = None) -> Any:
    q = dict(params or {})
    if refresh:
        q["refresh"] = "1"
    url = f"{base_url()}{path}"

    def do() -> Any:
        resp = _http_client().get(url, params=q, headers=_headers())
        resp.raise_for_status()
        data = resp.json()
        if isinstance(data, dict) and "error" in data:
            raise RuntimeError(data["error"])
        return data

    return _with_http_retry(do)


def fetch_account_costs(account: str, *, refresh: bool = False) -> dict[str, Any]:
    return _get("/api/account-costs", refresh=refresh, params={"account": account})


def fetch_dashboard(*, refresh: bool = False) -> dict[str, Any]:
    return _get("/api/dashboard", refresh=refresh)


def fetch_accounts() -> list[dict[str, str]]:
    return _get("/api/accounts")


def fetch_service_detail(
    account_id: str,
    service: str,
    *,
    start: str = "",
    end: str = "",
) -> dict[str, Any]:
    params: dict[str, str] = {"account_id": account_id, "service": service}
    if start:
        params["start"] = start
    if end:
        params["end"] = end
    return _get("/api/service-detail", params=params)


def fetch_trends(*, refresh: bool = False) -> dict[str, Any]:
    return _get("/api/trends", refresh=refresh)


def fetch_suggestions(*, refresh: bool = False) -> dict[str, Any]:
    return _get("/api/suggestions", refresh=refresh)


def fetch_report(*, refresh: bool = False) -> dict[str, Any]:
    return _get("/api/report", refresh=refresh)


def fetch_service_tag_delta(
    *,
    service: str,
    account_id: str = "",
    start: str = "",
    end: str = "",
    tag_key: str = "Name",
    top_accounts: int = 5,
) -> dict[str, Any]:
    params: dict[str, str] = {
        "service": service,
        "tag_key": tag_key,
        "top_accounts": str(top_accounts),
    }
    if account_id:
        params["account_id"] = account_id
    if start:
        params["start"] = start
    if end:
        params["end"] = end
    return _get("/api/service-tag-delta", params=params)


def fetch_service_tag_totals(
    *,
    service: str,
    account_id: str = "",
    start: str = "",
    end: str = "",
    tag_key: str = "Name",
    top_buckets: int = 10,
) -> dict[str, Any]:
    params: dict[str, str] = {
        "service": service,
        "tag_key": tag_key,
        "top_buckets": str(top_buckets),
    }
    if account_id:
        params["account_id"] = account_id
    if start:
        params["start"] = start
    if end:
        params["end"] = end
    return _get("/api/service-tag-totals", params=params)


def fetch_ask(question: str, *, refresh: bool = False) -> dict[str, Any]:
    url = f"{base_url()}/api/ask"

    def do() -> dict[str, Any]:
        resp = _http_client().post(
            url,
            json={"question": question, "refresh": refresh},
            headers={**_headers(), "Content-Type": "application/json"},
        )
        resp.raise_for_status()
        data = resp.json()
        if isinstance(data, dict) and "error" in data:
            raise RuntimeError(data["error"])
        return data

    return _with_http_retry(do)


def health_status() -> dict[str, Any]:
    """Probe health, readiness, demo flag, and optional auth."""
    status: dict[str, Any] = {
        "api_base": base_url(),
        "api_reachable": False,
        "healthy": False,
        "ready": False,
        "demo": False,
        "auth_ok": True,
    }
    try:
        data = _get("/api/health")
        status["api_reachable"] = data.get("status") == "ok"
    except Exception as exc:  # noqa: BLE001
        status["error"] = str(exc)
        return status
    try:
        ready = _get("/api/ready")
        status["ready"] = bool(ready.get("ready"))
        if not status["ready"]:
            status["ready_detail"] = {
                "billing_check": ready.get("billing_check"),
                "billing_error": ready.get("billing_error"),
                "accounts_check": ready.get("accounts_check"),
                "accounts_error": ready.get("accounts_error"),
            }
    except Exception as exc:  # noqa: BLE001
        status["ready"] = False
        status["ready_error"] = str(exc)
    try:
        meta = _get("/api/meta")
        status["demo"] = bool(meta.get("demo"))
    except Exception:
        pass
    if _api_token():
        try:
            _get("/api/accounts")
        except Exception as exc:  # noqa: BLE001
            status["auth_ok"] = False
            status["auth_error"] = str(exc)
    status["healthy"] = (
        status["api_reachable"] and status["ready"] and status["auth_ok"]
    )
    if not status["healthy"] and "hint" not in status:
        if not status["api_reachable"]:
            status["hint"] = f"Cannot reach Go API at {status['api_base']} — start the backend or set ORG_COST_API_URL."
        elif not status["ready"]:
            status["hint"] = (
                "API is up but not ready — check AWS SSO (granted sso login), billing_profile, "
                "or curl /api/ready for billing_error / accounts_error."
            )
        elif not status["auth_ok"]:
            status["hint"] = "Set ORG_COST_API_TOKEN in the MCP env to match the backend api_token."
    return status


def health_check() -> bool:
    return health_status()["healthy"]
