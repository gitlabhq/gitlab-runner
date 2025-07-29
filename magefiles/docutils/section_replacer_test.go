//go:build !integration && !windows

package docutils

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSectionReplacer_Replace(t *testing.T) {
	wikiPageFile, err := os.Open(filepath.Join("testdata", "source.md"))
	require.NoError(t, err)

	defer wikiPageFile.Close()

	replacer := NewSectionReplacer("runner_version_table", wikiPageFile)
	err = replacer.Replace(func(in io.Reader) (string, error) {
		return "Rewritten content\n", nil
	})
	assert.NoError(t, err)

	wikiRewrittenFile, err := os.ReadFile(filepath.Join("testdata", "source_rewritten.md"))
	require.NoError(t, err)

	assert.Equal(t, string(wikiRewrittenFile), replacer.Output())
}
