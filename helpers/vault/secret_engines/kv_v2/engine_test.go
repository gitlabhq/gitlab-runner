//go:build !integration

package kv_v2

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
	expectedPath := "engine/data/secret"
	missingData := map[string]interface{}{
		"test": "test",
	}
	expectedData := map[string]interface{}{
		"test": "testData",
	}
	data := map[string]interface{}{
		"test": "test",
		"data": expectedData,
	}

	tests := map[string]struct {
		setupClientMock func(*testing.T, *vault.MockClient)
		expectedError   error
		expectedData    map[string]interface{}
	}{
		"client read error": {
			setupClientMock: func(t *testing.T, c *vault.MockClient) {
				c.On("Read", expectedPath).
					Return(nil, assert.AnError).
					Once()
			},
			expectedError: assert.AnError,
		},
		"client read succeeded with nil result": {
			setupClientMock: func(t *testing.T, c *vault.MockClient) {
				c.On("Read", expectedPath).
					Return(nil, nil).
					Once()
			},
			expectedData: nil,
		},
		"client read succeeded with no data": {
			setupClientMock: func(t *testing.T, c *vault.MockClient) {
				result := vault.NewMockResult(t)
				result.On("Data").
					Return(nil).
					Once()

				c.On("Read", expectedPath).
					Return(result, nil).
					Once()
			},
			expectedData: nil,
		},
		"client read succeeded with nil data": {
			setupClientMock: func(t *testing.T, c *vault.MockClient) {
				nilData := map[string]interface{}{
					"test": "test",
					"data": nil,
				}
				result := vault.NewMockResult(t)
				result.On("Data").
					Return(nilData).
					Once()

				c.On("Read", expectedPath).
					Return(result, nil).
					Once()
			},
			expectedData: nil,
		},
		"client read succeeded with bogus data": {
			setupClientMock: func(t *testing.T, c *vault.MockClient) {
				nilData := map[string]interface{}{
					"test": "test",
					"data": "sdfhgskldfhkljshdfljkgh",
				}
				result := vault.NewMockResult(t)
				result.On("Data").
					Return(nilData).
					Once()

				c.On("Read", expectedPath).
					Return(result, nil).
					Once()
			},
			expectedData:  nil,
			expectedError: assert.AnError,
		},
		"client read succeeded with missing data key": {
			setupClientMock: func(t *testing.T, c *vault.MockClient) {
				result := vault.NewMockResult(t)
				result.On("Data").
					Return(missingData).
					Once()

				c.On("Read", expectedPath).
					Return(result, nil).
					Once()
			},
		},
		"client read succeeded with data": {
			setupClientMock: func(t *testing.T, c *vault.MockClient) {
				result := vault.NewMockResult(t)
				result.On("Data").
					Return(data).
					Once()

				c.On("Read", expectedPath).
					Return(result, nil).
					Once()
			},
			expectedData: expectedData,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			clientMock := vault.NewMockClient(t)
			tt.setupClientMock(t, clientMock)

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
	expectedPath := "engine/data/secret"
	data := map[string]interface{}{
		"test": "testData",
	}
	expectedData := map[string]interface{}{
		"data": data,
	}

	tests := map[string]struct {
		setupClientMock func(*testing.T, *vault.MockClient)
		expectedError   error
	}{
		"client write error": {
			setupClientMock: func(t *testing.T, c *vault.MockClient) {
				c.On("Write", expectedPath, expectedData).
					Return(nil, assert.AnError).
					Once()
			},
			expectedError: assert.AnError,
		},
		"client write succeeded": {
			setupClientMock: func(t *testing.T, c *vault.MockClient) {
				c.On("Write", expectedPath, expectedData).
					Return(nil, nil).
					Once()
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			clientMock := vault.NewMockClient(t)
			tt.setupClientMock(t, clientMock)

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
	expectedPath := "engine/metadata/secret"

	tests := map[string]struct {
		setupClientMock func(*testing.T, *vault.MockClient)
		expectedError   error
	}{
		"client delete error": {
			setupClientMock: func(t *testing.T, c *vault.MockClient) {
				c.On("Delete", expectedPath).
					Return(assert.AnError).
					Once()
			},
			expectedError: assert.AnError,
		},
		"client delete succeeded": {
			setupClientMock: func(t *testing.T, c *vault.MockClient) {
				c.On("Delete", expectedPath).
					Return(nil).
					Once()
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			clientMock := vault.NewMockClient(t)
			tt.setupClientMock(t, clientMock)

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
