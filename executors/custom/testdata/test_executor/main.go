package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"gitlab.com/gitlab-org/gitlab-runner/executors/custom/api"
)

const (
	isBuildError     = "CUSTOM_ENV_IS_BUILD_ERROR"
	isSystemError    = "CUSTOM_ENV_IS_SYSTEM_ERROR"
	isUnknownError   = "CUSTOM_ENV_IS_UNKNOWN_ERROR"
	isRunOnCustomDir = "CUSTOM_ENV_IS_RUN_ON_CUSTOM_DIR"
)

const (
	stageConfig  = "config"
	stagePrepare = "prepare"
	stageRun     = "run"
	stageCleanup = "cleanup"
)

var knownBuildStages = map[string]struct{}{
	"prepare_script":              {},
	"get_sources":                 {},
	"restore_cache":               {},
	"download_artifacts":          {},
	"build_script":                {},
	"after_script":                {},
	"archive_cache":               {},
	"archive_cache_on_failure":    {},
	"upload_artifacts_on_success": {},
	"upload_artifacts_on_failure": {},
	"cleanup_file_variables":      {},
}

func setBuildFailure(msg string, args ...interface{}) {
	fmt.Println("setting build failure")
	setFailure(api.BuildFailureExitCodeVariable, msg, args...)
}

func setSystemFailure(msg string, args ...interface{}) {
	fmt.Println("setting system failure")
	setFailure(api.SystemFailureExitCodeVariable, msg, args...)
}

func setFailure(failureType string, msg string, args ...interface{}) {
	fmt.Println()
	fmt.Printf(msg, args...)
	fmt.Println()

	exitCode := os.Getenv(failureType)

	code, err := strconv.Atoi(exitCode)
	if err != nil {
		panic(fmt.Sprintf("Error while parsing the variable: %v", err))
	}

	fmt.Printf("Exitting with code %d\n", code)

	os.Exit(code)
}

func printJobResponseDetails() {
	type fakeJobInfo struct {
		Name        string `json:"name"`
		Stage       string `json:"stage"`
		ProjectID   int    `json:"project_id"`
		ProjectName string `json:"project_name"`
	}

	type fakeJobResponse struct {
		ID      int         `json:"id"`
		JobInfo fakeJobInfo `json:"job_info"`
	}

	jobResponseFile := os.Getenv(api.JobResponseFileVariable)

	file, err := os.Open(jobResponseFile)
	if err != nil {
		panic(fmt.Sprintf("Error while opening job response file %q: %v", jobResponseFile, err))
	}

	defer func() { _ = file.Close() }()

	var jobResponse fakeJobResponse

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&jobResponse)
	if err != nil {
		panic(fmt.Sprintf("Error while decoding job response file %q: %v", jobResponseFile, err))
	}

	fmt.Println("Reading job response data:")
	fmt.Printf("job ID           => %d\n", jobResponse.ID)
	fmt.Printf("job name         => %s\n", jobResponse.JobInfo.Name)
	fmt.Printf("job stage        => %s\n", jobResponse.JobInfo.Stage)
	fmt.Printf("job project ID   => %d\n", jobResponse.JobInfo.ProjectID)
	fmt.Printf("job project name => %s\n", jobResponse.JobInfo.ProjectName)
	fmt.Println()
}

type stageFunc func(shell string, args []string)

func main() {
	defer func() {
		r := recover()
		if r == nil {
			return
		}

		setSystemFailure("Executor panicked with: %v", r)
	}()

	shell := os.Args[1]
	stage := os.Args[2]

	var args []string
	if len(os.Args) > 3 {
		args = os.Args[3:]
	}

	stages := map[string]stageFunc{
		stageConfig:  config,
		stagePrepare: prepare,
		stageRun:     run,
		stageCleanup: cleanup,
	}

	stageFn, ok := stages[stage]
	if !ok {
		setSystemFailure("Unknown stage %q", stage)
	}

	_, _ = fmt.Fprintf(os.Stderr, "Custom Executor binary - %q stage\n", stage)
	_, _ = fmt.Fprintf(os.Stderr, "Mocking execution of: %v\n", args)
	_, _ = fmt.Fprintln(os.Stderr)

	stageFn(shell, args)
}

func config(shell string, args []string) {
	customDir := os.Getenv(isRunOnCustomDir)
	if customDir == "" {
		return
	}

	concurrentID := os.Getenv("CUSTOM_ENV_CI_CONCURRENT_PROJECT_ID")
	projectSlug := os.Getenv("CUSTOM_ENV_CI_PROJECT_PATH_SLUG")

	dir := filepath.Join(customDir, concurrentID, projectSlug)

	type output struct {
		BuildsDir string `json:"builds_dir"`
	}

	jsonOutput, err := json.Marshal(output{BuildsDir: dir})
	if err != nil {
		panic(fmt.Errorf("error while creating JSON output: %w", err))
	}

	fmt.Print(string(jsonOutput))
}

func prepare(shell string, args []string) {
	fmt.Println("PREPARE doesn't accept any arguments. It just does its job")
	fmt.Println()
	printJobResponseDetails()
}

func run(shell string, args []string) {
	fmt.Println("RUN accepts two arguments: the path to the script to execute and the stage of the job")
	fmt.Println()

	mockError()

	if len(args) < 1 {
		setSystemFailure("Missing script for the run stage")
	}

	output := bytes.NewBuffer(nil)

	cmd := createCommand(shell, args[0], args[1])
	cmd.Stdout = output
	cmd.Stderr = output

	fmt.Printf("Executing: %#v\n\n", cmd)

	err := cmd.Run()
	if err != nil {
		setBuildFailure("Job script exited with: %v", err)
	}

	fmt.Printf(">>>>>>>>>>\n%s\n<<<<<<<<<<\n\n", output.String())
}

func mockError() {
	if len(os.Getenv(isBuildError)) > 0 {
		// It's a build error. For example: user used an invalid
		// command in his script which ends with an error thrown
		// from the underlying shell.

		setBuildFailure("mocked build failure")
	}

	if len(os.Getenv(isSystemError)) > 0 {
		// It's a system error. For example: the Custom Executor
		// script implements a libvirt executor and before executing
		// the job it needs to prepare the VM. But the preparation
		// failed.

		setSystemFailure("mocked system failure")
	}

	if len(os.Getenv(isUnknownError)) > 0 {
		// This situation should not happen. Custom Executor script
		// should define the type of failure and return either "build
		// failure" or "system failure", using the error code values
		// provided by dedicated variables.

		fmt.Println("mocked system failure")
		os.Exit(255)
	}
}

func createCommand(shell string, script string, stage string) *exec.Cmd {
	if _, ok := knownBuildStages[stage]; !ok {
		setSystemFailure("Unknown build stage %q", stage)
	}

	shellConfigs := map[string]struct {
		command string
		args    []string
	}{
		"bash": {
			command: "bash",
			args:    []string{},
		},
		"powershell": {
			command: "powershell",
			args:    []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command"},
		},
		"pwsh": {
			command: "pwsh",
			args:    []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command"},
		},
		"cmd": {
			command: "cmd",
			args:    []string{"/C"},
		},
	}

	shellConfig, ok := shellConfigs[shell]
	if !ok {
		panic(fmt.Sprintf("Unknown shell %q", shell))
	}

	args := append(shellConfig.args, script)

	return exec.Command(shellConfig.command, args...)
}

func cleanup(shell string, args []string) {
	fmt.Println("CLEANUP doesn't accept any arguments. It just does its job")
	fmt.Println()
}
