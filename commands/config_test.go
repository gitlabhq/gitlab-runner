//go:build !integration
// +build !integration

package commands

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type metricsServerTestExample struct {
	address         string
	setAddress      bool
	expectedAddress string
	errorIsExpected bool
}

type metricsServerConfigurationType string

const (
	configurationFromCli    metricsServerConfigurationType = "from-cli"
	configurationFromConfig metricsServerConfigurationType = "from-config"
)

func testListenAddressSetting(
	t *testing.T,
	exampleName string,
	example metricsServerTestExample,
	testType metricsServerConfigurationType,
) {
	t.Run(fmt.Sprintf("%s-%s", exampleName, testType), func(t *testing.T) {
		cfg := configOptionsWithListenAddress{}
		cfg.config = &common.Config{}
		if example.setAddress {
			if testType == configurationFromCli {
				cfg.ListenAddress = example.address
			} else {
				cfg.config.ListenAddress = example.address
			}
		}

		address, err := cfg.listenAddress()
		assert.Equal(t, example.expectedAddress, address)
		if example.errorIsExpected {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}
	})
}

func TestMetricsServer(t *testing.T) {
	examples := map[string]metricsServerTestExample{
		"address-set-without-port": {"localhost", true, "localhost:9252", false},
		"port-set-without-address": {":1234", true, ":1234", false},
		"address-set-with-port":    {"localhost:1234", true, "localhost:1234", false},
		"address-is-empty":         {"", true, "", false},
		"address-is-invalid":       {"localhost::1234", true, "", true},
		"address-not-set":          {"", false, "", false},
	}

	for exampleName, example := range examples {
		testListenAddressSetting(t, exampleName, example, configurationFromCli)
		testListenAddressSetting(t, exampleName, example, configurationFromConfig)
	}
}

func TestGetConfig(t *testing.T) {
	c := &configOptions{}
	assert.Nil(t, c.getConfig())

	c.config = &common.Config{}
	assert.True(t, c.config != c.getConfig())
}

func TestRunnerByName(t *testing.T) {
	examples := map[string]struct {
		runners       []*common.RunnerConfig
		runnerName    string
		expectedIndex int
		expectedError error
	}{
		"finds runner by name": {
			runners: []*common.RunnerConfig{
				{
					Name: "runner1",
				},
				{
					Name: "runner2",
				},
			},
			runnerName:    "runner2",
			expectedIndex: 1,
		},
		"does not find non-existent runner": {
			runners: []*common.RunnerConfig{
				{
					Name: "runner1",
				},
				{
					Name: "runner2",
				},
			},
			runnerName:    "runner3",
			expectedIndex: -1,
			expectedError: fmt.Errorf("could not find a runner with the name 'runner3'"),
		},
	}

	for tn, tt := range examples {
		t.Run(tn, func(t *testing.T) {
			config := configOptions{
				config: &common.Config{
					Runners: tt.runners,
				},
			}

			runner, err := config.RunnerByName(tt.runnerName)
			if tt.expectedIndex == -1 {
				assert.Nil(t, runner)
			} else {
				assert.Equal(t, tt.runners[tt.expectedIndex], runner)
			}
			assert.Equal(t, tt.expectedError, err)
		})
	}
}

func TestRunnerByURLAndID(t *testing.T) {
	examples := map[string]struct {
		runners       []*common.RunnerConfig
		runnerURL     string
		runnerID      int64
		expectedIndex int
		expectedError error
	}{
		"finds runner by name": {
			runners: []*common.RunnerConfig{
				{
					RunnerCredentials: common.RunnerCredentials{
						ID:  1,
						URL: "https://gitlab1.example.com/",
					},
				},
				{
					RunnerCredentials: common.RunnerCredentials{
						ID:  2,
						URL: "https://gitlab1.example.com/",
					},
				},
			},
			runnerURL:     "https://gitlab1.example.com/",
			runnerID:      1,
			expectedIndex: 0,
		},
		"does not find runner with wrong ID": {
			runners: []*common.RunnerConfig{
				{
					RunnerCredentials: common.RunnerCredentials{
						ID:  1,
						URL: "https://gitlab1.example.com/",
					},
				},
				{
					RunnerCredentials: common.RunnerCredentials{
						ID:  2,
						URL: "https://gitlab1.example.com/",
					},
				},
			},
			runnerURL:     "https://gitlab1.example.com/",
			runnerID:      3,
			expectedIndex: -1,
			expectedError: fmt.Errorf(`could not find a runner with the URL "https://gitlab1.example.com/" and ID 3`),
		},
		"does not find runner with wrong URL": {
			runners: []*common.RunnerConfig{
				{
					RunnerCredentials: common.RunnerCredentials{
						ID:  1,
						URL: "https://gitlab1.example.com/",
					},
				},
				{
					RunnerCredentials: common.RunnerCredentials{
						ID:  2,
						URL: "https://gitlab1.example.com/",
					},
				},
			},
			runnerURL:     "https://gitlab2.example.com/",
			runnerID:      1,
			expectedIndex: -1,
			expectedError: fmt.Errorf(`could not find a runner with the URL "https://gitlab2.example.com/" and ID 1`),
		},
	}

	for tn, tt := range examples {
		t.Run(tn, func(t *testing.T) {
			config := configOptions{
				config: &common.Config{
					Runners: tt.runners,
				},
			}

			runner, err := config.RunnerByURLAndID(tt.runnerURL, tt.runnerID)
			if tt.expectedIndex == -1 {
				assert.Nil(t, runner)
			} else {
				assert.Equal(t, tt.runners[tt.expectedIndex], runner)
			}
			assert.Equal(t, tt.expectedError, err)
		})
	}
}
