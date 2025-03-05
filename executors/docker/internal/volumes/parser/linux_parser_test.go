//go:build !integration

package parser

import (
	"strings"
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
		"destination not starting with / is not allowed": {
			volumeSpec:    "/source:blipp",
			expectedError: NewInvalidVolumeSpecErr("/source:blipp"),
		},

		"$VAR in destination is a allowed": {
			volumeSpec: "/source:/some/$VAR/blipp",
			expectedParts: &Volume{
				Source:      "/source",
				Destination: "/some/$VAR/blipp",
			},
		},
		"$VAR at start of destination is not allowed": {
			volumeSpec:    "/source:$VAR/blipp",
			expectedError: NewInvalidVolumeSpecErr("/source:$VAR/blipp"),
		},
		"${VAR} in destination is a allowed": {
			volumeSpec: "/source:/some/${VAR}/blipp",
			expectedParts: &Volume{
				Source:      "/source",
				Destination: "/some/${VAR}/blipp",
			},
		},
		"${VAR} at start of destination is not allowed": {
			volumeSpec:    "/source:${VAR}/blipp",
			expectedError: NewInvalidVolumeSpecErr("/source:${VAR}/blipp"),
		},
		"multiple different var refs in destination are allowed": {
			volumeSpec: "/source:/${root}/$sub-test/dir",
			expectedParts: &Volume{
				Source:      "/source",
				Destination: "/${root}/$sub-test/dir",
			},
		},
		// Even if the variable refs are syntactically not correct, the REs should not block them, we do the expansion later
		// and hand it to docker, and either of those will catch these cases.
		"invalid var refs in destination are allowed": {
			volumeSpec: "/source:/${r/$$-test/dir",
			expectedParts: &Volume{
				Source:      "/source",
				Destination: "/${r/$$-test/dir",
			},
		},
	}

	var identity = func(s string) string { return s }

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			parser := NewLinuxParser(identity)
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

func TestLinuxParser_DestinationVarExpansion(t *testing.T) {
	fakeVarExpander := strings.NewReplacer(
		"foo", "REPLACED(bar)",
		"blipp", "REPLACED(zark)",
	).Replace

	tests := map[string]*Volume{
		"/source:/foo:ro": &Volume{
			Source:      "/source",
			Destination: "/REPLACED(bar)",
			Mode:        "ro",
		},
		"/foo:/foo/some-blipp-ref/blapp": &Volume{
			Source:      "/foo",                                         // not expanded
			Destination: "/REPLACED(bar)/some-REPLACED(zark)-ref/blapp", // expanded
		},
	}

	for volumeSpec, expectedVolume := range tests {
		t.Run(volumeSpec, func(t *testing.T) {
			parser := NewLinuxParser(fakeVarExpander)

			volume, err := parser.ParseVolume(volumeSpec)
			assert.NoError(t, err)
			assert.Equal(t, expectedVolume, volume)
		})
	}
}
