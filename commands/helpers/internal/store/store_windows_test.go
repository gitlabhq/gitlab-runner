//go:build windows && !integration

package store

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeleteOpenFile(t *testing.T) {
	dir := t.TempDir()

	pathname := filepath.Join(dir, "masking.db")
	sum := sha256.Sum256([]byte(pathname))
	keyPath := filepath.Join(dir, "runner"+hex.EncodeToString(sum[:]))

	require.NoError(t, os.WriteFile(keyPath, nil, 0o640))
	db, err := Open(dir)
	defer db.Close()
	require.NoError(t, err)

	require.NoError(t, os.Remove(pathname))
}
