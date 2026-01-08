//go:build mage

package main

import (
	"fmt"
	"strings"

	"github.com/magefile/mage/mg"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/pulp"
)

type Pulp mg.Namespace

// SupportedOSVersions prints the list of OS/versions for which runner packages will be released (for the given package type and release branch)
func (p Pulp) SupportedOSVersions(dist, branch string) error {
	os, err := pulp.Releases(dist, branch)
	if err != nil {
		return err
	}
	fmt.Println(strings.Join(os, "\n"))
	return nil
}
