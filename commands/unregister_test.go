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

const (
	testRunner1 = "test-runner-1"
	testRunner2 = "test-runner-2"
	testToken1  = "test-token-1"
	testToken2  = "test-token-2"
)

var (
	testRunnerConfig1 = common.RunnerConfig{
		Name:              testRunner1,
		RunnerCredentials: common.RunnerCredentials{Token: testToken1},
	}
	testRunnerConfig2 = common.RunnerConfig{
		Name:              testRunner2,
		RunnerCredentials: common.RunnerCredentials{Token: testToken2},
	}
)

func TestUnregisterCommand_unregisterAllRunner(t *testing.T) {
	testCases := []struct {
		name            string
		cfgs            []*common.RunnerConfig
		setup           func(tb testing.TB) common.Network
		expectedRunners []*common.RunnerConfig
		expectedErr     string
	}{
		{
			name: "successfully unregister all runners",
			cfgs: []*common.RunnerConfig{
				&testRunnerConfig1,
				&testRunnerConfig2,
			},
			setup: func(tb testing.TB) common.Network {
				tb.Helper()
				mn := common.NewMockNetwork(t)
				mn.On(
					"UnregisterRunner",
					testRunnerConfig1,
				).Once().Return(true)
				mn.On(
					"UnregisterRunner",
					testRunnerConfig2,
				).Once().Return(true)
				return mn
			},
		},
		{
			name: "successfully unregister some runners",
			cfgs: []*common.RunnerConfig{
				&testRunnerConfig1,
				&testRunnerConfig2,
			},
			setup: func(tb testing.TB) common.Network {
				tb.Helper()
				mn := common.NewMockNetwork(t)
				mn.On(
					"UnregisterRunner",
					testRunnerConfig1,
				).Once().Return(true)
				mn.On(
					"UnregisterRunner",
					testRunnerConfig2,
				).Once().Return(false)
				return mn
			},
			expectedRunners: []*common.RunnerConfig{
				&testRunnerConfig2,
			},
			expectedErr: `failed to unregister runner "test-runner-2"`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := UnregisterCommand{network: tc.setup(t)}

			runners, err := cmd.unregisterAllRunners(&common.Config{Runners: tc.cfgs})

			if tc.expectedErr != "" {
				assert.Error(t, err)
				assert.ErrorContains(t, err, tc.expectedErr)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tc.expectedRunners, runners)
		})
	}
}

func TestUnregisterCommand_unregisterSingleRunner(t *testing.T) {
	testCases := []struct {
		name            string
		cfg             *common.Config
		runnerName      string
		runnerConfig    common.RunnerConfig
		setup           func(tb testing.TB) common.Network
		expectedRunners []*common.RunnerConfig
		expectedErr     string
	}{
		{
			name: "unregister with runner creds",
			cfg: &common.Config{
				Runners: []*common.RunnerConfig{
					&testRunnerConfig1,
					&testRunnerConfig2,
				},
			},
			runnerConfig: testRunnerConfig1,
			setup: func(tb testing.TB) common.Network {
				tb.Helper()
				mn := common.NewMockNetwork(t)
				mn.On(
					"UnregisterRunner",
					testRunnerConfig1,
				).Return(true)
				return mn
			},
			expectedRunners: []*common.RunnerConfig{
				&testRunnerConfig2,
			},
		},
		{
			name: "unregister with runner name",
			cfg: &common.Config{
				Runners: []*common.RunnerConfig{
					&testRunnerConfig1,
					&testRunnerConfig2,
				},
			},
			runnerName: testRunner1,
			setup: func(tb testing.TB) common.Network {
				tb.Helper()
				mn := common.NewMockNetwork(t)
				mn.On(
					"UnregisterRunner",
					testRunnerConfig1,
				).Return(true)
				return mn
			},
			expectedRunners: []*common.RunnerConfig{
				&testRunnerConfig2,
			},
		},
		{
			name: "unregister with runner name and creds",
			cfg: &common.Config{
				Runners: []*common.RunnerConfig{
					&testRunnerConfig1,
					&testRunnerConfig2,
				},
			},
			runnerName:   testRunner2,
			runnerConfig: testRunnerConfig2,
			setup: func(tb testing.TB) common.Network {
				tb.Helper()
				mn := common.NewMockNetwork(t)
				mn.On(
					"UnregisterRunner",
					testRunnerConfig2,
				).Return(true)
				return mn
			},
			expectedRunners: []*common.RunnerConfig{
				&testRunnerConfig1,
			},
		},
		{
			name:       "name not found",
			cfg:        &common.Config{},
			runnerName: "not-found-runner",
			setup: func(tb testing.TB) common.Network {
				tb.Helper()
				return common.NewMockNetwork(t)
			},
			expectedErr: "could not find a runner with the name 'not-found-runner'",
		},
		{
			name: "token not found",
			cfg:  &common.Config{},
			runnerConfig: common.RunnerConfig{
				RunnerCredentials: common.RunnerCredentials{Token: "not-found-token"},
			},
			setup: func(tb testing.TB) common.Network {
				tb.Helper()
				return common.NewMockNetwork(t)
			},
			expectedErr: "could not find a runner with the token 'not-found'",
		},
		{
			name: "missing name or token",
			cfg:  &common.Config{},
			setup: func(tb testing.TB) common.Network {
				tb.Helper()
				return common.NewMockNetwork(t)
			},
			expectedErr: "at least one of --name or --token must be specified",
		},
		{
			name: "unregister failure",
			cfg: &common.Config{
				Runners: []*common.RunnerConfig{
					&testRunnerConfig1,
					&testRunnerConfig2,
				},
			},
			runnerConfig: testRunnerConfig1,
			runnerName:   testRunner1,
			setup: func(tb testing.TB) common.Network {
				tb.Helper()
				mn := common.NewMockNetwork(t)
				mn.On(
					"UnregisterRunner",
					testRunnerConfig1,
				).Return(false)
				return mn
			},
			expectedErr: `failed to unregister runner "test-runner-1"`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := UnregisterCommand{
				network:           tc.setup(t),
				Name:              tc.runnerName,
				RunnerCredentials: tc.runnerConfig.RunnerCredentials,
			}

			runners, err := cmd.unregisterSingleRunner(tc.cfg)

			if tc.expectedErr != "" {
				assert.Error(t, err)
				assert.ErrorContains(t, err, tc.expectedErr)
				assert.Nil(t, runners)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedRunners, runners)
			}
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

			result := cmd.unregisterRunner(common.RunnerConfig{RunnerCredentials: common.RunnerCredentials{Token: tc.token}}, tc.systemID)

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
		{
			name: "partial failure removing all runners",
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
				).Once().Return(true)
				mn.On(
					"UnregisterRunner",
					mock.Anything,
				).Once().Return(false)
				return mn
			},
			removeAllRunners: true,
			remainingRunners: []string{"test-shell-runner-2"},
			removedRunners:   []string{"test-docker-runner", "test-shell-runner-1"},
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

			for _, runnerName := range tc.removedRunners {
				_, err = postExecuteConfig.Config().RunnerByName(runnerName)
				assert.Error(t, err)
				assert.ErrorContains(t, err, fmt.Sprintf("could not find a runner with the name '%s'", runnerName))
			}

			assert.Len(t, postExecuteConfig.Config().Runners, len(tc.remainingRunners))
			for _, runnerName := range tc.remainingRunners {
				_, err = postExecuteConfig.Config().RunnerByName(runnerName)
				assert.NoError(t, err)
			}
		})
	}
}
