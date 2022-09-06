//go:build !integration

package shells_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/gitlab-org/gitlab-runner/shells"
	"gitlab.com/gitlab-org/gitlab-runner/shells/shellstest"
)

func TestShellWriterFlush(t *testing.T) {
	// scripts are typically copied over stdin, so without a terminating newline
	// flush, it'd be like writing a command to a terminal and never hitting
	// return.

	shellstest.OnEachShellWithWriter(t, func(t *testing.T, shell string, writer shells.ShellWriter) {
		assert.True(t, strings.HasSuffix(writer.Finish(false), "\n"), "shell writer should terminate with newline")
	})
}
