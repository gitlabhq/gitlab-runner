//go:build !integration

package archives

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createDefaultArchive(t *testing.T, archive *zip.Writer) {
	testFile, err := archive.Create("temporary_file.txt")
	require.NoError(t, err)
	_, err = io.WriteString(testFile, "test file")
	require.NoError(t, err)
}

func createArchiveWithGitPath(t *testing.T, archive *zip.Writer) {
	testGitFile, err := archive.Create(".git/test_file")
	require.NoError(t, err)
	_, err = io.WriteString(testGitFile, "test git file")
	require.NoError(t, err)
}

func testOnArchive(
	t *testing.T,
	createArchive func(t *testing.T, archive *zip.Writer),
	testCase func(t *testing.T, fileName string),
) {
	tempFile, err := os.CreateTemp("", "archive")
	require.NoError(t, err)
	defer tempFile.Close()
	defer os.Remove(tempFile.Name())

	archive := zip.NewWriter(tempFile)
	defer archive.Close()

	createArchive(t, archive)
	archive.Close()
	tempFile.Close()

	testCase(t, tempFile.Name())
}

func TestExtractZipFile(t *testing.T) {
	testOnArchive(t, createDefaultArchive, func(t *testing.T, fileName string) {
		err := ExtractZipFile(fileName)
		require.NoError(t, err)

		stat, err := os.Stat("temporary_file.txt")
		assert.False(t, os.IsNotExist(err), "Expected temporary_file.txt to exist")
		if !os.IsNotExist(err) {
			assert.NoError(t, err)
		}

		if stat != nil {
			defer os.Remove("temporary_file.txt")
			assert.Equal(t, int64(9), stat.Size())
		}
	})
}

func TestExtractZipFileWithGitPath(t *testing.T) {
	testOnArchive(t, createArchiveWithGitPath, func(t *testing.T, fileName string) {
		output := logrus.StandardLogger().Out
		var buf bytes.Buffer
		logrus.SetOutput(&buf)
		defer logrus.SetOutput(output)

		err := ExtractZipFile(fileName)
		require.NoError(t, err)

		assert.Contains(t, buf.String(), "Part of .git directory is on the list of files to extract")

		stat, err := os.Stat(".git/test_file")
		assert.False(t, os.IsNotExist(err), "Expected .git/test_file to exist")
		if !os.IsNotExist(err) {
			assert.NoError(t, err)
		}

		if stat != nil {
			defer os.Remove(".git/test_file")
			assert.Equal(t, int64(13), stat.Size())
		}
	})
}

func TestExtractZipFileNotFound(t *testing.T) {
	err := ExtractZipFile("non_existing_zip_file.zip")
	assert.Error(t, err)
}
