package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDashboardCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := &Store{dir: dir}
	key := "lookback:2026-08-01:2026-08-31"
	payload := map[string]any{
		"start": "2026-08-01",
		"end":   "2026-08-31",
		"totals": map[string]any{"org_total": 42.0},
	}
	fetched := time.Now().UTC()
	if err := store.SaveDashboardCache(key, payload, fetched); err != nil {
		t.Fatal(err)
	}
	raw, at, err := store.LoadDashboardCache(key, 12*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if at.Unix() != fetched.Unix() {
		t.Fatalf("fetched_at %v, want %v", at, fetched)
	}
	var loaded map[string]any
	if err := json.Unmarshal(raw, &loaded); err != nil {
		t.Fatal(err)
	}
	if loaded["start"] != "2026-08-01" {
		t.Fatalf("start %v", loaded["start"])
	}
	path := filepath.Join(dir, "dashboard-cache", dashboardCacheFileName(key))
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected cache file: %v", err)
	}
	if err := store.DeleteDashboardCache(key); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.LoadDashboardCache(key, 12*time.Hour); err == nil {
		t.Fatal("expected miss after delete")
	}
}

func TestDashboardCacheExpired(t *testing.T) {
	dir := t.TempDir()
	store := &Store{dir: dir}
	key := "lite:mtd:2026-09-01:2026-09-15"
	if err := store.SaveDashboardCache(key, map[string]string{"ok": "1"}, time.Now().UTC().Add(-13*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.LoadDashboardCache(key, 12*time.Hour); err == nil {
		t.Fatal("expected expired cache")
	}
}
