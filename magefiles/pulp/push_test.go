//go:build !integration

package pulp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jpillora/backoff"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
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
			errorContains:   "failed after 5 retries",
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
			err := cmd.run()

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

func TestRpmPusherPush(t *testing.T) {
	tests := map[string]struct {
		releases      []string
		pkgFiles      []string
		expectedError bool
		errorContains string
	}{
		"successful push with helper images": {
			releases: []string{"fedora/43"},
			pkgFiles: []string{
				"out/rpm/gitlab-runner_18.8.0_x86_64.rpm",
				"out/rpm/gitlab-runner-helper-images.rpm",
			},
			expectedError: false,
		},
		"successful push without helper images": {
			releases: []string{"fedora/43", "fedora/44"},
			pkgFiles: []string{
				"out/rpm/gitlab-runner_18.8.0_x86_64.rpm",
			},
			expectedError: false,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			// Track which package files are being processed
			var lastPkgFile string

			// Create mock for run function
			runMock := func(cmd string, args ...string) error {
				// Mock successful execution for pulp commands
				return nil
			}

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

				return true, nil
			}

			// Create rpmPusher with mocked functions
			pusher := &rpmPusher{
				basePusher: basePusher{
					dryrun:      false,
					branch:      "main",
					concurrency: 1,
					run:         runMock,
					exec:        execMock,
				},
				archs: []string{"x86_64", "aarch64"},
			}

			// Call Push method
			err := pusher.Push(t.Context(), tt.releases, tt.pkgFiles)

			// Verify results
			if tt.expectedError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

type fakeContentCreator struct {
	mu       sync.Mutex
	calls    []fakeContentCall
	existing map[string]string // "<package>|<release_component>" -> href of an already existing link
}

type fakeContentCall struct {
	path string
	body map[string]string
}

func (f *fakeContentCreator) createContent(_ context.Context, path string, body any) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b := body.(map[string]string)
	f.calls = append(f.calls, fakeContentCall{path: path, body: b})
	switch path {
	case debReleaseArchitecturesPath:
		return "/arch/" + b["distribution"] + "/" + b["architecture"] + "/", nil
	case debPackageReleaseCompsPath:
		return "/prc/" + strings.TrimPrefix(b["release_component"], "/rc/") + strings.TrimPrefix(b["package"], "/pkg"), nil
	}
	return "", fmt.Errorf("unexpected path %s", path)
}

func (f *fakeContentCreator) findContent(_ context.Context, path string, query map[string]string) (string, error) {
	if path != debPackageReleaseCompsPath {
		return "", fmt.Errorf("unexpected lookup path %s", path)
	}
	return f.existing[query["package"]+"|"+query["release_component"]], nil
}

func TestDebPusherPush(t *testing.T) {
	releases := []string{"ubuntu/jammy", "ubuntu/noble", "debian/bookworm"}
	pkgFiles := []string{
		"out/deb/gitlab-runner_amd64.deb",
		"out/deb/gitlab-runner_arm64.deb",
		"out/deb/gitlab-runner-helper-images.deb",
	}
	archOf := map[string]string{
		"out/deb/gitlab-runner_amd64.deb":         "amd64",
		"out/deb/gitlab-runner_arm64.deb":         "arm64",
		"out/deb/gitlab-runner-helper-images.deb": "all",
	}

	var (
		mu       sync.Mutex
		uploads  []string
		modified = map[string][]string{} // repo -> hrefs added
	)

	// The mocks run on pool goroutines, so they use assert: require would call FailNow off the test goroutine.
	execMock := func(_ map[string]string, out io.Writer, _ io.Writer, cmd string, args ...string) (bool, error) {
		if !assert.Equal(t, "pulp", cmd) {
			return false, fmt.Errorf("unexpected command %s", cmd)
		}
		switch {
		case len(args) >= 3 && args[0] == deb && args[1] == "content" && args[2] == "upload":
			assert.NotContains(t, args, "--repository", "upload must not touch the repository")
			file := args[slices.Index(args, "--file")+1]
			mu.Lock()
			uploads = append(uploads, file)
			mu.Unlock()
			fmt.Fprintf(out, `{"pulp_href": "/pkg/%s/", "architecture": "%s", "package": "x"}`, filepath.Base(file), archOf[file])
		case len(args) >= 5 && args[0] == deb && args[1] == "content" && args[2] == "-t" && args[3] == "release_component" && args[4] == "create":
			fmt.Fprintf(out, `{"pulp_href": "/rc/%s/", "component": "main"}`, args[slices.Index(args, "--distribution")+1])
		default:
			return false, fmt.Errorf("unexpected exec: %v", args)
		}
		return true, nil
	}

	runMock := func(cmd string, args ...string) error {
		if !assert.Equal(t, "pulp", cmd) || !assert.Equal(t, []string{deb, "repository", "content", "modify"}, args[:4]) {
			return fmt.Errorf("unexpected command %s %v", cmd, args)
		}
		repo := args[slices.Index(args, "--repository")+1]
		content, err := os.ReadFile(strings.TrimPrefix(args[slices.Index(args, "--add-content")+1], "@"))
		if err != nil {
			return err
		}
		var units []contentUnit
		if err := json.Unmarshal(content, &units); err != nil {
			return err
		}
		mu.Lock()
		modified[repo] = lo.Map(units, func(u contentUnit, _ int) string { return u.PulpHref })
		mu.Unlock()
		return nil
	}

	api := &fakeContentCreator{existing: map[string]string{
		"/pkg/gitlab-runner_amd64.deb/|/rc/jammy/": "/prc/existing/",
	}}
	pusher := &debPusher{
		basePusher: basePusher{branch: "unstable", concurrency: 3, run: runMock, exec: execMock},
		api:        api,
	}

	require.NoError(t, pusher.Push(t.Context(), releases, pkgFiles))

	require.ElementsMatch(t, pkgFiles, uploads, "each package is uploaded exactly once")

	require.ElementsMatch(t, []string{
		"/pkg/gitlab-runner_amd64.deb/", "/pkg/gitlab-runner_arm64.deb/", "/pkg/gitlab-runner-helper-images.deb/",
		"/rc/jammy/", "/rc/noble/",
		"/arch/jammy/all/", "/arch/jammy/amd64/", "/arch/jammy/arm64/",
		"/arch/noble/all/", "/arch/noble/amd64/", "/arch/noble/arm64/",
		"/prc/existing/", "/prc/jammy//gitlab-runner_arm64.deb/", "/prc/jammy//gitlab-runner-helper-images.deb/",
		"/prc/noble//gitlab-runner_amd64.deb/", "/prc/noble//gitlab-runner_arm64.deb/", "/prc/noble//gitlab-runner-helper-images.deb/",
	}, modified["runner-unstable-ubuntu"])

	require.ElementsMatch(t, []string{
		"/pkg/gitlab-runner_amd64.deb/", "/pkg/gitlab-runner_arm64.deb/", "/pkg/gitlab-runner-helper-images.deb/",
		"/rc/bookworm/",
		"/arch/bookworm/all/", "/arch/bookworm/amd64/", "/arch/bookworm/arm64/",
		"/prc/bookworm//gitlab-runner_amd64.deb/", "/prc/bookworm//gitlab-runner_arm64.deb/", "/prc/bookworm//gitlab-runner-helper-images.deb/",
	}, modified["runner-unstable-debian"])

	require.Len(t, modified, 2, "one modify per repository")
	require.Len(t, api.calls, 3*3+3*3-1, "one release_architecture per release/arch and one link per release/package, minus the existing link")
}

func TestDebPusherPushUploadFailure(t *testing.T) {
	execMock := func(_ map[string]string, _ io.Writer, stderr io.Writer, _ string, _ ...string) (bool, error) {
		fmt.Fprint(stderr, "boom")
		return false, fmt.Errorf("exit status 1")
	}
	runCalled := false
	runMock := func(string, ...string) error { runCalled = true; return nil }

	pusher := &debPusher{
		basePusher: basePusher{branch: "unstable", concurrency: 1, run: runMock, exec: execMock},
		api:        &fakeContentCreator{},
	}
	err := pusher.Push(t.Context(), []string{"ubuntu/jammy"}, []string{"out/deb/gitlab-runner_amd64.deb"})
	require.ErrorContains(t, err, "uploading out/deb/gitlab-runner_amd64.deb")
	require.False(t, runCalled, "repository must not be modified when an upload fails")
}

func TestDebPusherPushDryRun(t *testing.T) {
	pusher := &debPusher{
		basePusher: basePusher{
			dryrun:      true,
			branch:      "unstable",
			concurrency: 1,
			run:         func(string, ...string) error { t.Error("run must not be called in dry run"); return nil },
			exec: func(map[string]string, io.Writer, io.Writer, string, ...string) (bool, error) {
				t.Error("exec must not be called in dry run")
				return false, nil
			},
		},
	}
	require.NoError(t, pusher.Push(t.Context(), []string{"ubuntu/jammy", "debian/bookworm"}, []string{"out/deb/gitlab-runner_amd64.deb"}))
}
