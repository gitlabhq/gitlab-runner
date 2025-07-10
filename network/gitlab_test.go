//go:build !integration

package network

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	. "gitlab.com/gitlab-org/gitlab-runner/common"
)

const (
	expiringToken  = "expiring"
	invalidToken   = "invalid"
	testSystemID   = "test-system-id"
	validGlrtToken = "glrt-valid-token"
	validToken     = "valid"
)

type registerRunnerResponse int

const (
	registerRunnerResponseOK = iota
	registerRunnerResponseRunnerNamespacesLimitHit
	registerRunnerResponseRunnerProjectsLimitHit
)

var brokenCredentials = RunnerCredentials{
	URL: "broken",
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

func testRegisterRunnerHandler(w http.ResponseWriter, r *http.Request, response registerRunnerResponse, t *testing.T) {
	w.Header().Add(correlationIDHeader, "foobar")

	if r.URL.Path != "/api/v4/runners" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotAcceptable)
		return
	}

	body, err := io.ReadAll(r.Body)
	assert.NoError(t, err)

	var req map[string]interface{}
	err = json.Unmarshal(body, &req)
	assert.NoError(t, err)

	res := make(map[string]interface{})

	token := req["token"].(string)
	require.NotEmpty(t, r.Header.Get(RunnerToken), "runner-token header is required")
	require.Equal(t, token, r.Header.Get("runner-token"), "token in header and body must match")

	switch token {
	case validToken:
		if req["description"].(string) != "test" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Set(ContentType, "application/json")

		mapResponseToBody := map[registerRunnerResponse]string{
			registerRunnerResponseRunnerNamespacesLimitHit: `{"message":{"runner_namespaces.base":["Maximum number of ci registered group runners (3) exceeded"]}}`,
			registerRunnerResponseRunnerProjectsLimitHit:   `{"message":{"runner_projects.base":["Maximum number of ci registered project runners (3) exceeded"]}}`,
		}
		if badRequestBody := mapResponseToBody[response]; badRequestBody != "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(badRequestBody))
			return
		}

		w.WriteHeader(http.StatusCreated)
		res["id"] = 12345
		res["token"] = req["token"].(string)
		res["token_expires_at"] = nil
	case expiringToken:
		w.Header().Set(ContentType, "application/json")
		w.WriteHeader(http.StatusCreated)
		res["id"] = 54321
		res["token"] = req["token"].(string)
		res["token_expires_at"] = "2684-10-16T13:25:59Z"
	case invalidToken:
		w.WriteHeader(http.StatusForbidden)
		return
	default:
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if r.Header.Get(Accept) != "application/json" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	output, err := json.Marshal(res)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set(ContentType, "application/json")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write(output)
}

func TestRegisterRunner(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testRegisterRunnerHandler(w, r, registerRunnerResponseOK, t)
	}))
	defer s.Close()

	validToken := RunnerCredentials{
		ID:    12345,
		URL:   s.URL,
		Token: validToken,
	}

	expiringToken := RunnerCredentials{
		ID:             54321,
		URL:            s.URL,
		Token:          expiringToken,
		TokenExpiresAt: time.Date(2684, 10, 16, 13, 25, 59, 0, time.UTC),
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
			Paused:      false,
		})
	if assert.NotNil(t, res) {
		assert.Equal(t, validToken.ID, res.ID)
		assert.Equal(t, validToken.Token, res.Token)
		assert.True(t, res.TokenExpiresAt.IsZero())
	}

	res = c.RegisterRunner(
		expiringToken,
		RegisterRunnerParameters{
			Description: "test",
			Tags:        "tags",
			RunUntagged: true,
			Locked:      true,
			Paused:      false,
		})
	if assert.NotNil(t, res) {
		assert.Equal(t, expiringToken.ID, res.ID)
		assert.Equal(t, expiringToken.Token, res.Token)
		assert.Equal(t, expiringToken.TokenExpiresAt, res.TokenExpiresAt)
	}

	res = c.RegisterRunner(
		validToken,
		RegisterRunnerParameters{
			Description: "invalid description",
			Tags:        "tags",
			RunUntagged: true,
			Locked:      true,
			AccessLevel: "not_protected",
			Paused:      false,
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
			Paused:      false,
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
			Paused:      false,
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
			Paused:      false,
		})
	assert.Nil(t, res)
}

func TestRegisterRunnerOnRunnerLimitHit(t *testing.T) {
	type testCase struct {
		response registerRunnerResponse

		expectedMessage string
	}

	testCases := map[string]testCase{
		"namespace runner limit hit": {
			response:        registerRunnerResponseRunnerNamespacesLimitHit,
			expectedMessage: "400 Bad Request (runner_namespaces.base: Maximum number of ci registered group runners (3) exceeded)",
		},
		"project runner limit hit": {
			response:        registerRunnerResponseRunnerProjectsLimitHit,
			expectedMessage: "400 Bad Request (runner_projects.base: Maximum number of ci registered project runners (3) exceeded)",
		},
	}

	c := NewGitLabClient()

	for tn, tc := range testCases {
		t.Run(tn, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				testRegisterRunnerHandler(w, r, tc.response, t)
			}))
			defer s.Close()

			validToken := RunnerCredentials{
				URL:   s.URL,
				Token: validToken,
			}

			h := newLogHook(logrus.ErrorLevel)
			logrus.AddHook(&h)

			res := c.RegisterRunner(
				validToken,
				RegisterRunnerParameters{
					Description: "test",
					Tags:        "tags",
					RunUntagged: true,
					Locked:      true,
					Paused:      false,
				})
			assert.Nil(t, res)
			require.Len(t, h.entries, 1)
			assert.Equal(t, "Registering runner... failed", h.entries[0].Message)
			assert.Equal(t, "foobar", h.entries[0].Data["correlation_id"])
			assert.Contains(t, h.entries[0].Data["status"], tc.expectedMessage)
		})
	}
}

func newLogHook(levels ...logrus.Level) logHook {
	return logHook{levels: levels}
}

type logHook struct {
	entries []*logrus.Entry
	levels  []logrus.Level
}

func (s *logHook) Levels() []logrus.Level {
	return s.levels
}

func (s *logHook) Fire(entry *logrus.Entry) error {
	s.entries = append(s.entries, entry)
	return nil
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

	body, err := io.ReadAll(r.Body)
	assert.NoError(t, err)

	var req map[string]interface{}
	err = json.Unmarshal(body, &req)
	assert.NoError(t, err)

	token := req["token"].(string)
	require.NotEmpty(t, r.Header.Get(RunnerToken), "runner-token header is required")
	require.Equal(t, token, r.Header.Get("runner-token"), "token in header and body must match")

	switch token {
	case validGlrtToken, validToken:
		w.WriteHeader(http.StatusNoContent)
	case invalidToken:
		w.WriteHeader(http.StatusForbidden)
	default:
		w.WriteHeader(http.StatusBadRequest)
	}
}

func testUnregisterRunnerManagerHandler(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.URL.Path != "/api/v4/runners/managers" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusNotAcceptable)
		return
	}

	body, err := io.ReadAll(r.Body)
	assert.NoError(t, err)

	var req map[string]interface{}
	err = json.Unmarshal(body, &req)
	assert.NoError(t, err)

	token := req["token"].(string)
	require.NotEmpty(t, r.Header.Get(RunnerToken), "runner-token header is required")
	require.Equal(t, token, r.Header.Get("runner-token"), "token in header and body must match")

	switch token {
	case validGlrtToken:
		if systemID, ok := req["system_id"].(string); ok && systemID == "s_some_system_id" {
			w.WriteHeader(http.StatusNoContent)
		} else {
			w.WriteHeader(http.StatusForbidden)
		}
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

func TestUnregisterRunnerManager(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		testUnregisterRunnerManagerHandler(w, r, t)
	}

	s := httptest.NewServer(http.HandlerFunc(handler))
	defer s.Close()

	validGlrtToken := RunnerCredentials{
		URL:   s.URL,
		Token: validGlrtToken,
	}

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

	state := c.UnregisterRunnerManager(validGlrtToken, "s_some_system_id")
	assert.True(t, state)

	state = c.UnregisterRunnerManager(validGlrtToken, "s_unknown_system_id")
	assert.False(t, state)

	state = c.UnregisterRunnerManager(validToken, "s_some_system_id")
	assert.True(t, state)

	state = c.UnregisterRunnerManager(invalidToken, "s_some_system_id")
	assert.False(t, state)

	state = c.UnregisterRunnerManager(otherToken, "s_some_system_id")
	assert.False(t, state)

	state = c.UnregisterRunnerManager(brokenCredentials, "s_some_system_id")
	assert.False(t, state)
}

func testVerifyRunnerHandler(w http.ResponseWriter, r *http.Request, legacyServer bool, t *testing.T) {
	if r.URL.Path != "/api/v4/runners/verify" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotAcceptable)
		return
	}

	body, err := io.ReadAll(r.Body)
	assert.NoError(t, err)

	var req map[string]interface{}
	err = json.Unmarshal(body, &req)
	assert.NoError(t, err)

	res := make(map[string]interface{})

	token := req["token"].(string)
	require.NotEmpty(t, r.Header.Get(RunnerToken), "runner-token header is required")
	require.Equal(t, token, r.Header.Get("runner-token"), "token in header and body must match")

	switch token {
	case validToken:
		if legacyServer {
			w.Header().Set("Content-Type", "plain/text")
			w.WriteHeader(http.StatusOK)
			return
		}
	case validGlrtToken:
		if legacyServer {
			w.Header().Set("Content-Type", "plain/text")
			w.WriteHeader(http.StatusOK)
			return
		}
	case invalidToken:
		w.WriteHeader(http.StatusForbidden)
		return
	default:
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // since the job id is broken, we should not find this job
	res["id"] = 54321
	res["token"] = req["token"].(string)
	res["token_expires_at"] = "2684-10-16T13:25:59Z"

	output, err := json.Marshal(res)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	_, _ = w.Write(output)
}

func TestVerifyRunnerOnLegacyServer(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		testVerifyRunnerHandler(w, r, true, t)
	}

	s := httptest.NewServer(http.HandlerFunc(handler))
	defer s.Close()

	validToken := RunnerCredentials{
		URL:   s.URL,
		Token: validToken,
	}

	validGlrtToken := RunnerCredentials{
		URL:   s.URL,
		Token: validGlrtToken,
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

	res := c.VerifyRunner(validToken, "")
	require.NotNil(t, res)
	assert.Equal(t, int64(0), res.ID)

	res = c.VerifyRunner(validGlrtToken, "")
	require.NotNil(t, res)

	res = c.VerifyRunner(invalidToken, "")
	assert.Nil(t, res)

	res = c.VerifyRunner(otherToken, "")
	assert.NotNil(
		t,
		res,
		"in other cases where we can't explicitly say that runner is valid we say that it is",
	)

	res = c.VerifyRunner(brokenCredentials, "")
	assert.NotNil(
		t,
		res,
		"in other cases where we can't explicitly say that runner is valid we say that it is",
	)
}

func TestVerifyRunner(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		testVerifyRunnerHandler(w, r, false, t)
	}

	s := httptest.NewServer(http.HandlerFunc(handler))
	defer s.Close()

	validToken := RunnerCredentials{
		URL:   s.URL,
		Token: validToken,
	}

	validGlrtToken := RunnerCredentials{
		URL:   s.URL,
		Token: validGlrtToken,
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

	res := c.VerifyRunner(validToken, "")
	require.NotNil(t, res)
	assert.Equal(t, int64(54321), res.ID)
	assert.Equal(t, validToken.Token, res.Token)
	assert.Equal(
		t,
		time.Date(2684, 10, 16, 13, 25, 59, 0, time.UTC),
		res.TokenExpiresAt,
	)

	res = c.VerifyRunner(validGlrtToken, "")
	require.NotNil(t, res)
	assert.Equal(t, int64(54321), res.ID)
	assert.Equal(t, validGlrtToken.Token, res.Token)
	assert.Equal(
		t,
		time.Date(2684, 10, 16, 13, 25, 59, 0, time.UTC),
		res.TokenExpiresAt,
	)

	res = c.VerifyRunner(invalidToken, "")
	assert.Nil(t, res)

	res = c.VerifyRunner(otherToken, "")
	assert.NotNil(
		t,
		res,
		"in other cases where we can't explicitly say that runner is valid we say that it is",
	)

	res = c.VerifyRunner(brokenCredentials, "")
	assert.NotNil(
		t,
		res,
		"in other cases where we can't explicitly say that runner is valid we say that it is",
	)
}

func testResetTokenHandler(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.URL.Path != "/api/v4/runners/reset_authentication_token" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotAcceptable)
		return
	}

	if r.Header.Get(Accept) != "application/json" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)
	require.NoError(t, err)

	var req map[string]interface{}
	err = json.Unmarshal(body, &req)
	require.NoError(t, err)

	res := make(map[string]interface{})

	switch req["token"].(string) {
	case validToken:
		res["token"] = "reset-token"
		res["token_expires_at"] = nil
	case expiringToken:
		res["token"] = "reset-expiring-token"
		res["token_expires_at"] = "2684-10-16T13:25:59Z"
	case invalidToken:
		w.WriteHeader(http.StatusForbidden)
		return
	default:
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	output, err := json.Marshal(res)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set(ContentType, "application/json")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write(output)
}

func TestResetToken(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		testResetTokenHandler(w, r, t)
	}

	s := httptest.NewServer(http.HandlerFunc(handler))
	defer s.Close()

	validToken := RunnerCredentials{
		URL:   s.URL,
		Token: validToken,
	}

	expiringToken := RunnerCredentials{
		URL:            s.URL,
		Token:          expiringToken,
		TokenExpiresAt: time.Date(2684, 10, 16, 13, 25, 59, 0, time.UTC),
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

	res := c.ResetToken(validToken, "system-id-1")
	if assert.NotNil(t, res) {
		assert.Equal(t, "reset-token", res.Token)
		assert.True(t, res.TokenExpiresAt.IsZero())
	}

	res = c.ResetToken(expiringToken, "system-id-1")
	if assert.NotNil(t, res) {
		assert.Equal(t, "reset-expiring-token", res.Token)
		assert.Equal(t, expiringToken.TokenExpiresAt, res.TokenExpiresAt)
	}

	res = c.ResetToken(invalidToken, "system-id-1")
	assert.Nil(t, res)

	res = c.ResetToken(otherToken, "system-id-1")
	assert.Nil(t, res)
}

func testResetTokenWithPATHandler(w http.ResponseWriter, r *http.Request) {
	regex := regexp.MustCompilePOSIX("^/api/v4/runners/(.*)/reset_authentication_token$")
	matches := regex.FindStringSubmatch(r.URL.Path)
	if len(matches) != 2 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	id := matches[1]

	pat := r.Header.Get(PrivateToken)
	if pat == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotAcceptable)
		return
	}

	if r.Header.Get(Accept) != "application/json" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	res := make(map[string]interface{})

	switch id {
	case "12345":
		if pat != "valid-pat" {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		res["token"] = validToken
		res["token_expires_at"] = nil
	case "54321":
		res["token"] = expiringToken
		res["token_expires_at"] = "2684-10-16T13:25:59Z"
	case "77777":
		w.WriteHeader(http.StatusNotFound)
		return
	default:
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	output, err := json.Marshal(res)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set(ContentType, "application/json")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write(output)
}

func TestResetTokenWithPAT(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		testResetTokenWithPATHandler(w, r)
	}

	s := httptest.NewServer(http.HandlerFunc(handler))
	defer s.Close()

	validToken := RunnerCredentials{
		ID:    12345,
		URL:   s.URL,
		Token: validToken,
	}

	expiringToken := RunnerCredentials{
		ID:             54321,
		URL:            s.URL,
		Token:          expiringToken,
		TokenExpiresAt: time.Date(2684, 10, 16, 13, 25, 59, 0, time.UTC),
	}

	invalidToken := RunnerCredentials{
		ID:    77777,
		URL:   s.URL,
		Token: invalidToken,
	}

	otherToken := RunnerCredentials{
		ID:    88888,
		URL:   s.URL,
		Token: "other",
	}

	c := NewGitLabClient()

	res := c.ResetTokenWithPAT(validToken, "system-id-1", "valid-pat")
	if assert.NotNil(t, res) {
		assert.Equal(t, validToken.Token, res.Token)
		assert.True(t, res.TokenExpiresAt.IsZero())
	}

	res = c.ResetTokenWithPAT(expiringToken, "system-id-1", "valid-pat")
	if assert.NotNil(t, res) {
		assert.Equal(t, expiringToken.Token, res.Token)
		assert.Equal(t, expiringToken.TokenExpiresAt, res.TokenExpiresAt)
	}

	res = c.ResetTokenWithPAT(validToken, "system-id-1", "")
	assert.Nil(t, res)

	res = c.ResetTokenWithPAT(validToken, "system-id-1", "invalid-pat")
	assert.Nil(t, res)

	res = c.ResetTokenWithPAT(invalidToken, "system-id-1", "valid-pat")
	assert.Nil(t, res)

	res = c.ResetTokenWithPAT(otherToken, "system-id-1", "valid-pat")
	assert.Nil(t, res)
}

func getRequestJobResponse(tb testing.TB, validResponse bool) string {
	tb.Helper()

	imageExecutorOptsKey := "platform"
	imageExecutorOptsValue := "arm64/v8"
	svcExecutorOptsKey := "platform"
	svcExecutorOptsValue := "amd64/linux"

	if !validResponse {
		imageExecutorOptsKey = "blammo"
		imageExecutorOptsValue = "invalid"
		svcExecutorOptsKey = "powpow"
		svcExecutorOptsValue = "invalid"
	}

	return fmt.Sprintf(`{
  "id": 10,
  "token": "job-token",
  "allow_git_fetch": false,
  "job_info": {
	"name": "test-job",
	"stage": "test",
	"project_id": 123,
	"project_name": "test-project"
  },
  "git_info": {
	"repo_url": "https://gitlab-ci-token:testTokenHere1234@gitlab.example.com/test/test-project.git",
	"ref": "main",
	"sha": "abcdef123456",
	"before_sha": "654321fedcba",
	"ref_type": "branch"
  },
  "runner_info": {
	"timeout": 3600
  },
  "variables": [
	{
	  "key": "CI_REF_NAME",
	  "value": "main",
	  "public": true,
	  "file": true,
	  "raw": true
	}
  ],
  "steps": [
	{
	  "name": "script",
	  "script": ["date", "ls -ls"],
	  "timeout": 3600,
	  "when": "on_success",
	  "allow_failure": false
	},
	{
	  "name": "after_script",
	  "script": ["ls -ls"],
	  "timeout": 3600,
	  "when": "always",
	  "allow_failure": true
	}
  ],
  "image": {
	"name": "ruby:3.3",
	"entrypoint": ["/bin/sh"],
	"executor_opts": {
	  "docker": {
		"%s": "%s"
	  }
	}
  },
  "services": [
	{
	  "name": "postgresql:9.5",
	  "entrypoint": ["/bin/sh"],
	  "command": ["sleep", "30"],
	  "alias": "db-pg",
	  "executor_opts": {
		"docker": {
		  "%s": "%s"
		}
	  }
	},
	{
	  "name": "mysql:5.6",
	  "alias": "db-mysql",
	  "executor_opts": {
		"docker": {
		  "platform": "arm"
		}
	  }
	}
  ],
  "artifacts": [
	{
	  "name": "artifact.zip",
	  "untracked": false,
	  "paths": ["out/*"],
	  "when": "always",
	  "expire_in": "7d"
	}
  ],
  "cache": [
	{
	  "key": "$CI_COMMIT_SHA",
	  "untracked": false,
	  "paths": ["vendor/*"],
	  "policy": "push"
	}
  ],
  "credentials": [
	{
	  "type": "Registry",
	  "url": "http://registry.gitlab.example.com/",
	  "username": "gitlab-ci-token",
	  "password": "job-token"
	}
  ],
  "dependencies": [
	{
	  "id": 9,
	  "name": "other-job",
	  "token": "other-job-token",
	  "artifacts_file": {
		"filename": "binaries.zip",
		"size": 13631488
	  }
	}
  ]
}`, imageExecutorOptsKey, imageExecutorOptsValue, svcExecutorOptsKey, svcExecutorOptsValue)
}

func mockRequestJobHandler(t *testing.T, w http.ResponseWriter, r *http.Request, jobResponse string) {
	t.Helper()
	w.Header().Add(correlationIDHeader, "foobar")

	if r.URL.Path != "/api/v4/jobs/request" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotAcceptable)
		return
	}

	body, err := io.ReadAll(r.Body)
	assert.NoError(t, err)

	var req map[string]interface{}
	err = json.Unmarshal(body, &req)
	assert.NoError(t, err)

	assert.Equal(t, testSystemID, req["system_id"])

	token := req["token"].(string)
	require.NotEmpty(t, r.Header.Get(RunnerToken), "runner-token header is required")
	require.Equal(t, token, r.Header.Get("runner-token"), "token in header and body must match")

	switch token {
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

	assert.Equal(t, testSystemID, req["system_id"])

	if r.Header.Get(Accept) != "application/json" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.Header().Set(ContentType, "application/json")
	w.WriteHeader(http.StatusCreated)
	_, err = w.Write([]byte(jobResponse))
	require.NoError(t, err, "failed to write job response")
}

func TestGitLabClient_RequestJob(t *testing.T) {
	t.Parallel()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := getRequestJobResponse(t, true)
		if strings.Contains(r.URL.Path, "/unsupported") {
			// Downstream handler performs a check on path. Unsupported is only
			// need to trigger the response with invalid options.
			r.URL.Path = strings.TrimPrefix(r.URL.Path, "/unsupported")
			response = getRequestJobResponse(t, false)
		}
		mockRequestJobHandler(t, w, r, response)
	}))
	defer s.Close()

	logHook := newLogHook(logrus.InfoLevel, logrus.ErrorLevel)
	logrus.AddHook(&logHook)

	type expected struct {
		responseOK  bool
		responseNil bool
	}

	testCases := []struct {
		name                  string
		token                 string
		gitlabURL             string
		assertUnsupportedOpts bool
		expected              expected
	}{
		{
			name:      "valid token",
			token:     validToken,
			gitlabURL: s.URL,
			expected: expected{
				responseOK: true,
			},
		},
		{
			name:      "no jobs token",
			token:     "no-jobs",
			gitlabURL: s.URL,
			expected: expected{
				responseOK:  true,
				responseNil: true,
			},
		},
		{
			name:      "invalid token",
			token:     invalidToken,
			gitlabURL: s.URL,
			expected: expected{
				responseNil: true,
			},
		},
		{
			name:      "invalid url",
			token:     validToken,
			gitlabURL: "invalid",
			expected: expected{
				responseNil: true,
			},
		},
		{
			name:                  "unsupported executor options",
			token:                 validToken,
			gitlabURL:             s.URL + "/unsupported",
			assertUnsupportedOpts: true,
			expected: expected{
				responseOK: true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			glc := NewGitLabClient()
			rc := RunnerConfig{
				RunnerCredentials: RunnerCredentials{
					URL:   tc.gitlabURL,
					Token: tc.token,
				},
				SystemID: testSystemID,
			}

			// Act
			res, ok := glc.RequestJob(t.Context(), rc, nil)

			// Assert
			assert.Equal(t, tc.expected.responseOK, ok)
			if tc.expected.responseNil {
				assert.Nil(t, res)
			} else {
				assertOnJobResponse(t, res, tc.assertUnsupportedOpts)
			}

			if tc.token == "no-jobs" {
				assert.Nil(t, res)
				assert.True(t, ok, "If no jobs, runner is healthy")
				assert.Equal(t, "a nice timestamp", glc.getLastUpdate(&rc.RunnerCredentials), "Last-Update should be set")
			}
		})
	}

	expectedLogs := []logrus.Entry{
		{
			Level:   logrus.InfoLevel,
			Message: "Checking for jobs... received",
			Data: logrus.Fields{
				"correlation_id": "foobar",
				"job":            int64(10),
				"repo_url":       "https://gitlab.example.com/test/test-project.git",
				"runner":         "valid",
			},
		},
		{
			Level:   logrus.ErrorLevel,
			Message: "Checking for jobs... forbidden",
			Data: logrus.Fields{
				"correlation_id": "foobar",
				"runner":         "invalid",
				"status":         "403 Forbidden",
			},
		},
		{
			Level:   logrus.ErrorLevel,
			Message: "Checking for jobs... client error",
			Data: logrus.Fields{
				"correlation_id": "",
				"runner":         "valid",
				"status":         "only http or https scheme supported",
			},
		},
		{
			Level:   logrus.InfoLevel,
			Message: "Checking for jobs... received",
			Data: logrus.Fields{
				"correlation_id": "foobar",
				"job":            int64(10),
				"repo_url":       "https://gitlab.example.com/test/test-project.git",
				"runner":         "valid",
			},
		},
	}

	require.Len(t, logHook.entries, len(expectedLogs))
	for i, l := range expectedLogs {
		assert.Equal(t, l.Level, logHook.entries[i].Level)
		assert.Equal(t, l.Message, logHook.entries[i].Message)
		assert.Equal(t, l.Data, logHook.entries[i].Data)
	}
}

func assertOnJobResponse(tb testing.TB, res *JobResponse, assertUnsupportedOpts bool) {
	tb.Helper()
	assert.NotNil(tb, res)
	assert.NotEmpty(tb, res.ID)
	assert.Equal(tb, "ruby:3.3", res.Image.Name)
	assert.Equal(tb, []string{"/bin/sh"}, res.Image.Entrypoint)

	require.Len(tb, res.Services, 2)
	assert.Equal(tb, "postgresql:9.5", res.Services[0].Name)
	assert.Equal(tb, []string{"/bin/sh"}, res.Services[0].Entrypoint)
	assert.Equal(tb, []string{"sleep", "30"}, res.Services[0].Command)
	assert.Equal(tb, "db-pg", res.Services[0].Alias)

	assert.Equal(tb, "mysql:5.6", res.Services[1].Name)
	assert.Equal(tb, "db-mysql", res.Services[1].Alias)
	assert.Equal(tb, "arm", res.Services[1].ExecutorOptions.Docker.Platform)

	require.Len(tb, res.Variables, 1)
	assert.Equal(tb, "CI_REF_NAME", res.Variables[0].Key)
	assert.Equal(tb, "main", res.Variables[0].Value)
	assert.True(tb, res.Variables[0].Public)
	assert.True(tb, res.Variables[0].File)
	assert.True(tb, res.Variables[0].Raw)

	if assertUnsupportedOpts {
		assert.NotNil(tb, res.UnsupportedOptions())
		assert.Contains(tb, res.UnsupportedOptions().Error(), "blammo")
		assert.Contains(tb, res.UnsupportedOptions().Error(), "powpow")
	} else {
		require.Equal(tb, "arm64/v8", res.Image.ExecutorOptions.Docker.Platform)
		assert.Equal(tb, "amd64/linux", res.Services[0].ExecutorOptions.Docker.Platform)
	}
}

func TestGitLabClient_RequestJob_TooManyRequests(t *testing.T) {
	t.Parallel()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(retryAfterHeader, "60")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer s.Close()

	validToken := RunnerConfig{
		RunnerCredentials: RunnerCredentials{
			URL:   s.URL,
			Token: validToken,
		},
		SystemID: testSystemID,
	}

	c := NewGitLabClient()

	testStart := time.Now()
	ctx, cancelFunc := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancelFunc()

	_, ok := c.RequestJob(ctx, validToken, nil)
	assert.True(t, ok)
	assert.WithinDuration(t, testStart, time.Now(), 1500*time.Millisecond)
}

func setStateForUpdateJobHandlerResponse(w http.ResponseWriter, req map[string]interface{}) {
	switch req["state"].(string) {
	case statusRunning, statusCanceling:
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
	w.Header().Add(correlationIDHeader, "foobar")

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

	body, err := io.ReadAll(r.Body)
	assert.NoError(t, err)

	var req map[string]interface{}
	err = json.Unmarshal(body, &req)
	assert.NoError(t, err)

	token := req["token"].(string)
	require.NotEmpty(t, r.Header.Get(JobToken), "job-token header is required")
	require.Equal(t, token, r.Header.Get("job-token"), "token in header and body must match")

	assert.Equal(t, "token", token)

	setStateForUpdateJobHandlerResponse(w, req)
}

func TestUpdateJob(t *testing.T) {
	output := JobTraceOutput{
		Checksum: "checksum",
		Bytesize: 42,
	}

	type testCase struct {
		updateJobInfo   UpdateJobInfo
		updateJobResult UpdateJobResult
		additionalLog   *logrus.Entry
	}

	testCases := map[string]testCase{
		"Update continues when running": {
			updateJobInfo:   UpdateJobInfo{ID: 200, State: Running, Output: output},
			updateJobResult: UpdateJobResult{State: UpdateSucceeded},
			additionalLog:   &logrus.Entry{Message: "Submitting job to coordinator...ok"},
		},
		"Update aborts if the access is forbidden": {
			updateJobInfo:   UpdateJobInfo{ID: 403, State: Success, Output: output},
			updateJobResult: UpdateJobResult{State: UpdateAbort},
		},
		"Update fails for badly formatted request": {
			updateJobInfo:   UpdateJobInfo{ID: 200, State: "invalid-state", Output: output},
			updateJobResult: UpdateJobResult{State: UpdateFailed},
		},
		"Update aborts for unknown job": {
			updateJobInfo:   UpdateJobInfo{ID: 404, State: Success, Output: output},
			updateJobResult: UpdateJobResult{State: UpdateAbort},
		},
		"Update returns accepted, but not completed if server returns `202 StatusAccepted`": {
			updateJobInfo:   UpdateJobInfo{ID: 202, State: Success, Output: output},
			updateJobResult: UpdateJobResult{State: UpdateAcceptedButNotCompleted},
			additionalLog: &logrus.Entry{
				Message: "Submitting job to coordinator...accepted, but not yet completed",
			},
		},
		"Update returns reset content requested if server returns `412 Precondition Failed`": {
			updateJobInfo:   UpdateJobInfo{ID: 412, State: Success, Output: output},
			updateJobResult: UpdateJobResult{State: UpdateTraceValidationFailed},
			additionalLog: &logrus.Entry{
				Message: "Submitting job to coordinator...trace validation failed",
			},
		},
		"Update should continue when script fails": {
			updateJobInfo:   UpdateJobInfo{ID: 200, State: Failed, FailureReason: ScriptFailure, Output: output},
			updateJobResult: UpdateJobResult{State: UpdateSucceeded},
			additionalLog:   &logrus.Entry{Message: "Submitting job to coordinator...ok"},
		},
		"Update fails for invalid failure reason": {
			updateJobInfo: UpdateJobInfo{
				ID:            200,
				State:         Failed,
				FailureReason: "invalid-failure-reason",
				Output:        output,
			},
			updateJobResult: UpdateJobResult{State: UpdateFailed},
		},
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		testUpdateJobHandler(w, r, t)
	}

	s := httptest.NewServer(http.HandlerFunc(handler))
	defer s.Close()

	config := RunnerConfig{
		RunnerCredentials: RunnerCredentials{
			URL: s.URL,
		},
		SystemID: testSystemID,
	}

	jobCredentials := &JobCredentials{
		Token: "token",
	}

	c := NewGitLabClient()

	for tn, tc := range testCases {
		t.Run(tn, func(t *testing.T) {
			h := newLogHook(logrus.InfoLevel)
			logrus.AddHook(&h)

			result := c.UpdateJob(config, jobCredentials, tc.updateJobInfo)
			assert.Equal(t, tc.updateJobResult, result, tn)

			entriesLen := 1
			if tc.additionalLog != nil {
				entriesLen++
			}
			require.Len(t, h.entries, entriesLen)
			assert.Equal(t, "Updating job...", h.entries[0].Message)
			assert.Equal(t, tc.updateJobInfo.ID, h.entries[0].Data["job"])
			assert.Equal(t, tc.updateJobInfo.Output.Bytesize, h.entries[0].Data["bytesize"])
			assert.Equal(t, tc.updateJobInfo.Output.Checksum, h.entries[0].Data["checksum"])
			if tc.additionalLog != nil {
				assert.Equal(t, tc.additionalLog.Message, h.entries[1].Message)
				assert.Equal(t, "foobar", h.entries[1].Data["correlation_id"])
			}
		})
	}
}

func testUpdateJobKeepAliveHandler(w http.ResponseWriter, r *http.Request, t *testing.T) {
	w.Header().Add(correlationIDHeader, "foobar")

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

	body, err := io.ReadAll(r.Body)
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
		SystemID: testSystemID,
	}

	jobCredentials := &JobCredentials{
		Token: "token",
	}

	c := NewGitLabClient()

	type testCase struct {
		updateJobInfo   UpdateJobInfo
		updateJobResult UpdateJobResult
		expectedLogs    []logrus.Entry
	}

	testCases := map[string]testCase{
		"Update should continue when running": {
			updateJobInfo:   UpdateJobInfo{ID: 10, State: Running},
			updateJobResult: UpdateJobResult{State: UpdateSucceeded},
			expectedLogs: []logrus.Entry{
				{
					Level:   logrus.InfoLevel,
					Message: "Updating job...",
				},
				{
					Level:   logrus.InfoLevel,
					Message: "Submitting job to coordinator...ok",
					Data: logrus.Fields{
						"bytesize":        0,
						"checksum":        "",
						"code":            200,
						"correlation_id":  "foobar",
						"job":             int64(10),
						"job-status":      "",
						"update-interval": time.Duration(0),
					},
				},
			},
		},
		"Update should be aborted when Job-Status=canceled": {
			updateJobInfo:   UpdateJobInfo{ID: 11, State: Running},
			updateJobResult: UpdateJobResult{State: UpdateAbort},
			expectedLogs: []logrus.Entry{
				{
					Level:   logrus.InfoLevel,
					Message: "Updating job...",
				},
				{
					Level:   logrus.WarnLevel,
					Message: "Submitting job to coordinator... job failed",
					Data: logrus.Fields{
						"bytesize":        0,
						"checksum":        "",
						"code":            200,
						"correlation_id":  "foobar",
						"job":             int64(11),
						"job-status":      "canceled",
						"status":          "200 OK",
						"update-interval": time.Duration(0),
					},
				},
			},
		},
		"Update should continue when Job-Status=failed": {
			updateJobInfo:   UpdateJobInfo{ID: 12, State: Running},
			updateJobResult: UpdateJobResult{State: UpdateAbort},
			expectedLogs: []logrus.Entry{
				{
					Level:   logrus.InfoLevel,
					Message: "Updating job...",
				},
				{
					Level:   logrus.WarnLevel,
					Message: "Submitting job to coordinator... job failed",
					Data: logrus.Fields{
						"bytesize":        0,
						"checksum":        "",
						"code":            200,
						"correlation_id":  "foobar",
						"job":             int64(12),
						"job-status":      "failed",
						"status":          "200 OK",
						"update-interval": time.Duration(0),
					},
				},
			},
		},
		"Update should continue when Job-Status=canceling": {
			updateJobInfo:   UpdateJobInfo{ID: 13, State: Running},
			updateJobResult: UpdateJobResult{State: UpdateSucceeded, CancelRequested: true},
			expectedLogs: []logrus.Entry{
				{
					Level:   logrus.InfoLevel,
					Message: "Updating job...",
				},
				{
					Level:   logrus.InfoLevel,
					Message: "Submitting job to coordinator...ok",
					Data: logrus.Fields{
						"bytesize":        0,
						"checksum":        "",
						"code":            200,
						"correlation_id":  "foobar",
						"job":             int64(13),
						"job-status":      "canceling",
						"update-interval": time.Duration(0),
					},
				},
			},
		},
	}

	for tn, tc := range testCases {
		t.Run(tn, func(t *testing.T) {
			h := newLogHook(logrus.InfoLevel, logrus.WarnLevel)
			logrus.AddHook(&h)

			result := c.UpdateJob(config, jobCredentials, tc.updateJobInfo)
			assert.Equal(t, tc.updateJobResult, result)
			require.Len(t, h.entries, len(tc.expectedLogs))
			for i, l := range tc.expectedLogs {
				assert.Equal(t, l.Level, h.entries[i].Level)
				assert.Equal(t, l.Message, h.entries[i].Message)

				if l.Data != nil {
					assert.Equal(t, l.Data, h.entries[i].Data)
				}
			}
		})
	}
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

		assert.Equal(t, patchToken, r.Header.Get(JobToken))

		body, err := io.ReadAll(r.Body)
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
		SystemID: testSystemID,
	}

	return server, NewGitLabClient(), config
}

func TestUnknownPatchTrace(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request, body []byte, offset, limit int) {
		w.WriteHeader(http.StatusNotFound)
	}

	server, client, config := getPatchServer(t, handler)
	defer server.Close()

	result := client.PatchTrace(config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent, 0, false)
	assert.Equal(t, PatchNotFound, result.State)
}

func TestForbiddenPatchTrace(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request, body []byte, offset, limit int) {
		w.WriteHeader(http.StatusForbidden)
	}

	server, client, config := getPatchServer(t, handler)
	defer server.Close()

	result := client.PatchTrace(config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent, 0, false)
	assert.Equal(t, PatchAbort, result.State)
}

func TestPatchTrace(t *testing.T) {
	tests := []struct {
		remoteState    string
		expectedResult PatchTraceResult
	}{
		{
			remoteState: statusRunning,
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
				w.Header().Add(correlationIDHeader, "foobar")
				w.WriteHeader(http.StatusAccepted)
			}

			server, client, config := getPatchServer(t, handler)
			defer server.Close()

			h := newLogHook(logrus.InfoLevel)
			logrus.AddHook(&h)

			result := client.PatchTrace(
				config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent, 0, false,
			)
			assert.Equal(t, tt.expectedResult.State, result.State)
			assert.Equal(t, tt.expectedResult.CancelRequested, result.CancelRequested)
			assert.Equal(t, len(patchTraceContent), result.SentOffset)

			result = client.PatchTrace(
				config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent[3:], 3, false,
			)
			assert.Equal(t, tt.expectedResult.State, result.State)
			assert.Equal(t, tt.expectedResult.CancelRequested, result.CancelRequested)
			assert.Equal(t, len(patchTraceContent), result.SentOffset)

			result = client.PatchTrace(
				config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent[3:10], 3, false,
			)
			assert.Equal(t, tt.expectedResult.State, result.State)
			assert.Equal(t, tt.expectedResult.CancelRequested, result.CancelRequested)
			assert.Equal(t, 10, result.SentOffset)

			require.Len(t, h.entries, 3)
			for _, entry := range h.entries {
				assert.Equal(t, entry.Message, "Appending trace to coordinator...ok")
				assert.Equal(t, "foobar", entry.Data["correlation_id"])
			}
		})
	}
}

func TestRangeMismatchPatchTrace(t *testing.T) {
	tests := []struct {
		remoteState    string
		expectedResult PatchTraceResult
	}{
		{
			remoteState: statusRunning,
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

	expectedLogs := []logrus.Entry{
		{
			Level:   logrus.WarnLevel,
			Message: "Appending trace to coordinator... range mismatch",
		},
		{
			Level:   logrus.WarnLevel,
			Message: "Appending trace to coordinator... range mismatch",
		},
		{
			Level:   logrus.InfoLevel,
			Message: "Appending trace to coordinator...ok",
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

			h := newLogHook(logrus.InfoLevel, logrus.WarnLevel)
			logrus.AddHook(&h)

			result := client.PatchTrace(
				config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent[11:], 11, false,
			)
			assert.Equal(t, PatchTraceResult{State: PatchRangeMismatch, SentOffset: 10}, result)

			result = client.PatchTrace(
				config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent[15:], 15, false,
			)
			assert.Equal(t, PatchTraceResult{State: PatchRangeMismatch, SentOffset: 10}, result)

			result = client.PatchTrace(
				config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent[5:], 5, false,
			)
			assert.Equal(t, tt.expectedResult, result)

			require.Len(t, h.entries, len(expectedLogs))
			for i, l := range expectedLogs {
				assert.Equal(t, l.Level, h.entries[i].Level)
				assert.Equal(t, l.Message, h.entries[i].Message)
			}
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

	result := client.PatchTrace(config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent, 0, false)
	assert.Equal(t, PatchAbort, result.State)
}

func TestPatchTraceCantConnect(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request, body []byte, offset, limit int) {}

	server, client, config := getPatchServer(t, handler)
	server.Close()

	result := client.PatchTrace(config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent, 0, false)
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
			remoteJobStatus:       statusRunning,
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
			remoteJobStatus:       statusRunning,
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
			remoteJobStatus:       statusRunning,
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
			remoteJobStatus:       statusRunning,
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

			h := newLogHook(logrus.InfoLevel)
			logrus.AddHook(&h)

			traceContent = append(traceContent, update.traceUpdate...)
			result := client.PatchTrace(
				config, &JobCredentials{ID: 1, Token: patchToken},
				traceContent[sentTrace:], sentTrace, false,
			)
			assert.Equal(t, update.expectedResult, result)
			require.Len(t, h.entries, 1)
			if update.expectedContentRange == "" || update.expectedContentLength == 0 {
				assert.Equal(t, "Appending trace to coordinator...skipped due to empty patch", h.entries[0].Message)
			} else {
				assert.Equal(t, "Appending trace to coordinator...ok", h.entries[0].Message)
			}

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
			remoteJobStatus: statusRunning,
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
			remoteJobStatus:       statusRunning,
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
			remoteJobStatus:       statusRunning,
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

			h := newLogHook(logrus.InfoLevel)
			logrus.AddHook(&h)

			result := client.PatchTrace(config, &JobCredentials{ID: 1, Token: patchToken}, test.trace, 0, false)
			assert.Equal(t, test.expectedResult, result)
			require.Len(t, h.entries, 1)
			if test.expectedContentRange == "" || test.expectedContentLength == 0 {
				assert.Equal(t, "Appending trace to coordinator...skipped due to empty patch", h.entries[0].Message)
			} else {
				assert.Equal(t, "Appending trace to coordinator...ok", h.entries[0].Message)
			}
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

	h := newLogHook(logrus.InfoLevel)
	logrus.AddHook(&h)

	client.PatchTrace(config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent, 0, false)
	require.Len(t, h.entries, 1)
	assert.Equal(t, "Appending trace to coordinator...ok", h.entries[0].Message)
}

func TestPatchTraceUrlParams(t *testing.T) {
	expected := "debug_trace=false"

	handler := func(w http.ResponseWriter, r *http.Request, body []byte, offset, limit int) {
		assert.Equal(t, "/api/v4/jobs/1/trace", r.URL.Path)
		assert.Equal(t, expected, r.URL.RawQuery)
		w.WriteHeader(http.StatusAccepted)
	}

	server, client, config := getPatchServer(t, handler)
	defer server.Close()

	result := client.PatchTrace(config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent, 0, false)
	assert.Equal(t, PatchSucceeded, result.State)

	expected = "debug_trace=true"
	result = client.PatchTrace(config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent, 0, true)
	assert.Equal(t, PatchSucceeded, result.State)
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

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			t.Run("UpdateJob", func(t *testing.T) {
				handler := func(w http.ResponseWriter, r *http.Request) {
					if tc.sendUpdateIntervalHeader {
						w.Header().Add(updateIntervalHeader, tc.updateIntervalHeaderValue)
					}

					testUpdateJobHandler(w, r, t)
				}

				server := httptest.NewServer(http.HandlerFunc(handler))
				defer server.Close()

				config := RunnerConfig{
					RunnerCredentials: RunnerCredentials{URL: server.URL},
					SystemID:          testSystemID,
				}

				h := newLogHook(logrus.InfoLevel, logrus.WarnLevel)
				logrus.AddHook(&h)

				result := NewGitLabClient().UpdateJob(config, &JobCredentials{ID: 10}, UpdateJobInfo{State: "success"})
				assert.Equal(t, tc.expectedUpdateInterval, result.NewUpdateInterval)
				expectedLogs := []logrus.Entry{
					{
						Level:   logrus.InfoLevel,
						Message: "Updating job...",
					},
				}
				if tc.updateIntervalHeaderValue == "some text" {
					// Invalid format header will expectedResult in an additional log
					expectedLogs = append(expectedLogs, logrus.Entry{
						Level:   logrus.WarnLevel,
						Message: fmt.Sprintf("Failed to parse %q header", updateIntervalHeader),
					})
				}
				expectedLogs = append(expectedLogs, logrus.Entry{
					Level:   logrus.WarnLevel,
					Message: "Submitting job to coordinator... not found",
				})

				require.Len(t, h.entries, len(expectedLogs))
				for i, l := range expectedLogs {
					assert.Equal(t, l.Level, h.entries[i].Level)
					assert.Equal(t, l.Message, h.entries[i].Message)
				}
			})

			t.Run("PatchTrace", func(t *testing.T) {
				handler := func(w http.ResponseWriter, r *http.Request, body []byte, offset, limit int) {
					if tc.sendUpdateIntervalHeader {
						w.Header().Add(updateIntervalHeader, tc.updateIntervalHeaderValue)
					}

					w.WriteHeader(http.StatusAccepted)
				}

				server, client, config := getPatchServer(t, handler)
				defer server.Close()

				h := newLogHook(logrus.InfoLevel)
				logrus.AddHook(&h)

				result := client.PatchTrace(
					config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent, 0, false,
				)
				assert.Equal(t, tc.expectedUpdateInterval, result.NewUpdateInterval)
				require.Len(t, h.entries, 1)
				assert.Equal(t, "Appending trace to coordinator...ok", h.entries[0].Message)
			})
		})
	}
}

func TestAbortedPatchTrace(t *testing.T) {
	tests := map[string]struct {
		expectedResult   PatchTraceResult
		expectedLogEntry logrus.Entry
	}{
		statusCanceling: {
			expectedResult: PatchTraceResult{SentOffset: 17, CancelRequested: true, State: PatchSucceeded},
			expectedLogEntry: logrus.Entry{
				Level:   logrus.InfoLevel,
				Message: "Appending trace to coordinator...ok",
			},
		},
		statusCanceled: {
			expectedResult: PatchTraceResult{State: PatchAbort},
			expectedLogEntry: logrus.Entry{
				Level:   logrus.WarnLevel,
				Message: "Appending trace to coordinator... job failed",
			},
		},
		statusFailed: {
			expectedResult: PatchTraceResult{State: PatchAbort},
			expectedLogEntry: logrus.Entry{
				Level:   logrus.WarnLevel,
				Message: "Appending trace to coordinator... job failed",
			},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			handler := func(w http.ResponseWriter, r *http.Request, body []byte, offset, limit int) {
				w.Header().Set("Job-Status", tn)
				w.WriteHeader(http.StatusAccepted)
			}

			server, client, config := getPatchServer(t, handler)
			defer server.Close()

			h := newLogHook(tc.expectedLogEntry.Level)
			logrus.AddHook(&h)

			result := client.PatchTrace(config, &JobCredentials{ID: 1, Token: patchToken}, patchTraceContent, 0, false)
			assert.Equal(t, tc.expectedResult, result)
			require.Len(t, h.entries, 1)
			assert.Equal(t, tc.expectedLogEntry.Message, h.entries[0].Message)
		})
	}
}

func checkTestArtifactsUploadHandlerContent(w http.ResponseWriter, r *http.Request, body string) {
	cases := map[string]struct {
		formValueKey string
		statusCode   int
		body         string
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
		"bad-request": {
			statusCode: http.StatusBadRequest,
			body:       `{"message": "duplicate variables"}`,
		},
		"bad-request-not-json": {
			statusCode: http.StatusBadRequest,
			body:       `not JSON response`,
		},
	}

	testCase, ok := cases[body]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.Header().Set(ContentType, "application/json")

	if testCase.formValueKey != "" {
		if r.FormValue(testCase.formValueKey) != body {
			return
		}
	}

	w.WriteHeader(testCase.statusCode)
	_, _ = w.Write([]byte(testCase.body))
}

func testArtifactsUploadHandler(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.URL.Path == "/api/v4/jobs/10/new-location" {
		w.WriteHeader(http.StatusCreated)
		return
	}

	if r.URL.Path != "/api/v4/jobs/10/artifacts" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotAcceptable)
		return
	}

	if r.Header.Get(JobToken) == "redirect" {
		w.Header().Set("Location", "new-location")
		w.WriteHeader(http.StatusTemporaryRedirect)
		return
	}

	if r.Header.Get(JobToken) != "token" {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(file)
	require.NoError(t, err)

	checkTestArtifactsUploadHandlerContent(w, r, string(body))
}

func uploadArtifacts(
	client *GitLabClient,
	config JobCredentials,
	artifactsFile,
	artifactType string,
	artifactFormat ArtifactFormat,
	logResponseDetails bool,
) (UploadState, string) {
	file, err := os.Open(artifactsFile)
	if err != nil {
		return UploadFailed, ""
	}
	defer file.Close()

	fi, err := file.Stat()
	if err != nil {
		return UploadFailed, ""
	}
	if fi.IsDir() {
		return UploadFailed, ""
	}

	bodyProvider := StreamProvider{
		ReaderFactory: func() (io.ReadCloser, error) {
			// Open the file again in case there are retries
			file, err := os.Open(artifactsFile)
			if err != nil {
				return nil, err
			}
			return file, nil
		},
	}

	options := ArtifactsOptions{
		BaseName:           filepath.Base(artifactsFile),
		Format:             artifactFormat,
		Type:               artifactType,
		LogResponseDetails: logResponseDetails,
	}
	return client.UploadRawArtifacts(config, bodyProvider, options)
}

func TestArtifactsUpload(t *testing.T) {
	defaultConfig := JobCredentials{ID: 10, Token: "token"}
	invalidToken := JobCredentials{ID: 10, Token: "invalid-token"}
	redirectToken := JobCredentials{ID: 10, Token: "redirect"}

	isLogMessage := func(t *testing.T, l *logrus.Entry, msgRE string, level logrus.Level) {
		assert.Regexp(t, msgRE, l.Message)
		assert.Equal(t, level, l.Level)
	}

	isResponseBodyLog := func(t *testing.T, l *logrus.Entry) {
		isLogMessage(t, l, "received response", logrus.WarnLevel)
		assert.Contains(t, l.Data, "body")
		assert.Contains(t, l.Data, "header[Content-Length]")
		assert.Contains(t, l.Data, "header[Date]")
	}

	tests := map[string]struct {
		content           []byte
		config            JobCredentials
		artifactType      string
		artifactFormat    ArtifactFormat
		overwriteFileName string

		expectedUploadState UploadState
		expectedLocation    string
		verifyLogs          func(*testing.T, bool, *logHook)
	}{
		"default": {
			content: []byte("content"),
			config:  defaultConfig,
			verifyLogs: func(t *testing.T, logResponseDetail bool, logs *logHook) {
				i := 0
				if logResponseDetail {
					isResponseBodyLog(t, logs.entries[i])
					i += 1
				}
				isLogMessage(t, logs.entries[i], "Uploading artifacts to coordinator... 201 Created", logrus.InfoLevel)
			},
		},
		"too large": {
			content:             []byte("too-large"),
			config:              defaultConfig,
			expectedUploadState: UploadTooLarge,
			verifyLogs: func(t *testing.T, logResponseDetail bool, logs *logHook) {
				i := 0
				if logResponseDetail {
					isResponseBodyLog(t, logs.entries[i])
					i += 1
				}
				isLogMessage(t, logs.entries[i], "Uploading artifacts to coordinator... 413 Request Entity Too Large", logrus.ErrorLevel)
			},
		},
		"zip": {
			content:        []byte("zip"),
			config:         defaultConfig,
			artifactFormat: ArtifactFormatZip,
			verifyLogs: func(t *testing.T, logResponseDetail bool, logs *logHook) {
				i := 0
				if logResponseDetail {
					isResponseBodyLog(t, logs.entries[i])
					i += 1
				}
				isLogMessage(t, logs.entries[i], "Uploading artifacts to coordinator... 201 Created", logrus.InfoLevel)
			},
		},
		"gzip": {
			content:        []byte("gzip"),
			config:         defaultConfig,
			artifactFormat: ArtifactFormatGzip,
			verifyLogs: func(t *testing.T, logResponseDetail bool, logs *logHook) {
				i := 0
				if logResponseDetail {
					isResponseBodyLog(t, logs.entries[i])
					i += 1
				}
				isLogMessage(t, logs.entries[i], "Uploading artifacts to coordinator... 201 Created", logrus.InfoLevel)
			},
		},
		"junit": {
			content:        []byte("junit"),
			config:         defaultConfig,
			artifactType:   "junit",
			artifactFormat: ArtifactFormatGzip,
			verifyLogs: func(t *testing.T, logResponseDetail bool, logs *logHook) {
				i := 0
				if logResponseDetail {
					isResponseBodyLog(t, logs.entries[i])
					i += 1
				}
				isLogMessage(t, logs.entries[i], "Uploading artifacts as \"junit\" to coordinator... 201 Created", logrus.InfoLevel)
			},
		},
		"non-existing-file": {
			config:              defaultConfig,
			overwriteFileName:   "not/existing/file",
			expectedUploadState: UploadFailed,
			verifyLogs: func(t *testing.T, _ bool, logs *logHook) {
				// we don't even do a request, thus there is no response
				assert.Len(t, logs.entries, 0, "expected no logs")
			},
		},
		"invalid-token": {
			config:              invalidToken,
			expectedUploadState: UploadForbidden,
			verifyLogs: func(t *testing.T, logResponseDetail bool, logs *logHook) {
				i := 0
				if logResponseDetail {
					isResponseBodyLog(t, logs.entries[i])
					i += 1
				}
				isLogMessage(t, logs.entries[i], "Uploading artifacts to coordinator... 403 Forbidden", logrus.ErrorLevel)
			},
		},
		"service-unavailable": {
			content:             []byte("service-unavailable"),
			config:              defaultConfig,
			expectedUploadState: UploadServiceUnavailable,
			verifyLogs: func(t *testing.T, logResponseDetail bool, logs *logHook) {
				i := 0
				if logResponseDetail {
					isResponseBodyLog(t, logs.entries[i])
					i += 1
				}
				isLogMessage(t, logs.entries[i], "Uploading artifacts to coordinator... 503 Service Unavailable", logrus.ErrorLevel)
			},
		},
		"bad-request": {
			content:             []byte("bad-request"),
			config:              defaultConfig,
			expectedUploadState: UploadFailed,
			verifyLogs: func(t *testing.T, logResponseDetail bool, logs *logHook) {
				i := 0
				if logResponseDetail {
					isResponseBodyLog(t, logs.entries[i])
					i += 1
				}
				isLogMessage(t, logs.entries[i], "Uploading artifacts to coordinator... POST .*: 400 Bad Request \\(duplicate variables\\)", logrus.WarnLevel)
			},
		},
		"bad-request-not-json": {
			content:             []byte("bad-request-not-json"),
			config:              defaultConfig,
			expectedUploadState: UploadFailed,
			verifyLogs: func(t *testing.T, logResponseDetail bool, logs *logHook) {
				i := 0
				if logResponseDetail {
					isResponseBodyLog(t, logs.entries[i])
					i += 1
				}
				isLogMessage(t, logs.entries[i], "Uploading artifacts to coordinator... 400 Bad Request", logrus.WarnLevel)
			},
		},
		// redirects are handled transparently with the use of http.Request's GetBody()
		"redirect": {
			content:             []byte("content"),
			config:              redirectToken,
			expectedUploadState: UploadSucceeded,
			artifactFormat:      ArtifactFormatZip,
			verifyLogs: func(t *testing.T, logResponseDetail bool, logs *logHook) {
				i := 0
				if logResponseDetail {
					isResponseBodyLog(t, logs.entries[i])
					i += 1
				}
				isLogMessage(t, logs.entries[i], "Uploading artifacts to coordinator... 201 Created", logrus.InfoLevel)
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			for _, withRespDetails := range []bool{false, true} {
				t.Run(fmt.Sprintf("withResponseDetails:%t", withRespDetails), func(t *testing.T) {
					handler := func(w http.ResponseWriter, r *http.Request) {
						testArtifactsUploadHandler(w, r, t)
					}

					s := httptest.NewServer(http.HandlerFunc(handler))
					defer s.Close()

					tempFile, err := os.CreateTemp("", "artifacts")
					assert.NoError(t, err)
					tempFile.Close()
					defer os.Remove(tempFile.Name())

					c := NewGitLabClient()

					logs := newLogHook(logrus.InfoLevel, logrus.WarnLevel, logrus.ErrorLevel)
					logrus.AddHook(&logs)

					if test.content != nil {
						require.NoError(t, os.WriteFile(tempFile.Name(), test.content, 0o600))
					}

					config := test.config
					config.URL = s.URL

					filename := cmp.Or(test.overwriteFileName, tempFile.Name())

					state, location := uploadArtifacts(c, config, filename, test.artifactType, test.artifactFormat, withRespDetails)
					assert.Equal(t, test.expectedUploadState, state, "wrong upload state")
					assert.Equal(t, test.expectedLocation, location, "wrong location")

					test.verifyLogs(t, withRespDetails, &logs)
				})
			}
		})
	}
}

func checkTestArtifactsDownloadHandlerContent(w http.ResponseWriter, token string) {
	cases := map[string]struct {
		statusCode  int
		body        string
		contentType string
	}{
		"token": {
			statusCode:  http.StatusOK,
			body:        "Artifact: direct_download=false",
			contentType: "text/plain",
		},
		"object-storage-forbidden": {
			statusCode: http.StatusForbidden,
			body: `<?xml version="1.0" encoding="UTF-8"?>
<Error>
<Code>SecurityPolicyViolated</Code>
<Message>Request violates VPC Service Controls</Message>
</Error>`,
			contentType: "application/xml",
		},
		"object-storage-forbidden-json": {
			statusCode:  http.StatusForbidden,
			body:        `{"message": "not allowed"}`,
			contentType: "application/json",
		},
		"object-storage-bad-xml": {
			statusCode:  http.StatusForbidden,
			body:        "This is not XML",
			contentType: "application/xml",
		},
		"object-storage-no-code-in-xml": {
			statusCode: http.StatusForbidden,
			body: `<?xml version="1.0" encoding="UTF-8"?>
<Test>Hello</Test>`,
			contentType: "text/xml",
		},
	}

	testCase, ok := cases[token]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.Header().Set(ContentType, testCase.contentType)
	w.WriteHeader(testCase.statusCode)
	_, _ = w.Write([]byte(testCase.body))
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

	token := r.Header.Get(JobToken)
	if token == "invalid-token" {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	if token == "unauthorized-token" {
		w.WriteHeader(http.StatusUnauthorized)
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
	}

	checkTestArtifactsDownloadHandlerContent(w, token)
}

type nopWriteCloser struct {
	w io.Writer
}

func (wc *nopWriteCloser) Write(p []byte) (int, error) {
	return wc.w.Write(p)
}

func (wc *nopWriteCloser) Close() error {
	return nil
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
	unauthorizedTokenCredentials := JobCredentials{
		ID:    10,
		URL:   s.URL,
		Token: "unauthorized-token",
	}
	fileNotFoundTokenCredentials := JobCredentials{
		ID:    11,
		URL:   s.URL,
		Token: "token",
	}
	objectStorageForbiddenCredentials := JobCredentials{
		ID:    10,
		URL:   s.URL,
		Token: "object-storage-forbidden",
	}
	objectStorageForbiddenJSONCredentials := JobCredentials{
		ID:    10,
		URL:   s.URL,
		Token: "object-storage-forbidden-json",
	}
	objectStorageForbiddenBadXMLCredentials := JobCredentials{
		ID:    10,
		URL:   s.URL,
		Token: "object-storage-bad-xml",
	}
	objectStorageForbiddenNoCodeInXMLCredentials := JobCredentials{
		ID:    10,
		URL:   s.URL,
		Token: "object-storage-no-code-in-xml",
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
		"unauthorized should be generated for unauthorized credentials": {
			credentials:    unauthorizedTokenCredentials,
			directDownload: &trueValue,
			expectedState:  DownloadUnauthorized,
		},
		"file should not be downloaded if not existing": {
			credentials:    fileNotFoundTokenCredentials,
			directDownload: &trueValue,
			expectedState:  DownloadNotFound,
		},
		"forbidden should be generated for object storage forbidden error": {
			credentials:    objectStorageForbiddenCredentials,
			directDownload: &falseValue,
			expectedState:  DownloadForbidden,
		},
		"forbidden should be generated for object storage forbidden with bad JSON error": {
			credentials:    objectStorageForbiddenJSONCredentials,
			directDownload: &falseValue,
			expectedState:  DownloadForbidden,
		},
		"forbidden should be generated for object storage forbidden with bad XML error": {
			credentials:    objectStorageForbiddenBadXMLCredentials,
			directDownload: &falseValue,
			expectedState:  DownloadForbidden,
		},
		"forbidden should be generated for object storage forbidden with no error code in XML": {
			credentials:    objectStorageForbiddenNoCodeInXMLCredentials,
			directDownload: &falseValue,
			expectedState:  DownloadForbidden,
		},
	}

	for tn, tc := range testCases {
		t.Run(tn, func(t *testing.T) {
			c := NewGitLabClient()

			tempDir := t.TempDir()

			artifactsFileName := filepath.Join(tempDir, "downloaded-artifact")
			file, err := os.Create(artifactsFileName)
			require.NoError(t, err)
			defer file.Close()

			buf := bufio.NewWriter(file)

			state := c.DownloadArtifacts(tc.credentials, &nopWriteCloser{w: buf}, tc.directDownload)
			require.Equal(t, tc.expectedState, state)

			if tc.expectedArtifact == "" {
				return
			}

			err = buf.Flush()
			require.NoError(t, err)

			artifact, err := os.ReadFile(artifactsFileName)

			assert.NoError(t, err)
			assert.Equal(t, string(artifact), tc.expectedArtifact)
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
	executorProvider := NewMockExecutorProvider(t)
	executorProvider.On("GetDefaultShell").Return("my-default-executor-shell").Twice()
	executorProvider.On("CanCreate").Return(true).Once()
	executorProvider.On("GetFeatures", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		features := args[0].(*FeaturesInfo)
		features.Shared = true
	})
	executorProvider.On("GetConfigInfo", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		info := args[1].(*ConfigInfo)
		info.Gpus = "all"
	})
	RegisterExecutorProviderForTest(t, "my-test-executor", executorProvider)

	shell := NewMockShell(t)
	shell.On("GetName").Return("my-default-executor-shell")
	shell.On("GetFeatures", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		features := args[0].(*FeaturesInfo)
		features.Variables = true
	})
	RegisterShell(shell)

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
	assert.Equal(t, "all", info.Config.Gpus)
}
