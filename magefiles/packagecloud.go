//go:build mage

package main

import (
	"errors"
	"fmt"
	"strings"

	"gitlab.com/gitlab-org/gitlab-runner/magefiles/mageutils"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/packagecloud"
)

var (
	packageCloudURL       = mageutils.EnvFallbackOrDefault("PACKAGE_CLOUD_URL", "", "https://packages.gitlab.com/")
	packageCloudNamespace = mageutils.EnvFallbackOrDefault("PACKAGE_CLOUD_NAMESPACE", "PACKAGE_CLOUD", "runner/gitlab-runner")
	packageCloudToken     = mageutils.EnvFallbackOrDefault("PACKAGE_CLOUD_TOKEN", "PACKAGECLOUD_TOKEN", "")
)

type PackageCloud mg.Namespace

// Yank yanks all packages from PackageCloud for the specified version
func (PackageCloud) Yank(version string) error {
	return packagecloud.Yank(packagecloud.YankOpts{
		Version:       version,
		PackageBuilds: packageBuilds,
		Token:         packageCloudToken,
		URL:           packageCloudURL,
		Namespace:     packageCloudNamespace,
		Concurrency:   config.Concurrency,
		DryRun:        config.DryRun,
	})
}

// Deps installs package_cloud CLI
func (PackageCloud) Deps() error {
	if err := sh.RunV("package_cloud", "version"); err != nil {
		return sh.RunV("gem", "install", "package_cloud", "--version", "~> 0.3.0", "--no-document")
	}

	return nil
}

// Push releases PackageCloud packages
func (p PackageCloud) Push(dist, branch, flavor string) error {
	mg.Deps(p.Deps)

	branch = strings.Split(branch, " ")[0]
	return packagecloud.Push(packagecloud.PushOpts{
		URL:         packageCloudURL,
		Namespace:   packageCloudNamespace,
		Token:       packageCloudToken,
		Branch:      branch,
		Dist:        dist,
		Flavor:      flavor,
		Concurrency: config.Concurrency,
		DryRun:      config.DryRun,
	})
}

// SupportedOSVersions prints the list of OS/versions for which runner packages will be released (for the given package type and release branch)
func (p PackageCloud) SupportedOSVersions(dist, branch string) error {
	os, err := supportedOSVersions(dist, branch)
	if err != nil {
		return err
	}
	fmt.Println(strings.Join(os, "\n"))
	return nil
}

func supportedOSVersions(dist, branch string) ([]string, error) {
	if packageCloudToken == "" {
		return nil, errors.New("required 'PACKAGE_CLOUD_TOKEN' variable missing")
	}

	return packagecloud.Releases(dist, branch, packageCloudToken, packageCloudURL)
}
