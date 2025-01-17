//go:build integration && kubernetes

package watchers_test

import (
	"bufio"
	"cmp"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	logrusTest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors/kubernetes/internal/watchers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	namespace = cmp.Or(os.Getenv("CI_RUNNER_TEST_NAMESPACE"), common.DefaultKubernetesIntegrationTestNamespace)
	labels    = map[string]string{"some": "label"}
)

// TestPodWatcherConnectionIssues tests how the pod watcher reacts to certain connection issues.
func TestPodWatcherConnectionIssues(t *testing.T) {
	tests := map[string]struct {
		BackoffDuration  time.Duration
		DisruptOnStart   Disrupter
		DisruptAtRuntime Disrupter
		ExpectStartErr   string
	}{
		"no issue": {
			BackoffDuration: time.Second * 3,
		},
		"issues at start": {
			BackoffDuration: time.Second * 3,
			DisruptOnStart: func(t *testing.T, proxies *Proxies) {
				t.Log("stopping inner proxy")
				proxies.Inner.Stop()
			},
			ExpectStartErr: "not synced: *v1.Pod",
		},
		"issues at start which resolve in time": {
			BackoffDuration: time.Second * 20,
			DisruptOnStart: func(t *testing.T, proxies *Proxies) {
				proxy := proxies.Inner
				rollbackAfter := time.Second * 3
				err := fmt.Errorf("some network error")

				go func() {
					t.Log("disrupting connection")

					orgTransport := proxy.Handler.Transport
					defer func() {
						proxy.Handler.Transport = orgTransport
						proxy.Server.CloseClientConnections()
						t.Log("connection disruption rolled back")
					}()

					proxy.Handler.Transport = Transport(func(*http.Request) (*http.Response, error) {
						return nil, err
					})
					proxy.Server.CloseClientConnections()
					time.Sleep(rollbackAfter)
				}()
			},
		},
		"issues at runtime": {
			DisruptAtRuntime: func(t *testing.T, proxies *Proxies) {
				t.Log("stopping inner proxy")
				proxies.Inner.Server.CloseClientConnections()
				proxies.Inner.Stop()
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.TODO())
			defer cancel()

			proxies := setupProxyChain(t, ctx)

			fakeLogger, _ := logrusTest.NewNullLogger()
			kubeClient := proxies.Outer.Client

			test.DisruptOnStart.Disrupt(t, proxies)

			watcher := watchers.NewPodWatcher(ctx, fakeLogger, kubeClient, namespace, labels, test.BackoffDuration)
			err := watcher.Start()
			if assertError(t, err, test.ExpectStartErr, "starting pod watcher") {
				return
			}
			defer watcher.Stop()

			test.DisruptAtRuntime.Disrupt(t, proxies)

			assertNoErrorOnChannel(t, time.Second*2, watcher.Errors())
		})
	}
}

func assertNoErrorOnChannel(t *testing.T, to time.Duration, ch <-chan error) {
	select {
	case err := <-ch:
		assert.NoError(t, err, "expected no error")
	case <-time.After(to):
		return
	}
}

type Disrupter func(*testing.T, *Proxies)

func (d Disrupter) Disrupt(t *testing.T, p *Proxies) {
	if d != nil {
		d(t, p)
	}
}

type Transport func(*http.Request) (*http.Response, error)

func (t Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t == nil {
		return http.DefaultTransport.RoundTrip(req)
	}
	return t(req)
}

func assertError(t *testing.T, err error, s string, msgAndArgs ...any) bool {
	if s == "" {
		return !assert.NoError(t, err, msgAndArgs...)
	}
	return assert.ErrorContains(t, err, s, msgAndArgs...)
}

type Proxies struct {
	Kubectl struct {
		URL    *url.URL
		Client kubernetes.Interface
		Stop   context.CancelFunc
	}
	Inner struct {
		URL     *url.URL
		Client  kubernetes.Interface
		Stop    context.CancelFunc
		Server  *httptest.Server
		Handler *httputil.ReverseProxy
	}
	Outer struct {
		URL     *url.URL
		Client  kubernetes.Interface
		Stop    context.CancelFunc
		Server  *httptest.Server
		Handler *httputil.ReverseProxy
	}
}

// setupProxyChain sets up a chain of proxies in front of the actual kubeAPI, so that we can intercept/interrupt the
// connections.
//
//	kubeAPI
//	  ^--- kubectlProxy ... uses the kubectl CLI to create a local proxy
//	         ^--- innerProxy ... this is where we inject some errors
//	                 ^--- outerProxy  ... this is where the pod watcher actually connects to, so it has a stable
//	                                      connection endpoint, whilst we are still able to produce connection errors in
//	                                      the innerProxy or the kubectlProxy
func setupProxyChain(t *testing.T, ctx context.Context) *Proxies {
	p := &Proxies{}

	p.Kubectl.URL, p.Kubectl.Stop = kubectlProxy(t, ctx)
	p.Kubectl.Client = getKubeClient(t, p.Kubectl.URL.String())

	p.Inner.Server, p.Inner.Handler, p.Inner.Stop = reverseProxy(t, ctx, p.Kubectl.URL)
	p.Inner.URL = parseURL(t, p.Inner.Server.URL)
	p.Inner.Client = getKubeClient(t, p.Inner.Server.URL)

	p.Outer.Server, p.Outer.Handler, p.Outer.Stop = reverseProxy(t, ctx, p.Inner.URL)
	p.Outer.URL = parseURL(t, p.Outer.Server.URL)
	p.Outer.Client = getKubeClient(t, p.Outer.Server.URL)

	return p
}

func getKubeClient(t *testing.T, url string) kubernetes.Interface {
	config, err := clientcmd.BuildConfigFromFlags(url, "")
	require.NoError(t, err, "getting client config for url %s", url)
	clientSet, err := kubernetes.NewForConfig(config)
	require.NoError(t, err, "creating client set for url %s", url)
	return clientSet
}

func parseURL(t *testing.T, u string) *url.URL {
	p, err := url.Parse(u)
	require.NoError(t, err, "parsing URL: %s", u)
	return p
}

// reverseProxy starts a reverse proxy in front of upstreamURL.
// It returns the http server and the proxy handler, so that users can interact with and intercept connections as they
// see fit.
func reverseProxy(t *testing.T, ctx context.Context, upstreamURL *url.URL) (*httptest.Server, *httputil.ReverseProxy, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)
	stopped := make(chan struct{})

	proxy := httputil.NewSingleHostReverseProxy(upstreamURL)
	server := httptest.NewServer(proxy)

	go func() {
		<-ctx.Done()
		server.Close()
		close(stopped)
	}()

	t.Cleanup(func() {
		<-stopped
	})

	return server, proxy, cancel
}

// kubectlProxy starts a proxy in front of the kubeAPI, using kubectl. This handles auth, TLS, ... and we can talk plain
// http now.
func kubectlProxy(t *testing.T, ctx context.Context) (*url.URL, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)
	stopped := make(chan struct{})

	cmd := exec.CommandContext(ctx, "kubectl", "proxy", "--port=0")

	stdoutPipe, err := cmd.StdoutPipe()
	require.NoError(t, err, "setting up stdout pipe")

	err = cmd.Start()
	require.NoError(t, err, "starting kubectl-proxy")

	go func() {
		// free resources asap
		_ = cmd.Wait()
		close(stopped)
	}()

	t.Cleanup(func() {
		// wait for the process shutdown before we shut down the test
		<-stopped
	})

	stdoutReader := bufio.NewReader(stdoutPipe)
	for {
		line, err := stdoutReader.ReadString('\n')
		require.NoError(t, err, "reading stdout line")

		rawURL, ok := strings.CutPrefix(line, "Starting to serve on ")
		if !ok {
			continue
		}

		u, err := url.Parse("http://" + strings.Trim(rawURL, "\n\r "))
		require.NoError(t, err, "parsing kubectl-proxy URL")

		return u, cancel
	}
}
