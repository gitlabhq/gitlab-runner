package test

import (
	"context"
	"os"
	"runtime"
	"testing"

	"github.com/docker/docker/client"
)

const (
	OSWindows = "windows"
	OSLinux   = "linux"
)

func SkipIfGitLabCI(t *testing.T) {
	_, ok := os.LookupEnv("CI")
	if ok {
		t.Skipf("Skipping test on CI builds: %s", t.Name())
	}
}

func SkipIfGitLabCIOn(t *testing.T, os string) {
	if runtime.GOOS != os {
		return
	}

	SkipIfGitLabCI(t)
}

func SkipIfGitLabCIWithMessage(t *testing.T, msg string) {
	_, ok := os.LookupEnv("CI")
	if ok {
		t.Skipf("Skipping test on CI builds: %s - %s", t.Name(), msg)
	}
}

func SkipIfDockerDaemonAPIVersionNotAtLeast(t *testing.T, version string) {
	ver, err := getDockerDaemonAPIVersion()
	if err != nil {
		t.Skipf("Skipping test, failed to get docker daemon version: %s", t.Name())
	}
	if ver < version {
		t.Skipf("Skipping test against docker daemon verion %s<%s: %s", ver, version, t.Name())
	}
}

func IsDockerDaemonAPIVersionAtLeast(version string) (bool, error) {
	ver, err := getDockerDaemonAPIVersion()
	if err != nil {
		return false, err
	}
	return ver >= version, nil
}

func getDockerDaemonAPIVersion() (string, error) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return "", err
	}
	defer cli.Close()

	ver, err := cli.ServerVersion(ctx)
	if err != nil {
		return "", err
	}
	return ver.APIVersion, nil
}
