package common

import (
	"context"
	"fmt"

	"gitlab.com/gitlab-org/gitlab-runner/cache"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/builder"
	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run/cacheprovider"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"

	"gitlab.com/gitlab-org/moa"
	"gitlab.com/gitlab-org/step-runner/schema/v1"
)

const stepRunBuildStage = BuildStage("step_" + spec.StepNameRun)

// stepDispatch converts a build stage to a list of steps to run.
//
// Depending on the configuration, this can also include stages that we're
// in the process of migrating (from scripts) to a step.
//
//nolint:gocognit
func stepDispatch(build *Build, executor Executor, stage BuildStage) (bool, []schema.Step) {
	switch stage {
	case BuildStagePrepare, BuildStageGetSources, BuildStageClearWorktree, BuildStageRestoreCache, BuildStageDownloadArtifacts, BuildStageArchiveOnSuccessCache, BuildStageArchiveOnFailureCache, BuildStageUploadOnFailureArtifacts, BuildStageUploadOnSuccessArtifacts, BuildStageCleanup:
		// don't handle non-user script stages
		return false, nil

	case stepRunBuildStage:
		return true, build.Job.Run

	case BuildStageAfterScript:
		// don't handle after_script (yet)
		return false, nil

	default: // user script
		if !build.IsFeatureFlagOn(featureflags.UseScriptToStepMigration) {
			return false, nil
		}

		shell := executor.Shell()
		if shell == nil {
			return false, nil
		}

		var script []string

		if shell.PreBuildScript != "" {
			script = append(script, shell.PreBuildScript)
		}

		for _, step := range build.Steps {
			if StepToBuildStage(step) == stage {
				script = append(script, step.Script...)
				if step.Name == "release" {
					for i, s := range step.Script {
						script[i] = build.GetAllVariables().ExpandValue(s)
					}
				}
				break
			}
		}

		if shell.PostBuildScript != "" {
			script = append(script, shell.PostBuildScript)
		}

		// if no script, no-op
		if len(script) == 0 {
			return true, nil // handled, but nothing to do
		}

		return true, []schema.Step{
			{
				Name: func(s string) *string { return &s }("user_script"),
				Step: "builtin://script_legacy",
				Inputs: schema.StepInputs{
					"script":           script,
					"debug_trace":      build.IsDebugTraceEnabled(),
					"posix_escape":     true,
					"check_for_errors": build.IsFeatureFlagOn(featureflags.EnableBashExitCodeCheck),
					"trace_sections":   build.IsFeatureFlagOn(featureflags.ScriptSections),
				},
			},
		}
	}
}

//nolint:gocognit
func stagesToConcreteStep(ctx context.Context, executor Executor) ([]schema.Step, error) {
	info := executor.Shell()
	if info == nil {
		return nil, fmt.Errorf("no shell defined for executor")
	}

	build := info.Build

	var opts []builder.Option

	opts = append(opts,
		builder.WithExecutorName(build.Runner.Executor),
		builder.WithRunnerName(build.Runner.Name),
		builder.WithStartedAt(build.startedAt),
		builder.WithDebug(build.IsDebugTraceEnabled()),
		builder.WithCloneURL(build.Runner.CloneURL),
		builder.WithShell(info.Shell),
		builder.WithLoginShell(info.Type == LoginShell),
		builder.WithCacheDir(build.CacheDir),
		builder.WithSafeDirectoryCheckout(build.SafeDirectoryCheckout),
		builder.WithArtifactTimeouts(
			build.Runner.Artifact.GetUploadTimeout(),
			build.Runner.Artifact.GetResponseHeaderTimeout(),
		),
		builder.WithPreBuildScript([]string{info.PreBuildScript}),
		builder.WithPostBuildScript([]string{info.PostBuildScript}),
		builder.WithPreCloneScript(func() []string {
			var s []string

			if info.PreGetSourcesScript != "" {
				s = append(s, info.PreGetSourcesScript)
			}

			h := info.Build.Hooks.Get(spec.HookPreGetSourcesScript)
			if len(h.Script) > 0 {
				s = append(s, h.Script...)
			}

			return s
		}()),
		builder.WithPostCloneScript(func() []string {
			var s []string

			h := info.Build.Hooks.Get(spec.HookPostGetSourcesScript)
			if len(h.Script) > 0 {
				s = append(s, h.Script...)
			}

			if info.PostGetSourcesScript != "" {
				s = append(s, info.PostGetSourcesScript)
			}

			return s
		}()),
		builder.WithGitCleanConfig(func() bool {
			// It's by default disabled for the shell executor or when the git
			// strategy is "none", and enabled otherwise; explicit
			// configuration however always has precedence.
			if build.Runner.CleanGitConfig != nil {
				return *build.Runner.CleanGitConfig
			}

			switch build.Runner.Executor {
			case "shell", "shell-integration-test":
				return false
			default:
				return true
			}
		}()),
		builder.WithGitalyCorrelationID(build.JobRequestCorrelationID),
		builder.WithUserAgent(fmt.Sprintf("%s %s %s/%s", AppVersion.Name, AppVersion.Version, AppVersion.OS, AppVersion.Architecture)),
	)

	//nolint:nestif
	if build.Runner.Cache != nil {
		opts = append(opts, builder.WithCacheMaxArchiveSize(build.Runner.Cache.MaxUploadedArchiveSize),
			builder.WithCacheDownloadDescriptor(func(cacheKey string) (cacheprovider.Descriptor, error) {
				adapter := cache.GetAdapter(build.Runner.Cache, build.GetBuildTimeout(), build.Runner.ShortDescription(), fmt.Sprintf("%d", build.JobInfo.ProjectID), cacheKey)

				goCloudURL, err := adapter.GetGoCloudURL(ctx, false)
				if goCloudURL.URL != nil {
					return cacheprovider.Descriptor{
						GoCloudURL: true,
						URL:        goCloudURL.URL.String(),
						Env:        goCloudURL.Environment,
					}, err
				}

				if url := adapter.GetDownloadURL(ctx); url.URL != nil {
					return cacheprovider.Descriptor{
						URL:     url.URL.String(),
						Headers: url.Headers,
					}, nil
				}

				return cacheprovider.Descriptor{}, nil
			}),
			builder.WithCacheUploadDescriptor(func(cacheKey string) (cacheprovider.Descriptor, error) {
				adapter := cache.GetAdapter(build.Runner.Cache, build.GetBuildTimeout(), build.Runner.ShortDescription(), fmt.Sprintf("%d", build.JobInfo.ProjectID), cacheKey)

				goCloudURL, err := adapter.GetGoCloudURL(ctx, true)
				if err != nil {
					return cacheprovider.Descriptor{}, err
				}

				if goCloudURL.URL != nil {
					return cacheprovider.Descriptor{
						GoCloudURL: true,
						URL:        goCloudURL.URL.String(),
						Env:        goCloudURL.Environment,
					}, err
				}

				url := adapter.GetUploadURL(ctx)
				if url.URL == nil {
					return cacheprovider.Descriptor{}, err
				}

				return cacheprovider.Descriptor{
					URL:     url.URL.String(),
					Headers: url.Headers,
				}, nil
			}),
		)
	}

	concrete, err := builder.Build(build.Job, build.GetAllVariables(), opts...)
	if err != nil {
		return nil, err
	}

	return []schema.Step{
		{
			// try to install git on linux by different means if not available
			// or return an error.
			//
			// todo: this is obviously terrible and must be replaced by a better mechanism.
			Name: func(s string) *string { return &s }("install_git"),
			Step: "builtin://script_legacy",
			Inputs: schema.StepInputs{
				"script":       []string{`command -v git >/dev/null 2>&1 || { (command -v apk >/dev/null 2>&1 && apk update && apk add git) || (command -v apt-get >/dev/null 2>&1 && apt-get update && apt-get install -y git) || (command -v dnf >/dev/null 2>&1 && dnf install -y git) || (command -v yum >/dev/null 2>&1 && yum install -y git) || (command -v pacman >/dev/null 2>&1 && pacman -Sy --noconfirm git) || (command -v zypper >/dev/null 2>&1 && zypper refresh && zypper install -y git) || { echo "Error: git is not available and no supported package manager found to install it." >&2; exit 1; }; } && command -v git >/dev/null 2>&1 || { echo "Error: git is not available." >&2; exit 1; }`},
				"posix_escape": true,
			},
		},
		{
			Name: func(s string) *string { return &s }("concrete"),
			Step: "builtin://concrete",
			Inputs: schema.StepInputs{
				"config": moa.EscapeTemplate(string(concrete)),
			},
		},
	}, nil
}
