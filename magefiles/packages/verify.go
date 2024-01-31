package packages

import (
	"fmt"
	"strconv"

	"gitlab.com/gitlab-org/gitlab-runner/magefiles/build"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/mageutils"
)

const iterationVar = "PACKAGES_ITERATION"

var (
	errInvalidIteration = fmt.Errorf("PACKAGES_ITERATION is invalid")
	errIterationNotSet  = fmt.Errorf("PACKAGES_ITERATION is not set")
	errIterationMain    = fmt.Errorf("PACKAGES_ITERATION can only be set to '1' on the main branch")
)

// VerifyIterationVariable verifies that the PACKAGES_ITERATION variable is set correctly.
// see more in magefiles/package.go
func VerifyIterationVariable() error {
	iteration := mageutils.Env(iterationVar)
	if iteration == "" {
		return errIterationNotSet
	}

	iterationNum, err := strconv.ParseInt(iteration, 10, 64)
	if err != nil {
		return errInvalidIteration
	}

	if iterationNum <= 0 {
		return errInvalidIteration
	}

	if iterationNum != 1 && build.IsMainBranch() {
		return errIterationMain
	}

	return nil
}
