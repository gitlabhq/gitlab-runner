//nolint:lll
//go:build integration && (aix || android || darwin || dragonfly || freebsd || hurd || illumos || linux || netbsd || openbsd || solaris)

package volumes_test

import (
	"testing"

	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes/parser"
)

var testCreateVolumesLabelsDestinationPath = "/test"

func TestCreateVolumesLabels(t *testing.T) {
	testCreateVolumesLabels(t, parser.NewLinuxParser())
}
