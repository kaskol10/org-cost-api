package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverPathExplicit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("demo: true\naccounts: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := DiscoverPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != path {
		t.Fatalf("got %q want %q", got, path)
	}
}

func TestDiscoverPathWalkUpFromSubdir(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "backend", "cmd", "server")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("demo: true\naccounts: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(sub); err != nil {
		t.Fatal(err)
	}
	_ = os.Unsetenv("CONFIG_PATH")
	_ = os.Unsetenv("ORG_COST_CONFIG")

	got, err := DiscoverPath("")
	if err != nil {
		t.Fatal(err)
	}
	gotClean, _ := filepath.EvalSymlinks(got)
	wantClean, _ := filepath.EvalSymlinks(cfgPath)
	if gotClean != wantClean {
		t.Fatalf("got %q want %q", got, cfgPath)
	}
}

func TestDiscoverPathConfigPathEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom.yaml")
	if err := os.WriteFile(path, []byte("demo: true\naccounts: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_PATH", path)
	got, err := DiscoverPath("")
	if err != nil {
		t.Fatal(err)
	}
	if got != path {
		t.Fatalf("got %q want %q", got, path)
	}
}

func TestDiscoverPathMissing(t *testing.T) {
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	_ = os.Unsetenv("CONFIG_PATH")
	_ = os.Unsetenv("ORG_COST_CONFIG")
	_ = os.Unsetenv("ORG_COST_DEMO")
	_, err = DiscoverPath("")
	if err == nil {
		t.Fatal("expected error when no config exists")
	}
}
