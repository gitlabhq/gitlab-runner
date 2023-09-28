package packages

import (
	"fmt"

	"gitlab.com/gitlab-org/gitlab-runner/magefiles/build"
)

type Build struct {
	Name             string
	Archs            []string
	PackageArchs     []string
	PackageFileArchs []string
}

type Builds map[string][]Build

func Filenames(packageBuilds Builds, dist, version string) []string {
	var f []string

	for _, b := range packageBuilds[dist] {
		for _, arch := range b.PackageFileArchs {
			switch dist {
			case "deb":
				f = append(f, fmt.Sprintf("%s_%s_%s.deb", build.AppName, version, arch))
			case "rpm":
				f = append(f, fmt.Sprintf("%s-%s-1.%s.rpm", build.AppName, version, arch))
				if arch == "x86_64" {
					// Special case for fips
					f = append(f, fmt.Sprintf("%s-fips-%s-1.%s.rpm", build.AppName, version, arch))
				}
			}
		}
	}

	return f
}
