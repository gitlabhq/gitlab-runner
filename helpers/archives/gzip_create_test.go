//go:build !integration

package archives

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	gzip "github.com/klauspost/pgzip"
	"github.com/stretchr/testify/require"
)

var testGzipFileContent = []byte("test content")

func testGzipStreams(t *testing.T, r io.Reader, streams [][]byte) {
	gz, err := gzip.NewReader(r)
	if err == io.EOF && len(streams) == 0 {
		return
	}
	require.NoError(t, err)
	defer gz.Close()

	stream := 0
	for ; stream < len(streams); stream++ {
		if stream > 0 {
			err := gz.Reset(r)
			require.NoError(t, err, "stream should have another gzip archive")
		}

		gz.Multistream(false)

		readed, err := io.ReadAll(gz)
		require.NoError(t, err, "gzip archive should be uncompressed")
		require.Equal(t, readed, streams[stream], "gzip archive should equal content")
	}

	if gz.Reset(r) != io.EOF {
		t.Fatal("gzip stream should end")
	}
}

func TestGzipArchiveOfMultipleFiles(t *testing.T) {
	file, err := os.CreateTemp("", "test_file")
	require.NoError(t, err)
	defer file.Close()
	defer os.Remove(file.Name())

	_, err = file.Write(testZipFileContent)
	require.NoError(t, err)
	file.Close()

	var buffer bytes.Buffer
	err = CreateGzipArchive(&buffer, []string{file.Name(), file.Name()})
	require.NoError(t, err)

	testGzipStreams(t, &buffer, [][]byte{testGzipFileContent, testGzipFileContent})
}

func TestGzipArchivingShouldFailIfDirectoryIsBeingArchived(t *testing.T) {
	dir := t.TempDir()

	var buffer bytes.Buffer
	err := CreateGzipArchive(&buffer, []string{dir})
	require.Errorf(t, err, "the %q is not a regular file", dir)
}

func TestGzipArchivingShouldFailIfSymlinkIsBeingArchived(t *testing.T) {
	dir := t.TempDir()

	filePath := filepath.Join(dir, "file")
	err := os.WriteFile(filePath, testGzipFileContent, 0o644)
	require.NoError(t, err)

	symlinkPath := filepath.Join(dir, "symlink")
	err = os.Symlink(filePath, symlinkPath)
	require.NoError(t, err)

	var buffer bytes.Buffer
	err = CreateGzipArchive(&buffer, []string{filePath, symlinkPath})
	require.Errorf(t, err, "the %q is not a regular file", symlinkPath)
}

func TestGzipDoesNotArchiveNonExistingFile(t *testing.T) {
	var buffer bytes.Buffer
	err := CreateGzipArchive(&buffer, []string{"non-existing-file"})
	require.NoError(t, err)

	// test that we have empty number of streams
	testGzipStreams(t, &buffer, [][]byte{})
}

func TestGzipArchivesExistingAndNonExistingFile(t *testing.T) {
	dir := t.TempDir()

	filePath := filepath.Join(dir, "file")
	err := os.WriteFile(filePath, testGzipFileContent, 0o644)
	require.NoError(t, err)

	var buffer bytes.Buffer
	err = CreateGzipArchive(&buffer, []string{filePath, "non-existing-file"})
	require.NoError(t, err)

	// we have only one stream
	testGzipStreams(t, &buffer, [][]byte{testGzipFileContent})
}
