package build

import (
	"fmt"
	"io"

	"github.com/magefile/mage/sh"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/mageutils"
)

const (
	AppName = "gitlab-runner"
)

var versionOnce mageutils.Once[string]

func Version() string {
	return versionOnce.Do(func() (string, error) {
		return sh.Output("sh", "-c", "./ci/version")
	})
}

func RefTag() string {
	return mageutils.EnvOrDefault("CI_COMMIT_TAG", "bleeding")
}

var latestStableTagOnce mageutils.Once[string]

func LatestStableTag() string {
	return latestStableTagOnce.Do(func() (string, error) {
		return sh.Output("sh", "-c", "git -c versionsort.prereleaseSuffix=\"-rc\" -c versionsort.prereleaseSuffix=\"-RC\" tag -l \"v*.*.*\" | sort -rV | awk '!/rc/' | head -n 1")
	})
}

var isLatestOnce mageutils.Once[bool]

func IsLatest() bool {
	return isLatestOnce.Do(func() (bool, error) {
		_, err := sh.Exec(
			nil,
			io.Discard,
			io.Discard,
			"git",
			"describe",
			"--exact-match",
			"--match",
			LatestStableTag(),
		)
		return err == nil, nil
	})
}

var revisionOnce mageutils.Once[string]

func Revision() string {
	return revisionOnce.Do(func() (string, error) {
		out, err := sh.Output("git", "rev-parse", "--short=8", "HEAD")
		if err != nil {
			return "", err
		}

		if out == "" {
			out = "unknown"
		}

		return out, nil
	})
}

func ReleaseArtifactsPath(f string) string {
	return fmt.Sprintf("out/release_artifacts/%s.json", f)
}
