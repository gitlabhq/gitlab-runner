package docker

import (
	"fmt"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
)

func Test_windowsHelperImage_Tag(t *testing.T) {
	revision := "4011f186"
	cases := []struct {
		osType          string
		expectedVersion string
		expectedErr     bool
	}{
		{
			osType:          "Windows Server 2019 Datacenter Evaluation Version 1809 (OS Build 17763.316)",
			expectedVersion: fmt.Sprintf("%s-%s-%s", "x86_64", revision, nanoserver1809),
			expectedErr:     false,
		},
		{
			osType:          "Windows Server Datacenter Version 1803 (OS Build 17134.590)",
			expectedVersion: fmt.Sprintf("%s-%s-%s", "x86_64", revision, nanoserver1803),
			expectedErr:     false,
		},
		{
			osType:      "some random string",
			expectedErr: true,
		},
	}

	for _, c := range cases {
		t.Run(c.osType, func(t *testing.T) {
			w := newWindowsHelperImage(types.Info{OSType: c.osType})

			tag, err := w.Tag(revision)

			if c.expectedErr {
				assert.Error(t, err)
				return
			}

			assert.Equal(t, c.expectedVersion, tag)
			assert.NoError(t, err)
		})
	}
}

func Test_windowsHelperImage_IsSupportingLocalImport(t *testing.T) {
	u := newWindowsHelperImage(types.Info{})
	assert.False(t, u.IsSupportingLocalImport())
}
