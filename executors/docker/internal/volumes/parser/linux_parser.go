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
	linuxMode            = `(:(?P<mode>(?i)ro|rw))?`
	linuxLabel           = `((:|,)(?P<label>(?i)z))?`
	linuxBindPropagation = `((:|,)(?P<bindPropagation>(?i)shared|slave|private|rshared|rslave|rprivate))?`
)

var (
	specExp = regexp.MustCompile(`^` + linuxSource + linuxDestination + linuxMode +
		linuxLabel + linuxBindPropagation + `$`)
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
	parts, err := p.matchesToVolumeSpecParts(spec, specExp)
	if err != nil {
		return nil, err
	}

	v := newVolume(parts["source"], parts["destination"], parts["mode"], parts["label"], parts["bindPropagation"])

	return v, nil
}
