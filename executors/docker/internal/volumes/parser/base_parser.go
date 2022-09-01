package parser

import (
	"regexp"
)

type baseParser struct {
	path Path
}

// The way how matchesToVolumeSpecParts parses the volume mount specification and assigns
// parts was inspired by how Docker Engine's `windowsParser` is created. The original sources
// can be found at:
//
// https://github.com/docker/engine/blob/a79fabbfe84117696a19671f4aa88b82d0f64fc1/volume/mounts/windows_parser.go
//
// The original source is licensed under Apache License 2.0 and the copyright for it
// goes to Docker, Inc.
func (p *baseParser) matchesToVolumeSpecParts(spec string, specExp *regexp.Regexp) (map[string]string, error) {
	match := specExp.FindStringSubmatch(spec)

	if len(match) == 0 {
		return nil, NewInvalidVolumeSpecErr(spec)
	}

	matchgroups := make(map[string]string)
	for i, name := range specExp.SubexpNames() {
		matchgroups[name] = match[i]
	}

	parts := map[string]string{
		"source":          "",
		"destination":     "",
		"mode":            "",
		"label":           "",
		"bindPropagation": "",
	}

	for group := range parts {
		content, ok := matchgroups[group]
		if ok {
			parts[group] = content
		}
	}

	return parts, nil
}

func (p *baseParser) Path() Path {
	return p.path
}
