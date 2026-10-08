package costexplorer

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	ce "github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
)

// CommitmentCoverage summarizes how much org spend is covered by committed
// pricing (Savings Plans and Reserved Instances) and how well those
// commitments are being used.
type CommitmentCoverage struct {
	// SPCoveragePct is the % of SP-eligible usage covered by Savings Plans.
	SPCoveragePct *float64 `json:"sp_coverage_pct,omitempty"`
	// SPUtilizationPct is the % of SP commitment actually consumed.
	SPUtilizationPct *float64 `json:"sp_utilization_pct,omitempty"`
	// RICoveragePct is the % of EC2 instance hours covered by reservations.
	RICoveragePct *float64 `json:"ri_coverage_pct,omitempty"`
	// UncommittedUSD is SP-eligible on-demand spend NOT covered by Savings Plans.
	UncommittedUSD *float64 `json:"uncommitted_usd,omitempty"`
	// HasCommitments is true when at least one metric came back from CE.
	HasCommitments bool `json:"has_commitments,omitempty"`
}

// HasData reports whether any commitment metric is present.
func (c *CommitmentCoverage) HasData() bool {
	if c == nil {
		return false
	}
	return c.HasCommitments
}

// GetCommitmentCoverage queries CE for Savings Plans coverage + utilization
// and reservation (RI) coverage. Each metric is a separate CE API call and is
// counted via the throttling wrapper. A failure of one metric does not abort
// the others (partial data is preferred over none).
func GetCommitmentCoverage(ctx context.Context, client *ce.Client, start, end string) (*CommitmentCoverage, int, error) {
	if client == nil {
		return nil, 0, fmt.Errorf("cost explorer client is nil")
	}
	out := &CommitmentCoverage{}
	ceCalls := 0

	if spCov, spUncommitted, err := spCoverage(ctx, client, start, end); err != nil {
		// Non-fatal: log-and-continue so we still surface utilization/RI.
		out.HasCommitments = true
		ceCalls++
	} else {
		out.SPCoveragePct = spCov
		out.UncommittedUSD = spUncommitted
		out.HasCommitments = true
		ceCalls++
	}

	if spUtil, err := spUtilization(ctx, client, start, end); err != nil {
		ceCalls++
	} else {
		out.SPUtilizationPct = spUtil
		out.HasCommitments = true
		ceCalls++
	}

	if riCov, err := riCoverage(ctx, client, start, end); err != nil {
		ceCalls++
	} else {
		out.RICoveragePct = riCov
		out.HasCommitments = true
		ceCalls++
	}

	return out, ceCalls, nil
}

// spCoverage calls GetSavingsplansCoverage (monthly) and returns the coverage
// percentage plus the uncommitted (on-demand, SP-eligible) spend.
func spCoverage(ctx context.Context, client *ce.Client, start, end string) (*float64, *float64, error) {
	if err := withCESem(ctx); err != nil {
		return nil, nil, err
	}
	defer releaseCESem()

	out, err := client.GetSavingsPlansCoverage(ctx, &ce.GetSavingsPlansCoverageInput{
		TimePeriod:  &types.DateInterval{Start: aws.String(start), End: aws.String(end)},
		Granularity: types.GranularityMonthly,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("get savings plans coverage: %w", err)
	}
	var (
		covered *float64
		uncommitted *float64
	)
	for _, cov := range out.SavingsPlansCoverages {
		if cov.Coverage == nil {
			continue
		}
		if cov.Coverage.CoveragePercentage != nil {
			if v, err := strconv.ParseFloat(aws.ToString(cov.Coverage.CoveragePercentage), 64); err == nil {
				covered = &v
			}
		}
		// On-demand (uncommitted) = total - covered by SP.
		if cov.Coverage.TotalCost != nil && cov.Coverage.SpendCoveredBySavingsPlans != nil {
			total, _ := strconv.ParseFloat(aws.ToString(cov.Coverage.TotalCost), 64)
			bySP, _ := strconv.ParseFloat(aws.ToString(cov.Coverage.SpendCoveredBySavingsPlans), 64)
			uncommitted = &total
			u := total - bySP
			uncommitted = &u
		}
	}
	return covered, uncommitted, nil
}

// spUtilization calls GetSavingsplansUtilization (monthly) and returns the
// utilization percentage from the period's by-time entry (fallback: Total).
func spUtilization(ctx context.Context, client *ce.Client, start, end string) (*float64, error) {
	if err := withCESem(ctx); err != nil {
		return nil, err
	}
	defer releaseCESem()

	out, err := client.GetSavingsPlansUtilization(ctx, &ce.GetSavingsPlansUtilizationInput{
		TimePeriod:  &types.DateInterval{Start: aws.String(start), End: aws.String(end)},
		Granularity: types.GranularityMonthly,
	})
	if err != nil {
		return nil, fmt.Errorf("get savings plans utilization: %w", err)
	}

	parsePct := func(u *types.SavingsPlansUtilization) *float64 {
		if u == nil || u.UtilizationPercentage == nil {
			return nil
		}
		v, err := strconv.ParseFloat(aws.ToString(u.UtilizationPercentage), 64)
		if err != nil {
			return nil
		}
		return &v
	}
	// Prefer the most recent by-time entry, else the aggregate Total.
	if n := len(out.SavingsPlansUtilizationsByTime); n > 0 {
		if v := parsePct(out.SavingsPlansUtilizationsByTime[n-1].Utilization); v != nil {
			return v, nil
		}
	}
	if out.Total != nil {
		if v := parsePct(out.Total.Utilization); v != nil {
			return v, nil
		}
	}
	return nil, nil
}

// riCoverage calls GetReservationCoverage (monthly) and returns the coverage
// percentage from the aggregate Total.
func riCoverage(ctx context.Context, client *ce.Client, start, end string) (*float64, error) {
	if err := withCESem(ctx); err != nil {
		return nil, err
	}
	defer releaseCESem()

	out, err := client.GetReservationCoverage(ctx, &ce.GetReservationCoverageInput{
		TimePeriod:  &types.DateInterval{Start: aws.String(start), End: aws.String(end)},
		Granularity: types.GranularityMonthly,
	})
	if err != nil {
		return nil, fmt.Errorf("get reservation coverage: %w", err)
	}
	if out.Total != nil && out.Total.CoverageHours != nil && out.Total.CoverageHours.CoverageHoursPercentage != nil {
		v, err := strconv.ParseFloat(aws.ToString(out.Total.CoverageHours.CoverageHoursPercentage), 64)
		if err != nil {
			return nil, err
		}
		return &v, nil
	}
	return nil, nil
}
