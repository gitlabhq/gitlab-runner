package buildtest

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
)

const testTimeout = 30 * time.Minute

type BuildSetupFn func(t *testing.T, build *common.Build)

func RunBuildReturningOutput(t *testing.T, build *common.Build) (string, error) {
	buf := new(bytes.Buffer)
	err := RunBuildWithTrace(t, build, &common.Trace{Writer: buf})
	output := buf.String()
	t.Log(output)

	return output, err
}

func RunBuildWithTrace(t *testing.T, build *common.Build, trace *common.Trace) error {
	return RunBuildWithOptions(t, build, trace, &common.Config{})
}

func RunBuildWithOptions(t *testing.T, build *common.Build, trace *common.Trace, config *common.Config) error {
	timeoutTimer := time.AfterFunc(testTimeout, func() {
		t.Log("Timed out")
		t.FailNow()
	})
	defer timeoutTimer.Stop()

	return build.Run(t.Context(), config, trace)
}

func RunBuild(t *testing.T, build *common.Build) error {
	err := RunBuildWithTrace(t, build, &common.Trace{Writer: os.Stdout})

	return err
}

// OnStage executes the provided function when CurrentStage() enters a stage
// whose name starts with the provided prefix. Thin wrapper over onAnyStage
// for the single-prefix case.
func OnStage(build *common.Build, prefix string, fn func()) func() {
	return onAnyStage(build, []string{prefix}, fn)
}

// onAnyStage executes the provided function when CurrentStage() enters a
// stage whose name starts with any of the provided prefixes. Polling-based;
// returns a cleanup function the caller must invoke (typically via defer)
// to stop the polling goroutine if the matching stage is never reached.
func onAnyStage(build *common.Build, prefixes []string, fn func()) func() {
	exit := make(chan struct{})

	inStage := func() bool {
		currentStage := string(build.CurrentStage())
		for _, p := range prefixes {
			if strings.HasPrefix(currentStage, p) {
				fn()
				return true
			}
		}
		return false
	}
	ticker := time.NewTicker(time.Millisecond * 200)

	go func() {
		defer ticker.Stop()

		for {
			if inStage() {
				return
			}

			select {
			case <-exit:
				return
			case <-ticker.C:
			}
		}
	}()

	return func() {
		close(exit)
	}
}

// OnUserStage executes the provided function when the CurrentStage() enters
// a non-predefined stage. Matches both the classic `step_<name>` stages
// emitted by the legacy/attach executors and the `concrete` stage emitted
// by native-steps dispatch (see common/build.go:executeStepStage).
func OnUserStage(build *common.Build, fn func()) func() {
	return onAnyStage(build, []string{"step_", "concrete"}, fn)
}

// onAnyStageWhen is onAnyStage with an additional readiness predicate.
// Fires fn only when CurrentStage() matches one of the prefixes AND
// readyFn returns true. Polling continues until both conditions are met
// or ctx is cancelled; if ctx expires first, fn does not run.
//
// Useful when the stage transition is observable before the resources
// the callback needs are observable (e.g., FF_CONCRETE dispatch sets the
// `concrete` stage before the build pod has been created in K8s).
func onAnyStageWhen(
	ctx context.Context,
	build *common.Build,
	prefixes []string,
	readyFn func() bool,
	fn func(),
) func() {
	exit := make(chan struct{})

	stageMatches := func() bool {
		currentStage := string(build.CurrentStage())
		for _, p := range prefixes {
			if strings.HasPrefix(currentStage, p) {
				return true
			}
		}
		return false
	}

	ticker := time.NewTicker(time.Millisecond * 200)

	go func() {
		defer ticker.Stop()

		for {
			if stageMatches() && readyFn() {
				fn()
				return
			}

			select {
			case <-ctx.Done():
				return
			case <-exit:
				return
			case <-ticker.C:
			}
		}
	}()

	return func() {
		close(exit)
	}
}

// OnUserStageWhen is OnUserStage with an additional readiness predicate
// and context-driven timeout. See onAnyStageWhen for semantics.
func OnUserStageWhen(ctx context.Context, build *common.Build, readyFn func() bool, fn func()) func() {
	return onAnyStageWhen(ctx, build, []string{"step_", "concrete"}, readyFn, fn)
}

func SetBuildFeatureFlag(build *common.Build, flag string, value bool) {
	for _, v := range build.Variables {
		if v.Key == flag {
			v.Value = fmt.Sprint(value)
			return
		}
	}

	build.Variables = append(build.Variables, spec.Variable{
		Key:   flag,
		Value: fmt.Sprint(value),
	})
}

type baseJobGetter func() (spec.Job, error)

// getJobResponseWithCommands is a wrapper that will decorate a JobResponse getter
// like common.GetRemoteSuccessfulBuild with a custom commands list
func getJobResponseWithCommands(t *testing.T, baseJobGetter baseJobGetter, commands ...string) spec.Job {
	jobResponse, err := baseJobGetter()
	require.NoError(t, err)

	jobResponse.Steps[0].Script = commands

	return jobResponse
}

// WithFeatureFlags runs a subtest for the on/off value for each flag provided,
// and allows a build object as part of the test to be decorated with the
// feature flag variable.
func WithEachFeatureFlag(t *testing.T, f func(t *testing.T, setup BuildSetupFn), flags ...string) {
	if len(flags) == 0 {
		t.Log("WithEachFeatureFlag: no feature flags provided. Running inner test with no feature flags.")
		f(t, func(t *testing.T, build *common.Build) {})
		return
	}

	for _, flag := range flags {
		for _, value := range []bool{false, true} {
			t.Run(fmt.Sprintf("%v=%v", flag, value), func(t *testing.T) {
				f(t, func(t *testing.T, build *common.Build) {
					SetBuildFeatureFlag(build, flag, value)
				})
			})
		}
	}
}

// injectJobToken injects a job token into an existing jobResponse by
// - setting the jobResponse's token
// - updating the jobResponse's gitInfo with an URL with the token
// - injecting a CI_JOB_TOKEN jobVariable
// It returns the new repo URL with the injected token.
func injectJobToken(t *testing.T, jobResponse *spec.Job, token string) *url.URL {
	repoURLWithToken := func(orgRepoURL, token string) *url.URL {
		u, err := url.Parse(orgRepoURL)
		require.NoError(t, err, "parsing original repo URL")
		u.User = url.UserPassword("gitlab-ci-token", token)
		return u
	}(jobResponse.GitInfo.RepoURL, token)

	jobResponse.Variables.Set(spec.Variable{Key: "CI_JOB_TOKEN", Value: token, Masked: true})

	jobResponse.Token = token
	jobResponse.GitInfo.RepoURL = repoURLWithToken.String()

	return repoURLWithToken
}

// InjectJobTokenFromEnv injects a job token from the environment into an existing jobResponse.
// It returns the token value and the new repo URL with the injected token.
func InjectJobTokenFromEnv(t *testing.T, jobResponse *spec.Job, envVars ...string) (string, *url.URL) {
	if len(envVars) == 0 {
		envVars = []string{"GITLAB_TEST_TOKEN", "CI_JOB_TOKEN", "OUTER_CI_JOB_TOKEN"}
	}

	var token string
	for _, envVar := range envVars {
		if tok, ok := os.LookupEnv(envVar); ok {
			t.Log("using token from env var", envVar)
			token = tok
			break
		}
	}
	if token == "" {
		t.Fatalf("no token available, considered env vars: %q", envVars)
	}

	u := injectJobToken(t, jobResponse, token)
	return token, u
}
