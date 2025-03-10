//go:build !integration

package archives

import (
	"bytes"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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

func TestGzipSanitization(t *testing.T) {
	tests := []struct {
		name          string
		dir           string
		file          string
		content       []byte
		needsEncoding bool
	}{
		{
			name:          "ASCII only",
			dir:           "test",
			file:          "file.txt",
			content:       []byte("content"),
			needsEncoding: false,
		},
		{
			name:          "with non-ASCII characters",
			dir:           "测试",
			file:          "file.txt",
			content:       []byte("コンテンツ"),
			needsEncoding: true,
		},
		{
			name:          "with percent sign",
			dir:           "test%dir",
			file:          "file.txt",
			content:       []byte("content"),
			needsEncoding: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, tt.dir)

			err := os.MkdirAll(path, 0o755)
			require.NoError(t, err)

			filePath := filepath.Join(path, tt.file)
			err = os.WriteFile(filePath, tt.content, 0o644)
			require.NoError(t, err)

			var buffer bytes.Buffer
			err = CreateGzipArchive(&buffer, []string{filePath})
			require.NoError(t, err)

			gz, err := gzip.NewReader(&buffer)
			require.NoError(t, err)
			defer gz.Close()

			if tt.needsEncoding {
				require.True(t, strings.HasPrefix(gz.Header.Comment, "e:"))

				decodedPath, err := url.PathUnescape(strings.TrimPrefix(gz.Header.Comment, "e:"))
				require.NoError(t, err)
				require.Equal(t, filePath, decodedPath)
			} else {
				require.Equal(t, filePath, gz.Header.Comment)
			}

			content, err := io.ReadAll(gz)
			require.NoError(t, err)
			require.Equal(t, tt.content, content)
		})
	}
}
