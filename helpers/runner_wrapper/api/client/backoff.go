package client

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v4"
)

var (
	ErrRetryTimeoutExceeded = errors.New("retry timeout exceeded")
)

func RetryWithBackoff(ctx context.Context, timeout time.Duration, fn func() error) error {
	b := backoff.NewExponentialBackOff()

	var err error

	cctx, cancelFn := context.WithDeadlineCause(
		ctx,
		time.Now().Add(timeout),
		fmt.Errorf("%w: %s", ErrRetryTimeoutExceeded, timeout),
	)
	defer cancelFn()

	for {
		err = fn()
		if err == nil {
			return nil
		}

		timer := time.NewTimer(b.NextBackOff())

		select {
		case <-cctx.Done():
			timer.Stop()
			return cctx.Err()
		case <-timer.C:
			// continue retrying
		}
	}
}

