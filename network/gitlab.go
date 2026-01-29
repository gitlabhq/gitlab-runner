package network

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

const (
	// createdRunnerTokenPrefix is the token prefix used for GitLab UI-created runner authentication tokens
	createdRunnerTokenPrefix = "glrt-"
	clientError              = -100
	responseBodyPeekMax      = 512

	correlationIDHeader   = "X-Request-Id"
	correlationIDLogField = "correlation_id"
)

func TokenIsCreatedRunnerToken(token string) bool {
	return strings.HasPrefix(token, createdRunnerTokenPrefix)
}

type GitLabClient struct {
	clients              map[string]*client
	lock                 sync.Mutex
	certDirectory        string
	apiRequestsCollector *APIRequestsCollector
	connectionMaxAge     time.Duration
}

func (n *GitLabClient) getClient(credentials requestCredentials) (*client, error) {
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
	c, ok := n.clients[key]
	if ok {
		return c, nil
	}

	c, err := newClient(
		credentials,
		n.apiRequestsCollector,
		withMaxAge(n.connectionMaxAge),
		withCertificateDirectory(n.certDirectory),
	)
	if err != nil {
		return nil, fmt.Errorf("new client: %w", err)
	}

	n.clients[key] = c

	return c, nil
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
	features.CancelGracefully = true
	features.TwoPhaseJobCommit = true
	features.JobInputs = true
}

func (n *GitLabClient) ExecutorSupportsNativeSteps(config common.RunnerConfig) bool {
	return n.getRunnerInfo(config).Features.NativeStepsIntegration
}

func (n *GitLabClient) getRunnerInfo(config common.RunnerConfig) common.Info {
	info := common.Info{
		Name:         common.AppVersion.Name,
		Version:      common.AppVersion.Version,
		Revision:     common.AppVersion.Revision,
		Platform:     runtime.GOOS,
		Architecture: runtime.GOARCH,
		Executor:     config.Executor,
		Shell:        config.Shell,
		Labels:       config.ComputedLabels(),
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

type doRawParams struct {
	credentials requestCredentials
	method      string
	uri         string
	request     common.ContentProvider
	requestType string
	headers     http.Header
}

// doMeasuredRaw is a decorator that adds metrics measurements through
// n.apiRequestsCollector to the doRaw() call
func (n *GitLabClient) doMeasuredRaw(
	ctx context.Context,
	log logrus.FieldLogger,
	runnerID string,
	systemID string,
	endpoint apiEndpoint,
	params doRawParams,
) (*http.Response, error) {
	var response *http.Response
	var err error

	fn := func() (int, string) {
		// Response body is handled after doMeasuredJSON() decorator call
		// Linting violation here is a false-positive.
		// nolint:bodyclose
		response, err = n.doRaw(
			ctx,
			params.credentials,
			params.method,
			params.uri,
			params.request,
			params.requestType,
			params.headers,
		)
		if err != nil {
			return clientError, ""
		}

		return response.StatusCode, params.method
	}

	n.apiRequestsCollector.Observe(
		log,
		runnerID,
		systemID,
		endpoint,
		fn,
	)

	if err != nil {
		return nil, fmt.Errorf("measured raw request: %w", err)
	}

	return response, nil
}

func (n *GitLabClient) doRaw(
	ctx context.Context,
	credentials requestCredentials,
	method, uri string,
	bodyProvider common.ContentProvider,
	requestType string,
	headers http.Header,
) (res *http.Response, err error) {
	c, err := n.getClient(credentials)
	if err != nil {
		return nil, fmt.Errorf("get client: %w", err)
	}

	response, err := c.do(ctx, uri, method, bodyProvider, requestType, headers)
	if err != nil {
		return nil, fmt.Errorf("execute raw request: %w", err)
	}

	return response, nil
}

type doJSONParams struct {
	credentials requestCredentials
	method      string
	uri         string
	statusCode  int
	headers     http.Header
	request     interface{}
	response    interface{}
}

// doMeasuredJSON is a decorator that adds metrics measurements through
// n.apiRequestsCollector to the doJSON() call
func (n *GitLabClient) doMeasuredJSON(
	ctx context.Context,
	log logrus.FieldLogger,
	runnerID string,
	systemID string,
	endpoint apiEndpoint,
	params doJSONParams,
) (int, string, *http.Response) {
	var result int
	var statusText string
	var httpResponse *http.Response

	fn := func() (int, string) {
		// Response body is handled after doMeasuredJSON() decorator call
		// Linting violation here is a false-positive.
		// nolint:bodyclose
		result, statusText, httpResponse = n.doJSON(
			ctx,
			params.credentials,
			params.method,
			params.uri,
			params.statusCode,
			params.headers,
			params.request,
			params.response,
		)

		return result, params.method
	}

	n.apiRequestsCollector.Observe(
		log,
		runnerID,
		systemID,
		endpoint,
		fn,
	)

	return result, statusText, httpResponse
}

// Create a PRIVATE-TOKEN http header for the specified private access token (pat).
func PrivateTokenHeader(pat string) http.Header {
	headers := http.Header{}
	if pat != "" {
		headers.Set(common.PrivateToken, pat)
	}
	return headers
}

// Create a JOB-TOKEN http header for the specified job token.
func JobTokenHeader(jobToken string) http.Header {
	headers := http.Header{}
	if jobToken != "" {
		headers.Set(common.JobToken, jobToken)
	}
	return headers
}

// Create a RUNNER-TOKEN http header for the specified job token.
func RunnerTokenHeader(runnerToken string) http.Header {
	headers := http.Header{}
	if runnerToken != "" {
		headers.Set(common.RunnerToken, runnerToken)
	}
	return headers
}

// addCorrelationID to passed in http.Header. If a nil value
// is passed, a new instance of http.Header is created and
// correlation ID is added to it.
func addCorrelationID(headers http.Header) (http.Header, string) {
	if headers == nil {
		headers = http.Header{}
	}
	correlationID := NewCorrelationID()
	headers.Set(correlationIDHeader, correlationID)
	return headers, correlationID
}

func NewCorrelationID() string {
	return strings.ReplaceAll(uuid.NewString(), "-", "")
}

func (n *GitLabClient) doJSON(
	ctx context.Context,
	credentials requestCredentials,
	method, uri string,
	statusCode int,
	headers http.Header,
	request interface{},
	response interface{},
) (int, string, *http.Response) {
	c, err := n.getClient(credentials)
	if err != nil {
		return clientError, fmt.Errorf("get client: %w", err).Error(), nil
	}

	return c.doJSON(ctx, uri, method, statusCode, headers, request, response)
}

func (n *GitLabClient) getResponseTLSData(
	credentials requestCredentials,
	resolveFullChain bool,
	response *http.Response,
) (ResponseTLSData, error) {
	c, err := n.getClient(credentials)
	if err != nil {
		return ResponseTLSData{}, fmt.Errorf("couldn't get client: %w", err)
	}

	return c.getResponseTLSData(response.TLS, resolveFullChain)
}

func (n *GitLabClient) SetConnectionMaxAge(age time.Duration) {
	n.connectionMaxAge = age
}

func (n *GitLabClient) RegisterRunner(
	runner common.RunnerCredentials,
	parameters common.RegisterRunnerParameters,
) *common.RegisterRunnerResponse {
	// TODO: pass executor
	request := common.RegisterRunnerRequest{
		RegisterRunnerParameters: parameters,
		Token:                    runner.Token,
		Info:                     n.getRunnerInfo(common.RunnerConfig{}),
	}

	headers, correlationID := addCorrelationID(RunnerTokenHeader(runner.Token))

	var response common.RegisterRunnerResponse
	result, statusText, resp := n.doJSON(
		context.Background(),
		&runner,
		http.MethodPost,
		"runners",
		http.StatusCreated,
		headers,
		&request,
		&response,
	)
	defer closeResponseBody(resp, false)

	logger := runner.Log().WithField(correlationIDLogField, getCorrelationID(resp, correlationID))

	switch result {
	case http.StatusCreated:
		logger.Println("Registering runner...", "succeeded")
		return &response
	case http.StatusForbidden:
		logger.WithField("status", statusText).Errorln("Registering runner...", "forbidden (check registration token)")
		return nil
	case clientError:
		logger.WithField("status", statusText).Errorln("Registering runner...", "client error")
		return nil
	default:
		logger.WithField("status", statusText).Errorln("Registering runner...", "failed")
		return nil
	}
}

func (n *GitLabClient) VerifyRunner(runner common.RunnerCredentials, systemID string) *common.VerifyRunnerResponse {
	request := common.VerifyRunnerRequest{
		Token:    runner.Token,
		SystemID: systemID,
	}

	headers, correlationID := addCorrelationID(RunnerTokenHeader(runner.Token))

	var response common.VerifyRunnerResponse
	//nolint:bodyclose
	// body is closed with closeResponseBody function call
	result, statusText, resp := n.doJSON(
		context.Background(),
		&runner,
		http.MethodPost,
		"runners/verify",
		http.StatusOK,
		headers,
		&request,
		&response,
	)
	if result == -1 {
		// if server is not able to return JSON, let's try plain text (the legacy response format)
		//nolint:bodyclose
		// body is closed with closeResponseBody function call
		result, statusText, resp = n.doJSON(
			context.Background(),
			&runner,
			http.MethodPost,
			"runners/verify",
			http.StatusOK,
			headers,
			&request,
			nil,
		)
	}
	defer closeResponseBody(resp, false)

	logger := runner.Log().WithField(correlationIDLogField, getCorrelationID(resp, correlationID))

	switch result {
	case http.StatusOK:
		// this is expected due to fact that we ask for non-existing job
		if TokenIsCreatedRunnerToken(runner.Token) {
			logger.Println("Verifying runner...", "is valid")
		} else {
			logger.Println("Verifying runner...", "is alive")
		}
		return &response
	case http.StatusForbidden:
		if TokenIsCreatedRunnerToken(runner.Token) {
			logger.Println("Verifying runner...", "is not valid")
		} else {
			logger.WithField("status", statusText).Errorln("Verifying runner...", "is removed")
		}
		return nil
	case clientError:
		logger.WithField("status", statusText).Errorln("Verifying runner...", "client error")
		return &response
	default:
		logger.WithField("status", statusText).Errorln("Verifying runner...", "failed")
		return &response
	}
}

func (n *GitLabClient) UnregisterRunner(runner common.RunnerCredentials) bool {
	request := common.UnregisterRunnerRequest{
		Token: runner.Token,
	}

	headers, correlationID := addCorrelationID(RunnerTokenHeader(runner.Token))

	result, statusText, resp := n.doJSON(
		context.Background(),
		&runner,
		http.MethodDelete,
		"runners",
		http.StatusNoContent,
		headers,
		&request,
		nil,
	)
	defer closeResponseBody(resp, false)

	logger := runner.Log().WithField(correlationIDLogField, getCorrelationID(resp, correlationID))

	const baseLogText = "Unregistering runner from GitLab"
	switch result {
	case http.StatusNoContent:
		logger.Println(baseLogText, "succeeded")
		return true
	case http.StatusForbidden:
		logger.WithField("status", statusText).Errorln(baseLogText, "forbidden")
		return false
	case clientError:
		logger.WithField("status", statusText).Errorln(baseLogText, "client error")
		return false
	default:
		logger.WithField("status", statusText).Errorln(baseLogText, "failed")
		return false
	}
}

func (n *GitLabClient) UnregisterRunnerManager(runner common.RunnerCredentials, systemID string) bool {
	request := common.UnregisterRunnerManagerRequest{
		Token:    runner.Token,
		SystemID: systemID,
	}

	headers, correlationID := addCorrelationID(RunnerTokenHeader(runner.Token))

	result, statusText, resp := n.doJSON(
		context.Background(),
		&runner,
		http.MethodDelete,
		"runners/managers",
		http.StatusNoContent,
		headers,
		&request,
		nil,
	)
	defer closeResponseBody(resp, false)

	logger := runner.Log().WithField(correlationIDLogField, getCorrelationID(resp, correlationID))

	const baseLogText = "Unregistering runner manager from GitLab"
	switch result {
	case http.StatusNoContent:
		logger.Println(baseLogText, "succeeded")
		return true
	case http.StatusForbidden:
		logger.WithField("status", statusText).Errorln(baseLogText, "forbidden")
		return false
	case clientError:
		logger.WithField("status", statusText).Errorln(baseLogText, "client error")
		return false
	default:
		logger.WithField("status", statusText).Errorln(baseLogText, "failed")
		return false
	}
}

func (n *GitLabClient) ResetToken(runner common.RunnerCredentials, systemID string) *common.ResetTokenResponse {
	return n.resetToken(runner, systemID, "runners/reset_authentication_token", "")
}

func (n *GitLabClient) ResetTokenWithPAT(
	runner common.RunnerCredentials,
	systemID string,
	pat string,
) *common.ResetTokenResponse {
	return n.resetToken(runner, systemID, fmt.Sprintf("runners/%d/reset_authentication_token", runner.ID), pat)
}

func (n *GitLabClient) resetToken(
	runner common.RunnerCredentials,
	systemID string,
	uri string,
	pat string,
) *common.ResetTokenResponse {
	var request *common.ResetTokenRequest
	if pat == "" {
		request = &common.ResetTokenRequest{
			Token: runner.Token,
		}
	}

	headers, correlationID := addCorrelationID(PrivateTokenHeader(pat))

	var response common.ResetTokenResponse
	result, statusText, resp := n.doMeasuredJSON(
		context.Background(),
		runner.Log(),
		runner.ShortDescription(),
		systemID,
		apiEndpointResetToken,
		doJSONParams{
			credentials: &runner,
			method:      http.MethodPost,
			uri:         uri,
			statusCode:  http.StatusCreated,
			headers:     headers,
			request:     request,
			response:    &response,
		},
	)

	defer closeResponseBody(resp, false)

	logger := runner.Log().WithField(correlationIDLogField, getCorrelationID(resp, correlationID))

	const baseLogText = "Resetting runner authentication token..."
	switch result {
	case http.StatusCreated:
		logger.Println(baseLogText, "succeeded")
		response.TokenObtainedAt = time.Now().UTC()
		return &response
	case http.StatusForbidden:
		logger.WithField("status", statusText).Errorln(baseLogText, "failed (check used token)")
		return nil
	case clientError:
		logger.WithField("status", statusText).Errorln(baseLogText, "client error")
		return nil
	default:
		logger.WithField("status", statusText).Errorln(baseLogText, "failed")
		return nil
	}
}

func loadTLSData(tlsData ResponseTLSData) spec.TLSData {
	var res spec.TLSData
	if tlsData.CAChain != "" {
		res.CAChain = tlsData.CAChain
	}

	if tlsData.CertFile != "" && tlsData.KeyFile != "" {
		data, err := os.ReadFile(tlsData.CertFile)
		if err == nil {
			res.AuthCert = string(data)
		}
		data, err = os.ReadFile(tlsData.KeyFile)
		if err == nil {
			res.AuthKey = string(data)
		}
	}
	return res
}

func (n *GitLabClient) PrepareJobRequest(
	config common.RunnerConfig,
	sessionInfo *common.SessionInfo,
) common.JobRequest {
	return common.JobRequest{
		Info:       n.getRunnerInfo(config),
		Token:      config.Token,
		SystemID:   config.SystemID,
		LastUpdate: n.getLastUpdate(&config.RunnerCredentials),
		Session:    sessionInfo,
	}
}

func (n *GitLabClient) RequestJob(
	ctx context.Context,
	config common.RunnerConfig,
	sessionInfo *common.SessionInfo,
) (*spec.Job, bool) {
	request := n.PrepareJobRequest(config, sessionInfo)

	var response spec.Job

	headers, correlationID := addCorrelationID(RunnerTokenHeader(config.Token))
	//nolint:bodyclose
	result, statusText, httpResponse := n.doMeasuredJSON(
		ctx,
		config.Log(),
		config.RunnerCredentials.ShortDescription(),
		config.SystemID,
		apiEndpointRequestJob,
		doJSONParams{
			credentials: &config.RunnerCredentials,
			method:      http.MethodPost,
			uri:         "jobs/request",
			statusCode:  http.StatusCreated,
			headers:     headers,
			request:     &request,
			response:    &response,
		},
	)
	defer closeResponseBody(httpResponse, false)

	logger := config.Log().WithField(correlationIDLogField, getCorrelationID(httpResponse, correlationID))

	switch result {
	case http.StatusCreated:
		logger.WithFields(logrus.Fields{
			"job":      response.ID,
			"repo_url": response.RepoCleanURL(),
		}).Println("Checking for jobs...", "received")

		resolveFullChain := config.IsFeatureFlagOn(featureflags.ResolveFullTLSChain)
		tlsData, err := n.getResponseTLSData(&config.RunnerCredentials, resolveFullChain, httpResponse)
		if err != nil {
			logger.WithError(err).Errorln("Error on fetching TLS Data from API response...", "error")
		}
		response.TLSData = loadTLSData(tlsData)
		response.JobRequestCorrelationID = getCorrelationID(httpResponse, correlationID)

		return &response, true
	case http.StatusForbidden:
		logger.WithField("status", statusText).Errorln("Checking for jobs...", "forbidden")
		return nil, false
	case http.StatusNoContent:
		logger.WithField("status", statusText).Debug("Checking for jobs...", "no content")
		return nil, true
	case http.StatusServiceUnavailable:
		logger.WithField("status", statusText).Warningln("Checking for jobs...", "GitLab instance currently unavailable")
		return nil, true
	case clientError:
		logger.WithField("status", statusText).Errorln("Checking for jobs...", "client error")
		return nil, false
	default:
		logger.WithField("status", statusText).Warningln("Checking for jobs...", "failed")
		return nil, true
	}
}

func (n *GitLabClient) UpdateJob(
	config common.RunnerConfig,
	jobCredentials *common.JobCredentials,
	jobInfo common.UpdateJobInfo,
) common.UpdateJobResult {
	request := common.UpdateJobRequest{
		Info:          n.getRunnerInfo(config),
		Token:         jobCredentials.Token,
		State:         jobInfo.State,
		FailureReason: jobInfo.FailureReason,
		Checksum:      jobInfo.Output.Checksum, // deprecated
		Output:        jobInfo.Output,
		ExitCode:      jobInfo.ExitCode,
	}

	headers, correlationID := addCorrelationID(JobTokenHeader(jobCredentials.Token))

	log := config.Log().WithFields(logrus.Fields{
		"job":                 jobInfo.ID,
		"checksum":            request.Output.Checksum,
		"bytesize":            request.Output.Bytesize,
		correlationIDLogField: correlationID,
	})

	log.Info("Updating job...")

	//nolint:bodyclose
	statusCode, statusText, response := n.doMeasuredJSON(
		context.Background(),
		config.Log(),
		config.RunnerCredentials.ShortDescription(),
		config.SystemID,
		apiEndpointUpdateJob,
		doJSONParams{
			credentials: &config.RunnerCredentials,
			method:      http.MethodPut,
			uri:         fmt.Sprintf("jobs/%d", jobInfo.ID),
			statusCode:  http.StatusOK,
			headers:     headers,
			request:     &request,
			response:    nil,
		},
	)

	return n.createUpdateJobResult(log, statusCode, statusText, response, correlationID)
}

func (n *GitLabClient) createUpdateJobResult(
	log *logrus.Entry,
	statusCode int,
	statusText string,
	response *http.Response,
	fallbackCorrelationID string,
) common.UpdateJobResult {
	defer closeResponseBody(response, false)

	remoteJobStateResponse := NewRemoteJobStateResponse(response, log)

	result := common.UpdateJobResult{
		NewUpdateInterval: remoteJobStateResponse.RemoteUpdateInterval,
		CancelRequested:   remoteJobStateResponse.IsCanceled(),
	}

	log = log.WithFields(logrus.Fields{
		"code":                statusCode,
		"job-status":          remoteJobStateResponse.RemoteState,
		"update-interval":     remoteJobStateResponse.RemoteUpdateInterval,
		correlationIDLogField: getCorrelationID(response, fallbackCorrelationID),
	})

	switch {
	case remoteJobStateResponse.IsFailed():
		log.WithField("status", statusText).Warningln("Submitting job to coordinator...", "job failed")
		result.State = common.UpdateAbort
	case statusCode == http.StatusOK:
		log.Info("Submitting job to coordinator...", "ok")
		result.State = common.UpdateSucceeded
	case statusCode == http.StatusAccepted:
		log.Info("Submitting job to coordinator...", "accepted, but not yet completed")
		result.State = common.UpdateAcceptedButNotCompleted
	case statusCode == http.StatusPreconditionFailed:
		log.Info("Submitting job to coordinator...", "trace validation failed")
		result.State = common.UpdateTraceValidationFailed
	case statusCode == http.StatusNotFound:
		log.WithField("status", statusText).Warningln("Submitting job to coordinator...", "not found")
		result.State = common.UpdateAbort
	case statusCode == http.StatusForbidden:
		log.WithField("status", statusText).Errorln("Submitting job to coordinator...", "forbidden")
		result.State = common.UpdateAbort
	case statusCode == clientError:
		log.WithField("status", statusText).Errorln("Submitting job to coordinator...", "client error")
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
	debugTraceEnabled bool,
) common.PatchTraceResult {
	id := jobCredentials.ID

	baseLog := config.Log().WithField("job", id)
	if len(content) == 0 {
		baseLog.Info("Appending trace to coordinator...", "skipped due to empty patch")
		return common.NewPatchTraceResult(startOffset, common.PatchSucceeded, 0)
	}

	endOffset := startOffset + len(content)
	contentRange := fmt.Sprintf("%d-%d", startOffset, endOffset-1)

	headers := JobTokenHeader(jobCredentials.Token)
	headers.Set("Content-Range", contentRange)
	headers, correlationID := addCorrelationID(headers)

	bodyProvider := common.BytesProvider{Data: content}

	response, err := n.doMeasuredRaw(
		context.Background(),
		config.Log(),
		config.RunnerCredentials.ShortDescription(),
		config.SystemID,
		apiEndpointPatchTrace,
		doRawParams{
			credentials: &config.RunnerCredentials,
			method:      "PATCH",
			uri:         fmt.Sprintf("jobs/%d/trace?%s", id, patchTraceQuery(debugTraceEnabled)),
			request:     bodyProvider,
			requestType: "text/plain",
			headers:     headers,
		},
	)
	if err != nil {
		config.Log().Errorln("Appending trace to coordinator...", "error", err.Error())
		return common.NewPatchTraceResult(startOffset, common.PatchFailed, 0)
	}

	defer closeResponseBody(response, true)

	tracePatchResponse := NewTracePatchResponse(response, baseLog)
	log := baseLog.WithFields(logrus.Fields{
		"sent-log":            contentRange,
		"job-log":             tracePatchResponse.RemoteRange,
		"job-status":          tracePatchResponse.RemoteState,
		"code":                response.StatusCode,
		"status":              response.Status,
		"update-interval":     tracePatchResponse.RemoteUpdateInterval,
		correlationIDLogField: getCorrelationID(response, correlationID),
	})

	return n.createPatchTraceResult(startOffset, tracePatchResponse, response, endOffset, log)
}

func patchTraceQuery(debugTraceEnabled bool) string {
	query := url.Values{}
	query.Set("debug_trace", strconv.FormatBool(debugTraceEnabled))

	return query.Encode()
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
		log.Info("Appending trace to coordinator...", "ok")
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
		log.Errorln("Appending trace to coordinator...", "client error")
		result.State = common.PatchAbort

		return result

	default:
		log.Warningln("Appending trace to coordinator...", "failed")
		result.State = common.PatchFailed

		return result
	}
}

func (n *GitLabClient) createArtifactsContentProvider(originalContentProvider common.ContentProvider, baseName string) (common.ContentProvider, string) {
	// Create an initial multipart writer with a buffer to get its boundary
	var buf bytes.Buffer
	mpw := multipart.NewWriter(&buf)
	boundary := mpw.Boundary()
	contentType := mpw.FormDataContentType()
	mpw.Close()

	// Return a body provider function that creates a new pipe each time
	bodyProvider := common.StreamProvider{
		ReaderFactory: func() (io.ReadCloser, error) {
			// Get a fresh reader from the original provider
			originalBody, err := originalContentProvider.GetReader()
			if err != nil {
				return nil, fmt.Errorf("couldn't get original body: %w", err)
			}

			pr, pw := io.Pipe()
			mpw := multipart.NewWriter(pw)

			// Use the same boundary to ensure consistent content type
			err = mpw.SetBoundary(boundary)
			if err != nil {
				originalBody.Close()
				pr.Close()
				pw.Close()
				return nil, fmt.Errorf("couldn't set form boundary: %w", err)
			}

			// Use goroutine to write to the pipe
			go func() {
				defer func() {
					originalBody.Close()
					mpw.Close()
					pw.Close()
				}()

				wr, err := mpw.CreateFormFile("file", baseName)
				if err != nil {
					_ = pw.CloseWithError(fmt.Errorf("failed to create form file: %w", err))
					return
				}

				// Copy from the fresh reader to the multipart form
				_, err = io.Copy(wr, originalBody)
				if err != nil {
					_ = pw.CloseWithError(fmt.Errorf("failed to copy content to form: %w", err))
				}
			}()

			return pr, nil
		},
	}

	return bodyProvider, contentType
}

func (n *GitLabClient) GetRouterDiscovery(
	ctx context.Context,
	config common.RunnerConfig,
) *common.RouterDiscovery {
	var response common.RouterDiscovery

	headers, correlationID := addCorrelationID(RunnerTokenHeader(config.Token))
	//nolint:bodyclose
	result, statusText, httpResponse := n.doMeasuredJSON(
		ctx,
		config.Log(),
		config.RunnerCredentials.ShortDescription(),
		config.SystemID,
		apiEndpointDiscovery,
		doJSONParams{
			credentials: &config.RunnerCredentials,
			method:      http.MethodGet,
			uri:         "runners/router/discovery",
			statusCode:  http.StatusOK,
			headers:     headers,
			response:    &response,
		},
	)
	defer closeResponseBody(httpResponse, false)

	logger := config.Log().WithField(correlationIDLogField, getCorrelationID(httpResponse, correlationID))
	const baseLogText = "Discovering Job Router..."
	switch result {
	case http.StatusOK:
		resolveFullChain := config.IsFeatureFlagOn(featureflags.ResolveFullTLSChain)
		tlsData, err := n.getResponseTLSData(&config.RunnerCredentials, resolveFullChain, httpResponse)
		if err != nil {
			logger.WithError(err).Errorln("Error on fetching TLS Data from API response...", "error")
		}
		response.TLSData = loadTLSData(tlsData)

		return &response
	case http.StatusForbidden:
		logger.WithField("status", statusText).Errorln(baseLogText, "failed (check used token)")
	case http.StatusNotImplemented:
		logger.WithField("status", statusText).Errorln(baseLogText, "not configured/enabled")
	case clientError:
		logger.WithField("status", statusText).Errorln(baseLogText, "client error")
	default:
		logger.WithField("status", statusText).Errorln(baseLogText, "failed")
	}
	return nil
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
	originalContentProvider common.ContentProvider,
	options common.ArtifactsOptions,
) (common.UploadState, string) {
	bodyProvider, contentType := n.createArtifactsContentProvider(originalContentProvider, options.BaseName)

	query := uploadRawArtifactsQuery(options)

	headers, correlationID := addCorrelationID(JobTokenHeader(config.Token))

	res, err := n.doRaw(
		context.Background(),
		&config,
		http.MethodPost,
		fmt.Sprintf("jobs/%d/artifacts?%s", config.ID, query.Encode()),
		bodyProvider,
		contentType,
		headers,
	)

	defer closeResponseBody(res, true)

	log := logrus.WithFields(logrus.Fields{
		"id":                  config.ID,
		"token":               helpers.ShortenToken(config.Token),
		correlationIDLogField: getCorrelationID(res, correlationID),
	})

	if options.LogResponseDetails {
		logResponseDetails(log, res, true)
	}

	if res != nil {
		log = log.WithField("responseStatus", res.Status)
	}

	messagePrefix := "Uploading artifacts to coordinator..."
	if options.Type != "" {
		messagePrefix = fmt.Sprintf("Uploading artifacts as %q to coordinator...", options.Type)
	}

	if err != nil {
		log.WithError(err).Errorln(messagePrefix, "error")
		return common.UploadFailed, ""
	}

	return n.determineUploadState(res, log, messagePrefix)
}

func logResponseDetails(logger *logrus.Entry, res *http.Response, withBody bool) {
	if res == nil {
		return
	}

	fields := logrus.Fields{"body": "<nil>"}

	for k, vs := range res.Header {
		fields["header["+k+"]"] = vs
	}

	if withBody && res.Body != nil {
		body := bufio.NewReader(res.Body)
		res.Body = struct {
			io.Reader
			io.Closer
		}{body, res.Body}

		// We ignore the error here, and let other body consumers handle it, if it persists.
		b, _ := body.Peek(responseBodyPeekMax)
		if res.ContentLength > int64(len(b)) {
			b = append(b, "..."...)
		}
		fields["body"] = string(b)
	}

	logger.WithFields(fields).Warn("received response")
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

	uri := fmt.Sprintf("jobs/%d/artifacts?%s", config.ID, query.Encode())

	headers, correlationID := addCorrelationID(JobTokenHeader(config.Token))
	res, err := n.doRaw(
		context.Background(),
		&config,
		http.MethodGet,
		uri,
		nil,
		"",
		headers,
	)

	log := logrus.WithFields(logrus.Fields{
		"id":                  config.ID,
		"token":               helpers.ShortenToken(config.Token),
		correlationIDLogField: getCorrelationID(res, correlationID),
	})

	if res != nil {
		log = log.WithField("responseStatus", res.Status)

		if res.Request != nil && res.Request.URL != nil {
			log = log.WithField("host", res.Request.URL.Host)
		}
	}

	if err != nil {
		log.Errorln("Downloading artifacts from coordinator...", "error", err.Error())
		return common.DownloadFailed
	}
	defer closeResponseBody(res, true)

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
	case http.StatusUnauthorized:
		log.WithField("status", res.Status).Errorln("Downloading artifacts from coordinator...", "unauthorized")
		return common.DownloadUnauthorized
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
	l := logrus.New().WithFields(logrus.Fields{
		"runner":      config.ShortDescription(),
		"runner_name": config.Name,
		"job":         jobCredentials.ID,
	})

	trace, err := newJobTrace(n, config, jobCredentials, l)
	if err != nil {
		return nil, fmt.Errorf("create job trace: %w", err)
	}

	trace.start()
	return trace, nil
}

func closeResponseBody(res *http.Response, discardBody bool) {
	if res == nil {
		return
	}
	if discardBody {
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 1025*1025))
	}
	_ = res.Body.Close()
}

type ClientOption func(*GitLabClient)

func WithAPIRequestsCollector(collector *APIRequestsCollector) ClientOption {
	return func(c *GitLabClient) {
		c.apiRequestsCollector = collector
	}
}

func WithCertificateDirectory(certDirectory string) ClientOption {
	return func(c *GitLabClient) {
		c.certDirectory = certDirectory
	}
}

func NewGitLabClient(options ...ClientOption) *GitLabClient {
	c := &GitLabClient{}
	for _, o := range options {
		o(c)
	}
	if c.apiRequestsCollector == nil {
		c.apiRequestsCollector = NewAPIRequestsCollector()
	}
	return c
}

func getCorrelationID(resp *http.Response, fallbackValue string) string {
	if resp == nil || resp.Header.Get(correlationIDHeader) == "" {
		return fallbackValue
	}
	return resp.Header.Get(correlationIDHeader)
}
