//go:build !integration

package commands

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/session"
)

func TestBuildsHelperAcquireRequestWithLimit(t *testing.T) {
	runner := common.RunnerConfig{
		RequestConcurrency: 2,
		SystemIDState:      common.NewSystemIDState(),
	}

	require.NoError(t, runner.SystemIDState.EnsureSystemID())

	b := newBuildsHelper()
	result := b.acquireRequest(&runner)
	require.True(t, result)

	result = b.acquireRequest(&runner)
	require.True(t, result)

	result = b.acquireRequest(&runner)
	require.False(t, result, "allow only two requests")

	result = b.releaseRequest(&runner)
	require.True(t, result)

	result = b.releaseRequest(&runner)
	require.True(t, result)

	result = b.releaseRequest(&runner)
	require.False(t, result, "release only two requests")
}

func TestBuildsHelperAcquireRequestWithDefault(t *testing.T) {
	runner := common.RunnerConfig{
		RequestConcurrency: 0,
		SystemIDState:      common.NewSystemIDState(),
	}

	require.NoError(t, runner.SystemIDState.EnsureSystemID())

	b := newBuildsHelper()
	result := b.acquireRequest(&runner)
	require.True(t, result)

	result = b.acquireRequest(&runner)
	require.False(t, result, "allow only one request")

	result = b.releaseRequest(&runner)
	require.True(t, result)

	result = b.releaseRequest(&runner)
	require.False(t, result, "release only one request")

	result = b.acquireRequest(&runner)
	require.True(t, result)

	result = b.releaseRequest(&runner)
	require.True(t, result)

	result = b.releaseRequest(&runner)
	require.False(t, result, "nothing to release")
}

func TestBuildsHelperAcquireBuildWithLimit(t *testing.T) {
	runner := common.RunnerConfig{
		Limit:         1,
		SystemIDState: common.NewSystemIDState(),
	}

	require.NoError(t, runner.SystemIDState.EnsureSystemID())

	b := newBuildsHelper()
	result := b.acquireBuild(&runner)
	require.True(t, result)

	result = b.acquireBuild(&runner)
	require.False(t, result, "allow only one build")

	result = b.releaseBuild(&runner)
	require.True(t, result)

	result = b.releaseBuild(&runner)
	require.False(t, result, "release only one build")
}

func TestBuildsHelperAcquireBuildUnlimited(t *testing.T) {
	runner := common.RunnerConfig{
		Limit:         0,
		SystemIDState: common.NewSystemIDState(),
	}

	require.NoError(t, runner.SystemIDState.EnsureSystemID())

	b := newBuildsHelper()
	result := b.acquireBuild(&runner)
	require.True(t, result)

	result = b.acquireBuild(&runner)
	require.True(t, result)

	result = b.releaseBuild(&runner)
	require.True(t, result)

	result = b.releaseBuild(&runner)
	require.True(t, result)
}

func TestBuildsHelperFindSessionByURL(t *testing.T) {
	sess, err := session.NewSession(nil)
	require.NoError(t, err)
	build := common.Build{
		Session: sess,
		Runner: &common.RunnerConfig{
			RunnerCredentials: common.RunnerCredentials{
				Token: "abcd1234",
			},
			SystemIDState: common.NewSystemIDState(),
		},
	}

	require.NoError(t, build.Runner.SystemIDState.EnsureSystemID())

	h := newBuildsHelper()
	h.addBuild(&build)

	foundSession := h.findSessionByURL(sess.Endpoint + "/action")
	assert.Equal(t, sess, foundSession)

	foundSession = h.findSessionByURL("/session/hash/action")
	assert.Nil(t, foundSession)
}

func TestBuildsHelper_ListJobsHandler(t *testing.T) {
	tests := map[string]struct {
		build          *common.Build
		expectedOutput []string
	}{
		"no jobs": {
			build: nil,
		},
		"job exists": {
			build: &common.Build{
				Runner: &common.RunnerConfig{
					SystemIDState: common.NewSystemIDState(),
				},
				JobResponse: common.JobResponse{
					ID:      1,
					JobInfo: common.JobInfo{ProjectID: 1},
					GitInfo: common.GitInfo{RepoURL: "https://gitlab.example.com/my-namespace/my-project.git"},
				},
			},
			expectedOutput: []string{
				"url=https://gitlab.example.com/my-namespace/my-project/-/jobs/1",
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			writer := httptest.NewRecorder()

			req, err := http.NewRequest(http.MethodGet, "/", nil)
			require.NoError(t, err)

			b := newBuildsHelper()
			b.addBuild(test.build)
			b.ListJobsHandler(writer, req)

			if test.build != nil {
				require.NoError(t, test.build.Runner.SystemIDState.EnsureSystemID())
			}

			resp := writer.Result()
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, "2", resp.Header.Get("X-List-Version"))
			assert.Equal(t, "text/plain", resp.Header.Get("Content-Type"))

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			if len(test.expectedOutput) == 0 {
				assert.Empty(t, body)
				return
			}

			for _, expectedOutput := range test.expectedOutput {
				assert.Contains(t, string(body), expectedOutput)
			}
		})
	}
}

func TestRestrictHTTPMethods(t *testing.T) {
	tests := map[string]int{
		http.MethodGet:  http.StatusOK,
		http.MethodHead: http.StatusOK,
		http.MethodPost: http.StatusMethodNotAllowed,
		"FOOBAR":        http.StatusMethodNotAllowed,
	}

	for method, expectedStatusCode := range tests {
		t.Run(method, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("hello world"))
			})

			server := httptest.NewServer(restrictHTTPMethods(mux, http.MethodGet, http.MethodHead))

			req, err := http.NewRequest(method, server.URL, nil)
			require.NoError(t, err)

			resp, err := server.Client().Do(req)
			require.NoError(t, err)
			require.Equal(t, expectedStatusCode, resp.StatusCode)
		})
	}
}
