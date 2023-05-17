//go:build !integration

package helpers

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestGeneratePredicate(t *testing.T) {
	gen := &artifactStatementGenerator{
		RunnerID:     1001,
		RepoURL:      "testurl",
		RepoDigest:   "testdigest",
		JobName:      "testjobname",
		ExecutorName: "testexecutorname",
		RunnerName:   "testrunnername",
		Parameters:   []string{"testparam"},
	}

	jobId := "10001"

	startTime := time.Now()
	endTime := startTime.Add(time.Minute)

	originalVersion := common.AppVersion.Version
	testVersion := "vTest"
	common.AppVersion.Version = testVersion

	defer func() {
		common.AppVersion.Version = originalVersion
	}()

	actualPredicate := gen.generatePredicate(jobId, startTime, endTime)

	expectedBuildType := fmt.Sprintf(attestationTypeFormat, testVersion)
	assert.Equal(t, expectedBuildType, actualPredicate.BuildDefinition.BuildType)

}
