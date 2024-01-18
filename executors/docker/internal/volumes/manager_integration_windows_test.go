//go:build integration

package volumes_test

import (
	"testing"

	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes/parser"
)

var testCreateVolumesLabelsDestinationPath = `C:\test`

func TestCreateVolumesLabels(t *testing.T) {
	testCreateVolumesLabels(t, parser.NewWindowsParser())
}

// On Windows local driver does not accept options so we do not run testCreateVolumesDriverOpts.
