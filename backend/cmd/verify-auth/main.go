// Verify AWS credentials and Cost Explorer access for config.yaml accounts.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	awsclient "github.com/kaskol10/org-cost-api/backend/internal/aws"
	appconfig "github.com/kaskol10/org-cost-api/backend/internal/config"
)

type checkStatus string

const (
	statusOK     checkStatus = "ok"
	statusFailed checkStatus = "failed"
)

type checkResult struct {
	Name    string      `json:"name"`
	Status  checkStatus `json:"status"`
	Detail  string      `json:"detail,omitempty"`
	Mode    string      `json:"mode,omitempty"`
	Account string      `json:"account_id,omitempty"`
}

type verifyReport struct {
	ConfigPath string        `json:"config_path"`
	Passed     bool          `json:"passed"`
	Checks     []checkResult `json:"checks"`
	Summary    string        `json:"summary,omitempty"`
}

func main() {
	format := flag.String("format", "text", "Output format: text, json, or markdown")
	flag.Parse()

	args := flag.Args()
	configPath := "../config.yaml"
	if len(args) > 0 {
		configPath = args[0]
	}

	cfg, err := appconfig.Load(configPath)
	if err != nil {
		emitError(*format, fmt.Sprintf("Failed to load config: %v", err))
		os.Exit(1)
	}

	ctx := context.Background()
	report := verifyReport{
		ConfigPath: configPath,
		Passed:     true,
	}

	end := time.Now().UTC()
	start := end.AddDate(0, 0, -7)
	startStr := start.Format("2006-01-02")
	endStr := end.Format("2006-01-02")

	var billingCE *costexplorer.Client

	if cfg.BillingProfile != "" {
		name := fmt.Sprintf("billing:%s", cfg.BillingProfile)
		if err := checkProfileSTS(ctx, cfg.BillingProfile); err != nil {
			report.Checks = append(report.Checks, checkResult{
				Name: name + ":sts", Status: statusFailed, Detail: err.Error(),
			})
			report.Passed = false
		} else {
			report.Checks = append(report.Checks, checkResult{
				Name: name + ":sts", Status: statusOK, Detail: "STS OK",
			})
		}
		ce, err := awsclient.LoadBillingCostClient(ctx, cfg.BillingProfile)
		if err != nil {
			report.Checks = append(report.Checks, checkResult{
				Name: name + ":ce", Status: statusFailed, Detail: err.Error(),
			})
			report.Passed = false
		} else {
			if err := smokeCE(ctx, ce, nil, startStr, endStr); err != nil {
				report.Checks = append(report.Checks, checkResult{
					Name: name + ":ce", Status: statusFailed, Detail: err.Error(),
				})
				report.Passed = false
			} else {
				report.Checks = append(report.Checks, checkResult{
					Name: name + ":ce", Status: statusOK, Detail: "CE OK (payer, unfiltered)", Mode: "payer-linked",
				})
				billingCE = ce
			}
		}
	}

	if cfg.HasBillingStaticCredentials() {
		envName := cfg.BillingSecretAccessKeyEnv
		if envName != "" && strings.TrimSpace(os.Getenv(envName)) == "" {
			report.Checks = append(report.Checks, checkResult{
				Name: "billing:keys", Status: statusFailed,
				Detail: fmt.Sprintf("env %s is not set", envName),
			})
			report.Passed = false
		} else {
			static, err := cfg.ResolveBillingStaticCredentials()
			if err != nil {
				report.Checks = append(report.Checks, checkResult{
					Name: "billing:keys", Status: statusFailed, Detail: err.Error(),
				})
				report.Passed = false
			} else {
				ce, err := awsclient.LoadBillingCostClientWithCredentials(ctx, static)
				if err != nil {
					report.Checks = append(report.Checks, checkResult{
						Name: "billing:keys:ce", Status: statusFailed, Detail: err.Error(),
					})
					report.Passed = false
				} else if err := smokeCE(ctx, ce, nil, startStr, endStr); err != nil {
					report.Checks = append(report.Checks, checkResult{
						Name: "billing:keys:ce", Status: statusFailed, Detail: err.Error(),
					})
					report.Passed = false
				} else {
					report.Checks = append(report.Checks, checkResult{
						Name: "billing:keys:ce", Status: statusOK, Detail: "CE OK (payer keys)", Mode: "payer-linked",
					})
					if billingCE == nil {
						billingCE = ce
					}
				}
			}
		}
	}

	for _, acct := range cfg.Accounts {
		acctName := acct.Name
		clients, err := awsclient.LoadAccountClients(ctx, acct)
		if err != nil {
			report.Checks = append(report.Checks, checkResult{
				Name: acctName + ":sts", Status: statusFailed, Detail: err.Error(),
			})
			report.Passed = false
			continue
		}
		report.Checks = append(report.Checks, checkResult{
			Name: acctName + ":sts", Status: statusOK,
			Detail: "STS OK", Account: clients.AccountID,
		})

		bp := acct.BillingProfile
		var ce *costexplorer.Client
		var linkedID *string
		mode := ""
		switch bp {
		case "account":
			ce = clients.Cost
			mode = "member-billed"
		case "":
			if billingCE != nil {
				ce = billingCE
				linkedID = aws.String(clients.AccountID)
				mode = "payer-linked"
			} else {
				ce = clients.Cost
				mode = "account-credentials"
			}
		default:
			var err error
			ce, err = awsclient.LoadBillingCostClient(ctx, bp)
			if err != nil {
				report.Checks = append(report.Checks, checkResult{
					Name: acctName + ":ce", Status: statusFailed, Detail: err.Error(), Mode: "custom-billing",
				})
				report.Passed = false
				continue
			}
			linkedID = aws.String(clients.AccountID)
			mode = fmt.Sprintf("custom-billing:%s", bp)
		}

		if err := smokeCE(ctx, ce, linkedID, startStr, endStr); err != nil {
			report.Checks = append(report.Checks, checkResult{
				Name: acctName + ":ce", Status: statusFailed, Detail: err.Error(), Mode: mode, Account: clients.AccountID,
			})
			report.Passed = false
		} else {
			report.Checks = append(report.Checks, checkResult{
				Name: acctName + ":ce", Status: statusOK, Detail: "CE OK", Mode: mode, Account: clients.AccountID,
			})
		}
	}

	if report.Passed {
		report.Summary = fmt.Sprintf("All checks passed. Start: cd backend && go run ./cmd/server -config %s", configPath)
	} else {
		report.Summary = "One or more checks failed — fix STS/CE errors above before starting the server."
	}

	emitReport(*format, report)
	if !report.Passed {
		os.Exit(1)
	}
}

func emitError(format, msg string) {
	switch format {
	case "json":
		b, _ := json.Marshal(map[string]string{"error": msg})
		fmt.Fprintln(os.Stderr, string(b))
	default:
		fmt.Fprintln(os.Stderr, msg)
	}
}

func emitReport(format string, report verifyReport) {
	switch format {
	case "json":
		b, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "json encode: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(b))
	case "markdown":
		fmt.Printf("# AWS auth verification\n\n")
		fmt.Printf("**Config:** `%s`\n\n", report.ConfigPath)
		fmt.Printf("| Check | Status | Mode | Detail |\n")
		fmt.Printf("|-------|--------|------|--------|\n")
		for _, c := range report.Checks {
			st := "✅ ok"
			if c.Status == statusFailed {
				st = "❌ failed"
			}
			mode := c.Mode
			if mode == "" {
				mode = "—"
			}
			fmt.Printf("| %s | %s | %s | %s |\n", c.Name, st, mode, c.Detail)
		}
		fmt.Printf("\n**Result:** %s\n", report.Summary)
	default:
		fmt.Printf("Using config: %s\n\n", report.ConfigPath)
		for _, c := range report.Checks {
			prefix := "OK"
			if c.Status == statusFailed {
				prefix = "FAILED"
			}
			line := fmt.Sprintf("[%s] %s", prefix, c.Name)
			if c.Mode != "" {
				line += fmt.Sprintf(" (%s)", c.Mode)
			}
			if c.Account != "" {
				line += fmt.Sprintf(" account=%s", c.Account)
			}
			line += ": " + c.Detail
			if c.Status == statusFailed {
				fmt.Fprintln(os.Stderr, line)
			} else {
				fmt.Println(line)
			}
		}
		fmt.Println()
		if report.Passed {
			fmt.Println(report.Summary)
		} else {
			fmt.Fprintln(os.Stderr, report.Summary)
		}
	}
}

func checkProfileSTS(ctx context.Context, profile string) error {
	_, err := awsclient.LoadAccountClients(ctx, appconfig.Account{Profile: profile, Region: "us-east-1"})
	return err
}

func smokeCE(ctx context.Context, client *costexplorer.Client, linkedAccountID *string, start, end string) error {
	input := &costexplorer.GetCostAndUsageInput{
		TimePeriod: &types.DateInterval{
			Start: aws.String(start),
			End:   aws.String(end),
		},
		Granularity: types.GranularityDaily,
		Metrics:     []string{"UnblendedCost"},
	}
	if linkedAccountID != nil && *linkedAccountID != "" {
		input.Filter = &types.Expression{
			Dimensions: &types.DimensionValues{
				Key:    types.DimensionLinkedAccount,
				Values: []string{*linkedAccountID},
			},
		}
	}
	out, err := client.GetCostAndUsage(ctx, input)
	if err != nil {
		return err
	}
	if len(out.ResultsByTime) == 0 {
		return fmt.Errorf("no results (check CE enabled and date range)")
	}
	return nil
}
