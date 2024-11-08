//go:build aix || android || darwin || dragonfly || freebsd || hurd || illumos || linux || netbsd || openbsd || solaris

package commands

import (
	"os"
	"path/filepath"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/homedir"
)

var ROOTCONFIGDIR = "/etc/gitlab-runner"

func getDefaultConfigDirectory() string {
	if os.Getuid() == 0 {
		return ROOTCONFIGDIR
	} else if homeDir := homedir.Get(); homeDir != "" {
		return filepath.Join(homeDir, ".gitlab-runner")
	} else if currentDir := homedir.GetWDOrEmpty(); currentDir != "" {
		return currentDir
	}
	panic("Cannot get default config file location")
}
