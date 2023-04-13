//go:build !integration

package autoscaler

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gitlab.com/gitlab-org/fleeting/nesting/api"
	"gitlab.com/gitlab-org/fleeting/nesting/api/mocks"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestWithInit(t *testing.T) {
	cases := []struct {
		name            string
		expectCallCount int
		testCall        *testCall
		expectInit      bool
		initError       error
		expectErr       bool
	}{{
		name:            "first call succeeds",
		expectCallCount: 1,
		testCall:        &testCall{},
		expectInit:      false,
		expectErr:       false,
	}, {
		name:            "first call fails with uninitialized",
		expectCallCount: 2,
		testCall: &testCall{
			firstCallError: api.ErrNotInitialized,
		},
		expectInit: true,
		expectErr:  false,
	}, {
		name:            "first call fails with unrelated error",
		expectCallCount: 1,
		testCall: &testCall{
			firstCallError: fmt.Errorf("no can do"),
		},
		expectInit: false,
		expectErr:  true,
	}, {
		name:            "second call fails",
		expectCallCount: 2,
		testCall: &testCall{
			firstCallError:  api.ErrNotInitialized,
			secondCallError: fmt.Errorf("no can do"),
		},
		expectInit: true,
		expectErr:  true,
	}, {
		name:            "initialization fails",
		expectCallCount: 1,
		testCall: &testCall{
			firstCallError: api.ErrNotInitialized,
		},
		initError:  fmt.Errorf("no can do"),
		expectInit: true,
		expectErr:  true,
	}, {
		name:            "already initialized (race between jobs)",
		expectCallCount: 2,
		testCall: &testCall{
			firstCallError: api.ErrNotInitialized,
		},
		initError:  api.ErrAlreadyInitialized,
		expectInit: true,
		expectErr:  false,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.TODO()
			config := &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Autoscaler: &common.AutoscalerConfig{},
				},
			}

			nc := mocks.NewClient(t)
			if tc.expectInit {
				nc.EXPECT().Init(ctx, mock.Anything).Return(tc.initError)
			}

			err := withInit(ctx, config, nc, tc.testCall.call())

			assert.Equal(t, tc.expectCallCount, tc.testCall.callCount)
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

type testCall struct {
	callCount       int
	firstCallError  error
	secondCallError error
}

func (tc *testCall) call() func() error {
	return func() error {
		tc.callCount++
		switch tc.callCount {
		case 1:
			return tc.firstCallError
		case 2:
			return tc.secondCallError
		default:
			return nil
		}
	}
}
