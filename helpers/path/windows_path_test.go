//go:build !integration && windows

package path

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWindowsJoin(t *testing.T) {
	p := NewWindowsPath()

	tests := map[string]struct {
		args     []string
		expected string
	}{
		"the same result": {
			args:     []string{"dir"},
			expected: "dir",
		},
		"joins absolute and relative": {
			args:     []string{"c:\\path\\to", "dir"},
			expected: "c:\\path\\to\\dir",
		},
		"joins absolute two absolutes": {
			args:     []string{"d:/path/to", "/dir/path"},
			expected: "d:\\path\\to\\dir\\path",
		},
		"cleans paths": {
			args:     []string{"path\\..\\to", "dir/with/my/../path"},
			expected: "to\\dir\\with\\path",
		},
		"does normalize separators": {
			args:     []string{"path/to/windows/dir"},
			expected: "path\\to\\windows\\dir",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, test.expected, p.Join(test.args...))
		})
	}
}

func TestWindowsIsAbs(t *testing.T) {
	p := NewWindowsPath()

	tests := map[string]struct {
		arg      string
		expected bool
	}{
		"relative path": {
			arg:      "dir",
			expected: false,
		},
		// Go's filepath.IsAbs() does not believe unix-style paths on Windows
		// are absolute. However, Windows will typically work fine with these
		// paths. For example:
		//     [System.IO.Path]::IsPathRooted("/path/to/dir")
		// will return True.
		// For now, we keep this as expected=false though, as it is what Go
		// returns.
		"unix absolute path": {
			arg:      "/path/to/dir",
			expected: false,
		},
		"unclean unix absolute path": {
			arg:      "/path/../to/dir",
			expected: false,
		},
		"windows absolute path": {
			arg:      "c:\\path\\to\\dir",
			expected: true,
		},
		"unclean windows absolute path": {
			arg:      "c:\\path\\..\\to\\..\\dir",
			expected: true,
		},
		"named pipe path": {
			arg:      `\\.\pipe\docker_engine`,
			expected: true,
		},
		"named pipe path with forward slashes": {
			arg:      `//./pipe/docker_engine`,
			expected: true,
		},
		"UNC share root path": {
			arg:      `\\server\path\`,
			expected: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, test.expected, p.IsAbs(test.arg))
		})
	}
}

func TestWindowsIsRoot(t *testing.T) {
	p := NewWindowsPath()

	tests := map[string]struct {
		arg      string
		expected bool
	}{
		"relative path": {
			arg:      "dir",
			expected: false,
		},
		"absolute path without drive": {
			arg:      "/path/to/dir",
			expected: false,
		},
		"root path without drive": {
			arg:      "/",
			expected: false,
		},
		"root path with drive": {
			arg:      "c:/",
			expected: true,
		},
		"absolute path with drive": {
			arg:      "c:/path/to/dir",
			expected: false,
		},
		"named pipe path": {
			arg:      `\\.\pipe\docker_engine`,
			expected: false,
		},
		"named pipe path with forward slashes": {
			arg:      `//./pipe/docker_engine`,
			expected: false,
		},
		"UNC share name": {
			arg:      `\\server\path`,
			expected: false,
		},
		"UNC share root path": {
			arg:      `\\server\path\`,
			expected: true,
		},
		"UNC path": {
			arg:      `\\server\path\sub-path`,
			expected: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, test.expected, p.IsRoot(test.arg))
		})
	}
}

func TestWindowsContains(t *testing.T) {
	p := NewWindowsPath()

	tests := map[string]struct {
		basepath   string
		targetpath string
		expected   bool
	}{
		"root path": {
			basepath:   "/",
			targetpath: "/path/to/dir",
			expected:   true,
		},
		"unclean root path": {
			basepath:   "/other/..",
			targetpath: "/path/../to/dir",
			expected:   true,
		},
		"absolute path": {
			basepath:   "/other",
			targetpath: "/path/to/dir",
			expected:   false,
		},
		"unclean absolute path": {
			basepath:   "/other/../my/path",
			targetpath: "/path/../to/dir",
			expected:   false,
		},
		"relative path": {
			basepath:   "other",
			targetpath: "path/to/dir",
			expected:   false,
		},
		"invalid absolute path": {
			basepath:   "c:\\other",
			targetpath: "\\path\\to\\dir",
			expected:   false,
		},
		"windows absolute path": {
			basepath:   "c:\\path",
			targetpath: "c:\\path\\to\\dir",
			expected:   true,
		},
		"the same path without drive": {
			basepath:   "/path/to/dir",
			targetpath: "/path/to/dir",
			expected:   true,
		},
		"the same path with one having the drive": {
			basepath:   "c:/path/to/dir",
			targetpath: "/path/to/dir",
			expected:   false,
		},
		"the same path with the drive": {
			basepath:   "c:/path/to/dir",
			targetpath: "c:\\path\\to\\dir\\",
			expected:   true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, test.expected, p.Contains(test.basepath, test.targetpath))
		})
	}
}
