package config

import (
	"os"
	"testing"
)

func TestResolveStaticCredentialsFromEnv(t *testing.T) {
	t.Setenv("TEST_AWS_SECRET", "secret-value")
	acct := Account{
		Name:               "test",
		AccessKeyID:        "AKIATEST",
		SecretAccessKeyEnv: "TEST_AWS_SECRET",
	}
	creds, err := acct.ResolveStaticCredentials()
	if err != nil {
		t.Fatal(err)
	}
	if creds == nil || creds.SecretAccessKey != "secret-value" {
		t.Fatalf("expected secret from env, got %+v", creds)
	}
}

func TestValidateAccountAuthRequiresMethod(t *testing.T) {
	err := validateAccountAuth(Account{Name: "orphan"})
	if err == nil {
		t.Fatal("expected error for account with no auth")
	}
}

func TestHasAuthMethodProfileOrKeys(t *testing.T) {
	prof := Account{Profile: "foo"}
	if !prof.HasAuthMethod() {
		t.Error("profile should count as auth")
	}
	acct := Account{AccessKeyID: "AKIA", SecretAccessKey: "x"}
	if !acct.HasAuthMethod() {
		t.Error("inline keys should count as auth")
	}
	roleOnly := Account{Name: "irsa", RoleARN: "arn:aws:iam::123456789012:role/OrgCostReadOnly"}
	if !roleOnly.HasAuthMethod() {
		t.Error("role_arn alone should count as auth (IRSA/default chain)")
	}
	if err := validateAccountAuth(roleOnly); err != nil {
		t.Fatalf("role_arn-only account should validate: %v", err)
	}
}

func TestResolveStaticCredentialsNilWhenNoKey(t *testing.T) {
	acct := Account{}
	creds, err := acct.ResolveStaticCredentials()
	if err != nil || creds != nil {
		t.Fatalf("expected nil,nil got %v %v", creds, err)
	}
}

func TestResolveStaticCredentialsMissingEnv(t *testing.T) {
	_ = os.Unsetenv("MISSING_SECRET")
	acct := Account{
		AccessKeyID:        "AKIA",
		SecretAccessKeyEnv: "MISSING_SECRET",
	}
	_, err := acct.ResolveStaticCredentials()
	if err == nil {
		t.Fatal("expected error for missing env")
	}
}
