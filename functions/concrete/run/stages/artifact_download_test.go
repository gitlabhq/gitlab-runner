//go:build !integration

package stages

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArtifactDownloadArgs(t *testing.T) {
	base := []string{"artifacts-downloader", "--url", "https://gitlab.example.com", "--token", "token", "--id", "123"}

	tests := map[string]struct {
		directDownload *bool
		expected       []string
	}{
		"not set leaves it to the helper": {
			expected: base,
		},
		"true": {
			directDownload: new(true),
			expected:       append(slices.Clip(base), "--direct-download=true"),
		},
		"false": {
			directDownload: new(false),
			expected:       append(slices.Clip(base), "--direct-download=false"),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			s := ArtifactDownload{ID: 123, Token: "token", DirectDownload: tc.directDownload}
			assert.Equal(t, tc.expected, s.args("https://gitlab.example.com"))
		})
	}
}
