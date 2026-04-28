//go:build !integration

package common

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
)

func TestBuildConcreteKitchenSink(t *testing.T) {
	build := Build{
		Runner: &RunnerConfig{
			RunnerSettings: RunnerSettings{
				Executor:             "shell",
				PreGetSourcesScript:  "echo 'pre get sources'",
				PostGetSourcesScript: "echo 'post get sources'",
				PreBuildScript:       "echo 'pre build sources'",
				PostBuildScript:      "echo 'post build sources'",
				CloneURL:             "https://example.com/override.git",
			},
		},
		Job: spec.Job{
			ID:    123456789,
			Token: "test-job-token",
			Hooks: spec.Hooks{
				{Name: spec.HookPreGetSourcesScript, Script: []string{"echo 'job pre get sources script'"}},
				{Name: spec.HookPostGetSourcesScript, Script: []string{"echo 'job post get sources script'"}},
			},
			Variables: spec.Variables{
				spec.Variable{Key: "A_BASIC_VAR", Value: "BASIC"},
			},
			GitInfo: GetGitInfo(repoRemoteURL),
			Steps: spec.Steps{
				spec.Step{
					Name:         spec.StepNameScript,
					Script:       []string{"echo 'script'"},
					When:         spec.StepWhenAlways,
					AllowFailure: false,
				},
				spec.Step{
					Name:   "release",
					Script: []string{"echo 'release'"},
					When:   spec.StepWhenOnSuccess,
				},
				spec.Step{
					Name:   spec.StepNameAfterScript,
					Script: []string{"echo 'after_script'"},
					When:   spec.StepWhenAlways,
				},
			},
			Artifacts: spec.Artifacts{
				{
					Name:      "",
					Untracked: true,
					Paths:     []string{"file1", "file2"},
				},
				{
					Name:     "dotenv",
					Paths:    []string{"dotenv"},
					Format:   spec.ArtifactFormatRaw,
					Type:     "dotenv",
					ExpireIn: "7 days",
					When:     spec.ArtifactWhenOnFailure,
				},
			},
			Cache: spec.Caches{
				{
					Key:    "foobar",
					Policy: spec.CachePolicyPullPush,
					Paths:  []string{"cache_me_if_you_can"},
					When:   spec.CacheWhenAlways,
				},
			},
			RunnerInfo: spec.RunnerInfo{
				Timeout: DefaultTimeout,
			},
		},
	}

	executor := NewMockExecutor(t)
	executor.EXPECT().Shell().RunAndReturn(func() *ShellScriptInfo {
		return &ShellScriptInfo{
			Shell:                "bash",
			Build:                &build,
			Type:                 NormalShell,
			PreGetSourcesScript:  build.Runner.PreGetSourcesScript,
			PostGetSourcesScript: build.Runner.PostGetSourcesScript,
			PreBuildScript:       build.Runner.PreBuildScript,
			PostBuildScript:      build.Runner.PostBuildScript,
		}
	})

	expectedJSON := fmt.Sprintf(`{
  "after_script_ignore_errors": true,
  "after_script_timeout": 300000000000,
  "artifacts_archive": [
    {
      "compression_level": "default",
      "on_success": true,
      "paths": ["file1", "file2"],
      "response_header_timeout": 600000000000,
      "timeout": 3600000000000,
      "untracked": true
    },
    {
      "artifact_name": "dotenv",
      "compression_level": "default",
      "expire_in": "7 days",
      "format": "raw",
      "on_failure": true,
      "paths": ["dotenv"],
      "response_header_timeout": 600000000000,
      "timeout": 3600000000000,
      "type": "dotenv"
    }
  ],
  "cache_archive": [
    {
      "compression_level": "default",
      "descriptor": {},
      "key": "foobar",
      "name": "foobar",
      "on_failure": true,
      "on_success": true,
      "paths": ["cache_me_if_you_can"],
      "timeout": 10
    }
  ],
  "cache_extract": [
    {
      "max_attempts": 1,
      "paths": ["cache_me_if_you_can"],
      "sources": [
        {
          "descriptor": {},
          "key": "foobar",
          "name": "foobar"
        }
      ],
      "timeout": 10
    }
  ],
  "cleanup": {
    "git_clean_flags": ["-ffdx"],
    "git_strategy": "clone",
    "submodule_strategy": "none"
  },
  "get_sources": {
    "checkout": true,
    "clear_worktree_on_retry": true,
    "git_clean_flags": ["-ffdx"],
    "git_fetch_flags": ["--prune", "--quiet"],
    "git_strategy": "clone",
    "instead_ofs": [
      [
        "https://gitlab-ci-token:test-job-token@example.com/override.git",
        "https://example.com/override.git"
      ],
      [
        "https://gitlab-ci-token:test-job-token@gitlab.com",
        "https://gitlab.com"
      ]
    ],
    "max_attempts": 1,
    "object_format": "sha1",
    "post_clone_step": {
      "on_success": true,
      "script": [
        "echo 'job post get sources script'",
        "echo 'post get sources'"
      ],
      "step": "post_clone_script"
    },
    "pre_clone_step": {
      "on_success": true,
      "script": ["echo 'pre get sources'", "echo 'job pre get sources script'"],
      "step": "pre_clone_script"
    },
    "ref": "main",
    "refspecs": [
      "+refs/heads/*:refs/origin/heads/*",
      "+refs/tags/*:refs/tags/*"
    ],
    "remote_host": "https://example.com",
    "repo_url": "https://gitlab.com/gitlab-org/ci-cd/gitlab-runner-pipeline-tests/gitlab-test.git",
    "sha": "69b18e5ed3610cf646119c3e38f462c64ec462b7",
    "submodule_strategy": "none",
    "use_bundled_uris": true,
    "user_agent": "%s %s %s/%s"
  },
  "id": 123456789,
  "shell": "bash",
  "steps": [
    {
      "on_failure": true,
      "on_success": true,
      "script": [
        "echo 'pre build sources'",
        "echo 'script'",
        "echo 'post build sources'"
      ],
      "step": "script"
    },
    {
      "on_success": true,
      "script": [
        "echo 'pre build sources'",
        "echo 'release'",
        "echo 'post build sources'"
      ],
      "step": "release"
    },
    {
      "allow_failure": true,
      "on_failure": true,
      "on_success": true,
      "script": ["echo 'after_script'"],
      "step": "after_script"
    }
  ],
  "timeout": 7200000000000,
  "token": "test-job-token"
}
`, AppVersion.Name, AppVersion.Version, AppVersion.OS, AppVersion.Architecture)

	schema, err := stagesToConcreteStep(t.Context(), executor)
	require.NoError(t, err)
	require.Equal(t, 1, len(schema))

	var a, b any
	require.NoError(t, json.Unmarshal([]byte(expectedJSON), &a))
	require.NoError(t, json.Unmarshal([]byte(schema[0].Inputs["config"].(string)), &b))
	msg, _ := json.MarshalIndent(b, "", " ")
	require.Equal(t, a, b, string(msg))
}
