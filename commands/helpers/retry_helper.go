package helpers

import (
	"time"

	"github.com/sirupsen/logrus"
)

type retryHelper struct {
	Retry     int           `long:"retry" description:"How many times to retry upload"`
	RetryTime time.Duration `long:"retry-time" description:"How long to wait between retries"`
}

// retryableErr indicates that an error can be retried. To specify that an error
// can be retried simply wrap the original error. For example:
//
// retryableErr{err: errors.New("some error")}
type retryableErr struct {
	err error
}

func (e retryableErr) Error() string {
	return e.err.Error()
}

func (r *retryHelper) doRetry(handler func() error) error {
	err := handler()

	for i := 0; i < r.Retry; i++ {
		if _, ok := err.(retryableErr); !ok {
			return err
		}

		time.Sleep(r.RetryTime)
		logrus.WithError(err).Warningln("Retrying...")

		err = handler()
	}

	return err
}
