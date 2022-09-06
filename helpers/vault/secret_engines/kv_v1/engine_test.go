//go:build !integration

package kv_v1

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault"
)

func TestEngine_EngineName(t *testing.T) {
	e := new(engine)
	assert.Equal(t, engineName, e.EngineName())
}

func TestEngine_Get(t *testing.T) {
	enginePath := "engine/"
	path := "/secret/"
	expectedPath := "engine/secret"
	expectedData := map[string]interface{}{
		"test": "testData",
	}

	tests := map[string]struct {
		setupClientMock func(*testing.T, *vault.MockClient) func()
		expectedError   error
		expectedData    map[string]interface{}
	}{
		"client read error": {
			setupClientMock: func(t *testing.T, c *vault.MockClient) func() {
				c.On("Read", expectedPath).
					Return(nil, assert.AnError).
					Once()

				return func() {
					c.AssertExpectations(t)
				}
			},
			expectedError: assert.AnError,
		},
		"client read succeeded with no data": {
			setupClientMock: func(t *testing.T, c *vault.MockClient) func() {
				result := new(vault.MockResult)
				result.On("Data").
					Return(nil).
					Once()

				c.On("Read", expectedPath).
					Return(result, nil).
					Once()

				return func() {
					c.AssertExpectations(t)
					result.AssertExpectations(t)
				}
			},
			expectedData: nil,
		},
		"client read succeeded with data": {
			setupClientMock: func(t *testing.T, c *vault.MockClient) func() {
				result := new(vault.MockResult)
				result.On("Data").
					Return(expectedData).
					Once()

				c.On("Read", expectedPath).
					Return(result, nil).
					Once()

				return func() {
					c.AssertExpectations(t)
					result.AssertExpectations(t)
				}
			},
			expectedData: expectedData,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			clientMock := new(vault.MockClient)
			assertions := tt.setupClientMock(t, clientMock)
			defer assertions()

			e := NewEngine(clientMock, enginePath)
			result, err := e.Get(path)
			if tt.expectedError != nil {
				assert.ErrorAs(t, err, &tt.expectedError)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedData, result)
		})
	}
}

func TestEngine_Put(t *testing.T) {
	enginePath := "engine/"
	path := "/secret/"
	expectedPath := "engine/secret"
	data := map[string]interface{}{
		"test": "testData",
	}

	tests := map[string]struct {
		setupClientMock func(*testing.T, *vault.MockClient) func()
		expectedError   error
	}{
		"client write error": {
			setupClientMock: func(t *testing.T, c *vault.MockClient) func() {
				c.On("Write", expectedPath, data).
					Return(nil, assert.AnError).
					Once()

				return func() {
					c.AssertExpectations(t)
				}
			},
			expectedError: assert.AnError,
		},
		"client write succeeded": {
			setupClientMock: func(t *testing.T, c *vault.MockClient) func() {
				c.On("Write", expectedPath, data).
					Return(nil, nil).
					Once()

				return func() {
					c.AssertExpectations(t)
				}
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			clientMock := new(vault.MockClient)
			assertions := tt.setupClientMock(t, clientMock)
			defer assertions()

			e := NewEngine(clientMock, enginePath)
			err := e.Put(path, data)
			if tt.expectedError != nil {
				assert.ErrorAs(t, err, &tt.expectedError)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestEngine_Delete(t *testing.T) {
	enginePath := "engine/"
	path := "/secret/"
	expectedPath := "engine/secret"

	tests := map[string]struct {
		setupClientMock func(*testing.T, *vault.MockClient) func()
		expectedError   error
	}{
		"client delete error": {
			setupClientMock: func(t *testing.T, c *vault.MockClient) func() {
				c.On("Delete", expectedPath).
					Return(assert.AnError).
					Once()

				return func() {
					c.AssertExpectations(t)
				}
			},
			expectedError: assert.AnError,
		},
		"client delete succeeded": {
			setupClientMock: func(t *testing.T, c *vault.MockClient) func() {
				c.On("Delete", expectedPath).
					Return(nil).
					Once()

				return func() {
					c.AssertExpectations(t)
				}
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			clientMock := new(vault.MockClient)
			assertions := tt.setupClientMock(t, clientMock)
			defer assertions()

			e := NewEngine(clientMock, enginePath)
			err := e.Delete(path)
			if tt.expectedError != nil {
				assert.ErrorAs(t, err, &tt.expectedError)
				return
			}
			assert.NoError(t, err)
		})
	}
}
