//go:build !integration

package cache

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

var defaultTimeout = 1 * time.Hour

type factorizeTestCase struct {
	adapter          Adapter
	errorOnFactorize error
	expectedError    string
	expectedAdapter  Adapter
}

func prepareMockedFactoriesMap() func() {
	oldFactories := factories
	factories = &FactoriesMap{}

	return func() {
		factories = oldFactories
	}
}

func makeTestFactory(test factorizeTestCase) Factory {
	return func(config *common.CacheConfig, timeout time.Duration, objectName string) (Adapter, error) {
		if test.errorOnFactorize != nil {
			return nil, test.errorOnFactorize
		}

		return test.adapter, nil
	}
}

func TestCreateAdapter(t *testing.T) {
	adapterMock := new(MockAdapter)

	tests := map[string]factorizeTestCase{
		"adapter doesn't exist": {
			adapter:          nil,
			errorOnFactorize: nil,
			expectedAdapter:  nil,
			expectedError:    `cache factory not found: factory for cache adapter \"test\" was not registered`,
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
			expectedError:    `cache adapter could not be initialized: test error`,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			cleanupFactoriesMap := prepareMockedFactoriesMap()
			defer cleanupFactoriesMap()

			adapterTypeName := "test"

			if test.adapter != nil {
				err := factories.Register(adapterTypeName, makeTestFactory(test))
				assert.NoError(t, err)
			}

			_ = factories.Register(
				"additional-adapter",
				func(config *common.CacheConfig, timeout time.Duration, objectName string) (Adapter, error) {
					return new(MockAdapter), nil
				})

			config := &common.CacheConfig{
				Type: adapterTypeName,
			}

			adapter, err := CreateAdapter(config, defaultTimeout, "key")

			if test.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}

			assert.Equal(t, test.expectedAdapter, adapter)
		})
	}
}

func TestDoubledRegistration(t *testing.T) {
	adapterTypeName := "test"
	fakeFactory := func(config *common.CacheConfig, timeout time.Duration, objectName string) (Adapter, error) {
		return nil, nil
	}

	f := &FactoriesMap{}

	err := f.Register(adapterTypeName, fakeFactory)
	assert.NoError(t, err)
	assert.Len(t, f.internal, 1)

	err = f.Register(adapterTypeName, fakeFactory)
	assert.Error(t, err)
	assert.Len(t, f.internal, 1)
}
