package archives

import (
	"archive/zip"
	"bytes"
	"io/ioutil"
	"os"
	"syscall"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testZipFileContent = []byte("test content")

func createTestFile(t *testing.T) string {
	err := ioutil.WriteFile("test_file.txt", testZipFileContent, 0640)
	assert.NoError(t, err)
	return "test_file.txt"
}

func createSymlinkFile(t *testing.T) string {
	err := os.Symlink("old_symlink", "new_symlink")
	assert.NoError(t, err)
	return "new_symlink"
}

func createTestDirectory(t *testing.T) string {
	err := os.Mkdir("test_directory", 0711)
	assert.NoError(t, err)
	return "test_directory"
}

func createTestPipe(t *testing.T) string {
	err := syscall.Mkfifo("test_pipe", 0600)
	assert.NoError(t, err)
	return "test_pipe"
}

func createTestGitPathFile(t *testing.T) string {
	err := os.Mkdir(".git", 0711)
	assert.NoError(t, err)

	err = ioutil.WriteFile(".git/test_file", testZipFileContent, 0640)
	assert.NoError(t, err)

	return ".git/test_file"
}

func testInWorkDir(t *testing.T, testCase func(t *testing.T, fileName string)) {
	wd, err := os.Getwd()
	assert.NoError(t, err)
	defer os.Chdir(wd)

	td, err := ioutil.TempDir("", "zip_create")
	require.NoError(t, err)

	err = os.Chdir(td)
	assert.NoError(t, err)

	tempFile, err := ioutil.TempFile("", "archive")
	require.NoError(t, err)
	tempFile.Close()
	defer os.Remove(tempFile.Name())

	testCase(t, tempFile.Name())
}

func TestZipCreate(t *testing.T) {
	testInWorkDir(t, func(t *testing.T, fileName string) {
		paths := []string{
			createTestFile(t),
			createSymlinkFile(t),
			createTestDirectory(t),
			createTestPipe(t),
			"non_existing_file.txt",
		}
		err := CreateZipFile(fileName, paths)
		require.NoError(t, err)

		archive, err := zip.OpenReader(fileName)
		require.NoError(t, err)
		defer archive.Close()

		assert.Len(t, archive.File, 3)

		assert.Equal(t, "test_file.txt", archive.File[0].Name)
		assert.Equal(t, os.FileMode(0640), archive.File[0].Mode().Perm())
		assert.NotEmpty(t, archive.File[0].Extra)

		assert.Equal(t, "new_symlink", archive.File[1].Name)

		assert.Equal(t, "test_directory/", archive.File[2].Name)
		assert.NotEmpty(t, archive.File[2].Extra)
		assert.True(t, archive.File[2].Mode().IsDir())
	})
}

func TestZipCreateWithGitPath(t *testing.T) {
	testInWorkDir(t, func(t *testing.T, fileName string) {
		output := logrus.StandardLogger().Out
		var buf bytes.Buffer
		logrus.SetOutput(&buf)
		defer logrus.SetOutput(output)

		paths := []string{
			createTestGitPathFile(t),
		}
		err := CreateZipFile(fileName, paths)
		require.NoError(t, err)

		assert.Contains(t, buf.String(), "Part of .git directory is on the list of files to archive")

		archive, err := zip.OpenReader(fileName)
		require.NoError(t, err)
		defer archive.Close()

		assert.Len(t, archive.File, 1)

		assert.Equal(t, ".git/test_file", archive.File[0].Name)
		assert.Equal(t, os.FileMode(0640), archive.File[0].Mode().Perm())
		assert.NotEmpty(t, archive.File[0].Extra)
	})
}
