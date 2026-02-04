package retry

import (
	"context"
	"fmt"
	"time"

	"github.com/jpillora/backoff"
	"github.com/sirupsen/logrus"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger"
)

const (
	defaultRetryMinBackoff = 1 * time.Second
	defaultRetryMaxBackoff = 5 * time.Second
)

type RunFunc func() error
type RunValueFunc[T any] func() (T, error)
type CheckFunc func(tries int, err error) bool
type checkFuncWithPrevious func(tries int, err error, shouldRetry bool) bool

// used only in tests to mock the run and check functions
type retryable interface {
	Run() error
	ShouldRetry(tries int, err error) bool
}

// used only in tests to mock the run and check functions
type valueRetryable[T any] interface {
	Run() (T, error)
	ShouldRetry(tries int, err error) bool
}

type Provider interface {
	NewRetry() *Retry
}

func (r RunFunc) toValueFunc() RunValueFunc[any] {
	return func() (any, error) {
		return nil, r()
	}
}

type Retry struct {
	run     RunFunc
	check   CheckFunc
	backoff *backoff.Backoff
	ctx     context.Context
}

type NoValueRetry struct {
	retry *Retry
	value any
	run   RunFunc
}

type ValueRetry[T any] struct {
	retry *Retry
	value T
	run   RunValueFunc[T]
}

func NewNoValue(retry *Retry, run RunFunc) *NoValueRetry {
	return &NoValueRetry{
		retry: retry,
		run:   run,
	}
}

func NewValue[T any](retry *Retry, run RunValueFunc[T]) *ValueRetry[T] {
	return &ValueRetry[T]{
		retry: retry,
		run:   run,
	}
}

func WithValueFn[T any](p Provider, run RunValueFunc[T]) *ValueRetry[T] {
	return NewValue[T](p.NewRetry(), run)
}

func WithFn(p Provider, run RunFunc) *NoValueRetry {
	return NewNoValue(p.NewRetry(), run)
}

func New() *Retry {
	return &Retry{
		check: func(_ int, _ error) bool {
			return true
		},
		backoff: &backoff.Backoff{
			Min: defaultRetryMinBackoff,
			Max: defaultRetryMaxBackoff,
		},
		ctx: context.Background(),
	}
}

func (r *Retry) WithContext(ctx context.Context) *Retry {
	if ctx != nil {
		r.ctx = ctx
	}
	return r
}

func (r *Retry) wrapCheck(newCheck checkFuncWithPrevious) *Retry {
	originalCheck := r.check
	return r.WithCheck(func(tries int, err error) bool {
		shouldRetry := false
		if originalCheck != nil {
			shouldRetry = originalCheck(tries, err)
		}

		return newCheck(tries, err, shouldRetry)
	})
}

func (r *Retry) WithCheck(check CheckFunc) *Retry {
	r.check = check
	return r
}

func (r *Retry) WithMaxTries(max int) *Retry {
	return r.WithMaxTriesFunc(func(_ error) int {
		return max
	})
}

func (r *Retry) WithMaxTriesFunc(maxTriesFunc func(err error) int) *Retry {
	return r.wrapCheck(func(tries int, err error, shouldRetry bool) bool {
		maxTries := maxTriesFunc(err)
		if tries >= maxTries {
			return false
		}

		return shouldRetry
	})
}

func (r *Retry) WithBackoff(min, max time.Duration) *Retry {
	r.backoff = &backoff.Backoff{Min: min, Max: max}
	return r
}

func (r *Retry) WithLogrus(log *logrus.Entry) *Retry {
	return r.wrapCheck(func(tries int, err error, shouldRetry bool) bool {
		if shouldRetry {
			log.WithError(err).Warningln("Retrying...")
		}

		return shouldRetry
	})
}

func (r *Retry) WithStdout() *Retry {
	return r.wrapCheck(func(tries int, err error, shouldRetry bool) bool {
		if shouldRetry {
			fmt.Println("Retrying...")
		}

		return shouldRetry
	})
}

func (r *Retry) WithBuildLog(log *buildlogger.Logger) *Retry {
	return r.wrapCheck(func(tries int, err error, shouldRetry bool) bool {
		if shouldRetry {
			logger := log.WithFields(logrus.Fields{logrus.ErrorKey: err})
			logger.Warningln("Retrying...")
		}

		return shouldRetry
	})
}

func retryRun[T any](retry *Retry, fn RunValueFunc[T]) (T, error) {
	var err error
	var tries int
	var value T

	select {
	case <-retry.ctx.Done():
		return value, retry.ctx.Err()
	default:
	}

	for {
		tries++

		value, err = fn()
		if err == nil || !retry.check(tries, err) {
			break
		}

		backoffDuration := retry.backoff.Duration()

		select {
		case <-time.After(backoffDuration):
		case <-retry.ctx.Done():
			return value, retry.ctx.Err()
		}
	}

	return value, err
}

func (r *NoValueRetry) Run() error {
	_, err := retryRun(r.retry, r.run.toValueFunc())
	return err
}

func (r *ValueRetry[T]) Run() (T, error) {
	return retryRun(r.retry, r.run)
}
