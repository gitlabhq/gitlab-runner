package helperimage

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_linuxInfo_create(t *testing.T) {
	cases := []struct {
		name         string
		dockerArch   string
		revision     string
		expectedInfo Info
	}{
		{
			name:       "When dockerArch not specified we fallback to runtime arch",
			dockerArch: "",
			revision:   "2923a43",
			expectedInfo: Info{
				Architecture: getExpectedArch(),
				Name:         name,
				Tag:          fmt.Sprintf("%s-2923a43", getExpectedArch()),
				IsSupportingLocalImport: true,
			},
		},
		{
			name:       "Docker runs on armv6l",
			dockerArch: "armv6l",
			revision:   "2923a43",
			expectedInfo: Info{
				Architecture: "arm",
				Name:         name,
				Tag:          "arm-2923a43",
				IsSupportingLocalImport: true,
			},
		},
		{
			name:       "Docker runs on amd64",
			dockerArch: "amd64",
			revision:   "2923a43",
			expectedInfo: Info{
				Architecture: "x86_64",
				Name:         name,
				Tag:          "x86_64-2923a43",
				IsSupportingLocalImport: true,
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := new(linuxInfo)

			image, err := l.Create(c.revision, Config{Architecture: c.dockerArch})

			assert.NoError(t, err)
			assert.Equal(t, c.expectedInfo, image)
		})
	}
}

// We re write amd64 to x86_64 for the helper image, and we don't want this test
// to be runtime dependant.
func getExpectedArch() string {
	if runtime.GOARCH == "amd64" {
		return "x86_64"

	}

	return runtime.GOARCH
}
