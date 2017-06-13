package archives

import (
	"path/filepath"
	"strings"

	"github.com/Sirupsen/logrus"
)

func doesPathsListContainGitDirectory(paths []string) bool {
	for _, path := range paths {
		parts := strings.Split(filepath.Clean(path), string(filepath.Separator))
		if len(parts) > 0 && parts[0] == ".git" {
			return true
		}
	}

	return false
}

func warnIfTryingToArchiveGitDirectory(paths []string) {
	if doesPathsListContainGitDirectory(paths) {
		logrus.Warn("Part of .git directory is on the list of files to archive")
		logrus.Warn("This may introduce unexpected problems")
	}
}

func warnIfTryingToExtractGitDirectory(paths []string) {
	if doesPathsListContainGitDirectory(paths) {
		logrus.Warn("Part of .git directory is on the list of files to extract")
		logrus.Warn("This may introduce unexpected problems")
	}
}
