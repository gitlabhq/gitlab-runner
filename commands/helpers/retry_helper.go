package helpers

import (
	"fmt"
	"net/http"
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

func (e retryableErr) Unwrap() error {
	return e.err
}

func (e retryableErr) Error() string {
	return e.err.Error()
}

func (r *retryHelper) doRetry(handler func(int) error) error {
	err := handler(0)

	for retry := 1; retry <= r.Retry; retry++ {
		if _, ok := err.(retryableErr); !ok {
			return err
		}

		time.Sleep(r.RetryTime)
		logrus.WithError(err).Warningln("Retrying...")

		err = handler(retry)
	}

	return err
}

// retryOnServerError will take the response and check if the the error should
// be of type retryableErr or not. When the status code is of 5xx it will be a
// retryableErr.
func retryOnServerError(resp *http.Response) error {
	if resp.StatusCode/100 == 2 {
		return nil
	}

	_ = resp.Body.Close()

	err := fmt.Errorf("received: %s", resp.Status)

	if resp.StatusCode/100 == 5 {
		err = retryableErr{err: err}
	}

	return err
}
