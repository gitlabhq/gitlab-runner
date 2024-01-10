//go:build scripts

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newDefaultArgs() args {
	return args{
		Revision:    "",
		Concurrency: 1,
		Command:     spaceSeparatedList{"skopeo"},
		Images:      commaSeparatedList{string(gitlabRunnerImage), string(gitlabRunnerHelperImage)},
		Filters:     nil,
		DryRun:      false,
	}
}

func TestParseArgs(t *testing.T) {
	// Do basic testing to verify our parseArgs function works as expected,
	// not to test the functionality of the CLI parsing library

	type cmdArg lo.Tuple2[string, string]

	tests := map[string]struct {
		cmdArg       *cmdArg
		expectedArgs func(a *args)
	}{
		"default, only revision": {},
		"concurrency": {
			cmdArg: &cmdArg{
				A: "concurrency",
				B: "10",
			},
			expectedArgs: func(a *args) {
				a.Concurrency = 10
			},
		},
		"concurrency less than 1": {
			cmdArg: &cmdArg{
				A: "concurrency",
				B: "-1",
			},
			expectedArgs: func(a *args) {
				a.Concurrency = 1
			},
		},
		"command": {
			cmdArg: &cmdArg{
				A: "command",
				B: "test command",
			},
			expectedArgs: func(a *args) {
				a.Command = []string{"test", "command"}
			},
		},
		"images": {
			cmdArg: &cmdArg{
				A: "images",
				B: "test, image",
			},
			expectedArgs: func(a *args) {
				a.Images = []string{"test", "image"}
			},
		},
		"filters": {
			cmdArg: &cmdArg{
				A: "filters",
				B: "test, filter",
			},
			expectedArgs: func(a *args) {
				a.Filters = []string{"test", "filter"}
			},
		},
		"dry run": {
			cmdArg: &cmdArg{
				A: "dry-run",
				B: "true",
			},
			expectedArgs: func(a *args) {
				a.DryRun = true
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			// Revision is required so it's always added
			cmdArgs := []cmdArg{
				{
					A: "revision",
					B: "v16.0.0",
				},
			}
			if tt.cmdArg != nil {
				cmdArgs = append(cmdArgs, *tt.cmdArg)
			}

			expectedArgs := newDefaultArgs()
			expectedArgs.Revision = "v16.0.0"
			if tt.expectedArgs != nil {
				tt.expectedArgs(&expectedArgs)
			}

			os.Args = os.Args[:1]
			for _, arg := range cmdArgs {
				os.Args = append(os.Args, fmt.Sprintf("--%s=%s", arg.A, arg.B))
			}

			parsedArgs := *parseArgs()

			compiledFilters := parsedArgs.compiledFilters
			parsedArgs.compiledFilters = nil

			assert.Equal(t, expectedArgs, parsedArgs)
			assert.Len(t, compiledFilters, len(parsedArgs.Filters))
		})
	}
}

func TestGenerateTags(t *testing.T) {
	tests := map[string]struct {
		image                image
		expectedImagesFile   string
		filter               string
		shouldGenerateLatest bool
	}{
		"gitlab-runner": {
			image:              gitlabRunnerImage,
			expectedImagesFile: "dockerhub_tags_gitlab_runner_test.txt",
		},
		"gitlab-runner-helper": {
			image:              gitlabRunnerHelperImage,
			expectedImagesFile: "dockerhub_tags_gitlab_runner_helper_test.txt",
			filter:             `v16\.1\.0`,
		},
		"gitlab-runner-helper-latest": {
			image:              gitlabRunnerHelperImage,
			expectedImagesFile: "dockerhub_tags_gitlab_runner_helper_latest_test.txt",
			filter:             "(latest-pwsh|latest$|latest-servercore|latest-nanoserver)",
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			dir, err := os.Getwd()
			require.NoError(t, err)

			f, err := os.ReadFile(filepath.Join(dir, "testdata", tt.expectedImagesFile))
			require.NoError(t, err)

			args := &args{
				Revision: "v16.1.0",
				Images:   commaSeparatedList{string(tt.image)},
				IsLatest: true,
			}

			if tt.filter != "" {
				args.Filters = commaSeparatedList{tt.filter}
			}

			args.compileFilters()

			tags, err := generateAllTags(args)
			require.NoError(t, err)

			expectedTags := lo.Filter(strings.Split(string(f), "\n"), func(tag string, _ int) bool {
				return tag != ""
			})

			for _, tag := range tags {
				assert.True(t, lo.Contains(expectedTags, tag.name), "expected %s to be in expected tags", tag.name)
			}

			for _, expectedTag := range expectedTags {
				assert.True(t, lo.ContainsBy(tags, func(t tag) bool {
					return t.name == expectedTag
				}), "expected %s to be in generated tags", expectedTag)
			}
		})
	}
}
