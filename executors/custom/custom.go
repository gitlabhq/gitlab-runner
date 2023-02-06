package custom

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/executors/custom/api"
	"gitlab.com/gitlab-org/gitlab-runner/executors/custom/command"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/process"
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

type ConfigExecOutput struct {
	api.ConfigExecOutput
}

type jsonService struct {
	Name       string   `json:"name"`
	Alias      string   `json:"alias"`
	Entrypoint []string `json:"entrypoint"`
	Command    []string `json:"command"`
}

func (c *ConfigExecOutput) InjectInto(executor *executor) {
	if c.Hostname != nil {
		executor.Build.Hostname = *c.Hostname
	}

	if c.BuildsDir != nil {
		executor.Config.BuildsDir = *c.BuildsDir
	}

	if c.CacheDir != nil {
		executor.Config.CacheDir = *c.CacheDir
	}

	if c.BuildsDirIsShared != nil {
		executor.SharedBuildsDir = *c.BuildsDirIsShared
	}

	executor.driverInfo = c.Driver

	if c.JobEnv != nil {
		executor.jobEnv = *c.JobEnv
	}
}

type executor struct {
	executors.AbstractExecutor

	config          *config
	tempDir         string
	jobResponseFile string

	driverInfo *api.DriverInfo

	jobEnv map[string]string
}

func (e *executor) Name() string {
	return "custom"
}

func (e *executor) Prepare(options common.ExecutorPrepareOptions) error {
	e.AbstractExecutor.PrepareConfiguration(options)

	err := e.prepareConfig()
	if err != nil {
		return err
	}

	e.tempDir, err = os.MkdirTemp("", "custom-executor")
	if err != nil {
		return err
	}

	e.jobResponseFile, err = e.createJobResponseFile()
	if err != nil {
		return err
	}

	err = e.dynamicConfig()
	if err != nil {
		return err
	}

	e.logStartupMessage()

	err = e.AbstractExecutor.PrepareBuildAndShell()
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

func (e *executor) createJobResponseFile() (string, error) {
	responseFile := filepath.Join(e.tempDir, "response.json")
	file, err := os.OpenFile(responseFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return "", fmt.Errorf("creating job response file %q: %w", responseFile, err)
	}
	defer func() { _ = file.Close() }()

	encoder := json.NewEncoder(file)
	err = encoder.Encode(e.Build.JobResponse)
	if err != nil {
		return "", fmt.Errorf("encoding job response file: %w", err)
	}

	return responseFile, nil
}

func (e *executor) dynamicConfig() error {
	if e.config.ConfigExec == "" {
		return nil
	}

	ctx, cancelFunc := context.WithTimeout(e.Context, e.config.GetConfigExecTimeout())
	defer cancelFunc()

	buf := bytes.NewBuffer(nil)
	outputs := commandOutputs{
		stdout: buf,
		stderr: e.Trace,
	}

	opts := prepareCommandOpts{
		executable: e.config.ConfigExec,
		args:       e.config.ConfigArgs,
		out:        outputs,
	}

	err := e.prepareCommand(ctx, opts).Run()
	if err != nil {
		return err
	}

	jsonConfig := buf.Bytes()
	if len(jsonConfig) < 1 {
		return nil
	}

	config := new(ConfigExecOutput)

	err = json.Unmarshal(jsonConfig, config)
	if err != nil {
		return fmt.Errorf("error while parsing JSON output: %w", err)
	}

	config.InjectInto(e)

	return nil
}

func (e *executor) logStartupMessage() {
	const usageLine = "Using Custom executor"

	info := e.driverInfo
	if info == nil || info.Name == nil {
		e.Println(fmt.Sprintf("%s...", usageLine))
		return
	}

	if info.Version == nil {
		e.Println(fmt.Sprintf("%s with driver %s...", usageLine, *info.Name))
		return
	}

	e.Println(fmt.Sprintf("%s with driver %s %s...", usageLine, *info.Name, *info.Version))
}

func (e *executor) defaultCommandOutputs() commandOutputs {
	return commandOutputs{
		stdout: e.Trace,
		stderr: e.Trace,
	}
}

var commandFactory = command.New

func (e *executor) prepareCommand(ctx context.Context, opts prepareCommandOpts) command.Command {
	logger := common.NewProcessLoggerAdapter(e.BuildLogger)

	cmdOpts := process.CommandOptions{
		Dir:                             e.tempDir,
		Env:                             make([]string, 0),
		Stdout:                          opts.out.stdout,
		Stderr:                          opts.out.stderr,
		Logger:                          logger,
		GracefulKillTimeout:             e.config.GetGracefulKillTimeout(),
		ForceKillTimeout:                e.config.GetForceKillTimeout(),
		UseWindowsLegacyProcessStrategy: e.Build.IsFeatureFlagOn(featureflags.UseWindowsLegacyProcessStrategy),
	}

	// Append job_env defined variable first to avoid overwriting any CI/CD or predefined variables.
	for k, v := range e.jobEnv {
		cmdOpts.Env = append(cmdOpts.Env, fmt.Sprintf("%s=%s", k, v))
	}

	variables := append(e.Build.GetAllVariables(), e.getCIJobServicesEnv())
	for _, variable := range variables {
		cmdOpts.Env = append(cmdOpts.Env, fmt.Sprintf("CUSTOM_ENV_%s=%s", variable.Key, variable.Value))
	}

	options := command.Options{
		JobResponseFile: e.jobResponseFile,
	}

	return commandFactory(ctx, opts.executable, opts.args, cmdOpts, options)
}

func (e *executor) getCIJobServicesEnv() common.JobVariable {
	if len(e.Build.Services) == 0 {
		return common.JobVariable{Key: "CI_JOB_SERVICES"}
	}

	var services []jsonService
	for _, service := range e.Build.Services {
		services = append(services, jsonService{
			Name:       service.Name,
			Alias:      append(service.Aliases(), "")[0],
			Entrypoint: service.Entrypoint,
			Command:    service.Command,
		})
	}

	servicesSerialized, err := json.Marshal(services)
	if err != nil {
		e.Warningln("Unable to serialize CI_JOB_SERVICES json:", err)
	}

	return common.JobVariable{
		Key:   "CI_JOB_SERVICES",
		Value: string(servicesSerialized),
	}
}

func (e *executor) Run(cmd common.ExecutorCommand) error {
	scriptDir, err := os.MkdirTemp(e.tempDir, "script")
	if err != nil {
		return err
	}

	scriptFile := filepath.Join(scriptDir, "script."+e.BuildShell.Extension)
	err = os.WriteFile(scriptFile, []byte(cmd.Script), 0o700)
	if err != nil {
		return err
	}

	// TODO: Remove this translation - https://gitlab.com/groups/gitlab-org/-/epics/6112
	stage := cmd.Stage
	if stage == "step_script" {
		e.BuildLogger.Warningln("Starting with version 17.0 the 'build_script' stage " +
			"will be replaced with 'step_script': https://gitlab.com/groups/gitlab-org/-/epics/6112")
		stage = "build_script"
	}

	args := append(e.config.RunArgs, scriptFile, string(stage))

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

	defer func() { _ = os.RemoveAll(e.tempDir) }()

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

	common.RegisterExecutorProvider("custom", executors.DefaultExecutorProvider{
		Creator:          creator,
		FeaturesUpdater:  featuresUpdater,
		DefaultShellName: options.Shell.Shell,
	})
}
