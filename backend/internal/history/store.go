package history

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ServiceAmount is org-wide spend for one AWS service in a snapshot period.
type ServiceAmount struct {
	Service string  `json:"service"`
	Amount  float64 `json:"amount"`
}

// AccountAmount is per-account spend in a snapshot.
type AccountAmount struct {
	AccountID   string          `json:"account_id"`
	AccountName string          `json:"account_name"`
	AllTotal    float64         `json:"all_total"`
	Services    []ServiceAmount `json:"services,omitempty"`
}

// Snapshot is a point-in-time cost summary for trend comparison (no extra CE calls).
type Snapshot struct {
	SavedAt     string          `json:"saved_at"`
	PeriodStart string          `json:"period_start"`
	PeriodEnd   string          `json:"period_end"`
	OrgTotal    float64         `json:"org_total"`
	Services    []ServiceAmount `json:"services"`
	Accounts    []AccountAmount `json:"accounts"`
}

// PriorCache stores a prior-period CE result to avoid repeat API calls.
type PriorCache struct {
	PeriodStart string          `json:"period_start"`
	PeriodEnd   string          `json:"period_end"`
	FetchedAt   string          `json:"fetched_at"`
	Services    []ServiceAmount `json:"services"`
	Accounts    []AccountAmount `json:"accounts,omitempty"`
	OrgTotal    float64         `json:"org_total"`
	Source      string          `json:"source"`
}

// Store persists daily snapshots and prior-period CE cache on disk.
type Store struct {
	dir string
}

func NewStore(dir string) (*Store, error) {
	if dir == "" {
		dir = defaultHistoryDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir history: %w", err)
	}
	return &Store{dir: dir}, nil
}

func defaultHistoryDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".org-cost", "history")
	}
	newDir := filepath.Join(home, ".org-cost", "history")
	legacyDir := filepath.Join(home, ".ec2-other", "history")
	if hasHistoryFiles(newDir) || !hasHistoryFiles(legacyDir) {
		return newDir
	}
	log.Printf("history: using legacy dir %s (migrate to ~/.org-cost/history)", legacyDir)
	return legacyDir
}

func hasHistoryFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") && e.Name() != "prior-cache.json" {
			return true
		}
	}
	return false
}

func (s *Store) Dir() string { return s.dir }

// SaveSnapshot writes one snapshot per UTC day (deduped by date + period).
func (s *Store) SaveSnapshot(snap Snapshot) error {
	if snap.PeriodStart == "" || snap.PeriodEnd == "" {
		return fmt.Errorf("snapshot missing period")
	}
	if snap.SavedAt == "" {
		snap.SavedAt = time.Now().UTC().Format(time.RFC3339)
	}
	day := strings.Split(snap.SavedAt, "T")[0]
	name := fmt.Sprintf("%s_%s_%s.json", day, snap.PeriodStart, snap.PeriodEnd)
	path := filepath.Join(s.dir, name)
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return writeJSON(path, snap)
}

// FindPriorSnapshot returns the snapshot whose period end is closest to targetEnd (prior period).
func (s *Store) FindPriorSnapshot(targetEnd time.Time, maxSkewDays int) (*Snapshot, error) {
	snaps, err := s.ListSnapshots()
	if err != nil {
		return nil, err
	}
	var best *Snapshot
	var bestDelta time.Duration
	for i := range snaps {
		end, err := time.Parse("2006-01-02", snaps[i].PeriodEnd)
		if err != nil {
			continue
		}
		delta := targetEnd.Sub(end)
		if delta < 0 {
			delta = -delta
		}
		if delta > time.Duration(maxSkewDays)*24*time.Hour {
			continue
		}
		if best == nil || delta < bestDelta {
			best = &snaps[i]
			bestDelta = delta
		}
	}
	return best, nil
}

// ListSnapshots returns all snapshots sorted by saved_at ascending.
func (s *Store) ListSnapshots() ([]Snapshot, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	var out []Snapshot
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") || e.Name() == "prior-cache.json" {
			continue
		}
		var snap Snapshot
		if err := readJSON(filepath.Join(s.dir, e.Name()), &snap); err != nil {
			continue
		}
		out = append(out, snap)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SavedAt < out[j].SavedAt })
	return out, nil
}

// LoadPriorCache reads cached prior-period CE totals.
func (s *Store) LoadPriorCache(periodStart, periodEnd string) (*PriorCache, error) {
	var c PriorCache
	if err := readJSON(filepath.Join(s.dir, "prior-cache.json"), &c); err != nil {
		return nil, err
	}
	if c.PeriodStart != periodStart || c.PeriodEnd != periodEnd {
		return nil, fmt.Errorf("cache miss")
	}
	return &c, nil
}

// SavePriorCache stores prior-period CE totals.
func (s *Store) SavePriorCache(c PriorCache) error {
	return writeJSON(filepath.Join(s.dir, "prior-cache.json"), c)
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
