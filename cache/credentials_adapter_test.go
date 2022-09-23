//go:build !integration

package cache

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type credentialsFactoryTestCase struct {
	adapter          CredentialsAdapter
	errorOnFactorize error
	expectedError    string
	expectedAdapter  CredentialsAdapter
}

func prepareMockedCredentialsFactoriesMap() func() {
	oldFactories := credentialsFactories
	credentialsFactories = &CredentialsFactoriesMap{}

	return func() {
		credentialsFactories = oldFactories
	}
}

func makeTestCredentialsFactory(test credentialsFactoryTestCase) CredentialsFactory {
	return func(config *common.CacheConfig) (CredentialsAdapter, error) {
		if test.errorOnFactorize != nil {
			return nil, test.errorOnFactorize
		}

		return test.adapter, nil
	}
}

func TestCreateCredentialsAdapter(t *testing.T) {
	adapterMock := new(MockCredentialsAdapter)

	tests := map[string]credentialsFactoryTestCase{
		"adapter doesn't exist": {
			adapter:          nil,
			errorOnFactorize: nil,
			expectedAdapter:  nil,
			//nolint:lll
			expectedError: `credentials adapter factory not found: factory for credentials adapter "test" not registered`,
		},
		"adapter exists": {
			adapter:          adapterMock,
			errorOnFactorize: nil,
			expectedAdapter:  adapterMock,
			expectedError:    "",
		},
		"adapter errors on factorize": {
			adapter:          adapterMock,
			errorOnFactorize: errors.New("test error"),
			expectedAdapter:  nil,
			expectedError:    `credentials adapter could not be initialized: test error`,
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			cleanupFactoriesMap := prepareMockedCredentialsFactoriesMap()
			defer cleanupFactoriesMap()

			adapterTypeName := "test"

			if tc.adapter != nil {
				err := credentialsFactories.Register(adapterTypeName, makeTestCredentialsFactory(tc))
				assert.NoError(t, err)
			}

			_ = credentialsFactories.Register(
				"additional-adapter",
				func(config *common.CacheConfig) (CredentialsAdapter, error) {
					return new(MockCredentialsAdapter), nil
				})

			config := &common.CacheConfig{
				Type: adapterTypeName,
			}

			adapter, err := CreateCredentialsAdapter(config)

			if tc.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.expectedError)
			}

			assert.Equal(t, tc.expectedAdapter, adapter)
		})
	}
}

func TestCredentialsFactoryDoubledRegistration(t *testing.T) {
	adapterTypeName := "test"
	fakeFactory := func(config *common.CacheConfig) (CredentialsAdapter, error) {
		return nil, nil
	}

	f := &CredentialsFactoriesMap{}

	err := f.Register(adapterTypeName, fakeFactory)
	assert.NoError(t, err)
	assert.Len(t, f.internal, 1)

	err = f.Register(adapterTypeName, fakeFactory)
	assert.Error(t, err)
	assert.Len(t, f.internal, 1)
}
