//go:build !integration

package logrotate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/usage_log"
)

func TestWriter_Store_WithRotation(t *testing.T) {
	dir := t.TempDir()

	w := New(
		WithLogDirectory(dir),
		WithMaxRotationAge(5*time.Millisecond),
	)
	defer func() {
		err := w.Close()
		assert.NoError(t, err)
	}()

	// Store first record
	record1 := testRecord()
	err := w.Store(record1)
	require.NoError(t, err)

	// Wait for rotation age to pass
	time.Sleep(10 * time.Millisecond)

	// Store second record - should trigger rotation
	record2 := testRecord()
	err = w.Store(record2)
	require.NoError(t, err)

	// Should have 2 files now
	files := w.allLogFiles()
	assert.Len(t, files, 2)
}

func TestWriter_Store_WithCleanup(t *testing.T) {
	dir := t.TempDir()

	w := New(
		WithLogDirectory(dir),
		WithMaxBackupFiles(2),
		WithMaxRotationAge(1*time.Millisecond),
	)
	defer func() {
		err := w.Close()
		assert.NoError(t, err)
	}()

	// Store multiple records with rotation to create multiple files
	for i := 0; i < 4; i++ {
		record := testRecord()
		err := w.Store(record)
		require.NoError(t, err)

		// Wait for rotation age to pass
		time.Sleep(5 * time.Millisecond)
	}

	// Should only keep MaxBackupFiles (2) files
	files := w.allLogFiles()
	assert.LessOrEqual(t, len(files), 3) // Current file + 2 backups
}

func TestWriter_Store(t *testing.T) {
	dir := t.TempDir()

	w := New(WithLogDirectory(dir))
	defer func() {
		err := w.Close()
		assert.NoError(t, err)
	}()

	record := testRecord()
	err := w.Store(record)
	assert.NoError(t, err)

	files := w.allLogFiles()
	require.Len(t, files, 1)

	// Verify the file contains valid JSON
	data, err := os.ReadFile(filepath.Join(dir, files[0].name))
	require.NoError(t, err)

	// Should have newline at end
	assert.True(t, strings.HasSuffix(string(data), "\n"))
}

func TestWriter_Store_ClosedStorage(t *testing.T) {
	dir := t.TempDir()

	w := New(WithLogDirectory(dir))

	// Close the storage
	err := w.Close()
	require.NoError(t, err)

	// Try to store after close
	record := testRecord()
	err = w.Store(record)
	assert.ErrorIs(t, err, usage_log.ErrStorageIsClosed)
}

func TestWriter_Close_Idempotent(t *testing.T) {
	dir := t.TempDir()

	w := New(WithLogDirectory(dir))

	err := w.Close()
	assert.NoError(t, err)

	// Second close should be safe
	err = w.Close()
	assert.NoError(t, err)
}

func testRecord() usage_log.Record {
	return usage_log.Record{
		UUID:      "test-uuid",
		Timestamp: time.Now(),
		Runner: usage_log.Runner{
			ID:       "runner-id",
			Name:     "runner-name",
			SystemID: "system-id",
		},
		Job: usage_log.Job{
			URL:             "https://example.com/job/1",
			DurationSeconds: 123.45,
			Status:          "success",
		},
		Labels: map[string]string{
			"test-label": "test-value",
		},
	}
}
