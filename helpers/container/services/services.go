package services

import (
	"fmt"
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

var referenceRegexpNoPort = regexp.MustCompile(`^(.*?)(|:[0-9]+)(|/.*)$`)

const imageVersionLatest = "latest"

// SplitNameAndVersion parses Docker registry image urls and constructs a struct with correct
// image url, name, version and aliases
func SplitNameAndVersion(serviceDescription string) Service {
	// Try to find matches in e.g. subdomain.domain.tld:8080/namespace/service:version
	matches := reference.ReferenceRegexp.FindStringSubmatch(serviceDescription)
	if len(matches) == 0 {
		return Service{
			ImageName: serviceDescription,
			Version:   imageVersionLatest,
		}
	}

	// -> subdomain.domain.tld:8080/namespace/service
	imageWithoutVersion := matches[1]
	// -> version
	imageVersion := matches[2]

	registryMatches := referenceRegexpNoPort.FindStringSubmatch(imageWithoutVersion)
	// -> subdomain.domain.tld
	registry := registryMatches[1]
	// -> /namespace/service
	imageName := registryMatches[3]

	service := Service{}
	service.Service = registry + imageName

	if len(imageVersion) > 0 {
		service.ImageName = serviceDescription
		service.Version = imageVersion
	} else {
		service.ImageName = fmt.Sprintf("%s:%s", imageWithoutVersion, imageVersionLatest)
		service.Version = imageVersionLatest
	}

	alias := strings.ReplaceAll(service.Service, "/", "__")
	service.Aliases = append(service.Aliases, alias)

	// Create alternative link name according to RFC 1123
	// Where you can use only `a-zA-Z0-9-`
	alternativeName := strings.ReplaceAll(service.Service, "/", "-")
	if alias != alternativeName {
		service.Aliases = append(service.Aliases, alternativeName)
	}

	return service
}
