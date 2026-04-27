package network

import (
	"context"
	"sync/atomic"
)

type retryTrackerKey struct{}

// WithRetryTracker returns a derived context carrying an atomic flag set by
// the network retry layer whenever a request under that context observes a
// retriable response (429, 5xx, etc.) — including a final attempt that
// returns the retriable response after exhausting maxAttempts.
//
// A set flag means the server pushed back at least once during the request,
// so the eventual outcome is not evidence of server capacity.
func WithRetryTracker(ctx context.Context) (context.Context, *atomic.Bool) {
	flag := &atomic.Bool{}
	return context.WithValue(ctx, retryTrackerKey{}, flag), flag
}

func retryTrackerFromContext(ctx context.Context) *atomic.Bool {
	f, _ := ctx.Value(retryTrackerKey{}).(*atomic.Bool)
	return f
}
