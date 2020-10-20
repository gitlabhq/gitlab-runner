package network

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	. "gitlab.com/gitlab-org/gitlab-runner/common"
)

const (
	validToken   = "valid"
	invalidToken = "invalid"
)

var brokenCredentials = RunnerCredentials{
	URL: "broken",
}

var brokenConfig = RunnerConfig{
	RunnerCredentials: brokenCredentials,
}

func TestClients(t *testing.T) {
	c := NewGitLabClient()
	c1, _ := c.getClient(&RunnerCredentials{
		URL: "http://test/",
	})
	c2, _ := c.getClient(&RunnerCredentials{
		URL: "http://test2/",
	})
	c4, _ := c.getClient(&RunnerCredentials{
		URL:       "http://test/",
		TLSCAFile: "ca_file",
	})
	c5, _ := c.getClient(&RunnerCredentials{
		URL:       "http://test/",
		TLSCAFile: "ca_file",
	})
	c6, _ := c.getClient(&RunnerCredentials{
		URL:         "http://test/",
		TLSCAFile:   "ca_file",
		TLSCertFile: "cert_file",
		TLSKeyFile:  "key_file",
	})
	c7, _ := c.getClient(&RunnerCredentials{
		URL:         "http://test/",
		TLSCAFile:   "ca_file",
		TLSCertFile: "cert_file",
		TLSKeyFile:  "key_file2",
	})
	c8, c8err := c.getClient(&brokenCredentials)
	assert.NotEqual(t, c1, c2)
	assert.NotEqual(t, c1, c4)
	assert.Equal(t, c4, c5)
	assert.NotEqual(t, c5, c6)
	assert.Equal(t, c6, c7)
	assert.Nil(t, c8)
	assert.Error(t, c8err)
}

func testRegisterRunnerHandler(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.URL.Path != "/api/v4/runners" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotAcceptable)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	assert.NoError(t, err)

	var req map[string]interface{}
	err = json.Unmarshal(body, &req)
	assert.NoError(t, err)

	res := make(map[string]interface{})

	switch req["token"].(string) {
	case validToken:
		if req["description"].(string) != "test" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		res["token"] = req["token"].(string)
	case invalidToken:
		w.WriteHeader(http.StatusForbidden)
		return
	default:
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if r.Header.Get("Accept") != "application/json" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	output, err := json.Marshal(res)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write(output)
}

func TestRegisterRunner(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testRegisterRunnerHandler(w, r, t)
	}))
	defer s.Close()

	validToken := RunnerCredentials{
		URL:   s.URL,
		Token: validToken,
	}

	invalidToken := RunnerCredentials{
		URL:   s.URL,
		Token: invalidToken,
	}

	otherToken := RunnerCredentials{
		URL:   s.URL,
		Token: "other",
	}

	c := NewGitLabClient()

	res := c.RegisterRunner(
		validToken,
		RegisterRunnerParameters{
			Description: "test",
			Tags:        "tags",
			RunUntagged: true,
			Locked:      true,
			Active:      true,
		})
	if assert.NotNil(t, res) {
		assert.Equal(t, validToken.Token, res.Token)
	}

	res = c.RegisterRunner(
		validToken,
		RegisterRunnerParameters{
			Description: "invalid description",
			Tags:        "tags",
			RunUntagged: true,
			Locked:      true,
			AccessLevel: "not_protected",
			Active:      true,
		})
	assert.Nil(t, res)

	res = c.RegisterRunner(
		invalidToken,
		RegisterRunnerParameters{
			Description: "test",
			Tags:        "tags",
			RunUntagged: true,
			Locked:      true,
			AccessLevel: "not_protected",
			Active:      true,
		})
	assert.Nil(t, res)

	res = c.RegisterRunner(
		otherToken,
		RegisterRunnerParameters{
			Description: "test",
			Tags:        "tags",
			RunUntagged: true,
			Locked:      true,
			AccessLevel: "not_protected",
			Active:      true,
		})
	assert.Nil(t, res)

	res = c.RegisterRunner(
		brokenCredentials,
		RegisterRunnerParameters{
			Description: "test",
			Tags:        "tags",
			RunUntagged: true,
			Locked:      true,
			AccessLevel: "not_protected",
			Active:      true,
		})
	assert.Nil(t, res)
}

func testUnregisterRunnerHandler(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.URL.Path != "/api/v4/runners" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusNotAcceptable)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	assert.NoError(t, err)

	var req map[string]interface{}
	err = json.Unmarshal(body, &req)
	assert.NoError(t, err)

	switch req["token"].(string) {
	case validToken:
		w.WriteHeader(http.StatusNoContent)
	case invalidToken:
		w.WriteHeader(http.StatusForbidden)
	default:
		w.WriteHeader(http.StatusBadRequest)
	}
}

func TestUnregisterRunner(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		testUnregisterRunnerHandler(w, r, t)
	}

	s := httptest.NewServer(http.HandlerFunc(handler))
	defer s.Close()

	validToken := RunnerCredentials{
		URL:   s.URL,
		Token: validToken,
	}

	invalidToken := RunnerCredentials{
		URL:   s.URL,
		Token: invalidToken,
	}

	otherToken := RunnerCredentials{
		URL:   s.URL,
		Token: "other",
	}

	c := NewGitLabClient()

	state := c.UnregisterRunner(validToken)
	assert.True(t, state)

	state = c.UnregisterRunner(invalidToken)
	assert.False(t, state)

	state = c.UnregisterRunner(otherToken)
	assert.False(t, state)

	state = c.UnregisterRunner(brokenCredentials)
	assert.False(t, state)
}

func testVerifyRunnerHandler(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.URL.Path != "/api/v4/runners/verify" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotAcceptable)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	assert.NoError(t, err)

	var req map[string]interface{}
	err = json.Unmarshal(body, &req)
	assert.NoError(t, err)

	switch req["token"].(string) {
	case validToken:
		w.WriteHeader(http.StatusOK) // since the job id is broken, we should not find this job
	case invalidToken:
		w.WriteHeader(http.StatusForbidden)
	default:
		w.WriteHeader(http.StatusBadRequest)
	}
}

func TestVerifyRunner(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		testVerifyRunnerHandler(w, r, t)
	}

	s := httptest.NewServer(http.HandlerFunc(handler))
	defer s.Close()

	validToken := RunnerCredentials{
		URL:   s.URL,
		Token: validToken,
	}

	invalidToken := RunnerCredentials{
		URL:   s.URL,
		Token: invalidToken,
	}

	otherToken := RunnerCredentials{
		URL:   s.URL,
		Token: "other",
	}

	c := NewGitLabClient()

	state := c.VerifyRunner(validToken)
	assert.True(t, state)

	state = c.VerifyRunner(invalidToken)
	assert.False(t, state)

	state = c.VerifyRunner(otherToken)
	assert.True(t, state, "in other cases where we can't explicitly say that runner is valid we say that it's")

	state = c.VerifyRunner(brokenCredentials)
	assert.True(t, state, "in other cases where we can't explicitly say that runner is valid we say that it's")
}

func getRequestJobResponse() map[string]interface{} {
	jobToken := "job-token"

	res := make(map[string]interface{})
	res["id"] = 10
	res["token"] = jobToken
	res["allow_git_fetch"] = false

	jobInfo := make(map[string]interface{})
	jobInfo["name"] = "test-job"
	jobInfo["stage"] = "test"
	jobInfo["project_id"] = 123
	jobInfo["project_name"] = "test-project"
	res["job_info"] = jobInfo

	gitInfo := make(map[string]interface{})
	gitInfo["repo_url"] = "https://gitlab-ci-token:testTokenHere1234@gitlab.example.com/test/test-project.git"
	gitInfo["ref"] = "master"
	gitInfo["sha"] = "abcdef123456"
	gitInfo["before_sha"] = "654321fedcba"
	gitInfo["ref_type"] = "branch"
	res["git_info"] = gitInfo

	runnerInfo := make(map[string]interface{})
	runnerInfo["timeout"] = 3600
	res["runner_info"] = runnerInfo

	variables := make([]map[string]interface{}, 1)
	variables[0] = make(map[string]interface{})
	variables[0]["key"] = "CI_REF_NAME"
	variables[0]["value"] = "master"
	variables[0]["public"] = true
	variables[0]["file"] = true
	variables[0]["raw"] = true
	res["variables"] = variables

	steps := make([]map[string]interface{}, 2)
	steps[0] = make(map[string]interface{})
	steps[0]["name"] = "script"
	steps[0]["script"] = []string{"date", "ls -ls"}
	steps[0]["timeout"] = 3600
	steps[0]["when"] = "on_success"
	steps[0]["allow_failure"] = false
	steps[1] = make(map[string]interface{})
	steps[1]["name"] = "after_script"
	steps[1]["script"] = []string{"ls -ls"}
	steps[1]["timeout"] = 3600
	steps[1]["when"] = "always"
	steps[1]["allow_failure"] = true
	res["steps"] = steps

	image := make(map[string]interface{})
	image["name"] = "ruby:2.6"
	image["entrypoint"] = []string{"/bin/sh"}
	res["image"] = image

	services := make([]map[string]interface{}, 2)
	services[0] = make(map[string]interface{})
	services[0]["name"] = "postgresql:9.5"
	services[0]["entrypoint"] = []string{"/bin/sh"}
	services[0]["command"] = []string{"sleep", "30"}
	services[0]["alias"] = "db-pg"
	services[1] = make(map[string]interface{})
	services[1]["name"] = "mysql:5.6"
	services[1]["alias"] = "db-mysql"
	res["services"] = services

	artifacts := make([]map[string]interface{}, 1)
	artifacts[0] = make(map[string]interface{})
	artifacts[0]["name"] = "artifact.zip"
	artifacts[0]["untracked"] = false
	artifacts[0]["paths"] = []string{"out/*"}
	artifacts[0]["when"] = "always"
	artifacts[0]["expire_in"] = "7d"
	res["artifacts"] = artifacts

	cache := make([]map[string]interface{}, 1)
	cache[0] = make(map[string]interface{})
	cache[0]["key"] = "$CI_COMMIT_SHA"
	cache[0]["untracked"] = false
	cache[0]["paths"] = []string{"vendor/*"}
	cache[0]["policy"] = "push"
	res["cache"] = cache

	credentials := make([]map[string]interface{}, 1)
	credentials[0] = make(map[string]interface{})
	credentials[0]["type"] = "Registry"
	credentials[0]["url"] = "http://registry.gitlab.example.com/"
	credentials[0]["username"] = "gitlab-ci-token"
	credentials[0]["password"] = jobToken
	res["credentials"] = credentials

	dependencies := make([]map[string]interface{}, 1)
	dependencies[0] = make(map[string]interface{})
	dependencies[0]["id"] = 9
	dependencies[0]["name"] = "other-job"
	dependencies[0]["token"] = "other-job-token"
	artifactsFile0 := make(map[string]interface{})
	artifactsFile0["filename"] = "binaries.zip"
	artifactsFile0["size"] = 13631488
	dependencies[0]["artifacts_file"] = artifactsFile0
	res["dependencies"] = dependencies

	return res
}

func testRequestJobHandler(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.URL.Path != "/api/v4/jobs/request" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotAcceptable)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	assert.NoError(t, err)

	var req map[string]interface{}
	err = json.Unmarshal(body, &req)
	assert.NoError(t, err)

	switch req["token"].(string) {
	case validToken:
	case "no-jobs":
		w.Header().Add("X-GitLab-Last-Update", "a nice timestamp")
		w.WriteHeader(http.StatusNoContent)
		return
	case invalidToken:
		w.WriteHeader(http.StatusForbidden)
		return
	default:
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if r.Header.Get("Accept") != "application/json" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	output, err := json.Marshal(getRequestJobResponse())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write(output)
	t.Logf("JobRequest response: %s\n", output)
}

func TestRequestJob(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testRequestJobHandler(w, r, t)
	}))
	defer s.Close()

	validToken := RunnerConfig{
		RunnerCredentials: RunnerCredentials{
			URL:   s.URL,
			Token: validToken,
		},
	}

	noJobsToken := RunnerConfig{
		RunnerCredentials: RunnerCredentials{
			URL:   s.URL,
			Token: "no-jobs",
		},
	}

	invalidToken := RunnerConfig{
		RunnerCredentials: RunnerCredentials{
			URL:   s.URL,
			Token: invalidToken,
		},
	}

	c := NewGitLabClient()

	res, ok := c.RequestJob(validToken, nil)
	if assert.NotNil(t, res) {
		assert.NotEmpty(t, res.ID)
	}
	assert.True(t, ok)

	assert.Equal(t, "ruby:2.6", res.Image.Name)
	assert.Equal(t, []string{"/bin/sh"}, res.Image.Entrypoint)
	require.Len(t, res.Services, 2)
	assert.Equal(t, "postgresql:9.5", res.Services[0].Name)
	assert.Equal(t, []string{"/bin/sh"}, res.Services[0].Entrypoint)
	assert.Equal(t, []string{"sleep", "30"}, res.Services[0].Command)
	assert.Equal(t, "db-pg", res.Services[0].Alias)
	assert.Equal(t, "mysql:5.6", res.Services[1].Name)
	assert.Equal(t, "db-mysql", res.Services[1].Alias)

	require.Len(t, res.Variables, 1)
	assert.Equal(t, "CI_REF_NAME", res.Variables[0].Key)
	assert.Equal(t, "master", res.Variables[0].Value)
	assert.True(t, res.Variables[0].Public)
	assert.True(t, res.Variables[0].File)
	assert.True(t, res.Variables[0].Raw)

	assert.Empty(t, c.getLastUpdate(&noJobsToken.RunnerCredentials), "Last-Update should not be set")
	res, ok = c.RequestJob(noJobsToken, nil)
	assert.Nil(t, res)
	assert.True(t, ok, "If no jobs, runner is healthy")
	assert.Equal(t, "a nice timestamp", c.getLastUpdate(&noJobsToken.RunnerCredentials), "Last-Update should be set")

	res, ok = c.RequestJob(invalidToken, nil)
	assert.Nil(t, res)
	assert.False(t, ok, "If token is invalid, the runner is unhealthy")

	res, ok = c.RequestJob(brokenConfig, nil)
	assert.Nil(t, res)
	assert.False(t, ok)
}

func setStateForUpdateJobHandlerResponse(w http.ResponseWriter, req map[string]interface{}) {
	switch req["state"].(string) {
	case "running", "canceling":
		w.WriteHeader(http.StatusOK)
	case "failed":
		failureReason, ok := req["failure_reason"].(string)
		if ok && (JobFailureReason(failureReason) == ScriptFailure ||
			JobFailureReason(failureReason) == RunnerSystemFailure) {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
	default:
		w.WriteHeader(http.StatusBadRequest)
	}
}

func testUpdateJobHandler(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.Method != http.MethodPut {
		w.WriteHeader(http.StatusNotAcceptable)
		return
	}

	switch r.URL.Path {
	case "/api/v4/jobs/200":
	case "/api/v4/jobs/202":
		w.WriteHeader(http.StatusAccepted)
		return
	case "/api/v4/jobs/403":
		w.WriteHeader(http.StatusForbidden)
		return
	case "/api/v4/jobs/412":
		w.WriteHeader(http.StatusPreconditionFailed)
		return
	default:
		w.WriteHeader(http.StatusNotFound)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	assert.NoError(t, err)

	var req map[string]interface{}
	err = json.Unmarshal(body, &req)
	assert.NoError(t, err)

	assert.Equal(t, "token", req["token"])

	setStateForUpdateJobHandlerResponse(w, req)
}

func TestUpdateJob(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		testUpdateJobHandler(w, r, t)
	}

	s := httptest.NewServer(http.HandlerFunc(handler))
	defer s.Close()

	config := RunnerConfig{
		RunnerCredentials: RunnerCredentials{
			URL: s.URL,
		},
	}

	jobCredentials := &JobCredentials{
		Token: "token",
	}

	c := NewGitLabClient()

	result := c.UpdateJob(config, jobCredentials, UpdateJobInfo{ID: 200, State: Running})
	assert.Equal(t, UpdateJobResult{State: UpdateSucceeded}, result, "Update should continue when running")

	result = c.UpdateJob(config, jobCredentials, UpdateJobInfo{ID: 403, State: Success})
	assert.Equal(t, UpdateJobResult{State: UpdateAbort}, result, "Update should be aborted if the access is forbidden")

	result = c.UpdateJob(config, jobCredentials, UpdateJobInfo{ID: 200, State: "invalid-state"})
	assert.Equal(t, UpdateJobResult{State: UpdateFailed}, result, "Update should fail for badly formatted request")

	result = c.UpdateJob(config, jobCredentials, UpdateJobInfo{ID: 404, State: Success})
	assert.Equal(t, UpdateJobResult{State: UpdateAbort}, result, "Update should abort for unknown job")

	result = c.UpdateJob(config, jobCredentials, UpdateJobInfo{ID: 202, State: Success})
	assert.Equal(
		t, UpdateJobResult{State: UpdateAcceptedButNotCompleted}, result,
		"Update should return accepted, but not completed if server returns `202 StatusAccepted`",
	)

	result = c.UpdateJob(config, jobCredentials, UpdateJobInfo{ID: 412, State: Success})
	assert.Equal(
		t, UpdateJobResult{State: UpdateTraceValidationFailed}, result,
		"Update should return reset content requested if server returns `412 Precondition Failed`",
	)

	result = c.UpdateJob(
		config,
		jobCredentials,
		UpdateJobInfo{ID: 200, State: Failed, FailureReason: "script_failure"},
	)
	assert.Equal(t, UpdateJobResult{State: UpdateSucceeded}, result, "Update should continue when running")

	result = c.UpdateJob(
		config,
		jobCredentials,
		UpdateJobInfo{ID: 200, State: Failed, FailureReason: "invalid-failure-reason"},
	)
	assert.Equal(t, UpdateJobResult{State: UpdateFailed}, result, "Update should fail for badly formatted request")
}

func testUpdateJobKeepAliveHandler(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.Method != http.MethodPut {
		w.WriteHeader(http.StatusNotAcceptable)
		return
	}

	switch r.URL.Path {
	case "/api/v4/jobs/10":
	case "/api/v4/jobs/11":
		w.Header().Set("Job-Status", "canceled")
	case "/api/v4/jobs/12":
		w.Header().Set("Job-Status", "failed")
	case "/api/v4/jobs/13":
		w.Header().Set("Job-Status", "canceling")
	default:
		w.WriteHeader(http.StatusNotFound)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	assert.NoError(t, err)

	var req map[string]interface{}
	err = json.Unmarshal(body, &req)
	assert.NoError(t, err)

	assert.Equal(t, "token", req["token"])

	w.WriteHeader(http.StatusOK)
}

func TestUpdateJobAsKeepAlive(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		testUpdateJobKeepAliveHandler(w, r, t)
	}

	s := httptest.NewServer(http.HandlerFunc(handler))
	defer s.Close()

	config := RunnerConfig{
		RunnerCredentials: RunnerCredentials{
			URL: s.URL,
		},
	}

	jobCredentials := &JobCredentials{
		Token: "token",
	}

	c := NewGitLabClient()

	result := c.UpdateJob(config, jobCredentials, UpdateJobInfo{ID: 10, State: Running})
	assert.Equal(t, UpdateJobResult{State: UpdateSucceeded}, result, "Update should continue when running")

	result = c.UpdateJob(config, jobCredentials, UpdateJobInfo{ID: 11, State: Running})
	assert.Equal(t, UpdateJobResult{State: UpdateAbort}, result, "Update should be aborted when Job-Status=canceled")

	result = c.UpdateJob(config, jobCredentials, UpdateJobInfo{ID: 12, State: Running})
	assert.Equal(t, UpdateJobResult{State: UpdateAbort}, result, "Update should continue when Job-Status=failed")

	result = c.UpdateJob(config, jobCredentials, UpdateJobInfo{ID: 13, State: Running})
	assert.Equal(
		t,
		UpdateJobResult{State: UpdateSucceeded, CancelRequested: true},
		result,
		"Update should continue when Job-Status=canceling",
	)
}

const patchToken = "token"

var patchTraceContent = []byte("trace trace trace")

func getPatchServer(
	t *testing.T,
	handler func(
		w http.ResponseWriter,
		r *http.Request,
		body []byte,
		offset, limit int),
) (*httptest.Server, *GitLabClient, RunnerConfig) {
	patchHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/jobs/1/trace" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if r.Method != "PATCH" {
			w.WriteHeader(http.StatusNotAcceptable)
			return
		}

		assert.Equal(t, patchToken, r.Header.Get("JOB-TOKEN"))

		body, err := ioutil.ReadAll(r.Body)
		assert.NoError(t, err)

		contentRange := r.Header.Get("Content-Range")
		ranges := strings.Split(contentRange, "-")

		offset, err := strconv.Atoi(ranges[0])
		assert.NoError(t, err)

		limit, err := strconv.Atoi(ranges[1])
		assert.NoError(t, err)

		handler(w, r, body, offset, limit)
	}

	server := httptest.NewServer(http.HandlerFunc(patchHandler))

	config := RunnerConfig{
		RunnerCredentials: RunnerCredentials{
			URL: server.URL,
		},
	}

	return server, NewGitLabClient(), config
}

func TestUnknownPatchTrace(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request, body []byte, offset, limit int) {
		w.WriteHeader(http.StatusNotFound)
	}

	server, client, config := getPatchServer(t, handler)
	defer server.Close()

	result := client.PatchTrace(config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent, 0)
	assert.Equal(t, PatchNotFound, result.State)
}

func TestForbiddenPatchTrace(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request, body []byte, offset, limit int) {
		w.WriteHeader(http.StatusForbidden)
	}

	server, client, config := getPatchServer(t, handler)
	defer server.Close()

	result := client.PatchTrace(config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent, 0)
	assert.Equal(t, PatchAbort, result.State)
}

func TestPatchTrace(t *testing.T) {
	tests := []struct {
		remoteState    string
		expectedResult PatchTraceResult
	}{
		{
			remoteState: "running",
			expectedResult: PatchTraceResult{
				CancelRequested: false,
				State:           PatchSucceeded,
			},
		},
		{
			remoteState: statusCanceling,
			expectedResult: PatchTraceResult{
				CancelRequested: true,
				State:           PatchSucceeded,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.remoteState, func(t *testing.T) {
			handler := func(w http.ResponseWriter, r *http.Request, body []byte, offset, limit int) {
				assert.Equal(t, patchTraceContent[offset:limit+1], body)

				w.Header().Add(remoteStateHeader, tt.remoteState)
				w.WriteHeader(http.StatusAccepted)
			}

			server, client, config := getPatchServer(t, handler)
			defer server.Close()

			result := client.PatchTrace(config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent, 0)
			assert.Equal(t, tt.expectedResult.State, result.State)
			assert.Equal(t, tt.expectedResult.CancelRequested, result.CancelRequested)
			assert.Equal(t, len(patchTraceContent), result.SentOffset)

			result = client.PatchTrace(config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent[3:], 3)
			assert.Equal(t, tt.expectedResult.State, result.State)
			assert.Equal(t, tt.expectedResult.CancelRequested, result.CancelRequested)
			assert.Equal(t, len(patchTraceContent), result.SentOffset)

			result = client.PatchTrace(config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent[3:10], 3)
			assert.Equal(t, tt.expectedResult.State, result.State)
			assert.Equal(t, tt.expectedResult.CancelRequested, result.CancelRequested)
			assert.Equal(t, 10, result.SentOffset)
		})
	}
}

func TestRangeMismatchPatchTrace(t *testing.T) {
	tests := []struct {
		remoteState    string
		expectedResult PatchTraceResult
	}{
		{
			remoteState: "running",
			expectedResult: PatchTraceResult{
				SentOffset:      len(patchTraceContent),
				CancelRequested: false,
				State:           PatchSucceeded,
			},
		},
		{
			remoteState: statusCanceling,
			expectedResult: PatchTraceResult{
				SentOffset:      len(patchTraceContent),
				CancelRequested: true,
				State:           PatchSucceeded,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.remoteState, func(t *testing.T) {
			handler := func(w http.ResponseWriter, r *http.Request, body []byte, offset, limit int) {
				if offset > 10 {
					w.Header().Set("Range", "0-10")
					w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
				}

				w.Header().Add(remoteStateHeader, tt.remoteState)
				w.WriteHeader(http.StatusAccepted)
			}

			server, client, config := getPatchServer(t, handler)
			defer server.Close()

			result := client.PatchTrace(config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent[11:], 11)
			assert.Equal(t, PatchTraceResult{State: PatchRangeMismatch, SentOffset: 10}, result)

			result = client.PatchTrace(config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent[15:], 15)
			assert.Equal(t, PatchTraceResult{State: PatchRangeMismatch, SentOffset: 10}, result)

			result = client.PatchTrace(config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent[5:], 5)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestJobFailedStatePatchTrace(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request, body []byte, offset, limit int) {
		w.Header().Set("Job-Status", "failed")
		w.WriteHeader(http.StatusAccepted)
	}

	server, client, config := getPatchServer(t, handler)
	defer server.Close()

	result := client.PatchTrace(config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent, 0)
	assert.Equal(t, PatchAbort, result.State)
}

func TestPatchTraceCantConnect(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request, body []byte, offset, limit int) {}

	server, client, config := getPatchServer(t, handler)
	server.Close()

	result := client.PatchTrace(config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent, 0)
	assert.Equal(t, PatchFailed, result.State)
}

func TestPatchTraceUpdatedTrace(t *testing.T) {
	sentTrace := 0
	var traceContent []byte

	updates := []struct {
		traceUpdate             []byte
		remoteJobStatus         string
		expectedContentRange    string
		expectedContentLength   int64
		expectedResult          PatchTraceResult
		shouldNotCallPatchTrace bool
	}{
		{
			traceUpdate:           []byte("test"),
			remoteJobStatus:       "running",
			expectedContentRange:  "0-3",
			expectedContentLength: 4,
			expectedResult: PatchTraceResult{
				SentOffset:        4,
				CancelRequested:   false,
				State:             PatchSucceeded,
				NewUpdateInterval: 0,
			},
		},
		{
			traceUpdate:           []byte{},
			remoteJobStatus:       "running",
			expectedContentLength: 4,
			expectedResult: PatchTraceResult{
				SentOffset:        4,
				CancelRequested:   false,
				State:             PatchSucceeded,
				NewUpdateInterval: 0,
			},
			shouldNotCallPatchTrace: true,
		},
		{
			traceUpdate:           []byte(" "),
			remoteJobStatus:       "running",
			expectedContentRange:  "4-4",
			expectedContentLength: 1,
			expectedResult: PatchTraceResult{
				SentOffset:        5,
				CancelRequested:   false,
				State:             PatchSucceeded,
				NewUpdateInterval: 0,
			},
		},
		{
			traceUpdate:           []byte("test"),
			remoteJobStatus:       "running",
			expectedContentRange:  "5-8",
			expectedContentLength: 4,
			expectedResult: PatchTraceResult{
				SentOffset:        9,
				CancelRequested:   false,
				State:             PatchSucceeded,
				NewUpdateInterval: 0,
			},
		},
		{
			traceUpdate:           []byte("test"),
			remoteJobStatus:       statusCanceling,
			expectedContentRange:  "9-12",
			expectedContentLength: 4,
			expectedResult: PatchTraceResult{
				SentOffset:        13,
				CancelRequested:   true,
				State:             PatchSucceeded,
				NewUpdateInterval: 0,
			},
		},
		{
			traceUpdate:           []byte(" "),
			remoteJobStatus:       statusCanceling,
			expectedContentRange:  "13-13",
			expectedContentLength: 1,
			expectedResult: PatchTraceResult{
				SentOffset:        14,
				CancelRequested:   true,
				State:             PatchSucceeded,
				NewUpdateInterval: 0,
			},
		},
		{
			traceUpdate:           []byte(""),
			remoteJobStatus:       statusCanceling,
			expectedContentRange:  "13-13",
			expectedContentLength: 0,
			expectedResult: PatchTraceResult{
				SentOffset:        14,
				CancelRequested:   false, // Empty patches are not sent to remote.
				State:             PatchSucceeded,
				NewUpdateInterval: 0,
			},
		},
	}

	for id, update := range updates {
		t.Run(fmt.Sprintf("patch-%d", id+1), func(t *testing.T) {
			handler := func(w http.ResponseWriter, r *http.Request, body []byte, offset, limit int) {
				if update.shouldNotCallPatchTrace {
					t.Error("PatchTrace endpoint should not be called")
					return
				}

				if limit+1 <= len(traceContent) {
					assert.Equal(t, traceContent[offset:limit+1], body)
				}

				assert.Equal(t, update.traceUpdate, body)
				assert.Equal(t, update.expectedContentRange, r.Header.Get("Content-Range"))
				assert.Equal(t, update.expectedContentLength, r.ContentLength)

				w.Header().Add(remoteStateHeader, update.remoteJobStatus)
				w.WriteHeader(http.StatusAccepted)
			}

			server, client, config := getPatchServer(t, handler)
			defer server.Close()

			traceContent = append(traceContent, update.traceUpdate...)
			result := client.PatchTrace(
				config, &JobCredentials{ID: 1, Token: patchToken},
				traceContent[sentTrace:], sentTrace,
			)
			assert.Equal(t, update.expectedResult, result)

			sentTrace = result.SentOffset
		})
	}
}

func TestPatchTraceContentRangeAndLength(t *testing.T) {
	tests := map[string]struct {
		trace                   []byte
		remoteJobStatus         string
		expectedContentRange    string
		expectedContentLength   int64
		expectedResult          PatchTraceResult
		shouldNotCallPatchTrace bool
	}{
		"0 bytes": {
			trace:           []byte{},
			remoteJobStatus: "running",
			expectedResult: PatchTraceResult{
				SentOffset:        0,
				CancelRequested:   false,
				State:             PatchSucceeded,
				NewUpdateInterval: 0,
			},
			shouldNotCallPatchTrace: true,
		},
		"1 byte": {
			trace:                 []byte("1"),
			remoteJobStatus:       "running",
			expectedContentRange:  "0-0",
			expectedContentLength: 1,
			expectedResult: PatchTraceResult{
				SentOffset:        1,
				CancelRequested:   false,
				State:             PatchSucceeded,
				NewUpdateInterval: 0,
			},
			shouldNotCallPatchTrace: false,
		},
		"2 bytes": {
			trace:                 []byte("12"),
			remoteJobStatus:       "running",
			expectedContentRange:  "0-1",
			expectedContentLength: 2,
			expectedResult: PatchTraceResult{
				SentOffset:        2,
				CancelRequested:   false,
				State:             PatchSucceeded,
				NewUpdateInterval: 0,
			},
			shouldNotCallPatchTrace: false,
		},
		"2 bytes canceling job": {
			trace:                 []byte("12"),
			remoteJobStatus:       statusCanceling,
			expectedContentRange:  "0-1",
			expectedContentLength: 2,
			expectedResult: PatchTraceResult{
				SentOffset:        2,
				CancelRequested:   true,
				State:             PatchSucceeded,
				NewUpdateInterval: 0,
			},
			shouldNotCallPatchTrace: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			handler := func(w http.ResponseWriter, r *http.Request, body []byte, offset, limit int) {
				if test.shouldNotCallPatchTrace {
					t.Error("PatchTrace endpoint should not be called")
					return
				}

				assert.Equal(t, test.expectedContentRange, r.Header.Get("Content-Range"))
				assert.Equal(t, test.expectedContentLength, r.ContentLength)

				w.Header().Add(remoteStateHeader, test.remoteJobStatus)
				w.WriteHeader(http.StatusAccepted)
			}

			server, client, config := getPatchServer(t, handler)
			defer server.Close()

			result := client.PatchTrace(config, &JobCredentials{ID: 1, Token: patchToken}, test.trace, 0)
			assert.Equal(t, test.expectedResult, result)
		})
	}
}

func TestPatchTraceContentRangeHeaderValues(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request, body []byte, offset, limit int) {
		contentRange := r.Header.Get("Content-Range")
		bytes := strings.Split(contentRange, "-")

		startByte, err := strconv.Atoi(bytes[0])
		require.NoError(t, err, "Should not set error when parsing Content-Range startByte component")

		endByte, err := strconv.Atoi(bytes[1])
		require.NoError(t, err, "Should not set error when parsing Content-Range endByte component")

		assert.Equal(t, 0, startByte, "Content-Range should contain start byte as first field")
		assert.Equal(t, len(patchTraceContent)-1, endByte, "Content-Range should contain end byte as second field")

		w.WriteHeader(http.StatusAccepted)
	}

	server, client, config := getPatchServer(t, handler)
	defer server.Close()

	client.PatchTrace(config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent, 0)
}

func TestUpdateIntervalHeaderHandling(t *testing.T) {
	tests := map[string]struct {
		sendUpdateIntervalHeader  bool
		updateIntervalHeaderValue string
		expectedUpdateInterval    time.Duration
	}{
		"header set to negative integer": {
			sendUpdateIntervalHeader:  true,
			updateIntervalHeaderValue: "-10",
			expectedUpdateInterval:    -10 * time.Second,
		},
		"header set to zero": {
			sendUpdateIntervalHeader:  true,
			updateIntervalHeaderValue: "0",
			expectedUpdateInterval:    time.Duration(0),
		},
		"header set to positive integer": {
			sendUpdateIntervalHeader:  true,
			updateIntervalHeaderValue: "10",
			expectedUpdateInterval:    10 * time.Second,
		},
		"header set to invalid format": {
			sendUpdateIntervalHeader:  true,
			updateIntervalHeaderValue: "some text",
			expectedUpdateInterval:    time.Duration(0),
		},
		"empty header": {
			sendUpdateIntervalHeader:  true,
			updateIntervalHeaderValue: "",
			expectedUpdateInterval:    time.Duration(0),
		},
		"header not set": {
			sendUpdateIntervalHeader: false,
			expectedUpdateInterval:   time.Duration(0),
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			t.Run("UpdateJob", func(t *testing.T) {
				handler := func(w http.ResponseWriter, r *http.Request) {
					if tt.sendUpdateIntervalHeader {
						w.Header().Add(updateIntervalHeader, tt.updateIntervalHeaderValue)
					}

					testUpdateJobHandler(w, r, t)
				}

				server := httptest.NewServer(http.HandlerFunc(handler))
				defer server.Close()

				config := RunnerConfig{
					RunnerCredentials: RunnerCredentials{URL: server.URL},
				}

				result := NewGitLabClient().UpdateJob(config, &JobCredentials{ID: 10}, UpdateJobInfo{State: "success"})
				assert.Equal(t, tt.expectedUpdateInterval, result.NewUpdateInterval)
			})

			t.Run("PatchTrace", func(t *testing.T) {
				handler := func(w http.ResponseWriter, r *http.Request, body []byte, offset, limit int) {
					if tt.sendUpdateIntervalHeader {
						w.Header().Add(updateIntervalHeader, tt.updateIntervalHeaderValue)
					}

					w.WriteHeader(http.StatusAccepted)
				}

				server, client, config := getPatchServer(t, handler)
				defer server.Close()

				result := client.PatchTrace(config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent, 0)
				assert.Equal(t, tt.expectedUpdateInterval, result.NewUpdateInterval)
			})
		})
	}
}

func TestAbortedPatchTrace(t *testing.T) {
	tests := map[string]PatchTraceResult{
		"canceling": {SentOffset: 17, CancelRequested: true, State: PatchSucceeded},
		"canceled":  {State: PatchAbort},
		"failed":    {State: PatchAbort},
	}

	for status, expectedResult := range tests {
		t.Run(status, func(t *testing.T) {
			handler := func(w http.ResponseWriter, r *http.Request, body []byte, offset, limit int) {
				w.Header().Set("Job-Status", status)
				w.WriteHeader(http.StatusAccepted)
			}

			server, client, config := getPatchServer(t, handler)
			defer server.Close()

			result := client.PatchTrace(config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent, 0)
			assert.Equal(t, expectedResult, result)
		})
	}
}

func checkTestArtifactsUploadHandlerContent(w http.ResponseWriter, r *http.Request, body string) {
	cases := map[string]struct {
		formValueKey string
		statusCode   int
	}{
		"too-large": {
			statusCode: http.StatusRequestEntityTooLarge,
		},
		"content": {
			statusCode: http.StatusCreated,
		},
		"zip": {
			statusCode:   http.StatusCreated,
			formValueKey: "artifact_format",
		},
		"gzip": {
			statusCode:   http.StatusCreated,
			formValueKey: "artifact_format",
		},
		"junit": {
			statusCode:   http.StatusCreated,
			formValueKey: "artifact_type",
		},
		"service-unavailable": {
			statusCode: http.StatusServiceUnavailable,
		},
	}

	testCase, ok := cases[body]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if len(testCase.formValueKey) > 0 {
		if r.FormValue(testCase.formValueKey) != body {
			return
		}
	}

	w.WriteHeader(testCase.statusCode)
}

func testArtifactsUploadHandler(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.URL.Path != "/api/v4/jobs/10/artifacts" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotAcceptable)
		return
	}

	if r.Header.Get("JOB-TOKEN") != "token" {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	body, err := ioutil.ReadAll(file)
	require.NoError(t, err)

	checkTestArtifactsUploadHandlerContent(w, r, string(body))
}

func uploadArtifacts(
	client *GitLabClient,
	config JobCredentials,
	artifactsFile,
	artifactType string,
	artifactFormat ArtifactFormat,
) UploadState {
	file, err := os.Open(artifactsFile)
	if err != nil {
		return UploadFailed
	}
	defer file.Close()

	fi, err := file.Stat()
	if err != nil {
		return UploadFailed
	}
	if fi.IsDir() {
		return UploadFailed
	}

	options := ArtifactsOptions{
		BaseName: filepath.Base(artifactsFile),
		Format:   artifactFormat,
		Type:     artifactType,
	}
	return client.UploadRawArtifacts(config, file, options)
}
func TestArtifactsUpload(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		testArtifactsUploadHandler(w, r, t)
	}

	s := httptest.NewServer(http.HandlerFunc(handler))
	defer s.Close()

	config := JobCredentials{
		ID:    10,
		URL:   s.URL,
		Token: "token",
	}
	invalidToken := JobCredentials{
		ID:    10,
		URL:   s.URL,
		Token: "invalid-token",
	}

	tempFile, err := ioutil.TempFile("", "artifacts")
	assert.NoError(t, err)
	tempFile.Close()
	defer os.Remove(tempFile.Name())

	c := NewGitLabClient()

	require.NoError(t, ioutil.WriteFile(tempFile.Name(), []byte("content"), 0600))
	state := uploadArtifacts(c, config, tempFile.Name(), "", ArtifactFormatDefault)
	assert.Equal(t, UploadSucceeded, state, "Artifacts should be uploaded")

	require.NoError(t, ioutil.WriteFile(tempFile.Name(), []byte("too-large"), 0600))
	state = uploadArtifacts(c, config, tempFile.Name(), "", ArtifactFormatDefault)
	assert.Equal(t, UploadTooLarge, state, "Artifacts should be not uploaded, because of too large archive")

	require.NoError(t, ioutil.WriteFile(tempFile.Name(), []byte("zip"), 0600))
	state = uploadArtifacts(c, config, tempFile.Name(), "", ArtifactFormatZip)
	assert.Equal(t, UploadSucceeded, state, "Artifacts should be uploaded, as zip")

	require.NoError(t, ioutil.WriteFile(tempFile.Name(), []byte("gzip"), 0600))
	state = uploadArtifacts(c, config, tempFile.Name(), "", ArtifactFormatGzip)
	assert.Equal(t, UploadSucceeded, state, "Artifacts should be uploaded, as gzip")

	require.NoError(t, ioutil.WriteFile(tempFile.Name(), []byte("junit"), 0600))
	state = uploadArtifacts(c, config, tempFile.Name(), "junit", ArtifactFormatGzip)
	assert.Equal(t, UploadSucceeded, state, "Artifacts should be uploaded, as gzip")

	state = uploadArtifacts(c, config, "not/existing/file", "", ArtifactFormatDefault)
	assert.Equal(t, UploadFailed, state, "Artifacts should fail to be uploaded")

	state = uploadArtifacts(c, invalidToken, tempFile.Name(), "", ArtifactFormatDefault)
	assert.Equal(t, UploadForbidden, state, "Artifacts should be rejected if invalid token")

	require.NoError(t, ioutil.WriteFile(tempFile.Name(), []byte("service-unavailable"), 0600))
	state = uploadArtifacts(c, config, tempFile.Name(), "", ArtifactFormatDefault)
	assert.Equal(t, UploadServiceUnavailable, state, "Artifacts should get service unavailable")
}

func testArtifactsDownloadHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/direct-download" {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bytes.NewBufferString("Artifact: direct_download=true").Bytes())
		return
	}

	if r.URL.Path != "/api/v4/jobs/10/artifacts" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusNotAcceptable)
		return
	}

	if r.Header.Get("JOB-TOKEN") != "token" {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	// parse status of direct download
	directDownloadFlag := r.URL.Query().Get("direct_download")
	if directDownloadFlag == "" {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bytes.NewBufferString("Artifact: direct_download=missing").Bytes())
		return
	}

	directDownload, err := strconv.ParseBool(directDownloadFlag)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if directDownload {
		w.Header().Set("Location", "/direct-download")
		w.WriteHeader(http.StatusTemporaryRedirect)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(bytes.NewBufferString("Artifact: direct_download=false").Bytes())
}

func TestArtifactsDownload(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		testArtifactsDownloadHandler(w, r)
	}

	s := httptest.NewServer(http.HandlerFunc(handler))
	defer s.Close()

	validCredentials := JobCredentials{
		ID:    10,
		URL:   s.URL,
		Token: "token",
	}
	invalidTokenCredentials := JobCredentials{
		ID:    10,
		URL:   s.URL,
		Token: "invalid-token",
	}
	fileNotFoundTokenCredentials := JobCredentials{
		ID:    11,
		URL:   s.URL,
		Token: "token",
	}

	trueValue := true
	falseValue := false

	testCases := map[string]struct {
		credentials      JobCredentials
		directDownload   *bool
		expectedState    DownloadState
		expectedArtifact string
	}{
		"successful download": {
			credentials:      validCredentials,
			expectedState:    DownloadSucceeded,
			expectedArtifact: "Artifact: direct_download=missing",
		},
		"properly handles direct_download=false": {
			credentials:      validCredentials,
			directDownload:   &falseValue,
			expectedState:    DownloadSucceeded,
			expectedArtifact: "Artifact: direct_download=false",
		},
		"properly handles direct_download=true": {
			credentials:      validCredentials,
			directDownload:   &trueValue,
			expectedState:    DownloadSucceeded,
			expectedArtifact: "Artifact: direct_download=true",
		},
		"forbidden should be generated for invalid credentials": {
			credentials:    invalidTokenCredentials,
			directDownload: &trueValue,
			expectedState:  DownloadForbidden,
		},
		"file should not be downloaded if not existing": {
			credentials:    fileNotFoundTokenCredentials,
			directDownload: &trueValue,
			expectedState:  DownloadNotFound,
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			c := NewGitLabClient()

			tempDir, err := ioutil.TempDir("", "artifacts")
			require.NoError(t, err)
			defer os.Remove(tempDir)

			artifactsFileName := filepath.Join(tempDir, "downloaded-artifact")

			state := c.DownloadArtifacts(testCase.credentials, artifactsFileName, testCase.directDownload)
			require.Equal(t, testCase.expectedState, state)

			artifact, err := ioutil.ReadFile(artifactsFileName)

			if testCase.expectedArtifact == "" {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, string(artifact), testCase.expectedArtifact)
		})
	}
}

func TestRunnerVersion(t *testing.T) {
	c := NewGitLabClient()
	info := c.getRunnerVersion(RunnerConfig{
		RunnerSettings: RunnerSettings{
			Executor: "my-executor",
			Shell:    "my-shell",
		},
	})

	assert.NotEmpty(t, info.Name)
	assert.NotEmpty(t, info.Version)
	assert.NotEmpty(t, info.Revision)
	assert.NotEmpty(t, info.Platform)
	assert.NotEmpty(t, info.Architecture)
	assert.Equal(t, "my-executor", info.Executor)
	assert.Equal(t, "my-shell", info.Shell)
}

func TestRunnerVersionToGetExecutorAndShellFeaturesWithTheDefaultShell(t *testing.T) {
	executorProvider := MockExecutorProvider{}
	defer executorProvider.AssertExpectations(t)
	executorProvider.On("GetDefaultShell").Return("my-default-executor-shell").Twice()
	executorProvider.On("CanCreate").Return(true).Once()
	executorProvider.On("GetFeatures", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		features := args[0].(*FeaturesInfo)
		features.Shared = true
	})
	RegisterExecutorProvider("my-test-executor", &executorProvider)

	shell := MockShell{}
	defer shell.AssertExpectations(t)
	shell.On("GetName").Return("my-default-executor-shell")
	shell.On("GetFeatures", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		features := args[0].(*FeaturesInfo)
		features.Variables = true
	})
	RegisterShell(&shell)

	c := NewGitLabClient()
	info := c.getRunnerVersion(RunnerConfig{
		RunnerSettings: RunnerSettings{
			Executor: "my-test-executor",
			Shell:    "",
		},
	})

	assert.Equal(t, "my-test-executor", info.Executor)
	assert.Equal(t, "my-default-executor-shell", info.Shell)
	assert.False(t, info.Features.Artifacts, "dry-run that this is not enabled")
	assert.True(t, info.Features.Shared, "feature is enabled by executor")
	assert.True(t, info.Features.Variables, "feature is enabled by shell")
}
