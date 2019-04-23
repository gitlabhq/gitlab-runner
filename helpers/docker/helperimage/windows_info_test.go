package helperimage

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_windowsInfo_create(t *testing.T) {
	revision := "4011f186"
	cases := []struct {
		operatingSystem string
		expectedInfo    Info
		expectedErr     error
	}{
		{
			operatingSystem: "Windows Server 2019 Datacenter Evaluation Version 1809 (OS Build 17763.316)",
			expectedInfo: Info{
				Architecture: windowsSupportedArchitecture,
				Name:         name,
				Tag:          fmt.Sprintf("%s-%s-%s", "x86_64", revision, baseImage1809),
				IsSupportingLocalImport: false,
			},
			expectedErr: nil,
		},
		{
			operatingSystem: "Windows Server Datacenter Version 1809 (OS Build 1803.590)",
			expectedInfo: Info{
				Architecture: windowsSupportedArchitecture,
				Name:         name,
				Tag:          fmt.Sprintf("%s-%s-%s", "x86_64", revision, baseImage1809),
				IsSupportingLocalImport: false,
			},
			expectedErr: nil,
		},
		{
			operatingSystem: "Windows Server Datacenter Version 1803 (OS Build 17134.590)",
			expectedInfo: Info{
				Architecture: windowsSupportedArchitecture,
				Name:         name,
				Tag:          fmt.Sprintf("%s-%s-%s", "x86_64", revision, baseImage1803),
				IsSupportingLocalImport: false,
			},
			expectedErr: nil,
		},
		{
			operatingSystem: "some random string",
			expectedErr:     ErrUnsupportedOSVersion,
		},
	}

	for _, c := range cases {
		t.Run(c.operatingSystem, func(t *testing.T) {
			w := new(windowsInfo)

			image, err := w.Create(revision, Config{OperatingSystem: c.operatingSystem})

			assert.Equal(t, c.expectedInfo, image)
			assert.Equal(t, c.expectedErr, err)
		})
	}
}
