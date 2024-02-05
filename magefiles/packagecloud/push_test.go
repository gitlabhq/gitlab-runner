package packagecloud

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var exitError = errors.New("exit status 1")

func TestPackageCloudCommand(t *testing.T) {
	tests := map[string]struct {
		execFunc    execFunc
		retryErrors []string

		expectedError  error
		expectedOutput string
	}{
		"success": {
			execFunc: func(env map[string]string, stdout, stderr io.Writer, cmd string, args ...string) (ran bool, err error) {
				_, err = fmt.Fprintln(stdout, "success")
				return true, err
			},
			expectedOutput: `Running PackageCloud upload command "package_cloud []" try #1
success`,
		},
		"retry error 5 times": {
			execFunc: func() execFunc {
				var counter int

				return func(env map[string]string, stdout, stderr io.Writer, cmd string, args ...string) (ran bool, err error) {
					counter++
					if counter == 5 {
						_, _ = fmt.Fprintln(stdout, "success")
						return true, nil
					}

					_, _ = fmt.Fprintln(stderr, "retry me")
					return true, exitError
				}
			}(),
			retryErrors: []string{
				"retry me",
			},
			expectedOutput: `Running PackageCloud upload command "package_cloud []" try #1
retry me
Running PackageCloud upload command "package_cloud []" try #2
retry me
Running PackageCloud upload command "package_cloud []" try #3
retry me
Running PackageCloud upload command "package_cloud []" try #4
retry me
Running PackageCloud upload command "package_cloud []" try #5
success
`,
		},
		"retry never succeed": {
			execFunc: func(env map[string]string, stdout, stderr io.Writer, cmd string, args ...string) (ran bool, err error) {
				_, _ = fmt.Fprintln(stderr, "retry me")
				return true, exitError
			},
			retryErrors: []string{
				"retry me",
			},
			expectedError: failedToRunPackageCloudCommandError,
			expectedOutput: `Running PackageCloud upload command "package_cloud []" try #1
retry me
Running PackageCloud upload command "package_cloud []" try #2
retry me
Running PackageCloud upload command "package_cloud []" try #3
retry me
Running PackageCloud upload command "package_cloud []" try #4
retry me
Running PackageCloud upload command "package_cloud []" try #5
retry me
`,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			if len(tt.retryErrors) > 0 {
				originalRetryPackageCloudErrors := retryPackageCloudErrors
				defer func() {
					retryPackageCloudErrors = originalRetryPackageCloudErrors
				}()

				retryPackageCloudErrors = tt.retryErrors
			}

			cmd := newPackageCloudCommand(nil)
			cmd.exec = tt.execFunc
			cmd.backoff.Min = time.Millisecond
			cmd.backoff.Max = time.Millisecond

			var out bytes.Buffer
			cmd.stdout = &out
			cmd.stderr = &out

			err := cmd.run()
			t.Log(out.String())

			require.Equal(t, tt.expectedError, err)

			require.Contains(t, strings.TrimSpace(out.String()), strings.TrimSpace(tt.expectedOutput))
		})
	}
}
