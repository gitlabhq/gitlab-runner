//go:build !integration

package retry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger"
)

func TestRetry_Run(t *testing.T) {
	runErr := errors.New("runErr")

	tests := map[string]struct {
		calls       []error
		shouldRetry bool

		expectedErr error
	}{
		"no error should succeed": {
			calls:       []error{nil},
			shouldRetry: false,

			expectedErr: nil,
		},
		"one error succeed on second call": {
			calls:       []error{runErr, nil},
			shouldRetry: true,

			expectedErr: nil,
		},
		"on error should not retry": {
			calls:       []error{runErr},
			shouldRetry: false,

			expectedErr: runErr,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			m := &mockRetryable{}
			defer m.AssertExpectations(t)

			for _, e := range tt.calls {
				m.On("Run").Return(e).Once()
			}
			m.On("ShouldRetry", mock.Anything, mock.Anything).Return(tt.shouldRetry).Maybe()

			retry := New(m.Run).WithCheck(m.ShouldRetry)
			err := retry.Run()
			assert.Equal(t, tt.expectedErr, err)
		})
	}
}

func TestRunBackoff(t *testing.T) {
	sleepTime := 2 * time.Second
	runErr := errors.New("err")

	m := &mockRetryable{}
	defer m.AssertExpectations(t)

	m.On("Run").Return(runErr).Times(2)
	m.On("ShouldRetry", mock.Anything, mock.Anything).Return(true).Once()
	m.On("ShouldRetry", mock.Anything, mock.Anything).Return(false).Once()

	start := time.Now()

	err := New(m.Run).
		WithCheck(m.ShouldRetry).
		WithMaxTries(3).
		WithBackoff(sleepTime, sleepTime).
		Run()

	assert.True(t, time.Since(start) >= sleepTime)
	assert.Equal(t, runErr, err)
}

func TestRunOnceNoRetry(t *testing.T) {
	err := errors.New("err")

	m := &mockRetryable{}
	defer m.AssertExpectations(t)

	m.On("Run").Return(err).Once()
	m.On("ShouldRetry", mock.Anything, mock.Anything).Return(false).Once()

	assert.Equal(t, err, New(m.Run).WithCheck(m.ShouldRetry).Run())
}

func TestRetryableLogrusDecorator(t *testing.T) {
	err := errors.New("err")

	m := &mockRetryable{}
	defer m.AssertExpectations(t)

	m.On("Run").Return(err).Twice()
	m.On("ShouldRetry", mock.Anything, mock.Anything).Return(true).Once()
	m.On("ShouldRetry", mock.Anything, mock.Anything).Return(false).Once()

	logger, hook := test.NewNullLogger()
	r := New(m.Run).WithCheck(m.ShouldRetry).WithLogrus(logger.WithContext(context.Background()))

	assert.Equal(t, err, r.Run())
	assert.Len(t, hook.Entries, 1)
}

func TestRetryableBuildLoggerDecorator(t *testing.T) {
	err := errors.New("err")

	m := &mockRetryable{}
	defer m.AssertExpectations(t)

	m.On("Run").Return(err).Twice()
	m.On("ShouldRetry", mock.Anything, mock.Anything).Return(true).Once()
	m.On("ShouldRetry", mock.Anything, mock.Anything).Return(false).Once()

	logger, hook := test.NewNullLogger()
	buildLogger := buildlogger.New(nil, logger.WithContext(context.Background()))
	r := New(m.Run).WithCheck(m.ShouldRetry).WithBuildLog(&buildLogger)

	assert.Equal(t, err, r.Run())
	assert.Len(t, hook.Entries, 1)
}

func TestMaxTries(t *testing.T) {
	err := errors.New("err")

	m := &mockRetryable{}
	defer m.AssertExpectations(t)

	m.On("Run").Return(err).Times(6)
	m.On("ShouldRetry", mock.Anything, mock.Anything).Return(true).Times(5)
	m.On("ShouldRetry", mock.Anything, mock.Anything).Return(false).Once()

	assert.Equal(t, err, New(m.Run).WithBackoff(0, 0).WithCheck(m.ShouldRetry).WithMaxTries(6).Run())
}
