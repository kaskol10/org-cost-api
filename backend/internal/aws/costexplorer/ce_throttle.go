package costexplorer

import (
	"context"
	"sync/atomic"
)

// Cost Explorer is rate-limited and billed per request.
// We cap concurrent CE calls across all accounts to avoid throttling bursts.
const maxConcurrentCostExplorerCalls = 3

var ceSem = make(chan struct{}, maxConcurrentCostExplorerCalls)

// ceCallCount measures actual CE API calls (per GetCostAndUsage group-by)
// instead of hand-waved per-endpoint estimates.
var ceCallCount atomic.Int64

func withCESem(ctx context.Context) error {
	select {
	case ceSem <- struct{}{}:
		ceCallCount.Add(1)
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

// CECallCount returns the total number of CE API calls made since process start.
func CECallCount() int64 {
	return ceCallCount.Load()
}
