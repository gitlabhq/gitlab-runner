//go:build !integration

package fastzip

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

type newArchiverFunc func(io.Writer, string, archive.CompressionLevel) (archive.Archiver, error)

func TestArchiverStableFileOrder(t *testing.T) {
	t.Setenv(featureflags.UseFastzipStableFileOrder, "true")
	// Entries only finish out of order when several are compressed at once.
	t.Setenv(archiverConcurrency, "8")

	dir := t.TempDir()
	files := writeRandomTestFiles(t, dir, 100)

	tests := map[string]struct {
		newArchiver newArchiverFunc
		level       archive.CompressionLevel
	}{
		"deflate": {newArchiver: NewArchiver, level: archive.DefaultCompression},
		"store":   {newArchiver: NewArchiver, level: archive.FastestCompression},
		"zstd":    {newArchiver: NewZstdArchiver, level: archive.DefaultCompression},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			first := archiveChecksum(t, tc.newArchiver, tc.level, dir, files)

			for range 3 {
				assert.Equal(t, first, archiveChecksum(t, tc.newArchiver, tc.level, dir, files))
			}
		})
	}
}

// writeRandomTestFiles writes files of varied sizes with incompressible content,
// so their compression takes different amounts of time.
func writeRandomTestFiles(t *testing.T, dir string, count int) map[string]os.FileInfo {
	t.Helper()

	source := rand.NewChaCha8([32]byte{})
	rng := rand.New(source)
	files := make(map[string]os.FileInfo, count)

	for i := range count {
		data := make([]byte, rng.IntN(128*1024)+1)
		_, _ = source.Read(data)

		pathname := filepath.Join(dir, fmt.Sprintf("file-%03d", i))
		require.NoError(t, os.WriteFile(pathname, data, 0o644))

		fi, err := os.Stat(pathname)
		require.NoError(t, err)
		files[pathname] = fi
	}

	return files
}

func archiveChecksum(
	t *testing.T,
	newArchiver newArchiverFunc,
	level archive.CompressionLevel,
	dir string,
	files map[string]os.FileInfo,
) [sha256.Size]byte {
	t.Helper()

	var buf bytes.Buffer
	archiver, err := newArchiver(&buf, dir, level)
	require.NoError(t, err)
	require.NoError(t, archiver.Archive(t.Context(), files))

	return sha256.Sum256(buf.Bytes())
}
