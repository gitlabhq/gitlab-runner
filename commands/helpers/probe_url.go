package helpers

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/log"
)

const defaultProbeURLTimeout = 15 * time.Second

// ProbeURLCommand checks that an HTTP(S) URL answers. Any HTTP response
// passes, transport failures (DNS, routing, timeout, TLS) fail. The
// boot-verify canary runs it inside the job environment.
type ProbeURLCommand struct {
	URL      string `long:"url" description:"URL to probe"`
	Timeout  int    `long:"timeout" description:"Probe timeout in seconds (default 15)"`
	CAFile   string `long:"ca-file" description:"PEM file with the CA certificates to verify the server against, instead of the system roots"`
	Insecure bool   `long:"insecure" description:"Skip TLS certificate verification"`
}

func NewProbeURLCommand() cli.Command {
	return common.NewCommand("probe-url", "check that an HTTP(S) URL answers at the HTTP level (internal)", &ProbeURLCommand{})
}

func (c *ProbeURLCommand) probe() error {
	if c.URL == "" {
		return fmt.Errorf("missing --url")
	}

	timeout := time.Duration(c.Timeout) * time.Second
	if timeout <= 0 {
		timeout = defaultProbeURLTimeout
	}

	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	switch {
	case c.Insecure:
		tlsConfig.InsecureSkipVerify = true //nolint:gosec
	case c.CAFile != "":
		// Trust only the given chain, matching how git uses sslCAInfo.
		pem, err := os.ReadFile(c.CAFile)
		if err != nil {
			return fmt.Errorf("reading --ca-file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return fmt.Errorf("no CA certificates found in %s", c.CAFile)
		}
		tlsConfig.RootCAs = pool
	}

	// Clone the default transport to keep proxy environment support: a
	// proxied network path that git would use must not fail the probe.
	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return fmt.Errorf("unexpected default transport type %T", http.DefaultTransport)
	}
	transport := defaultTransport.Clone()
	transport.TLSClientConfig = tlsConfig

	client := &http.Client{
		Timeout:   timeout,
		Transport: transport,
		// Don't follow redirects: they could point at hosts the runner
		// configuration never named.
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Get(c.URL)
	if err != nil {
		return fmt.Errorf("probing %s: %w", c.URL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	fmt.Printf("received HTTP %d: %s is reachable\n", resp.StatusCode, c.URL)
	return nil
}

func (c *ProbeURLCommand) Execute(*cli.Context) {
	log.SetRunnerFormatter()

	if err := c.probe(); err != nil {
		logrus.Fatalln(err)
	}
}
