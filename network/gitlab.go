package network

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"

	"github.com/Sirupsen/logrus"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

const clientError = -100

type GitLabClient struct {
	clients map[string]*client
	lock    sync.Mutex
}

func (n *GitLabClient) getClient(credentials requestCredentials) (c *client, err error) {
	n.lock.Lock()
	defer n.lock.Unlock()

	if n.clients == nil {
		n.clients = make(map[string]*client)
	}
	key := fmt.Sprintf("%s_%s_%s_%s", credentials.GetURL(), credentials.GetToken(), credentials.GetTLSCAFile(), credentials.GetTLSCertFile())
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

func (n *GitLabClient) getRunnerVersion(config common.RunnerConfig) common.VersionInfo {
	info := common.VersionInfo{
		Name:         common.NAME,
		Version:      common.VERSION,
		Revision:     common.REVISION,
		Platform:     runtime.GOOS,
		Architecture: runtime.GOARCH,
		Executor:     config.Executor,
	}

	if executor := common.GetExecutor(config.Executor); executor != nil {
		executor.GetFeatures(&info.Features)
	}

	if shell := common.GetShell(config.Shell); shell != nil {
		shell.GetFeatures(&info.Features)
	}

	return info
}

func (n *GitLabClient) doRaw(credentials requestCredentials, method, uri string, request io.Reader, requestType string, headers http.Header) (res *http.Response, err error) {
	c, err := n.getClient(credentials)
	if err != nil {
		return nil, err
	}

	return c.do(uri, method, request, requestType, headers)
}

func (n *GitLabClient) doJSON(credentials requestCredentials, method, uri string, statusCode int, request interface{}, response interface{}) (int, string, ResponseTLSData) {
	c, err := n.getClient(credentials)
	if err != nil {
		return clientError, err.Error(), ResponseTLSData{}
	}

	return c.doJSON(uri, method, statusCode, request, response)
}

func (n *GitLabClient) RegisterRunner(runner common.RunnerCredentials, description, tags string, runUntagged, locked bool, maxTimeout int) *common.RegisterRunnerResponse {
	// TODO: pass executor
	request := common.RegisterRunnerRequest{
		Token:          runner.Token,
		Description:    description,
		Info:           n.getRunnerVersion(common.RunnerConfig{}),
		Locked:         locked,
		RunUntagged:    runUntagged,
		Tags:           tags,
		MaximumTimeout: maxTimeout,
	}

	var response common.RegisterRunnerResponse
	result, statusText, _ := n.doJSON(&runner, "POST", "runners", http.StatusCreated, &request, &response)

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

	result, statusText, _ := n.doJSON(&runner, "POST", "runners/verify", http.StatusOK, &request, nil)

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

	result, statusText, _ := n.doJSON(&runner, "DELETE", "runners", http.StatusNoContent, &request, nil)

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

func (n *GitLabClient) RequestJob(config common.RunnerConfig) (*common.JobResponse, bool) {
	request := common.JobRequest{
		Info:       n.getRunnerVersion(config),
		Token:      config.Token,
		LastUpdate: n.getLastUpdate(&config.RunnerCredentials),
	}

	var response common.JobResponse
	result, statusText, tlsData := n.doJSON(&config.RunnerCredentials, "POST", "jobs/request", http.StatusCreated, &request, &response)

	switch result {
	case http.StatusCreated:
		config.Log().WithFields(logrus.Fields{
			"job":      strconv.Itoa(response.ID),
			"repo_url": response.RepoCleanURL(),
		}).Println("Checking for jobs...", "received")
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

func (n *GitLabClient) UpdateJob(config common.RunnerConfig, jobCredentials *common.JobCredentials, jobInfo common.UpdateJobInfo) common.UpdateState {
	request := common.UpdateJobRequest{
		Info:          n.getRunnerVersion(config),
		Token:         jobCredentials.Token,
		State:         jobInfo.State,
		FailureReason: jobInfo.FailureReason,
		Trace:         jobInfo.Trace,
	}

	log := config.Log().WithField("job", jobInfo.ID)

	result, statusText, _ := n.doJSON(&config.RunnerCredentials, "PUT", fmt.Sprintf("jobs/%d", jobInfo.ID), http.StatusOK, &request, nil)
	switch result {
	case http.StatusOK:
		log.Debugln("Submitting job to coordinator...", "ok")
		return common.UpdateSucceeded
	case http.StatusNotFound:
		log.Warningln("Submitting job to coordinator...", "aborted")
		return common.UpdateAbort
	case http.StatusForbidden:
		log.WithField("status", statusText).Errorln("Submitting job to coordinator...", "forbidden")
		return common.UpdateAbort
	case clientError:
		log.WithField("status", statusText).Errorln("Submitting job to coordinator...", "error")
		return common.UpdateAbort
	default:
		log.WithField("status", statusText).Warningln("Submitting job to coordinator...", "failed")
		return common.UpdateFailed
	}
}

func (n *GitLabClient) PatchTrace(config common.RunnerConfig, jobCredentials *common.JobCredentials, tracePatch common.JobTracePatch) common.UpdateState {
	id := jobCredentials.ID

	contentRange := fmt.Sprintf("%d-%d", tracePatch.Offset(), tracePatch.Limit())
	headers := make(http.Header)
	headers.Set("Content-Range", contentRange)
	headers.Set("JOB-TOKEN", jobCredentials.Token)

	uri := fmt.Sprintf("jobs/%d/trace", id)
	request := bytes.NewReader(tracePatch.Patch())

	response, err := n.doRaw(&config.RunnerCredentials, "PATCH", uri, request, "text/plain", headers)
	if err != nil {
		config.Log().Errorln("Appending trace to coordinator...", "error", err.Error())
		return common.UpdateFailed
	}

	defer response.Body.Close()
	defer io.Copy(ioutil.Discard, response.Body)

	tracePatchResponse := NewTracePatchResponse(response)
	log := config.Log().WithFields(logrus.Fields{
		"job":        id,
		"sent-log":   contentRange,
		"job-log":    tracePatchResponse.RemoteRange,
		"job-status": tracePatchResponse.RemoteState,
		"code":       response.StatusCode,
		"status":     response.Status,
	})

	switch {
	case tracePatchResponse.IsAborted():
		log.Warningln("Appending trace to coordinator", "aborted")
		return common.UpdateAbort
	case response.StatusCode == http.StatusAccepted:
		log.Debugln("Appending trace to coordinator...", "ok")
		return common.UpdateSucceeded
	case response.StatusCode == http.StatusNotFound:
		log.Warningln("Appending trace to coordinator...", "not-found")
		return common.UpdateNotFound
	case response.StatusCode == http.StatusRequestedRangeNotSatisfiable:
		log.Warningln("Appending trace to coordinator...", "range mismatch")
		tracePatch.SetNewOffset(tracePatchResponse.NewOffset())
		return common.UpdateRangeMismatch
	case response.StatusCode == clientError:
		log.Errorln("Appending trace to coordinator...", "error")
		return common.UpdateAbort
	default:
		log.Warningln("Appending trace to coordinator...", "failed")
		return common.UpdateFailed
	}
}

func (n *GitLabClient) createArtifactsForm(mpw *multipart.Writer, reader io.Reader, baseName string) error {
	wr, err := mpw.CreateFormFile("file", baseName)
	if err != nil {
		return err
	}

	_, err = io.Copy(wr, reader)
	if err != nil {
		return err
	}
	return nil
}

func (n *GitLabClient) UploadRawArtifacts(config common.JobCredentials, reader io.Reader, baseName string, expireIn string) common.UploadState {
	pr, pw := io.Pipe()
	defer pr.Close()

	mpw := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()
		defer mpw.Close()
		err := n.createArtifactsForm(mpw, reader, baseName)
		if err != nil {
			pw.CloseWithError(err)
		}
	}()

	query := url.Values{}
	if expireIn != "" {
		query.Set("expire_in", expireIn)
	}

	headers := make(http.Header)
	headers.Set("JOB-TOKEN", config.Token)
	res, err := n.doRaw(&config, "POST", fmt.Sprintf("jobs/%d/artifacts?%s", config.ID, query.Encode()), pr, mpw.FormDataContentType(), headers)

	log := logrus.WithFields(logrus.Fields{
		"id":    config.ID,
		"token": helpers.ShortenToken(config.Token),
	})

	if res != nil {
		log = log.WithField("responseStatus", res.Status)
	}

	if err != nil {
		log.WithError(err).Errorln("Uploading artifacts to coordinator...", "error")
		return common.UploadFailed
	}
	defer res.Body.Close()
	defer io.Copy(ioutil.Discard, res.Body)

	switch res.StatusCode {
	case http.StatusCreated:
		log.Println("Uploading artifacts to coordinator...", "ok")
		return common.UploadSucceeded
	case http.StatusForbidden:
		log.WithField("status", res.Status).Errorln("Uploading artifacts to coordinator...", "forbidden")
		return common.UploadForbidden
	case http.StatusRequestEntityTooLarge:
		log.WithField("status", res.Status).Errorln("Uploading artifacts to coordinator...", "too large archive")
		return common.UploadTooLarge
	default:
		log.WithField("status", res.Status).Warningln("Uploading artifacts to coordinator...", "failed")
		return common.UploadFailed
	}
}

func (n *GitLabClient) UploadArtifacts(config common.JobCredentials, artifactsFile string) common.UploadState {
	log := logrus.WithFields(logrus.Fields{
		"id":    config.ID,
		"token": helpers.ShortenToken(config.Token),
	})

	file, err := os.Open(artifactsFile)
	if err != nil {
		log.WithError(err).Errorln("Uploading artifacts to coordinator...", "error")
		return common.UploadFailed
	}
	defer file.Close()

	fi, err := file.Stat()
	if err != nil {
		log.WithError(err).Errorln("Uploading artifacts to coordinator...", "error")
		return common.UploadFailed
	}
	if fi.IsDir() {
		log.WithField("error", "cannot upload directories").Errorln("Uploading artifacts to coordinator...", "error")
		return common.UploadFailed
	}

	baseName := filepath.Base(artifactsFile)
	return n.UploadRawArtifacts(config, file, baseName, "")
}

func (n *GitLabClient) DownloadArtifacts(config common.JobCredentials, artifactsFile string) common.DownloadState {
	headers := make(http.Header)
	headers.Set("JOB-TOKEN", config.Token)
	res, err := n.doRaw(&config, "GET", fmt.Sprintf("jobs/%d/artifacts", config.ID), nil, "", headers)

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
	defer res.Body.Close()
	defer io.Copy(ioutil.Discard, res.Body)

	switch res.StatusCode {
	case http.StatusOK:
		file, err := os.Create(artifactsFile)
		if err == nil {
			defer file.Close()
			_, err = io.Copy(file, res.Body)
		}
		if err != nil {
			file.Close()
			os.Remove(file.Name())
			log.WithError(err).Errorln("Downloading artifacts from coordinator...", "error")
			return common.DownloadFailed
		}
		log.Println("Downloading artifacts from coordinator...", "ok")
		return common.DownloadSucceeded
	case http.StatusForbidden:
		log.WithField("status", res.Status).Errorln("Downloading artifacts from coordinator...", "forbidden")
		return common.DownloadForbidden
	case http.StatusNotFound:
		log.Errorln("Downloading artifacts from coordinator...", "not found")
		return common.DownloadNotFound
	default:
		log.WithField("status", res.Status).Warningln("Downloading artifacts from coordinator...", "failed")
		return common.DownloadFailed
	}
}

func (n *GitLabClient) ProcessJob(config common.RunnerConfig, jobCredentials *common.JobCredentials) common.JobTrace {
	trace := newJobTrace(n, config, jobCredentials)
	trace.start()
	return trace
}

func NewGitLabClient() *GitLabClient {
	return &GitLabClient{}
}
