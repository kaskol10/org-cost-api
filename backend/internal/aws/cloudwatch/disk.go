package cloudwatch

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

// InstanceDiskUsage is aggregate filesystem usage reported by the CloudWatch agent (CWAgent).
type InstanceDiskUsage struct {
	UsedBytes  float64
	TotalBytes float64
}

// FetchInstanceDiskUsage returns latest disk_used / disk_total per instance (summed across mount points).
// Missing instances have no entry in the map (agent not installed or no permission).
func FetchInstanceDiskUsage(ctx context.Context, client *cloudwatch.Client, instanceIDs []string) (map[string]InstanceDiskUsage, error) {
	if client == nil || len(instanceIDs) == 0 {
		return map[string]InstanceDiskUsage{}, nil
	}

	const batchSize = 80 // 2 queries per instance, stay under 500 GetMetricData queries
	out := make(map[string]InstanceDiskUsage, len(instanceIDs))
	end := time.Now().UTC()
	start := end.Add(-3 * time.Hour)

	for i := 0; i < len(instanceIDs); i += batchSize {
		endIdx := i + batchSize
		if endIdx > len(instanceIDs) {
			endIdx = len(instanceIDs)
		}
		batch := instanceIDs[i:endIdx]
		used, err := fetchDiskMetric(ctx, client, batch, "disk_used", start, end)
		if err != nil {
			return nil, err
		}
		total, err := fetchDiskMetric(ctx, client, batch, "disk_total", start, end)
		if err != nil {
			return nil, err
		}
		for _, id := range batch {
			u, okU := used[id]
			t, okT := total[id]
			if !okU && !okT {
				continue
			}
			out[id] = InstanceDiskUsage{UsedBytes: u, TotalBytes: t}
		}
	}
	return out, nil
}

func fetchDiskMetric(
	ctx context.Context,
	client *cloudwatch.Client,
	instanceIDs []string,
	metricName string,
	start, end time.Time,
) (map[string]float64, error) {
	queries := make([]types.MetricDataQuery, 0, len(instanceIDs))
	for _, id := range instanceIDs {
		safe := strings.ReplaceAll(id, "-", "_")
		queries = append(queries, types.MetricDataQuery{
			Id: aws.String(fmt.Sprintf("%s_%s", metricName, safe)),
			Expression: aws.String(fmt.Sprintf(
				`SUM(SEARCH('{CWAgent,%s,InstanceId} InstanceId="%s"', 'Maximum', 3600))`,
				metricName, id,
			)),
			ReturnData: aws.Bool(true),
		})
	}

	out := make(map[string]float64, len(instanceIDs))
	paginator := cloudwatch.NewGetMetricDataPaginator(client, &cloudwatch.GetMetricDataInput{
		StartTime:         aws.Time(start),
		EndTime:           aws.Time(end),
		MetricDataQueries: queries,
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			if isAccessDenied(err) {
				return out, nil
			}
			return nil, fmt.Errorf("get metric data %s: %w", metricName, err)
		}
		for _, result := range page.MetricDataResults {
			if result.Id == nil || len(result.Values) == 0 {
				continue
			}
			id := instanceIDFromQueryID(aws.ToString(result.Id), metricName)
			if id == "" {
				continue
			}
			// Use the latest non-zero datapoint in the window.
			val := latestValue(result.Values)
			if val > 0 {
				out[id] = val
			}
		}
	}
	return out, nil
}

func instanceIDFromQueryID(queryID, metricName string) string {
	prefix := metricName + "_"
	if !strings.HasPrefix(queryID, prefix) {
		return ""
	}
	safe := strings.TrimPrefix(queryID, prefix)
	return "i-" + strings.ReplaceAll(safe, "_", "-")
}

func latestValue(values []float64) float64 {
	var best float64
	for _, v := range values {
		if v > best {
			best = v
		}
	}
	return best
}

func isAccessDenied(err error) bool {
	return err != nil && strings.Contains(err.Error(), "AccessDenied")
}
