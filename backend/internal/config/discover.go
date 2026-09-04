package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultConfigName = "config.yaml"
	demoConfigName  = "config.demo.yaml"
)

// DiscoverPath returns the first existing config file from explicit path, env, or search paths.
// explicit is the -config flag value; when empty, auto-discovery runs.
func DiscoverPath(explicit string) (string, error) {
	if p := strings.TrimSpace(explicit); p != "" {
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("config file %q: %w", p, err)
		}
		return p, nil
	}

	for _, candidate := range discoverCandidates() {
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	hint := "copy config.example.yaml to config.yaml in the repo root, or set CONFIG_PATH"
	if DemoEnabled() {
		hint = "copy config.demo.yaml or set ORG_COST_DEMO=1 with a config file, or set CONFIG_PATH"
	}
	return "", fmt.Errorf("no config file found (%s)", hint)
}

func discoverCandidates() []string {
	var out []string
	seen := map[string]bool{}
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		out = append(out, p)
	}

	for _, env := range []string{"CONFIG_PATH", "ORG_COST_CONFIG"} {
		add(os.Getenv(env))
	}

	cwd, err := os.Getwd()
	if err == nil {
		dir := cwd
		for i := 0; i < 12; i++ {
			add(filepath.Join(dir, defaultConfigName))
			if DemoEnabled() {
				add(filepath.Join(dir, demoConfigName))
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	if home, err := os.UserHomeDir(); err == nil {
		add(filepath.Join(home, ".org-cost", defaultConfigName))
		add(filepath.Join(home, ".config", "org-cost-api", defaultConfigName))
	}

	return out
}

// LoadAuto discovers config.yaml (unless explicit is set) and loads it.
func LoadAuto(explicit string) (*Config, string, error) {
	path, err := DiscoverPath(explicit)
	if err != nil {
		return nil, "", err
	}
	cfg, err := Load(path)
	if err != nil {
		return nil, path, err
	}
	return cfg, path, nil
}
