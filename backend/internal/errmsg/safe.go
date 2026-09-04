package errmsg

import "strings"

// Code returns a stable machine-readable code for AWS or internal errors.
func Code(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "accessdenied"),
		strings.Contains(msg, "not authorized"),
		strings.Contains(msg, "is not authorized to perform"):
		return "access_denied"
	case strings.Contains(msg, "throttl"),
		strings.Contains(msg, "toomanyrequests"),
		strings.Contains(msg, "requestlimitexceeded"):
		return "rate_limited"
	case strings.Contains(msg, "expiredtoken"),
		strings.Contains(msg, "expired token"),
		strings.Contains(msg, "token has expired"):
		return "credentials_expired"
	case strings.Contains(msg, "invalidclienttokenid"),
		strings.Contains(msg, "invalidaccesskeyid"),
		strings.Contains(msg, "security token included in the request is invalid"):
		return "invalid_credentials"
	case strings.Contains(msg, "context canceled"),
		strings.Contains(msg, "context deadline exceeded"):
		return "timeout"
	default:
		return "data_unavailable"
	}
}

// AccountComponent formats a safe per-account dashboard error (e.g. "volumes: access_denied").
func AccountComponent(component string, err error) string {
	return component + ": " + Code(err)
}

// Note returns a user-safe note for optional enrichment fields (inventory_note, usage_note, etc.).
func Note(prefix string, err error) string {
	switch Code(err) {
	case "access_denied":
		return prefix + ": AWS permission denied"
	case "rate_limited":
		return prefix + ": AWS rate limited — retry later"
	case "credentials_expired":
		return prefix + ": AWS credentials expired"
	case "invalid_credentials":
		return prefix + ": AWS credentials invalid"
	case "timeout":
		return prefix + ": request timed out"
	default:
		return prefix + ": data temporarily unavailable"
	}
}

// CURNote sanitizes CUR inventory failure messages.
func CURNote(err error) string {
	return Note("CUR daily inventory unavailable", err)
}

// HistoryPriorNote sanitizes prior-period CE failure messages.
func HistoryPriorNote(err error) string {
	return Note("Prior period unavailable", err) + ". Run dashboard daily to build history snapshots."
}
