//go:build !integration

package path

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnixJoin(t *testing.T) {
	p := NewUnixPath()

	tests := map[string]struct {
		args     []string
		expected string
	}{
		"the same result": {
			args:     []string{"dir"},
			expected: "dir",
		},
		"joins absolute and relative": {
			args:     []string{"/path/to", "dir"},
			expected: "/path/to/dir",
		},
		"joins absolute two absolutes": {
			args:     []string{"/path/to", "/dir/path"},
			expected: "/path/to/dir/path",
		},
		"cleans paths": {
			args:     []string{"path/../to", "dir/with/my/../path"},
			expected: "to/dir/with/path",
		},
		"does not normalize separators": {
			args:     []string{"path\\to\\windows\\dir"},
			expected: "path\\to\\windows\\dir",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, test.expected, p.Join(test.args...))
		})
	}
}

func TestUnixIsAbs(t *testing.T) {
	p := NewUnixPath()

	tests := map[string]struct {
		arg      string
		expected bool
	}{
		"relative path": {
			arg:      "dir",
			expected: false,
		},
		"relative path with dots": {
			arg:      "../dir",
			expected: false,
		},
		"absolute path": {
			arg:      "/path/to/dir",
			expected: true,
		},
		"unclean absolute": {
			arg:      "/path/../to/dir",
			expected: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, test.expected, p.IsAbs(test.arg))
		})
	}
}

func TestUnixIsRoot(t *testing.T) {
	p := NewUnixPath()

	tests := map[string]struct {
		arg      string
		expected bool
	}{
		"relative path": {
			arg: "dir", expected: false,
		},
		"absolute path": {
			arg: "/path/to/dir", expected: false,
		},
		"root path": {
			arg: "/", expected: true,
		},
		"unclean root": {
			arg: "/path/..", expected: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, test.expected, p.IsRoot(test.arg))
		})
	}
}

func TestUnixContains(t *testing.T) {
	p := NewUnixPath()

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
		"the same path": {
			basepath:   "/path/to/dir",
			targetpath: "/path/to/dir",
			expected:   true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, test.expected, p.Contains(test.basepath, test.targetpath))
		})
	}
}
