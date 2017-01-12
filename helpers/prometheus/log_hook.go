package prometheus

import (
	"github.com/Sirupsen/logrus"
	"github.com/prometheus/client_golang/prometheus"
)

var numErrorsDesc = prometheus.NewDesc("ci_runner_errors", "The  number of catched errors.", []string{"level"}, nil)

type LogHook struct {
	errorsNumber map[logrus.Level]float64
}

func (lh *LogHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
		logrus.WarnLevel,
	}
}

func (lh *LogHook) Fire(entry *logrus.Entry) error {
	lh.errorsNumber[entry.Level]++
	return nil
}

func (lh *LogHook) Describe(ch chan<- *prometheus.Desc) {
	ch <- numErrorsDesc
}

func (lh *LogHook) Collect(ch chan<- prometheus.Metric) {
	for level, number := range lh.errorsNumber {
		ch <- prometheus.MustNewConstMetric(numErrorsDesc, prometheus.CounterValue, number, level.String())
	}
}

func NewLogHook() LogHook {
	lh := LogHook{}

	levels := lh.Levels()
	lh.errorsNumber = make(map[logrus.Level]float64, len(levels))
	for _, level := range levels {
		lh.errorsNumber[level] = 0
	}

	return lh
}
