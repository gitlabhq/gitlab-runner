package commands

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"
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

func testMetricsServerSetting(t *testing.T, exampleName string, example metricsServerTestExample, testType metricsServerConfigurationType) {
	t.Run(fmt.Sprintf("%s-%s", exampleName, testType), func(t *testing.T) {
		cfg := configOptionsWithMetricsServer{}
		cfg.config = &common.Config{}
		if example.setAddress {
			if testType == configurationFromCli {
				cfg.MetricsServerAddress = example.address
			} else {
				cfg.config.MetricsServerAddress = example.address
			}
		}

		address, err := cfg.metricsServerAddress()
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
		testMetricsServerSetting(t, exampleName, example, configurationFromCli)
		testMetricsServerSetting(t, exampleName, example, configurationFromConfig)
	}
}
