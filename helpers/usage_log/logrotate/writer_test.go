//go:build !integration

package logrotate

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriter_Write(t *testing.T) {
	const (
		line1 = "Line 1"
		line2 = "Line 2"
	)
	dir := t.TempDir()

	w := New(
		WithLogDirectory(dir),
		WithMaxRotationAge(5*time.Millisecond),
	)
	defer func() {
		err := w.Close()
		assert.NoError(t, err)
	}()

	assert.Empty(t, w.allLogFiles())

	_, err := fmt.Fprintln(w, line1)
	assert.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	_, err = fmt.Fprintln(w, line2)
	assert.NoError(t, err)

	logFiles := w.allLogFiles()
	w.timesortLogFiles(logFiles)
	require.Len(t, logFiles, 2)

	data1, err := os.ReadFile(filepath.Join(dir, logFiles[1].name))
	assert.Equal(t, line1+"\n", string(data1))
	assert.NoError(t, err)

	data2, err := os.ReadFile(filepath.Join(dir, logFiles[0].name))
	assert.Equal(t, line2+"\n", string(data2))
	assert.NoError(t, err)
}

func TestWriter_Write_concurrent(t *testing.T) {
	const (
		loopsNum       = 5
		loopIterations = 100
	)

	dir := t.TempDir()

	w := New(
		WithLogDirectory(dir),
	)
	defer func() {
		err := w.Close()
		assert.NoError(t, err)
	}()

	writeLoop := func(t *testing.T, wg *sync.WaitGroup, w io.Writer, id int) {
		defer wg.Done()
		for i := 0; i < loopIterations; i++ {
			_, err := fmt.Fprintf(w, "test %d-%d\n", id, i)
			assert.NoError(t, err)
		}
	}

	wg := new(sync.WaitGroup)
	wg.Add(loopsNum)

	for i := 0; i < loopsNum; i++ {
		go writeLoop(t, wg, w, i)
	}

	wg.Wait()

	require.Len(t, w.allLogFiles(), 1)
	path := filepath.Join(dir, w.allLogFiles()[0].name)
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	assert.Len(t, strings.Split(strings.TrimSpace(string(data)), "\n"), loopsNum*loopIterations)
}

func TestWriter_rotate_maxRotationAgeLimitation(t *testing.T) {
	dir := t.TempDir()

	w := New(
		WithLogDirectory(dir),
		WithMaxRotationAge(24*time.Hour),
	)
	defer func() {
		err := w.Close()
		assert.NoError(t, err)
	}()

	dirEntries, err := os.ReadDir(dir)
	assert.NoError(t, err)
	assert.Len(t, dirEntries, 0)

	require.NoError(t, w.reCreateFile())
	require.NoError(t, w.rotate())
	require.NoError(t, w.rotate())
	require.NoError(t, w.rotate())
	require.NoError(t, w.rotate())

	dirEntries, err = os.ReadDir(dir)
	assert.NoError(t, err)
	assert.Len(t, dirEntries, 1)

	time.Sleep(3 * time.Millisecond)
	w.options.MaxRotationAge = 1 * time.Millisecond
	require.NoError(t, w.rotate())

	dirEntries, err = os.ReadDir(dir)
	assert.NoError(t, err)
	assert.Len(t, dirEntries, 2)
}

func TestWriter_cleanup(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, os.Mkdir(filepath.Join(dir, "test-1"), 0755))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "test-2"), 0755))
	createTestFile(t, dir, "test-3")
	createTestFile(t, dir, "test-4")

	now := time.Now()
	createTestFile(t, dir, now.Add(10*time.Millisecond).Format(fileNameFormat))
	createTestFile(t, dir, now.Add(20*time.Millisecond).Format(fileNameFormat))
	createTestFile(t, dir, now.Add(30*time.Millisecond).Format(fileNameFormat))

	w := New(
		WithLogDirectory(dir),
		WithMaxBackupFiles(2),
	)
	defer func() {
		err := w.Close()
		assert.NoError(t, err)
	}()

	before, err := os.ReadDir(dir)
	require.NoError(t, err)

	w.cleanup()

	after, err := os.ReadDir(dir)
	require.NoError(t, err)

	assert.Len(t, diffDirEntries(before, after), 1)
}

func createTestFile(t *testing.T, dir string, name string) {
	f, err := os.Create(filepath.Join(dir, name))
	require.NoError(t, err)
	require.NoError(t, f.Close())
}

func diffDirEntries(entriesA []os.DirEntry, entriesB []os.DirEntry) []os.DirEntry {
	var c []os.DirEntry
	for _, a := range entriesA {
		found := false
		for _, b := range entriesB {
			if a.Name() == b.Name() {
				found = true
			}
		}
		if !found {
			c = append(c, a)
		}
	}

	return c
}
