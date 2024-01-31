//go:build !integration

package packages

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/mageutils"
)

func TestVerifyIterationVariable(t *testing.T) {
	tests := map[string]struct {
		iteration     string
		commitBranch  string
		defaultBranch string

		expectedError error
	}{
		"iteration is not set": {
			iteration:     "",
			commitBranch:  "main",
			defaultBranch: "main",

			expectedError: errIterationNotSet,
		},
		"iteration is 1 on main": {
			iteration:     "1",
			commitBranch:  "main",
			defaultBranch: "main",

			expectedError: nil,
		},
		"iteration is not 1 on non-main": {
			iteration:     "2",
			commitBranch:  "feature",
			defaultBranch: "main",

			expectedError: nil,
		},
		"iteration is 1 on non-main": {
			iteration:     "1",
			commitBranch:  "feature",
			defaultBranch: "main",

			expectedError: nil,
		},
		"iteration is not a number": {
			iteration:     "not-a-number",
			commitBranch:  "main",
			defaultBranch: "main",

			expectedError: errInvalidIteration,
		},
		"iteration is negative": {
			iteration:     "-1",
			commitBranch:  "main",
			defaultBranch: "main",

			expectedError: errInvalidIteration,
		},
		"iteration is positive number other than 1 on main": {
			iteration:     "2",
			commitBranch:  "main",
			defaultBranch: "main",

			expectedError: errIterationMain,
		},
		"iteration is string that can be parsed to negative number": {
			iteration:     "-2",
			commitBranch:  "main",
			defaultBranch: "main",

			expectedError: errInvalidIteration,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			orig := mageutils.GetEnv
			defer func() {
				mageutils.GetEnv = orig
			}()
			mageutils.GetEnv = func(env string) string {
				if env == "PACKAGES_ITERATION" {
					return tt.iteration
				}

				if env == "CI_COMMIT_BRANCH" {
					return tt.commitBranch
				}

				if env == "CI_DEFAULT_BRANCH" {
					return tt.defaultBranch
				}

				return ""
			}

			err := VerifyIterationVariable()
			require.Equal(t, tt.expectedError, err)
		})
	}
}
