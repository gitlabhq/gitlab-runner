//go:build !integration

package cachekey

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitize(t *testing.T) {
	tests := []struct {
		rawKey      string
		expectedKey string
		wantErr     bool
	}{
		// ── Empty / identity ────────────────────────────────────────
		{rawKey: ""},
		{rawKey: "fallback_key", expectedKey: "fallback_key"},
		{rawKey: "some-job/some-ref", expectedKey: "some-job/some-ref"},
		{rawKey: ".../....", expectedKey: ".../...."},
		{rawKey: "...", expectedKey: "..."},

		// ── Trailing whitespace / slashes / backslashes ─────────────
		{rawKey: "fallback_key/", expectedKey: "fallback_key"},
		{rawKey: "fallback_key ", expectedKey: "fallback_key"},
		{rawKey: "fallback_key\\", expectedKey: "fallback_key"},
		{rawKey: "fallback_key/ \\", expectedKey: "fallback_key"},
		{rawKey: "fallback_key/ / \\  \\", expectedKey: "fallback_key"},
		{rawKey: "fallback_key/o", expectedKey: "fallback_key/o"},
		{rawKey: "fallback_key / \\o", expectedKey: "fallback_key / /o"},
		{rawKey: "\t foo bar \t\r", expectedKey: "\t foo bar"},
		{rawKey: " foo / bar ", expectedKey: " foo / bar"},
		{rawKey: "foo\r", expectedKey: "foo"},
		{rawKey: "foo\t", expectedKey: "foo"},
		{rawKey: "foo \t \r ", expectedKey: "foo"},

		// ── Completely unsanitisable ────────────────────────────────
		{rawKey: "\\", wantErr: true},
		{rawKey: "\\.", wantErr: true},
		{rawKey: "/", wantErr: true},
		{rawKey: " ", wantErr: true},
		{rawKey: ".", wantErr: true},
		{rawKey: "..", wantErr: true},
		{rawKey: " / ", wantErr: true},
		{rawKey: "//", wantErr: true},
		{rawKey: `//\`, wantErr: true},
		{rawKey: "../.", wantErr: true},
		{rawKey: "foo\\bar\\..\\..", wantErr: true},
		{rawKey: "foo/bar/../..", wantErr: true},
		{rawKey: " \t\r\n", wantErr: true},

		// ── URL-encoded slashes (%2f / %2F) ────────────────────────
		{rawKey: "something %2F something", expectedKey: "something / something"},
		{rawKey: "something %2f something", expectedKey: "something / something"},
		{rawKey: "some%2f../job/some/ref/.", expectedKey: "job/some/ref"},

		// ── URL-encoded dots (%2e / %2E) ───────────────────────────
		{rawKey: "%2E", wantErr: true},
		{rawKey: "%2E%2E", wantErr: true},
		{rawKey: "%2E%2E%2E", expectedKey: "..."},
		{rawKey: "%2e", wantErr: true},
		{rawKey: "%2e%2E", wantErr: true},
		{rawKey: ".%2E", wantErr: true},
		{rawKey: "%2e.", wantErr: true},
		{rawKey: "%2E%2e%2E", expectedKey: "..."},

		// %5C is left as-is (literal percent-encoded backslash is fine).
		{rawKey: "%5C", expectedKey: "%5C"},
		{rawKey: "%5c", expectedKey: "%5c"},

		// ── Forward-slash path traversal ────────────────────────────
		{rawKey: "foo/./bar", expectedKey: "foo/bar"},
		{rawKey: "foo/blipp/../bar", expectedKey: "foo/bar"},
		{rawKey: "/foo/bar", expectedKey: "foo/bar"},
		{rawKey: "//foo/bar", expectedKey: "foo/bar"},
		{rawKey: "./foo/bar", expectedKey: "foo/bar"},
		{rawKey: "../foo/bar", expectedKey: "foo/bar"},
		{rawKey: ".../foo/bar", expectedKey: ".../foo/bar"},
		{rawKey: "foo/bar/..", expectedKey: "foo"},
		{rawKey: "foo/bar/../../../.././blerp", expectedKey: "blerp"},
		{rawKey: "a/b/c/../../d", expectedKey: "a/d"},

		// ── Backslash path traversal ────────────────────────────────
		{rawKey: `job\name/git\ref`, expectedKey: "job/name/git/ref"},
		{rawKey: "foo\\.\\bar", expectedKey: "foo/bar"},
		{rawKey: "foo\\blipp\\..\\bar", expectedKey: "foo/bar"},
		{rawKey: "\\foo\\bar", expectedKey: "foo/bar"},
		{rawKey: "\\\\foo\\bar", expectedKey: "foo/bar"},
		{rawKey: ".\\foo\\bar", expectedKey: "foo/bar"},
		{rawKey: "..\\foo\\bar", expectedKey: "foo/bar"},
		{rawKey: "...\\foo\\bar", expectedKey: ".../foo/bar"},
		{rawKey: "foo\\bar\\..", expectedKey: "foo"},
		{rawKey: "foo\\bar\\..\\..\\..\\..\\.\\blerp", expectedKey: "blerp"},

		// ── Space-only segments & misc ──────────────────────────────
		{rawKey: "foo/ /bar", expectedKey: "foo/ /bar"},
		{rawKey: "foo/ /", expectedKey: "foo"},
		{rawKey: "foo/ / /", expectedKey: "foo"},
	}

	for i, tt := range tests {
		name := fmt.Sprintf("%d:%q", i, tt.rawKey)
		t.Run(name, func(t *testing.T) {
			actual, err := Sanitize(tt.rawKey)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedKey, actual)
		})
	}
}

// TestSanitizeInvariants checks properties that must hold for every sanitised
// key, regardless of input.
func TestSanitizeInvariants(t *testing.T) {
	cases := []string{
		"a", "a/b", "../a", "a/../b", "a/./b",
		"a\\b", `a\..\\b`, "/a/b/", " a ", "...",
		"%2e%2e/%2f", "a/b/c/../../d/e",
	}
	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			key, _ := Sanitize(raw)
			if key == "" {
				return // unsanitisable, nothing to check
			}
			assert.False(t, strings.HasPrefix(key, "/"), "must not start with /")
			assert.False(t, key == ".." || strings.HasPrefix(key, "../"), "must not start with .. segment")
			assert.False(t, strings.Contains(key, `\`), "must not contain backslash")
			assert.False(t, strings.HasSuffix(key, " "), "must not end with space")
			assert.False(t, strings.HasSuffix(key, "/"), "must not end with /")

			// No segment should be "." or ".."
			for _, seg := range strings.Split(key, "/") {
				assert.NotEqual(t, ".", seg, "must not contain '.' segment")
				assert.NotEqual(t, "..", seg, "must not contain '..' segment")
			}
		})
	}
}

// TestSanitizeIdempotent verifies that sanitising an already-clean key
// returns it unchanged with no error.
func TestSanitizeIdempotent(t *testing.T) {
	inputs := []string{
		"fallback_key",
		"some-job/some-ref",
		"a/b/c",
		"...",
		".../foo/bar",
	}
	for _, raw := range inputs {
		t.Run(raw, func(t *testing.T) {
			first, err1 := Sanitize(raw)
			assert.NoError(t, err1)

			second, err2 := Sanitize(first)
			assert.NoError(t, err2)
			assert.Equal(t, first, second, "sanitise should be idempotent")
		})
	}
}
