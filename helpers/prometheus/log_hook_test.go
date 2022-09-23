//go:build !integration

package prometheus

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func callFireConcurrent(lh *LogHook, repeats int, finish chan bool) {
	for i := 0; i < repeats; i++ {
		_ = lh.Fire(&logrus.Entry{
			Level: logrus.ErrorLevel,
		})
		finish <- true
	}
}

func TestConcurrentFireCall(t *testing.T) {
	lh := NewLogHook()
	finish := make(chan bool)

	times := 5
	repeats := 100
	total := times * repeats

	for i := 0; i < times; i++ {
		go callFireConcurrent(&lh, repeats, finish)
	}

	finished := 0
	for {
		if finished >= total {
			break
		}

		<-finish
		finished++
	}

	assert.Equal(t, int64(total), *lh.errorsNumber[logrus.ErrorLevel], "Should fire log_hook N times")
}

func callCollectConcurrent(lh *LogHook, repeats int, ch chan<- prometheus.Metric, finish chan bool) {
	for i := 0; i < repeats; i++ {
		lh.Collect(ch)
		finish <- true
	}
}

func TestCouncurrentFireCallWithCollect(t *testing.T) {
	lh := NewLogHook()
	finish := make(chan bool)
	ch := make(chan prometheus.Metric)

	times := 5
	repeats := 100
	total := times * repeats * 2

	go func() {
		for {
			<-ch
		}
	}()

	for i := 0; i < times; i++ {
		go callFireConcurrent(&lh, repeats, finish)
		go callCollectConcurrent(&lh, repeats, ch, finish)
	}

	finished := 0
	for {
		if finished >= total {
			break
		}

		<-finish
		finished++
	}

	assert.Equal(t, int64(total/2), *lh.errorsNumber[logrus.ErrorLevel], "Should fire log_hook N times")
}
