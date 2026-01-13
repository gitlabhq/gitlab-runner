package router

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"gitlab.com/gitlab-org/gitlab-runner/router/internal/wstunnel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

const (
	protocolGRPC  = "grpc"
	protocolGRPCS = "grpcs"
	protocolWS    = "ws"
	protocolWSS   = "wss"

	webSocketMaxMessageSize = 10 * 1024 * 1024 // matches kas limit
	// tunnelWebSocketProtocol is a subprotocol that allows client and server to recognize each other.
	// See https://datatracker.ietf.org/doc/html/rfc6455#section-11.3.4
	tunnelWebSocketProtocol = "ws-tunnel"
)

type ClientConn interface {
	grpc.ClientConnInterface
	Done()
}

type DialTarget struct {
	URL         string
	Token       string
	TLSCAFile   string
	TLSCertFile string
	TLSKeyFile  string
}

type connHolder struct {
	conn        *grpc.ClientConn
	mu          sync.Mutex
	numUsers    int32
	shouldClose bool
	closed      bool
}

func (h *connHolder) Invoke(ctx context.Context, method string, args any, reply any, opts ...grpc.CallOption) error {
	return h.conn.Invoke(ctx, method, args, reply, opts...)
}

func (h *connHolder) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return h.conn.NewStream(ctx, desc, method, opts...)
}

func (h *connHolder) Done() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.numUsers--
	if h.numUsers < 0 {
		panic("Done() called more than once")
	}
	if !h.shouldClose {
		return
	}
	h.maybeCloseLocked()
}

// Tells the connHolder to close the underlying connection when the last user calls Done().
// Returns true if the connection was closed earlier or during this call.
func (h *connHolder) scheduleClose() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.shouldClose = true
	return h.maybeCloseLocked() // close straight away if there are no users
}

func (h *connHolder) maybeCloseLocked() bool {
	if h.numUsers == 0 && !h.closed {
		_ = h.conn.Close()
		h.closed = true
	}
	return h.closed
}

func (h *connHolder) forceClose() {
	h.mu.Lock()
	defer h.mu.Unlock()
	_ = h.conn.Close()
	h.closed = true
}

func (h *connHolder) isClosed() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.closed
}

func (h *connHolder) addUser() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.numUsers++
}

// ClientConnFactory is a connection pool of maximum size 1.
// It maintains at most one active connection and zero or more connections with in-flight RPCs.
type ClientConnFactory struct {
	certDirectory string
	userAgent     string

	mu                   sync.Mutex
	currentConn          *connHolder // nil or current connection.
	currentDialTarget    DialTarget
	currentConstructedAt time.Time
	closingConns         []*connHolder // connections that are marked to be closed but still have users.
	closed               bool
}

func NewClientConnFactory(certDirectory, userAgent string) *ClientConnFactory {
	return &ClientConnFactory{
		certDirectory: certDirectory,
		userAgent:     userAgent,
	}
}

func (f *ClientConnFactory) Dial(target DialTarget) (ClientConn, error) {
	target, err := f.maybeSetCertificates(target)
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return nil, errors.New("pool has been closed, cannot dial up new connections")
	}
	f.gcClosedConnsLocked()
	if f.currentConn != nil {
		if !f.isStaleConnLocked(target) {
			f.currentConn.addUser()
			return f.currentConn, nil
		}
		if !f.currentConn.scheduleClose() {
			f.closingConns = append(f.closingConns, f.currentConn)
		}
		f.currentConn = nil
	}
	c, err := f.newConn(target)
	if err != nil {
		return nil, err
	}
	c.addUser()
	f.currentConn = c
	f.currentDialTarget = target
	f.currentConstructedAt = time.Now()
	return c, nil
}

// Shutdown closes the underlying connection(s). It doesn't wait for any in-flight RPCs to finish - the caller should
// ensure they are done before calling this method if dropping them is not desired.
func (f *ClientConnFactory) Shutdown() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return
	}
	f.closed = true
	if f.currentConn != nil {
		f.currentConn.forceClose()
		f.currentConn = nil
	}
	for _, h := range f.closingConns {
		h.forceClose()
	}
	f.closingConns = nil
}

func (f *ClientConnFactory) isStaleConnLocked(target DialTarget) bool {
	if f.currentDialTarget != target {
		return true
	}
	return f.isFileNewerLocked(target.TLSCAFile) || f.isFileNewerLocked(target.TLSCertFile) || f.isFileNewerLocked(target.TLSKeyFile)
}

func (f *ClientConnFactory) isFileNewerLocked(name string) bool {
	stat, err := os.Stat(name)
	return err == nil && f.currentConstructedAt.Before(stat.ModTime())
}

func (f *ClientConnFactory) newConn(target DialTarget) (*connHolder, error) {
	u, err := url.Parse(target.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid kas address: %w", err)
	}
	var tlsConfig *tls.Config
	if u.Scheme == protocolWSS || u.Scheme == protocolGRPCS {
		tlsConfig, err = maybeConstructTLSConfig(target.TLSCAFile, target.TLSCertFile, target.TLSKeyFile)
		if err != nil {
			return nil, err
		}
	}
	var opts []grpc.DialOption
	var addressToDial string
	// "grpcs" is the only scheme where encryption is done by gRPC.
	// "wss" is secure too but gRPC cannot know that, so we tell it it's not.
	secure := u.Scheme == protocolGRPCS
	switch u.Scheme {
	case protocolWS, protocolWSS:
		// See https://github.com/grpc/grpc/blob/master/doc/naming.md.
		addressToDial = "passthrough:" + target.URL
		dialer := net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}
		opts = append(opts, grpc.WithContextDialer(wstunnel.DialerForGRPC(
			webSocketMaxMessageSize,
			websocket.Dialer{
				NetDialContext:   dialer.DialContext,
				Proxy:            http.ProxyFromEnvironment,
				TLSClientConfig:  tlsConfig,
				HandshakeTimeout: 10 * time.Second,
				Subprotocols:     []string{tunnelWebSocketProtocol},
			},
			http.Header{
				"Authorization": []string{"Bearer " + target.Token},
				"User-Agent":    []string{f.userAgent},
			},
		)))
	case protocolGRPC:
		// See https://github.com/grpc/grpc/blob/master/doc/naming.md.
		addressToDial = "dns:" + hostWithPort(u)
		opts = append(opts,
			// See https://github.com/grpc/grpc/blob/master/doc/service_config.md.
			// See https://github.com/grpc/grpc/blob/master/doc/load-balancing.md.
			grpc.WithDefaultServiceConfig(`{"loadBalancingConfig":[{"round_robin":{}}]}`),
		)
	case protocolGRPCS:
		// See https://github.com/grpc/grpc/blob/master/doc/naming.md.
		addressToDial = "dns:" + hostWithPort(u)
		opts = append(opts,
			grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
			// See https://github.com/grpc/grpc/blob/master/doc/service_config.md.
			// See https://github.com/grpc/grpc/blob/master/doc/load-balancing.md.
			grpc.WithDefaultServiceConfig(`{"loadBalancingConfig":[{"round_robin":{}}]}`),
		)
	default:
		return nil, fmt.Errorf("unsupported scheme in GitLab Kubernetes Agent Server address: %q", u.Scheme)
	}
	if !secure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	opts = append(opts,
		grpc.WithUserAgent(f.userAgent),
		// keepalive.ClientParameters must be specified at least as large as what is allowed by the
		// Server-side grpc.KeepaliveEnforcementPolicy
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			// kas allows min 20 seconds, trying to stay below 60 seconds (typical load-balancer timeout) and
			// above kas' Server keepalive Time so that kas pings the client sometimes. This helps mitigate
			// reverse-proxies' enforced Server response timeout.
			Time:                55 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.WithPerRPCCredentials(newTokenCredentials(target.Token, !secure)),
	)
	conn, err := grpc.NewClient(addressToDial, opts...)
	if err != nil {
		return nil, fmt.Errorf("gRPC.dial: %w", err)
	}
	return &connHolder{
		conn: conn,
	}, nil
}

func (f *ClientConnFactory) gcClosedConnsLocked() {
	f.closingConns = slices.DeleteFunc(f.closingConns, func(c *connHolder) bool {
		return c.isClosed()
	})
}

func (f *ClientConnFactory) findCertificate(currentValue, fileName string) (string, error) {
	if currentValue != "" {
		return currentValue, nil
	}
	path := filepath.Join(f.certDirectory, fileName)
	_, err := os.Stat(path)
	switch {
	case os.IsNotExist(err):
		return "", nil
	case err == nil:
		return path, nil
	default:
		return "", err
	}
}

func (f *ClientConnFactory) maybeSetCertificates(target DialTarget) (DialTarget, error) {
	u, err := url.Parse(target.URL)
	if err != nil {
		return DialTarget{}, err
	}
	host := u.Hostname()
	target.TLSCAFile, err = f.findCertificate(target.TLSCAFile, host+".crt")
	if err != nil {
		return DialTarget{}, err
	}
	target.TLSCertFile, err = f.findCertificate(target.TLSCertFile, host+".auth.crt")
	if err != nil {
		return DialTarget{}, err
	}
	target.TLSKeyFile, err = f.findCertificate(target.TLSKeyFile, host+".auth.key")
	if err != nil {
		return DialTarget{}, err
	}
	return target, nil
}

func maybeConstructTLSConfig(caFile, certFile, keyFile string) (*tls.Config, error) {
	rootCAs, err := maybeLoadRootCAs(caFile)
	if err != nil {
		return nil, err
	}
	cert, err := maybeLoadCertificate(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		RootCAs:      rootCAs,
		Certificates: cert,
	}, nil
}

func maybeLoadRootCAs(caFile string) (*x509.CertPool, error) {
	if caFile == "" {
		return nil, nil
	}
	pool, err := loadCACert(caFile)
	if err != nil {
		if os.IsNotExist(err) {
			// As if there was no file when the client was constructed. Log for debugging.
			logrus.WithError(err).Errorln("Failed to load", caFile)
			return nil, nil
		}
		return nil, fmt.Errorf("CA certificate: %w", err)
	}
	return pool, nil
}

func maybeLoadCertificate(certFile, keyFile string) ([]tls.Certificate, error) {
	if certFile == "" || keyFile == "" {
		return nil, nil
	}
	certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		if os.IsNotExist(err) {
			// As if there was no file when the client was constructed. Log for debugging.
			logrus.WithError(err).Errorln("Failed to load", certFile, keyFile)
			return nil, nil
		} else {
			return nil, fmt.Errorf("TLS certificate: %w", err)
		}
	}
	return []tls.Certificate{certificate}, nil
}

func loadCACert(caCertFile string) (*x509.CertPool, error) {
	certPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("SystemCertPool: %w", err)
	}
	caCert, err := os.ReadFile(caCertFile) //nolint: gosec
	if err != nil {
		return nil, fmt.Errorf("CA certificate file: %w", err)
	}
	ok := certPool.AppendCertsFromPEM(caCert)
	if !ok {
		return nil, fmt.Errorf("AppendCertsFromPEM(%s) failed", caCertFile)
	}
	return certPool, nil
}

// hostWithPort adds port if it was not specified in a URL with a "grpc" or "grpcs" scheme.
func hostWithPort(u *url.URL) string {
	port := u.Port()
	if port != "" {
		return u.Host
	}
	switch u.Scheme {
	case protocolGRPC:
		return net.JoinHostPort(u.Host, "80")
	case protocolGRPCS:
		return net.JoinHostPort(u.Host, "443")
	default:
		// Function called with unknown scheme, just return the original host.
		return u.Host
	}
}
