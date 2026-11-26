//go:build !integration

package windows

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolve(t *testing.T) {
	tests := []struct {
		name     string
		kernel   string
		want     Resolution
		fallback bool
		wantErr  error
	}{
		// Exact match — IsFallback() is false.
		{name: "exact: dotted", kernel: "10.0.22631", want: Resolution{Version: V21H2, HostBuild: "10.0.22631", MatchedBuild: "10.0.22631"}},
		// Docker format: build in parens (26100) differs from host build; host build must be used.
		{name: "exact: docker format with differing BuildLabEx", kernel: "10.0 26300 (26100.1.amd64fre.ge_release.240331-1435)", want: Resolution{Version: V24H2, HostBuild: "10.0.26300", MatchedBuild: "10.0.26300"}},
		// Fallback — IsFallback() is true, MatchedBuild < HostBuild.
		{name: "fallback: above highest known build", kernel: "10.0.26400", want: Resolution{Version: V24H2, HostBuild: "10.0.26400", MatchedBuild: "10.0.26300"}, fallback: true},
		{name: "fallback: gap 10.0.26150 -> V24H2 via 26100", kernel: "10.0.26150", want: Resolution{Version: V24H2, HostBuild: "10.0.26150", MatchedBuild: "10.0.26100"}, fallback: true},
		{name: "fallback: bucket boundary 10.0.24000 -> V21H2 not 26100", kernel: "10.0.24000", want: Resolution{Version: V21H2, HostBuild: "10.0.24000", MatchedBuild: "10.0.22631"}, fallback: true},
		{name: "fallback: SAC gap 10.0.19042 -> V1809 via server entry 17763", kernel: "10.0.19042", want: Resolution{Version: V1809, HostBuild: "10.0.19042", MatchedBuild: "10.0.17763"}, fallback: true},
		// Error — zero Resolution must be returned.
		{name: "error: below oldest", kernel: "10.0 17134 (17134.1.amd64fre.rs4_release.180410-1804)", wantErr: ErrUnsupportedWindowsVersion},
		{name: "error: other major 6.3.9600", kernel: "6.3.9600", wantErr: ErrUnsupportedWindowsVersion},
		{name: "error: other minor 10.1.30000", kernel: "10.1.30000", wantErr: ErrUnsupportedWindowsVersion},
		{name: "error: unparseable build 10.0.abc", kernel: "10.0.abc", wantErr: ErrUnsupportedWindowsVersion},
		{name: "error: non-numeric minor 10.abc.123", kernel: "10.abc.123", wantErr: ErrUnsupportedWindowsVersion},
		{name: "error: too short 10.0", kernel: "10.0", wantErr: ErrUnsupportedWindowsVersion},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := Resolve(tt.kernel)
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.want, r)
			assert.Equal(t, tt.fallback, r.IsFallback())
		})
	}
}

// TestSupportedWindowsBuildsAllKeysParse asserts that every key in
// supportedWindowsBuilds parses via parseBuildKey. This guards against future
// typos in the map.
func TestSupportedWindowsBuildsAllKeysParse(t *testing.T) {
	for key := range supportedWindowsBuilds {
		_, _, _, ok := parseBuildKey(key)
		assert.True(t, ok, "key %q does not parse", key)
	}
}
