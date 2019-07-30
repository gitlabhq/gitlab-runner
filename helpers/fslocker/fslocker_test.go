package fslocker

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockFn struct {
	mock.Mock
}

func (l *mockFn) Run() {
	l.Called()
}

func mockDefaultLocker(t *testing.T, expectedPath string) (*mockLocker, func()) {
	fsLockerMock := new(mockLocker)

	oldLocker := defaultLocker
	cleanup := func() {
		defaultLocker = oldLocker
		fsLockerMock.AssertExpectations(t)
	}

	defaultLocker = func(path string) locker {
		assert.Equal(t, expectedPath, path)

		return fsLockerMock
	}

	return fsLockerMock, cleanup
}

func TestInLock(t *testing.T) {
	filePath := "/some/path/to/config/file"
	testError := errors.New("test-error")

	tests := map[string]struct {
		fileLocker            locker
		prepareMockAssertions func(locker *mockLocker, fn *mockFn)
		expectedError         error
	}{
		"error on file locking": {
			prepareMockAssertions: func(locker *mockLocker, fn *mockFn) {
				locker.On("TryLock").
					Return(true, testError).
					Once()
			},
			expectedError: errCantAcquireLock(testError, filePath),
		},
		"can't lock the file": {
			prepareMockAssertions: func(locker *mockLocker, fn *mockFn) {
				locker.On("TryLock").
					Return(false, nil).
					Once()

			},
			expectedError: errFileInUse(filePath),
		},
		"file locked properly, fails on unlock": {
			prepareMockAssertions: func(locker *mockLocker, fn *mockFn) {
				locker.On("TryLock").
					Return(true, nil).
					Once()
				locker.On("Unlock").
					Return(testError).
					Once()

				fn.On("Run").Once()
			},
			expectedError: errCantReleaseLock(testError, filePath),
		},
		"file locked and unlocked properly": {
			prepareMockAssertions: func(locker *mockLocker, fn *mockFn) {
				locker.On("TryLock").
					Return(true, nil).
					Once()
				locker.On("Unlock").
					Return(nil).
					Once()

				fn.On("Run").Once()
			},
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			fnMock := new(mockFn)
			defer fnMock.AssertExpectations(t)

			fsLockerMock, cleanup := mockDefaultLocker(t, filePath)
			defer cleanup()

			testCase.prepareMockAssertions(fsLockerMock, fnMock)

			err := InLock(filePath, fnMock.Run)

			if testCase.expectedError == nil {
				assert.NoError(t, err)
				return
			}

			assert.EqualError(t, err, testCase.expectedError.Error())
		})
	}
}
