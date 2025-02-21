package configfile

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type ConfigFile struct {
	mu       sync.Mutex
	cfg      *common.Config
	systemID string

	pathname        string
	accessCollector *configAccessCollector
}

func New(pathname string, opts ...Option) *ConfigFile {
	var options options
	for _, opt := range opts {
		opt(&options)
	}

	cfg := &ConfigFile{pathname: pathname}
	if options.AccessCollector {
		cfg.accessCollector = newConfigAccessCollector()
	}
	cfg.cfg = options.Config
	cfg.systemID = options.SystemID

	return cfg
}

func (cf *ConfigFile) Load(opts ...LoadOption) error {
	var options loadOptions
	for _, opt := range opts {
		opt(&options)
	}

	cf.mu.Lock()
	defer cf.mu.Unlock()

	config := common.NewConfig()
	err := config.LoadConfig(cf.pathname)
	if err != nil {
		if cf.accessCollector != nil {
			cf.accessCollector.loadingError.Inc()
		}
		return err
	}

	// restore config saver
	if cf.cfg != nil {
		config.ConfigSaver = cf.cfg.ConfigSaver
	}

	// config validation is best-effort
	if err := validate(config); err != nil {
		logrus.Infof(
			"There might be a problem with your config based on "+
				"jsonschema annotations in common/config.go "+
				"(experimental feature):\n%v\n",
			err,
		)
	}

	if cf.accessCollector != nil {
		cf.accessCollector.loaded.Inc()
	}

	if cf.systemID == "" {
		systemIDState, err := newSystemIDState(filepath.Join(filepath.Dir(cf.pathname), ".runner_system_id"))
		if err != nil {
			return fmt.Errorf("loading system ID file: %w", err)
		}
		cf.systemID = systemIDState.GetSystemID()
	}

	cf.cfg = config
	for _, runnerCfg := range cf.cfg.Runners {
		runnerCfg.SystemID = cf.systemID
		runnerCfg.ConfigLoadedAt = time.Now()
		runnerCfg.ConfigDir = filepath.Dir(cf.pathname)
	}

	for _, mutate := range options.Mutate {
		if err := mutate(cf.cfg); err != nil {
			return fmt.Errorf("mutate config: %w", err)
		}
	}

	return nil
}

func (cf *ConfigFile) SystemID() string {
	return cf.systemID
}

func (cf *ConfigFile) Save() error {
	err := cf.cfg.SaveConfig(cf.pathname)
	if err != nil {
		if cf.accessCollector != nil {
			cf.accessCollector.savingError.Inc()
		}

		return err
	}

	if cf.accessCollector != nil {
		cf.accessCollector.saved.Inc()
	}

	return nil
}

func (cf *ConfigFile) Config() *common.Config {
	cf.mu.Lock()
	defer cf.mu.Unlock()

	return cf.cfg
}

func (cf *ConfigFile) AccessCollector() prometheus.Collector {
	return cf.accessCollector
}
