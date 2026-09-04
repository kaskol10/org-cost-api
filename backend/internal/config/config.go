package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
)

type Account struct {
	ID      string `yaml:"id"`
	Name    string `yaml:"name"`
	Profile string `yaml:"profile,omitempty"`
	RoleARN string `yaml:"role_arn,omitempty"`
	Region  string `yaml:"region,omitempty"`
	// BillingProfile overrides the global billing_profile for Cost Explorer.
	// Use "account" to query from this account's own CE client (no LINKED_ACCOUNT filter).
	BillingProfile string `yaml:"billing_profile,omitempty"`

	// Optional static credentials (alternative to Granted / profile).
	// Prefer secret_access_key_env over putting secrets in config.yaml.
	AccessKeyID        string `yaml:"access_key_id,omitempty"`
	SecretAccessKey    string `yaml:"secret_access_key,omitempty"`
	SecretAccessKeyEnv string `yaml:"secret_access_key_env,omitempty"`
	SessionToken       string `yaml:"session_token,omitempty"`
	SessionTokenEnv    string `yaml:"session_token_env,omitempty"`
}

// CURConfig enables historical EBS inventory via Athena over the Cost and Usage Report.
type CURConfig struct {
	Enabled     bool   `yaml:"enabled"`
	Profile     string `yaml:"profile,omitempty"` // defaults to billing_profile
	Region      string `yaml:"region,omitempty"`  // Athena workgroup region, default eu-west-1
	Database    string `yaml:"database"`
	Table       string `yaml:"table"`       // table name or database.table
	Workgroup   string `yaml:"workgroup,omitempty"`
	OutputS3    string `yaml:"output_s3"`   // s3://bucket/prefix/ for Athena results
	Partitioned bool   `yaml:"partitioned"` // set true if table has year/month columns
}

type Config struct {
	Accounts   []Account `yaml:"accounts"`
	ListenAddr string    `yaml:"listen_addr"`
	CORSOrigin string    `yaml:"cors_origin"`
	// CostLookbackDays is how far back to query Cost Explorer (max useful ~14 months).
	CostLookbackDays int `yaml:"cost_lookback_days"`
	// BillingProfile is the Granted/AWS profile for the organization payer (e.g. Master).
	// When set, all Cost Explorer queries run from that account with a per-account LINKED_ACCOUNT filter.
	BillingProfile string `yaml:"billing_profile,omitempty"`
	// BillingRoleARN assumes this role via the default credential chain (IRSA) for payer CE.
	// Use when billing_profile is unset (e.g. EKS). Typical: arn:aws:iam::PAYER:role/OrgCostReadOnly.
	BillingRoleARN string `yaml:"billing_role_arn,omitempty"`
	// Optional payer static credentials (alternative to billing_profile).
	BillingAccessKeyID        string `yaml:"billing_access_key_id,omitempty"`
	BillingSecretAccessKey    string `yaml:"billing_secret_access_key,omitempty"`
	BillingSecretAccessKeyEnv string `yaml:"billing_secret_access_key_env,omitempty"`
	BillingSessionToken       string `yaml:"billing_session_token,omitempty"`
	BillingSessionTokenEnv    string `yaml:"billing_session_token_env,omitempty"`
	// HistoryDir stores daily cost snapshots for trends (default ~/.org-cost/history).
	HistoryDir string `yaml:"history_dir,omitempty"`
	// PriorPeriodCacheHours avoids repeat CE calls for prior-period comparison (default 24).
	PriorPeriodCacheHours int `yaml:"prior_period_cache_hours,omitempty"`
	// APIToken optionally protects /api/* routes (prefer api_token_env).
	APIToken    string `yaml:"api_token,omitempty"`
	APITokenEnv string `yaml:"api_token_env,omitempty"`
	CUR            *CURConfig `yaml:"cur,omitempty"`
	// Demo serves fixture data only — no AWS calls (also enabled via ORG_COST_DEMO=1).
	Demo bool `yaml:"demo,omitempty"`
}

// DemoEnabled is true when ORG_COST_DEMO is set (env wins over config file).
func DemoEnabled() bool {
	v := strings.TrimSpace(os.Getenv("ORG_COST_DEMO"))
	return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if DemoEnabled() {
		cfg.Demo = true
	}
	if cfg.Demo {
		return finalizeDemoConfig(&cfg)
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8080"
	}
	if cfg.CORSOrigin == "" {
		cfg.CORSOrigin = "http://localhost:5173"
	}
	if cfg.CostLookbackDays <= 0 {
		cfg.CostLookbackDays = 30
	}
	if cfg.PriorPeriodCacheHours <= 0 {
		cfg.PriorPeriodCacheHours = 24
	}
	if len(cfg.Accounts) == 0 {
		return nil, fmt.Errorf("at least one account must be configured")
	}
	for _, acct := range cfg.Accounts {
		if err := validateAccountAuth(acct); err != nil {
			return nil, err
		}
	}
	if cfg.HasBillingStaticCredentials() {
		if _, err := cfg.ResolveBillingStaticCredentials(); err != nil {
			return nil, fmt.Errorf("billing credentials: %w", err)
		}
	}
	if err := validateAPIToken(&cfg); err != nil {
		return nil, err
	}
	if dir := strings.TrimSpace(os.Getenv("HISTORY_DIR")); dir != "" {
		cfg.HistoryDir = dir
	}
	return &cfg, nil
}

func finalizeDemoConfig(cfg *Config) (*Config, error) {
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8080"
	}
	if cfg.CORSOrigin == "" {
		cfg.CORSOrigin = "*"
	}
	if cfg.CostLookbackDays <= 0 {
		cfg.CostLookbackDays = 30
	}
	if len(cfg.Accounts) == 0 {
		cfg.Accounts = []Account{
			{ID: "111111111111", Name: "production"},
			{ID: "222222222222", Name: "staging"},
			{ID: "333333333333", Name: "analytics"},
			{ID: "444444444444", Name: "sandbox"},
			{ID: "555555555555", Name: "legacy"},
		}
	}
	if dir := strings.TrimSpace(os.Getenv("HISTORY_DIR")); dir != "" {
		cfg.HistoryDir = dir
	}
	return cfg, nil
}

// validateAPIToken fails closed when api_token_env is set but resolves to an empty token.
func validateAPIToken(cfg *Config) error {
	envName := strings.TrimSpace(cfg.APITokenEnv)
	if envName == "" {
		return nil
	}
	if strings.TrimSpace(cfg.APIToken) != "" {
		return nil
	}
	if strings.TrimSpace(os.Getenv(envName)) == "" {
		return fmt.Errorf("api_token_env %q is set but environment variable is empty (and no inline api_token)", envName)
	}
	return nil
}

func (c *Config) CostDateRange() (start, end string) {
	endTime := time.Now().UTC()
	startTime := endTime.AddDate(0, 0, -c.CostLookbackDays)
	return startTime.Format("2006-01-02"), endTime.Format("2006-01-02")
}

// ResolveAPIToken returns the configured bearer token, if any.
func (c *Config) ResolveAPIToken() string {
	if token := strings.TrimSpace(c.APIToken); token != "" {
		return token
	}
	if env := strings.TrimSpace(c.APITokenEnv); env != "" {
		return strings.TrimSpace(os.Getenv(env))
	}
	return ""
}
