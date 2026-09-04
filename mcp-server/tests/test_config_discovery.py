"""Tests for config.yaml auto-discovery."""

from __future__ import annotations

import os
from pathlib import Path

from org_cost_mcp import config_discovery


def test_discover_walks_up_to_repo_root(tmp_path, monkeypatch):
    root = tmp_path / "repo"
    sub = root / "mcp-server" / "org_cost_mcp"
    sub.mkdir(parents=True)
    cfg = root / "config.yaml"
    cfg.write_text("listen_addr: ':9090'\naccounts: []\n", encoding="utf-8")
    monkeypatch.chdir(sub)
    monkeypatch.delenv("CONFIG_PATH", raising=False)
    monkeypatch.delenv("ORG_COST_CONFIG", raising=False)
    found = config_discovery.discover_config_path()
    assert found == cfg


def test_apply_sets_api_url_from_listen_addr(tmp_path, monkeypatch):
    cfg = tmp_path / "config.yaml"
    cfg.write_text(
        "listen_addr: ':8080'\n"
        "api_token_env: ORG_COST_API_TOKEN\n"
        "chat:\n"
        "  provider: vllm\n"
        "  model: test-model\n"
        "  base_url: http://llm:4000/v1\n",
        encoding="utf-8",
    )
    monkeypatch.chdir(tmp_path)
    monkeypatch.delenv("ORG_COST_API_URL", raising=False)
    monkeypatch.delenv("LLM_PROVIDER", raising=False)
    monkeypatch.delenv("LLM_MODEL", raising=False)
    monkeypatch.delenv("LLM_BASE_URL", raising=False)
    monkeypatch.setenv("ORG_COST_API_TOKEN", "secret")
    config_discovery._APPLIED = False
    config_discovery.apply_discovered_config(str(cfg))
    assert os.environ["ORG_COST_API_URL"] == "http://127.0.0.1:8080"
    assert os.environ["LLM_PROVIDER"] == "vllm"
    assert os.environ["LLM_MODEL"] == "test-model"
    assert os.environ["ORG_COST_API_TOKEN"] == "secret"
