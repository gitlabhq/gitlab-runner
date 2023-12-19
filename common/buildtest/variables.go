package buildtest

import (
	"bytes"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/shells"
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
		setup(t, build)
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

func RunBuildWithPassingEnvsMultistep(t *testing.T, config *common.RunnerConfig, setup BuildSetupFn) {
	formatter := shellFormatter(config.Shell)

	steps := []string{formatter.PipeVar("hello=world") + formatter.EnvName("GITLAB_ENV")}
	if config.Shell == "bash" {
		steps = append(steps, `echo 'executed=$(echo "yes")' >> $GITLAB_ENV`)
	}

	resp, err := common.GetRemoteBuildResponse(steps...)
	require.NoError(t, err)

	build := &common.Build{
		JobResponse: resp,
		Runner:      config,
	}

	if runtime.GOOS == "linux" && config.Shell == shells.SNPwsh {
		build.Image.Name = common.TestPwshImage
	}

	dir := t.TempDir()
	build.Runner.RunnerSettings.BuildsDir = filepath.Join(dir, "build")
	build.Runner.RunnerSettings.CacheDir = filepath.Join(dir, "cache")
	build.Variables = append(build.Variables, common.JobVariable{
		Key:   "existing",
		Value: "existingvalue",
	})

	build.Steps = append(
		build.Steps,
		common.Step{
			Name: "custom-step",
			Script: []string{
				`echo ` + formatter.EnvName("GITLAB_ENV"),
				`echo hellovalue=` + formatter.EnvName("hello"),
				`echo executed=` + formatter.EnvName("executed"),
				formatter.PipeVar("foo=bar") + formatter.EnvName("GITLAB_ENV"),
			},
			When: common.StepWhenOnSuccess,
		},
		common.Step{
			Name: common.StepNameAfterScript,
			Script: []string{
				`echo foovalue=` + formatter.EnvName("foo"),
				`echo existing=` + formatter.EnvName("existing"),
			},
			When: common.StepWhenAlways,
		},
	)
	build.Cache = append(build.Cache, common.Cache{
		Key:    "cache",
		Paths:  common.ArtifactPaths{"unknown/path/${foo}"},
		Policy: common.CachePolicyPullPush,
	})

	if setup != nil {
		setup(t, build)
	}

	buf := new(bytes.Buffer)
	trace := &common.Trace{Writer: buf}
	assert.NoError(t, RunBuildWithTrace(t, build, trace))

	contents := buf.String()
	assert.Contains(t, contents, "existing=existingvalue")
	assert.Contains(t, contents, "hellovalue=world")
	assert.Contains(t, contents, "foovalue=bar")
	assert.Contains(t, contents, "unknown/path/bar: no matching files")
	assert.NotContains(t, contents, "executed=yes")
}

func RunBuildWithPassingEnvsJobIsolation(t *testing.T, config *common.RunnerConfig, setup BuildSetupFn) {
	dir := t.TempDir()
	run := func(response common.JobResponse) string {
		build := &common.Build{
			JobResponse: response,
			Runner:      config,
		}

		if runtime.GOOS == "linux" && config.Shell == shells.SNPwsh {
			build.Image.Name = common.TestPwshImage
		}

		dir := dir
		build.Runner.RunnerSettings.BuildsDir = filepath.Join(dir, "build")
		build.Runner.RunnerSettings.CacheDir = filepath.Join(dir, "cache")
		if setup != nil {
			setup(t, build)
		}

		buf := new(bytes.Buffer)
		trace := &common.Trace{Writer: buf}
		assert.NoError(t, RunBuildWithTrace(t, build, trace))
		return buf.String()
	}

	formatter := shellFormatter(config.Shell)

	job1, err := common.GetRemoteBuildResponse(formatter.PipeVar("job_isolation_test=not_isolated") + formatter.EnvName("GITLAB_ENV"))
	require.NoError(t, err)

	job2, err := common.GetRemoteBuildResponse(`echo job1_isolation=` + formatter.EnvName("job_isolation_test"))
	require.NoError(t, err)

	job1Output := run(job1)
	job2Output := run(job2)

	assert.Contains(t, job1Output, formatter.PipeVar("job_isolation_test=not_isolated")+formatter.EnvName("GITLAB_ENV"))
	assert.Contains(t, job2Output, "job1_isolation")
	assert.NotContains(t, job2Output, "job1_isolation=not_isolated")
}

type shellFormatter string

func (s shellFormatter) EnvName(name string) string {
	switch s {
	case shells.SNPwsh, shells.SNPowershell:
		return "$env:" + name
	default:
		return "$" + name
	}
}

func (s shellFormatter) PipeVar(variable string) string {
	return `echo '` + variable + `' >> `
}
