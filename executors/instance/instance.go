package instance

import (
	"errors"
	"fmt"
	"strings"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/executors/internal/autoscaler"
)

type executor struct {
	executors.AbstractExecutor
	client executors.Client
}

func (e *executor) Name() string {
	return "instance"
}

func (e *executor) Prepare(options common.ExecutorPrepareOptions) error {
	err := e.AbstractExecutor.Prepare(options)
	if err != nil {
		return fmt.Errorf("preparing AbstractExecutor: %w", err)
	}

	if e.BuildShell.PassFile {
		return errors.New("the instance executor doesn't support shells that require a script file")
	}

	// validate if the image defined is allowed if nesting is enabled
	// if nesting is not enabled, the image is irrelevant
	if options.Config.Autoscaler.VMIsolation.Enabled {
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

	e.Println("Preparing instance...")
	e.client, err = environment.Prepare(options.Context, e.BuildLogger, options)
	if err != nil {
		return fmt.Errorf("creating instance environment: %w", err)
	}

	return nil
}

func (e *executor) Run(cmd common.ExecutorCommand) error {
	return e.client.Run(cmd.Context, executors.RunOptions{
		Command: e.BuildShell.CmdLine,
		Stdin:   strings.NewReader(cmd.Script),
		Stdout:  e.Trace,
		Stderr:  e.Trace,
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
