"""Tests for LLM provider configuration."""

from __future__ import annotations

import os
from unittest.mock import patch

from org_cost_mcp.llm_config import llm_completion_kwargs, llm_configured, resolve_llm_settings

_LLM_ENV_PREFIXES = ("LLM_", "LITELLM_", "VLLM_")


def _env_without_llm(**overrides: str) -> dict[str, str]:
    env = {
        k: v
        for k, v in os.environ.items()
        if not k.startswith(_LLM_ENV_PREFIXES)
    }
    env.update(overrides)
    return env


def test_litellm_proxy_from_legacy_env():
    with patch.dict(
        os.environ,
        _env_without_llm(
            LITELLM_BASE_URL="https://litellm.internal/v1",
            LITELLM_API_KEY="sk-proxy",
            LITELLM_MODEL="claude-sonnet-4-20250514",
        ),
        clear=True,
    ):
        settings = resolve_llm_settings()
        assert settings.provider == "litellm"
        assert settings.model == "claude-sonnet-4-20250514"
        kwargs = settings.completion_kwargs()
        assert kwargs["api_base"] == "https://litellm.internal/v1"
        assert kwargs["api_key"] == "sk-proxy"


def test_generic_llm_env_takes_precedence():
    with patch.dict(
        os.environ,
        _env_without_llm(
            LLM_PROVIDER="openai",
            LLM_BASE_URL="http://proxy:8080/v1",
            LLM_API_KEY="key-a",
            LLM_MODEL="gpt-4o",
            LITELLM_MODEL="ignored",
        ),
        clear=True,
    ):
        settings = resolve_llm_settings()
        assert settings.provider == "openai"
        assert settings.model == "gpt-4o"
        assert settings.base_url == "http://proxy:8080/v1"


def test_vllm_defaults_and_model_prefix():
    with patch.dict(
        os.environ,
        _env_without_llm(
            LLM_PROVIDER="vllm",
            LLM_MODEL="meta-llama/Meta-Llama-3-8B-Instruct",
        ),
        clear=True,
    ):
        settings = resolve_llm_settings()
        assert settings.provider == "vllm"
        assert settings.base_url == "http://localhost:8000/v1"
        assert settings.model.startswith("hosted_vllm/")
        kwargs = llm_completion_kwargs()
        assert kwargs["api_base"] == "http://localhost:8000/v1"
        assert kwargs["api_key"] == "EMPTY"


def test_vllm_custom_base_url():
    with patch.dict(
        os.environ,
        _env_without_llm(
            LLM_PROVIDER="vllm",
            VLLM_BASE_URL="http://gpu-host:8000/v1",
            LLM_MODEL="hosted_vllm/my-model",
        ),
        clear=True,
    ):
        settings = resolve_llm_settings()
        assert settings.base_url == "http://gpu-host:8000/v1"
        assert settings.model == "hosted_vllm/my-model"


def test_llm_configured_requires_endpoint_or_key():
    with patch.dict(os.environ, {}, clear=True):
        assert llm_configured() is False

    with patch.dict(
        os.environ,
        {"LLM_MODEL": "gpt-4o-mini"},
        clear=True,
    ):
        assert llm_configured() is False

    with patch.dict(
        os.environ,
        {"LLM_BASE_URL": "http://proxy:8080/v1", "LLM_MODEL": "gpt-4o"},
        clear=True,
    ):
        assert llm_configured() is True

    with patch.dict(
        os.environ,
        {"LLM_PROVIDER": "vllm", "VLLM_BASE_URL": "http://localhost:8000/v1"},
        clear=True,
    ):
        assert llm_configured() is True
