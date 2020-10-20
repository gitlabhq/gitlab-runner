package buildtest

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

const testTimeout = 30 * time.Minute

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

	return build.Run(config, trace)
}

func RunBuild(t *testing.T, build *common.Build) error {
	err := RunBuildWithTrace(t, build, &common.Trace{Writer: os.Stdout})

	return err
}

// OnUserStage executes the provided function when the CurrentStage() enters
// a non-predefined stage.
func OnUserStage(build *common.Build, fn func()) func() {
	exit := make(chan struct{})

	go func() {
		for {
			select {
			case <-exit:
				return

			case <-time.After(time.Second):
				if strings.HasPrefix(string(build.CurrentStage()), "step_") {
					fn()
					return
				}
			}
		}
	}()

	return func() {
		close(exit)
	}
}
