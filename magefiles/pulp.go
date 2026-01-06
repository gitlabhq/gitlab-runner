//go:build mage

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"

	"gitlab.com/gitlab-org/gitlab-runner/magefiles/mageutils"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/pulp"
)

type Pulp mg.Namespace

var (
	pulpURL  = mageutils.EnvOrDefault("PULP_URL", "https://pulp.gitlab.com/")
	username = mageutils.EnvOrDefault("PULP_USER", "runner")
)

// SupportedOSVersions prints the list of OS/versions for which runner packages will be released (for the given package type and release branch)
func (p Pulp) SupportedOSVersions(dist, branch string) error {
	os, err := pulp.Releases(dist, branch)
	if err != nil {
		return err
	}
	fmt.Println(strings.Join(os, "\n"))
	return nil
}

// CreateConfig creates a working pulp configuration for pulp.gitlab.com
func (p Pulp) CreateConfig() error {
	password, ok := os.LookupEnv("PULP_PASSWORD")
	if !ok || strings.TrimSpace(password) == "" {
		return fmt.Errorf("missing or invalid PULP_PASSWORD")
	}

	return sh.RunV("pulp", "config", "create", "--overwrite",
		"--base-url", pulpURL,
		"--api-root", "/pulp/",
		"--verify-ssl",
		"--format", "json",
		"--username", username,
		"--password", password,
	)
}
