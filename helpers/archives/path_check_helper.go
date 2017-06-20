package archives

import (
	"fmt"
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

func warnOnGitDirectory(operation string, paths []string) {
	if !doesPathsListContainGitDirectory(paths) {
		return
	}

	logrus.Warn(fmt.Sprintf("Part of .git directory is on the list of files to %s", operation))
	logrus.Warn("This may introduce unexpected problems")
}
