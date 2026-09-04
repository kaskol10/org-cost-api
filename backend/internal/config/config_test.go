package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveAPIToken(t *testing.T) {
	t.Setenv("ORG_COST_API_TOKEN", "from-env")
	cfg := &Config{APITokenEnv: "ORG_COST_API_TOKEN"}
	if got := cfg.ResolveAPIToken(); got != "from-env" {
		t.Fatalf("got %q, want from-env", got)
	}

	cfg = &Config{APIToken: "inline"}
	if got := cfg.ResolveAPIToken(); got != "inline" {
		t.Fatalf("got %q, want inline", got)
	}

	cfg = &Config{}
	if got := cfg.ResolveAPIToken(); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestValidateAPITokenFailClosed(t *testing.T) {
	t.Setenv("EMPTY_API_TOKEN", "")
	_ = os.Unsetenv("MISSING_API_TOKEN")

	err := validateAPIToken(&Config{APITokenEnv: "MISSING_API_TOKEN"})
	if err == nil {
		t.Fatal("expected error when api_token_env is set but env is unset")
	}
	if !strings.Contains(err.Error(), "MISSING_API_TOKEN") {
		t.Fatalf("error should mention env name: %v", err)
	}

	err = validateAPIToken(&Config{APITokenEnv: "EMPTY_API_TOKEN"})
	if err == nil {
		t.Fatal("expected error when api_token_env resolves to empty")
	}

	// Inline token satisfies auth even if env is empty.
	if err := validateAPIToken(&Config{APIToken: "inline", APITokenEnv: "MISSING_API_TOKEN"}); err != nil {
		t.Fatalf("inline token should pass: %v", err)
	}

	t.Setenv("SET_API_TOKEN", "secret")
	if err := validateAPIToken(&Config{APITokenEnv: "SET_API_TOKEN"}); err != nil {
		t.Fatalf("populated env should pass: %v", err)
	}

	if err := validateAPIToken(&Config{}); err != nil {
		t.Fatalf("no auth config should pass: %v", err)
	}
}

func TestLoadFailsWhenAPITokenEnvEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `
accounts:
  - name: test
    id: "123456789012"
    profile: test-profile
api_token_env: ORG_COST_API_TOKEN_UNSET
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Unsetenv("ORG_COST_API_TOKEN_UNSET")
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected Load to fail when api_token_env is empty")
	}
	if !strings.Contains(err.Error(), "api_token_env") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadHistoryDirEnvOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `
accounts:
  - name: test
    id: "123456789012"
    profile: test-profile
history_dir: /from-yaml
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HISTORY_DIR", "/data/history")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HistoryDir != "/data/history" {
		t.Fatalf("HistoryDir=%q, want /data/history", cfg.HistoryDir)
	}
}

func TestLoadDemoMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.yaml")
	content := `
demo: true
accounts:
  - name: production
    id: "111111111111"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Demo {
		t.Fatal("expected demo=true")
	}
	if len(cfg.Accounts) != 1 {
		t.Fatalf("accounts=%d", len(cfg.Accounts))
	}

	t.Setenv("ORG_COST_DEMO", "1")
	minimal := `
listen_addr: ":9090"
accounts: []
`
	path2 := filepath.Join(dir, "minimal.yaml")
	if err := os.WriteFile(path2, []byte(minimal), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg2, err := Load(path2)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg2.Demo {
		t.Fatal("ORG_COST_DEMO should enable demo")
	}
	if len(cfg2.Accounts) < 5 {
		t.Fatalf("expected default demo accounts, got %d", len(cfg2.Accounts))
	}
}
