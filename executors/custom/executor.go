package custom

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/executors/custom/command"
)

type commandOutputs struct {
	stdout io.Writer
	stderr io.Writer
}

type prepareCommandOpts struct {
	executable string
	args       []string
	out        commandOutputs
}

type executor struct {
	executors.AbstractExecutor

	config  *config
	tempDir string
}

func (e *executor) Prepare(options common.ExecutorPrepareOptions) error {
	err := e.AbstractExecutor.Prepare(options)
	if err != nil {
		return err
	}

	err = e.prepareConfig()
	if err != nil {
		return err
	}

	e.Println("Using Custom executor...")

	e.tempDir, err = ioutil.TempDir("", "custom-executor")
	if err != nil {
		return err
	}

	// nothing to do, as there's no prepare_script
	if e.config.PrepareExec == "" {
		return nil
	}

	ctx, cancelFunc := context.WithTimeout(e.Context, e.config.GetPrepareExecTimeout())
	defer cancelFunc()

	opts := prepareCommandOpts{
		executable: e.config.PrepareExec,
		args:       e.config.PrepareArgs,
		out:        e.defaultCommandOutputs(),
	}

	return e.prepareCommand(ctx, opts).Run()
}

func (e *executor) prepareConfig() error {
	if e.Config.Custom == nil {
		return common.MakeBuildError("custom executor not configured")
	}

	e.config = &config{
		CustomConfig: e.Config.Custom,
	}

	if e.config.RunExec == "" {
		return common.MakeBuildError("custom executor is missing RunExec")
	}

	return nil
}

func (e *executor) defaultCommandOutputs() commandOutputs {
	return commandOutputs{
		stdout: e.Trace,
		stderr: e.Trace,
	}
}

var commandFactory = command.New

func (e *executor) prepareCommand(ctx context.Context, opts prepareCommandOpts) command.Command {
	cmdOpts := command.CreateOptions{
		Dir:                 e.tempDir,
		Env:                 make([]string, 0),
		Stdout:              opts.out.stdout,
		Stderr:              opts.out.stderr,
		Logger:              e.BuildLogger,
		GracefulKillTimeout: e.config.GetGracefulKillTimeout(),
		ForceKillTimeout:    e.config.GetForceKillTimeout(),
	}

	for _, variable := range e.Build.GetAllVariables() {
		cmdOpts.Env = append(cmdOpts.Env, fmt.Sprintf("CUSTOM_ENV_%s=%s", variable.Key, variable.Value))
	}

	return commandFactory(ctx, opts.executable, opts.args, cmdOpts)
}

func (e *executor) Run(cmd common.ExecutorCommand) error {
	scriptDir, err := ioutil.TempDir(e.tempDir, "script")
	if err != nil {
		return err
	}

	scriptFile := filepath.Join(scriptDir, "script."+e.BuildShell.Extension)
	err = ioutil.WriteFile(scriptFile, []byte(cmd.Script), 0700)
	if err != nil {
		return err
	}

	args := append(e.config.RunArgs, scriptFile, string(cmd.Stage))

	opts := prepareCommandOpts{
		executable: e.config.RunExec,
		args:       args,
		out:        e.defaultCommandOutputs(),
	}

	return e.prepareCommand(cmd.Context, opts).Run()
}

func (e *executor) Cleanup() {
	e.AbstractExecutor.Cleanup()

	err := e.prepareConfig()
	if err != nil {
		e.Warningln(err)

		// at this moment we don't care about the errors
		return
	}

	// nothing to do, as there's no cleanup_script
	if e.config.CleanupExec == "" {
		return
	}

	ctx, cancelFunc := context.WithTimeout(context.Background(), e.config.GetCleanupScriptTimeout())
	defer cancelFunc()

	stdoutLogger := e.BuildLogger.WithFields(logrus.Fields{"cleanup_std": "out"})
	stderrLogger := e.BuildLogger.WithFields(logrus.Fields{"cleanup_std": "err"})

	outputs := commandOutputs{
		stdout: stdoutLogger.WriterLevel(logrus.DebugLevel),
		stderr: stderrLogger.WriterLevel(logrus.WarnLevel),
	}

	opts := prepareCommandOpts{
		executable: e.config.CleanupExec,
		args:       e.config.CleanupArgs,
		out:        outputs,
	}

	err = e.prepareCommand(ctx, opts).Run()
	if err != nil {
		e.Warningln("Cleanup script failed:", err)
	}
}

func init() {
	options := executors.ExecutorOptions{
		DefaultCustomBuildsDirEnabled: false,
		Shell: common.ShellScriptInfo{
			Shell:         common.GetDefaultShell(),
			Type:          common.NormalShell,
			RunnerCommand: "gitlab-runner",
		},
		ShowHostname: false,
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

	common.RegisterExecutor("custom", executors.DefaultExecutorProvider{
		Creator:          creator,
		FeaturesUpdater:  featuresUpdater,
		DefaultShellName: options.Shell.Shell,
	})
}
