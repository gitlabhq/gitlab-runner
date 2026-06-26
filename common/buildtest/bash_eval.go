package buildtest

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

// These helpers cover the bash `eval` cancellation fix from issue #39005 /
// !6784, which replaced the `: | eval '<body>'` pipeline with a trapped
// subshell `(trap 'exit 1' TERM; eval '<body>') < /dev/null`.
//
//   - RunBuildWithStdinClosed proves the new form preserves the stdin
//     semantics the pipeline provided (deterministic, runs on any bash).
//   - RunBuildWithScriptBodyLeakOnCancel proves a cancellation does not dump
//     the script body to the log. The leak is bash-version-specific runtime
//     behavior, so this only has teeth on an executor pinned to an affected
//     bash (>= 5.3 or < 4.4) — see the docker executor's bash:5.3 wiring.

// RunBuildWithStdinClosed verifies two properties of the generated bash script:
//
//  1. The user script receives EOF on stdin. It must not consume the runner's
//     script-delivery stream nor block waiting for input. This is the property
//     the historical `: | eval` pipeline provided and that the trapped-subshell
//     `(...) < /dev/null` form must preserve. See issue #39005 / !6784.
//  2. Script delivery still succeeds on this executor. The Kubernetes legacy
//     attach strategy and the instance executor feed the script body over the
//     process stdin (see executors/instance/instance.go); redirecting the eval
//     subshell's stdin must not break that. This is the regression that stalled
//     !5821.
//
// Asserted for both the default form and FF_USE_LEGACY_BASH_EVAL=true so the two
// forms are proven to have identical stdin semantics.
//
// The behavior is bash-specific — PowerShell builds a differently structured
// script with no eval pipeline — so the check is skipped for non-bash shells.
func RunBuildWithStdinClosed(t *testing.T, config *common.RunnerConfig, setup BuildSetupFn) {
	if config.Shell != "" && config.Shell != "bash" { //nolint:goconst
		t.Skipf("stdin-closed behavior is bash-specific; skipping shell %q", config.Shell)
	}

	WithEachFeatureFlag(t, func(t *testing.T, ffSetup BuildSetupFn) {
		resp := getJobResponseWithCommands(t, common.GetRemoteSuccessfulBuild,
			// `read -t 5` returns non-zero on EOF immediately; on a live stdin it
			// would consume a line (the script-delivery stream) or block. Guarded
			// by `if` so `set -o errexit` is not tripped by the non-zero return.
			//
			// The sentinel is written with shell string concatenation
			// ("STDIN""_LEAK:") so the runner's command echo — which prints the
			// literal script source — shows `STDIN""_LEAK:`, while only an
			// actually-executed leak prints the joined `STDIN_LEAK:`. The
			// assertion below therefore matches a genuine leak, not the echo.
			`if IFS= read -r -t 5 leaked; then echo "STDIN""_LEAK:[$leaked]"; else echo "stdin-eof-ok"; fi`,
			`echo done-marker`,
		)

		build := &common.Build{Job: resp, Runner: config}
		if setup != nil {
			setup(t, build)
		}
		ffSetup(t, build)

		out, err := RunBuildReturningOutput(t, build)
		require.NoError(t, err, "job must complete (delivery works, no hang)")
		assert.Contains(t, out, "stdin-eof-ok", "user script must see EOF on stdin")
		assert.Contains(t, out, "done-marker", "script must run to completion")
		assert.NotContains(t, out, "STDIN_LEAK:[", "user script must not read past its body")
	}, featureflags.UseLegacyBashEval)
}

// RunBuildWithScriptBodyLeakOnCancel verifies that cancelling a job does not
// dump the generated script body — including expanded variables — into the job
// log. See issue #39005 / !6784.
//
// When a job is cancelled the runner sends SIGTERM to the bash processes
// running the stage script (sendSIGTERMToContainerProcs / the bash.go
// cancellation script). On bash >= 5.3 (and bash < 4.4) the unfixed pipeline
// form caused the shell to print the command it was running — the whole eval
// body — to stderr, which lands in the public log. The trapped-subshell form
// turns the SIGTERM into a clean exit so no dump occurs.
//
// The leak is bash-version-specific, so this test only has teeth on an executor
// pinned to an affected bash (the docker executor wires it with a bash:5.3
// image); on an unaffected bash it still asserts the fix produces no dump. Only
// the default (fixed) form is asserted — FF_USE_LEGACY_BASH_EVAL is the known
// vulnerable escape hatch and is intentionally not exercised here.
//
// The behavior is bash-specific, so the check is skipped for non-bash shells.
func RunBuildWithScriptBodyLeakOnCancel(t *testing.T, config *common.RunnerConfig, setup BuildSetupFn) {
	if config.Shell != "" && config.Shell != "bash" { //nolint:goconst
		t.Skipf("script-body leak is bash-specific; skipping shell %q", config.Shell)
	}

	// Marker placed on a non-first line of a multi-line command. The shell
	// echoes only the first line of a collapsed multi-line command (see
	// AbstractShell.writeCommands), so the marker never appears in normal
	// output — it can only surface if the entire eval body is dumped to the log.
	const canary = "RUNNER_SCRIPT_BODY_LEAK_CANARY_e3b0c44298fc"

	resp := getJobResponseWithCommands(t, common.GetRemoteSuccessfulBuild,
		"echo build-started\n"+canary+"_marker=true\nsleep 60",
	)

	build := &common.Build{
		Job:             resp,
		Runner:          config,
		SystemInterrupt: make(chan os.Signal, 1),
	}
	// Only the user script stage matters; skip cloning so the test does not
	// depend on git being present in the (deliberately minimal) job image.
	build.Variables = append(build.Variables, spec.Variable{Key: "GIT_STRATEGY", Value: "none"})

	if setup != nil {
		setup(t, build)
	}

	buf := new(bytes.Buffer)
	trace := &common.Trace{Writer: io.MultiWriter(buf, os.Stdout)}

	// Cancel while the user script is running (mid-`sleep`); this triggers the
	// graceful SIGTERM path that historically dumped the eval body.
	done := OnUserStage(build, func() {
		// Guard the return value: if the cancel is not accepted the SIGTERM path
		// never fires and the canary assertion below would pass vacuously.
		assert.True(t, trace.Cancel(), "cancellation must be accepted to exercise the SIGTERM path")
	})
	defer done()

	err := RunBuildWithTrace(t, build, trace)
	t.Log(buf.String())

	var buildErr *common.BuildError
	require.ErrorAs(t, err, &buildErr, "cancelled build must return a BuildError")
	assert.Equal(t, common.JobCanceled, buildErr.FailureReason, "build must have been cancelled (SIGTERM path exercised)")
	assert.NotContains(t, buf.String(), canary, "cancellation must not dump the script body to the log")
}
