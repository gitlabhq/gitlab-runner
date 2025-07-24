//go:build !integration

package commands

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
	"gitlab.com/gitlab-org/gitlab-runner/commands/internal/configfile"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestUnregisterCommand_unregisterAllRunner(t *testing.T) {
	testCases := []struct {
		name            string
		cfgs            []*common.RunnerConfig
		setup           func(tb testing.TB) common.Network
		expectedRunners []*common.RunnerConfig
	}{
		{
			name: "successfully unregister all runners",
			cfgs: []*common.RunnerConfig{
				{
					RunnerCredentials: common.RunnerCredentials{
						Token: "test-token1",
					},
				},
				{
					RunnerCredentials: common.RunnerCredentials{
						Token: "test-token2",
					},
				},
			},
			setup: func(tb testing.TB) common.Network {
				tb.Helper()
				mn := common.NewMockNetwork(t)
				mn.On(
					"UnregisterRunner",
					mock.Anything,
				).Twice().Return(true)
				return mn
			},
		},
		{
			name: "successfully unregister some runners",
			cfgs: []*common.RunnerConfig{
				{
					RunnerCredentials: common.RunnerCredentials{
						Token: "test-token1",
					},
				},
				{
					RunnerCredentials: common.RunnerCredentials{
						Token: "test-token2",
					},
				},
			},
			setup: func(tb testing.TB) common.Network {
				tb.Helper()
				mn := common.NewMockNetwork(t)
				mn.On(
					"UnregisterRunner",
					mock.Anything,
				).Once().Return(true)
				mn.On(
					"UnregisterRunner",
					mock.Anything,
				).Once().Return(false)
				return mn
			},
			expectedRunners: []*common.RunnerConfig{
				{
					RunnerCredentials: common.RunnerCredentials{
						Token: "test-token2",
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := UnregisterCommand{network: tc.setup(t)}

			runners := cmd.unregisterAllRunners(&common.Config{Runners: tc.cfgs})

			assert.Equal(t, tc.expectedRunners, runners)
		})
	}
}

func TestUnregisterCommand_unregisterSingleRunner(t *testing.T) {
	runnerCredentials1 := common.RunnerCredentials{Token: "token-1"}
	runnerCredentials2 := common.RunnerCredentials{Token: "token-2"}

	testCases := []struct {
		name            string
		cfg             *common.Config
		setup           func(tb testing.TB) common.Network
		expectedRunners []*common.RunnerConfig
	}{
		{
			name: "unregister successful",
			cfg: &common.Config{
				Runners: []*common.RunnerConfig{
					{
						Name:              "test-runner-1",
						RunnerCredentials: runnerCredentials1,
					},
					{
						Name:              "test-runner-2",
						RunnerCredentials: runnerCredentials2,
					},
				},
			},
			setup: func(tb testing.TB) common.Network {
				tb.Helper()
				mn := common.NewMockNetwork(t)
				mn.On(
					"UnregisterRunner",
					mock.Anything,
				).Return(true)
				return mn
			},
			expectedRunners: []*common.RunnerConfig{
				{
					Name:              "test-runner-2",
					RunnerCredentials: runnerCredentials2,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := UnregisterCommand{
				network:           tc.setup(t),
				Name:              "test-runner-1",
				RunnerCredentials: runnerCredentials1,
			}

			runners := cmd.unregisterSingleRunner(tc.cfg)

			assert.Equal(t, tc.expectedRunners, runners)
		})
	}
}

func TestUnregisterCommand_unregisterRunner(t *testing.T) {
	testCases := []struct {
		name     string
		setup    func(tb testing.TB) common.Network
		token    string
		systemID string
		expected bool
	}{
		{
			name: "unregister runner manager success",
			setup: func(tb testing.TB) common.Network {
				tb.Helper()
				mn := common.NewMockNetwork(t)
				mn.On(
					"UnregisterRunnerManager",
					mock.Anything,
					"test-system-id",
				).Return(true)
				return mn
			},
			token:    "glrt-test-token",
			systemID: "test-system-id",
			expected: true,
		},
		{
			name: "unregister runner manager failure",
			setup: func(tb testing.TB) common.Network {
				tb.Helper()
				mn := common.NewMockNetwork(t)
				mn.On(
					"UnregisterRunnerManager",
					mock.Anything,
					"test-system-id",
				).Return(false)
				return mn
			},
			token:    "glrt-test-token",
			systemID: "test-system-id",
			expected: false,
		},
		{
			name: "unregister runner success",
			setup: func(tb testing.TB) common.Network {
				tb.Helper()
				mn := common.NewMockNetwork(t)
				mn.On(
					"UnregisterRunner",
					mock.Anything,
				).Return(true)
				return mn
			},
			token:    "test-token",
			expected: true,
		},
		{
			name: "unregister runner failure",
			setup: func(tb testing.TB) common.Network {
				tb.Helper()
				mn := common.NewMockNetwork(t)
				mn.On(
					"UnregisterRunner",
					mock.Anything,
				).Return(false)
				return mn
			},
			token:    "test-token",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := UnregisterCommand{network: tc.setup(t)}

			result := cmd.unregisterRunner(common.RunnerCredentials{Token: tc.token}, tc.systemID)

			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestUnregisterCommand_Execute(t *testing.T) {
	testCases := []struct {
		name             string
		removeAllRunners bool
		runnerName       string
		setup            func(tb testing.TB) common.Network
		removedRunners   []string
		remainingRunners []string
	}{
		{
			name:       "success removing single runner",
			runnerName: "test-docker-runner",
			setup: func(tb testing.TB) common.Network {
				tb.Helper()
				mn := common.NewMockNetwork(t)
				mn.On(
					"UnregisterRunner",
					mock.Anything,
				).Return(true)
				return mn
			},
			removedRunners:   []string{"test-docker-runner"},
			remainingRunners: []string{"test-shell-runner-1", "test-shell-runner-2"},
		},
		{
			name: "success removing all runners",
			setup: func(tb testing.TB) common.Network {
				tb.Helper()
				mn := common.NewMockNetwork(t)
				mn.On(
					"UnregisterRunner",
					mock.Anything,
				).Return(true)
				return mn
			},
			removeAllRunners: true,
			removedRunners:   []string{"test-docker-runner", "test-shell-runner-1", "test-shell-runner-2"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			oldConfig, err := os.ReadFile("./testdata/test-config.toml")
			require.NoError(t, err)

			t.Cleanup(func() {
				require.NoError(t, os.WriteFile("./testdata/test-config.toml", oldConfig, 0o600))
			})

			cmd := &UnregisterCommand{
				network:    tc.setup(t),
				ConfigFile: "./testdata/test-config.toml",
				Name:       tc.runnerName,
				AllRunners: tc.removeAllRunners,
			}
			cmd.Execute(&cli.Context{})

			postExecuteConfig := configfile.New("./testdata/test-config.toml")
			err = postExecuteConfig.Load()
			require.NoError(t, err)

			if tc.removeAllRunners {
				assert.Equal(t, 0, len(postExecuteConfig.Config().Runners))
			} else {
				assert.Greater(t, len(postExecuteConfig.Config().Runners), 0)
			}

			for _, runnerName := range tc.removedRunners {
				_, err = postExecuteConfig.Config().RunnerByName(runnerName)
				assert.Error(t, err)
				assert.ErrorContains(t, err, fmt.Sprintf("could not find a runner with the name '%s'", runnerName))
			}

			for _, runnerName := range tc.remainingRunners {
				_, err = postExecuteConfig.Config().RunnerByName(runnerName)
				assert.NoError(t, err)
			}
		})
	}
}
