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

	"gitlab.com/gitlab-org/gitlab-runner/common"
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
			mockRetryable := &MockRetryable{}
			defer mockRetryable.AssertExpectations(t)

			for _, e := range tt.calls {
				mockRetryable.On("Run").Return(e).Once()
			}
			mockRetryable.On("ShouldRetry", mock.Anything, mock.Anything).Return(tt.shouldRetry).Maybe()

			retry := New(mockRetryable)
			err := retry.Run()
			assert.Equal(t, tt.expectedErr, err)
		})
	}
}

func TestRunBackoff(t *testing.T) {
	sleepTime := 2 * time.Second
	runErr := errors.New("err")

	mockRetryable := &MockRetryable{}
	defer mockRetryable.AssertExpectations(t)

	mockRetryable.On("Run").Return(runErr).Times(2)
	mockRetryable.On("ShouldRetry", mock.Anything, mock.Anything).Return(true).Once()
	mockRetryable.On("ShouldRetry", mock.Anything, mock.Anything).Return(false).Once()

	retry := New(mockRetryable)
	retry.backoff.Min = sleepTime
	retry.backoff.Max = sleepTime

	start := time.Now()
	err := retry.Run()
	assert.True(t, time.Since(start) >= sleepTime)
	assert.Equal(t, runErr, err)
}

func TestRetryable(t *testing.T) {
	err := errors.New("err")
	var runCalled, shouldRetryCalled bool
	r := newRetryableDecorator(func() error {
		runCalled = true
		return err
	}, func(tries int, err error) bool {
		shouldRetryCalled = true
		return true
	})

	assert.Equal(t, err, r.Run())
	assert.True(t, r.ShouldRetry(0, nil))
	assert.True(t, runCalled)
	assert.True(t, shouldRetryCalled)
}

func TestRetryableLogrusDecorator(t *testing.T) {
	err := errors.New("err")

	mr := &MockRetryable{}
	defer mr.AssertExpectations(t)
	mr.On("Run").Return(err).Once()
	mr.On("ShouldRetry", mock.Anything, mock.Anything).Return(true).Once()

	logger, hook := test.NewNullLogger()
	r := WithLogrus(mr, logger.WithContext(context.Background()))

	assert.Equal(t, err, r.Run())
	assert.Equal(t, true, r.ShouldRetry(0, nil))
	assert.Len(t, hook.Entries, 1)
}

func TestRetryableBuildLoggerDecorator(t *testing.T) {
	err := errors.New("err")

	mr := &MockRetryable{}
	defer mr.AssertExpectations(t)
	mr.On("Run").Return(err).Once()
	mr.On("ShouldRetry", mock.Anything, mock.Anything).Return(true).Once()

	logger, hook := test.NewNullLogger()
	buildLogger := common.NewBuildLogger(nil, logger.WithContext(context.Background()))
	r := WithBuildLog(mr, &buildLogger)

	assert.Equal(t, err, r.Run())
	assert.Equal(t, true, r.ShouldRetry(0, nil))
	assert.Len(t, hook.Entries, 1)
}
