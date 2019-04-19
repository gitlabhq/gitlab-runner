package helperimage

import (
	"fmt"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
)

func Test_windowsInfo_Tag(t *testing.T) {
	revision := "4011f186"
	cases := []struct {
		operatingSystem string
		expectedVersion string
		expectedErr     error
	}{
		{
			operatingSystem: "Windows Server 2019 Datacenter Evaluation Version 1809 (OS Build 17763.316)",
			expectedVersion: fmt.Sprintf("%s-%s-%s", "x86_64", revision, baseImage1809),
			expectedErr:     nil,
		},
		{
			operatingSystem: "Windows Server Datacenter Version 1809 (OS Build 1803.590)",
			expectedVersion: fmt.Sprintf("%s-%s-%s", "x86_64", revision, baseImage1809),
			expectedErr:     nil,
		},
		{
			operatingSystem: "Windows Server Datacenter Version 1803 (OS Build 17134.590)",
			expectedVersion: fmt.Sprintf("%s-%s-%s", "x86_64", revision, baseImage1803),
			expectedErr:     nil,
		},
		{
			operatingSystem: "some random string",
			expectedErr:     ErrUnsupportedOSVersion,
		},
	}

	for _, c := range cases {
		t.Run(c.operatingSystem, func(t *testing.T) {
			w := newWindowsInfo(types.Info{OperatingSystem: c.operatingSystem})

			tag, err := w.Tag(revision)

			assert.Equal(t, c.expectedVersion, tag)
			assert.Equal(t, c.expectedErr, err)
		})
	}
}

func Test_windowsInfo_IsSupportingLocalImport(t *testing.T) {
	u := newWindowsInfo(types.Info{})
	assert.False(t, u.IsSupportingLocalImport())
}
