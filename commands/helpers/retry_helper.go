package helpers

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// Cloud Providers supported currently send error in case of HTTP API request failure in XML Format
// The Format spec is the same for:
// GCS: https://cloud.google.com/storage/docs/xml-api/reference-status
// AWS S3: https://docs.aws.amazon.com/AmazonS3/latest/API/ErrorResponses.html#RESTErrorResponses
// and Azure Blob Storage: https://learn.microsoft.com/en-us/rest/api/storageservices/status-and-error-codes2
// storageErrorResponse is used to deserialize such error responses and provide better error failures message in the log.
type storageErrorResponse struct {
	XMLName xml.Name `xml:"Error"`
	Code    string   `xml:"Code"`
	Message string   `xml:"Message"`
}

func (ser *storageErrorResponse) isValid() bool {
	return ser.Code != "" || ser.Message != ""
}

func (ser *storageErrorResponse) String() string {
	if !ser.isValid() {
		return ""
	}

	msg := ""
	if ser.Code != "" {
		msg = "code: " + ser.Code
	}

	if ser.Message != "" {
		msg += ", message: " + ser.Message
	}

	return msg
}

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

	errResp := &storageErrorResponse{}
	bodyBytes, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	errMsg := fmt.Sprintf("received: %s", resp.Status)

	if err := xml.Unmarshal(bodyBytes, errResp); err == nil && errResp.isValid() {
		errMsg = fmt.Sprintf("%s. Request failed with %s", errMsg, errResp.String())
	}

	err := errors.New(errMsg)

	if resp.StatusCode/100 == 5 {
		err = retryableErr{err: err}
	}

	return err
}
