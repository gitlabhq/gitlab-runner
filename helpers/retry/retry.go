package retry

import (
	"time"

	"github.com/jpillora/backoff"
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

const (
	defaultRetryMinBackoff = 1 * time.Second
	defaultRetryMaxBackoff = 5 * time.Second
)

//go:generate mockery --name=Retryable --inpackage
type Retryable interface {
	Run() error
	ShouldRetry(tries int, err error) bool
}

type Retry struct {
	retryable Retryable
	backoff   *backoff.Backoff
}

func New(retry Retryable) *Retry {
	return &Retry{
		retryable: retry,
		backoff:   &backoff.Backoff{Min: defaultRetryMinBackoff, Max: defaultRetryMaxBackoff},
	}
}

func (r *Retry) Run() error {
	var err error
	var tries int
	for {
		tries++
		err = r.retryable.Run()
		if err == nil || !r.retryable.ShouldRetry(tries, err) {
			break
		}

		time.Sleep(r.backoff.Duration())
	}

	return err
}

func WithLogrus(retry Retryable, log *logrus.Entry) Retryable {
	return newRetryableDecorator(retry.Run, func(tries int, err error) bool {
		shouldRetry := retry.ShouldRetry(tries, err)
		if shouldRetry {
			log.WithError(err).Warningln("Retrying...")
		}

		return shouldRetry
	})
}

func WithBuildLog(retry Retryable, log *common.BuildLogger) Retryable {
	return newRetryableDecorator(retry.Run, func(tries int, err error) bool {
		shouldRetry := retry.ShouldRetry(tries, err)
		if shouldRetry {
			logger := log.WithFields(logrus.Fields{logrus.ErrorKey: err})
			logger.Warningln("Retrying...")
		}

		return shouldRetry
	})
}

type retryableDecorator struct {
	run         func() error
	shouldRetry func(tries int, err error) bool
}

func newRetryableDecorator(run func() error, shouldRetry func(tries int, err error) bool) *retryableDecorator {
	return &retryableDecorator{
		run:         run,
		shouldRetry: shouldRetry,
	}
}

func (d *retryableDecorator) Run() error {
	return d.run()
}

func (d *retryableDecorator) ShouldRetry(tries int, err error) bool {
	return d.shouldRetry(tries, err)
}
