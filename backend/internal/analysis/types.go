package analysis

// DashboardView is a CE-agnostic snapshot for trends and suggestions.
type DashboardView struct {
	GeneratedAt string
	Start       string
	End         string
	OrgTotal    float64
	TopServices []ServiceDriverView
	Accounts    []AccountView
	Totals      TotalsView
	// OrgDaily is org-wide daily spend for the period (usage, all services).
	OrgDaily []DailyPoint
	// AccountDaily holds per-account daily spend for anomaly attribution.
	AccountDaily []AccountDailyView
	// IncompleteDays marks the most recent days as CE-lagging (understated).
	IncompleteDays int
	// Commitments summarizes savings-plan / RI coverage (payer CE only).
	Commitments *CommitmentView
}

// CommitmentView is a CE-agnostic commitment coverage snapshot.
type CommitmentView struct {
	SPCoveragePct  *float64 `json:"sp_coverage_pct,omitempty"`
	SPUtilizationPct *float64 `json:"sp_utilization_pct,omitempty"`
	RICoveragePct  *float64 `json:"ri_coverage_pct,omitempty"`
	UncommittedUSD *float64 `json:"uncommitted_usd,omitempty"`
	HasCommitments bool     `json:"has_commitments,omitempty"`
}

// DailyPoint is org-wide spend for one calendar day.
type DailyPoint struct {
	Date   string  `json:"date"`
	Amount float64 `json:"amount"`
}

// AccountDailyView is one account's daily spend.
type AccountDailyView struct {
	AccountID   string       `json:"account_id"`
	AccountName string       `json:"account_name"`
	Daily       []DailyPoint `json:"daily"`
}

type ServiceDriverView struct {
	Service string
	Amount  float64
}

type AccountView struct {
	AccountID   string
	AccountName string
	AllTotal    float64
	OtherServicesTotal float64
	ByService   []ServiceDriverView
	Volumes     *VolumeView
}

type VolumeView struct {
	Count          int
	AvailableCount int
	AvailableGiB   float64
	TotalGiB       float64
}

type TotalsView struct {
	OrgTotal                 float64
	VolumeAvailable          int
	VolumeSizeGiB            float64
	VolumeFilesystemUsedGiB  *float64
	VolumeUtilizationPercent *float64
}
