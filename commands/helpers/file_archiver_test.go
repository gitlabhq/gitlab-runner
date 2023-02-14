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

func TestGlobbedFilePath(t *testing.T) {
	// Set up directories used in all test cases
	const (
		fileArchiverGlobPath  = "foo/bar/baz"
		fileArchiverGlobPath2 = "foo/bar/baz2"
	)
	err := os.MkdirAll(fileArchiverGlobPath, 0700)
	require.NoError(t, err, "Creating directory path: %s", fileArchiverGlobPath)
	defer os.RemoveAll(strings.Split(fileArchiverGlobPath, "/")[0])

	err = os.MkdirAll(fileArchiverGlobPath2, 0700)
	require.NoError(t, err, "Creating directory path: %s", fileArchiverGlobPath2)
	defer os.RemoveAll(strings.Split(fileArchiverGlobPath2, "/")[0])

	// Write a dir that is outside any glob patterns
	const (
		fileArchiverGlobNonMatchingPath = "bar/foo"
	)
	err = os.MkdirAll(fileArchiverGlobNonMatchingPath, 0700)
	writeTestFile(t, "bar/foo/test.txt")
	require.NoError(t, err, "Creating directory path: %s", fileArchiverGlobNonMatchingPath)
	defer os.RemoveAll(strings.Split(fileArchiverGlobNonMatchingPath, "/")[0])

	workingDirectory, err := os.Getwd()
	require.NoError(t, err)

	testCases := map[string]struct {
		paths   []string
		exclude []string

		// files that will be created and matched by the patterns
		expectedMatchingFiles []string

		// directories that will be matched by the patterns
		expectedMatchingDirs []string

		// files that will be created but will not be matched
		nonMatchingFiles []string

		// directories that will not be matched by the patterns
		nonMatchingDirs []string

		// files that are excluded by Exclude patterns
		excludedFilesCount int64

		warningLog string
	}{
		"find nothing with empty path": {
			paths: []string{""},
			nonMatchingFiles: []string{
				"file.txt",
				"foo/file.txt",
				"foo/bar/file.txt",
				"foo/bar/baz/file.txt",
				"foo/bar/baz/file.extra.dots.txt",
			},
			warningLog: "No matching files. Path is empty.",
		},
		"files by extension at several depths": {
			paths: []string{"foo/**/*.txt"},
			expectedMatchingFiles: []string{
				"foo/file.txt",
				"foo/bar/file.txt",
				"foo/bar/baz/file.txt",
				"foo/bar/baz/file.extra.dots.txt",
			},
			nonMatchingFiles: []string{
				"foo/file.txt.md",
				"foo/bar/file.txt.md",
				"foo/bar/baz/file.txt.md",
				"foo/bar/baz/file.extra.dots.txt.md",
			},
		},
		"files by extension at several depths - with exclude": {
			paths:   []string{"foo/**/*.txt"},
			exclude: []string{"foo/**/xfile.txt"},
			expectedMatchingFiles: []string{
				"foo/file.txt",
				"foo/bar/file.txt",
				"foo/bar/baz/file.txt",
			},
			nonMatchingFiles: []string{
				"foo/xfile.txt",
				"foo/bar/xfile.txt",
				"foo/bar/baz/xfile.txt",
			},
			excludedFilesCount: 3,
		},
		"double slash matches a single slash": {
			paths: []string{"foo//*.txt"},
			expectedMatchingFiles: []string{
				"foo/file.txt",
			},
			nonMatchingFiles: []string{
				"foo/bar/file.txt",
				"foo/bar/baz/file.txt",
			},
		},
		"double slash matches a single slash - with exclude": {
			paths:   []string{"foo//*.txt"},
			exclude: []string{"foo//*2.txt"},
			expectedMatchingFiles: []string{
				"foo/file.txt",
			},
			nonMatchingFiles: []string{
				"foo/file2.txt",
				"foo/bar/file.txt",
			},
			excludedFilesCount: 1,
		},
		"absolute path to working directory": {
			paths: []string{filepath.Join(workingDirectory, "*.thing")},
			expectedMatchingFiles: []string{
				"file.thing",
			},
			nonMatchingFiles: []string{
				"foo/file.thing",
				"foo/bar/file.thing",
				"foo/bar/baz/file.thing",
			},
		},
		"absolute path to working directory - with exclude": {
			paths:   []string{filepath.Join(workingDirectory, "*.thing")},
			exclude: []string{filepath.Join(workingDirectory, "*2.thing")},
			expectedMatchingFiles: []string{
				"file.thing",
			},
			nonMatchingFiles: []string{
				"file2.thing",
			},
			excludedFilesCount: 1,
		},
		"absolute path to nested directory": {
			paths: []string{filepath.Join(workingDirectory, "foo/bar/*.bin")},
			expectedMatchingFiles: []string{
				"foo/bar/file.bin",
			},
			nonMatchingFiles: []string{
				"foo/bar/file.txt",
				"foo/bar/baz/file.bin",
			},
		},
		"absolute path to nested directory - with exclude": {
			paths:   []string{filepath.Join(workingDirectory, "foo/bar/*.bin")},
			exclude: []string{filepath.Join(workingDirectory, "foo/bar/*2.bin")},
			expectedMatchingFiles: []string{
				"foo/bar/file.bin",
			},
			nonMatchingFiles: []string{
				"foo/bar/file2.bin",
				"foo/bar/file.txt",
				"foo/bar/baz/file.bin",
			},
			excludedFilesCount: 1,
		},
		"double slash and multiple stars - must be at least two dirs deep": {
			paths: []string{"./foo/**//*/*.*"},
			expectedMatchingFiles: []string{
				"foo/bar/file.bin",
				"foo/bar/file.txt",
				"foo/bar/baz/file.bin",
				"foo/bar/baz/file.txt",
				"foo/bar/baz2/file.bin",
				"foo/bar/baz2/file.txt",
			},
			nonMatchingFiles: []string{
				"foo/file.txt",
			},
		},
		"double slash and multiple stars - must be at least two dirs deep - with exclude": {
			paths:   []string{"./foo/**//*/*.*"},
			exclude: []string{"**/*.bin"},
			expectedMatchingFiles: []string{
				"foo/bar/file.txt",
				"foo/bar/baz/file.txt",
				"foo/bar/baz2/file.txt",
			},
			nonMatchingFiles: []string{
				"foo/file.txt",
				"foo/bar/file.bin",
				"foo/bar/baz/file.bin",
				"foo/bar/baz2/file.bin",
			},
			excludedFilesCount: 3,
		},
		"all the files": {
			paths: []string{"foo/**/*.*"},
			expectedMatchingFiles: []string{
				"foo/file.bin",
				"foo/file.txt",
				"foo/bar/file.bin",
				"foo/bar/file.txt",
				"foo/bar/baz/file.bin",
				"foo/bar/baz/file.txt",
				"foo/bar/baz2/file.bin",
				"foo/bar/baz2/file.txt",
			},
			nonMatchingFiles: []string{},
		},
		"all the files - with exclude": {
			paths:   []string{"foo/**/*.*"},
			exclude: []string{"**/*.bin", "**/*even-this*"},
			expectedMatchingFiles: []string{
				"foo/file.txt",
				"foo/bar/file.txt",
				"foo/bar/baz/file.txt",
				"foo/bar/baz2/file.txt",
			},
			nonMatchingFiles: []string{
				"foo/wow-even-this.go",
				"foo/file.bin",
				"foo/bar/file.bin",
				"foo/bar/baz/file.bin",
				"foo/bar/baz2/file.bin",
			},
			excludedFilesCount: 5,
		},
		"all the things - dirs included": {
			paths: []string{"foo/**"},
			expectedMatchingFiles: []string{
				"foo/file.bin",
				"foo/file.txt",
				"foo/bar/file.bin",
				"foo/bar/file.txt",
				"foo/bar/baz/file.bin",
				"foo/bar/baz/file.txt",
				"foo/bar/baz2/file.bin",
				"foo/bar/baz2/file.txt",
			},
			expectedMatchingDirs: []string{
				"foo",
				"foo/bar",
				"foo/bar/baz",
				"foo/bar/baz2",
			},
			nonMatchingFiles: []string{
				"root.txt",
			},
		},
		"relative path that leaves project and returns": {
			paths: []string{filepath.Join("..", filepath.Base(workingDirectory), "foo/*.txt")},
			expectedMatchingFiles: []string{
				"foo/file.txt",
			},
		},
		"relative path that leaves project and returns - with exclude": {
			paths:   []string{filepath.Join("..", filepath.Base(workingDirectory), "foo/*.txt")},
			exclude: []string{filepath.Join("..", filepath.Base(workingDirectory), "foo/*2.txt")},
			expectedMatchingFiles: []string{
				"foo/file.txt",
			},
			nonMatchingFiles: []string{
				"foo/file2.txt",
			},
			excludedFilesCount: 1,
		},
		"invalid path": {
			paths:      []string{">/**"},
			warningLog: "no matching files. Ensure that the artifact path is relative to the working directory",
		},
		"cancel out everything": {
			paths:      []string{"**"},
			exclude:    []string{"**"},
			warningLog: "no matching files. Ensure that the artifact path is relative to the working directory",
		},
	}

	for testName, tc := range testCases {
		t.Run(testName, func(t *testing.T) {
			h := newLogHook(logrus.WarnLevel)
			logrus.AddHook(&h)

			for _, f := range tc.expectedMatchingFiles {
				writeTestFile(t, f)
			}
			for _, f := range tc.nonMatchingFiles {
				writeTestFile(t, f)
			}

			f := fileArchiver{
				Paths:   tc.paths,
				Exclude: tc.exclude,
			}
			err = f.enumerate()
			assert.NoError(t, err)

			sortedFiles := f.sortedFiles()
			assert.Len(t, sortedFiles, len(tc.expectedMatchingFiles)+len(tc.expectedMatchingDirs))
			for _, p := range tc.expectedMatchingFiles {
				assert.Contains(t, f.sortedFiles(), p)
			}
			for _, p := range tc.expectedMatchingDirs {
				assert.Contains(t, f.sortedFiles(), p)
			}

			var excludedFilesCount int64
			for _, v := range f.excluded {
				excludedFilesCount += v
			}
			if tc.excludedFilesCount > 0 {
				assert.Equal(t, tc.excludedFilesCount, excludedFilesCount)
			}

			if tc.warningLog != "" {
				require.Len(t, h.entries, 1)
				assert.Contains(t, h.entries[0].Message, tc.warningLog)
			}

			// remove test files from this test case
			// deferred removal will still happen if needed in the os.RemoveAll call above
			for _, f := range tc.expectedMatchingFiles {
				removeTestFile(t, f)
			}
			for _, f := range tc.nonMatchingFiles {
				removeTestFile(t, f)
			}
		})
	}
}

func TestExcludedFilePaths(t *testing.T) {
	const fooTestDirectory = "foo/test/bar/baz"

	err := os.MkdirAll(fooTestDirectory, 0700)
	require.NoError(t, err, "could not create test directory")
	defer os.RemoveAll(strings.Split(fooTestDirectory, "/")[0])

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

func Test_isExcluded(t *testing.T) {
	testCases := map[string]struct {
		pattern string
		path    string
		match   bool
		log     string
	}{
		`direct match`: {
			pattern: "file.txt",
			path:    "file.txt",
			match:   true,
		},
		`pattern matches`: {
			pattern: "**/*.txt",
			path:    "foo/bar/file.txt",
			match:   true,
		},
		`no match - pattern not in project`: {
			pattern: "../*.*",
			path:    "file.txt",
			match:   false,
			log:     "isExcluded: artifact path is not a subpath of project directory: ../*.*",
		},
		`no match - absolute pattern not in project`: {
			pattern: "/foo/file.txt",
			path:    "file.txt",
			match:   false,
			log:     "isExcluded: artifact path is not a subpath of project directory: /foo/file.txt",
		},
	}

	workingDirectory, err := os.Getwd()
	require.NoError(t, err)

	for testName, tc := range testCases {
		t.Run(testName, func(t *testing.T) {
			f := fileArchiver{
				wd:      workingDirectory,
				Exclude: []string{tc.pattern},
			}

			h := newLogHook(logrus.WarnLevel)
			logrus.AddHook(&h)

			isExcluded, rule := f.isExcluded(tc.path)
			assert.Equal(t, tc.match, isExcluded)
			if tc.match {
				assert.Equal(t, tc.pattern, rule)
			} else {
				assert.Empty(t, rule)
			}
			if tc.log != "" {
				require.Len(t, h.entries, 1)
				assert.Contains(t, h.entries[0].Message, tc.log)
			}
		})
	}
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

func TestFileArchiverToSucceedOnAbsoluteFileInProject(t *testing.T) {
	path, err := os.Getwd()
	require.NoError(t, err)
	fpath := filepath.Join(path, "file.txt")
	writeTestFile(t, fpath)
	defer os.Remove(fpath)

	f := fileArchiver{
		Paths: []string{fpath},
	}

	err = f.enumerate()
	assert.NoError(t, err)
	assert.Len(t, f.sortedFiles(), 1)
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
		`empty path not in project`: {
			path:      "",
			inProject: false,
		},
		`relative path in project`: {
			path:      "in/the/project/for/realzy",
			inProject: true,
		},
		`relative path not in project`: {
			path:          "../nope",
			inProject:     false,
			errorExpected: true,
		},
		`relative path to parent directory with pattern - not in project`: {
			path:          "../*.*",
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
		`absolute path to working directory in project`: {
			path:      wd,
			inProject: true,
		},
		`relative path to working directory in project`: {
			path:      filepath.Join("..", filepath.Base(wd)),
			inProject: true,
		},
		`absolute path to working directory in project with trailing slash`: {
			path:      wd + "/",
			inProject: true,
		},
		`relative path to working directory in project with trailing slash`: {
			path:      filepath.Join("..", filepath.Base(wd)) + "/",
			inProject: true,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			_, err := c.findRelativePathInProject(tc.path)
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
