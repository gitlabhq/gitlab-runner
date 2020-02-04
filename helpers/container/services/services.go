package services

import (
	"regexp"
	"strings"

	"github.com/docker/distribution/reference"
)

type Service struct {
	Service   string
	Version   string
	ImageName string
	Aliases   []string
}

func SplitNameAndVersion(serviceDescription string) (out Service) {
	ReferenceRegexpNoPort := regexp.MustCompile(`^(.*?)(|:[0-9]+)(|/.*)$`)
	out.ImageName = serviceDescription
	out.Version = "latest"

	if match := reference.ReferenceRegexp.FindStringSubmatch(serviceDescription); match != nil {
		matchService := ReferenceRegexpNoPort.FindStringSubmatch(match[1])
		out.Service = matchService[1] + matchService[3]

		if len(match[2]) > 0 {
			out.Version = match[2]
		} else {
			out.ImageName = match[1] + ":" + out.Version
		}
	} else {
		return
	}

	alias := strings.Replace(out.Service, "/", "__", -1)
	out.Aliases = append(out.Aliases, alias)

	// Create alternative link name according to RFC 1123
	// Where you can use only `a-zA-Z0-9-`
	if alternativeName := strings.Replace(out.Service, "/", "-", -1); alias != alternativeName {
		out.Aliases = append(out.Aliases, alternativeName)
	}
	return
}
