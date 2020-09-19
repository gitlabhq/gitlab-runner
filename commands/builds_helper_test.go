package commands

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildtest"
	"gitlab.com/gitlab-org/gitlab-runner/session"
)

func TestBuildsHelperCollect(t *testing.T) {
	dir, err := ioutil.TempDir("", "gitlab-runner-helper-collector")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	ch := make(chan prometheus.Metric, 50)
	b := newBuildsHelper()

	longRunningBuild, err := common.GetLongRunningBuild()
	require.NoError(t, err)

	shell := "bash"
	if runtime.GOOS == "windows" {
		shell = "powershell"
	}

	build := &common.Build{
		JobResponse: longRunningBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				BuildsDir: dir,
				Executor:  "shell",
				Shell:     shell,
			},
		},
	}
	trace := &common.Trace{Writer: ioutil.Discard}

	done := make(chan error)
	go func() {
		done <- buildtest.RunBuildWithTrace(t, build, trace)
	}()

	b.builds = append(b.builds, build)
	// collect many logs whilst the build is being executed to trigger any
	// potential race conditions that arise from the build progressing whilst
	// metrics are collected.
	for i := 0; i < 200; i++ {
		if i == 100 {
			// Build might have not started yet, wait until cancel is
			// successful.
			require.Eventually(
				t,
				func() bool {
					return trace.Abort()
				},
				time.Minute,
				10*time.Millisecond,
			)
		}
		b.Collect(ch)
		<-ch
	}

	err = <-done
	expected := &common.BuildError{FailureReason: common.JobCanceled}
	assert.True(t, errors.Is(err, expected), "expected: %[1]T (%[1]v), got: %[2]T (%[2]v)", expected, err)
}

func TestBuildsHelperAcquireRequestWithLimit(t *testing.T) {
	runner := common.RunnerConfig{
		RequestConcurrency: 2,
	}

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
	}

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
		Limit: 1,
	}

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
		Limit: 0,
	}

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
		},
	}

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
				Runner: &common.RunnerConfig{},
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

			resp := writer.Result()
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, "2", resp.Header.Get("X-List-Version"))
			assert.Equal(t, "text/plain", resp.Header.Get("Content-Type"))

			body, err := ioutil.ReadAll(resp.Body)
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

func TestCreateJobURL(t *testing.T) {
	//nolint:lll
	testCases := map[string]string{
		"http://gitlab.example.com/my-namespace/my-project.git":     "http://gitlab.example.com/my-namespace/my-project/-/jobs/1",
		"http://gitlab.example.com/my-namespace/my-project":         "http://gitlab.example.com/my-namespace/my-project/-/jobs/1",
		"http://gitlab.example.com/my-namespace/my.git.project.git": "http://gitlab.example.com/my-namespace/my.git.project/-/jobs/1",
		"http://gitlab.example.com/my-namespace/my.git.project":     "http://gitlab.example.com/my-namespace/my.git.project/-/jobs/1",
	}

	for URL, expectedURL := range testCases {
		jobURL := CreateJobURL(URL, 1)
		assert.Equal(t, expectedURL, jobURL)
	}
}
