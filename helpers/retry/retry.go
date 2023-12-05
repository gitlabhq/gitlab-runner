package retry

import (
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
//
//go:generate mockery --name=retryable --inpackage
type retryable interface {
	Run() error
	RunValue() (any, error)
	ShouldRetry(tries int, err error) bool
}

func (r RunFunc) ToValueFunc() RunValueFunc[any] {
	return func() (any, error) {
		return nil, r()
	}
}

type Retry[T any] struct {
	value   T
	run     RunValueFunc[T]
	check   CheckFunc
	backoff *backoff.Backoff
}

func New(run RunFunc) *Retry[any] {
	return NewWithValue(func() (any, error) {
		return nil, run()
	})
}

func NewWithValue[T any](run RunValueFunc[T]) *Retry[T] {
	return &Retry[T]{
		run: run,
		check: func(_ int, _ error) bool {
			return true
		},
		backoff: &backoff.Backoff{Min: defaultRetryMinBackoff, Max: defaultRetryMaxBackoff},
	}
}

func (r *Retry[T]) wrapCheck(newCheck checkFuncWithPrevious) *Retry[T] {
	originalCheck := r.check
	return r.WithCheck(func(tries int, err error) bool {
		shouldRetry := false
		if originalCheck != nil {
			shouldRetry = originalCheck(tries, err)
		}

		return newCheck(tries, err, shouldRetry)
	})
}

func (r *Retry[T]) WithCheck(check CheckFunc) *Retry[T] {
	r.check = check
	return r
}

func (r *Retry[T]) WithMaxTries(max int) *Retry[T] {
	return r.wrapCheck(func(tries int, err error, shouldRetry bool) bool {
		if tries >= max {
			return false
		}

		return shouldRetry
	})
}

func (r *Retry[T]) WithBackoff(min, max time.Duration) *Retry[T] {
	r.backoff = &backoff.Backoff{Min: min, Max: max}
	return r
}

func (r *Retry[T]) WithLogrus(log *logrus.Entry) *Retry[T] {
	return r.wrapCheck(func(tries int, err error, shouldRetry bool) bool {
		if shouldRetry {
			log.WithError(err).Warningln("Retrying...")
		}

		return shouldRetry
	})
}

func (r *Retry[T]) WithStdout() *Retry[T] {
	return r.wrapCheck(func(tries int, err error, shouldRetry bool) bool {
		if shouldRetry {
			fmt.Println("Retrying...")
		}

		return shouldRetry
	})
}

func (r *Retry[T]) WithBuildLog(log *buildlogger.Logger) *Retry[T] {
	return r.wrapCheck(func(tries int, err error, shouldRetry bool) bool {
		if shouldRetry {
			logger := log.WithFields(logrus.Fields{logrus.ErrorKey: err})
			logger.Warningln("Retrying...")
		}

		return shouldRetry
	})
}

func (r *Retry[T]) Run() error {
	_, err := r.RunValue()
	return err
}

func (r *Retry[T]) RunValue() (T, error) {
	var err error
	var tries int
	var value T
	for {
		tries++
		value, err = r.run()
		if err == nil || !r.check(tries, err) {
			break
		}

		time.Sleep(r.backoff.Duration())
	}

	return value, err
}
