package prometheus

import (
	"testing"

	"github.com/Sirupsen/logrus"
)

func callFireConcurrent(t *testing.T, lh *LogHook, repeats int, finish chan bool) {
	for i := 0; i < repeats; i++ {
		lh.Fire(&logrus.Entry{
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
		go callFireConcurrent(t, &lh, repeats, finish)
	}

	finished := 0
	for {
		if finished >= total {
			break
		}

		<-finish
		finished++
	}
}
