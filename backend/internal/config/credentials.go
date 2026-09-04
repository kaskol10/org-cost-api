package config

import (
	"fmt"
	"os"
	"strings"
)

// StaticCredentials holds long-lived AWS access keys (optional session token).
type StaticCredentials struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
}

// HasStaticCredentials reports whether inline or env-based keys are configured.
func (a Account) HasStaticCredentials() bool {
	return strings.TrimSpace(a.AccessKeyID) != "" &&
		(strings.TrimSpace(a.SecretAccessKey) != "" || strings.TrimSpace(a.SecretAccessKeyEnv) != "")
}

// ResolveStaticCredentials returns access keys from config or environment.
func (a Account) ResolveStaticCredentials() (*StaticCredentials, error) {
	return resolveStaticCredentials(
		a.AccessKeyID,
		a.SecretAccessKey,
		a.SecretAccessKeyEnv,
		a.SessionToken,
		a.SessionTokenEnv,
	)
}

// HasAuthMethod reports whether the account has any way to authenticate.
// RoleARN alone is valid when the host uses the default credential chain (IRSA, instance profile).
func (a Account) HasAuthMethod() bool {
	return strings.TrimSpace(a.Profile) != "" ||
		a.HasStaticCredentials() ||
		strings.TrimSpace(a.RoleARN) != ""
}

// ResolveBillingStaticCredentials returns payer access keys when configured.
func (c *Config) ResolveBillingStaticCredentials() (*StaticCredentials, error) {
	return resolveStaticCredentials(
		c.BillingAccessKeyID,
		c.BillingSecretAccessKey,
		c.BillingSecretAccessKeyEnv,
		c.BillingSessionToken,
		c.BillingSessionTokenEnv,
	)
}

func (c *Config) HasBillingStaticCredentials() bool {
	return strings.TrimSpace(c.BillingAccessKeyID) != "" &&
		(strings.TrimSpace(c.BillingSecretAccessKey) != "" || strings.TrimSpace(c.BillingSecretAccessKeyEnv) != "")
}

func resolveStaticCredentials(
	accessKeyID, secret, secretEnv, sessionToken, sessionTokenEnv string,
) (*StaticCredentials, error) {
	accessKeyID = strings.TrimSpace(accessKeyID)
	if accessKeyID == "" {
		return nil, nil
	}

	secretKey := strings.TrimSpace(secret)
	if secretKey == "" && strings.TrimSpace(secretEnv) != "" {
		secretKey = strings.TrimSpace(os.Getenv(secretEnv))
		if secretKey == "" {
			return nil, fmt.Errorf("environment variable %q is not set or empty", secretEnv)
		}
	}
	if secretKey == "" {
		return nil, fmt.Errorf("access_key_id is set but no secret_access_key or secret_access_key_env")
	}

	token := strings.TrimSpace(sessionToken)
	if token == "" && strings.TrimSpace(sessionTokenEnv) != "" {
		token = strings.TrimSpace(os.Getenv(sessionTokenEnv))
	}

	return &StaticCredentials{
		AccessKeyID:     accessKeyID,
		SecretAccessKey: secretKey,
		SessionToken:    token,
	}, nil
}

func validateAccountAuth(acct Account) error {
	if !acct.HasAuthMethod() {
		return fmt.Errorf(
			"account %q: set profile, role_arn (IRSA/default chain), or access_key_id + secret_access_key(_env)",
			acct.Name,
		)
	}
	if acct.HasStaticCredentials() {
		if _, err := acct.ResolveStaticCredentials(); err != nil {
			return fmt.Errorf("account %q: %w", acct.Name, err)
		}
	}
	return nil
}
