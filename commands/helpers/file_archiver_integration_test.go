//go:build integration

package helpers_test

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers"
)

func newFileArchiveInitTestApp(file string, paths []string) (*cli.App, *helpers.CacheArchiverCommand) {
	cmd := helpers.NewCacheArchiverCommandForTest(file, paths)
	app := cli.NewApp()
	app.Name = filepath.Base(os.Args[0])
	app.Commands = append(app.Commands, cli.Command{
		Name:   "cache-archiver",
		Action: cmd.Execute,
	})

	return app, &cmd
}

func TestFileArchiver(t *testing.T) {
	var err error

	// Create a temporary directory to hold our project
	parentDir, err := os.Getwd()
	require.NoError(t, err, "Error retrieving working directory")

	dir := filepath.Join(
		parentDir,
		fmt.Sprintf("test-%s-%s", t.Name(), time.Now().Format("20060102-150405.000")),
	)
	err = os.MkdirAll(dir, 0755)
	require.NoError(t, err, "Error creating directory")

	archive := fmt.Sprintf("%s.%s", dir, "zip")
	paths := []string{"**/project"}

	t.Cleanup(func() {
		t.Logf("Removing temporary directory: %s", dir)
		os.RemoveAll(dir)
		t.Logf("Removing archive: %s", archive)
		os.RemoveAll(archive)
	})

	files := setupEnvironment(t, fmt.Sprintf("%s", parentDir), dir)

	// Start a new cli with the arguments for the command.
	args := []string{os.Args[0], "cache-archiver"}

	app, cmd := newFileArchiveInitTestApp(archive, paths)
	err = app.Run(args)

	matches := helpers.GetMatches(cmd)
	require.ElementsMatch(t, files, slices.Collect(maps.Keys(matches)), "Elements in archive don't match with expected")
}

func setupEnvironment(t *testing.T, parentDir, dir string) []string {
	t.Helper()

	t.Logf("Creating project structure in: %s", dir)

	// Define project root path
	projectRoot := filepath.Join(dir, "project")

	dirs := []string{
		projectRoot,
		filepath.Join(projectRoot, "folder1"),
		filepath.Join(projectRoot, "folder1", "subfolder"),
		filepath.Join(projectRoot, "folder2"),
		filepath.Join(projectRoot, "folder3"),
		filepath.Join(projectRoot, "selfreferential"),
	}

	for _, d := range dirs {
		err := os.MkdirAll(d, 0755)
		require.NoError(t, err, "Error creating directory")
	}

	files := []string{
		filepath.Join(projectRoot, "folder1", "file1.txt"),
		filepath.Join(projectRoot, "folder1", "subfolder", "data.csv"),
		filepath.Join(projectRoot, "folder2", "file2.txt"),
		filepath.Join(projectRoot, "folder2", "report.csv"),
		filepath.Join(projectRoot, "folder3", "file3.csv"),
	}

	for _, f := range files {
		createFile(t, f)
	}

	symlinks := []struct{ target, link string }{
		{"../folder2", filepath.Join(projectRoot, "folder1", "loop")},
		{"../folder1/subfolder", filepath.Join(projectRoot, "folder2", "subfolder")},
		{"../../folder1", filepath.Join(projectRoot, "folder1", "subfolder", "back")},
		{"../folder3", filepath.Join(projectRoot, "folder2", "another")},
		{"../folder1", filepath.Join(projectRoot, "folder3", "link_to_folder1")},
		{".", filepath.Join(projectRoot, "selfreferential", "myself")},
	}

	for _, s := range symlinks {
		err := os.Symlink(s.target, s.link)
		require.NoError(t, err, "Error creating symlink")
	}

	var createdPaths []string
	allPaths := append(dirs, files...)
	for _, s := range symlinks {
		allPaths = append(allPaths, s.link)
	}

	for _, path := range allPaths {
		relPath := trimPrefixes(path, parentDir, "/", "\\")
		createdPaths = append(createdPaths, strings.ReplaceAll(relPath, "\\", "/"))
	}

	return createdPaths
}

func trimPrefixes(s string, prefixes ...string) string {
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			s = strings.TrimPrefix(s, prefix)
		}
	}

	return s
}

func createFile(t *testing.T, path string) {
	t.Helper()

	file, err := os.Create(path)
	require.NoError(t, err, "creating file %q", path)
	require.NoError(t, file.Close(), "closing file %q", path)
}
