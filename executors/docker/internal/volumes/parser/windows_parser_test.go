//go:build !integration && windows

package parser

import (
	"strings"
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
		"source and destination case insensitive disk mount": {
			volumeSpec:    `C:\source:C:\destination`,
			expectedParts: &Volume{Source: `C:\source`, Destination: `C:\destination`},
		},
		"source and destination case insensitive": {
			volumeSpec:    `c:\Source:c:\Destination`,
			expectedParts: &Volume{Source: `c:\Source`, Destination: `c:\Destination`},
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
		"named pipes": {
			volumeSpec:    `\\.\pipe\docker_engine1:\\.\pipe\docker_engine2`,
			expectedParts: &Volume{Source: `\\.\pipe\docker_engine1`, Destination: `\\.\pipe\docker_engine2`},
		},
		"named pipes with forward slashes": {
			volumeSpec:    `//./pipe/docker_engine1://./pipe/docker_engine2`,
			expectedParts: &Volume{Source: `//./pipe/docker_engine1`, Destination: `//./pipe/docker_engine2`},
		},

		"$VAR in destination is a allowed": {
			volumeSpec: `volume_name:c:\some\$VAR\blipp`,
			expectedParts: &Volume{
				Source:      `volume_name`,
				Destination: `c:\some\$VAR\blipp`,
			},
		},
		"$VAR at start of destination is not allowed": {
			volumeSpec:    `volume_name:$VAR\blipp`,
			expectedError: NewInvalidVolumeSpecErr(`volume_name:$VAR\blipp`),
		},
		"${VAR} in destination is a allowed": {
			volumeSpec: `volume_name:c:\some\${VAR}\blipp`,
			expectedParts: &Volume{
				Source:      `volume_name`,
				Destination: `c:\some\${VAR}\blipp`,
			},
		},
		"${VAR} at start of destination is not allowed": {
			volumeSpec:    `volume_name:${VAR}\blipp`,
			expectedError: NewInvalidVolumeSpecErr(`volume_name:${VAR}\blipp`),
		},
		"multiple different var refs in destination are allowed": {
			volumeSpec: `volume_name:c:\${root}\$sub-test\dir`,
			expectedParts: &Volume{
				Source:      `volume_name`,
				Destination: `c:\${root}\$sub-test\dir`,
			},
		},
		// Even if the variable refs are syntactically not correct, the REs should not block them, we do the expansion later
		// and hand it to docker, and either of those will catch these cases.
		"invalid var refs in destination are allowed": {
			volumeSpec: `volume_name:c:\${r\$$-test\dir`,
			expectedParts: &Volume{
				Source:      `volume_name`,
				Destination: `c:\${r\$$-test\dir`,
			},
		},
	}

	var identity = func(s string) string { return s }

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			parser := NewWindowsParser(identity)
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

func TestWindowsParser_DestinationVarExpansion(t *testing.T) {
	fakeVarExpander := strings.NewReplacer(
		"foo", "REPLACED(bar)",
		"blipp", "REPLACED(zark)",
	).Replace

	tests := map[string]*Volume{
		`volume_name:c:\foo:ro`: &Volume{
			Source:      `volume_name`,
			Destination: `c:\REPLACED(bar)`,
			Mode:        "ro",
		},
		`f:\foo:c:\foo\some-blipp-ref\blapp`: &Volume{
			Source:      `f:\foo`,                                         // not expanded
			Destination: `c:\REPLACED(bar)\some-REPLACED(zark)-ref\blapp`, // expanded
		},
	}

	for volumeSpec, expectedVolume := range tests {
		t.Run(volumeSpec, func(t *testing.T) {
			parser := NewWindowsParser(fakeVarExpander)

			volume, err := parser.ParseVolume(volumeSpec)
			assert.NoError(t, err)
			assert.Equal(t, expectedVolume, volume)
		})
	}
}
