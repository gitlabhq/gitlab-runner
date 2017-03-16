package commands

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"gitlab.com/ayufan/golang-cli-helpers"
	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"
	"gopkg.in/yaml.v2"

	// Force to load all executors, executes init() on them
	_ "gitlab.com/gitlab-org/gitlab-ci-multi-runner/executors/docker"
	_ "gitlab.com/gitlab-org/gitlab-ci-multi-runner/executors/parallels"
	_ "gitlab.com/gitlab-org/gitlab-ci-multi-runner/executors/shell"
	_ "gitlab.com/gitlab-org/gitlab-ci-multi-runner/executors/ssh"
	_ "gitlab.com/gitlab-org/gitlab-ci-multi-runner/executors/virtualbox"
	"strconv"
)

type ExecCommand struct {
	common.RunnerSettings
	Job     string
	Timeout int `long:"timeout" description:"Job execution timeout (in seconds)"`
}

func (c *ExecCommand) runCommand(name string, arg ...string) (string, error) {
	cmd := exec.Command(name, arg...)
	cmd.Env = os.Environ()
	cmd.Stderr = os.Stderr
	result, err := cmd.Output()
	return string(result), err
}

func (c *ExecCommand) getCommands(commands interface{}) (string, error) {
	if lines, ok := commands.([]interface{}); ok {
		text := ""
		for _, line := range lines {
			if lineText, ok := line.(string); ok {
				text += lineText + "\n"
			} else {
				return "", errors.New("unsupported script")
			}
		}
		return text + "\n", nil
	} else if text, ok := commands.(string); ok {
		return text + "\n", nil
	} else if commands != nil {
		return "", errors.New("unsupported script")
	}
	return "", nil
}

func (c *ExecCommand) buildCommands(configBeforeScript, jobConfigBeforeScript, jobScript interface{}) (commands string, err error) {
	// get before_script
	beforeScript, err := c.getCommands(configBeforeScript)
	if err != nil {
		return
	}

	// get job before_script
	jobBeforeScript, err := c.getCommands(jobConfigBeforeScript)
	if err != nil {
		return
	}

	if jobBeforeScript == "" {
		commands += beforeScript
	} else {
		commands += jobBeforeScript
	}

	// get script
	script, err := c.getCommands(jobScript)
	if err != nil {
		return
	} else if jobScript == nil {
		err = fmt.Errorf("missing 'script' for job")
		return
	}
	commands += script
	return
}

func (c *ExecCommand) buildVariables(configVariables interface{}) (buildVariables common.BuildVariables, err error) {
	if variables, ok := configVariables.(map[string]interface{}); ok {
		for key, value := range variables {
			if valueText, ok := value.(string); ok {
				buildVariables = append(buildVariables, common.BuildVariable{
					Key:    key,
					Value:  valueText,
					Public: true,
				})
			} else {
				err = fmt.Errorf("invalid value for variable %q", key)
			}
		}
	} else if configVariables != nil {
		err = errors.New("unsupported variables")
	}
	return
}

func (c *ExecCommand) buildGlobalAndJobVariables(global, job interface{}) (buildVariables common.BuildVariables, err error) {
	buildVariables, err = c.buildVariables(global)
	if err != nil {
		return
	}

	jobVariables, err := c.buildVariables(job)
	if err != nil {
		return
	}

	buildVariables = append(buildVariables, jobVariables...)
	return
}

func (c *ExecCommand) parseYaml(job string, build *common.JobResponse) error {
	data, err := ioutil.ReadFile(".gitlab-ci.yml")
	if err != nil {
		return err
	}

	build.JobInfo.Name = job

	// parse gitlab-ci.yml
	config := make(common.BuildOptions)
	err = yaml.Unmarshal(data, config)
	if err != nil {
		return err
	}

	err = config.Sanitize()
	if err != nil {
		return err
	}

	// get job
	jobConfig, ok := config.GetSubOptions(job)
	if !ok {
		return fmt.Errorf("no job named %q", job)
	}

	build.Commands, err = c.buildCommands(config["before_script"], jobConfig["before_script"], jobConfig["script"])
	if err != nil {
		return err
	}

	build.Variables, err = c.buildGlobalAndJobVariables(config["variables"], jobConfig["variables"])
	if err != nil {
		return err
	}

	build.Image.Name = getOption("image", config, jobConfig)

	services := []string(getOption("services", config, jobConfig))
	for _, service := range services {
		build.Services = append(build.Services, common.JRImage{
			Name: service,
		})
	}

	artifacts := getOption("artifacts", config, jobConfig)
	untracked, err := strconv.ParseBool(artifacts["untracked"])
	if err != nil {
		untracked = false
	}
	build.Artifacts[0] = common.JRArtifact{
		Name:      artifacts["name"],
		Untracted: untracked,
		Paths:     artifacts["paths"],
		When:      artifacts["when"],
		ExpireIn:  artifacts["expireIn"],
	}

	cache := getOption("cache", config, jobConfig)
	untracked, err = strconv.ParseBool(cache["untracked"])
	if err != nil {
		untracked = false
	}
	build.Cache[0] = common.JRCache{
		Key:       cache["key"],
		Untracted: untracked,
		Paths:     cache["paths"],
	}

	if stage, ok := jobConfig.GetString("stage"); ok {
		build.JobInfo.Stage = stage
	} else {
		build.JobInfo.Stage = "test"
	}
	return nil
}

func getOption(optionKey string, config, jobConfig common.BuildOptions) interface{} {
	// parse job options
	for key, value := range jobConfig {
		if key == optionKey {
			return value
		}
	}

	// parse global options
	for key, value := range config {
		if key == optionKey {
			return value
		}
	}

	return
}

func (c *ExecCommand) createBuild(repoURL string, abortSignal chan os.Signal) (build *common.Build, err error) {
	// Check if we have uncommitted changes
	_, err = c.runCommand("git", "diff", "--quiet", "HEAD")
	if err != nil {
		logrus.Warningln("You most probably have uncommitted changes.")
		logrus.Warningln("These changes will not be tested.")
	}

	// Parse Git settings
	sha, err := c.runCommand("git", "rev-parse", "HEAD")
	if err != nil {
		return
	}

	beforeSha, err := c.runCommand("git", "rev-parse", "HEAD~1")
	if err != nil {
		beforeSha = "0000000000000000000000000000000000000000"
	}

	refName, err := c.runCommand("git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return
	}

	build = &common.Build{
		JobResponse: common.JobResponse{
			ID:            1,
			Token:         "",
			AllowGitFetch: false,
			JobInfo: common.JRJobInfo{
				Name:        "",
				Stage:       "",
				ProjectID:   1,
				ProjectName: "",
			},
			GitInfo: common.JRGitInfo{
				RepoURL:   repoURL,
				Ref:       strings.TrimSpace(refName),
				Sha:       strings.TrimSpace(sha),
				BeforeSha: strings.TrimSpace(beforeSha),
			},
			RunnerInfo: common.JRRunnerInfo{
				Timeout: c.getTimeout(),
			},
		},
		Runner: &common.RunnerConfig{
			RunnerSettings: c.RunnerSettings,
		},
		SystemInterrupt: abortSignal,
	}
	return
}

func (c *ExecCommand) getTimeout() int {
	if c.Timeout > 0 {
		return c.Timeout
	}

	return common.DefaultExecTimeout
}

func (c *ExecCommand) Execute(context *cli.Context) {
	wd, err := os.Getwd()
	if err != nil {
		logrus.Fatalln(err)
	}

	switch len(context.Args()) {
	case 1:
		c.Job = context.Args().Get(0)
	default:
		cli.ShowSubcommandHelp(context)
		os.Exit(1)
		return
	}

	c.Executor = context.Command.Name

	abortSignal := make(chan os.Signal)
	doneSignal := make(chan int, 1)

	go waitForInterrupts(nil, abortSignal, doneSignal)

	// Add self-volume to docker
	if c.RunnerSettings.Docker == nil {
		c.RunnerSettings.Docker = &common.DockerConfig{}
	}
	c.RunnerSettings.Docker.Volumes = append(c.RunnerSettings.Docker.Volumes, wd+":"+wd+":ro")

	// Create build
	build, err := c.createBuild(wd, abortSignal)
	if err != nil {
		logrus.Fatalln(err)
	}

	err = c.parseYaml(c.Job, &build.JobResponse)
	if err != nil {
		logrus.Fatalln(err)
	}

	err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	if err != nil {
		logrus.Fatalln(err)
	}
}

func init() {
	cmd := &ExecCommand{}

	flags := clihelpers.GetFlagsFromStruct(cmd)
	cliCmd := cli.Command{
		Name:  "exec",
		Usage: "execute a build locally",
	}

	for _, executor := range common.GetExecutors() {
		subCmd := cli.Command{
			Name:   executor,
			Usage:  "use " + executor + " executor",
			Action: cmd.Execute,
			Flags:  flags,
		}
		cliCmd.Subcommands = append(cliCmd.Subcommands, subCmd)
	}

	common.RegisterCommand(cliCmd)
}
