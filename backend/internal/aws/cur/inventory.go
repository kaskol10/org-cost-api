package cur

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// DailyResourceCount is distinct billed resources per day from CUR (e.g. vol-* per day).
type DailyResourceCount struct {
	Date        string  `json:"date"`
	Count       int     `json:"count"`
	UsageAmount float64 `json:"usage_amount,omitempty"`
}

// AccountDailyInventory maps account ID to daily series.
type AccountDailyInventory struct {
	Volumes   map[string][]DailyResourceCount
	Snapshots map[string][]DailyResourceCount
}

// FetchEBSInventory returns per-account daily distinct volume and snapshot counts from CUR.
func (c *Client) FetchEBSInventory(ctx context.Context, accountIDs []string, start, end string) (*AccountDailyInventory, error) {
	if len(accountIDs) == 0 {
		return &AccountDailyInventory{
			Volumes:   map[string][]DailyResourceCount{},
			Snapshots: map[string][]DailyResourceCount{},
		}, nil
	}

	volRows, err := c.queryDailyResourceCounts(ctx, accountIDs, start, end, "volume")
	if err != nil {
		return nil, err
	}
	snapRows, err := c.queryDailyResourceCounts(ctx, accountIDs, start, end, "snapshot")
	if err != nil {
		return nil, err
	}

	return &AccountDailyInventory{
		Volumes:   groupDailyRows(volRows),
		Snapshots: groupDailyRows(snapRows),
	}, nil
}

func (c *Client) queryDailyResourceCounts(ctx context.Context, accountIDs []string, start, end, kind string) ([][]string, error) {
	inList := make([]string, len(accountIDs))
	for i, id := range accountIDs {
		inList[i] = "'" + escapeSQL(id) + "'"
	}

	var resourceFilter, usageFilter string
	switch kind {
	case "volume":
		resourceFilter = "line_item_resource_id LIKE 'vol-%'"
		usageFilter = "LOWER(line_item_usage_type) LIKE '%volumeusage%'"
	case "snapshot":
		resourceFilter = "line_item_resource_id LIKE 'snap-%'"
		usageFilter = "(LOWER(line_item_usage_type) LIKE '%snapshot%' OR LOWER(line_item_operation) LIKE '%snapshot%')"
	default:
		return nil, fmt.Errorf("unknown resource kind %q", kind)
	}

	partitionClause := c.partitionClause(start, end)

	sql := fmt.Sprintf(`
SELECT
  line_item_usage_account_id,
  CAST(line_item_usage_start_date AS DATE) AS usage_date,
  COUNT(DISTINCT line_item_resource_id) AS resource_count,
  COALESCE(SUM(line_item_usage_amount), 0) AS usage_amount
FROM %s
WHERE line_item_line_item_type = 'Usage'
  AND %s
  AND %s
  AND line_item_usage_account_id IN (%s)
  AND line_item_usage_start_date >= TIMESTAMP '%s 00:00:00'
  AND line_item_usage_start_date < TIMESTAMP '%s 00:00:00'
  %s
GROUP BY line_item_usage_account_id, CAST(line_item_usage_start_date AS DATE)
ORDER BY usage_date
`, c.tableRef(), resourceFilter, usageFilter, strings.Join(inList, ","),
		escapeSQL(start), escapeSQL(end), partitionClause)

	return c.query(ctx, sql)
}

func (c *Client) partitionClause(start, end string) string {
	if !c.cfg.Partitioned {
		return ""
	}
	startT, err1 := time.Parse("2006-01-02", start)
	endT, err2 := time.Parse("2006-01-02", end)
	if err1 != nil || err2 != nil {
		return ""
	}
	// Typical CUR Athena tables: year string, month string '1'-'12'
	var parts []string
	for d := startT; !d.After(endT); d = d.AddDate(0, 1, 0) {
		parts = append(parts, fmt.Sprintf("(year = '%d' AND month = '%d')", d.Year(), int(d.Month())))
	}
	if len(parts) == 0 {
		return ""
	}
	return "AND (" + strings.Join(parts, " OR ") + ")"
}

func groupDailyRows(rows [][]string) map[string][]DailyResourceCount {
	byAccount := make(map[string][]DailyResourceCount)
	for _, row := range rows {
		if len(row) < 3 {
			continue
		}
		accountID := row[0]
		date := row[1]
		if len(date) > 10 {
			date = date[:10]
		}
		count, _ := strconv.Atoi(row[2])
		var usage float64
		if len(row) > 3 {
			usage, _ = strconv.ParseFloat(row[3], 64)
		}
		byAccount[accountID] = append(byAccount[accountID], DailyResourceCount{
			Date:        date,
			Count:       count,
			UsageAmount: usage,
		})
	}
	for acct := range byAccount {
		sort.Slice(byAccount[acct], func(i, j int) bool {
			return byAccount[acct][i].Date < byAccount[acct][j].Date
		})
	}
	return byAccount
}
