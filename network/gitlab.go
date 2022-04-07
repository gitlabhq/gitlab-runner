package network

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

const clientError = -100

var apiRequestStatuses = prometheus.NewDesc(
	"gitlab_runner_api_request_statuses_total",
	"The total number of api requests, partitioned by runner, endpoint and status.",
	[]string{"runner", "endpoint", "status"},
	nil,
)

type APIEndpoint string

const (
	APIEndpointRequestJob APIEndpoint = "request_job"
	APIEndpointUpdateJob  APIEndpoint = "update_job"
	APIEndpointPatchTrace APIEndpoint = "patch_trace"
)

type apiRequestStatusPermutation struct {
	runnerID string
	endpoint APIEndpoint
	status   int
}

type APIRequestStatusesMap struct {
	internal map[apiRequestStatusPermutation]int
	lock     sync.RWMutex
}

func (arspm *APIRequestStatusesMap) Append(runnerID string, endpoint APIEndpoint, status int) {
	arspm.lock.Lock()
	defer arspm.lock.Unlock()

	permutation := apiRequestStatusPermutation{runnerID: runnerID, endpoint: endpoint, status: status}

	if _, ok := arspm.internal[permutation]; !ok {
		arspm.internal[permutation] = 0
	}

	arspm.internal[permutation]++
}

// Describe implements prometheus.Collector.
func (arspm *APIRequestStatusesMap) Describe(ch chan<- *prometheus.Desc) {
	ch <- apiRequestStatuses
}

// Collect implements prometheus.Collector.
func (arspm *APIRequestStatusesMap) Collect(ch chan<- prometheus.Metric) {
	arspm.lock.RLock()
	defer arspm.lock.RUnlock()

	for permutation, count := range arspm.internal {
		ch <- prometheus.MustNewConstMetric(
			apiRequestStatuses,
			prometheus.CounterValue,
			float64(count),
			permutation.runnerID,
			string(permutation.endpoint),
			strconv.Itoa(permutation.status),
		)
	}
}

func NewAPIRequestStatusesMap() *APIRequestStatusesMap {
	return &APIRequestStatusesMap{
		internal: make(map[apiRequestStatusPermutation]int),
	}
}

type GitLabClient struct {
	clients map[string]*client
	lock    sync.Mutex

	requestsStatusesMap *APIRequestStatusesMap
}

func (n *GitLabClient) getClient(credentials requestCredentials) (c *client, err error) {
	n.lock.Lock()
	defer n.lock.Unlock()

	if n.clients == nil {
		n.clients = make(map[string]*client)
	}
	key := fmt.Sprintf(
		"%s_%s_%s_%s",
		credentials.GetURL(),
		credentials.GetToken(),
		credentials.GetTLSCAFile(),
		credentials.GetTLSCertFile(),
	)
	c = n.clients[key]
	if c == nil {
		c, err = newClient(credentials)
		if err != nil {
			return
		}
		n.clients[key] = c
	}

	return
}

func (n *GitLabClient) getLastUpdate(credentials requestCredentials) (lu string) {
	cli, err := n.getClient(credentials)
	if err != nil {
		return ""
	}
	return cli.getLastUpdate()
}

// getFeatures enables features that are properties of networking client
func (n *GitLabClient) getFeatures(features *common.FeaturesInfo) {
	features.TraceReset = true
	features.TraceChecksum = true
	features.TraceSize = true
	features.Cancelable = true
}

func (n *GitLabClient) getRunnerVersion(config common.RunnerConfig) common.VersionInfo {
	info := common.VersionInfo{
		Name:         common.NAME,
		Version:      common.VERSION,
		Revision:     common.REVISION,
		Platform:     runtime.GOOS,
		Architecture: runtime.GOARCH,
		Executor:     config.Executor,
		Shell:        config.Shell,
	}

	n.getFeatures(&info.Features)

	if executorProvider := common.GetExecutorProvider(config.Executor); executorProvider != nil {
		_ = executorProvider.GetFeatures(&info.Features)

		if info.Shell == "" {
			info.Shell = executorProvider.GetDefaultShell()
		}

		executorProvider.GetConfigInfo(&config, &info.Config)
	}

	if shell := common.GetShell(info.Shell); shell != nil {
		shell.GetFeatures(&info.Features)
	}

	return info
}

func (n *GitLabClient) doRaw(
	ctx context.Context,
	credentials requestCredentials,
	method, uri string,
	request io.Reader,
	requestType string,
	headers http.Header,
) (res *http.Response, err error) {
	c, err := n.getClient(credentials)
	if err != nil {
		return nil, err
	}

	return c.do(ctx, uri, method, request, requestType, headers)
}

func (n *GitLabClient) doJSON(
	ctx context.Context,
	credentials requestCredentials,
	method, uri string,
	statusCode int,
	request interface{},
	response interface{},
) (int, string, *http.Response) {
	c, err := n.getClient(credentials)
	if err != nil {
		return clientError, err.Error(), nil
	}

	return c.doJSON(ctx, uri, method, statusCode, request, response)
}

func (n *GitLabClient) getResponseTLSData(
	credentials requestCredentials,
	response *http.Response,
) (ResponseTLSData, error) {
	c, err := n.getClient(credentials)
	if err != nil {
		return ResponseTLSData{}, fmt.Errorf("couldn't get client: %w", err)
	}

	return c.getResponseTLSData(response.TLS)
}

func (n *GitLabClient) RegisterRunner(
	runner common.RunnerCredentials,
	parameters common.RegisterRunnerParameters,
) *common.RegisterRunnerResponse {
	// TODO: pass executor
	request := common.RegisterRunnerRequest{
		RegisterRunnerParameters: parameters,
		Token:                    runner.Token,
		Info:                     n.getRunnerVersion(common.RunnerConfig{}),
	}

	var response common.RegisterRunnerResponse
	result, statusText, resp := n.doJSON(
		context.Background(),
		&runner,
		http.MethodPost,
		"runners",
		http.StatusCreated,
		&request,
		&response,
	)
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}

	switch result {
	case http.StatusCreated:
		runner.Log().Println("Registering runner...", "succeeded")
		return &response
	case http.StatusForbidden:
		runner.Log().Errorln("Registering runner...", "forbidden (check registration token)")
		return nil
	case clientError:
		runner.Log().WithField("status", statusText).Errorln("Registering runner...", "error")
		return nil
	default:
		runner.Log().WithField("status", statusText).Errorln("Registering runner...", "failed")
		return nil
	}
}

func (n *GitLabClient) VerifyRunner(runner common.RunnerCredentials) bool {
	request := common.VerifyRunnerRequest{
		Token: runner.Token,
	}

	result, statusText, resp := n.doJSON(
		context.Background(),
		&runner,
		http.MethodPost,
		"runners/verify",
		http.StatusOK,
		&request,
		nil,
	)
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}

	switch result {
	case http.StatusOK:
		// this is expected due to fact that we ask for non-existing job
		runner.Log().Println("Verifying runner...", "is alive")
		return true
	case http.StatusForbidden:
		runner.Log().Errorln("Verifying runner...", "is removed")
		return false
	case clientError:
		runner.Log().WithField("status", statusText).Errorln("Verifying runner...", "error")
		return true
	default:
		runner.Log().WithField("status", statusText).Errorln("Verifying runner...", "failed")
		return true
	}
}

func (n *GitLabClient) UnregisterRunner(runner common.RunnerCredentials) bool {
	request := common.UnregisterRunnerRequest{
		Token: runner.Token,
	}

	result, statusText, resp := n.doJSON(
		context.Background(),
		&runner,
		http.MethodDelete,
		"runners",
		http.StatusNoContent,
		&request,
		nil,
	)
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}

	const baseLogText = "Unregistering runner from GitLab"
	switch result {
	case http.StatusNoContent:
		runner.Log().Println(baseLogText, "succeeded")
		return true
	case http.StatusForbidden:
		runner.Log().Errorln(baseLogText, "forbidden")
		return false
	case clientError:
		runner.Log().WithField("status", statusText).Errorln(baseLogText, "error")
		return false
	default:
		runner.Log().WithField("status", statusText).Errorln(baseLogText, "failed")
		return false
	}
}

func addTLSData(response *common.JobResponse, tlsData ResponseTLSData) {
	if tlsData.CAChain != "" {
		response.TLSCAChain = tlsData.CAChain
	}

	if tlsData.CertFile != "" && tlsData.KeyFile != "" {
		data, err := ioutil.ReadFile(tlsData.CertFile)
		if err == nil {
			response.TLSAuthCert = string(data)
		}
		data, err = ioutil.ReadFile(tlsData.KeyFile)
		if err == nil {
			response.TLSAuthKey = string(data)
		}
	}
}

func (n *GitLabClient) RequestJob(
	ctx context.Context,
	config common.RunnerConfig,
	sessionInfo *common.SessionInfo,
) (*common.JobResponse, bool) {
	request := common.JobRequest{
		Info:       n.getRunnerVersion(config),
		Token:      config.Token,
		LastUpdate: n.getLastUpdate(&config.RunnerCredentials),
		Session:    sessionInfo,
	}

	var response common.JobResponse
	result, statusText, httpResponse := n.doJSON(
		ctx,
		&config.RunnerCredentials,
		http.MethodPost,
		"jobs/request",
		http.StatusCreated,
		&request,
		&response,
	)

	n.requestsStatusesMap.Append(config.RunnerCredentials.ShortDescription(), APIEndpointRequestJob, result)

	switch result {
	case http.StatusCreated:
		config.Log().WithFields(logrus.Fields{
			"job":      response.ID,
			"repo_url": response.RepoCleanURL(),
		}).Println("Checking for jobs...", "received")

		tlsData, err := n.getResponseTLSData(&config.RunnerCredentials, httpResponse)
		if err != nil {
			config.Log().
				WithError(err).Errorln("Error on fetching TLS Data from API response...", "error")
		}
		addTLSData(&response, tlsData)

		return &response, true
	case http.StatusForbidden:
		config.Log().Errorln("Checking for jobs...", "forbidden")
		return nil, false
	case http.StatusNoContent:
		config.Log().Debugln("Checking for jobs...", "nothing")
		return nil, true
	case clientError:
		config.Log().WithField("status", statusText).Errorln("Checking for jobs...", "error")
		return nil, false
	default:
		config.Log().WithField("status", statusText).Warningln("Checking for jobs...", "failed")
		return nil, true
	}
}

func (n *GitLabClient) UpdateJob(
	config common.RunnerConfig,
	jobCredentials *common.JobCredentials,
	jobInfo common.UpdateJobInfo,
) common.UpdateJobResult {
	request := common.UpdateJobRequest{
		Info:          n.getRunnerVersion(config),
		Token:         jobCredentials.Token,
		State:         jobInfo.State,
		FailureReason: jobInfo.FailureReason,
		Checksum:      jobInfo.Output.Checksum, // deprecated
		Output:        jobInfo.Output,
		ExitCode:      jobInfo.ExitCode,
	}

	statusCode, statusText, response := n.doJSON(
		context.Background(),
		&config.RunnerCredentials,
		http.MethodPut,
		fmt.Sprintf("jobs/%d", jobInfo.ID),
		http.StatusOK,
		&request,
		nil,
	)
	n.requestsStatusesMap.Append(config.RunnerCredentials.ShortDescription(), APIEndpointUpdateJob, statusCode)

	log := config.Log().WithField("job", jobInfo.ID)

	return n.createUpdateJobResult(log, statusCode, statusText, response)
}

func (n *GitLabClient) createUpdateJobResult(
	log *logrus.Entry,
	statusCode int,
	statusText string,
	response *http.Response,
) common.UpdateJobResult {
	remoteJobStateResponse := NewRemoteJobStateResponse(response, log)

	result := common.UpdateJobResult{
		NewUpdateInterval: remoteJobStateResponse.RemoteUpdateInterval,
		CancelRequested:   remoteJobStateResponse.IsCanceled(),
	}

	log = log.WithFields(logrus.Fields{
		"code":            statusCode,
		"job-status":      remoteJobStateResponse.RemoteState,
		"update-interval": remoteJobStateResponse.RemoteUpdateInterval,
	})

	switch {
	case remoteJobStateResponse.IsFailed():
		log.Warningln("Submitting job to coordinator...", "job failed")
		result.State = common.UpdateAbort
	case statusCode == http.StatusOK:
		log.Debugln("Submitting job to coordinator...", "ok")
		result.State = common.UpdateSucceeded
	case statusCode == http.StatusAccepted:
		log.Debugln("Submitting job to coordinator...", "accepted, but not yet completed")
		result.State = common.UpdateAcceptedButNotCompleted
	case statusCode == http.StatusPreconditionFailed:
		log.Debugln("Submitting job to coordinator...", "trace validation failed")
		result.State = common.UpdateTraceValidationFailed
	case statusCode == http.StatusNotFound:
		log.Warningln("Submitting job to coordinator...", "not found")
		result.State = common.UpdateAbort
	case statusCode == http.StatusForbidden:
		log.WithField("status", statusText).Errorln("Submitting job to coordinator...", "forbidden")
		result.State = common.UpdateAbort
	case statusCode == clientError:
		log.WithField("status", statusText).Errorln("Submitting job to coordinator...", "error")
		result.State = common.UpdateAbort
	default:
		log.WithField("status", statusText).Warningln("Submitting job to coordinator...", "failed")
		result.State = common.UpdateFailed
	}

	return result
}

func (n *GitLabClient) PatchTrace(
	config common.RunnerConfig,
	jobCredentials *common.JobCredentials,
	content []byte,
	startOffset int,
) common.PatchTraceResult {
	id := jobCredentials.ID

	baseLog := config.Log().WithField("job", id)
	if len(content) == 0 {
		baseLog.Debugln("Appending trace to coordinator...", "skipped due to empty patch")
		return common.NewPatchTraceResult(startOffset, common.PatchSucceeded, 0)
	}

	endOffset := startOffset + len(content)
	contentRange := fmt.Sprintf("%d-%d", startOffset, endOffset-1)

	headers := make(http.Header)
	headers.Set("Content-Range", contentRange)
	headers.Set("JOB-TOKEN", jobCredentials.Token)

	uri := fmt.Sprintf("jobs/%d/trace", id)
	request := bytes.NewReader(content)

	response, err := n.doRaw(
		context.Background(),
		&config.RunnerCredentials,
		"PATCH",
		uri,
		request,
		"text/plain",
		headers,
	)
	if err != nil {
		config.Log().Errorln("Appending trace to coordinator...", "error", err.Error())
		return common.NewPatchTraceResult(startOffset, common.PatchFailed, 0)
	}

	n.requestsStatusesMap.Append(
		config.RunnerCredentials.ShortDescription(),
		APIEndpointPatchTrace,
		response.StatusCode,
	)

	defer func() {
		_, _ = io.Copy(ioutil.Discard, response.Body)
		_ = response.Body.Close()
	}()

	tracePatchResponse := NewTracePatchResponse(response, baseLog)
	log := baseLog.WithFields(logrus.Fields{
		"sent-log":        contentRange,
		"job-log":         tracePatchResponse.RemoteRange,
		"job-status":      tracePatchResponse.RemoteState,
		"code":            response.StatusCode,
		"status":          response.Status,
		"update-interval": tracePatchResponse.RemoteUpdateInterval,
	})

	return n.createPatchTraceResult(startOffset, tracePatchResponse, response, endOffset, log)
}

func (n *GitLabClient) createPatchTraceResult(
	startOffset int,
	tracePatchResponse *TracePatchResponse,
	response *http.Response,
	endOffset int,
	log *logrus.Entry,
) common.PatchTraceResult {
	result := common.PatchTraceResult{
		SentOffset:        startOffset,
		NewUpdateInterval: tracePatchResponse.RemoteUpdateInterval,
		CancelRequested:   tracePatchResponse.IsCanceled(),
	}

	switch {
	case tracePatchResponse.IsFailed():
		log.Warningln("Appending trace to coordinator...", "job failed")
		result.State = common.PatchAbort

		return result

	case response.StatusCode == http.StatusAccepted:
		log.Debugln("Appending trace to coordinator...", "ok")
		result.SentOffset = endOffset
		result.State = common.PatchSucceeded

		return result

	case response.StatusCode == http.StatusNotFound:
		log.Warningln("Appending trace to coordinator...", "not-found")
		result.State = common.PatchNotFound

		return result

	case response.StatusCode == http.StatusRequestedRangeNotSatisfiable:
		log.Warningln("Appending trace to coordinator...", "range mismatch")
		result.SentOffset = tracePatchResponse.NewOffset()
		result.State = common.PatchRangeMismatch

		return result

	case response.StatusCode == clientError:
		log.Errorln("Appending trace to coordinator...", "error")
		result.State = common.PatchAbort

		return result

	default:
		log.Warningln("Appending trace to coordinator...", "failed")
		result.State = common.PatchFailed

		return result
	}
}

func (n *GitLabClient) createArtifactsForm(reader io.Reader, baseName string) (io.ReadCloser, string) {
	pr, pw := io.Pipe()

	mpw := multipart.NewWriter(pw)

	go func() {
		defer func() {
			_ = mpw.Close()
			_ = pw.Close()
		}()

		wr, err := mpw.CreateFormFile("file", baseName)
		if err != nil {
			_ = pw.CloseWithError(err)
			return
		}

		_, err = io.Copy(wr, reader)
		if err != nil {
			_ = pw.CloseWithError(err)
		}
	}()

	return pr, mpw.FormDataContentType()
}

func uploadRawArtifactsQuery(options common.ArtifactsOptions) url.Values {
	q := url.Values{}

	if options.ExpireIn != "" {
		q.Set("expire_in", options.ExpireIn)
	}

	if options.Format != "" {
		q.Set("artifact_format", string(options.Format))
	}

	if options.Type != "" {
		q.Set("artifact_type", options.Type)
	}

	return q
}

func (n *GitLabClient) UploadRawArtifacts(
	config common.JobCredentials,
	reader io.ReadCloser,
	options common.ArtifactsOptions,
) (common.UploadState, string) {
	defer func() {
		_ = reader.Close()
	}()

	pr, contentType := n.createArtifactsForm(reader, options.BaseName)
	defer func() {
		_ = pr.Close()
	}()

	query := uploadRawArtifactsQuery(options)

	headers := make(http.Header)
	headers.Set("JOB-TOKEN", config.Token)
	res, err := n.doRaw(
		context.Background(),
		&config,
		http.MethodPost,
		fmt.Sprintf("jobs/%d/artifacts?%s", config.ID, query.Encode()),
		pr,
		contentType,
		headers,
	)

	log := logrus.WithFields(logrus.Fields{
		"id":    config.ID,
		"token": helpers.ShortenToken(config.Token),
	})

	if res != nil {
		log = log.WithField("responseStatus", res.Status)
	}

	closeWithLogging(log, pr, "pipe reader")
	closeWithLogging(log, reader, "archive reader")

	messagePrefix := "Uploading artifacts to coordinator..."
	if options.Type != "" {
		messagePrefix = fmt.Sprintf("Uploading artifacts as %q to coordinator...", options.Type)
	}

	if err != nil {
		log.WithError(err).Errorln(messagePrefix, "error")
		return common.UploadFailed, ""
	}

	defer func() {
		_, _ = io.Copy(ioutil.Discard, res.Body)
		_ = res.Body.Close()
	}()

	return n.determineUploadState(res, log, messagePrefix)
}

func closeWithLogging(log logrus.FieldLogger, c io.Closer, name string) {
	err := c.Close()
	if err != nil {
		log.WithError(err).Warningf("Error while closing the %s", name)
	}
}

func (n *GitLabClient) determineUploadState(
	resp *http.Response,
	log *logrus.Entry,
	messagePrefix string,
) (common.UploadState, string) {
	statusText := getMessageFromJSONResponse(resp)

	switch resp.StatusCode {
	case http.StatusCreated:
		log.Println(messagePrefix, statusText)
		return common.UploadSucceeded, ""
	case http.StatusTemporaryRedirect:
		return handleUploadRedirectionState(resp, log, messagePrefix, statusText)
	case http.StatusForbidden:
		log.WithField("status", resp.StatusCode).Errorln(messagePrefix, statusText)
		return common.UploadForbidden, ""
	case http.StatusRequestEntityTooLarge:
		log.WithField("status", resp.StatusCode).Errorln(messagePrefix, statusText)
		return common.UploadTooLarge, ""
	case http.StatusServiceUnavailable:
		log.WithField("status", resp.StatusCode).Errorln(messagePrefix, statusText)
		return common.UploadServiceUnavailable, ""
	default:
		log.WithField("status", resp.StatusCode).Warningln(messagePrefix, statusText)
		return common.UploadFailed, ""
	}
}

func handleUploadRedirectionState(
	resp *http.Response,
	log *logrus.Entry,
	messagePrefix string,
	statusText string,
) (common.UploadState, string) {
	location := resp.Header.Get("Location")
	if location == "" {
		log.WithField("status", resp.StatusCode).Errorln(messagePrefix, statusText, "empty location")
		return common.UploadFailed, ""
	}

	return common.UploadRedirected, location
}

func (n *GitLabClient) DownloadArtifacts(
	config common.JobCredentials,
	artifactsFile io.WriteCloser,
	directDownload *bool,
) common.DownloadState {
	query := url.Values{}

	if directDownload != nil {
		query.Set("direct_download", strconv.FormatBool(*directDownload))
	}

	headers := make(http.Header)
	headers.Set("JOB-TOKEN", config.Token)
	uri := fmt.Sprintf("jobs/%d/artifacts?%s", config.ID, query.Encode())

	res, err := n.doRaw(context.Background(), &config, http.MethodGet, uri, nil, "", headers)

	log := logrus.WithFields(logrus.Fields{
		"id":    config.ID,
		"token": helpers.ShortenToken(config.Token),
	})

	if res != nil {
		log = log.WithField("responseStatus", res.Status)
	}

	if err != nil {
		log.Errorln("Downloading artifacts from coordinator...", "error", err.Error())
		return common.DownloadFailed
	}
	defer func() {
		_, _ = io.Copy(ioutil.Discard, res.Body)
		_ = res.Body.Close()
	}()

	switch res.StatusCode {
	case http.StatusOK:
		return n.downloadArtifactFile(log, artifactsFile, res)
	case http.StatusForbidden:
		// We generally expect JSON responses from the GitLab API, but a
		// 302 redirection to object storage may result in an XML
		// response that might include important details why the request
		// was rejected (e.g. Google VPC Service Controls).
		statusText := getMessageFromJSONOrXMLResponse(res)
		log.WithField("status", statusText).Errorln("Downloading artifacts from coordinator...", "forbidden")
		return common.DownloadForbidden
	case http.StatusNotFound:
		log.Errorln("Downloading artifacts from coordinator...", "not found")
		return common.DownloadNotFound
	default:
		log.WithField("status", res.Status).Warningln("Downloading artifacts from coordinator...", "failed")
		return common.DownloadFailed
	}
}

func (n *GitLabClient) downloadArtifactFile(
	log logrus.FieldLogger,
	file io.WriteCloser,
	res *http.Response,
) common.DownloadState {
	_, err := io.Copy(file, res.Body)

	closeWithLogging(log, file, "file writer")

	if err != nil {
		log.WithError(err).Errorln("Downloading artifacts from coordinator...", "error")
		return common.DownloadFailed
	}

	log.Println("Downloading artifacts from coordinator...", "ok")

	return common.DownloadSucceeded
}

func (n *GitLabClient) ProcessJob(
	config common.RunnerConfig,
	jobCredentials *common.JobCredentials,
) (common.JobTrace, error) {
	trace, err := newJobTrace(n, config, jobCredentials)
	if err != nil {
		return nil, err
	}

	trace.start()
	return trace, nil
}

func NewGitLabClientWithRequestStatusesMap(rsMap *APIRequestStatusesMap) *GitLabClient {
	return &GitLabClient{
		requestsStatusesMap: rsMap,
	}
}

func NewGitLabClient() *GitLabClient {
	return NewGitLabClientWithRequestStatusesMap(NewAPIRequestStatusesMap())
}
