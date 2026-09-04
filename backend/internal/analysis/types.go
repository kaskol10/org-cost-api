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
