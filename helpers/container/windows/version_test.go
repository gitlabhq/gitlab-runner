package windows

import (
	"errors"
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
			operatingSystem: "Windows Server Datacenter Version 1903 (OS Build 18362.592)",
			expectedVersion: V1903,
			expectedErr:     nil,
		},
		{
			operatingSystem: "Windows Server Datacenter Version 1909 (OS Build 18363.720)",
			expectedVersion: V1909,
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
			assert.True(t, errors.Is(err, tt.expectedErr), "expected err %T, but got %T", tt.expectedErr, err)
		})
	}
}
