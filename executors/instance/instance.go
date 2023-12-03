package instance

import (
	"errors"
	"fmt"
	"strings"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/executors/internal/autoscaler"
)

type executor struct {
	executors.AbstractExecutor
	client executors.Client
}

//nolint:gocognit
func (e *executor) Prepare(options common.ExecutorPrepareOptions) error {
	if options.Config.Instance != nil && options.Config.Instance.UseCommonBuildDir {
		// a common build directory can only be used if the build is isolated
		// max use count 1 or if VM isolation is on.
		if options.Config.Autoscaler.VMIsolation.Enabled || options.Config.Autoscaler.MaxUseCount == 1 {
			e.SharedBuildsDir = false
		} else {
			e.BuildLogger.Warningln("use_common_build_dir has no effect: requires vm isolation or max_use_count = 1")
		}
	}

	err := e.AbstractExecutor.Prepare(options)
	if err != nil {
		return fmt.Errorf("preparing AbstractExecutor: %w", err)
	}

	if e.BuildShell.PassFile {
		return errors.New("the instance executor doesn't support shells that require a script file")
	}

	// Validate if the image defined in a job is allowed
	//
	// If nesting is not enabled, the image is irrelevant.
	// If image is not defined on a job level there is no need for validation - runner config
	// variable will be enforced later.
	if options.Config.Autoscaler.VMIsolation.Enabled && options.Build.Image.Name != "" {
		var allowed []string
		if options.Config.Instance != nil {
			allowed = options.Config.Instance.AllowedImages
		}

		// verify image is allowed
		if err := common.VerifyAllowedImage(common.VerifyAllowedImageOptions{
			Image:         options.Build.Image.Name,
			OptionName:    "images",
			AllowedImages: allowed,
		}, e.BuildLogger); err != nil {
			return err
		}
	}

	environment, ok := e.Build.ExecutorData.(executors.Environment)
	if !ok {
		return errors.New("expected environment executor data")
	}

	e.BuildLogger.Println("Preparing instance...")
	e.client, err = environment.Prepare(options.Context, e.BuildLogger, options)
	if err != nil {
		return fmt.Errorf("creating instance environment: %w", err)
	}

	return nil
}

func (e *executor) Run(cmd common.ExecutorCommand) error {
	logger := e.BuildLogger.StreamID(buildlogger.StreamWorkLevel)

	return e.client.Run(cmd.Context, executors.RunOptions{
		Command: e.BuildShell.CmdLine,
		Stdin:   strings.NewReader(cmd.Script),
		Stdout:  logger.Stdout(),
		Stderr:  logger.Stderr(),
	})
}

func (e *executor) Cleanup() {
	if e.client != nil {
		e.client.Close()
	}
	e.AbstractExecutor.Cleanup()
}

func init() {
	options := executors.ExecutorOptions{
		DefaultCustomBuildsDirEnabled: false,
		DefaultBuildsDir:              "builds",
		DefaultCacheDir:               "cache",
		SharedBuildsDir:               true,
		Shell: common.ShellScriptInfo{
			Shell:         "bash",
			RunnerCommand: "gitlab-runner",
		},
		ShowHostname: true,
	}

	creator := func() common.Executor {
		return &executor{
			AbstractExecutor: executors.AbstractExecutor{
				ExecutorOptions: options,
			},
		}
	}

	featuresUpdater := func(features *common.FeaturesInfo) {
		features.Variables = true
		features.Shared = true
	}

	common.RegisterExecutorProvider("instance", autoscaler.New(executors.DefaultExecutorProvider{
		Creator:          creator,
		FeaturesUpdater:  featuresUpdater,
		DefaultShellName: options.Shell.Shell,
	}, autoscaler.Config{
		MapJobImageToVMImage: true,
	}))
}
