package main

import (
	"fmt"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func newDefaultArgs() args {
	return args{
		Revision:    "",
		Concurrency: 1,
		Command:     commaSeparatedList{"skopeo"},
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
				B: "test, command",
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

			parsedArgs := parseArgs()

			assert.Equal(t, expectedArgs, parsedArgs)
		})
	}
}
