//go:build !integration

package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLinuxParser_ParseVolume(t *testing.T) {
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
			volumeSpec:    "/destination",
			expectedParts: &Volume{Destination: "/destination"},
		},
		"source and destination": {
			volumeSpec:    "/source:/destination",
			expectedParts: &Volume{Source: "/source", Destination: "/destination"},
		},
		"destination and mode": {
			volumeSpec:    "/destination:rw",
			expectedParts: &Volume{Destination: "/destination", Mode: "rw"},
		},
		"all values": {
			volumeSpec:    "/source:/destination:rw",
			expectedParts: &Volume{Source: "/source", Destination: "/destination", Mode: "rw"},
		},
		"read only": {
			volumeSpec:    "/source:/destination:ro",
			expectedParts: &Volume{Source: "/source", Destination: "/destination", Mode: "ro"},
		},
		"SELinux label and read only is shared among multiple containers": {
			volumeSpec:    "/source:/destination:ro,z",
			expectedParts: &Volume{Source: "/source", Destination: "/destination", Mode: "ro", Label: "z"},
		},
		"SELinux label and read only is private": {
			volumeSpec:    "/source:/destination:ro,Z",
			expectedParts: &Volume{Source: "/source", Destination: "/destination", Mode: "ro", Label: "Z"},
		},
		"volume case sensitive": {
			volumeSpec:    "/Source:/Destination:rw",
			expectedParts: &Volume{Source: "/Source", Destination: "/Destination", Mode: "rw"},
		},
		"support SELinux label bind mount content is shared among multiple containers": {
			volumeSpec:    "/source:/destination:z",
			expectedParts: &Volume{Source: "/source", Destination: "/destination", Mode: "", Label: "z"},
		},
		"support SELinux label bind mount content is private and unshare": {
			volumeSpec:    "/source:/destination:Z",
			expectedParts: &Volume{Source: "/source", Destination: "/destination", Mode: "", Label: "Z"},
		},
		"unsupported mode": {
			volumeSpec:    "/source:/destination:T",
			expectedError: NewInvalidVolumeSpecErr("/source:/destination:T"),
		},
		"too much colons": {
			volumeSpec:    "/source:/destination:rw:something",
			expectedError: NewInvalidVolumeSpecErr("/source:/destination:rw:something"),
		},
		"invalid source": {
			volumeSpec:    ":/destination",
			expectedError: NewInvalidVolumeSpecErr(":/destination"),
		},
		"named source": {
			volumeSpec:    "volume_name:/destination",
			expectedParts: &Volume{Source: "volume_name", Destination: "/destination"},
		},
		"bind propagation": {
			volumeSpec:    "/source:/destination:rslave",
			expectedParts: &Volume{Source: "/source", Destination: "/destination", BindPropagation: "rslave"},
		},
		"mode with bind propagation": {
			volumeSpec: "/source:/destination:ro,rslave",
			expectedParts: &Volume{
				Source:          "/source",
				Destination:     "/destination",
				Mode:            "ro",
				BindPropagation: "rslave",
			},
		},
		"unsupported bind propagation": {
			volumeSpec:    "/source:/destination:unknown",
			expectedError: NewInvalidVolumeSpecErr("/source:/destination:unknown"),
		},
		"unsupported bind propagation with mode": {
			volumeSpec:    "/source:/destination:ro,unknown",
			expectedError: NewInvalidVolumeSpecErr("/source:/destination:ro,unknown"),
		},
		"malformed bind propagation": {
			volumeSpec:    "/source:/destination:,rslave",
			expectedError: NewInvalidVolumeSpecErr("/source:/destination:,rslave"),
		},
		// This is not a valid syntax for Docker but GitLab Runner still parses
		// for the sake of simplicity, check
		// https://gitlab.com/gitlab-org/gitlab-runner/merge_requests/1632#note_240079623
		// for the discussion and rationale.
		"too much colons for bind propagation": {
			volumeSpec: "/source:/destination:rw:rslave",
			expectedParts: &Volume{
				Source:          "/source",
				Destination:     "/destination",
				Mode:            "rw",
				BindPropagation: "rslave",
			},
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			parser := NewLinuxParser()
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
