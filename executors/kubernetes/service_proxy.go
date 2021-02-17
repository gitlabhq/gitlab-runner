package kubernetes

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	terminal "gitlab.com/gitlab-org/gitlab-terminal"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8net "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/rest"

	"gitlab.com/gitlab-org/gitlab-runner/session/proxy"
)

const runningState = "Running"

func (s *executor) Pool() proxy.Pool {
	return s.ProxyPool
}

func (s *executor) newProxy(serviceName string, ports []proxy.Port) *proxy.Proxy {
	return &proxy.Proxy{
		Settings:          proxy.NewProxySettings(serviceName, ports),
		ConnectionHandler: s,
	}
}

func (s *executor) ProxyRequest(
	w http.ResponseWriter,
	r *http.Request,
	requestedURI string,
	port string,
	settings *proxy.Settings,
) {
	logger := logrus.WithFields(logrus.Fields{
		"uri":      r.RequestURI,
		"method":   r.Method,
		"port":     port,
		"settings": settings,
	})

	portSettings, err := settings.PortByNameOrNumber(port)
	if err != nil {
		logger.WithError(err).Errorf("port proxy %q not found", port)
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	if !s.servicesRunning() {
		logger.Errorf("services are not ready yet")
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return
	}

	if websocket.IsWebSocketUpgrade(r) {
		proxyWSRequest(s, w, r, requestedURI, portSettings, settings, logger)
		return
	}

	proxyHTTPRequest(s, w, r, requestedURI, portSettings, settings, logger)
}

func (s *executor) servicesRunning() bool {
	// TODO: handle the context properly with https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27932
	pod, err := s.kubeClient.CoreV1().Pods(s.pod.Namespace).Get(context.TODO(), s.pod.Name, metav1.GetOptions{})
	if err != nil || pod.Status.Phase != runningState {
		return false
	}

	for _, container := range pod.Status.ContainerStatuses {
		if !container.Ready {
			return false
		}
	}

	return true
}

func (s *executor) serviceEndpointRequest(
	verb, serviceName, requestedURI string,
	port proxy.Port,
) (*rest.Request, error) {
	scheme, err := port.Scheme()
	if err != nil {
		return nil, err
	}

	result := s.kubeClient.CoreV1().RESTClient().Verb(verb).
		Namespace(s.pod.Namespace).
		Resource("services").
		SubResource("proxy").
		Name(k8net.JoinSchemeNamePort(scheme, serviceName, strconv.Itoa(port.Number))).
		Suffix(requestedURI)

	return result, nil
}

func proxyWSRequest(
	s *executor,
	w http.ResponseWriter,
	r *http.Request,
	requestedURI string,
	port proxy.Port,
	proxySettings *proxy.Settings,
	logger *logrus.Entry,
) {
	// In order to avoid calling this method, and use one of its own,
	// we should refactor the library "gitlab.com/gitlab-org/gitlab-terminal"
	// and make it more generic, not so terminal focused, with a broader
	// terminology. (https://gitlab.com/gitlab-org/gitlab-runner/issues/4059)
	settings, err := s.getTerminalSettings()
	if err != nil {
		logger.WithError(err).Errorf("service proxy: error getting WS settings")
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return
	}

	req, err := s.serviceEndpointRequest(r.Method, proxySettings.ServiceName, requestedURI, port)
	if err != nil {
		logger.WithError(err).Errorf("service proxy: error proxying WS request")
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return
	}

	u := req.URL()
	u.Scheme = proxy.WebsocketProtocolFor(u.Scheme)

	settings.Url = u.String()
	serviceProxy := terminal.NewWebSocketProxy(1)

	terminal.ProxyWebSocket(w, r, settings, serviceProxy)
}

func proxyHTTPRequest(
	s *executor,
	w http.ResponseWriter,
	r *http.Request,
	requestedURI string,
	port proxy.Port,
	proxy *proxy.Settings,
	logger *logrus.Entry,
) {
	req, err := s.serviceEndpointRequest(r.Method, proxy.ServiceName, requestedURI, port)
	if err != nil {
		logger.WithError(err).Errorf("service proxy: error proxying HTTP request")
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return
	}

	// TODO: handle the context properly with https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27932
	body, err := req.Stream(context.TODO())
	if err != nil {
		message, code := handleProxyHTTPErr(err, logger)
		w.WriteHeader(code)

		if message != "" {
			_, _ = fmt.Fprint(w, message)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, body)
}

func handleProxyHTTPErr(err error, logger *logrus.Entry) (string, int) {
	statusError, ok := err.(*errors.StatusError)
	if !ok {
		return "", http.StatusInternalServerError
	}

	code := int(statusError.Status().Code)
	// When the error is a 503 we don't want to give any information
	// coming from Kubernetes
	if code == http.StatusServiceUnavailable {
		logger.Error(statusError.Status().Message)
		return "", code
	}

	details := statusError.Status().Details
	if details == nil {
		return "", code
	}

	causes := details.Causes
	if len(causes) > 0 {
		return causes[0].Message, code
	}

	return "", code
}
