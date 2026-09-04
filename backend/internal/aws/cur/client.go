package cur

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	"github.com/aws/aws-sdk-go-v2/service/athena/types"

	appconfig "github.com/kaskol10/org-cost-api/backend/internal/config"
)

// Client runs Athena SQL against the organization CUR table.
type Client struct {
	athena *athena.Client
	cfg    appconfig.CURConfig
}

func NewClient(ctx context.Context, cfg appconfig.CURConfig, profile string) (*Client, error) {
	if cfg.Database == "" || cfg.Table == "" {
		return nil, fmt.Errorf("cur.database and cur.table are required")
	}
	if cfg.OutputS3 == "" {
		return nil, fmt.Errorf("cur.output_s3 is required (Athena query results bucket)")
	}
	region := cfg.Region
	if region == "" {
		region = "eu-west-1"
	}
	var opts []func(*config.LoadOptions) error
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}
	opts = append(opts, config.WithRegion(region))

	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("load aws config for CUR/Athena: %w", err)
	}
	return &Client{
		athena: athena.NewFromConfig(awsCfg),
		cfg:    cfg,
	}, nil
}

func (c *Client) tableRef() string {
	t := c.cfg.Table
	if strings.Contains(t, ".") {
		return t
	}
	return fmt.Sprintf("%s.%s", c.cfg.Database, t)
}

func (c *Client) query(ctx context.Context, sql string) ([][]string, error) {
	input := &athena.StartQueryExecutionInput{
		QueryString: aws.String(sql),
		QueryExecutionContext: &types.QueryExecutionContext{
			Database: aws.String(c.cfg.Database),
		},
		ResultConfiguration: &types.ResultConfiguration{
			OutputLocation: aws.String(c.cfg.OutputS3),
		},
	}
	if c.cfg.Workgroup != "" {
		input.WorkGroup = aws.String(c.cfg.Workgroup)
	}

	startOut, err := c.athena.StartQueryExecution(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("start athena query: %w", err)
	}
	execID := aws.ToString(startOut.QueryExecutionId)

	deadline := time.Now().Add(3 * time.Minute)
	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("athena query timed out")
		}
		statusOut, err := c.athena.GetQueryExecution(ctx, &athena.GetQueryExecutionInput{
			QueryExecutionId: aws.String(execID),
		})
		if err != nil {
			return nil, fmt.Errorf("get query execution: %w", err)
		}
		state := statusOut.QueryExecution.Status.State
		switch state {
		case types.QueryExecutionStateSucceeded:
			return c.readResults(ctx, execID)
		case types.QueryExecutionStateFailed:
			reason := aws.ToString(statusOut.QueryExecution.Status.StateChangeReason)
			return nil, fmt.Errorf("athena query failed: %s", reason)
		case types.QueryExecutionStateCancelled:
			return nil, fmt.Errorf("athena query cancelled")
		default:
			time.Sleep(2 * time.Second)
		}
	}
}

func (c *Client) readResults(ctx context.Context, execID string) ([][]string, error) {
	var rows [][]string
	var nextToken *string
	for {
		out, err := c.athena.GetQueryResults(ctx, &athena.GetQueryResultsInput{
			QueryExecutionId: aws.String(execID),
			NextToken:        nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("get query results: %w", err)
		}
		for i, row := range out.ResultSet.Rows {
			if i == 0 && len(rows) == 0 {
				continue // header row
			}
			var cells []string
			for _, d := range row.Data {
				cells = append(cells, aws.ToString(d.VarCharValue))
			}
			rows = append(rows, cells)
		}
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}
	return rows, nil
}

func escapeSQL(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
