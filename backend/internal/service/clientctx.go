package service

import "context"

type ctxKey int

const (
	clientKey ctxKey = iota
	refreshInfoKey
)

// AnonymousClient is the client name used when API auth is disabled.
const AnonymousClient = "anonymous"

// WithClient tags ctx with the authenticated API client name.
func WithClient(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, clientKey, name)
}

// ClientFromContext returns the API client name, or AnonymousClient.
func ClientFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(clientKey).(string); ok && v != "" {
		return v
	}
	return AnonymousClient
}

// RefreshInfo reports what happened to a forced refresh during a request.
type RefreshInfo struct {
	// Coalesced is true when refresh=1 was answered with data another client
	// force-refreshed moments ago, instead of a new Cost Explorer fetch.
	Coalesced bool
}

// WithRefreshInfo attaches a RefreshInfo the service fills in for this request.
func WithRefreshInfo(ctx context.Context) (context.Context, *RefreshInfo) {
	info := &RefreshInfo{}
	return context.WithValue(ctx, refreshInfoKey, info), info
}

func refreshInfoFrom(ctx context.Context) *RefreshInfo {
	if v, ok := ctx.Value(refreshInfoKey).(*RefreshInfo); ok {
		return v
	}
	return nil
}
