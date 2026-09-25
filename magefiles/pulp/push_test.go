//go:build !integration

package pulp

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jpillora/backoff"
	"github.com/stretchr/testify/require"
)

func TestParseRPMVersion(t *testing.T) {
	tests := map[string]struct {
		input           string
		expectedName    string
		expectedVersion string
		expectedArch    string
		expectedError   bool
		errorContains   string
	}{
		// Happy path cases
		"standard version format": {
			input:           "Name        : gitlab-runner\nVersion     : 1.0.0\nArchitecture: x86_64\n",
			expectedName:    "gitlab-runner",
			expectedVersion: "1.0.0",
			expectedArch:    "x86_64",
			expectedError:   false,
		},
		"pre-release version": {
			input:           "Name        : gitlab-runner\nVersion     : 18.8.0~pre.496.g9b6f071f\nArchitecture: x86_64\n",
			expectedName:    "gitlab-runner",
			expectedVersion: "18.8.0~pre.496.g9b6f071f",
			expectedArch:    "x86_64",
			expectedError:   false,
		},
		"version with dashes": {
			input:           "Name        : gitlab-runner\nVersion     : 1.0.0-rc1\nArchitecture: x86_64\n",
			expectedName:    "gitlab-runner",
			expectedVersion: "1.0.0-rc1",
			expectedArch:    "x86_64",
			expectedError:   false,
		},
		"version with plus": {
			input:           "Name        : gitlab-runner\nVersion     : 1.0.0+build123\nArchitecture: x86_64\nRelease     : 1\n",
			expectedName:    "gitlab-runner",
			expectedVersion: "1.0.0+build123",
			expectedArch:    "x86_64",
			expectedError:   false,
		},
		"version in middle of output": {
			input:           "Name        : gitlab-runner\nArchitecture: aarch64\nVersion     : 2.5.3\nRelease     : 1\nLicense     : MIT\n",
			expectedName:    "gitlab-runner",
			expectedVersion: "2.5.3",
			expectedArch:    "aarch64",
			expectedError:   false,
		},
		"version with extra whitespace": {
			input:           "Name        : gitlab-runner\nVersion     :     1.2.3     \nArchitecture: x86_64\nRelease     : 1\n",
			expectedName:    "gitlab-runner",
			expectedVersion: "1.2.3",
			expectedArch:    "x86_64",
			expectedError:   false,
		},
		"version with multiple dots": {
			input:           "Name        : gitlab-runner\nVersion     : 1.2.3.4.5\nArchitecture: x86_64\nRelease     : 1\n",
			expectedName:    "gitlab-runner",
			expectedVersion: "1.2.3.4.5",
			expectedArch:    "x86_64",
			expectedError:   false,
		},
		"version at start of output": {
			input:           "Version     : 3.0.0\nName        : gitlab-runner\nArchitecture: x86_64\nRelease     : 1\n",
			expectedName:    "gitlab-runner",
			expectedVersion: "3.0.0",
			expectedArch:    "x86_64",
			expectedError:   false,
		},
		"version at end of output": {
			input:           "Name        : gitlab-runner\nArchitecture: x86_64\nRelease     : 1\nVersion     : 4.0.0\n",
			expectedName:    "gitlab-runner",
			expectedVersion: "4.0.0",
			expectedArch:    "x86_64",
			expectedError:   false,
		},
		"actual rpm output format": {
			input: `Name        : gitlab-runner
Version     : 18.8.0~pre.496.g9b6f071f
Release     : 1
Architecture: aarch64
Install Date: (not installed)
Group       : default
Size        : 110926961
License     : MIT
Signature   : (none)
Source RPM  : gitlab-runner-18.8.0~pre.496.g9b6f071f-1.src.rpm
Build Date  : Wed 14 Jan 2026 09:14:54 PM UTC
Build Host  : cc3fa1eaba09
Relocations : /
Packager    : GitLab Inc. <support@gitlab.com>
Vendor      : GitLab Inc.
URL         : https://gitlab.com/gitlab-org/gitlab-runner
Summary     : GitLab Runner
Description : GitLab Runner
`,
			expectedName:    "gitlab-runner",
			expectedVersion: "18.8.0~pre.496.g9b6f071f",
			expectedArch:    "aarch64",
			expectedError:   false,
		},

		// Edge cases
		"version with tabs instead of spaces": {
			input:           "Name\t:\tgitlab-runner\nVersion\t:\t1.0.0\nArchitecture\t:\tx86_64\nRelease\t:\t1\n",
			expectedName:    "gitlab-runner",
			expectedVersion: "1.0.0",
			expectedArch:    "x86_64",
			expectedError:   false,
		},
		"multiple version lines returns second": {
			input:           "Version     : 1.0.0\nName        : gitlab-runner\nArchitecture: x86_64\nVersion     : 2.0.0\nRelease     : 1\n",
			expectedName:    "gitlab-runner",
			expectedVersion: "1.0.0",
			expectedArch:    "x86_64",
			expectedError:   false,
		},
		"very long version string": {
			input:           "Name        : gitlab-runner\nVersion     : 1.0.0-very-long-version-string-with-many-characters-and-numbers-12345678901234567890\nArchitecture: x86_64\nRelease     : 1\n",
			expectedName:    "gitlab-runner",
			expectedVersion: "1.0.0-very-long-version-string-with-many-characters-and-numbers-12345678901234567890",
			expectedArch:    "x86_64",
			expectedError:   false,
		},
		"version with special characters": {
			input:           "Name        : gitlab-runner\nVersion     : 1.0.0_alpha.beta-rc1+build.123\nArchitecture: x86_64\nRelease     : 1\n",
			expectedName:    "gitlab-runner",
			expectedVersion: "1.0.0_alpha.beta-rc1+build.123",
			expectedArch:    "x86_64",
			expectedError:   false,
		},

		// Error cases
		"missing version field": {
			input:         "Name        : gitlab-runner\nRelease     : 1\nArchitecture: aarch64\n",
			expectedError: true,
			errorContains: "at least one field not found",
		},
		"empty input": {
			input:         "",
			expectedError: true,
			errorContains: "at least one field not found",
		},
		"whitespace only input": {
			input:         "   \n\n   \n",
			expectedError: true,
			errorContains: "at least one field not found",
		},
		"malformed version line missing colon": {
			input:         "Name        : gitlab-runner\nVersion     1.0.0\nArchitecture: x86_64\nRelease     : 1\n",
			expectedError: true,
			errorContains: "at least one field not found",
		},
		"empty version value": {
			input:         "Name        : gitlab-runner\nVersion     : \nArchitecture: x86_64\nRelease     : 1\n",
			expectedError: true,
			errorContains: "at least one field not found",
		},
		"version with only whitespace value": {
			input:         "Name        : gitlab-runner\nVersion     :    \nArchitecture: x86_64\nRelease     : 1\n",
			expectedError: true,
			errorContains: "at least one field not found",
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			info, err := parseRPMInfo(reader)

			if tt.expectedError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedName, info.name)
				require.Equal(t, tt.expectedVersion, info.version)
				require.Equal(t, tt.expectedArch, info.arch)
			}
		})
	}
}

func TestRetryCommandRun(t *testing.T) {
	tests := map[string]struct {
		execBehavior    func(attempt int) (bool, string, error) // returns (success, stderr, error )
		retryableErrs   []*regexp.Regexp
		expectedError   bool
		errorContains   string
		expectedAttempt int
	}{
		"successful on first attempt": {
			execBehavior: func(attempt int) (bool, string, error) {
				return true, "", nil
			},
			retryableErrs:   []*regexp.Regexp{},
			expectedError:   false,
			expectedAttempt: 1,
		},
		"successful on second attempt with retryable error": {
			execBehavior: func(attempt int) (bool, string, error) {
				if attempt == 1 {
					return false, "Artifact with checksum of 'abc123' already exists.", fmt.Errorf("artifact error")
				}
				return true, "", nil
			},
			retryableErrs: []*regexp.Regexp{
				regexp.MustCompile(`Artifact with checksum of '.*' already exists\.`),
			},
			expectedError:   false,
			expectedAttempt: 2,
		},
		"successful on third attempt with retryable error": {
			execBehavior: func(attempt int) (bool, string, error) {
				if attempt <= 2 {
					return false, "Artifact with checksum of 'xyz789' already exists.", fmt.Errorf("artifact error")
				}
				return true, "", nil
			},
			retryableErrs: []*regexp.Regexp{
				regexp.MustCompile(`Artifact with checksum of '.*' already exists\.`),
			},
			expectedError:   false,
			expectedAttempt: 3,
		},
		"fails with non-retryable error on first attempt": {
			execBehavior: func(attempt int) (bool, string, error) {
				return false, "Permission denied: cannot access repository", fmt.Errorf("permission denied")
			},
			retryableErrs: []*regexp.Regexp{
				regexp.MustCompile(`Artifact with checksum of '.*' already exists\.`),
			},
			expectedError:   true,
			errorContains:   "Permission denied",
			expectedAttempt: 1,
		},
		"fails after max retries with retryable error": {
			execBehavior: func(attempt int) (bool, string, error) {
				return false, "Artifact with checksum of 'def456' already exists.", fmt.Errorf("artifact error")
			},
			retryableErrs: []*regexp.Regexp{
				regexp.MustCompile(`Artifact with checksum of '.*' already exists\.`),
			},
			expectedError:   true,
			errorContains:   "failed after 5 attempts",
			expectedAttempt: 5,
		},
		"multiple retryable error patterns": {
			execBehavior: func(attempt int) (bool, string, error) {
				if attempt == 1 {
					return false, "Connection timeout: server not responding", fmt.Errorf("timeout")
				}
				if attempt == 2 {
					return false, "Artifact with checksum of 'ghi012' already exists.", fmt.Errorf("artifact error")
				}
				return true, "", nil
			},
			retryableErrs: []*regexp.Regexp{
				regexp.MustCompile(`Artifact with checksum of '.*' already exists\.`),
				regexp.MustCompile(`Connection timeout:.*`),
			},
			expectedError:   false,
			expectedAttempt: 3,
		},
		"retryable error on last attempt succeeds": {
			execBehavior: func(attempt int) (bool, string, error) {
				if attempt < 5 {
					return false, "Artifact with checksum of 'jkl345' already exists.", fmt.Errorf("artifact error")
				}
				return true, "", nil
			},
			retryableErrs: []*regexp.Regexp{
				regexp.MustCompile(`Artifact with checksum of '.*' already exists\.`),
			},
			expectedError:   false,
			expectedAttempt: 5,
		},
		"no retryable errors configured": {
			execBehavior: func(attempt int) (bool, string, error) {
				return false, "Some error message", fmt.Errorf("some error")
			},
			retryableErrs:   []*regexp.Regexp{},
			expectedError:   true,
			errorContains:   "Some error message",
			expectedAttempt: 1,
		},
		"empty stderr with error": {
			execBehavior: func(attempt int) (bool, string, error) {
				return false, "", fmt.Errorf("command failed")
			},
			retryableErrs: []*regexp.Regexp{
				regexp.MustCompile(`Artifact with checksum of '.*' already exists\.`),
			},
			expectedError:   true,
			errorContains:   "execution of command",
			expectedAttempt: 1,
		},
		"pulp: retries when cloudflare returns a 5xx error page": {
			execBehavior: func(attempt int) (bool, string, error) {
				if attempt == 1 {
					return false, cloudflareErrorPage(502, "Bad gateway"), fmt.Errorf("exit status 1")
				}
				return true, "", nil
			},
			retryableErrs:   pulpRetryErrors,
			expectedError:   false,
			expectedAttempt: 2,
		},
		"pulp: retries when the origin returns an unknown error through cloudflare": {
			execBehavior: func(attempt int) (bool, string, error) {
				if attempt == 1 {
					return false, cloudflareErrorPage(520, "Web server is returning an unknown error"), fmt.Errorf("exit status 1")
				}
				return true, "", nil
			},
			retryableErrs:   pulpRetryErrors,
			expectedError:   false,
			expectedAttempt: 2,
		},
		"pulp: retries when the worker running the task goes missing": {
			execBehavior: func(attempt int) (bool, string, error) {
				if attempt == 1 {
					return false, "....Upload complete.\nStarted background task /pulp/api/v3/tasks/01a0ce20-ea71-7ad9-8791-cef28888e5fe/\n" +
						".....Error: Task /pulp/api/v3/tasks/01a0ce20-ea71-7ad9-8791-cef28888e5fe/ failed: 'Worker has gone missing.'\n", fmt.Errorf("exit status 1")
				}
				return true, "", nil
			},
			retryableErrs:   pulpRetryErrors,
			expectedError:   false,
			expectedAttempt: 2,
		},
		"pulp: retries when the connection to the API fails": {
			execBehavior: func(attempt int) (bool, string, error) {
				if attempt == 1 {
					return false, "Error: HTTPSConnectionPool(host='pulp.gitlab.com', port=443): Read timed out. (read timeout=120)", fmt.Errorf("exit status 1")
				}
				return true, "", nil
			},
			retryableErrs:   pulpRetryErrors,
			expectedError:   false,
			expectedAttempt: 2,
		},
		"pulp: retries when the artifact already exists": {
			execBehavior: func(attempt int) (bool, string, error) {
				if attempt == 1 {
					return false, `Error: {"non_field_errors":["Artifact with sha256 checksum of '97095ba5' already exists."]}`, fmt.Errorf("exit status 1")
				}
				return true, "", nil
			},
			retryableErrs:   pulpRetryErrors,
			expectedError:   false,
			expectedAttempt: 2,
		},
		"pulp: does not retry a client error": {
			execBehavior: func(attempt int) (bool, string, error) {
				return false, `Error: {"non_field_errors":["Destination upload repository was not provided."]}`, fmt.Errorf("exit status 1")
			},
			retryableErrs:   pulpRetryErrors,
			expectedError:   true,
			errorContains:   "Destination upload repository was not provided",
			expectedAttempt: 1,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			attempt := 0

			// Create mock exec function that tracks attempts
			execMock := func(env map[string]string, out io.Writer, stderr io.Writer, cmd string, args ...string) (bool, error) {
				attempt++
				success, stderrMsg, err := tt.execBehavior(attempt)

				if stderrMsg != "" {
					_, _ = io.WriteString(stderr, stderrMsg)
				}

				return success, err
			}

			// Create retryCommand with mocked exec
			cmd := newRetryCommand("test-cmd", []string{"arg1", "arg2"}, tt.retryableErrs, io.Discard, execMock)
			// make it a bit faster
			cmd.backoff = backoff.Backoff{Min: 10 * time.Millisecond, Max: 50 * time.Millisecond}

			// Run the command
			err := cmd.run(t.Context())

			// Verify results
			if tt.expectedError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tt.expectedAttempt, attempt, "expected %d attempts, got %d", tt.expectedAttempt, attempt)
		})
	}
}

// cloudflareErrorPage mimics the HTML page Cloudflare serves for an error from the origin, as it
// ends up on the pulp CLI's stderr.
func cloudflareErrorPage(code int, title string) string {
	return fmt.Sprintf(`....Error: <!DOCTYPE html>
<html>
<head>
  <title>500 Error - GitLab</title>
</head>
<body>
    <div class="cf-error-details cf-error-%[1]d">
  <h1>%[2]s</h1>
  <ul>
    <li>Ray ID: a3f8715ac8ceed82</li>
    <li>Error reference number: %[1]d</li>
    <li>Cloudflare Location: Atlanta</li>
  </ul>
</div>
</body>
</html>
`, code, title)
}

func TestRetryCommandRunAbandonedWhenContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	attempts := 0
	execMock := func(env map[string]string, out io.Writer, stderr io.Writer, cmd string, args ...string) (bool, error) {
		attempts++
		// the push is aborted while this command is failing with a retryable error
		cancel()
		_, _ = io.WriteString(stderr, cloudflareErrorPage(502, "Bad gateway"))
		return false, fmt.Errorf("exit status 1")
	}

	cmd := newRetryCommand("test-cmd", []string{"arg1"}, pulpRetryErrors, io.Discard, execMock)
	cmd.backoff = backoff.Backoff{Min: 500 * time.Millisecond, Max: 500 * time.Millisecond}

	err := cmd.run(ctx)

	require.ErrorIs(t, err, context.Canceled)
	require.ErrorContains(t, err, "abandoned after attempt 1")
	require.Equal(t, 1, attempts, "no further attempt must be made once the push is aborted")
}

func TestDebPusherPush(t *testing.T) {
	pkgFiles := []string{"out/deb/gitlab-runner_amd64.deb", "out/deb/gitlab-runner-helper-images.deb"}

	tests := map[string]struct {
		releases        []string
		autopublish     bool
		failOnUpload    int // 1-based index of the upload that fails for good, 0 for none
		expectedError   string
		expectedUploads int
		expectedPublish []string // repositories a publication is created for, in order
	}{
		"uploads every package to every release, then publishes each repository once": {
			releases:        []string{"debian/bookworm", "debian/trixie", "ubuntu/noble"},
			expectedUploads: 3 * len(pkgFiles),
			expectedPublish: []string{"runner-unstable-debian", "runner-unstable-ubuntu"},
		},
		"leaves publishing to a repository that autopublishes": {
			releases:        []string{"debian/bookworm", "debian/trixie"},
			autopublish:     true,
			expectedUploads: 2 * len(pkgFiles),
		},
		"stops starting uploads once one has failed for good, and does not publish": {
			releases:        []string{"debian/bookworm", "debian/trixie"},
			failOnUpload:    2,
			expectedError:   "Destination upload repository was not provided",
			expectedUploads: 2,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			var uploads atomic.Int32
			var published []string

			execMock := func(env map[string]string, out io.Writer, stderr io.Writer, cmd string, args ...string) (bool, error) {
				require.Equal(t, "pulp", cmd)
				repo := args[len(args)-1]
				if i := slices.Index(args, "--repository"); i >= 0 {
					repo = args[i+1]
				}

				switch strings.Join(args[:3], " ") {
				case "deb content upload":
					require.Contains(t, []string{"runner-unstable-debian", "runner-unstable-ubuntu"}, repo)
					if int(uploads.Add(1)) == tt.failOnUpload {
						_, _ = io.WriteString(stderr, `Error: {"non_field_errors":["Destination upload repository was not provided."]}`)
						return false, fmt.Errorf("exit status 1")
					}
				case "deb repository show":
					require.EqualValues(t, tt.expectedUploads, uploads.Load(), "publishing must wait for all uploads")
					fmt.Fprintf(out, `{"name": %q, "autopublish": %t}`, repo, tt.autopublish)
				case "deb publication create":
					published = append(published, repo)
				default:
					require.Failf(t, "unexpected command", "pulp %s", strings.Join(args, " "))
				}
				return true, nil
			}

			pusher := &debPusher{
				basePusher: basePusher{
					branch:      "unstable",
					concurrency: 1,
					exec:        execMock,
				},
			}

			err := pusher.Push(tt.releases, pkgFiles)

			if tt.expectedError != "" {
				require.ErrorContains(t, err, tt.expectedError)
			} else {
				require.NoError(t, err)
			}
			require.EqualValues(t, tt.expectedUploads, uploads.Load())
			require.Equal(t, tt.expectedPublish, published)
		})
	}
}

func TestRpmPusherPush(t *testing.T) {
	tests := map[string]struct {
		releases      []string
		pkgFiles      []string
		expectedError bool
		errorContains string
		expectedLinks int
	}{
		"successful push with helper images": {
			releases: []string{"fedora/43"},
			pkgFiles: []string{
				"out/rpm/gitlab-runner_18.8.0_x86_64.rpm",
				"out/rpm/gitlab-runner-helper-images.rpm",
			},
			expectedError: false,
			// the runner package is linked for its own arch, the helper images package for every arch
			expectedLinks: 1 + 2,
		},
		"successful push without helper images": {
			releases: []string{"fedora/43", "fedora/44"},
			pkgFiles: []string{
				"out/rpm/gitlab-runner_18.8.0_x86_64.rpm",
			},
			expectedError: false,
			expectedLinks: 2,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			// Track which package files are being processed
			var lastPkgFile string
			var links atomic.Int32

			// Create mock for exec function
			execMock := func(env map[string]string, out io.Writer, stderr io.Writer, cmd string, args ...string) (bool, error) {
				// Detect rpm -qi command
				if cmd == rpm && len(args) >= 2 && args[0] == "-qi" {
					// Track the package file being queried
					lastPkgFile = args[1]

					// Determine package name from the file path
					pkgName := "gitlab-runner"
					if strings.Contains(lastPkgFile, "helper-images") {
						pkgName = "gitlab-runner-helper-images"
					}

					// Write rpm version output
					fmt.Fprintf(out, `Name        : %s
Version     : 18.8.0
Release     : 1
Architecture: x86_64
Install Date: (not installed)
Group       : default
Size        : 110926961
License     : MIT
Signature   : (none)
Source RPM  : %s-18.8.0-1.src.rpm
Build Date  : Wed 14 Jan 2026 09:14:54 PM UTC
Build Host  : cc3fa1eaba09
Relocations : /
Packager    : GitLab Inc. <support@gitlab.com>
Vendor      : GitLab Inc.
URL         : https://gitlab.com/gitlab-org/gitlab-runner
Summary     : GitLab Runner
Description : GitLab Runner
`, pkgName, pkgName)
					return true, nil
				}

				// Detect pulp rpm content upload command
				if cmd == "pulp" && len(args) >= 5 && args[0] == rpm && args[1] == "content" && args[2] == "upload" {
					// Write JSON response with pulp_href
					fmt.Fprintf(out, `{"pulp_href": "/pulp/api/v3/content/rpm/packages/abc123/"}`)
					return true, nil
				}

				// Detect pulp rpm repository content modify command, linking the package to a repo
				if cmd == "pulp" && len(args) >= 5 && args[0] == rpm && args[1] == "repository" && args[3] == "modify" {
					links.Add(1)
					return true, nil
				}

				return true, nil
			}

			// Create rpmPusher with mocked functions
			pusher := &rpmPusher{
				basePusher: basePusher{
					dryrun:      false,
					branch:      "main",
					concurrency: 1,
					exec:        execMock,
				},
				archs: []string{"x86_64", "aarch64"},
			}

			// Call Push method
			err := pusher.Push(tt.releases, tt.pkgFiles)

			// Verify results
			if tt.expectedError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
			}
			require.EqualValues(t, tt.expectedLinks, links.Load())
		})
	}
}
