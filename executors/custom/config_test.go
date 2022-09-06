//go:build !integration

package custom

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/process"
)

type getDurationTestCase struct {
	source        *int
	expectedValue time.Duration
}

func testGetDuration(t *testing.T, defaultValue time.Duration, assert func(*testing.T, getDurationTestCase)) {
	tests := map[string]getDurationTestCase{
		"source undefined": {
			expectedValue: defaultValue,
		},
		"source value lower than zero": {
			source:        func() *int { i := -10; return &i }(),
			expectedValue: defaultValue,
		},
		"source value greater than zero": {
			source:        func() *int { i := 10; return &i }(),
			expectedValue: time.Duration(10) * time.Second,
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			assert(t, tt)
		})
	}
}

func TestConfig_GetConfigExecTimeout(t *testing.T) {
	testGetDuration(t, defaultConfigExecTimeout, func(t *testing.T, tt getDurationTestCase) {
		c := &config{
			CustomConfig: &common.CustomConfig{
				ConfigExecTimeout: tt.source,
			},
		}

		assert.Equal(t, tt.expectedValue, c.GetConfigExecTimeout())
	})
}

func TestConfig_GetPrepareExecTimeout(t *testing.T) {
	testGetDuration(t, defaultPrepareExecTimeout, func(t *testing.T, tt getDurationTestCase) {
		c := &config{
			CustomConfig: &common.CustomConfig{
				PrepareExecTimeout: tt.source,
			},
		}

		assert.Equal(t, tt.expectedValue, c.GetPrepareExecTimeout())
	})
}

func TestConfig_GetCleanupExecTimeout(t *testing.T) {
	testGetDuration(t, defaultCleanupExecTimeout, func(t *testing.T, tt getDurationTestCase) {
		c := &config{
			CustomConfig: &common.CustomConfig{
				CleanupExecTimeout: tt.source,
			},
		}

		assert.Equal(t, tt.expectedValue, c.GetCleanupScriptTimeout())
	})
}

func TestConfig_GetTerminateTimeout(t *testing.T) {
	testGetDuration(t, process.GracefulTimeout, func(t *testing.T, tt getDurationTestCase) {
		c := &config{
			CustomConfig: &common.CustomConfig{
				GracefulKillTimeout: tt.source,
			},
		}

		assert.Equal(t, tt.expectedValue, c.GetGracefulKillTimeout())
	})
}

func TestConfig_GetForceKillTimeout(t *testing.T) {
	testGetDuration(t, process.KillTimeout, func(t *testing.T, tt getDurationTestCase) {
		c := &config{
			CustomConfig: &common.CustomConfig{
				ForceKillTimeout: tt.source,
			},
		}

		assert.Equal(t, tt.expectedValue, c.GetForceKillTimeout())
	})
}
