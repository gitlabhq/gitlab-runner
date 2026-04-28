//go:build !integration

package builder

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/builder/variables"
	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run"
	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run/cacheprovider"
	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run/stages"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

func newTestVars(t *testing.T, overrides map[string]string, setup ...func(*variables.MockProvider)) *variables.MockProvider {
	t.Helper()

	m := variables.NewMockProvider(t)

	defaults := map[string]string{
		"CI_SERVER_URL":            "https://gitlab.example.com",
		"CI_SERVER_HOST":           "gitlab.example.com",
		"CI_SERVER_SHELL_SSH_HOST": "",
		"CI_SERVER_SHELL_SSH_PORT": "",
		"CI_PROJECT_PATH":          "group/project",
		"CI_RUNNER_ID":             "42",
	}

	for k, v := range overrides {
		defaults[k] = v
	}

	for k, v := range defaults {
		m.EXPECT().Get(k).Maybe().Return(v)
		m.EXPECT().ExpandValue(k).Maybe().Return(v)
	}

	for _, fn := range setup {
		fn(m)
	}

	m.On("Get", mock.Anything).Maybe().Return("")
	m.On("ExpandValue", mock.Anything).Maybe().Return("")

	return m
}

func expandValues(mappings map[string]string) func(*variables.MockProvider) {
	return func(m *variables.MockProvider) {
		for k, v := range mappings {
			m.EXPECT().ExpandValue(k).Maybe().Return(v)
		}
	}
}

func buildConfig(t *testing.T, job spec.Job, vars variables.Provider, opts ...Option) run.Config {
	t.Helper()

	data, err := Build(job, vars, opts...)
	require.NoError(t, err)

	var config run.Config
	require.NoError(t, json.Unmarshal(data, &config))

	return config
}

func baseJob() spec.Job {
	return spec.Job{
		ID:    123,
		Token: "test-token",
		GitInfo: spec.GitInfo{
			RepoURL:  "https://gitlab.example.com/group/project.git",
			Ref:      "main",
			Sha:      "abc123def456",
			Refspecs: []string{"+refs/heads/*:refs/remotes/origin/*"},
			Depth:    0,
		},
		JobInfo: spec.JobInfo{
			Name:      "test-job",
			ProjectID: 99,
		},
		RunnerInfo: spec.RunnerInfo{
			Timeout: 3600,
		},
	}
}

func TestBuild_BasicConfig(t *testing.T) {
	tests := map[string]struct {
		opts   []Option
		assert func(t *testing.T, config run.Config)
	}{
		"explicit options": {
			opts: []Option{WithShell("bash"), WithCacheDir("/cache")},
			assert: func(t *testing.T, c run.Config) {
				assert.Equal(t, "bash", c.Shell)
				assert.Equal(t, "/cache", c.CacheDir)
			},
		},
		"defaults": {
			opts: nil,
			assert: func(t *testing.T, c run.Config) {
				assert.Equal(t, "sh", c.Shell)
				assert.Equal(t, "cache", c.CacheDir)
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			config := buildConfig(t, baseJob(), newTestVars(t, nil), tc.opts...)

			assert.Equal(t, int64(123), config.ID)
			assert.Equal(t, "test-token", config.Token)
			assert.Equal(t, "https://gitlab.example.com", config.BaseURL)
			assert.Equal(t, 3600*time.Second, config.Timeout)
			tc.assert(t, config)
		})
	}
}

func TestBuild_GetSources(t *testing.T) {
	t.Run("git strategy", func(t *testing.T) {
		for _, strategy := range []string{"fetch", "clone", "none", "empty"} {
			t.Run(strategy, func(t *testing.T) {
				vars := newTestVars(t, map[string]string{"GIT_STRATEGY": strategy})
				config := buildConfig(t, baseJob(), vars)
				assert.Equal(t, strategy, config.GetSources.GitStrategy)
			})
		}
	})

	t.Run("defaults", func(t *testing.T) {
		config := buildConfig(t, baseJob(), newTestVars(t, nil))

		assert.Equal(t, "clone", config.GetSources.GitStrategy)
		assert.True(t, config.GetSources.Checkout)
		assert.Equal(t, 1, config.GetSources.MaxAttempts)
		assert.Equal(t, "none", config.GetSources.SubmoduleStrategy)
		assert.Equal(t, "sha1", config.GetSources.ObjectFormat)
		assert.False(t, config.GetSources.LFSDisabled)
	})

	t.Run("depth", func(t *testing.T) {
		job := baseJob()
		job.GitInfo.Depth = 50
		config := buildConfig(t, job, newTestVars(t, nil))
		assert.Equal(t, 50, config.GetSources.Depth)
	})

	t.Run("flags", func(t *testing.T) {
		vars := newTestVars(t, map[string]string{
			"GIT_CLONE_FLAGS": "--no-tags --single-branch",
			"GIT_FETCH_FLAGS": "--prune",
			"GIT_CLEAN_FLAGS": "-ffdx",
		})
		config := buildConfig(t, baseJob(), vars)

		assert.Equal(t, []string{"--no-tags", "--single-branch"}, config.GetSources.GitCloneFlags)
		assert.Equal(t, []string{"--prune"}, config.GetSources.GitFetchFlags)
		assert.Equal(t, []string{"-ffdx"}, config.GetSources.GitCleanFlags)
	})

	t.Run("submodules", func(t *testing.T) {
		vars := newTestVars(t, map[string]string{
			"GIT_SUBMODULE_STRATEGY":     "recursive",
			"GIT_SUBMODULE_DEPTH":        "5",
			"GIT_SUBMODULE_UPDATE_FLAGS": "--remote",
			"GIT_SUBMODULE_PATHS":        "sub1 sub2",
		})
		config := buildConfig(t, baseJob(), vars)

		assert.Equal(t, "recursive", config.GetSources.SubmoduleStrategy)
		assert.Equal(t, 5, config.GetSources.SubmoduleDepth)
		assert.Equal(t, []string{"--remote"}, config.GetSources.SubmoduleUpdateFlags)
		assert.Equal(t, []string{"sub1", "sub2"}, config.GetSources.SubmodulePaths)
	})

	t.Run("feature flags", func(t *testing.T) {
		ff := func(name string) bool {
			switch name {
			case featureflags.UseGitNativeClone,
				featureflags.UseGitBundleURIs,
				featureflags.UseGitProactiveAuth:
				return true
			}
			return false
		}
		config := buildConfig(t, baseJob(), newTestVars(t, nil), WithFeatureFlagProvider(ff))

		assert.True(t, config.GetSources.UseNativeClone)
		assert.True(t, config.GetSources.UseBundleURIs)
		assert.True(t, config.GetSources.UseProactiveAuth)
	})

	t.Run("pre/post clone scripts", func(t *testing.T) {
		config := buildConfig(t, baseJob(), newTestVars(t, nil),
			WithPreCloneScript([]string{"echo pre"}),
			WithPostCloneScript([]string{"echo post"}),
		)

		assert.Equal(t, "pre_clone_script", config.GetSources.PreCloneStep.Step)
		assert.Equal(t, []string{"echo pre"}, config.GetSources.PreCloneStep.Script)
		assert.True(t, config.GetSources.PreCloneStep.OnSuccess)

		assert.Equal(t, "post_clone_script", config.GetSources.PostCloneStep.Step)
		assert.Equal(t, []string{"echo post"}, config.GetSources.PostCloneStep.Script)
		assert.True(t, config.GetSources.PostCloneStep.OnSuccess)
	})

	t.Run("options", func(t *testing.T) {
		config := buildConfig(t, baseJob(), newTestVars(t, nil),
			WithSafeDirectoryCheckout(true),
			WithGitCleanConfig(true),
			WithIsSharedEnv(true),
			WithUserAgent("runner/1.0"),
			WithGitalyCorrelationID("corr-123"),
		)

		assert.True(t, config.GetSources.SafeDirectoryCheckout)
		assert.True(t, config.GetSources.CleanGitConfig)
		assert.True(t, config.GetSources.IsSharedEnv)
		assert.Equal(t, "runner/1.0", config.GetSources.UserAgent)
		assert.Equal(t, "corr-123", config.GetSources.GitalyCorrelationID)
	})
}

func TestBuild_Steps(t *testing.T) {
	t.Run("pre/post build script wraps each user step in same shell", func(t *testing.T) {
		job := baseJob()
		job.Steps = []spec.Step{
			{Name: "build", Script: []string{"make build"}, When: spec.StepWhenOnSuccess},
			{Name: "test", Script: []string{"make test"}, When: spec.StepWhenAlways},
		}

		config := buildConfig(t, job, newTestVars(t, nil),
			WithPreBuildScript([]string{"echo pre-build"}),
			WithPostBuildScript([]string{"echo post-build"}),
		)

		// pre/post build are folded into each user step rather than emitted as
		// their own steps; this matches abstract shell semantics where they
		// share a shell process with the user script.
		require.Len(t, config.Steps, 2)

		assert.Equal(t, "build", config.Steps[0].Step)
		assert.True(t, config.Steps[0].OnSuccess)
		assert.False(t, config.Steps[0].OnFailure)
		assert.Equal(t,
			[]string{"echo pre-build", "make build", "echo post-build"},
			config.Steps[0].Script)

		assert.Equal(t, "test", config.Steps[1].Step)
		assert.True(t, config.Steps[1].OnSuccess)
		assert.True(t, config.Steps[1].OnFailure)
		assert.Equal(t,
			[]string{"echo pre-build", "make test", "echo post-build"},
			config.Steps[1].Script)
	})

	t.Run("after_script is not wrapped with pre/post build", func(t *testing.T) {
		job := baseJob()
		job.Steps = []spec.Step{
			{Name: spec.StepNameAfterScript, Script: []string{"echo cleanup"}, When: spec.StepWhenAlways},
		}

		config := buildConfig(t, job, newTestVars(t, nil),
			WithPreBuildScript([]string{"echo pre-build"}),
			WithPostBuildScript([]string{"echo post-build"}),
		)

		require.Len(t, config.Steps, 1)
		assert.Equal(t, string(spec.StepNameAfterScript), config.Steps[0].Step)
		assert.Equal(t, []string{"echo cleanup"}, config.Steps[0].Script)
	})

	t.Run("after_script moved to end", func(t *testing.T) {
		job := baseJob()
		job.Steps = []spec.Step{
			{Name: "build", Script: []string{"make"}, When: spec.StepWhenOnSuccess},
			{Name: spec.StepNameAfterScript, Script: []string{"echo cleanup"}, When: spec.StepWhenAlways},
		}

		config := buildConfig(t, job, newTestVars(t, nil))

		last := config.Steps[len(config.Steps)-1]
		assert.Equal(t, string(spec.StepNameAfterScript), last.Step)
		assert.True(t, last.AllowFailure)
		assert.True(t, last.OnSuccess)
		assert.True(t, last.OnFailure)
	})

	t.Run("release expands variables", func(t *testing.T) {
		job := baseJob()
		job.Steps = []spec.Step{
			{Name: "release", Script: []string{"release-cli create --name $CI_COMMIT_TAG"}, When: spec.StepWhenOnSuccess},
		}

		vars := newTestVars(t, nil, func(m *variables.MockProvider) {
			m.EXPECT().ExpandValue("release-cli create --name $CI_COMMIT_TAG").Return("release-cli create --name v1.0.0")
		})

		config := buildConfig(t, job, vars)

		var releaseStep *stages.Step
		for i := range config.Steps {
			if config.Steps[i].Step == "release" {
				releaseStep = &config.Steps[i]
				break
			}
		}
		require.NotNil(t, releaseStep)
		assert.Equal(t, []string{"release-cli create --name v1.0.0"}, releaseStep.Script)
	})

	t.Run("debug and sections", func(t *testing.T) {
		job := baseJob()
		job.Steps = []spec.Step{
			{Name: "build", Script: []string{"make"}, When: spec.StepWhenOnSuccess},
		}
		job.Features = spec.GitlabFeatures{TraceSections: true}

		ff := func(name string) bool {
			return name == featureflags.ScriptSections || name == featureflags.EnableBashExitCodeCheck
		}
		config := buildConfig(t, job, newTestVars(t, nil), WithDebug(true), WithFeatureFlagProvider(ff))

		for _, step := range config.Steps {
			assert.True(t, step.Debug)
			assert.True(t, step.BashExitCodeCheck)
			assert.True(t, step.ScriptSections)
		}
	})
}

func TestBuild_Timeouts(t *testing.T) {
	tests := map[string]struct {
		varName  string
		varValue string
		check    func(t *testing.T, config run.Config)
	}{
		"script timeout unset": {
			varName: "RUNNER_SCRIPT_TIMEOUT", varValue: "",
			check: func(t *testing.T, c run.Config) { assert.Equal(t, time.Duration(0), c.ScriptTimeout) },
		},
		"script timeout valid": {
			varName: "RUNNER_SCRIPT_TIMEOUT", varValue: "30m",
			check: func(t *testing.T, c run.Config) { assert.Equal(t, 30*time.Minute, c.ScriptTimeout) },
		},
		"script timeout invalid": {
			varName: "RUNNER_SCRIPT_TIMEOUT", varValue: "notaduration",
			check: func(t *testing.T, c run.Config) { assert.Equal(t, time.Duration(0), c.ScriptTimeout) },
		},
		"after script timeout default": {
			varName: "RUNNER_AFTER_SCRIPT_TIMEOUT", varValue: "",
			check: func(t *testing.T, c run.Config) { assert.Equal(t, 5*time.Minute, c.AfterScriptTimeout) },
		},
		"after script timeout custom": {
			varName: "RUNNER_AFTER_SCRIPT_TIMEOUT", varValue: "10m",
			check: func(t *testing.T, c run.Config) { assert.Equal(t, 10*time.Minute, c.AfterScriptTimeout) },
		},
		"after script timeout invalid": {
			varName: "RUNNER_AFTER_SCRIPT_TIMEOUT", varValue: "bad",
			check: func(t *testing.T, c run.Config) { assert.Equal(t, 5*time.Minute, c.AfterScriptTimeout) },
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			vars := newTestVars(t, map[string]string{tc.varName: tc.varValue})
			config := buildConfig(t, baseJob(), vars)
			tc.check(t, config)
		})
	}
}

func TestBuild_AfterScriptIgnoreErrors(t *testing.T) {
	tests := map[string]struct {
		varValue string
		expected bool
	}{
		"default true":   {varValue: "", expected: true},
		"explicit true":  {varValue: "true", expected: true},
		"explicit false": {varValue: "false", expected: false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			vars := newTestVars(t, map[string]string{"AFTER_SCRIPT_IGNORE_ERRORS": tc.varValue})
			config := buildConfig(t, baseJob(), vars)
			assert.Equal(t, tc.expected, config.AfterScriptIgnoreErrors)
		})
	}
}

func TestBuild_CacheExtract(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		job := baseJob()
		job.Cache = []spec.Cache{
			{Key: "test-key", Paths: []string{"vendor/", ".cache/"}, Policy: spec.CachePolicyPullPush},
		}

		vars := newTestVars(t, nil, expandValues(map[string]string{"test-key": "test-key"}))
		config := buildConfig(t, job, vars)

		require.Len(t, config.CacheExtract, 1)
		require.Len(t, config.CacheExtract[0].Sources, 1)
		assert.Equal(t, "test-key", config.CacheExtract[0].Sources[0].Name)
		assert.Equal(t, "test-key", config.CacheExtract[0].Sources[0].Key)
		assert.Equal(t, []string{"vendor/", ".cache/"}, config.CacheExtract[0].Paths)
	})

	t.Run("policy filtering", func(t *testing.T) {
		tests := map[string]struct {
			policy        spec.CachePolicy
			expectExtract bool
		}{
			"undefined extracts": {policy: spec.CachePolicyUndefined, expectExtract: true},
			"pull-push extracts": {policy: spec.CachePolicyPullPush, expectExtract: true},
			"pull extracts":      {policy: spec.CachePolicyPull, expectExtract: true},
			"push skips":         {policy: spec.CachePolicyPush, expectExtract: false},
		}

		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				job := baseJob()
				job.Cache = []spec.Cache{
					{Key: "cache", Paths: []string{"build/"}, Policy: tc.policy},
				}

				vars := newTestVars(t, nil, expandValues(map[string]string{"cache": "cache"}))
				config := buildConfig(t, job, vars)

				if tc.expectExtract {
					require.Len(t, config.CacheExtract, 1)
				} else {
					assert.Empty(t, config.CacheExtract)
				}
			})
		}
	})

	t.Run("with descriptor", func(t *testing.T) {
		job := baseJob()
		job.Cache = []spec.Cache{
			{Key: "my-cache", Paths: []string{"build/"}, Policy: spec.CachePolicyPull},
		}

		vars := newTestVars(t, nil, expandValues(map[string]string{"my-cache": "my-cache"}))

		desc := cacheprovider.Descriptor{URL: "https://storage.example.com/cache", GoCloudURL: true}

		config := buildConfig(t, job, vars,
			WithCacheDownloadDescriptor(func(key string) (cacheprovider.Descriptor, error) {
				assert.Equal(t, "my-cache", key)
				return desc, nil
			}),
		)

		require.Len(t, config.CacheExtract, 1)
		assert.Equal(t, desc.URL, config.CacheExtract[0].Sources[0].Descriptor.URL)
		assert.True(t, config.CacheExtract[0].Sources[0].Descriptor.GoCloudURL)
	})

	t.Run("fallback keys", func(t *testing.T) {
		job := baseJob()
		job.Cache = []spec.Cache{
			{Key: "primary", FallbackKeys: []string{"fb-1", "fb-2"}, Paths: []string{"build/"}, Policy: spec.CachePolicyPullPush},
		}

		vars := newTestVars(t, nil, expandValues(map[string]string{
			"primary": "primary",
			"fb-1":    "fb-1",
			"fb-2":    "fb-2",
		}))

		config := buildConfig(t, job, vars)

		require.Len(t, config.CacheExtract, 1)
		require.Len(t, config.CacheExtract[0].Sources, 3)
		assert.Equal(t, "primary", config.CacheExtract[0].Sources[0].Name)
		assert.Equal(t, "fb-1", config.CacheExtract[0].Sources[1].Name)
		assert.Equal(t, "fb-2", config.CacheExtract[0].Sources[2].Name)
	})

	t.Run("CACHE_FALLBACK_KEY from env", func(t *testing.T) {
		job := baseJob()
		job.Cache = []spec.Cache{
			{Key: "primary", Paths: []string{"build/"}, Policy: spec.CachePolicyPullPush},
		}

		vars := newTestVars(t, map[string]string{"CACHE_FALLBACK_KEY": "env-fallback"},
			expandValues(map[string]string{
				"primary":      "primary",
				"env-fallback": "env-fallback",
			}),
		)

		config := buildConfig(t, job, vars)

		require.Len(t, config.CacheExtract, 1)
		require.Len(t, config.CacheExtract[0].Sources, 2)
		assert.Equal(t, "env-fallback", config.CacheExtract[0].Sources[1].Name)
	})

	t.Run("skips when no paths", func(t *testing.T) {
		job := baseJob()
		job.Cache = []spec.Cache{
			{Key: "no-paths", Policy: spec.CachePolicyPullPush},
		}

		config := buildConfig(t, job, newTestVars(t, nil))
		assert.Empty(t, config.CacheExtract)
	})

	t.Run("skips push-only policy", func(t *testing.T) {
		job := baseJob()
		job.Cache = []spec.Cache{
			{Key: "push-only", Paths: []string{"build/"}, Policy: spec.CachePolicyPush},
		}

		config := buildConfig(t, job, newTestVars(t, nil))
		assert.Empty(t, config.CacheExtract)
	})

	t.Run("hashed cache keys", func(t *testing.T) {
		job := baseJob()
		job.Cache = []spec.Cache{
			{Key: "my-key", Paths: []string{"build/"}, Policy: spec.CachePolicyPullPush},
		}

		vars := newTestVars(t, nil, expandValues(map[string]string{"my-key": "my-key"}))

		ff := func(name string) bool { return name == featureflags.HashCacheKeys }
		config := buildConfig(t, job, vars, WithFeatureFlagProvider(ff))

		require.Len(t, config.CacheExtract, 1)
		src := config.CacheExtract[0].Sources[0]
		assert.Equal(t, "my-key", src.Name)
		assert.NotEqual(t, "my-key", src.Key)
		assert.Len(t, src.Key, 64)
	})

	t.Run("key is expanded via variables", func(t *testing.T) {
		job := baseJob()
		job.Cache = []spec.Cache{
			{Key: "$BRANCH_NAME", Paths: []string{"build/"}, Policy: spec.CachePolicyPullPush, When: spec.CacheWhenOnSuccess},
		}

		vars := newTestVars(t, nil, expandValues(map[string]string{"$BRANCH_NAME": "feature/cool-thing"}))
		config := buildConfig(t, job, vars)

		require.Len(t, config.CacheExtract, 1)
		assert.Equal(t, "feature/cool-thing", config.CacheExtract[0].Sources[0].Name)

		require.Len(t, config.CacheArchive, 1)
		assert.Equal(t, "feature/cool-thing", config.CacheArchive[0].Name)
	})

	t.Run("empty key defaults to jobName/ref", func(t *testing.T) {
		job := baseJob()
		job.Cache = []spec.Cache{
			{Key: "", Paths: []string{"build/"}, Policy: spec.CachePolicyPullPush, When: spec.CacheWhenOnSuccess},
		}

		vars := newTestVars(t, nil)
		config := buildConfig(t, job, vars)

		expectedKey := "test-job/main"
		require.Len(t, config.CacheExtract, 1)
		assert.Equal(t, expectedKey, config.CacheExtract[0].Sources[0].Name)

		require.Len(t, config.CacheArchive, 1)
		assert.Equal(t, expectedKey, config.CacheArchive[0].Name)
	})

	t.Run("download descriptor receives resolved key not raw key", func(t *testing.T) {
		job := baseJob()
		job.Cache = []spec.Cache{
			{Key: "my-cache", Paths: []string{"build/"}, Policy: spec.CachePolicyPull},
		}

		vars := newTestVars(t, nil, expandValues(map[string]string{"my-cache": "my-cache"}))
		ff := func(f string) bool { return f == featureflags.HashCacheKeys }

		var receivedKey string
		config := buildConfig(t, job, vars,
			WithFeatureFlagProvider(ff),
			WithCacheDownloadDescriptor(func(key string) (cacheprovider.Descriptor, error) {
				receivedKey = key
				return cacheprovider.Descriptor{}, nil
			}),
		)

		require.Len(t, config.CacheExtract, 1)
		src := config.CacheExtract[0].Sources[0]
		assert.Equal(t, src.Key, receivedKey,
			"download descriptor should receive the resolved (hashed) key, not the raw cache key")
		assert.NotEqual(t, "my-cache", receivedKey)
		assert.Len(t, receivedKey, 64)
	})

	t.Run("sanitized key produces warning", func(t *testing.T) {
		job := baseJob()
		job.Cache = []spec.Cache{
			{Key: `foo\bar`, Paths: []string{"build/"}, Policy: spec.CachePolicyPullPush},
		}

		vars := newTestVars(t, nil, expandValues(map[string]string{`foo\bar`: `foo\bar`}))
		config := buildConfig(t, job, vars)

		require.Len(t, config.CacheExtract, 1)
		src := config.CacheExtract[0].Sources[0]
		assert.Equal(t, "foo/bar", src.Name)
		assert.Equal(t, "foo/bar", src.Key)
		require.Len(t, src.Warnings, 1)
		assert.Contains(t, src.Warnings[0], `cache key "foo\\bar" sanitized to "foo/bar"`)
	})
}

func TestBuild_CacheExtract_ProtectedFallbackKey(t *testing.T) {
	tests := map[string]struct {
		fallbackKey   string
		expectBlocked bool
	}{
		"blocked":           {fallbackKey: "some-key-protected", expectBlocked: true},
		"blocked with dots": {fallbackKey: "some-key-protected. ", expectBlocked: true},
		"allowed":           {fallbackKey: "some-key-safe", expectBlocked: false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			job := baseJob()
			job.Cache = []spec.Cache{
				{Key: "primary", Paths: []string{"build/"}, Policy: spec.CachePolicyPullPush},
			}

			expands := map[string]string{"primary": "primary"}
			if !tc.expectBlocked {
				expands[tc.fallbackKey] = tc.fallbackKey
			}

			vars := newTestVars(t, map[string]string{"CACHE_FALLBACK_KEY": tc.fallbackKey},
				expandValues(expands),
			)
			config := buildConfig(t, job, vars)

			require.Len(t, config.CacheExtract, 1)
			if tc.expectBlocked {
				assert.Len(t, config.CacheExtract[0].Sources, 1)
				assert.NotEmpty(t, config.CacheExtract[0].Warnings)
			} else {
				assert.Len(t, config.CacheExtract[0].Sources, 2)
			}
		})
	}
}

func TestBuild_CacheArchive(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		job := baseJob()
		job.Cache = []spec.Cache{
			{Key: "archive-key", Paths: []string{"dist/"}, Policy: spec.CachePolicyPullPush, When: spec.CacheWhenOnSuccess},
		}

		vars := newTestVars(t, nil, expandValues(map[string]string{"archive-key": "archive-key"}))
		config := buildConfig(t, job, vars)

		require.Len(t, config.CacheArchive, 1)
		assert.Equal(t, "archive-key", config.CacheArchive[0].Name)
		assert.Equal(t, "archive-key", config.CacheArchive[0].Key)
		assert.Equal(t, []string{"dist/"}, config.CacheArchive[0].Paths)
		assert.True(t, config.CacheArchive[0].OnSuccess)
		assert.False(t, config.CacheArchive[0].OnFailure)
	})

	t.Run("policy filtering", func(t *testing.T) {
		tests := map[string]struct {
			policy        spec.CachePolicy
			expectArchive bool
		}{
			"undefined archives": {policy: spec.CachePolicyUndefined, expectArchive: true},
			"pull-push archives": {policy: spec.CachePolicyPullPush, expectArchive: true},
			"push archives":      {policy: spec.CachePolicyPush, expectArchive: true},
			"pull skips":         {policy: spec.CachePolicyPull, expectArchive: false},
		}

		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				job := baseJob()
				job.Cache = []spec.Cache{
					{Key: "cache", Paths: []string{"build/"}, Policy: tc.policy, When: spec.CacheWhenOnSuccess},
				}

				vars := newTestVars(t, nil, expandValues(map[string]string{"cache": "cache"}))
				config := buildConfig(t, job, vars)

				if tc.expectArchive {
					require.Len(t, config.CacheArchive, 1)
				} else {
					assert.Empty(t, config.CacheArchive)
				}
			})
		}
	})

	t.Run("when conditions", func(t *testing.T) {
		tests := map[string]struct {
			when      spec.CacheWhen
			onSuccess bool
			onFailure bool
		}{
			"on_success": {when: spec.CacheWhenOnSuccess, onSuccess: true, onFailure: false},
			"on_failure": {when: spec.CacheWhenOnFailure, onSuccess: false, onFailure: true},
			"always":     {when: spec.CacheWhenAlways, onSuccess: true, onFailure: true},
		}

		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				job := baseJob()
				job.Cache = []spec.Cache{
					{Key: "cache", Paths: []string{"build/"}, Policy: spec.CachePolicyPush, When: tc.when},
				}

				vars := newTestVars(t, nil, expandValues(map[string]string{"cache": "cache"}))
				config := buildConfig(t, job, vars)

				require.Len(t, config.CacheArchive, 1)
				assert.Equal(t, tc.onSuccess, config.CacheArchive[0].OnSuccess)
				assert.Equal(t, tc.onFailure, config.CacheArchive[0].OnFailure)
			})
		}
	})

	t.Run("skips pull-only policy", func(t *testing.T) {
		job := baseJob()
		job.Cache = []spec.Cache{
			{Key: "pull-only", Paths: []string{"build/"}, Policy: spec.CachePolicyPull},
		}

		vars := newTestVars(t, nil, expandValues(map[string]string{"pull-only": "pull-only"}))
		config := buildConfig(t, job, vars)
		assert.Empty(t, config.CacheArchive)
	})

	t.Run("with descriptor", func(t *testing.T) {
		job := baseJob()
		job.Cache = []spec.Cache{
			{Key: "upload-cache", Paths: []string{"build/"}, Policy: spec.CachePolicyPush, When: spec.CacheWhenOnSuccess},
		}

		vars := newTestVars(t, nil, expandValues(map[string]string{"upload-cache": "upload-cache"}))

		desc := cacheprovider.Descriptor{
			URL:     "https://storage.example.com/upload",
			Headers: map[string][]string{"X-Custom": {"val"}},
		}

		config := buildConfig(t, job, vars,
			WithCacheUploadDescriptor(func(key string) (cacheprovider.Descriptor, error) {
				assert.Equal(t, "upload-cache", key)
				return desc, nil
			}),
		)

		require.Len(t, config.CacheArchive, 1)
		assert.Equal(t, desc.URL, config.CacheArchive[0].Descriptor.URL)
		assert.Equal(t, desc.Headers, config.CacheArchive[0].Descriptor.Headers)
	})

	t.Run("upload descriptor receives resolved key not raw key", func(t *testing.T) {
		job := baseJob()
		job.Cache = []spec.Cache{
			{Key: "my-cache", Paths: []string{"build/"}, Policy: spec.CachePolicyPush, When: spec.CacheWhenOnSuccess},
		}

		vars := newTestVars(t, nil, expandValues(map[string]string{"my-cache": "my-cache"}))
		ff := func(f string) bool { return f == featureflags.HashCacheKeys }

		var receivedKey string
		config := buildConfig(t, job, vars,
			WithFeatureFlagProvider(ff),
			WithCacheUploadDescriptor(func(key string) (cacheprovider.Descriptor, error) {
				receivedKey = key
				return cacheprovider.Descriptor{}, nil
			}),
		)

		require.Len(t, config.CacheArchive, 1)
		assert.Equal(t, config.CacheArchive[0].Key, receivedKey,
			"upload descriptor should receive the resolved (hashed) key, not the raw cache key")
		assert.NotEqual(t, "my-cache", receivedKey,
			"with HashCacheKeys on, the descriptor key should be the hash, not the human key")
		assert.Len(t, receivedKey, 64)
	})

	t.Run("max upload size", func(t *testing.T) {
		job := baseJob()
		job.Cache = []spec.Cache{
			{Key: "sized", Paths: []string{"build/"}, Policy: spec.CachePolicyPush, When: spec.CacheWhenOnSuccess},
		}

		vars := newTestVars(t, nil, expandValues(map[string]string{"sized": "sized"}))
		config := buildConfig(t, job, vars, WithCacheMaxArchiveSize(100*1024*1024))

		require.Len(t, config.CacheArchive, 1)
		assert.Equal(t, int64(100*1024*1024), config.CacheArchive[0].MaxUploadedArchiveSize)
	})

	t.Run("sanitized key produces warning", func(t *testing.T) {
		job := baseJob()
		job.Cache = []spec.Cache{
			{Key: `path\key`, Paths: []string{"build/"}, Policy: spec.CachePolicyPush, When: spec.CacheWhenOnSuccess},
		}

		vars := newTestVars(t, nil, expandValues(map[string]string{`path\key`: `path\key`}))
		config := buildConfig(t, job, vars)

		require.Len(t, config.CacheArchive, 1)
		assert.Equal(t, "path/key", config.CacheArchive[0].Name)
		require.Len(t, config.CacheArchive[0].Warnings, 1)
		assert.Contains(t, config.CacheArchive[0].Warnings[0], `cache key "path\\key" sanitized to "path/key"`)
	})
}

func TestBuild_ArtifactDownloads(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		job := baseJob()
		job.Dependencies = []spec.Dependency{
			{ID: 456, Token: "dep-token", Name: "build-job", ArtifactsFile: spec.DependencyArtifactsFile{Filename: "artifacts.zip"}},
			{ID: 789, Name: "no-artifact-job"},
		}

		config := buildConfig(t, job, newTestVars(t, nil))

		require.Len(t, config.ArtifactExtract, 1)
		assert.Equal(t, int64(456), config.ArtifactExtract[0].ID)
		assert.Equal(t, "dep-token", config.ArtifactExtract[0].Token)
		assert.Equal(t, "build-job", config.ArtifactExtract[0].ArtifactName)
		assert.Equal(t, "artifacts.zip", config.ArtifactExtract[0].Filename)
	})

	t.Run("skips no filename", func(t *testing.T) {
		job := baseJob()
		job.Dependencies = []spec.Dependency{
			{ID: 789, Name: "no-artifact"},
		}

		config := buildConfig(t, job, newTestVars(t, nil))
		assert.Empty(t, config.ArtifactExtract)
	})
}

func TestBuild_ArtifactUploads(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		job := baseJob()
		job.Artifacts = []spec.Artifact{
			{
				Name: "my-artifact", Paths: []string{"dist/**"}, Exclude: []string{"dist/temp"},
				ExpireIn: "1 week", Format: spec.ArtifactFormatZip, Type: "archive",
				When: spec.ArtifactWhenOnSuccess,
			},
		}

		config := buildConfig(t, job, newTestVars(t, nil),
			WithArtifactTimeouts(30*time.Minute, 10*time.Minute),
		)

		require.Len(t, config.ArtifactsArchive, 1)
		a := config.ArtifactsArchive[0]
		assert.Equal(t, "my-artifact", a.ArtifactName)
		assert.Equal(t, []string{"dist/**"}, a.Paths)
		assert.Equal(t, []string{"dist/temp"}, a.Exclude)
		assert.Equal(t, "1 week", a.ExpireIn)
		assert.Equal(t, "zip", a.Format)
		assert.Equal(t, "archive", a.Type)
		assert.True(t, a.OnSuccess)
		assert.False(t, a.OnFailure)
		assert.Equal(t, 30*time.Minute, a.Timeout)
		assert.Equal(t, 10*time.Minute, a.ResponseHeaderTimeout)
	})

	t.Run("default when is success", func(t *testing.T) {
		job := baseJob()
		job.Artifacts = []spec.Artifact{{Paths: []string{"output/"}}}

		config := buildConfig(t, job, newTestVars(t, nil))

		require.Len(t, config.ArtifactsArchive, 1)
		assert.True(t, config.ArtifactsArchive[0].OnSuccess)
		assert.False(t, config.ArtifactsArchive[0].OnFailure)
	})

	t.Run("skips no paths", func(t *testing.T) {
		job := baseJob()
		job.Artifacts = []spec.Artifact{{Name: "empty"}}

		config := buildConfig(t, job, newTestVars(t, nil))
		assert.Empty(t, config.ArtifactsArchive)
	})
}

func TestBuild_ArtifactMetadata(t *testing.T) {
	t.Run("generated for zip with flag", func(t *testing.T) {
		job := baseJob()
		job.Artifacts = []spec.Artifact{
			{Paths: []string{"dist/"}, Format: spec.ArtifactFormatZip, When: spec.ArtifactWhenOnSuccess},
		}
		job.Variables = spec.Variables{{Key: "VAR1"}, {Key: "VAR2"}}

		vars := newTestVars(t, map[string]string{"GENERATE_ARTIFACTS_METADATA": "true"})

		config := buildConfig(t, job, vars,
			WithExecutorName("docker"),
			WithRunnerName("my-runner"),
			WithStartedAt(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)),
		)

		require.Len(t, config.ArtifactsArchive, 1)
		meta := config.ArtifactsArchive[0].Metadata
		require.NotNil(t, meta)
		assert.Equal(t, "42", meta.RunnerID)
		assert.Equal(t, "https://gitlab.example.com/group/project", meta.RepoURL)
		assert.Equal(t, "abc123def456", meta.RepoDigest)
		assert.Equal(t, "test-job", meta.JobName)
		assert.Equal(t, "docker", meta.ExecutorName)
		assert.Equal(t, "my-runner", meta.RunnerName)
		assert.Equal(t, "2025-01-15T10:00:00Z", meta.StartedAt)
		assert.Equal(t, "unknown", meta.SchemaVersion)
		assert.Equal(t, []string{"VAR1", "VAR2"}, meta.Parameters)
	})

	t.Run("not generated for non-zip", func(t *testing.T) {
		job := baseJob()
		job.Artifacts = []spec.Artifact{
			{Paths: []string{"dist/"}, Format: spec.ArtifactFormatGzip, When: spec.ArtifactWhenOnSuccess},
		}

		vars := newTestVars(t, map[string]string{"GENERATE_ARTIFACTS_METADATA": "true"})
		config := buildConfig(t, job, vars)

		require.Len(t, config.ArtifactsArchive, 1)
		assert.Nil(t, config.ArtifactsArchive[0].Metadata)
	})

	t.Run("not generated when flag off", func(t *testing.T) {
		job := baseJob()
		job.Artifacts = []spec.Artifact{
			{Paths: []string{"dist/"}, Format: spec.ArtifactFormatZip, When: spec.ArtifactWhenOnSuccess},
		}

		config := buildConfig(t, job, newTestVars(t, nil))

		require.Len(t, config.ArtifactsArchive, 1)
		assert.Nil(t, config.ArtifactsArchive[0].Metadata)
	})
}

func TestBuild_Cleanup(t *testing.T) {
	vars := newTestVars(t, map[string]string{
		"GIT_STRATEGY":           "clone",
		"GIT_SUBMODULE_STRATEGY": "recursive",
		"GIT_CLEAN_FLAGS":        "-ffdx -e .env",
	})

	ff := func(name string) bool { return name == featureflags.EnableJobCleanup }
	config := buildConfig(t, baseJob(), vars, WithGitCleanConfig(true), WithFeatureFlagProvider(ff))

	assert.Equal(t, "clone", config.Cleanup.GitStrategy)
	assert.Equal(t, "recursive", config.Cleanup.SubmoduleStrategy)
	assert.Equal(t, []string{"-ffdx", "-e", ".env"}, config.Cleanup.GitCleanFlags)
	assert.True(t, config.Cleanup.EnableJobCleanup)
	assert.True(t, config.Cleanup.CleanGitConfig)
}

func TestBuild_FeatureFlags(t *testing.T) {
	t.Run("HashCacheKeys", func(t *testing.T) {
		job := baseJob()
		job.Cache = []spec.Cache{
			{Key: "my-key", Paths: []string{"build/"}, Policy: spec.CachePolicyPullPush, When: spec.CacheWhenOnSuccess},
		}

		tests := map[string]struct {
			enabled  bool
			checkKey func(t *testing.T, name, key string)
		}{
			"off uses sanitized human key": {
				enabled: false,
				checkKey: func(t *testing.T, name, key string) {
					assert.Equal(t, "my-key", name)
					assert.Equal(t, "my-key", key)
					assert.Equal(t, name, key, "name and key should be identical when hashing is off")
				},
			},
			"on uses human name but hashed key": {
				enabled: true,
				checkKey: func(t *testing.T, name, key string) {
					assert.Equal(t, "my-key", name, "human name should be preserved")
					assert.NotEqual(t, name, key, "key should differ from name when hashed")
					assert.Len(t, key, 64, "hashed key should be sha256 hex")
				},
			},
		}

		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				vars := newTestVars(t, nil, expandValues(map[string]string{"my-key": "my-key"}))
				ff := func(f string) bool { return f == featureflags.HashCacheKeys && tc.enabled }
				config := buildConfig(t, job, vars, WithFeatureFlagProvider(ff))

				require.Len(t, config.CacheExtract, 1)
				src := config.CacheExtract[0].Sources[0]
				tc.checkKey(t, src.Name, src.Key)

				require.Len(t, config.CacheArchive, 1)
				tc.checkKey(t, config.CacheArchive[0].Name, config.CacheArchive[0].Key)
			})
		}
	})

	t.Run("HashCacheKeys skips sanitization", func(t *testing.T) {
		job := baseJob()
		job.Cache = []spec.Cache{
			{Key: `foo\bar`, Paths: []string{"build/"}, Policy: spec.CachePolicyPullPush, When: spec.CacheWhenOnSuccess},
		}

		tests := map[string]struct {
			enabled        bool
			expectedName   string
			expectWarnings bool
		}{
			"off sanitizes key": {
				enabled:        false,
				expectedName:   "foo/bar",
				expectWarnings: true,
			},
			"on preserves raw key": {
				enabled:        true,
				expectedName:   `foo\bar`,
				expectWarnings: false,
			},
		}

		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				vars := newTestVars(t, nil, expandValues(map[string]string{`foo\bar`: `foo\bar`}))
				ff := func(f string) bool { return f == featureflags.HashCacheKeys && tc.enabled }
				config := buildConfig(t, job, vars, WithFeatureFlagProvider(ff))

				require.Len(t, config.CacheExtract, 1)
				src := config.CacheExtract[0].Sources[0]
				assert.Equal(t, tc.expectedName, src.Name)
				if tc.expectWarnings {
					assert.NotEmpty(t, src.Warnings)
				} else {
					assert.Empty(t, src.Warnings)
				}
			})
		}
	})

	t.Run("CleanUpFailedCacheExtract", func(t *testing.T) {
		job := baseJob()
		job.Cache = []spec.Cache{
			{Key: "cache", Paths: []string{"build/"}, Policy: spec.CachePolicyPull},
		}

		tests := map[string]struct {
			enabled  bool
			expected bool
		}{
			"off": {enabled: false, expected: false},
			"on":  {enabled: true, expected: true},
		}

		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				vars := newTestVars(t, nil, expandValues(map[string]string{"cache": "cache"}))
				ff := func(f string) bool { return f == featureflags.CleanUpFailedCacheExtract && tc.enabled }
				config := buildConfig(t, job, vars, WithFeatureFlagProvider(ff))

				require.Len(t, config.CacheExtract, 1)
				assert.Equal(t, tc.expected, config.CacheExtract[0].CleanupFailedExtract)
			})
		}
	})

	t.Run("EnableBashExitCodeCheck", func(t *testing.T) {
		job := baseJob()
		job.Steps = []spec.Step{
			{Name: "build", Script: []string{"make"}, When: spec.StepWhenOnSuccess},
		}

		tests := map[string]struct {
			enabled  bool
			expected bool
		}{
			"off": {enabled: false, expected: false},
			"on":  {enabled: true, expected: true},
		}

		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				ff := func(f string) bool { return f == featureflags.EnableBashExitCodeCheck && tc.enabled }
				config := buildConfig(t, job, newTestVars(t, nil), WithFeatureFlagProvider(ff))

				for _, step := range config.Steps {
					assert.Equal(t, tc.expected, step.BashExitCodeCheck)
				}

				assert.Equal(t, tc.expected, config.GetSources.PreCloneStep.BashExitCodeCheck)
				assert.Equal(t, tc.expected, config.GetSources.PostCloneStep.BashExitCodeCheck)
			})
		}
	})

	t.Run("ScriptSections", func(t *testing.T) {
		job := baseJob()
		job.Steps = []spec.Step{
			{Name: "build", Script: []string{"make"}, When: spec.StepWhenOnSuccess},
		}

		tests := map[string]struct {
			ffEnabled     bool
			traceSections bool
			expected      bool
		}{
			"both off":         {ffEnabled: false, traceSections: false, expected: false},
			"ff on, trace off": {ffEnabled: true, traceSections: false, expected: false},
			"ff off, trace on": {ffEnabled: false, traceSections: true, expected: false},
			"both on":          {ffEnabled: true, traceSections: true, expected: true},
		}

		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				j := job
				j.Features = spec.GitlabFeatures{TraceSections: tc.traceSections}

				ff := func(f string) bool { return f == featureflags.ScriptSections && tc.ffEnabled }
				config := buildConfig(t, j, newTestVars(t, nil), WithFeatureFlagProvider(ff))

				for _, step := range config.Steps {
					assert.Equal(t, tc.expected, step.ScriptSections,
						"step %s: ScriptSections should be %v", step.Step, tc.expected)
				}
			})
		}
	})

	t.Run("EnableJobCleanup", func(t *testing.T) {
		tests := map[string]struct {
			enabled  bool
			expected bool
		}{
			"off": {enabled: false, expected: false},
			"on":  {enabled: true, expected: true},
		}

		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				ff := func(f string) bool { return f == featureflags.EnableJobCleanup && tc.enabled }
				config := buildConfig(t, baseJob(), newTestVars(t, nil), WithFeatureFlagProvider(ff))
				assert.Equal(t, tc.expected, config.Cleanup.EnableJobCleanup)
			})
		}
	})

	t.Run("UseGitNativeClone", func(t *testing.T) {
		tests := map[string]struct {
			enabled  bool
			expected bool
		}{
			"off": {enabled: false, expected: false},
			"on":  {enabled: true, expected: true},
		}

		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				ff := func(f string) bool { return f == featureflags.UseGitNativeClone && tc.enabled }
				config := buildConfig(t, baseJob(), newTestVars(t, nil), WithFeatureFlagProvider(ff))
				assert.Equal(t, tc.expected, config.GetSources.UseNativeClone)
			})
		}
	})

	t.Run("UseGitBundleURIs", func(t *testing.T) {
		tests := map[string]struct {
			enabled  bool
			expected bool
		}{
			"off": {enabled: false, expected: false},
			"on":  {enabled: true, expected: true},
		}

		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				ff := func(f string) bool { return f == featureflags.UseGitBundleURIs && tc.enabled }
				config := buildConfig(t, baseJob(), newTestVars(t, nil), WithFeatureFlagProvider(ff))
				assert.Equal(t, tc.expected, config.GetSources.UseBundleURIs)
			})
		}
	})

	t.Run("UseGitProactiveAuth", func(t *testing.T) {
		tests := map[string]struct {
			enabled  bool
			expected bool
		}{
			"off": {enabled: false, expected: false},
			"on":  {enabled: true, expected: true},
		}

		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				ff := func(f string) bool { return f == featureflags.UseGitProactiveAuth && tc.enabled }
				config := buildConfig(t, baseJob(), newTestVars(t, nil), WithFeatureFlagProvider(ff))
				assert.Equal(t, tc.expected, config.GetSources.UseProactiveAuth)
			})
		}
	})

	t.Run("GitURLsWithoutTokens", func(t *testing.T) {
		tests := map[string]struct {
			enabled  bool
			expected bool
		}{
			"off": {enabled: false, expected: false},
			"on":  {enabled: true, expected: true},
		}

		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				ff := func(f string) bool { return f == featureflags.GitURLsWithoutTokens && tc.enabled }
				config := buildConfig(t, baseJob(), newTestVars(t, nil), WithFeatureFlagProvider(ff))
				assert.Equal(t, tc.expected, config.GetSources.UseCredentialHelper)
			})
		}
	})
}

func TestBuild_OptionsWiring(t *testing.T) {
	t.Run("ArchiverStagingDir", func(t *testing.T) {
		config := buildConfig(t, baseJob(), newTestVars(t, nil),
			WithArchiverStagingDir("/tmp/staging"),
		)
		assert.Equal(t, "/tmp/staging", config.ArchiverStagingDir)
	})

	t.Run("LoginShell", func(t *testing.T) {
		config := buildConfig(t, baseJob(), newTestVars(t, nil),
			WithLoginShell(true),
		)
		assert.True(t, config.LoginShell)
	})

	t.Run("LoginShell default false", func(t *testing.T) {
		config := buildConfig(t, baseJob(), newTestVars(t, nil))
		assert.False(t, config.LoginShell)
	})
}
