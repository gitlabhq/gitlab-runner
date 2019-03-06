package shellstest

import (
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/shells"
)

func OnEachShell(t *testing.T, f func(t *testing.T, shell string)) {
	shells := []string{"bash", "cmd", "powershell"}

	for _, shell := range shells {
		t.Run(shell, func(t *testing.T) {
			if helpers.SkipIntegrationTests(t, shell) {
				t.Skip()
			}

			f(t, shell)
		})

	}
}

func OnEachShellWithWriter(t *testing.T, f func(t *testing.T, shell string, writer ShellWriter)) {
	writers := map[string]ShellWriterFactory{
		"bash": func() ShellWriter {
			return &shells.BashWriter{}
		},
		"cmd": func() ShellWriter {
			return &shells.CmdWriter{}
		},
		"powershell": func() ShellWriter {
			return &shells.PsWriter{}
		},
	}

	OnEachShell(t, func(t *testing.T, shell string) {
		writer, ok := writers[shell]
		require.True(t, ok, "Missing factory for %s", shell)

		f(t, shell, writer())
	})
}
