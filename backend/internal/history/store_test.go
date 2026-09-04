package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveSnapshotDedupe(t *testing.T) {
	dir := t.TempDir()
	store := &Store{dir: dir}
	snap := Snapshot{
		SavedAt:     "2026-08-10T12:00:00Z",
		PeriodStart: "2026-07-11",
		PeriodEnd:   "2026-08-10",
		OrgTotal:    1000,
	}
	if err := store.SaveSnapshot(snap); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSnapshot(snap); err != nil {
		t.Fatal(err)
	}
	snaps, err := store.ListSnapshots()
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 1 {
		t.Fatalf("got %d snapshots, want 1", len(snaps))
	}
}

func TestFindPriorSnapshot(t *testing.T) {
	dir := t.TempDir()
	store := &Store{dir: dir}
	target := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)

	for _, snap := range []Snapshot{
		{SavedAt: "2026-06-01T00:00:00Z", PeriodStart: "2026-05-02", PeriodEnd: "2026-06-01", OrgTotal: 900},
		{SavedAt: "2026-07-01T00:00:00Z", PeriodStart: "2026-06-02", PeriodEnd: "2026-07-01", OrgTotal: 950},
	} {
		if err := store.SaveSnapshot(snap); err != nil {
			t.Fatal(err)
		}
	}

	best, err := store.FindPriorSnapshot(target, 5)
	if err != nil {
		t.Fatal(err)
	}
	if best == nil {
		t.Fatal("expected a prior snapshot")
	}
	if best.PeriodEnd != "2026-07-01" {
		t.Fatalf("got period end %q, want 2026-07-01", best.PeriodEnd)
	}
}

func TestPriorCacheMissHit(t *testing.T) {
	dir := t.TempDir()
	store := &Store{dir: dir}

	if _, err := store.LoadPriorCache("2026-06-01", "2026-07-01"); err == nil {
		t.Fatal("expected cache miss")
	}

	cache := PriorCache{
		PeriodStart: "2026-06-01",
		PeriodEnd:   "2026-07-01",
		FetchedAt:   time.Now().UTC().Format(time.RFC3339),
		OrgTotal:    500,
		Services:    []ServiceAmount{{Service: "Amazon S3", Amount: 100}},
		Source:      "cost_explorer_payer",
	}
	if err := store.SavePriorCache(cache); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.LoadPriorCache("2026-06-01", "2026-07-01")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.OrgTotal != 500 {
		t.Fatalf("got total %v, want 500", loaded.OrgTotal)
	}
	if _, err := store.LoadPriorCache("2026-05-01", "2026-06-01"); err == nil {
		t.Fatal("expected period mismatch")
	}
}

func TestDefaultHistoryDirPrefersNewPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	newDir := filepath.Join(home, ".org-cost", "history")
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newDir, "2026-08-10_2026-07-11_2026-08-10.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := defaultHistoryDir()
	if got != newDir {
		t.Fatalf("got %q, want %q", got, newDir)
	}
}
