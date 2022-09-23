//go:build !integration

package kubernetes

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/rest/fake"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/session/proxy"
)

func TestPoolGetter(t *testing.T) {
	pool := proxy.Pool{"test": &proxy.Proxy{Settings: fakeProxySettings()}}
	ex := executor{
		AbstractExecutor: executors.AbstractExecutor{
			ProxyPool: pool,
		},
	}

	assert.Equal(t, pool, ex.Pool())
}

func TestProxyRequestError(t *testing.T) {
	version, codec := testVersionAndCodec()
	objectInfo := metav1.ObjectMeta{Name: "test-pod", Namespace: "test-ns"}

	tests := map[string]struct {
		port            string
		podStatus       api.PodPhase
		containerReady  bool
		expectedErrCode int
	}{
		"Invalid port number": {
			port:            "81",
			podStatus:       api.PodRunning,
			expectedErrCode: http.StatusNotFound,
		},
		"Invalid port name": {
			port:            "foobar",
			podStatus:       api.PodRunning,
			expectedErrCode: http.StatusNotFound,
		},
		"Pod is not ready yet": {
			port:            "80",
			podStatus:       api.PodPending,
			expectedErrCode: http.StatusServiceUnavailable,
		},
		"Service containers are not ready yet": {
			port:            "80",
			podStatus:       api.PodRunning,
			containerReady:  false,
			expectedErrCode: http.StatusServiceUnavailable,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ex := executor{
				pod: &api.Pod{ObjectMeta: objectInfo},
				kubeClient: testKubernetesClient(
					version,
					fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
						return mockPodRunningStatus(
							req,
							version,
							codec,
							objectInfo,
							test.podStatus,
							test.containerReady,
						)
					})),
			}

			h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ex.ProxyRequest(w, r, "", test.port, fakeProxySettings())
			})

			rw := httptest.NewRecorder()
			req, err := http.NewRequest(http.MethodGet, "/", nil)
			require.NoError(t, err)

			h.ServeHTTP(rw, req)

			resp := rw.Result()
			assert.Equal(t, test.expectedErrCode, resp.StatusCode)
			defer resp.Body.Close()
		})
	}
}

func fakeProxySettings() *proxy.Settings {
	return &proxy.Settings{
		ServiceName: "name",
		Ports: []proxy.Port{
			{
				Number:   80,
				Protocol: "http",
				Name:     "port-name",
			},
		},
	}
}

func TestProxyRequestHTTP(t *testing.T) {
	version, codec := testVersionAndCodec()
	objectInfo := metav1.ObjectMeta{Name: "test-pod", Namespace: "test-ns"}
	defaultBody := "ACK"
	defaultPort := "80"
	defaultPortNumber, err := strconv.Atoi(defaultPort)
	require.NoError(t, err)

	serviceName := "service-name"
	proxyEndpointURI :=
		"/api/" + version + "/namespaces/" + objectInfo.Namespace + "/services/http:" +
			serviceName + ":" + defaultPort + "/proxy"
	defaultProxySettings := proxy.Settings{
		ServiceName: serviceName,
		Ports: []proxy.Port{
			{
				Number:   defaultPortNumber,
				Protocol: "http",
			},
		},
	}

	ex := executor{
		pod: &api.Pod{ObjectMeta: objectInfo},
	}

	tests := map[string]struct {
		podStatus          api.PodPhase
		requestedURI       string
		proxySettings      proxy.Settings
		endpointURI        string
		expectedBody       string
		expectedStatusCode int
	}{
		"Returns error if the pod is not ready": {
			podStatus:          api.PodPending,
			proxySettings:      defaultProxySettings,
			expectedBody:       "Service Unavailable\n",
			expectedStatusCode: http.StatusServiceUnavailable,
		},
		"Returns error if invalid port protocol": {
			podStatus: api.PodRunning,
			proxySettings: proxy.Settings{
				ServiceName: serviceName,
				Ports: []proxy.Port{
					{
						Number:   defaultPortNumber,
						Protocol: "whatever",
					},
				},
			},
			expectedBody:       "Service Unavailable\n",
			expectedStatusCode: http.StatusServiceUnavailable,
		},
		"Handles HTTP requests": {
			podStatus:          api.PodRunning,
			proxySettings:      defaultProxySettings,
			endpointURI:        proxyEndpointURI,
			expectedBody:       defaultBody,
			expectedStatusCode: http.StatusOK,
		},
		"Adds the requested URI to the proxy path": {
			podStatus:          api.PodRunning,
			requestedURI:       "foobar",
			proxySettings:      defaultProxySettings,
			endpointURI:        proxyEndpointURI + "/foobar",
			expectedBody:       defaultBody,
			expectedStatusCode: http.StatusOK,
		},
		"Uses the right protocol based on the proxy configuration": {
			podStatus: api.PodRunning,
			proxySettings: proxy.Settings{
				ServiceName: serviceName,
				Ports: []proxy.Port{
					{
						Number:   defaultPortNumber,
						Protocol: "https",
					},
				},
			},
			endpointURI: "/api/" + version + "/namespaces/" + objectInfo.Namespace + "/services/https:" +
				serviceName + ":" + defaultPort + "/proxy",
			expectedBody:       defaultBody,
			expectedStatusCode: http.StatusOK,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ex.ProxyRequest(w, r, test.requestedURI, defaultPort, &test.proxySettings)
			})

			ex.kubeClient = testKubernetesClient(
				version,
				fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
					switch p, m := req.URL.Path, req.Method; {
					case p == test.endpointURI && m == http.MethodGet:
						return &http.Response{
							StatusCode: http.StatusOK,
							Body:       io.NopCloser(bytes.NewReader([]byte(defaultBody))),
						}, nil
					default:
						return mockPodRunningStatus(req, version, codec, objectInfo, test.podStatus, true)
					}
				}))

			rw := httptest.NewRecorder()
			req, err := http.NewRequest(http.MethodGet, "/", nil)
			require.NoError(t, err)

			h.ServeHTTP(rw, req)

			resp := rw.Result()
			defer resp.Body.Close()

			b, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Equal(t, test.expectedStatusCode, resp.StatusCode)
			assert.Equal(t, test.expectedBody, string(b))
		})
	}
}

func TestProxyRequestHTTPError(t *testing.T) {
	version, codec := testVersionAndCodec()
	objectInfo := metav1.ObjectMeta{Name: "test-pod", Namespace: "test-ns"}

	ex := executor{
		pod: &api.Pod{ObjectMeta: objectInfo},
	}

	proxySettings := proxy.Settings{
		ServiceName: "service-name",
		Ports: []proxy.Port{
			{
				Number:   80,
				Protocol: "http",
			},
		},
	}

	endpointURI := "/api/" + version + "/namespaces/" + objectInfo.Namespace + "/services/http:service-name:80/proxy"
	errorMessage := "Error Message"

	tests := map[string]struct {
		expectedErrorCode int
		expectedErrorMsg  string
	}{
		"Error is StatusServiceUnavailable": {
			expectedErrorCode: http.StatusServiceUnavailable,
			expectedErrorMsg:  "",
		},
		"Any other error": {
			expectedErrorCode: http.StatusNotFound,
			expectedErrorMsg:  errorMessage,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ex.ProxyRequest(w, r, "", "80", &proxySettings)
			})

			ex.kubeClient = testKubernetesClient(
				version,
				fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
					switch p, m := req.URL.Path, req.Method; {
					case p == endpointURI && m == http.MethodGet:
						return &http.Response{
							StatusCode: test.expectedErrorCode,
							Body:       io.NopCloser(bytes.NewReader([]byte(errorMessage))),
						}, nil
					default:
						return mockPodRunningStatus(req, version, codec, objectInfo, api.PodRunning, true)
					}
				}))

			rw := httptest.NewRecorder()
			req, err := http.NewRequest(http.MethodGet, "/", nil)
			require.NoError(t, err)

			h.ServeHTTP(rw, req)

			resp := rw.Result()
			defer resp.Body.Close()

			b, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Equal(t, test.expectedErrorCode, resp.StatusCode)
			assert.Equal(t, test.expectedErrorMsg, string(b))
		})
	}
}

func mockPodRunningStatus(
	req *http.Request,
	version string,
	codec runtime.Codec,
	objectInfo metav1.ObjectMeta,
	status api.PodPhase,
	servicesReady bool,
) (*http.Response, error) {
	switch p, m := req.URL.Path, req.Method; {
	case p == "/api/"+version+"/namespaces/"+objectInfo.Namespace+"/pods/"+objectInfo.Name && m == http.MethodGet:
		pod := &api.Pod{
			ObjectMeta: objectInfo,
			Status: api.PodStatus{
				Phase:             status,
				ContainerStatuses: []api.ContainerStatus{{Ready: servicesReady}},
			},
		}
		return &http.Response{StatusCode: http.StatusOK, Body: objBody(codec, pod), Header: map[string][]string{
			"Content-Type": {"application/json"},
		}}, nil
	default:
		return nil, errors.New("unexpected request")
	}
}

func TestProxyRequestWebsockets(t *testing.T) {
	version, codec := testVersionAndCodec()
	objectInfo := metav1.ObjectMeta{Name: "test-pod", Namespace: "test-ns"}
	defaultPort := "80"
	defaultPortNumber, err := strconv.Atoi(defaultPort)
	require.NoError(t, err)

	serviceName := "service-name"
	proxyEndpointURI :=
		"/api/" + version + "/namespaces/" + objectInfo.Namespace + "/services/http:" +
			serviceName + ":" + defaultPort + "/proxy"
	defaultProxySettings := proxy.Settings{
		ServiceName: serviceName,
		Ports: []proxy.Port{
			{
				Number:   defaultPortNumber,
				Protocol: "http",
			},
		},
	}

	ex := executor{
		AbstractExecutor: executors.AbstractExecutor{
			Config: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Host: "localhost",
					},
				},
			},
		},
		configurationOverwrites: &overwrites{},
		pod:                     &api.Pod{ObjectMeta: objectInfo},
	}

	tests := map[string]struct {
		podStatus          api.PodPhase
		requestedURI       string
		proxySettings      proxy.Settings
		endpointURI        string
		expectedStatusCode int
	}{
		"Returns error if the service is not ready": {
			podStatus:          api.PodPending,
			proxySettings:      defaultProxySettings,
			expectedStatusCode: http.StatusServiceUnavailable,
		},
		"Returns error if invalid port protocol": {
			podStatus: api.PodRunning,
			proxySettings: proxy.Settings{
				ServiceName: serviceName,
				Ports: []proxy.Port{
					{
						Number:   80,
						Protocol: "whatever",
					},
				},
			},
			expectedStatusCode: http.StatusServiceUnavailable,
		},
		"Handles Websockets requests": {
			podStatus:          api.PodRunning,
			proxySettings:      defaultProxySettings,
			endpointURI:        proxyEndpointURI,
			expectedStatusCode: http.StatusSwitchingProtocols,
		},
		"Adds the requested URI to the proxy path": {
			podStatus:          api.PodRunning,
			requestedURI:       "foobar",
			proxySettings:      defaultProxySettings,
			endpointURI:        proxyEndpointURI + "/foobar",
			expectedStatusCode: http.StatusSwitchingProtocols,
		},
		"Uses the right protocol based on the proxy configuration": {
			podStatus: api.PodRunning,
			proxySettings: proxy.Settings{
				ServiceName: "service-name",
				Ports: []proxy.Port{
					{
						Number:   80,
						Protocol: "https",
					},
				},
			},
			endpointURI: "/api/" + version + "/namespaces/" + objectInfo.Namespace +
				"/services/https:service-name:80/proxy",
			expectedStatusCode: http.StatusSwitchingProtocols,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ex.ProxyRequest(w, r, r.URL.Path, defaultPort, &test.proxySettings)
			})

			// Mocked Kubernetes API server making the proxy request
			kubeAPISrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, test.endpointURI, r.URL.Path)

				upgrader := websocket.Upgrader{}
				c, err := upgrader.Upgrade(w, r, nil)
				require.NoError(t, err)

				for {
					mt, message, err := c.ReadMessage()
					if err != nil {
						break
					}
					err = c.WriteMessage(mt, message)
					if err != nil {
						break
					}
				}
				defer c.Close()
			}))
			defer kubeAPISrv.Close()

			ex.kubeClient = mockKubernetesClientWithHost(
				version,
				kubeAPISrv.Listener.Addr().String(),
				fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
					return mockPodRunningStatus(req, version, codec, objectInfo, test.podStatus, true)
				}))

			// HTTP server
			srv := httptest.NewServer(h)
			defer srv.Close()

			u := url.URL{
				Scheme: "ws",
				Host:   srv.Listener.Addr().String(),
				Path:   test.requestedURI,
			}

			conn, resp, _ := websocket.DefaultDialer.Dial(u.String(), http.Header{})
			defer func() {
				resp.Body.Close()
				if conn != nil {
					_ = conn.Close()
				}
			}()

			assert.Equal(t, test.expectedStatusCode, resp.StatusCode)

			if resp.StatusCode == http.StatusSwitchingProtocols {
				testMessage := "testmessage"
				err := conn.WriteMessage(websocket.TextMessage, []byte(testMessage))
				require.NoError(t, err)

				_, p, err := conn.ReadMessage()
				require.NoError(t, err)
				assert.Equal(t, testMessage, string(p))
			}
		})
	}
}

func mockKubernetesClientWithHost(version string, host string, httpClient *http.Client) *kubernetes.Clientset {
	conf := restclient.Config{
		Host: host,
		ContentConfig: restclient.ContentConfig{
			GroupVersion: &schema.GroupVersion{Version: version},
		},
	}
	kube := kubernetes.NewForConfigOrDie(&conf)
	fakeClient := fake.RESTClient{Client: httpClient}
	kube.CoreV1().RESTClient().(*restclient.RESTClient).Client = fakeClient.Client
	kube.ExtensionsV1beta1().RESTClient().(*restclient.RESTClient).Client = fakeClient.Client

	return kube
}
