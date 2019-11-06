package parser

import (
	"regexp"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/path"
)

const (
	linuxDir        = `/(?:[^\\/:*?"<>|\r\n ]+/?)*`
	linuxVolumeName = `[^\\/:*?"<>|\r\n]+`

	linuxSource = `((?P<source>((` + linuxDir + `)|(` + linuxVolumeName + `))):)?`

	linuxDestination     = `(?P<destination>(?:` + linuxDir + `))`
	linuxMode            = `(:(?P<mode>(?i)ro|rw|z))?`
	linuxBindPropagation = `((:|,)(?P<bindPropagation>(?i)shared|slave|private|rshared|rslave|rprivate))?`
)

type linuxParser struct {
	baseParser
}

func NewLinuxParser() Parser {
	return &linuxParser{
		baseParser: baseParser{
			path: path.NewUnixPath(),
		},
	}
}

func (p *linuxParser) ParseVolume(spec string) (*Volume, error) {
	specExp := regexp.MustCompile(`^` + linuxSource + linuxDestination + linuxMode + linuxBindPropagation + `$`)

	parts, err := p.matchesToVolumeSpecParts(spec, specExp)
	if err != nil {
		return nil, err
	}

	return newVolume(parts["source"], parts["destination"], parts["mode"], parts["bindPropagation"]), nil
}
