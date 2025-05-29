package configfile

import "github.com/prometheus/client_golang/prometheus"

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
