package commands

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/network"
)

func getDefaultConfigFile() string {
	return filepath.Join(getDefaultConfigDirectory(), "config.toml")
}

func getDefaultCertificateDirectory() string {
	return filepath.Join(getDefaultConfigDirectory(), "certs")
}

type configOptions struct {
	config *common.Config

	ConfigFile string `short:"c" long:"config" env:"CONFIG_FILE" description:"Config file"`
}

func (c *configOptions) saveConfig() error {
	return c.config.SaveConfig(c.ConfigFile)
}

func (c *configOptions) loadConfig() error {
	config := common.NewConfig()
	err := config.LoadConfig(c.ConfigFile)
	if err != nil {
		return err
	}
	c.config = config
	return nil
}

func (c *configOptions) touchConfig() error {
	// try to load existing config
	err := c.loadConfig()
	if err != nil {
		return err
	}

	// save config for the first time
	if !c.config.Loaded {
		return c.saveConfig()
	}
	return nil
}

func (c *configOptions) RunnerByName(name string) (*common.RunnerConfig, error) {
	if c.config == nil {
		return nil, fmt.Errorf("Config has not been loaded")
	}

	for _, runner := range c.config.Runners {
		if runner.Name == name {
			return runner, nil
		}
	}

	return nil, fmt.Errorf("Could not find a runner with the name '%s'", name)
}

type configOptionsWithListenAddress struct {
	configOptions

	ListenAddress string `long:"listen-address" env:"LISTEN_ADDRESS" description:"Metrics / pprof server listening address"`

	// TODO: Remove in 12.0
	MetricsServerAddress string `long:"metrics-server" env:"METRICS_SERVER" description:"(DEPRECATED) Metrics / pprof server listening address"` //DEPRECATED
}

func (c *configOptionsWithListenAddress) listenAddress() (string, error) {
	address := c.listenOrMetricsServerAddress()

	if address == "" {
		return "", nil
	}

	_, port, err := net.SplitHostPort(address)
	if err != nil && !strings.Contains(err.Error(), "missing port in address") {
		return "", err
	}

	if len(port) == 0 {
		return fmt.Sprintf("%s:%d", address, common.DefaultMetricsServerPort), nil
	}
	return address, nil
}

func (c *configOptionsWithListenAddress) listenOrMetricsServerAddress() string {
	if c.ListenAddress != "" {
		return c.ListenAddress
	}

	// TODO: Remove in 12.0
	if c.MetricsServerAddress != "" {
		logrus.Warnln("'metrics-server' command line option is deprecated and will be removed in one of future releases; please use 'listen-address' instead")

		return c.MetricsServerAddress
	}

	return c.config.ListenOrServerMetricAddress()
}

func init() {
	configFile := os.Getenv("CONFIG_FILE")
	if configFile == "" {
		os.Setenv("CONFIG_FILE", getDefaultConfigFile())
	}

	network.CertificateDirectory = getDefaultCertificateDirectory()
}
