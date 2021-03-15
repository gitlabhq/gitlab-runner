// +build linux darwin freebsd openbsd

package volumes_test

import (
	"testing"

	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes/parser"
)

var testCreateVolumesLabelsDestinationPath = "/test"

func TestCreateVolumesLabels(t *testing.T) {
	testCreateVolumesLabels(t, parser.NewLinuxParser())
}
