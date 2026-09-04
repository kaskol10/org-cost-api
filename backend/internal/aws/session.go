package awsclient

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	appconfig "github.com/kaskol10/org-cost-api/backend/internal/config"
)

type AccountClients struct {
	Account    appconfig.Account
	AccountID  string
	EC2        *ec2.Client
	CloudWatch *cloudwatch.Client
	Cost       *costexplorer.Client
}

func LoadAccountClients(ctx context.Context, acct appconfig.Account) (*AccountClients, error) {
	region := acct.Region
	if region == "" {
		region = "us-east-1"
	}

	static, err := acct.ResolveStaticCredentials()
	if err != nil {
		return nil, fmt.Errorf("account %q: %w", acct.Name, err)
	}

	baseCfg, err := loadAWSConfig(ctx, region, acct.Profile, static)
	if err != nil {
		return nil, fmt.Errorf("load aws config for %q: %w", acct.Name, err)
	}

	cfg := baseCfg
	if acct.RoleARN != "" {
		stsClient := sts.NewFromConfig(baseCfg)
		cfg.Credentials = stscreds.NewAssumeRoleProvider(stsClient, acct.RoleARN)
	}

	stsClient := sts.NewFromConfig(cfg)
	identity, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, fmt.Errorf("get caller identity for %q: %w", acct.Name, err)
	}
	accountID := aws.ToString(identity.Account)
	if acct.ID != "" && acct.ID != accountID {
		return nil, fmt.Errorf("account %q: configured id %s does not match caller %s", acct.Name, acct.ID, accountID)
	}

	// Cost Explorer is only available in us-east-1.
	costCfg := cfg.Copy()
	costCfg.Region = "us-east-1"

	return &AccountClients{
		Account:    acct,
		AccountID:  accountID,
		EC2:        ec2.NewFromConfig(cfg),
		CloudWatch: cloudwatch.NewFromConfig(cfg),
		Cost:       costexplorer.NewFromConfig(costCfg),
	}, nil
}

// LoadBillingCostClient returns a Cost Explorer client for the organization payer profile.
// Empty profile uses the default credential chain (IRSA / instance profile).
func LoadBillingCostClient(ctx context.Context, profile string) (*costexplorer.Client, error) {
	cfg, err := loadAWSConfig(ctx, "us-east-1", profile, nil)
	if err != nil {
		if profile != "" {
			return nil, fmt.Errorf("load billing profile %q: %w", profile, err)
		}
		return nil, fmt.Errorf("load billing default credentials: %w", err)
	}
	return costexplorer.NewFromConfig(cfg), nil
}

// LoadBillingCostClientWithRole returns a payer CE client using the default chain then AssumeRole.
func LoadBillingCostClientWithRole(ctx context.Context, roleARN string) (*costexplorer.Client, error) {
	roleARN = strings.TrimSpace(roleARN)
	if roleARN == "" {
		return nil, fmt.Errorf("billing role ARN is empty")
	}
	baseCfg, err := loadAWSConfig(ctx, "us-east-1", "", nil)
	if err != nil {
		return nil, fmt.Errorf("load default credentials for billing role: %w", err)
	}
	stsClient := sts.NewFromConfig(baseCfg)
	cfg := baseCfg.Copy()
	cfg.Credentials = stscreds.NewAssumeRoleProvider(stsClient, roleARN)
	cfg.Region = "us-east-1"
	return costexplorer.NewFromConfig(cfg), nil
}

// LoadBillingCostClientWithCredentials returns a payer CE client using static keys.
func LoadBillingCostClientWithCredentials(ctx context.Context, static *appconfig.StaticCredentials) (*costexplorer.Client, error) {
	if static == nil {
		return nil, fmt.Errorf("billing static credentials are nil")
	}
	cfg, err := loadAWSConfig(ctx, "us-east-1", "", static)
	if err != nil {
		return nil, fmt.Errorf("load billing credentials: %w", err)
	}
	return costexplorer.NewFromConfig(cfg), nil
}

// ProbeCallerIdentity verifies credentials with STS GetCallerIdentity (does not treat
// "client constructed" as success).
func ProbeCallerIdentity(ctx context.Context, static *appconfig.StaticCredentials) error {
	if static == nil {
		return fmt.Errorf("static credentials are nil")
	}
	cfg, err := loadAWSConfig(ctx, "us-east-1", "", static)
	if err != nil {
		return fmt.Errorf("load credentials for STS probe: %w", err)
	}
	stsClient := sts.NewFromConfig(cfg)
	_, err = stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return fmt.Errorf("sts GetCallerIdentity: %w", err)
	}
	return nil
}

func loadAWSConfig(ctx context.Context, region, profile string, static *appconfig.StaticCredentials) (aws.Config, error) {
	var opts []func(*config.LoadOptions) error
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}
	if static != nil {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				static.AccessKeyID,
				static.SecretAccessKey,
				static.SessionToken,
			),
		))
	}
	opts = append(opts, config.WithRegion(region))
	opts = append(opts, config.WithRetryer(func() aws.Retryer {
		return retry.NewStandard(func(o *retry.StandardOptions) {
			o.MaxAttempts = 8
		})
	}))
	return config.LoadDefaultConfig(ctx, opts...)
}
