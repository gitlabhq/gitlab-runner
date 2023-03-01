package commands

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/network"
)

func GetDefaultConfigFile() string {
	return filepath.Join(getDefaultConfigDirectory(), "config.toml")
}

func getDefaultCertificateDirectory() string {
	return filepath.Join(getDefaultConfigDirectory(), "certs")
}

var (
	_ prometheus.Collector = &configAccessCollector{}
)

type configAccessCollector struct {
	loadingError prometheus.Counter
	loaded       prometheus.Counter
	savingError  prometheus.Counter
	saved        prometheus.Counter
}

func newConfigAccessCollector() *configAccessCollector {
	return &configAccessCollector{
		loadingError: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gitlab_runner_configuration_loading_error_total",
			Help: "Total number of times the configuration file was not loaded by Runner process due to errors",
		}),
		loaded: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gitlab_runner_configuration_loaded_total",
			Help: "Total number of times the configuration file was loaded by Runner process",
		}),
		savingError: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gitlab_runner_configuration_saving_error_total",
			Help: "Total number of times the configuration file was not saved by Runner process due to errors",
		}),
		saved: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gitlab_runner_configuration_saved_total",
			Help: "Total number of times the configuration file was saved by Runner process",
		}),
	}
}

func (c *configAccessCollector) Describe(descs chan<- *prometheus.Desc) {
	c.loadingError.Describe(descs)
	c.loaded.Describe(descs)
	c.savingError.Describe(descs)
	c.saved.Describe(descs)
}

func (c *configAccessCollector) Collect(metrics chan<- prometheus.Metric) {
	c.loadingError.Collect(metrics)
	c.loaded.Collect(metrics)
	c.savingError.Collect(metrics)
	c.saved.Collect(metrics)
}

type configOptions struct {
	configMutex         sync.Mutex
	config              *common.Config
	loadedSystemIDState *common.SystemIDState

	configAccessCollector *configAccessCollector

	ConfigFile string `short:"c" long:"config" env:"CONFIG_FILE" description:"Config file"`
}

// getConfig returns a copy of the config as it was during the function call.
// This makes sure the properties of the config won't change after the mutex
// in this function has been unlocked. We don't change the inner objects,
// which makes a shallow copy OK in this case, if that ever changes we should
// look into deep copying or other alternatives.
// All writes of the config property should be protected by a mutex while
// all reads should use this function.
func (c *configOptions) getConfig() *common.Config {
	c.configMutex.Lock()
	defer c.configMutex.Unlock()

	if c.config == nil {
		return nil
	}

	config := *c.config
	return &config
}

func (c *configOptions) saveConfig() error {
	err := c.config.SaveConfig(c.ConfigFile)
	if err != nil {
		c.onConfigurationAccessCollector(func(m *configAccessCollector) {
			m.savingError.Inc()
		})

		return err
	}

	c.onConfigurationAccessCollector(func(m *configAccessCollector) {
		m.saved.Inc()
	})

	return nil
}

func (c *configOptions) onConfigurationAccessCollector(callback func(*configAccessCollector)) {
	if c.configAccessCollector == nil {
		return
	}

	callback(c.configAccessCollector)
}

func (c *configOptions) loadConfig() error {
	c.configMutex.Lock()
	defer c.configMutex.Unlock()

	config := common.NewConfig()
	err := config.LoadConfig(c.ConfigFile)
	if err != nil {
		c.onConfigurationAccessCollector(func(m *configAccessCollector) {
			m.loadingError.Inc()
		})

		return err
	}

	// Config validation is best-effort
	if err := common.Validate(config); err != nil {
		logrus.Warningf("There might be a problem with your config\n%v", err)
	}

	c.onConfigurationAccessCollector(func(m *configAccessCollector) {
		m.loaded.Inc()
	})

	systemIDState, err := c.loadSystemID(filepath.Join(filepath.Dir(c.ConfigFile), ".runner_system_id"))
	if err != nil {
		return fmt.Errorf("loading system ID file: %w", err)
	}

	c.config = config
	for _, runnerCfg := range c.config.Runners {
		runnerCfg.SystemIDState = systemIDState
	}

	c.loadedSystemIDState = systemIDState

	return nil
}

func (c *configOptions) loadSystemID(filePath string) (*common.SystemIDState, error) {
	systemIDState := common.NewSystemIDState()
	err := systemIDState.LoadFromFile(filePath)
	if err != nil {
		return nil, err
	}

	// ensure we have a system ID
	if systemIDState.GetSystemID() == "" {
		err = systemIDState.EnsureSystemID()
		if err != nil {
			return nil, err
		}

		err = systemIDState.SaveConfig(filePath)
		if err != nil {
			logrus.
				WithFields(logrus.Fields{
					"state_file": filePath,
					"system_id":  systemIDState.GetSystemID(),
				}).
				Warningf("Couldn't save new system ID on state file. "+
					"This file will be mandatory in GitLab Runner 16.0 and later.\n"+
					"Please ensure there is text file at the location specified in `state_file` "+
					"with the contents of `system_id`. Example: echo %q > %q\n", systemIDState.GetSystemID(), filePath)
			// TODO return error starting in %16.0, see https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29591
		}
	}

	return systemIDState, nil
}

func (c *configOptions) RunnerByName(name string) (*common.RunnerConfig, error) {
	config := c.getConfig()
	if config == nil {
		return nil, fmt.Errorf("config has not been loaded")
	}

	for _, runner := range config.Runners {
		if runner.Name == name {
			return runner, nil
		}
	}

	return nil, fmt.Errorf("could not find a runner with the name '%s'", name)
}

func (c *configOptions) RunnerByURLAndID(url string, id int64) (*common.RunnerConfig, error) {
	if c.config == nil {
		return nil, fmt.Errorf("config has not been loaded")
	}

	for _, runner := range c.config.Runners {
		if runner.URL == url && runner.ID == id {
			return runner, nil
		}
	}

	return nil, fmt.Errorf("could not find a runner with the URL %q and ID %d", url, id)
}

//nolint:lll
type configOptionsWithListenAddress struct {
	configOptions

	ListenAddress string `long:"listen-address" env:"LISTEN_ADDRESS" description:"Metrics / pprof server listening address"`
}

func (c *configOptionsWithListenAddress) listenAddress() (string, error) {
	address := c.getConfig().ListenAddress
	if c.ListenAddress != "" {
		address = c.ListenAddress
	}

	if address == "" {
		return "", nil
	}

	_, port, err := net.SplitHostPort(address)
	if err != nil && !strings.Contains(err.Error(), "missing port in address") {
		return "", err
	}

	if port == "" {
		return fmt.Sprintf("%s:%d", address, common.DefaultMetricsServerPort), nil
	}
	return address, nil
}

func init() {
	configFile := os.Getenv("CONFIG_FILE")
	if configFile == "" {
		err := os.Setenv("CONFIG_FILE", GetDefaultConfigFile())
		if err != nil {
			logrus.WithError(err).Fatal("Couldn't set CONFIG_FILE environment variable")
		}
	}

	network.CertificateDirectory = getDefaultCertificateDirectory()
}
