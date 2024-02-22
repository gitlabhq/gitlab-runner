//go:build integration

package volumes_test

import (
	"testing"

	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes/parser"
)

var (
	testCreateVolumesLabelsDestinationPath     = `C:\test`
	testCreateVolumesDriverOptsDestinationPath = `C:\test`
)

func TestCreateVolumesLabels(t *testing.T) {
	testCreateVolumesLabels(t, parser.NewWindowsParser())
}

func TestCreateVolumesDriverOpts(t *testing.T) {
	t.Skip("Windows local driver does not accept volume driver options.")
	//testCreateVolumesDriverOpts(t, parser.NewWindowsParser())
}
