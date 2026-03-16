//go:build !windows && !integration

package store

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenFilePermissions(t *testing.T) {
	t.Run("new file gets 0666 regardless of umask", func(t *testing.T) {
		oldUmask := syscall.Umask(0077)
		defer syscall.Umask(oldUmask)

		dir := t.TempDir()

		db, err := Open(dir)
		require.NoError(t, err)
		defer db.Close()

		info, err := os.Stat(filepath.Join(dir, "masking.db"))
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0666), info.Mode().Perm())
	})

	t.Run("existing file permissions unchanged on reopen", func(t *testing.T) {
		dir := t.TempDir()

		db, err := Open(dir)
		require.NoError(t, err)
		db.Close()

		dbPath := filepath.Join(dir, "masking.db")
		require.NoError(t, os.Chmod(dbPath, 0600))

		db, err = Open(dir)
		require.NoError(t, err)
		defer db.Close()

		info, err := os.Stat(dbPath)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0600), info.Mode().Perm())
	})
}
