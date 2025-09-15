package test

import (
	"context"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/docker/docker/client"
	"github.com/hashicorp/go-version"
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

func SkipIfVariable(t *testing.T, varName string) {
	val, ok := os.LookupEnv(varName)

	if !ok {
		return
	}

	set, err := strconv.ParseBool(val)
	if err != nil {
		return
	}

	if set {
		t.Skipf("Skipping test %s because variable %s set", t.Name(), varName)
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

// CommandVersionIsAtLeast runs the getVersionCommand and tries to parse a version string from that output. It will
// compare then compare that to minVersion.
// On errors parsing version strings or running the command, the test will be aborted.
func CommandVersionIsAtLeast(t *testing.T, minVersion string, getVersionCommand ...string) bool {
	t.Helper()

	vMin, err := version.NewVersion(minVersion)
	if err != nil {
		t.Fatalf("error parsing minimal version %q: %v", minVersion, err)
	}

	bin, args := getVersionCommand[0], getVersionCommand[1:]
	cmd := exec.Command(bin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("error running command %v: %v", getVersionCommand, err)
	}

	vRE := regexp.MustCompile(`v?(\d+\.)*\d+`)
	out = vRE.Find(out)
	vCurrent, err := version.NewVersion(string(out))
	if err != nil {
		t.Fatalf("error parsing current version %q: %v", out, err)
	}

	isAtLeast := vCurrent.GreaterThanOrEqual(vMin)

	msg := "⚠"
	if isAtLeast {
		msg = "✔"
	}

	t.Logf("version for %q: %s (current: %q, minimum: %q)", bin, msg, vCurrent.String(), vMin.String())

	return isAtLeast
}

// NormalizePath is a quick & dirty way to handle some path oddities for our tests / test infra.
func NormalizePath(orgPath string) string {
	replacements := []string{
		// on the hosted runners sometimes we get the short path, so we just normalize that here.
		`C:\Users\GITLAB~1\AppData\`, `C:\Users\gitlab_runner\AppData\`,
	}
	return strings.NewReplacer(replacements...).Replace(orgPath)
}
