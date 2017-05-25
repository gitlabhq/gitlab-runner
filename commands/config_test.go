package commands

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"
)

func TestMetricsServerNone(t *testing.T) {
	cfg := configOptionsWithMetricsServer{}
	cfg.config = &common.Config{}

	address, err := cfg.metricsServerAddress()
	assert.NoError(t, err)
	assert.Equal(t, address, "")
}

func TestMetricsServerEmpty(t *testing.T) {
	cfg := configOptionsWithMetricsServer{}
	cfg.config = &common.Config{}
	cfg.MetricsServerAddress = ""

	address, err := cfg.metricsServerAddress()
	assert.NoError(t, err)
	assert.Equal(t, address, "")
}

func TestMetricsServerEmptyCommonConfig(t *testing.T) {
	cfg := configOptionsWithMetricsServer{}
	cfg.config = &common.Config{}
	cfg.config.MetricsServerAddress = ""

	address, err := cfg.metricsServerAddress()
	assert.NoError(t, err)
	assert.Equal(t, address, "")
}

func TestMetricsServerInvalid(t *testing.T) {
	cfg := configOptionsWithMetricsServer{}
	cfg.config = &common.Config{}
	cfg.MetricsServerAddress = "localhost::1234"

	address, err := cfg.metricsServerAddress()
	assert.Error(t, err)
	assert.Equal(t, address, "")
}

func TestMetricsServerInvalidCommonConfig(t *testing.T) {
	cfg := configOptionsWithMetricsServer{}
	cfg.config = &common.Config{}
	cfg.config.MetricsServerAddress = "localhost::1234"

	address, err := cfg.metricsServerAddress()
	assert.Error(t, err)
	assert.Equal(t, address, "")
}

func TestMetricsServerDefaultPort(t *testing.T) {
	cfg := configOptionsWithMetricsServer{}
	cfg.config = &common.Config{}
	cfg.MetricsServerAddress = "localhost"

	address, err := cfg.metricsServerAddress()
	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("%s:%d", cfg.MetricsServerAddress, common.DefaultMetricsServerPort), address)
}

func TestMetricsServerDefaultPortCommonConfig(t *testing.T) {
	cfg := configOptionsWithMetricsServer{}
	cfg.config = &common.Config{}
	cfg.config.MetricsServerAddress = "localhost"

	address, err := cfg.metricsServerAddress()
	assert.NoError(t, err)
	assert.Equal(t, address, fmt.Sprintf("%s:%d", cfg.config.MetricsServerAddress, common.DefaultMetricsServerPort))
}

func TestMetricsServerDoesNotTouchExistingPort(t *testing.T) {
	cfg := configOptionsWithMetricsServer{}
	cfg.config = &common.Config{}
	cfg.MetricsServerAddress = "localhost:1234"

	address, err := cfg.metricsServerAddress()
	assert.NoError(t, err)
	assert.Equal(t, address, cfg.MetricsServerAddress)
}

func TestMetricsServerDoesNotTouchExistingPortCommonConfig(t *testing.T) {
	cfg := configOptionsWithMetricsServer{}
	cfg.config = &common.Config{}
	cfg.config.MetricsServerAddress = "localhost:1234"

	address, err := cfg.metricsServerAddress()
	assert.NoError(t, err)
	assert.Equal(t, address, cfg.config.MetricsServerAddress)
}
