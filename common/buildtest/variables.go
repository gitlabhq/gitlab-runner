package buildtest

import (
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func RunBuildWithExpandedFileVariable(t *testing.T, config *common.RunnerConfig, setup BuildSetupFn) {
	resp, err := common.GetRemoteSuccessfulBuildPrintVars(
		config.Shell,
		"MY_FILE_VARIABLE",
		"MY_EXPANDED_FILE_VARIABLE",
		"RUNNER_TEMP_PROJECT_DIR",
	)
	require.NoError(t, err)

	build := &common.Build{
		JobResponse: resp,
		Runner:      config,
	}

	build.Variables = append(
		build.Variables,
		common.JobVariable{Key: "MY_FILE_VARIABLE", Value: "FILE_CONTENTS", File: true},
		common.JobVariable{Key: "MY_EXPANDED_FILE_VARIABLE", Value: "${MY_FILE_VARIABLE}_FOOBAR"},
	)

	if setup != nil {
		setup(build)
	}

	out, err := RunBuildReturningOutput(t, build)
	require.NoError(t, err)

	matches := regexp.MustCompile(`RUNNER_TEMP_PROJECT_DIR=([^\$%].*)`).FindStringSubmatch(out)
	require.Equal(t, 2, len(matches))

	assert.NotRegexp(t, "MY_EXPANDED_FILE_VARIABLE=.*FILE_CONTENTS_FOOBAR", out)

	if runtime.GOOS == "windows" {
		tmpPath := strings.TrimRight(matches[1], "\r")
		assert.Contains(t, out, "MY_EXPANDED_FILE_VARIABLE="+tmpPath+"\\MY_FILE_VARIABLE_FOOBAR")
	} else {
		assert.Contains(t, out, "MY_EXPANDED_FILE_VARIABLE="+matches[1]+"/MY_FILE_VARIABLE_FOOBAR")
	}
}
