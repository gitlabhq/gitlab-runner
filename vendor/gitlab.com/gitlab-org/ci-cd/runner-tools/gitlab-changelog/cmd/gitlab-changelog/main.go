package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/ci-cd/runner-tools/gitlab-changelog"
	"gitlab.com/gitlab-org/ci-cd/runner-tools/gitlab-changelog/config"
	"gitlab.com/gitlab-org/ci-cd/runner-tools/gitlab-changelog/generator"
)

var (
	projectID            = flag.String("project-id", "", "ID of the project to scan Merge Requests for")
	release              = flag.String("release", "", "Release name to use as entries header")
	startingPoint        = flag.String("starting-point", "", "Starting point for git log (last tag autodiscovered if empty)")
	startingPointMatcher = flag.String("starting-point-matcher", "v[0-9]*.[0-9]*.[0-9]*", "Reference matcher for starting point autodiscovery")

	changelogFile = flag.String("changelog-file", "", "Changelog file to prepend with new entries")

	configFile        = flag.String("config-file", "", "File with configuration")
	dumpDefaultConfig = flag.Bool("dump-default-config", false, "Prints default configuration and exits")

	debug   = flag.Bool("debug", false, "Enable debug output")
	version = flag.Bool("version", false, "Print generator version and exit")
)

func main() {
	defer func() {
		r := recover()
		if r != nil {
			logrus.Fatalf("Panic failure with: %v", r)
		}
	}()

	setup()

	logrus.WithFields(logrus.Fields{
		"pid":     os.Getpid(),
		"version": gitlab_changelog.Version().SimpleLine(),
	}).Info("Starting GitLab Changelog generator")

	g := generator.New(generator.Opts{
		ProjectID:            *projectID,
		Release:              *release,
		StartingPoint:        *startingPoint,
		StartingPointMatcher: *startingPointMatcher,
		ChangelogFile:        *changelogFile,
		ConfigFile:           *configFile,
	})

	err := g.Generate()
	if err != nil {
		logrus.WithError(err).
			Fatal("Failed to generate changelog entries")
	}
}

func setup() {
	flag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		_, _ = fmt.Fprintf(os.Stderr, "\n  %s [OPTIONS]\n\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if *version {
		fmt.Println(gitlab_changelog.Version().Extended())
		os.Exit(0)
	}

	dumpDefaultConfiguration()

	exitIfFlagEmpty(*projectID)
	exitIfFlagEmpty(*release)

	if *debug {
		logrus.SetLevel(logrus.DebugLevel)
	}
}

func dumpDefaultConfiguration() {
	if !*dumpDefaultConfig {
		return
	}

	data, err := config.DumpDefaultConfig()
	if err != nil {
		logrus.WithError(err).
			Fatal("Failed to dump default scope configuration")
	}

	fmt.Println(string(data))
	os.Exit(0)
}

func exitIfFlagEmpty(value string) {
	if value != "" {
		return
	}

	flag.Usage()
	os.Exit(1)
}
