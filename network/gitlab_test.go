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

	if r.Method != "POST" {
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
	case "valid":
		if req["description"].(string) != "test" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		res["token"] = req["token"].(string)
	case "invalid":
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
	w.Write(output)
}

func TestRegisterRunner(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testRegisterRunnerHandler(w, r, t)
	}))
	defer s.Close()

	validToken := RunnerCredentials{
		URL:   s.URL,
		Token: "valid",
	}

	invalidToken := RunnerCredentials{
		URL:   s.URL,
		Token: "invalid",
	}

	otherToken := RunnerCredentials{
		URL:   s.URL,
		Token: "other",
	}

	c := NewGitLabClient()

	res := c.RegisterRunner(validToken, RegisterRunnerParameters{Description: "test", Tags: "tags", RunUntagged: true, Locked: true, Active: true})
	if assert.NotNil(t, res) {
		assert.Equal(t, validToken.Token, res.Token)
	}

	res = c.RegisterRunner(validToken, RegisterRunnerParameters{Description: "invalid description", Tags: "tags", RunUntagged: true, Locked: true, AccessLevel: "not_protected", Active: true})
	assert.Nil(t, res)

	res = c.RegisterRunner(invalidToken, RegisterRunnerParameters{Description: "test", Tags: "tags", RunUntagged: true, Locked: true, AccessLevel: "not_protected", Active: true})
	assert.Nil(t, res)

	res = c.RegisterRunner(otherToken, RegisterRunnerParameters{Description: "test", Tags: "tags", RunUntagged: true, Locked: true, AccessLevel: "not_protected", Active: true})
	assert.Nil(t, res)

	res = c.RegisterRunner(brokenCredentials, RegisterRunnerParameters{Description: "test", Tags: "tags", RunUntagged: true, Locked: true, AccessLevel: "not_protected", Active: true})
	assert.Nil(t, res)
}

func testUnregisterRunnerHandler(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.URL.Path != "/api/v4/runners" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if r.Method != "DELETE" {
		w.WriteHeader(http.StatusNotAcceptable)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	assert.NoError(t, err)

	var req map[string]interface{}
	err = json.Unmarshal(body, &req)
	assert.NoError(t, err)

	switch req["token"].(string) {
	case "valid":
		w.WriteHeader(http.StatusNoContent)
	case "invalid":
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
		Token: "valid",
	}

	invalidToken := RunnerCredentials{
		URL:   s.URL,
		Token: "invalid",
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

	if r.Method != "POST" {
		w.WriteHeader(http.StatusNotAcceptable)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	assert.NoError(t, err)

	var req map[string]interface{}
	err = json.Unmarshal(body, &req)
	assert.NoError(t, err)

	switch req["token"].(string) {
	case "valid":
		w.WriteHeader(http.StatusOK) // since the job id is broken, we should not find this job
	case "invalid":
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
		Token: "valid",
	}

	invalidToken := RunnerCredentials{
		URL:   s.URL,
		Token: "invalid",
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

func getRequestJobResponse() (res map[string]interface{}) {
	jobToken := "job-token"

	res = make(map[string]interface{})
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

	return
}

func testRequestJobHandler(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.URL.Path != "/api/v4/jobs/request" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if r.Method != "POST" {
		w.WriteHeader(http.StatusNotAcceptable)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	assert.NoError(t, err)

	var req map[string]interface{}
	err = json.Unmarshal(body, &req)
	assert.NoError(t, err)

	switch req["token"].(string) {
	case "valid":
	case "no-jobs":
		w.Header().Add("X-GitLab-Last-Update", "a nice timestamp")
		w.WriteHeader(http.StatusNoContent)
		return
	case "invalid":
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
	w.Write(output)
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
			Token: "valid",
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
			Token: "invalid",
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
	case "running":
		w.WriteHeader(http.StatusOK)
	case "failed":
		failureReason, ok := req["failure_reason"].(string)
		if ok && (JobFailureReason(failureReason) == ScriptFailure ||
			JobFailureReason(failureReason) == RunnerSystemFailure) {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
	case "forbidden":
		w.WriteHeader(http.StatusForbidden)
	default:
		w.WriteHeader(http.StatusBadRequest)
	}
}

func testUpdateJobHandler(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.URL.Path != "/api/v4/jobs/10" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if r.Method != "PUT" {
		w.WriteHeader(http.StatusNotAcceptable)
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

	var state UpdateState

	state = c.UpdateJob(config, jobCredentials, UpdateJobInfo{ID: 10, State: "running", FailureReason: ""})
	assert.Equal(t, UpdateSucceeded, state, "Update should continue when running")

	state = c.UpdateJob(config, jobCredentials, UpdateJobInfo{ID: 10, State: "forbidden", FailureReason: ""})
	assert.Equal(t, UpdateAbort, state, "Update should be aborted if the state is forbidden")

	state = c.UpdateJob(config, jobCredentials, UpdateJobInfo{ID: 10, State: "other", FailureReason: ""})
	assert.Equal(t, UpdateFailed, state, "Update should fail for badly formatted request")

	state = c.UpdateJob(config, jobCredentials, UpdateJobInfo{ID: 4, State: "state", FailureReason: ""})
	assert.Equal(t, UpdateAbort, state, "Update should abort for unknown job")

	state = c.UpdateJob(brokenConfig, jobCredentials, UpdateJobInfo{ID: 4, State: "state", FailureReason: ""})
	assert.Equal(t, UpdateAbort, state)

	state = c.UpdateJob(config, jobCredentials, UpdateJobInfo{ID: 10, State: "failed", FailureReason: "script_failure"})
	assert.Equal(t, UpdateSucceeded, state, "Update should continue when running")

	state = c.UpdateJob(config, jobCredentials, UpdateJobInfo{ID: 10, State: "failed", FailureReason: "unknown_failure_reason"})
	assert.Equal(t, UpdateFailed, state, "Update should fail for badly formatted request")

	state = c.UpdateJob(config, jobCredentials, UpdateJobInfo{ID: 10, State: "failed", FailureReason: ""})
	assert.Equal(t, UpdateFailed, state, "Update should fail for badly formatted request")
}

func testUpdateJobKeepAliveHandler(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.Method != "PUT" {
		w.WriteHeader(http.StatusNotAcceptable)
		return
	}

	switch r.URL.Path {
	case "/api/v4/jobs/10":
	case "/api/v4/jobs/11":
		w.Header().Set("Job-Status", "canceled")
	case "/api/v4/jobs/12":
		w.Header().Set("Job-Status", "failed")
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

	var state UpdateState

	state = c.UpdateJob(config, jobCredentials, UpdateJobInfo{ID: 10, State: "running"})
	assert.Equal(t, UpdateSucceeded, state, "Update should continue when running")

	state = c.UpdateJob(config, jobCredentials, UpdateJobInfo{ID: 11, State: "running"})
	assert.Equal(t, UpdateAbort, state, "Update should be aborted when Job-Status=canceled")

	state = c.UpdateJob(config, jobCredentials, UpdateJobInfo{ID: 12, State: "running"})
	assert.Equal(t, UpdateAbort, state, "Update should continue when Job-Status=failed")
}

var patchToken = "token"
var patchTraceContent = []byte("trace trace trace")

func getPatchServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request, body []byte, offset, limit int)) (*httptest.Server, *GitLabClient, RunnerConfig) {
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
	assert.Equal(t, UpdateNotFound, result.State)
}

func TestForbiddenPatchTrace(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request, body []byte, offset, limit int) {
		w.WriteHeader(http.StatusForbidden)
	}

	server, client, config := getPatchServer(t, handler)
	defer server.Close()

	result := client.PatchTrace(config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent, 0)
	assert.Equal(t, UpdateAbort, result.State)
}

func TestPatchTrace(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request, body []byte, offset, limit int) {
		assert.Equal(t, patchTraceContent[offset:limit+1], body)
		w.WriteHeader(http.StatusAccepted)
	}

	server, client, config := getPatchServer(t, handler)
	defer server.Close()

	result := client.PatchTrace(config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent, 0)
	assert.Equal(t, UpdateSucceeded, result.State)
	assert.Equal(t, len(patchTraceContent), result.SentOffset)

	result = client.PatchTrace(config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent[3:], 3)
	assert.Equal(t, UpdateSucceeded, result.State)
	assert.Equal(t, len(patchTraceContent), result.SentOffset)

	result = client.PatchTrace(config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent[3:10], 3)
	assert.Equal(t, UpdateSucceeded, result.State)
	assert.Equal(t, 10, result.SentOffset)
}

func TestRangeMismatchPatchTrace(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request, body []byte, offset, limit int) {
		if offset > 10 {
			w.Header().Set("Range", "0-10")
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		}

		w.WriteHeader(http.StatusAccepted)
	}

	server, client, config := getPatchServer(t, handler)
	defer server.Close()

	result := client.PatchTrace(config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent[11:], 11)
	assert.Equal(t, UpdateRangeMismatch, result.State)
	assert.Equal(t, 10, result.SentOffset)

	result = client.PatchTrace(config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent[15:], 15)
	assert.Equal(t, UpdateRangeMismatch, result.State)
	assert.Equal(t, 10, result.SentOffset)

	result = client.PatchTrace(config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent[5:], 5)
	assert.Equal(t, UpdateSucceeded, result.State)
	assert.Equal(t, len(patchTraceContent), result.SentOffset)
}

func TestJobFailedStatePatchTrace(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request, body []byte, offset, limit int) {
		w.Header().Set("Job-Status", "failed")
		w.WriteHeader(http.StatusAccepted)
	}

	server, client, config := getPatchServer(t, handler)
	defer server.Close()

	result := client.PatchTrace(config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent, 0)
	assert.Equal(t, UpdateAbort, result.State)
}

func TestPatchTraceCantConnect(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request, body []byte, offset, limit int) {}

	server, client, config := getPatchServer(t, handler)
	server.Close()

	result := client.PatchTrace(config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent, 0)
	assert.Equal(t, UpdateFailed, result.State)
}

func TestPatchTraceUpdatedTrace(t *testing.T) {
	sentTrace := 0
	traceContent := []byte{}

	updates := []struct {
		traceUpdate             []byte
		expectedContentRange    string
		expectedContentLength   int64
		expectedResult          UpdateState
		shouldNotCallPatchTrace bool
	}{
		{
			traceUpdate:           []byte("test"),
			expectedContentRange:  "0-3",
			expectedContentLength: 4,
			expectedResult:        UpdateSucceeded,
		},
		{
			traceUpdate:             []byte{},
			expectedContentLength:   4,
			expectedResult:          UpdateSucceeded,
			shouldNotCallPatchTrace: true,
		},
		{
			traceUpdate:          []byte(" "),
			expectedContentRange: "4-4", expectedContentLength: 1,
			expectedResult: UpdateSucceeded,
		},
		{
			traceUpdate:          []byte("test"),
			expectedContentRange: "5-8", expectedContentLength: 4,
			expectedResult: UpdateSucceeded,
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
				w.WriteHeader(http.StatusAccepted)
			}

			server, client, config := getPatchServer(t, handler)
			defer server.Close()

			traceContent = append(traceContent, update.traceUpdate...)
			result := client.PatchTrace(config, &JobCredentials{ID: 1, Token: patchToken},
				traceContent[sentTrace:], sentTrace)
			assert.Equal(t, update.expectedResult, result.State)

			sentTrace = result.SentOffset
		})
	}
}

func TestPatchTraceContentRangeAndLength(t *testing.T) {
	tests := map[string]struct {
		name                    string
		trace                   []byte
		expectedContentRange    string
		expectedContentLength   int64
		expectedResult          UpdateState
		shouldNotCallPatchTrace bool
	}{
		"0 bytes": {
			trace:                   []byte{},
			expectedResult:          UpdateSucceeded,
			shouldNotCallPatchTrace: true,
		},
		"1 byte": {
			name:                    "1 byte",
			trace:                   []byte("1"),
			expectedContentRange:    "0-0",
			expectedContentLength:   1,
			expectedResult:          UpdateSucceeded,
			shouldNotCallPatchTrace: false,
		},
		"2 bytes": {
			trace:                   []byte("12"),
			expectedContentRange:    "0-1",
			expectedContentLength:   2,
			expectedResult:          UpdateSucceeded,
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
				w.WriteHeader(http.StatusAccepted)
			}

			server, client, config := getPatchServer(t, handler)
			defer server.Close()

			result := client.PatchTrace(config, &JobCredentials{ID: 1, Token: patchToken}, test.trace, 0)
			assert.Equal(t, test.expectedResult, result.State)
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

	client.PatchTrace(config, &JobCredentials{ID: 1, Token: patchToken},
		patchTraceContent, 0)
}

func TestPatchTraceUpdateIntervalHeaderHandling(t *testing.T) {
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
			handler := func(w http.ResponseWriter, r *http.Request, body []byte, offset, limit int) {
				if tt.sendUpdateIntervalHeader {
					w.Header().Add(traceUpdateIntervalHeader, tt.updateIntervalHeaderValue)
				}

				w.WriteHeader(http.StatusAccepted)
			}

			server, client, config := getPatchServer(t, handler)
			defer server.Close()

			result := client.PatchTrace(config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent, 0)
			assert.Equal(t, tt.expectedUpdateInterval, result.NewUpdateInterval)
		})
	}
}

func TestAbortedPatchTrace(t *testing.T) {
	statuses := []string{"canceled", "failed"}

	for _, status := range statuses {
		t.Run(status, func(t *testing.T) {
			handler := func(w http.ResponseWriter, r *http.Request, body []byte, offset, limit int) {
				w.Header().Set("Job-Status", status)
				w.WriteHeader(http.StatusAccepted)
			}

			server, client, config := getPatchServer(t, handler)
			defer server.Close()

			result := client.PatchTrace(config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent, 0)
			assert.Equal(t, UpdateAbort, result.State)
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

	if r.Method != "POST" {
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

func uploadArtifacts(client *GitLabClient, config JobCredentials, artifactsFile, artifactType string, artifactFormat ArtifactFormat) UploadState {
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

	ioutil.WriteFile(tempFile.Name(), []byte("content"), 0600)
	state := uploadArtifacts(c, config, tempFile.Name(), "", ArtifactFormatDefault)
	assert.Equal(t, UploadSucceeded, state, "Artifacts should be uploaded")

	ioutil.WriteFile(tempFile.Name(), []byte("too-large"), 0600)
	state = uploadArtifacts(c, config, tempFile.Name(), "", ArtifactFormatDefault)
	assert.Equal(t, UploadTooLarge, state, "Artifacts should be not uploaded, because of too large archive")

	ioutil.WriteFile(tempFile.Name(), []byte("zip"), 0600)
	state = uploadArtifacts(c, config, tempFile.Name(), "", ArtifactFormatZip)
	assert.Equal(t, UploadSucceeded, state, "Artifacts should be uploaded, as zip")

	ioutil.WriteFile(tempFile.Name(), []byte("gzip"), 0600)
	state = uploadArtifacts(c, config, tempFile.Name(), "", ArtifactFormatGzip)
	assert.Equal(t, UploadSucceeded, state, "Artifacts should be uploaded, as gzip")

	ioutil.WriteFile(tempFile.Name(), []byte("junit"), 0600)
	state = uploadArtifacts(c, config, tempFile.Name(), "junit", ArtifactFormatGzip)
	assert.Equal(t, UploadSucceeded, state, "Artifacts should be uploaded, as gzip")

	state = uploadArtifacts(c, config, "not/existing/file", "", ArtifactFormatDefault)
	assert.Equal(t, UploadFailed, state, "Artifacts should fail to be uploaded")

	state = uploadArtifacts(c, invalidToken, tempFile.Name(), "", ArtifactFormatDefault)
	assert.Equal(t, UploadForbidden, state, "Artifacts should be rejected if invalid token")

	ioutil.WriteFile(tempFile.Name(), []byte("service-unavailable"), 0600)
	state = uploadArtifacts(c, config, tempFile.Name(), "", ArtifactFormatDefault)
	assert.Equal(t, UploadServiceUnavailable, state, "Artifacts should get service unavailable")
}

func testArtifactsDownloadHandler(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.URL.Path != "/api/v4/jobs/10/artifacts" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if r.Method != "GET" {
		w.WriteHeader(http.StatusNotAcceptable)
		return
	}

	if r.Header.Get("JOB-TOKEN") != "token" {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(bytes.NewBufferString("Test artifact file content").Bytes())
}

func TestArtifactsDownload(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		testArtifactsDownloadHandler(w, r, t)
	}

	s := httptest.NewServer(http.HandlerFunc(handler))
	defer s.Close()

	credentials := JobCredentials{
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

	c := NewGitLabClient()

	tempDir, err := ioutil.TempDir("", "artifacts")
	assert.NoError(t, err)

	artifactsFileName := filepath.Join(tempDir, "downloaded-artifact")
	defer os.Remove(artifactsFileName)

	state := c.DownloadArtifacts(credentials, artifactsFileName)
	assert.Equal(t, DownloadSucceeded, state, "Artifacts should be downloaded")

	state = c.DownloadArtifacts(invalidTokenCredentials, artifactsFileName)
	assert.Equal(t, DownloadForbidden, state, "Artifacts should be not downloaded if invalid token is used")

	state = c.DownloadArtifacts(fileNotFoundTokenCredentials, artifactsFileName)
	assert.Equal(t, DownloadNotFound, state, "Artifacts should be bit downloaded if it's not found")
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
