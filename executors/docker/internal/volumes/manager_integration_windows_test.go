//go:build integration

package volumes_test

import (
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes/parser"
)

var (
	testCreateVolumesLabelsDestinationPath     = `C:\test`
	testCreateVolumesDriverOptsDestinationPath = `C:\test`
)

func parserCreator(varExpander func(string) string) parser.Parser {
	return parser.NewWindowsParser(varExpander)
}
