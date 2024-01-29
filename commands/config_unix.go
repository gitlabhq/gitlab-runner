//go:build aix || android || darwin || dragonfly || freebsd || hurd || illumos || linux || netbsd || openbsd || solaris

package commands

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

var ROOTCONFIGDIR = "/etc/gitlab-runner"

func getDefaultConfigDirectory(user string) string {
	switch {
	case os.Getuid() == 0:
		if user == "" {
			return ROOTCONFIGDIR
		}
		return filepath.Join(getUserHomeDir(user), ".gitlab-runner")

	case helpers.GetHomeDir() != "":
		return filepath.Join(helpers.GetHomeDir(), ".gitlab-runner")
	case helpers.GetCurrentWorkingDirectory() != "":
		return helpers.GetCurrentWorkingDirectory()
	default:
		panic("Cannot get default config file location")
	}
}

func getUserHomeDir(username string) string {
	u, err := user.Lookup(username)
	if err != nil {
		panic(fmt.Sprintf("Failed to get home for user %q: %s", username, err.Error()))
	}
	return u.HomeDir
}
