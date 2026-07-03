//go:build !integration

package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBootVerifyAccessorsUseDefaults(t *testing.T) {
	cases := map[string]*BootVerify{
		"nil":         nil,
		"zero values": {},
	}
	for name, b := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, defaultBootVerifyTimeout, b.GetTimeout())
			assert.Equal(t, defaultBootVerifyAcquireMinBackoff, b.GetAcquireMinBackoff())
			assert.Equal(t, defaultBootVerifyAcquireMaxBackoff, b.GetAcquireMaxBackoff())
		})
	}
}

func TestGetBootVerify(t *testing.T) {
	bv := &BootVerify{Enabled: true}
	cases := map[string]struct {
		runner   *RunnerConfig
		expected *BootVerify
	}{
		"no experimental section":   {runner: &RunnerConfig{}, expected: nil},
		"no boot_verify subsection": {runner: &RunnerConfig{Experimental: &RunnerExperimental{}}, expected: nil},
		"configured":                {runner: &RunnerConfig{Experimental: &RunnerExperimental{BootVerify: bv}}, expected: bv},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Same(t, tc.expected, tc.runner.GetBootVerify())
		})
	}
}

func TestBootVerifyAccessorsUseConfiguredValues(t *testing.T) {
	b := &BootVerify{
		Timeout:           90 * time.Second,
		AcquireMinBackoff: 2 * time.Second,
		AcquireMaxBackoff: 20 * time.Second,
	}

	assert.Equal(t, 90*time.Second, b.GetTimeout())
	assert.Equal(t, 2*time.Second, b.GetAcquireMinBackoff())
	assert.Equal(t, 20*time.Second, b.GetAcquireMaxBackoff())
}
