//go:build !integration
// +build !integration

package windows

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersion(t *testing.T) {
	tests := []struct {
		operatingSystem string
		expectedVersion string
		expectedErr     error
	}{
		{
			operatingSystem: "Windows Server 2019 Datacenter Evaluation Version 1809 (OS Build 17763.316)",
			expectedVersion: V1809,
			expectedErr:     nil,
		},
		{
			operatingSystem: "Windows Server Datacenter Version 1809 (OS Build 1803.590)",
			expectedVersion: V1809,
			expectedErr:     nil,
		},
		{
			operatingSystem: "Windows 10 Pro Version 2004 (OS Build 19041.329)",
			expectedVersion: V2004,
			expectedErr:     nil,
		},
		{
			operatingSystem: "Windows Server Datacenter Version 2009 (OS Build 19042.985)",
			expectedVersion: V20H2,
			expectedErr:     nil,
		},
		{
			operatingSystem: "some random string",
			expectedErr:     NewUnsupportedWindowsVersionError("some random string"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.operatingSystem, func(t *testing.T) {
			version, err := Version(tt.operatingSystem)

			assert.Equal(t, tt.expectedVersion, version)
			assert.ErrorIs(t, err, tt.expectedErr)
		})
	}
}
