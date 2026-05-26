//go:build !integration

package stages

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run/env"
)

func TestCacheArchivePath(t *testing.T) {
	e := &env.Env{
		WorkingDir: "/builds/group/project",
		CacheDir:   "/cache",
	}

	// archivePath uses filepath.Rel, which returns OS-native
	// separators (backslash on Windows). Convert the expected
	// POSIX form so the test passes on both.
	want := filepath.FromSlash("../../../cache/abc123/cache.zip")

	t.Run("CacheArchive returns path relative to WorkingDir", func(t *testing.T) {
		s := CacheArchive{Key: "abc123"}
		assert.Equal(t, want, s.archivePath(e))
	})

	t.Run("CacheExtract returns path relative to WorkingDir", func(t *testing.T) {
		s := CacheExtract{}
		assert.Equal(t, want, s.archivePath(e, "abc123"))
	})
}

// TestCacheArchiveAlternatePath asserts the cache-archiver's
// --alternate-file argument resolves the same way as --file.
func TestCacheArchiveAlternatePath(t *testing.T) {
	e := &env.Env{WorkingDir: "/builds/group/project", CacheDir: "/cache"}

	s := CacheArchive{Key: "primary", AlternateKey: "alt"}
	assert.Equal(t, filepath.FromSlash("../../../cache/primary/cache.zip"), s.archivePath(e))
	assert.Equal(t, filepath.FromSlash("../../../cache/alt/cache.zip"), s.alternateArchivePath(e))
}
