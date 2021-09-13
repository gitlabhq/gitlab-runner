//go:build integration
// +build integration

package commands

import (
	"io/ioutil"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildtest"
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
	assert.ErrorIs(t, err, expected)
}
