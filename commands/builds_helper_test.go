package commands

import (
	"bytes"
	"fmt"
	"net/http"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/session"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

var fakeRunner = &common.RunnerConfig{
	RunnerCredentials: common.RunnerCredentials{
		Token: "a1b2c3d4e5f6",
	},
}

func TestBuildsHelperCollect(t *testing.T) {
	ch := make(chan prometheus.Metric, 50)
	b := newBuildsHelper()
	b.builds = append(b.builds, &common.Build{
		CurrentState: common.BuildRunStatePending,
		CurrentStage: common.BuildStagePrepare,
		Runner:       fakeRunner,
	})
	b.Collect(ch)
	assert.Len(t, ch, 1)
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

type fakeResponseWriter struct {
	output     *bytes.Buffer
	header     http.Header
	statusCode int
}

func (w *fakeResponseWriter) Header() http.Header            { return w.header }
func (w *fakeResponseWriter) Write(data []byte) (int, error) { return w.output.Write(data) }
func (w *fakeResponseWriter) WriteHeader(statusCode int)     { w.statusCode = statusCode }

func newFakeResponseWriter() *fakeResponseWriter {
	return &fakeResponseWriter{
		output: &bytes.Buffer{},
		header: http.Header{},
	}
}

var testBuildCurrentID int

func getTestBuild() *common.Build {
	testBuildCurrentID++

	runner := common.RunnerConfig{}
	runner.Token = "a1b2c3d4"
	jobInfo := common.JobInfo{
		ProjectID: 1,
	}

	build := &common.Build{}
	build.ID = testBuildCurrentID
	build.Runner = &runner
	build.JobInfo = jobInfo
	build.GitInfo = common.GitInfo{
		RepoURL: "https://gitlab.example.com/my-namespace/my-project.git",
	}

	return build
}

type listJobsHandlerTest struct {
	build          *common.Build
	expectedOutput []string
}

func TestBuildsHelper_ListJobsHandler(t *testing.T) {
	build := getTestBuild()

	tests := map[string]listJobsHandlerTest{
		"no jobs": {
			build: nil,
		},
		"job exists": {
			build: build,
			expectedOutput: []string{
				fmt.Sprintf("url=https://gitlab.example.com/my-namespace/my-project/-/jobs/%d", build.ID),
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			writer := newFakeResponseWriter()

			req, err := http.NewRequest(http.MethodGet, "/", nil)
			require.NoError(t, err)

			b := newBuildsHelper()
			b.addBuild(test.build)
			b.ListJobsHandler(writer, req)

			assert.Equal(t, http.StatusOK, writer.statusCode)
			assert.Equal(t, writer.Header().Get("X-List-Version"), "2")
			assert.Equal(t, writer.Header().Get("Content-Type"), "text/plain")

			if len(test.expectedOutput) == 0 {
				assert.Empty(t, writer.output.String())
				return
			}

			for _, expectedOutput := range test.expectedOutput {
				assert.Contains(t, writer.output.String(), expectedOutput)
			}
		})
	}
}

func TestCreateJobURL(t *testing.T) {
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
