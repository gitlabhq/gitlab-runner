package buildtest

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

const TestTimeout = 60 * time.Second

func RunBuildReturningOutput(t *testing.T, build *common.Build) (string, error) {
	var buf bytes.Buffer
	err := RunBuildWithTrace(t, build, &common.Trace{Writer: &buf})
	output := buf.String()
	t.Log(output)

	return output, err
}

func RunBuildWithTrace(t *testing.T, build *common.Build, trace *common.Trace) error {
	return RunBuildWithOptions(t, build, &common.Config{}, trace)
}

func RunBuildWithOptions(t *testing.T, build *common.Build, config *common.Config, trace *common.Trace) error {
	timeoutTimer := time.AfterFunc(TestTimeout, func() {
		t.Log("Timed out")
		t.FailNow()
	})
	defer timeoutTimer.Stop()

	return build.Run(config, trace)
}

func RunBuild(t *testing.T, build *common.Build) error {
	err := RunBuildWithTrace(t, build, &common.Trace{Writer: os.Stdout})
	assert.True(t, build.IsSharedEnv())

	return err
}
