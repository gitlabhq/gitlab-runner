package commands

import (
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

func getDefaultConfigDirectory() string {
	if currentDir := helpers.GetCurrentWorkingDirectory(); currentDir != "" {
		return currentDir
	}

	panic("Cannot get default config file location")
}
