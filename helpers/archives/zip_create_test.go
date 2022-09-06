//go:build !integration

package archives

import (
	"archive/zip"
	"bytes"
	"os"
	"runtime"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testZipFileContent = []byte("test content")

type charsetByte int

const (
	singleByte charsetByte = iota
	multiBytes
)

func createTestFile(t *testing.T, csb charsetByte) string {
	name := "test_file.txt"
	if csb == multiBytes {
		name = "テストファイル.txt"
	}

	err := os.WriteFile(name, testZipFileContent, 0o640)
	assert.NoError(t, err)
	return name
}

func createSymlinkFile(t *testing.T, csb charsetByte) string {
	name := "new_symlink"
	if csb == multiBytes {
		name = "新しいシンボリックリンク"
	}

	err := os.Symlink("old_symlink", name)
	assert.NoError(t, err)
	return name
}

func createTestDirectory(t *testing.T, csb charsetByte) string {
	name := "test_directory"
	if csb == multiBytes {
		name = "テストディレクトリ"
	}

	err := os.Mkdir(name, 0o711)
	assert.NoError(t, err)
	return name
}

func createTestGitPathFile(t *testing.T, csb charsetByte) string {
	_, err := os.Stat(".git")
	if err != nil {
		err = os.Mkdir(".git", 0711)
		assert.NoError(t, err)
	}

	name := ".git/test_file"
	if csb == multiBytes {
		name = ".git/テストファイル"
	}

	err = os.WriteFile(name, testZipFileContent, 0o640)
	assert.NoError(t, err)

	return name
}

func testInWorkDir(t *testing.T, testCase func(t *testing.T, fileName string)) {
	wd, err := os.Getwd()
	assert.NoError(t, err)
	defer func() { _ = os.Chdir(wd) }()

	td := t.TempDir()

	err = os.Chdir(td)
	assert.NoError(t, err)

	tempFile, err := os.CreateTemp("", "archive")
	require.NoError(t, err)
	tempFile.Close()
	defer os.Remove(tempFile.Name())

	testCase(t, tempFile.Name())
}

func TestZipCreate(t *testing.T) {
	testInWorkDir(t, func(t *testing.T, fileName string) {
		paths := []string{
			createTestFile(t, singleByte),
			createSymlinkFile(t, singleByte),
			createTestDirectory(t, singleByte),
			createTestFile(t, multiBytes),
			createSymlinkFile(t, multiBytes),
			createTestDirectory(t, multiBytes),
			"non_existing_file.txt",
		}

		// only check how pipes are handled on unix
		if runtime.GOOS != "windows" {
			paths = append(
				paths,
				createTestPipe(t, singleByte),
				createTestPipe(t, multiBytes),
			)
		}

		expectedMode := os.FileMode(0640)
		if runtime.GOOS == "windows" {
			// windows doesn't support the same permissions as Linux
			expectedMode = 0666
		}

		f, err := os.Create(fileName)
		require.NoError(t, err)
		defer f.Close()

		err = CreateZipArchive(f, paths)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		archive, err := zip.OpenReader(fileName)
		require.NoError(t, err)
		defer archive.Close()

		assert.Len(t, archive.File, 6)

		assert.Equal(t, paths[0], archive.File[0].Name)
		assert.Equal(t, expectedMode, archive.File[0].Mode().Perm())
		assert.NotEmpty(t, archive.File[0].Extra)

		assert.Equal(t, paths[1], archive.File[1].Name)

		assert.Equal(t, paths[2]+"/", archive.File[2].Name)
		assert.NotEmpty(t, archive.File[2].Extra)
		assert.True(t, archive.File[2].Mode().IsDir())

		assert.Equal(t, paths[3], archive.File[3].Name)
		assert.Equal(t, expectedMode, archive.File[3].Mode().Perm())
		assert.NotEmpty(t, archive.File[3].Extra)

		assert.Equal(t, paths[4], archive.File[4].Name)

		assert.Equal(t, paths[5]+"/", archive.File[5].Name)
		assert.NotEmpty(t, archive.File[5].Extra)
		assert.True(t, archive.File[5].Mode().IsDir())
	})
}

func TestZipCreateWithGitPath(t *testing.T) {
	testInWorkDir(t, func(t *testing.T, fileName string) {
		output := logrus.StandardLogger().Out
		var buf bytes.Buffer
		logrus.SetOutput(&buf)
		defer logrus.SetOutput(output)

		paths := []string{
			createTestGitPathFile(t, singleByte),
			createTestGitPathFile(t, multiBytes),
		}

		expectedMode := os.FileMode(0640)
		if runtime.GOOS == "windows" {
			// windows doesn't support the same permissions as Linux
			expectedMode = 0666
		}

		f, err := os.Create(fileName)
		require.NoError(t, err)
		defer f.Close()

		err = CreateZipArchive(f, paths)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		assert.Contains(t, buf.String(), "Part of .git directory is on the list of files to archive")

		archive, err := zip.OpenReader(fileName)
		require.NoError(t, err)
		defer archive.Close()

		assert.Len(t, archive.File, 2)

		assert.Equal(t, paths[0], archive.File[0].Name)
		assert.Equal(t, expectedMode, archive.File[0].Mode().Perm())
		assert.NotEmpty(t, archive.File[0].Extra)

		assert.Equal(t, paths[1], archive.File[1].Name)
		assert.Equal(t, expectedMode, archive.File[1].Mode().Perm())
		assert.NotEmpty(t, archive.File[1].Extra)
	})
}
