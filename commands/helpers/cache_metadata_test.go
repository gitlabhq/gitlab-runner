//go:build !integration

package helpers

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteCacheMetadataFile(t *testing.T) {
	tests := map[string]struct {
		metadata    map[string]string
		archiveFile string

		expectWriteError bool
		expectedBlob     string
	}{
		"no metadata": {
			archiveFile:  "archive.zip",
			expectedBlob: "{}",
		},
		"no archive": {
			expectedBlob: "{}",
		},
		"bubbles up write errors": {
			archiveFile:      "some/path/which/does/not/exist/archive.zip",
			expectWriteError: true,
		},
		"canonicalizes metadata keys": {
			metadata: map[string]string{
				"FoO": "some Foo",
				"BAR": "some Bar",
				"":    "nope",
			},
			expectedBlob: `{"bar":"some Bar","foo":"some Foo"}`,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()

			archiveFile := filepath.Join(dir, test.archiveFile)
			expectedMetadataFile := filepath.Join(filepath.Dir(archiveFile), "metadata.json")

			err := writeCacheMetadataFile(archiveFile, test.metadata)

			if test.expectWriteError {
				msg := "writing metadata file: open %s:"
				require.ErrorContains(t, err, fmt.Sprintf(msg, expectedMetadataFile))
				return
			}

			require.NoError(t, err)

			b, err := os.ReadFile(expectedMetadataFile)
			require.NoError(t, err, "reading metadata file")
			assert.Equal(t, test.expectedBlob, string(b), "metadata file content")
		})
	}
}
