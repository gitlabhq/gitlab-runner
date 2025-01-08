//go:build !integration

package windows

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersion(t *testing.T) {
	tests := []struct {
		kernelVersion   string
		expectedVersion string
		expectedErr     error
	}{
		{
			kernelVersion:   "10.0 17763 (17763.1.amd64fre.rs5_release.180914-1434)",
			expectedVersion: V1809,
			expectedErr:     nil,
		},
		{
			kernelVersion:   "10.0 20348 (20348.1.amd64fre.fe_release.210507-1500)",
			expectedVersion: V21H2,
			expectedErr:     nil,
		},
		{
			kernelVersion:   "10.0 26100 (26100.1.amd64fre.ge_release.240331-1435)",
			expectedVersion: V24H2,
			expectedErr:     nil,
		},
		{
			kernelVersion:   "10.0.17763",
			expectedVersion: V1809,
			expectedErr:     nil,
		},
		{
			kernelVersion:   "10.0.20348",
			expectedVersion: V21H2,
			expectedErr:     nil,
		},
		{
			kernelVersion:   "10.0.22631",
			expectedVersion: V21H2,
			expectedErr:     nil,
		},
		{
			kernelVersion: "10.0 17134 (17134.1.amd64fre.rs4_release.180410-1804)",
			expectedErr:   ErrUnsupportedWindowsVersion,
		},
		{
			kernelVersion: "some random string",
			expectedErr:   ErrUnsupportedWindowsVersion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.kernelVersion, func(t *testing.T) {
			version, err := Version(tt.kernelVersion)

			assert.Equal(t, tt.expectedVersion, version)
			assert.ErrorIs(t, err, tt.expectedErr)
		})
	}
}
