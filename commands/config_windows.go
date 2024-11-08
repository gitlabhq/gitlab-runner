package commands

import (
	"gitlab.com/gitlab-org/gitlab-runner/helpers/homedir"
)

func getDefaultConfigDirectory() string {
	if currentDir := homedir.GetWDOrEmpty(); currentDir != "" {
		return currentDir
	}

	panic("Cannot get default config file location")
}
