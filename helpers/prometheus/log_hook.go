package prometheus

import (
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

var numErrorsDesc = prometheus.NewDesc(
	"gitlab_runner_errors_total",
	"The number of caught errors.",
	[]string{"level"},
	nil,
)

type LogHook struct {
	errorsNumber map[logrus.Level]*int64
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
	atomic.AddInt64(lh.errorsNumber[entry.Level], 1)
	return nil
}

func (lh *LogHook) Describe(ch chan<- *prometheus.Desc) {
	ch <- numErrorsDesc
}

func (lh *LogHook) Collect(ch chan<- prometheus.Metric) {
	for _, level := range lh.Levels() {
		number := float64(atomic.LoadInt64(lh.errorsNumber[level]))
		ch <- prometheus.MustNewConstMetric(numErrorsDesc, prometheus.CounterValue, number, level.String())
	}
}

func NewLogHook() LogHook {
	lh := LogHook{}

	levels := lh.Levels()
	lh.errorsNumber = make(map[logrus.Level]*int64, len(levels))
	for _, level := range levels {
		lh.errorsNumber[level] = new(int64)
	}

	return lh
}
