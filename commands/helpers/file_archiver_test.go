package helpers

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fileArchiverUntrackedFile = "untracked_test_file.txt"
const fileArchiverArchiveZipFile = "archive.zip"
const fileArchiverOtherFile = "other_test_file.txt"
const fileArchiverNotExistingFile = "not_existing_file.txt"
const fileArchiverAbsoluteFile = "/absolute.txt"
const fileArchiverRelativeFile = "../../../relative.txt"

func TestCacheArchiverAddingUntrackedFiles(t *testing.T) {
	ioutil.WriteFile(fileArchiverUntrackedFile, nil, 0600)
	defer os.Remove(fileArchiverUntrackedFile)

	f := fileArchiver{
		Untracked: true,
	}
	err := f.enumerate()
	assert.NoError(t, err)
	assert.Len(t, f.sortedFiles(), 1)
	assert.Contains(t, f.sortedFiles(), fileArchiverUntrackedFile)
}

func TestCacheArchiverAddingFile(t *testing.T) {
	ioutil.WriteFile(fileArchiverUntrackedFile, nil, 0600)
	defer os.Remove(fileArchiverUntrackedFile)

	f := fileArchiver{
		Paths: []string{fileArchiverUntrackedFile},
	}
	err := f.enumerate()
	assert.NoError(t, err)
	assert.Len(t, f.sortedFiles(), 1)
	assert.Contains(t, f.sortedFiles(), fileArchiverUntrackedFile)
}

func TestFileArchiverToFailOnAbsoulteFile(t *testing.T) {
	f := fileArchiver{
		Paths: []string{fileArchiverAbsoluteFile},
	}
	err := f.enumerate()
	assert.NoError(t, err)
	assert.Empty(t, f.sortedFiles())
	assert.NotContains(t, f.sortedFiles(), fileArchiverAbsoluteFile)
}

func TestFileArchiverToFailOnRelativeFile(t *testing.T) {
	f := fileArchiver{
		Paths: []string{fileArchiverRelativeFile},
	}
	err := f.enumerate()
	assert.NoError(t, err)
	assert.Empty(t, f.sortedFiles())
}

func TestFileArchiverToAddNotExistingFile(t *testing.T) {
	f := fileArchiver{
		Paths: []string{fileArchiverNotExistingFile},
	}
	err := f.enumerate()
	assert.NoError(t, err)
	assert.Empty(t, f.sortedFiles())
}

func TestFileArchiverChanged(t *testing.T) {
	ioutil.WriteFile(fileArchiverUntrackedFile, nil, 0600)
	defer os.Remove(fileArchiverUntrackedFile)

	now := time.Now()
	require.NoError(t, os.Chtimes(fileArchiverUntrackedFile, now, now.Add(-time.Second)))

	f := fileArchiver{
		Paths: []string{fileArchiverUntrackedFile},
	}
	err := f.enumerate()
	require.NoError(t, err)
	assert.Len(t, f.sortedFiles(), 1)
	assert.False(t, f.isChanged(now.Add(time.Minute)))
	assert.True(t, f.isChanged(now.Add(-time.Minute)))
}

func TestFileArchiverFileIsNotChanged(t *testing.T) {
	now := time.Now()

	ioutil.WriteFile(fileArchiverUntrackedFile, nil, 0600)
	defer os.Remove(fileArchiverUntrackedFile)

	ioutil.WriteFile(fileArchiverArchiveZipFile, nil, 0600)
	defer os.Remove(fileArchiverArchiveZipFile)

	f := fileArchiver{
		Paths: []string{fileArchiverUntrackedFile},
	}
	err := f.enumerate()
	require.NoError(t, err)

	require.NoError(t, os.Chtimes(fileArchiverUntrackedFile, now, now.Add(-time.Second)))
	assert.False(t, f.isFileChanged(fileArchiverArchiveZipFile), "should return false if file was modified before the listed file")
}

func TestFileArchiverFileIsChanged(t *testing.T) {
	now := time.Now()

	ioutil.WriteFile(fileArchiverUntrackedFile, nil, 0600)
	defer os.Remove(fileArchiverUntrackedFile)

	ioutil.WriteFile(fileArchiverArchiveZipFile, nil, 0600)
	defer os.Remove(fileArchiverArchiveZipFile)

	f := fileArchiver{
		Paths: []string{fileArchiverUntrackedFile},
	}
	err := f.enumerate()
	require.NoError(t, err)

	require.NoError(t, os.Chtimes(fileArchiverArchiveZipFile, now, now.Add(-time.Minute)))
	assert.True(t, f.isFileChanged(fileArchiverArchiveZipFile), "should return true if file was modified")
}

func TestFileArchiverFileDoesNotExist(t *testing.T) {
	ioutil.WriteFile(fileArchiverUntrackedFile, nil, 0600)
	defer os.Remove(fileArchiverUntrackedFile)

	f := fileArchiver{
		Paths: []string{fileArchiverUntrackedFile},
	}
	err := f.enumerate()
	require.NoError(t, err)

	assert.True(t, f.isFileChanged(fileArchiverNotExistingFile), "should return true if file doesn't exist")
}
