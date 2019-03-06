package shellstest

import "gitlab.com/gitlab-org/gitlab-runner/shells"

type ShellWriter interface {
	shells.ShellWriter

	Finish(trace bool) string
}

type ShellWriterFactory func() ShellWriter
