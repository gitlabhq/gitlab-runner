//go:build !integration

package helpers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	fileArchiverUntrackedFile          = "untracked_test_file.txt"
	fileArchiverArchiveZipFile         = "archive.zip"
	fileArchiverNotExistingFile        = "not_existing_file.txt"
	fileArchiverAbsoluteFile           = "/absolute.txt"
	fileArchiverAbsoluteDoubleStarFile = "/**/absolute.txt"
	fileArchiverRelativeFile           = "../../../relative.txt"
)

func TestGlobbedFilePaths(t *testing.T) {
	const (
		fileArchiverGlobbedFilePath = "foo/**/*.txt"
		fileArchiverGlobPath        = "foo/bar/baz"
	)

	err := os.MkdirAll(fileArchiverGlobPath, 0700)
	require.NoError(t, err, "Creating directory path: %s", fileArchiverGlobPath)
	defer os.RemoveAll(strings.Split(fileArchiverGlobPath, "/")[0])

	expectedMatchingFiles := []string{
		"foo/bar/baz/glob1.txt",
		"foo/bar/baz/glob2.txt",
		"foo/bar/glob3.txt",
	}
	for _, f := range expectedMatchingFiles {
		writeTestFile(t, f)
	}

	// Write a file that doesn't match glob
	writeTestFile(t, "foo/bar/baz/main.go")

	// Write a dir that is outside of glob pattern
	const (
		fileArchiverGlobNonMatchingPath = "bar/foo"
	)
	err = os.MkdirAll(fileArchiverGlobNonMatchingPath, 0700)
	writeTestFile(t, "bar/foo/test.txt")
	require.NoError(t, err, "Creating directory path: %s", fileArchiverGlobNonMatchingPath)
	defer os.RemoveAll(strings.Split(fileArchiverGlobNonMatchingPath, "/")[0])

	f := fileArchiver{
		Paths: []string{fileArchiverGlobbedFilePath},
	}
	err = f.enumerate()
	assert.NoError(t, err)
	assert.Equal(t, expectedMatchingFiles, f.sortedFiles())
}

func TestExcludedFilePaths(t *testing.T) {
	fooTestDirectory := "foo/test/bar/baz"

	err := os.MkdirAll(fooTestDirectory, 0700)
	require.NoError(t, err, "could not create test directory")
	defer os.RemoveAll(fooTestDirectory)

	existingFiles := []string{
		"foo/test/bar/baz/1.txt",
		"foo/test/bar/baz/1.md",
		"foo/test/bar/baz/2.txt",
		"foo/test/bar/baz/2.md",
		"foo/test/bar/baz/3.txt",
	}
	for _, f := range existingFiles {
		writeTestFile(t, f)
	}

	f := fileArchiver{
		Paths:   []string{"foo/test/"},
		Exclude: []string{"foo/test/bar/baz/3.txt", "foo/**/*.md"},
	}

	err = f.enumerate()

	includedFiles := []string{
		"foo/test",
		"foo/test/bar",
		"foo/test/bar/baz",
		"foo/test/bar/baz/1.txt",
		"foo/test/bar/baz/2.txt",
	}

	assert.NoError(t, err)
	assert.Equal(t, includedFiles, f.sortedFiles())
	assert.Equal(t, 2, len(f.excluded))
	require.Contains(t, f.excluded, "foo/test/bar/baz/3.txt")
	assert.Equal(t, int64(1), f.excluded["foo/test/bar/baz/3.txt"])
	require.Contains(t, f.excluded, "foo/**/*.md")
	assert.Equal(t, int64(2), f.excluded["foo/**/*.md"])
}

func TestCacheArchiverAddingUntrackedFiles(t *testing.T) {
	writeTestFile(t, artifactsTestArchivedFile)
	defer os.Remove(artifactsTestArchivedFile)

	writeTestFile(t, artifactsTestArchivedFile2)
	defer os.Remove(artifactsTestArchivedFile2)

	f := fileArchiver{
		Untracked: true,
	}
	err := f.enumerate()
	assert.NoError(t, err)
	assert.Len(t, f.sortedFiles(), 2)
	assert.Contains(t, f.sortedFiles(), artifactsTestArchivedFile)
	assert.Contains(t, f.sortedFiles(), artifactsTestArchivedFile2)
}

func TestCacheArchiverAddingUntrackedUnicodeFiles(t *testing.T) {
	const fileArchiverUntrackedUnicodeFile = "неотслеживаемый_тестовый_файл.txt"

	writeTestFile(t, fileArchiverUntrackedUnicodeFile)
	defer os.Remove(fileArchiverUntrackedUnicodeFile)

	f := fileArchiver{
		Untracked: true,
	}
	err := f.enumerate()
	assert.NoError(t, err)
	assert.Len(t, f.sortedFiles(), 1)
	assert.Contains(t, f.sortedFiles(), fileArchiverUntrackedUnicodeFile)
}

func TestCacheArchiverAddingFile(t *testing.T) {
	writeTestFile(t, fileArchiverUntrackedFile)
	defer os.Remove(fileArchiverUntrackedFile)

	f := fileArchiver{
		Paths: []string{fileArchiverUntrackedFile},
	}
	err := f.enumerate()
	assert.NoError(t, err)
	assert.Len(t, f.sortedFiles(), 1)
	assert.Contains(t, f.sortedFiles(), fileArchiverUntrackedFile)
}

func TestFileArchiverToFailOnAbsoluteFile(t *testing.T) {
	f := fileArchiver{
		Paths: []string{fileArchiverAbsoluteFile},
	}

	h := newLogHook(logrus.WarnLevel)
	logrus.AddHook(&h)

	err := f.enumerate()
	assert.NoError(t, err)
	assert.Empty(t, f.sortedFiles())
	require.Len(t, h.entries, 1)
	assert.Contains(t, h.entries[0].Message, "artifact path is not a subpath of project directory")
}

func TestFileArchiverToNotAddFilePathOutsideProjectDirectory(t *testing.T) {
	f := fileArchiver{
		Paths: []string{fileArchiverAbsoluteDoubleStarFile},
	}

	h := newLogHook(logrus.WarnLevel)
	logrus.AddHook(&h)

	err := f.enumerate()
	assert.NoError(t, err)
	assert.Empty(t, f.sortedFiles())
	require.Len(t, h.entries, 1)
	assert.Contains(t, h.entries[0].Message, "artifact path is not a subpath of project directory")
}

func TestFileArchiverToFailOnRelativeFile(t *testing.T) {
	f := fileArchiver{
		Paths: []string{fileArchiverRelativeFile},
	}

	h := newLogHook(logrus.WarnLevel)
	logrus.AddHook(&h)

	err := f.enumerate()
	assert.NoError(t, err)
	assert.Empty(t, f.sortedFiles())
	require.Len(t, h.entries, 1)
	assert.Contains(t, h.entries[0].Message, "artifact path is not a subpath of project directory")
}

func TestFileArchiver_pathIsInProject(t *testing.T) {
	wd, err := os.Getwd()
	assert.NoError(t, err)

	c := &fileArchiver{
		wd: wd,
	}

	testCases := map[string]struct {
		path          string
		inProject     bool
		errorExpected bool
	}{
		`relative path in project`: {
			path:      "in/the/project/for/realzy",
			inProject: true,
		},
		`relative path not in project`: {
			path:          "../nope",
			inProject:     false,
			errorExpected: true,
		},
		`absolute path in project`: {
			path:      filepath.Join(wd, "yo/i/am/in"),
			inProject: true,
		},
		`absolute path not in project`: {
			path:          "/totally/not/in/the/project",
			inProject:     false,
			errorExpected: true,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			err := c.assertPathInProject(tc.path)
			if tc.errorExpected {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
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
	writeTestFile(t, fileArchiverUntrackedFile)
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

	writeTestFile(t, fileArchiverUntrackedFile)
	defer os.Remove(fileArchiverUntrackedFile)

	writeTestFile(t, fileArchiverArchiveZipFile)
	defer os.Remove(fileArchiverArchiveZipFile)

	f := fileArchiver{
		Paths: []string{fileArchiverUntrackedFile},
	}
	err := f.enumerate()
	require.NoError(t, err)

	require.NoError(t, os.Chtimes(fileArchiverUntrackedFile, now, now.Add(-time.Second)))
	assert.False(
		t,
		f.isFileChanged(fileArchiverArchiveZipFile),
		"should return false if file was modified before the listed file",
	)
}

func TestFileArchiverFileIsChanged(t *testing.T) {
	now := time.Now()

	writeTestFile(t, fileArchiverUntrackedFile)
	defer os.Remove(fileArchiverUntrackedFile)

	writeTestFile(t, fileArchiverArchiveZipFile)
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
	writeTestFile(t, fileArchiverUntrackedFile)
	defer os.Remove(fileArchiverUntrackedFile)

	f := fileArchiver{
		Paths: []string{fileArchiverUntrackedFile},
	}
	err := f.enumerate()
	require.NoError(t, err)

	assert.True(
		t,
		f.isFileChanged(fileArchiverNotExistingFile),
		"should return true if file doesn't exist",
	)
}

func newLogHook(levels ...logrus.Level) logHook {
	return logHook{levels: levels}
}

type logHook struct {
	entries []*logrus.Entry
	levels  []logrus.Level
}

func (s *logHook) Levels() []logrus.Level {
	return s.levels
}

func (s *logHook) Fire(entry *logrus.Entry) error {
	s.entries = append(s.entries, entry)
	return nil
}
