package network

import (
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
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

const clientError = -100

type GitLabClient struct {
	clients map[string]*client
	lock    sync.Mutex

	apiRequestsCollector *APIRequestsCollector
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

type doRawParams struct {
	credentials requestCredentials
	method      string
	uri         string
	request     io.Reader
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

	fn := func() int {
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
			return clientError
		}

		return response.StatusCode
	}

	n.apiRequestsCollector.Observe(
		log,
		runnerID,
		systemID,
		endpoint,
		fn,
	)

	return response, err
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

type doJSONParams struct {
	credentials requestCredentials
	method      string
	uri         string
	statusCode  int
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

	fn := func() int {
		// Response body is handled after doMeasuredJSON() decorator call
		// Linting violation here is a false-positive.
		// nolint:bodyclose
		result, statusText, httpResponse = n.doJSON(
			ctx,
			params.credentials,
			params.method,
			params.uri,
			params.statusCode,
			params.request,
			params.response,
		)

		return result
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

func (n *GitLabClient) doJSON(
	ctx context.Context,
	credentials requestCredentials,
	method, uri string,
	statusCode int,
	request interface{},
	response interface{},
) (int, string, *http.Response) {
	return n.doJSONWithPAT(ctx, credentials, method, uri, statusCode, "", request, response)
}

type doJSONWithPATParams struct {
	credentials requestCredentials
	method      string
	uri         string
	statusCode  int
	pat         string
	request     interface{}
	response    interface{}
}

// doMeasuredJSONWithPAT is a decorator that adds metrics measurements through
// n.apiRequestsCollector to the doJSONWithPAT() call
func (n *GitLabClient) doMeasuredJSONWithPAT(
	ctx context.Context,
	log logrus.FieldLogger,
	runnerID string,
	systemID string,
	endpoint apiEndpoint,
	params doJSONWithPATParams,
) (int, string, *http.Response) {
	var result int
	var statusText string
	var httpResponse *http.Response

	fn := func() int {
		// Response body is handled after doJSONWithPATParams() decorator call
		// Linting violation here is a false-positive.
		// nolint:bodyclose
		result, statusText, httpResponse = n.doJSONWithPAT(
			ctx,
			params.credentials,
			params.method,
			params.uri,
			params.statusCode,
			params.pat,
			params.request,
			params.response,
		)

		return result
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

func (n *GitLabClient) doJSONWithPAT(
	ctx context.Context,
	credentials requestCredentials,
	method, uri string,
	statusCode int,
	pat string,
	request interface{},
	response interface{},
) (int, string, *http.Response) {
	c, err := n.getClient(credentials)
	if err != nil {
		return clientError, err.Error(), nil
	}

	return c.doJSONWithPAT(ctx, uri, method, statusCode, pat, request, response)
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

func (n *GitLabClient) VerifyRunner(runner common.RunnerCredentials, systemID string) *common.VerifyRunnerResponse {
	request := common.VerifyRunnerRequest{
		Token:    runner.Token,
		SystemID: systemID,
	}

	var response common.VerifyRunnerResponse
	result, statusText, resp := n.doJSON(
		context.Background(),
		&runner,
		http.MethodPost,
		"runners/verify",
		http.StatusOK,
		&request,
		&response,
	)
	if result == -1 {
		// if server is not able to return JSON, let's try plain text (the legacy response format)
		result, statusText, resp = n.doJSON(
			context.Background(),
			&runner,
			http.MethodPost,
			"runners/verify",
			http.StatusOK,
			&request,
			nil,
		)
	}
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}

	switch result {
	case http.StatusOK:
		// this is expected due to fact that we ask for non-existing job
		runner.Log().Println("Verifying runner...", "is alive")
		return &response
	case http.StatusForbidden:
		runner.Log().Errorln("Verifying runner...", "is removed")
		return nil
	case clientError:
		runner.Log().WithField("status", statusText).Errorln("Verifying runner...", "error")
		return &response
	default:
		runner.Log().WithField("status", statusText).Errorln("Verifying runner...", "failed")
		return &response
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

	var response common.ResetTokenResponse

	result, statusText, resp := n.doMeasuredJSONWithPAT(
		context.Background(),
		runner.Log(),
		runner.ShortDescription(),
		systemID,
		apiEndpointResetToken,
		doJSONWithPATParams{
			credentials: &runner,
			method:      http.MethodPost,
			uri:         uri,
			statusCode:  http.StatusCreated,
			pat:         pat,
			request:     request,
			response:    &response,
		},
	)

	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}

	const baseLogText = "Resetting runner token..."
	switch result {
	case http.StatusCreated:
		runner.Log().Println(baseLogText, "succeeded")
		response.TokenObtainedAt = time.Now().UTC()
		return &response
	case http.StatusForbidden:
		runner.Log().Errorln(baseLogText, "failed (check used token)")
		return nil
	case clientError:
		runner.Log().WithField("status", statusText).Errorln(baseLogText, "error")
		return nil
	default:
		runner.Log().WithField("status", statusText).Errorln(baseLogText, "failed")
		return nil
	}
}

func addTLSData(response *common.JobResponse, tlsData ResponseTLSData) {
	if tlsData.CAChain != "" {
		response.TLSCAChain = tlsData.CAChain
	}

	if tlsData.CertFile != "" && tlsData.KeyFile != "" {
		data, err := os.ReadFile(tlsData.CertFile)
		if err == nil {
			response.TLSAuthCert = string(data)
		}
		data, err = os.ReadFile(tlsData.KeyFile)
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
		SystemID:   config.SystemIDState.GetSystemID(),
		LastUpdate: n.getLastUpdate(&config.RunnerCredentials),
		Session:    sessionInfo,
	}

	var response common.JobResponse

	result, statusText, httpResponse := n.doMeasuredJSON(
		ctx,
		config.Log(),
		config.RunnerCredentials.ShortDescription(),
		config.SystemIDState.GetSystemID(),
		apiEndpointRequestJob,
		doJSONParams{
			credentials: &config.RunnerCredentials,
			method:      http.MethodPost,
			uri:         "jobs/request",
			statusCode:  http.StatusCreated,
			request:     &request, response: &response,
		},
	)

	switch result {
	case http.StatusCreated:
		config.Log().WithFields(logrus.Fields{
			"job":      response.ID,
			"repo_url": response.RepoCleanURL(),
		}).Println("Checking for jobs...", "received")

		resolveFullChain := config.IsFeatureFlagOn(featureflags.ResolveFullTLSChain)
		tlsData, err := n.getResponseTLSData(&config.RunnerCredentials, resolveFullChain, httpResponse)
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
		config.Log().Debug("Checking for jobs...", "nothing")
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

	log := config.Log().
		WithField("job", jobInfo.ID).
		WithField("checksum", request.Output.Checksum).
		WithField("bytesize", request.Output.Bytesize)

	log.Info("Updating job...")

	statusCode, statusText, response := n.doMeasuredJSON(
		context.Background(),
		config.Log(),
		config.RunnerCredentials.ShortDescription(),
		config.SystemIDState.GetSystemID(),
		apiEndpointUpdateJob,
		doJSONParams{
			credentials: &config.RunnerCredentials,
			method:      http.MethodPut,
			uri:         fmt.Sprintf("jobs/%d", jobInfo.ID),
			statusCode:  http.StatusOK,
			request:     &request,
			response:    nil,
		},
	)

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
		log.Info("Submitting job to coordinator...", "ok")
		result.State = common.UpdateSucceeded
	case statusCode == http.StatusAccepted:
		log.Info("Submitting job to coordinator...", "accepted, but not yet completed")
		result.State = common.UpdateAcceptedButNotCompleted
	case statusCode == http.StatusPreconditionFailed:
		log.Info("Submitting job to coordinator...", "trace validation failed")
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

	headers := make(http.Header)
	headers.Set("Content-Range", contentRange)
	headers.Set("JOB-TOKEN", jobCredentials.Token)

	response, err := n.doMeasuredRaw(
		context.Background(),
		config.Log(),
		config.RunnerCredentials.ShortDescription(),
		config.SystemIDState.GetSystemID(),
		apiEndpointPatchTrace,
		doRawParams{
			credentials: &config.RunnerCredentials,
			method:      "PATCH",
			uri:         fmt.Sprintf("jobs/%d/trace?%s", id, patchTraceQuery(debugTraceEnabled)),
			request:     bytes.NewReader(content),
			requestType: "text/plain",
			headers:     headers,
		},
	)
	if err != nil {
		config.Log().Errorln("Appending trace to coordinator...", "error", err.Error())
		return common.NewPatchTraceResult(startOffset, common.PatchFailed, 0)
	}

	defer func() {
		_, _ = io.Copy(io.Discard, response.Body)
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
		_, _ = io.Copy(io.Discard, res.Body)
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

		if res.Request != nil && res.Request.URL != nil {
			log = log.WithField("host", res.Request.URL.Host)
		}
	}

	if err != nil {
		log.Errorln("Downloading artifacts from coordinator...", "error", err.Error())
		return common.DownloadFailed
	}
	defer func() {
		_, _ = io.Copy(io.Discard, res.Body)
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
	trace, err := newJobTrace(n, config, jobCredentials)
	if err != nil {
		return nil, err
	}

	trace.start()
	return trace, nil
}

func NewGitLabClientWithAPIRequestsCollector(c *APIRequestsCollector) *GitLabClient {
	return &GitLabClient{
		apiRequestsCollector: c,
	}
}

func NewGitLabClient() *GitLabClient {
	return NewGitLabClientWithAPIRequestsCollector(NewAPIRequestsCollector())
}
