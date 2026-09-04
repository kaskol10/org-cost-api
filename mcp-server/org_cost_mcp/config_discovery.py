"""Discover org-cost-api config.yaml and apply chat/API settings to the environment."""

from __future__ import annotations

import os
from pathlib import Path
from typing import Any

import yaml

DEFAULT_CONFIG_NAME = "config.yaml"
DEMO_CONFIG_NAME = "config.demo.yaml"
_APPLIED = False


def _demo_enabled() -> bool:
    v = os.environ.get("ORG_COST_DEMO", "").strip().lower()
    return v in {"1", "true", "yes"}


def _add_candidate(seen: set[str], out: list[str], path: Path | str | None) -> None:
    if path is None:
        return
    p = str(Path(path).expanduser())
    if not p or p in seen:
        return
    seen.add(p)
    out.append(p)


def discover_config_path(explicit: str | None = None) -> Path | None:
    """Return the first existing config file, mirroring the Go discover order."""
    if explicit and str(explicit).strip():
        p = Path(explicit).expanduser()
        return p if p.is_file() else None

    seen: set[str] = set()
    candidates: list[str] = []
    for env in ("CONFIG_PATH", "ORG_COST_CONFIG"):
        _add_candidate(seen, candidates, os.environ.get(env))

    cwd = Path.cwd()
    for directory in [cwd, *cwd.parents]:
        _add_candidate(seen, candidates, directory / DEFAULT_CONFIG_NAME)
        if _demo_enabled():
            _add_candidate(seen, candidates, directory / DEMO_CONFIG_NAME)

    home = Path.home()
    _add_candidate(seen, candidates, home / ".org-cost" / DEFAULT_CONFIG_NAME)
    _add_candidate(seen, candidates, home / ".config" / "org-cost-api" / DEFAULT_CONFIG_NAME)

    for candidate in candidates:
        if Path(candidate).is_file():
            return Path(candidate)
    return None


def load_config_dict(path: Path | None = None) -> dict[str, Any]:
    cfg_path = path or discover_config_path()
    if cfg_path is None:
        return {}
    with cfg_path.open(encoding="utf-8") as fh:
        data = yaml.safe_load(fh)
    return data if isinstance(data, dict) else {}


def _listen_addr_to_api_url(listen_addr: str) -> str:
    addr = listen_addr.strip()
    if not addr:
        return "http://127.0.0.1:8080"
    if addr.startswith(":"):
        return f"http://127.0.0.1{addr}"
    if "://" in addr:
        return addr
    return f"http://{addr}"


def _setenv_if_empty(key: str, value: str | None) -> None:
    if value is None or str(value).strip() == "":
        return
    if not os.environ.get(key, "").strip():
        os.environ[key] = str(value).strip()


def _resolve_env_ref(value: Any) -> str:
    if not isinstance(value, str):
        return ""
    text = value.strip()
    if text.endswith("_env"):
        return os.environ.get(text, "").strip()
    if text.isidentifier() and text.isupper():
        return os.environ.get(text, "").strip()
    return text


def apply_discovered_config(explicit: str | None = None) -> Path | None:
    """
    Load config.yaml and set ORG_COST_* / LLM_* env vars when not already set.
    Safe to call multiple times; only applies once per process unless forced.
    """
    global _APPLIED
    if _APPLIED and explicit is None:
        return discover_config_path()

    cfg_path = discover_config_path(explicit)
    data = load_config_dict(cfg_path) if cfg_path else {}

    listen = str(data.get("listen_addr", "")).strip()
    if listen:
        _setenv_if_empty("ORG_COST_API_URL", _listen_addr_to_api_url(listen))

    api_token_env = str(data.get("api_token_env", "")).strip()
    if api_token_env:
        _setenv_if_empty("ORG_COST_API_TOKEN", os.environ.get(api_token_env, ""))

    chat = data.get("chat")
    if isinstance(chat, dict):
        _setenv_if_empty("LLM_PROVIDER", chat.get("provider"))
        _setenv_if_empty("LLM_BASE_URL", chat.get("base_url"))
        _setenv_if_empty("LLM_MODEL", chat.get("model"))
        _setenv_if_empty("VLLM_BASE_URL", chat.get("vllm_base_url"))

        api_key = _resolve_env_ref(chat.get("api_key_env")) or _resolve_env_ref(
            chat.get("api_key")
        )
        _setenv_if_empty("LLM_API_KEY", api_key)

        if chat.get("api_url"):
            _setenv_if_empty("ORG_COST_API_URL", str(chat["api_url"]))

    if cfg_path is not None:
        os.environ.setdefault("ORG_COST_CONFIG_PATH", str(cfg_path))

    _APPLIED = True
    return cfg_path
