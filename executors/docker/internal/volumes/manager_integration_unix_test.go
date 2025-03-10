//go:build integration && (aix || android || darwin || dragonfly || freebsd || hurd || illumos || linux || netbsd || openbsd || solaris)

package volumes_test

import (
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes/parser"
)

var testCreateVolumesLabelsDestinationPath = "/test"
var testCreateVolumesDriverOptsDestinationPath = "/test"

func parserCreator(varExpander func(string) string) parser.Parser {
	return parser.NewLinuxParser(varExpander)
}
