package helperimage

import (
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
)

func Test_linuxInfo_Tag(t *testing.T) {
	cases := []struct {
		name        string
		dockerArch  string
		revision    string
		expectedTag string
	}{
		{
			name:        "When dockerArch not specified we fallback to runtime arch",
			dockerArch:  "",
			revision:    "2923a43",
			expectedTag: "x86_64-2923a43",
		},
		{
			name:        "Docker runs on armv6l",
			dockerArch:  "armv6l",
			revision:    "2923a43",
			expectedTag: "arm-2923a43",
		},
		{
			name:        "Docker runs on amd64",
			dockerArch:  "amd64",
			revision:    "2923a43",
			expectedTag: "x86_64-2923a43",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			u := newLinuxInfo(types.Info{Architecture: c.dockerArch})

			tag, err := u.Tag(c.revision)

			assert.NoError(t, err)
			assert.Equal(t, c.expectedTag, tag)
		})
	}
}

func Test_linuxInfo_IsSupportingLocalImport(t *testing.T) {
	u := newLinuxInfo(types.Info{})
	assert.True(t, u.IsSupportingLocalImport())
}
