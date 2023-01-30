package network

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jpillora/backoff"
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/tls/ca_chain"
	url_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/url"
)

const jsonMimeType = "application/json"
const applicationXMLMimeType = "application/xml"
const textXMLMimeType = "text/xml"

type requestCredentials interface {
	GetURL() string
	GetToken() string
	GetTLSCAFile() string
	GetTLSCertFile() string
	GetTLSKeyFile() string
}

var dialer = net.Dialer{
	Timeout:   30 * time.Second,
	KeepAlive: 30 * time.Second,
}

const (
	backOffMinDelay    = 100 * time.Millisecond
	backOffMaxDelay    = 60 * time.Second
	backOffDelayFactor = 2.0
	backOffDelayJitter = true
)

type client struct {
	http.Client
	url             *url.URL
	caFile          string
	certFile        string
	keyFile         string
	caData          []byte
	updateTime      time.Time
	lastUpdate      string
	requestBackOffs map[string]*backoff.Backoff
	lock            sync.Mutex

	requester requester
}

type ResponseTLSData struct {
	CAChain  string
	CertFile string
	KeyFile  string
}

func (n *client) getLastUpdate() string {
	return n.lastUpdate
}

func (n *client) setLastUpdate(headers http.Header) {
	if lu := headers.Get("X-GitLab-Last-Update"); len(lu) > 0 {
		n.lastUpdate = lu
	}
}

func (n *client) ensureTLSConfig() {
	// certificate got modified
	if stat, err := os.Stat(n.caFile); err == nil && n.updateTime.Before(stat.ModTime()) {
		n.Transport = nil
	}

	// client certificate got modified
	if stat, err := os.Stat(n.certFile); err == nil && n.updateTime.Before(stat.ModTime()) {
		n.Transport = nil
	}

	// client private key got modified
	if stat, err := os.Stat(n.keyFile); err == nil && n.updateTime.Before(stat.ModTime()) {
		n.Transport = nil
	}

	// create or update transport
	if n.Transport == nil {
		n.updateTime = time.Now()
		n.createTransport()
	}
}

func (n *client) addTLSCA(tlsConfig *tls.Config) {
	// load TLS CA certificate
	file := n.caFile
	if file == "" {
		return
	}

	logrus.Debugln("Trying to load", file, "...")

	data, err := os.ReadFile(file)
	if err != nil {
		if !os.IsNotExist(err) {
			logrus.Errorln("Failed to load", n.caFile, err)
		}
		return
	}

	pool, err := x509.SystemCertPool()
	if err != nil {
		logrus.Warningln("Failed to load system CertPool:", err)
	}
	if pool == nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(data) {
		logrus.Errorln("Failed to parse PEM in", n.caFile)
		return
	}

	tlsConfig.RootCAs = pool
	n.caData = data
}

func (n *client) addTLSAuth(tlsConfig *tls.Config) {
	if n.certFile == "" || n.keyFile == "" {
		return
	}

	logrus.Debugln("Trying to load", n.certFile, "and", n.keyFile, "pair...")

	// load TLS client keypair
	certificate, err := tls.LoadX509KeyPair(n.certFile, n.keyFile)
	if err != nil {
		if !os.IsNotExist(err) {
			logrus.Errorln("Failed to load", n.certFile, n.keyFile, err)
		}
		return
	}

	tlsConfig.Certificates = []tls.Certificate{certificate}
	tlsConfig.BuildNameToCertificate()
}

func (n *client) createTransport() {
	// create reference TLS config
	tlsConfig := tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	n.addTLSCA(&tlsConfig)
	n.addTLSAuth(&tlsConfig)

	// create transport
	n.Transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: func(network, addr string) (net.Conn, error) {
			logrus.Debugln("Dialing:", network, addr, "...")
			return dialer.Dial(network, addr)
		},
		TLSClientConfig:       &tlsConfig,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 10 * time.Minute,
	}
	n.Timeout = common.DefaultNetworkClientTimeout
}

func (n *client) ensureBackoff(method, uri string) *backoff.Backoff {
	n.lock.Lock()
	defer n.lock.Unlock()

	key := fmt.Sprintf("%s_%s", method, uri)
	if n.requestBackOffs[key] == nil {
		n.requestBackOffs[key] = &backoff.Backoff{
			Min:    backOffMinDelay,
			Max:    backOffMaxDelay,
			Factor: backOffDelayFactor,
			Jitter: backOffDelayJitter,
		}
	}

	return n.requestBackOffs[key]
}

func (n *client) backoffRequired(res *http.Response) bool {
	return res.StatusCode >= 400 && res.StatusCode < 600
}

func (n *client) checkBackoffRequest(req *http.Request, res *http.Response) {
	backoffDelay := n.ensureBackoff(req.Method, req.RequestURI)
	if n.backoffRequired(res) {
		time.Sleep(backoffDelay.Duration())
	} else {
		backoffDelay.Reset()
	}
}

func (n *client) do(
	ctx context.Context,
	uri, method string,
	request io.Reader,
	requestType string,
	headers http.Header,
) (*http.Response, error) {
	url, err := n.url.Parse(uri)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, url.String(), request)
	if err != nil {
		err = fmt.Errorf("failed to create NewRequest: %w", err)
		return nil, err
	}

	if headers != nil {
		req.Header = headers
	}

	req.Header.Set("User-Agent", common.AppVersion.UserAgent())
	if request != nil {
		req.Header.Set("Content-Type", requestType)
	}

	n.ensureTLSConfig()

	res, err := n.requester.Do(req)
	if err != nil {
		return nil, err
	}

	n.checkBackoffRequest(req, res)

	return res, nil
}

// ErrorResponse is an error type that is returned when there is an issue
// calling the remote server. It contains the http.Response responsible for
// the error and the error payload provided by the server.
type ErrorResponse struct {
	Response *http.Response       `json:"-"`
	Message  ErrorResponseMessage `json:"message"`
}

// XMLErrorResponse is an error type that is returned when there is an issue
// from an object storage provider that returns XML. It contains the
// http.Response responsible for the error and the error payload provided by
// the server.
//
// Google: https://cloud.google.com/storage/docs/xml-api/reference-status
// Amazon: https://docs.aws.amazon.com/AmazonS3/latest/API/ErrorResponses.html
// Azure: https://docs.microsoft.com/en-us/rest/api/storageservices/status-and-error-codes2
type XMLErrorResponse struct {
	Response *http.Response `xml:"-"`
	XMLName  xml.Name       `xml:"Error"`
	Code     string         `xml:"Code"`
	Message  string         `xml:"Message"`
}

type ErrorResponseMessage string

func (r *ErrorResponse) Error() string {
	statusCodeMsg := fmt.Sprintf("%d %s", r.Response.StatusCode, http.StatusText(r.Response.StatusCode))
	reqURL := url_helpers.CleanURL(r.Response.Request.URL.String())
	errMessage := fmt.Sprintf("%v %s: %s", r.Response.Request.Method, reqURL, statusCodeMsg)

	if string(r.Message) == statusCodeMsg {
		// If the message returned by the server is the status text, then don't repeat it in the message
		return errMessage
	}

	return fmt.Sprintf("%s (%s)", errMessage, r.Message)
}

func (r *XMLErrorResponse) Error() string {
	statusCodeMsg := fmt.Sprintf("%d %s", r.Response.StatusCode, http.StatusText(r.Response.StatusCode))

	if r.Code == "" {
		return statusCodeMsg
	}

	return fmt.Sprintf("%s (%s: %s)", statusCodeMsg, r.Code, r.Message)
}

func (e *ErrorResponseMessage) UnmarshalJSON(data []byte) error {
	type simple ErrorResponseMessage
	err := json.Unmarshal(data, (*simple)(e))
	if err == nil {
		return nil
	}

	var complex map[string][]interface{}
	err = json.Unmarshal(data, &complex)
	if err != nil {
		// explicitly ignore error, we can't decode this type
		return nil
	}

	messages := make([]string, 0, len(complex))
	for key, val := range complex {
		values := make([]string, 0, len(val))
		for _, msg := range val {
			values = append(values, fmt.Sprintf("%v", msg))
		}
		messages = append(messages, fmt.Sprintf("%s: %s", key, strings.Join(values, "; ")))
	}

	*e = ErrorResponseMessage(strings.Join(messages, ", "))
	return nil
}

func (n *client) doJSONWithPAT(
	ctx context.Context,
	uri, method string,
	statusCode int,
	pat string,
	request interface{},
	response interface{},
) (int, string, *http.Response) {
	var body io.Reader

	if request != nil {
		requestBody, err := json.Marshal(request)
		if err != nil {
			return -1, fmt.Sprintf("failed to marshal project object: %v", err), nil
		}
		body = bytes.NewReader(requestBody)
	}

	headers := make(http.Header)
	if response != nil {
		headers.Set("Accept", jsonMimeType)
	}
	if pat != "" {
		headers.Set("PRIVATE-TOKEN", pat)
	}

	res, err := n.do(ctx, uri, method, body, jsonMimeType, headers)
	if err != nil {
		return -1, err.Error(), nil
	}
	defer func() {
		_, _ = io.Copy(io.Discard, res.Body)
		_ = res.Body.Close()
	}()

	status := getMessageFromJSONResponse(res)
	if res.StatusCode == statusCode && response != nil {
		isApplicationJSON, err := isResponseApplicationJSON(res)
		if !isApplicationJSON {
			return -1, err.Error(), nil
		}

		d := json.NewDecoder(res.Body)
		err = d.Decode(response)
		if err != nil {
			return -1, fmt.Sprintf("Error decoding json payload %v", err), nil
		}
	}

	n.setLastUpdate(res.Header)

	return res.StatusCode, status, res
}

func getMessageFromJSONResponse(res *http.Response) string {
	if res.StatusCode >= 200 && res.StatusCode <= 299 {
		return res.Status
	}

	if isApplicationJSON, _ := isResponseApplicationJSON(res); isApplicationJSON {
		errMsg, _ := decodeJSONResponse(res)

		if errMsg != "" {
			return errMsg
		}
	}

	return res.Status
}

func getMimeAndContentType(res *http.Response) (mimeType, contentType string, e error) {
	contentType = res.Header.Get("Content-Type")

	mimeType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return "", contentType, fmt.Errorf("parsing Content-Type: %w", err)
	}

	return mimeType, contentType, nil
}

func decodeJSONResponse(res *http.Response) (string, error) {
	errResp := ErrorResponse{Response: res}
	err := json.NewDecoder(res.Body).Decode(&errResp)
	if err == nil {
		return errResp.Error(), nil
	}

	return "", err
}

func decodeXMLResponse(res *http.Response) (string, error) {
	xmlResp := XMLErrorResponse{Response: res}
	err := xml.NewDecoder(res.Body).Decode(&xmlResp)
	if err == nil {
		return xmlResp.Error(), nil
	}

	return "", err
}

func getMessageFromJSONOrXMLResponse(res *http.Response) string {
	if res.StatusCode >= 200 && res.StatusCode <= 299 {
		return res.Status
	}

	mimeType, _, err := getMimeAndContentType(res)
	if err != nil {
		return res.Status
	}

	var decodeErr error
	var errMsg string

	switch mimeType {
	case jsonMimeType:
		errMsg, decodeErr = decodeJSONResponse(res)
	case applicationXMLMimeType, textXMLMimeType:
		errMsg, decodeErr = decodeXMLResponse(res)
	}

	if errMsg != "" {
		return errMsg
	} else if decodeErr != nil {
		return fmt.Sprintf("%s (%s decode error: %v)", res.Status, mimeType, decodeErr)
	}

	return res.Status
}

func (n *client) getResponseTLSData(tls *tls.ConnectionState, resolveFullChain bool) (ResponseTLSData, error) {
	TLSData := ResponseTLSData{
		CertFile: n.certFile,
		KeyFile:  n.keyFile,
	}

	caChain, err := n.buildCAChain(tls, resolveFullChain)
	if err != nil {
		return TLSData, fmt.Errorf("couldn't build CA Chain: %w", err)
	}

	TLSData.CAChain = caChain

	return TLSData, nil
}

func (n *client) buildCAChain(tls *tls.ConnectionState, resolveFullChain bool) (string, error) {
	if len(n.caData) != 0 {
		return string(n.caData), nil
	}

	if tls == nil {
		return "", nil
	}

	builder := ca_chain.NewBuilder(logrus.StandardLogger(), resolveFullChain)
	err := builder.BuildChainFromTLSConnectionState(tls)
	if err != nil {
		return "", fmt.Errorf("error while fetching certificates from TLS ConnectionState: %w", err)
	}

	return builder.String(), nil
}

func isResponseApplicationJSON(res *http.Response) (result bool, err error) {
	mimeType, contentType, err := getMimeAndContentType(res)
	if err != nil {
		return false, err
	}

	if mimeType != jsonMimeType {
		return false, fmt.Errorf("server should return application/json. Got: %v", contentType)
	}

	return true, nil
}

func fixCIURL(url string) string {
	url = strings.TrimRight(url, "/")
	url = strings.TrimSuffix(url, "/ci")
	return url
}

func (n *client) findCertificate(certificate *string, base string, name string) {
	if *certificate != "" {
		return
	}
	path := filepath.Join(base, name)
	if _, err := os.Stat(path); err == nil {
		*certificate = path
	}
}

func newClient(requestCredentials requestCredentials) (*client, error) {
	url, err := url.Parse(fixCIURL(requestCredentials.GetURL()) + "/api/v4/")
	if err != nil {
		return nil, err
	}

	if url.Scheme != "http" && url.Scheme != "https" {
		return nil, errors.New("only http or https scheme supported")
	}

	c := &client{
		url:             url,
		caFile:          requestCredentials.GetTLSCAFile(),
		certFile:        requestCredentials.GetTLSCertFile(),
		keyFile:         requestCredentials.GetTLSKeyFile(),
		requestBackOffs: make(map[string]*backoff.Backoff),
	}
	c.requester = newRateLimitRequester(&c.Client)

	host := strings.Split(url.Host, ":")[0]
	if CertificateDirectory != "" {
		c.findCertificate(&c.caFile, CertificateDirectory, host+".crt")
		c.findCertificate(&c.certFile, CertificateDirectory, host+".auth.crt")
		c.findCertificate(&c.keyFile, CertificateDirectory, host+".auth.key")
	}

	return c, nil
}
