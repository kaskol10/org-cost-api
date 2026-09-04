"""LLM provider settings for org-cost-chat (LiteLLM proxy, vLLM, or any OpenAI-compatible API)."""

from __future__ import annotations

import os
from dataclasses import dataclass
from typing import Any


def _first_env(*names: str) -> str:
    for name in names:
        raw = os.environ.get(name, "").strip()
        if raw:
            return raw
    return ""


@dataclass(frozen=True)
class LLMSettings:
    provider: str
    model: str
    base_url: str
    api_key: str

    def completion_kwargs(self) -> dict[str, Any]:
        """Keyword args for litellm.completion()."""
        kwargs: dict[str, Any] = {"model": self.model}
        if self.base_url:
            kwargs["api_base"] = self.base_url.rstrip("/")
        if self.api_key and self.api_key.upper() not in {"NONE", "EMPTY", "NOT-NEEDED"}:
            kwargs["api_key"] = self.api_key
        elif self.provider == "vllm":
            # vLLM OpenAI server often requires a non-empty key string.
            kwargs["api_key"] = "EMPTY"
        return kwargs

    def as_health_dict(self) -> dict[str, str]:
        return {
            "llm_provider": self.provider,
            "llm_model": self.model,
            "llm_base_url": self.base_url or "(default)",
        }


def _default_model(provider: str) -> str:
    if provider == "vllm":
        return "meta-llama/Meta-Llama-3-8B-Instruct"
    return "gpt-4o-mini"


def _normalize_vllm_model(model: str) -> str:
    """LiteLLM expects hosted_vllm/ prefix for direct vLLM OpenAI servers."""
    if model.startswith(("openai/", "hosted_vllm/", "vllm/")):
        return model
    return f"hosted_vllm/{model}"


def resolve_llm_settings() -> LLMSettings:
    """
    Resolve LLM config from environment.

    Preferred generic vars: LLM_PROVIDER, LLM_BASE_URL, LLM_API_KEY, LLM_MODEL
    Legacy aliases: LITELLM_BASE_URL, LITELLM_API_KEY, LITELLM_MODEL

    LLM_PROVIDER:
      - litellm (default) — org LiteLLM or any OpenAI-compatible proxy
      - vllm — local/cloud vLLM OpenAI server (default base http://localhost:8000/v1)
      - openai — same as litellm; explicit OpenAI-compatible endpoint
    """
    provider = _first_env("LLM_PROVIDER").lower() or "litellm"
    if provider not in {"litellm", "vllm", "openai"}:
        provider = "litellm"

    model = _first_env("LLM_MODEL", "LITELLM_MODEL") or _default_model(provider)
    base_url = _first_env("LLM_BASE_URL", "LITELLM_BASE_URL")
    api_key = _first_env("LLM_API_KEY", "LITELLM_API_KEY")

    if provider == "vllm":
        if not base_url:
            base_url = _first_env("VLLM_BASE_URL") or "http://localhost:8000/v1"
        model = _normalize_vllm_model(model)
        if not api_key:
            api_key = _first_env("VLLM_API_KEY") or "EMPTY"

    return LLMSettings(
        provider=provider,
        model=model,
        base_url=base_url,
        api_key=api_key,
    )


def llm_completion_kwargs() -> dict[str, Any]:
    return resolve_llm_settings().completion_kwargs()


def llm_configured() -> bool:
    """
    True when the chat agent can reach an LLM endpoint.

    A default model name alone is not enough — require a proxy base URL, API key,
    or (for vLLM) an explicit server URL.
    """
    settings = resolve_llm_settings()
    if settings.provider == "vllm":
        return bool(settings.base_url)
    return bool(settings.base_url or settings.api_key)
