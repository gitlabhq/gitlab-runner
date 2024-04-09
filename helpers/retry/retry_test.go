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

			err := NewNoValue(New().WithCheck(m.ShouldRetry), m.Run).Run()
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

	err := NewNoValue(New().
		WithCheck(m.ShouldRetry).
		WithMaxTries(3).
		WithBackoff(sleepTime, sleepTime),
		m.Run,
	).Run()

	assert.True(t, time.Since(start) >= sleepTime)
	assert.Equal(t, runErr, err)
}

func TestRunOnceNoRetry(t *testing.T) {
	err := errors.New("err")

	m := &mockRetryable{}
	defer m.AssertExpectations(t)

	m.On("Run").Return(err).Once()
	m.On("ShouldRetry", mock.Anything, mock.Anything).Return(false).Once()

	assert.Equal(t, err, NewNoValue(New().WithCheck(m.ShouldRetry), m.Run).Run())
}

func TestRetryableLogrusDecorator(t *testing.T) {
	err := errors.New("err")

	m := &mockRetryable{}
	defer m.AssertExpectations(t)

	m.On("Run").Return(err).Twice()
	m.On("ShouldRetry", mock.Anything, mock.Anything).Return(true).Once()
	m.On("ShouldRetry", mock.Anything, mock.Anything).Return(false).Once()

	logger, hook := test.NewNullLogger()
	r := NewNoValue(
		New().WithCheck(m.ShouldRetry).WithLogrus(logger.WithContext(context.Background())),
		m.Run,
	)

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
	buildLogger := buildlogger.New(nil, logger.WithContext(context.Background()), buildlogger.Options{})
	r := NewNoValue(
		New().WithCheck(m.ShouldRetry).WithBuildLog(&buildLogger),
		m.Run,
	)

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

	r := NewNoValue(
		New().WithBackoff(0, 0).WithCheck(m.ShouldRetry).WithMaxTries(6),
		m.Run,
	)
	assert.Equal(t, err, r.Run())
}

func TestMaxTriesFunc(t *testing.T) {
	err := errors.New("err")

	m := &mockRetryable{}
	defer m.AssertExpectations(t)

	m.On("Run").Return(err).Times(6)
	m.On("ShouldRetry", mock.Anything, mock.Anything).Return(true).Times(5)
	m.On("ShouldRetry", mock.Anything, mock.Anything).Return(false).Once()

	r := NewNoValue(
		New().WithBackoff(0, 0).WithCheck(m.ShouldRetry).WithMaxTriesFunc(func(err error) int {
			return 6
		}),
		m.Run,
	)
	assert.Equal(t, err, r.Run())
}

func TestRunValue(t *testing.T) {
	m := &mockValueRetryable[int]{}
	defer m.AssertExpectations(t)

	m.On("Run").Return(1, errors.New("err")).Times(5)
	m.On("ShouldRetry", mock.Anything, mock.Anything).Return(true).Times(5)
	m.On("Run").Return(5, nil).Once()

	v, err := NewValue(
		New().WithBackoff(0, 0).WithCheck(m.ShouldRetry).WithMaxTries(6),
		m.Run,
	).Run()
	assert.Nil(t, err)
	assert.Equal(t, 5, v)
}
