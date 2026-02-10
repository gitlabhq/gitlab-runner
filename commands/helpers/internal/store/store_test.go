//go:build !integration

package store

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpen(t *testing.T) {
	t.Run("create and reopen", func(t *testing.T) {
		dir := t.TempDir()

		db, err := Open(dir)
		require.NoError(t, err)
		require.NoError(t, db.Add("test-secret"))
		db.Close()

		db, err = Open(dir)
		require.NoError(t, err)
		defer db.Close()

		items, err := db.List()
		require.NoError(t, err)
		require.Equal(t, []string{"test-secret"}, items)
	})

	t.Run("recreates key when db missing", func(t *testing.T) {
		dir := t.TempDir()

		db, err := Open(dir)
		require.NoError(t, err)
		require.NoError(t, db.Add("old-secret"))
		db.Close()

		require.NoError(t, os.Remove(filepath.Join(dir, "masking.db")))

		db, err = Open(dir)
		require.NoError(t, err)
		defer db.Close()

		items, err := db.List()
		require.NoError(t, err)
		require.Empty(t, items)
	})

	t.Run("fails with missing key file", func(t *testing.T) {
		dir := t.TempDir()

		db, err := Open(dir)
		require.NoError(t, err)
		db.Close()

		pathname := filepath.Join(dir, "masking.db")
		sum := sha256.Sum256([]byte(pathname))
		keyPath := filepath.Join(dir, "runner"+hex.EncodeToString(sum[:]))
		require.NoError(t, os.Remove(keyPath))

		_, err = Open(dir)
		require.Error(t, err)
	})
}
