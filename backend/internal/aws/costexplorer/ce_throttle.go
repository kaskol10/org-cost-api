package costexplorer

import (
	"context"
)

// Cost Explorer is rate-limited and billed per request.
// We cap concurrent CE calls across all accounts to avoid throttling bursts.
const maxConcurrentCostExplorerCalls = 3

var ceSem = make(chan struct{}, maxConcurrentCostExplorerCalls)

func withCESem(ctx context.Context) error {
	select {
	case ceSem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func releaseCESem() {
	select {
	case <-ceSem:
	default:
	}
}

