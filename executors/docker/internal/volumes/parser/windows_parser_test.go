package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWindowsParser_ParseVolume(t *testing.T) {
	testCases := map[string]struct {
		volumeSpec    string
		expectedParts *Volume
		expectedError error
	}{
		"empty": {
			volumeSpec:    "",
			expectedError: NewInvalidVolumeSpecErr(""),
		},
		"destination only": {
			volumeSpec:    `c:\destination`,
			expectedParts: &Volume{Destination: `c:\destination`},
		},
		"source and destination": {
			volumeSpec:    `c:\source:c:\destination`,
			expectedParts: &Volume{Source: `c:\source`, Destination: `c:\destination`},
		},
		"destination and mode": {
			volumeSpec:    `c:\destination:rw`,
			expectedParts: &Volume{Destination: `c:\destination`, Mode: "rw"},
		},
		"all values": {
			volumeSpec:    `c:\source:c:\destination:rw`,
			expectedParts: &Volume{Source: `c:\source`, Destination: `c:\destination`, Mode: "rw"},
		},
		"too much colons": {
			volumeSpec:    `c:\source:c:\destination:rw:something`,
			expectedError: NewInvalidVolumeSpecErr(`c:\source:c:\destination:rw:something`),
		},
		"invalid source": {
			volumeSpec:    `/destination:c:\destination`,
			expectedError: NewInvalidVolumeSpecErr(`/destination:c:\destination`),
		},
		"named source": {
			volumeSpec:    `volume_name:c:\destination`,
			expectedParts: &Volume{Source: "volume_name", Destination: `c:\destination`},
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			parser := newWindowsParser()
			parts, err := parser.ParseVolume(testCase.volumeSpec)

			if testCase.expectedError == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, testCase.expectedError.Error())
			}

			assert.Equal(t, testCase.expectedParts, parts)
		})
	}
}
