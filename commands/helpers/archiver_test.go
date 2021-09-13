//go:build !integration
// +build !integration

package helpers

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive"
)

func TestCompressionLevel(t *testing.T) {
	tests := map[string]archive.CompressionLevel{
		"fastest": archive.FastestCompression,
		"fast":    archive.FastCompression,
		"slow":    archive.SlowCompression,
		"slowest": archive.SlowestCompression,
		"default": archive.DefaultCompression,
		"":        archive.DefaultCompression,
		"invalid": archive.DefaultCompression,
	}

	for name, level := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, level, GetCompressionLevel(name))
		})
	}
}
