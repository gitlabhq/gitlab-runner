//go:build windows

package parser

import "gitlab.com/gitlab-org/gitlab-runner/helpers/path"

func newWindowsPath() Path {
	return path.NewWindowsPath()
}
