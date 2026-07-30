//go:build !integration

package commands

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

// TestVerifyCommand_Execute exercises VerifyCommand through the real cli.App
// flag/env binding (rather than constructing VerifyCommand directly), since
// the bug this guards against - gitlab-org/charts/gitlab-runner#577 - is in
// how --url/--token are bound to CI_SERVER_URL/CI_SERVER_TOKEN, not in the
// filtering logic itself.
func TestVerifyCommand_Execute(t *testing.T) {
	testCases := []struct {
		name  string
		env   map[string]string
		args  []string
		setup func(t *testing.T) common.Network
	}{
		{
			name: "no selector verifies all runners",
			setup: func(t *testing.T) common.Network {
				mn := common.NewMockNetwork(t)
				mn.On("VerifyRunner", mock.Anything, mock.Anything).Return(&common.VerifyRunnerResponse{}).Times(3)
				return mn
			},
		},
		{
			// Regression test for gitlab-org/charts/gitlab-runner#577: a
			// CI_SERVER_TOKEN left in the environment (e.g. by a Vault/
			// bank-vaults sidecar) that doesn't match any configured
			// runner's token must not turn "verify everything" into a
			// filter that matches nothing.
			name: "unrelated CI_SERVER_TOKEN in environment does not filter runners",
			env:  map[string]string{"CI_SERVER_TOKEN": "vault:secret/data/foo#secrets.bar"},
			setup: func(t *testing.T) common.Network {
				mn := common.NewMockNetwork(t)
				mn.On("VerifyRunner", mock.Anything, mock.Anything).Return(&common.VerifyRunnerResponse{}).Times(3)
				return mn
			},
		},
		{
			name: "unrelated CI_SERVER_URL in environment does not filter runners",
			env:  map[string]string{"CI_SERVER_URL": "https://not-configured.example.com/"},
			setup: func(t *testing.T) common.Network {
				mn := common.NewMockNetwork(t)
				mn.On("VerifyRunner", mock.Anything, mock.Anything).Return(&common.VerifyRunnerResponse{}).Times(3)
				return mn
			},
		},
		{
			name: "explicit --token still filters to the matching runner",
			args: []string{"--token", "test-token2"},
			setup: func(t *testing.T) common.Network {
				mn := common.NewMockNetwork(t)
				mn.On(
					"VerifyRunner",
					mock.MatchedBy(func(cfg common.RunnerConfig) bool {
						return cfg.Name == "test-shell-runner-1"
					}),
					mock.Anything,
				).Return(&common.VerifyRunnerResponse{}).Once()
				return mn
			},
		},
		{
			name: "explicit --url and --token still filters to the matching runner",
			args: []string{"--url", "https://gitlab.example.com/", "--token", "test-token2"},
			setup: func(t *testing.T) common.Network {
				mn := common.NewMockNetwork(t)
				mn.On(
					"VerifyRunner",
					mock.MatchedBy(func(cfg common.RunnerConfig) bool {
						return cfg.Name == "test-shell-runner-1"
					}),
					mock.Anything,
				).Return(&common.VerifyRunnerResponse{}).Once()
				return mn
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			app := cli.NewApp()
			app.Commands = []cli.Command{NewVerifyCommand(tc.setup(t))}

			args := append([]string{"gitlab-runner", "verify", "--config", "./testdata/test-config.toml"}, tc.args...)
			require.NoError(t, app.Run(args))
		})
	}
}
