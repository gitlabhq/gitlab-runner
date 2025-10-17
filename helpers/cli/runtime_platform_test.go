//go:build !integration

package cli_helpers_test

import (
	"io"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
	cli_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/cli"
)

func TestLogRuntimePlatform(t *testing.T) {
	tests := []struct {
		name                       string
		args                       []string
		expectedRuntimePlatformLog bool
	}{
		{
			name:                       "no args",
			expectedRuntimePlatformLog: true,
		},
		{
			name:                       "some random args",
			args:                       []string{"foo", "steps"},
			expectedRuntimePlatformLog: true,
		},
		{
			name: "first arg blocks runtime platform logging",
			args: []string{"steps", "bar"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			beforeHasBeenCalled := false

			app := cli.NewApp()
			app.Action = func(ctx *cli.Context) error {
				return nil
			}
			app.Before = func(ctx *cli.Context) error {
				beforeHasBeenCalled = true
				return nil
			}

			hook := test.NewGlobal()
			logrus.SetOutput(io.Discard)

			cli_helpers.LogRuntimePlatform(app)

			err := app.Run(append([]string{"fakeArgv0"}, tc.args...))
			require.NoError(t, err, "running app")

			seen := hasRuntimePlatformLog(hook.Entries)

			assert.Equal(t, tc.expectedRuntimePlatformLog, seen)
			assert.True(t, beforeHasBeenCalled, "other before should be called")
		})
	}
}

func hasRuntimePlatformLog(entries []logrus.Entry) bool {
	for _, e := range entries {
		if strings.Contains(e.Message, "Runtime platform") {
			return true
		}
	}
	return false
}
