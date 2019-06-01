package parser

import "gitlab.com/gitlab-org/gitlab-runner/helpers/path"

type Parser interface {
	ParseVolume(spec string) (*Volume, error)
	Path() path.Path
}
