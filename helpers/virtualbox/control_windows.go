package virtualbox

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

func init() {
	addDirectoryToPATH(os.Getenv("ProgramFiles"))
	addDirectoryToPATH(os.Getenv("ProgramFiles(X86)"))
}

func addDirectoryToPATH(programFilesPath string) {
	if programFilesPath == "" {
		return
	}

	virtualBoxPath := filepath.Join(programFilesPath, "Oracle", "VirtualBox")
	newPath := fmt.Sprintf("%s;%s", os.Getenv("PATH"), virtualBoxPath)
	err := os.Setenv("PATH", newPath)
	if err != nil {
		logrus.Warnf(
			"Failed to add path to VBoxManage.exe (%q) to end of local PATH: %v",
			virtualBoxPath,
			err)
		return
	}

	logrus.Debugf("Added path to VBoxManage.exe to end of local PATH: %q", virtualBoxPath)
}
