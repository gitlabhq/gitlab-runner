package buildtest

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func RunBuildWithCleanupGitClone(t *testing.T, build *common.Build) {
	build.Variables = append(
		build.Variables,
		common.JobVariable{Key: "GIT_STRATEGY", Value: "clone"},
		common.JobVariable{Key: "FF_ENABLE_JOB_CLEANUP", Value: "true"},
	)
	out, err := RunBuildReturningOutput(t, build)
	assert.NoError(t, err)
	assert.Contains(t, out, "Cleaning up project directory and file based variables")
}

func RunBuildWithCleanupGitFetch(t *testing.T, build *common.Build, untrackedFilename string) {
	build.Variables = append(
		build.Variables,
		common.JobVariable{Key: "GIT_STRATEGY", Value: "fetch"},
		common.JobVariable{Key: "FF_ENABLE_JOB_CLEANUP", Value: "true"},
	)

	out, err := RunBuildReturningOutput(t, build)
	assert.NoError(t, err)
	assert.Contains(t, out, "Cleaning up project directory and file based variables")
	assert.Contains(t, out, fmt.Sprintf("Removing %s", untrackedFilename))
}

func RunBuildWithCleanupNormalSubmoduleStrategy(
	t *testing.T,
	build *common.Build,
	untrackedFileName,
	untrackedFileInSubmodule string,
) {
	build.Variables = append(
		build.Variables,
		common.JobVariable{Key: "GIT_STRATEGY", Value: "fetch"},
		common.JobVariable{Key: "GIT_SUBMODULE_STRATEGY", Value: "normal"},
		common.JobVariable{Key: "FF_ENABLE_JOB_CLEANUP", Value: "true"},
	)

	out, err := RunBuildReturningOutput(t, build)
	assert.NoError(t, err)

	assert.Contains(t, out, "Cleaning up project directory and file based variables")
	assert.Contains(t, out, fmt.Sprintf("Removing %s", untrackedFileName))
	assert.Contains(t, out, fmt.Sprintf("Removing %s", untrackedFileInSubmodule))
}

func RunBuildWithCleanupRecursiveSubmoduleStrategy(
	t *testing.T,
	build *common.Build,
	untrackedFileName,
	untrackedFileInSubmodule,
	untrackedFileInSubSubmodule string,
) {
	build.Variables = append(
		build.Variables,
		common.JobVariable{Key: "GIT_STRATEGY", Value: "fetch"},
		common.JobVariable{Key: "GIT_SUBMODULE_STRATEGY", Value: "recursive"},
		common.JobVariable{Key: "FF_ENABLE_JOB_CLEANUP", Value: "true"},
	)

	out, err := RunBuildReturningOutput(t, build)
	assert.NoError(t, err)

	assert.Contains(t, out, "Cleaning up project directory and file based variables")
	assert.Contains(t, out, fmt.Sprintf("Removing %s", untrackedFileName))
	assert.Contains(t, out, fmt.Sprintf("Removing %s", untrackedFileInSubmodule))
	assert.Contains(t, out, fmt.Sprintf("Removing %s", untrackedFileInSubSubmodule))
}

func GetNewUntrackedFileIntoSubmodulesCommands(
	untrackedFile,
	untrackedFileInSubmodule,
	untrackedFileInSubSubmodule string,
) []string {
	var untrackedFilesResult []string
	if untrackedFile != "" {
		untrackedFilesResult = append(
			untrackedFilesResult,
			fmt.Sprintf("echo 'this is an untracked file' >> %s", untrackedFile),
		)
	}
	if untrackedFileInSubmodule != "" {
		untrackedFilesResult = append(
			untrackedFilesResult,
			fmt.Sprintf(
				"echo 'this is an untracked file in the submodule' >> gitlab-grack/%s",
				untrackedFileInSubmodule,
			))
	}
	if untrackedFileInSubSubmodule != "" {
		untrackedFilesResult = append(
			untrackedFilesResult,
			fmt.Sprintf(
				"echo 'this is an untracked file in the sub-submodule' >> gitlab-grack/tests/example/%s",
				untrackedFileInSubSubmodule,
			))
	}
	return untrackedFilesResult
}
