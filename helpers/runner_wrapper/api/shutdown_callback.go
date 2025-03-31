package api

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

type ShutdownCallbackDef interface {
	URL() string
	Method() string
	Headers() map[string]string
}

type ShutdownCallback interface {
	Run(ctx context.Context)
}

type defaultShutdownCallbackDef struct {
	url     string
	method  string
	headers map[string]string
}

func NewShutdownCallbackDef(url string, method string, headers map[string]string) ShutdownCallbackDef {
	return &defaultShutdownCallbackDef{
		url:     url,
		method:  method,
		headers: headers,
	}
}

func (d *defaultShutdownCallbackDef) URL() string {
	return d.url
}

func (d *defaultShutdownCallbackDef) Method() string {
	return d.method
}

func (d *defaultShutdownCallbackDef) Headers() map[string]string {
	return d.headers
}

type defaultShutdownCallback struct {
	log logrus.FieldLogger

	url     string
	method  string
	headers map[string]string
}

func NewShutdownCallback(log logrus.FieldLogger, def ShutdownCallbackDef) ShutdownCallback {
	return &defaultShutdownCallback{
		log:     log,
		url:     def.URL(),
		method:  def.Method(),
		headers: def.Headers(),
	}
}

func (s *defaultShutdownCallback) URL() string {
	return s.url
}

func (s *defaultShutdownCallback) Method() string {
	return s.method
}

func (s *defaultShutdownCallback) Headers() map[string]string {
	m := make(map[string]string, len(s.headers))
	for k, v := range s.headers {
		m[k] = v
	}

	return m
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
