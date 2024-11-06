package runner_wrapper

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	defaultShutdownCallbackTimeout = 10 * time.Second
)

//go:generate mockery --name=shutdownCallbackDef --inpackage --with-expecter
type shutdownCallbackDef interface {
	URL() string
	Method() string
	Headers() map[string]string
}

//go:generate mockery --name=shutdownCallback --inpackage --with-expecter
type shutdownCallback interface {
	Run(ctx context.Context)
}

type defaultShutdownCallback struct {
	log logrus.FieldLogger

	url     string
	method  string
	headers map[string]string
}

func newShutdownCallback(log logrus.FieldLogger, def shutdownCallbackDef) shutdownCallback {
	return &defaultShutdownCallback{
		log:     log,
		url:     def.URL(),
		method:  def.Method(),
		headers: def.Headers(),
	}
}

func (s *defaultShutdownCallback) Run(ctx context.Context) {
	s.log.Info("Running shutdown callback call")

	tctx, cancelFn := context.WithTimeout(ctx, defaultShutdownCallbackTimeout)
	defer cancelFn()

	req, err := http.NewRequestWithContext(tctx, s.method, s.url, nil)
	if err != nil {
		s.log.WithError(err).Error("Could not create shutdown callback request")
		return
	}

	for k, v := range s.headers {
		req.Header.Add(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.log.WithError(err).Error("Shutdown callback request failure")
		return
	}

	defer func() {
		_ = resp.Body.Close()
	}()
	_, _ = io.Copy(io.Discard, resp.Body)

	s.log.
		WithField("status-code", resp.StatusCode).
		WithField("status", resp.Status).
		Info("Received shutdown callback response")
}
