package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/ci-cd/runner-tools/release-index-generator"
	"gitlab.com/gitlab-org/ci-cd/runner-tools/release-index-generator/gpg"
	"gitlab.com/gitlab-org/ci-cd/runner-tools/release-index-generator/release"
)

const (
	defaultGPGKeyEnv      = "GPG_KEY"
	defaultGPGPasswordEnv = "GPG_PASSWORD"
)

var (
	workingDirectory = flag.String("working-directory", filepath.Clean("./"), "Directory used to scanWorkingDirectory for files and create index/checksum result")

	projectVersion     = flag.String("project-version", "", "Version of the released project")
	projectGitRef      = flag.String("project-git-ref", "", "The git ref of the released project")
	projectGitRevision = flag.String("project-git-revision", "", "The git revision of the released project")

	projectName    = flag.String("project-name", os.Getenv("CI_PROJECT_NAME"), "The name of the released project")
	projectRepoURL = flag.String("project-repo-url", os.Getenv("CI_PROJECT_URL"), "The URL of the released project's repository root")

	gpgKeyEnv      = flag.String("gpg-key-env", defaultGPGKeyEnv, "Name of ENV variable containing private GPG key for release signing. If variable is empty, signing will be disabled")
	gpgPasswordEnv = flag.String("gpg-password-env", defaultGPGPasswordEnv, "Name of ENV variable containing private key password. If GPG key is added to the variable above, this one can't be empty")

	version = flag.Bool("version", false, "Print generator version and exit")
)

func exitIfFlagEmpty(value string) {
	if value != "" {
		return
	}

	flag.Usage()
	os.Exit(1)
}

func main() {
	flag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		_, _ = fmt.Fprintf(os.Stderr, "\n  %s [OPTIONS]\n\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if *version {
		fmt.Println(releaseindexgenerator.Version().Extended())
		os.Exit(0)
	}

	exitIfFlagEmpty(*projectVersion)
	exitIfFlagEmpty(*projectGitRef)
	exitIfFlagEmpty(*projectGitRevision)

	signer := prepareGPGSignerFromEnv()

	releaseInfo := release.Info{
		Name:      *projectVersion,
		Project:   *projectName,
		SourceURL: fmt.Sprintf("%s/tree/%s", *projectRepoURL, *projectGitRef),
		Ref:       *projectGitRef,
		Revision:  *projectGitRevision,
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	r := release.NewGenerator(strings.Trim(*workingDirectory, "/"), releaseInfo, signer)

	err := r.Prepare()
	if err != nil {
		logrus.WithError(err).Fatalf("Failed to prepare release info")
	}

	err = r.GenerateIndexFile()
	if err != nil {
		logrus.WithError(err).Fatalf("Failed to render the index file")
	}
}

func prepareGPGSignerFromEnv() gpg.Signer {
	armoredGPGKey := os.Getenv(*gpgKeyEnv)
	if armoredGPGKey == "" {
		return nil
	}

	gpgKeyPassword := os.Getenv(*gpgPasswordEnv)
	if gpgKeyPassword == "" {
		logrus.Fatalf("No password for GPP key provided")
	}

	signer, err := gpg.NewSigner(armoredGPGKey, gpgKeyPassword)
	if err != nil {
		logrus.WithError(err).Fatalf("Failed to load the GPG key")
	}

	return signer
}
