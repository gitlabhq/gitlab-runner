package helpers

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/sirupsen/logrus"
)

type fileArchiver struct {
	Paths     []string `long:"path" description:"Add paths to archive"`
	Exclude   []string `long:"exclude" description:"Exclude paths from the archive"`
	Untracked bool     `long:"untracked" description:"Add git untracked files"`
	Verbose   bool     `long:"verbose" description:"Detailed information"`

	wd       string
	files    map[string]os.FileInfo
	excluded map[string]int64
}

func (c *fileArchiver) isChanged(modTime time.Time) bool {
	for _, info := range c.files {
		if modTime.Before(info.ModTime()) {
			return true
		}
	}
	return false
}

func (c *fileArchiver) isFileChanged(fileName string) bool {
	ai, err := os.Stat(fileName)
	if ai != nil {
		if !c.isChanged(ai.ModTime()) {
			return false
		}
	} else if !os.IsNotExist(err) {
		logrus.Warningln(err)
	}
	return true
}

func (c *fileArchiver) sortedFiles() []string {
	files := make([]string, len(c.files))

	i := 0
	for file := range c.files {
		files[i] = file
		i++
	}

	sort.Strings(files)
	return files
}

func (c *fileArchiver) process(match string) bool {
	var absolute, relative string
	var err error

	absolute, err = filepath.Abs(match)
	if err == nil {
		// Let's try to find a real relative path to an absolute from working directory
		relative, err = filepath.Rel(c.wd, absolute)
	}

	if err == nil {
		// Process path only if it lives in our build directory
		if !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			excluded, rule := c.isExcluded(relative)
			if excluded {
				c.exclude(rule)
				return false
			}

			err = c.add(relative)
		} else {
			err = errors.New("not supported: outside build directory")
		}
	}

	if err == nil {
		return true
	}

	if os.IsNotExist(err) {
		// We hide the error that file doesn't exist
		return false
	}

	logrus.Warningf("%s: %v", match, err)
	return false
}

func (c *fileArchiver) isExcluded(path string) (bool, string) {
	// Both path and pattern need to be normalized with filepath.ToSlash().
	// Matching will fail with Windows machines using "\\" path separators and patterns with "/" path separators
	path = filepath.ToSlash(path)
	for _, pattern := range c.Exclude {
		pattern = filepath.ToSlash(pattern)
		excluded, err := doublestar.Match(pattern, path)
		if err == nil && excluded {
			return true, pattern
		}
	}

	return false, ""
}

func (c *fileArchiver) exclude(rule string) {
	c.excluded[rule]++
}

func (c *fileArchiver) add(path string) error {
	// Always use slashes
	path = filepath.ToSlash(path)

	// Check if file exist
	info, err := os.Lstat(path)
	if err == nil {
		c.files[path] = info
	}

	return err
}

func (c *fileArchiver) processPaths() {
	for _, path := range c.Paths {
		c.processPath(path)
	}
}

func (c *fileArchiver) processPath(path string) {
	path = filepath.ToSlash(path)
	base, patt := doublestar.SplitPattern(path)
	rel, err := c.findRelativePathInProject(base)
	if err != nil {
		// Do not fail job when a file is invalid or not found.
		logrus.Warningf(err.Error())
		return
	}

	fsys := os.DirFS(c.wd)

	// Relative path is needed now that our fsys "root" is at the working directory
	path = filepath.Join(rel, patt)
	path = filepath.ToSlash(path)
	if err != nil {
		logrus.Warningf("%s: %v", path, err)
		return
	}
	matches, err := doublestar.Glob(fsys, path)
	if err != nil {
		logrus.Warningf("%s: %v", path, err)
		return
	}

	found := 0

	for _, match := range matches {
		err := filepath.Walk(match, func(path string, info os.FileInfo, err error) error {
			if c.process(path) {
				found++
			}
			return nil
		})
		if err != nil {
			logrus.Warningln("Walking", match, err)
		}
	}

	if found == 0 {
		logrus.Warningf(
			"%s: no matching files. Ensure that the artifact path is relative to the working directory (%s)",
			path,
			c.wd,
		)
	} else {
		logrus.Infof("%s: found %d matching artifact files and directories", path, found)
	}
}

func (c *fileArchiver) findRelativePathInProject(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("could not resolve artifact absolute path %s: %v", path, err)
	}

	rel, err := filepath.Rel(c.wd, abs)
	if err != nil {
		return "", fmt.Errorf("could not resolve artifact relative path %s: %v", path, err)
	}

	// If fully resolved relative path begins with ".." it is not a subpath of our working directory
	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("artifact path is not a subpath of project directory: %s", path)
	}

	return rel, nil
}

func (c *fileArchiver) processUntracked() {
	if !c.Untracked {
		return
	}

	found := 0

	var output bytes.Buffer
	cmd := exec.Command("git", "ls-files", "-o", "-z")
	cmd.Env = os.Environ()
	cmd.Stdout = &output
	cmd.Stderr = os.Stderr
	logrus.Debugln("Executing command:", strings.Join(cmd.Args, " "))
	err := cmd.Run()
	if err != nil {
		logrus.Warningf("untracked: %v", err)
		return
	}

	reader := bufio.NewReader(&output)
	for {
		line, err := reader.ReadString(0)
		if err == io.EOF {
			break
		} else if err != nil {
			logrus.Warningln(err)
			break
		}
		if c.process(line[:len(line)-1]) {
			found++
		}
	}

	if found == 0 {
		logrus.Warningf("untracked: no files")
	} else {
		logrus.Infof("untracked: found %d files", found)
	}
}

func (c *fileArchiver) enumerate() error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	c.wd = wd
	c.files = make(map[string]os.FileInfo)
	c.excluded = make(map[string]int64)

	c.processPaths()
	c.processUntracked()

	for path, count := range c.excluded {
		logrus.Infof("%s: excluded %d files", path, count)
	}

	return nil
}
