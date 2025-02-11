package client

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v3"
)

var (
	ErrRetryTimeoutExceeded = errors.New("retry timeout exceeded")
)

func RetryWithBackoff(ctx context.Context, timeout time.Duration, fn func() error) error {
	b := backoff.NewExponentialBackOff()

	var err error

	cctx, cancelFn := context.WithDeadlineCause(ctx, time.Now().Add(timeout), fmt.Errorf("%w: %s", ErrRetryTimeoutExceeded, timeout))
	defer cancelFn()

	for {
		err = fn()
		if err == nil {
			return nil
		}

		select {
		case <-cctx.Done():
			return ctx.Err()
		case <-time.After(b.NextBackOff()):
		}
	}
}
