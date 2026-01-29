//nolint:goconst
package common

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/stretchr/testify/assert/yaml"

	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/step-runner/schema/v1"
)

const (
	repoRemoteURL = "https://gitlab.com/gitlab-org/ci-cd/gitlab-runner-pipeline-tests/gitlab-test.git"

	repoRefType = spec.RefTypeBranch

	repoSHA       = "69b18e5ed3610cf646119c3e38f462c64ec462b7"
	repoBeforeSHA = "1ea27a9695f80d7816d9e8ce025d9b2df83d0dd7"
	repoRefName   = "main"

	repoLFSSHA       = "c8f2a61def956871b91f73fcd0c320afb257fd6e"
	repoLFSBeforeSHA = "86002a2304d89a193f91b8b0907c4cf2f95a6d28"
	repoLFSRefName   = "add-lfs-object"

	repoSubmoduleLFSSHA       = "86002a2304d89a193f91b8b0907c4cf2f95a6d28"
	repoSubmoduleLFSBeforeSHA = "1ea27a9695f80d7816d9e8ce025d9b2df83d0dd7"
	repoSubmoduleLFSRefName   = "add-lfs-submodule"

	repoStepsSHA       = "1142c6530a1eb81f0a5476db25fbfbf9a4e08f30"
	repoStepsBeforeSHA = "1ea27a9695f80d7816d9e8ce025d9b2df83d0dd7"
	repoStepsRefName   = "add-steps"

	FilesLFSFile1LFSsize = int64(2097152)
)

var (
	gitLabComChain        string
	gitLabComChainFetched atomic.Bool
)

func GetGitInfo(url string) spec.GitInfo {
	return spec.GitInfo{
		RepoURL:   url,
		Sha:       repoSHA,
		BeforeSha: repoBeforeSHA,
		Ref:       repoRefName,
		RefType:   repoRefType,
		Refspecs:  []string{"+refs/heads/*:refs/origin/heads/*", "+refs/tags/*:refs/tags/*"},
	}
}

func GetLFSGitInfo(url string) spec.GitInfo {
	return spec.GitInfo{
		RepoURL:   url,
		Sha:       repoLFSSHA,
		BeforeSha: repoLFSBeforeSHA,
		Ref:       repoLFSRefName,
		RefType:   repoRefType,
		Refspecs:  []string{"+refs/heads/*:refs/origin/heads/*", "+refs/tags/*:refs/tags/*"},
	}
}

func GetSubmoduleLFSGitInfo(url string) spec.GitInfo {
	return spec.GitInfo{
		RepoURL:   url,
		Sha:       repoSubmoduleLFSSHA,
		BeforeSha: repoSubmoduleLFSBeforeSHA,
		Ref:       repoSubmoduleLFSRefName,
		RefType:   repoRefType,
		Refspecs:  []string{"+refs/heads/*:refs/origin/heads/*", "+refs/tags/*:refs/tags/*"},
	}
}

func GetStepsGitInfo(url string) spec.GitInfo {
	return spec.GitInfo{
		RepoURL:   url,
		Sha:       repoStepsSHA,
		BeforeSha: repoStepsBeforeSHA,
		Ref:       repoStepsRefName,
		RefType:   repoRefType,
		Refspecs:  []string{"+refs/heads/*:refs/origin/heads/*", "+refs/tags/*:refs/tags/*"},
	}
}

func GetSuccessfulBuild() (spec.Job, error) {
	return GetLocalBuildResponse("echo Hello World")
}

func GetSuccessfulMultilineCommandBuild() (spec.Job, error) {
	return GetLocalBuildResponse(`echo "Hello
World"`)
}

func GetRemoteSuccessfulBuild() (spec.Job, error) {
	return GetRemoteBuildResponse("echo Hello World")
}

func GetRemoteSuccessfulLFSBuild() (spec.Job, error) {
	response, err := GetRemoteBuildResponse("echo Hello World")
	response.GitInfo = GetLFSGitInfo(repoRemoteURL)

	return response, err
}

func GetRemoteSuccessfulBuildWithAfterScript() (spec.Job, error) {
	jobResponse, err := GetRemoteBuildResponse("echo Hello World")
	jobResponse.Steps = append(
		jobResponse.Steps,
		spec.Step{
			Name:   spec.StepNameAfterScript,
			Script: []string{"echo Hello World"},
			When:   spec.StepWhenAlways,
		},
	)
	return jobResponse, err
}

func GetRemoteSuccessfulBuildPrintVars(shell string, vars ...string) (spec.Job, error) {
	printVarsCmd := getShellPrintVars(shell, vars...)

	return GetRemoteBuildResponse(printVarsCmd...)
}

func GetRemoteSuccessfulBuildPrintVarsAfterScript(shell string, vars ...string) (spec.Job, error) {
	printVarsCmd := getShellPrintVars(shell, vars...)

	return GetRemoteBuildResponse(printVarsCmd...)
}

func GetRemoteSuccessfulMultistepBuild() (spec.Job, error) {
	jobResponse, err := GetRemoteBuildResponse("echo Hello World")
	if err != nil {
		return spec.Job{}, err
	}

	jobResponse.Steps = append(
		jobResponse.Steps,
		spec.Step{
			Name:   "release",
			Script: []string{"echo Release"},
			When:   spec.StepWhenOnSuccess,
		},
		spec.Step{
			Name:   spec.StepNameAfterScript,
			Script: []string{"echo After Script"},
			When:   spec.StepWhenAlways,
		},
	)

	return jobResponse, nil
}

func GetRemoteFailingMultistepBuild(failingStepName spec.StepName) (spec.Job, error) {
	jobResponse, err := GetRemoteSuccessfulMultistepBuild()
	if err != nil {
		return spec.Job{}, err
	}

	for i, step := range jobResponse.Steps {
		if step.Name == failingStepName {
			jobResponse.Steps[i].Script = append(step.Script, "exit 1") //nolint:gocritic
		}
	}

	return jobResponse, nil
}

func GetRemoteFailingMultistepBuildPrintVars(shell string, fail bool, vars ...string) (spec.Job, error) {
	jobResponse, err := GetRemoteBuildResponse("echo 'Hello World'")
	if err != nil {
		return spec.Job{}, err
	}

	printVarsCmd := getShellPrintVars(shell, vars...)

	exitCommand := "exit 0"
	if fail {
		exitCommand = "exit 1"
	}

	jobResponse.Steps = append(
		jobResponse.Steps,
		spec.Step{
			Name:   "env",
			Script: append(printVarsCmd, exitCommand),
			When:   spec.StepWhenOnSuccess,
		},
		spec.Step{
			Name:   spec.StepNameAfterScript,
			Script: printVarsCmd,
			When:   spec.StepWhenAlways,
		},
	)

	return jobResponse, nil
}

func getShellPrintVars(shell string, vars ...string) []string {
	var envCommand []string
	var fmtStr string

	switch shell {
	case "powershell", "pwsh":
		fmtStr = "echo %s=$env:%s"
	default:
		fmtStr = "echo %s=$%s"
	}

	for _, v := range vars {
		envCommand = append(envCommand, fmt.Sprintf(fmtStr, v, v))
	}

	return envCommand
}

func GetRemoteSuccessfulBuildWithDumpedVariables() (spec.Job, error) {
	variableName := "test_dump"
	variableValue := "test"

	response, err := GetRemoteBuildResponse(
		fmt.Sprintf("[[ \"${%s}\" != \"\" ]]", variableName),
		fmt.Sprintf("[[ $(cat $%s) == \"%s\" ]]", variableName, variableValue),
	)
	if err != nil {
		return spec.Job{}, err
	}

	dumpedVariable := spec.Variable{
		Key: variableName, Value: variableValue,
		Internal: true, Public: true, File: true,
	}
	response.Variables = append(response.Variables, dumpedVariable)

	return response, nil
}

func GetFailedBuild() (spec.Job, error) {
	return GetLocalBuildResponse("exit 1")
}

func GetRemoteFailedBuild() (spec.Job, error) {
	return GetRemoteBuildResponse("exit 1")
}

func GetLongRunningBuild() (spec.Job, error) {
	return GetLocalBuildResponse("sleep 3600")
}

func GetRemoteLongRunningBuild() (spec.Job, error) {
	return GetRemoteBuildResponse("sleep 3600")
}

func GetRemoteLongRunningBuildWithAfterScript(shell string) (spec.Job, error) {
	var jobResponse spec.Job
	var err error

	jobResponse, err = GetRemoteLongRunningBuild()
	if err != nil {
		return spec.Job{}, err
	}

	switch shell {
	default:
		jobResponse.Steps = append(jobResponse.Steps, spec.Step{
			Name: spec.StepNameAfterScript,
			Script: []string{
				"echo \"Hello World from after_script\"",
				"echo \"job status $CI_JOB_STATUS\"",
			},
		})

	case "pwsh":
		jobResponse.Steps = append(jobResponse.Steps, spec.Step{
			Name: spec.StepNameAfterScript,
			Script: []string{
				"echo \"Hello World from after_script\"",
				"echo \"job status $env:CI_JOB_STATUS\"",
			},
		})

	case "cmd":
		jobResponse.Steps = append(jobResponse.Steps, spec.Step{
			Name: spec.StepNameAfterScript,
			Script: []string{
				"echo \"Hello World from after_script\"",
				"echo \"job status %CI_JOB_STATUS%\"",
			},
		})
	}

	return jobResponse, nil
}

func GetMultilineBashBuild() (spec.Job, error) {
	return GetRemoteBuildResponse(`if true; then
	echo 'Hello World'
fi
`)
}

func GetMultilineBashBuildPowerShell() (spec.Job, error) {
	return GetRemoteBuildResponse("if (0 -eq 0) {\n\recho \"Hello World\"\n\r}")
}

func GetRemoteBrokenTLSBuild() (spec.Job, error) {
	invalidCert, err := buildSnakeOilCert()
	if err != nil {
		return spec.Job{}, err
	}

	return getRemoteCustomTLSBuild(invalidCert)
}

func GetRemoteGitLabComTLSBuild() (spec.Job, error) {
	cert, err := getGitLabComTLSChain()
	if err != nil {
		return spec.Job{}, err
	}

	return getRemoteCustomTLSBuild(cert)
}

func getRemoteCustomTLSBuild(chain string) (spec.Job, error) {
	job, err := GetRemoteBuildResponse("echo Hello World")
	if err != nil {
		return spec.Job{}, err
	}

	job.TLSData.CAChain = chain
	job.Variables = append(
		job.Variables,
		spec.Variable{Key: "GIT_STRATEGY", Value: "clone"},
		spec.Variable{Key: "GIT_SUBMODULE_STRATEGY", Value: "normal"},
	)

	return job, nil
}

func getBuildResponse(repoURL string, commands []string) spec.Job {
	return spec.Job{
		Variables: spec.Variables{
			spec.Variable{Key: "CI_JOB_TOKEN", Value: "test-job-token"},
		},
		GitInfo: GetGitInfo(repoURL),
		Steps: spec.Steps{
			spec.Step{
				Name:         spec.StepNameScript,
				Script:       commands,
				When:         spec.StepWhenAlways,
				AllowFailure: false,
			},
		},
		RunnerInfo: spec.RunnerInfo{
			Timeout: DefaultTimeout,
		},
	}
}

func getStepsBuildResponse(repoURL, stepsYAML string) (spec.Job, error) {
	var steps []schema.Step
	if err := yaml.Unmarshal([]byte(stepsYAML), &steps); err != nil {
		return spec.Job{}, err
	}

	return spec.Job{
		GitInfo: GetStepsGitInfo(repoURL),
		Run:     steps,
		Steps:   spec.Steps{spec.Step{Name: spec.StepNameRun}},
		RunnerInfo: spec.RunnerInfo{
			Timeout: DefaultTimeout,
		},
	}, nil
}

func GetRemoteStepsBuildResponse(stepsYAML string) (spec.Job, error) {
	return getStepsBuildResponse(repoRemoteURL, stepsYAML)
}

func GetRemoteBuildResponse(commands ...string) (spec.Job, error) {
	return getBuildResponse(repoRemoteURL, commands), nil
}

func GetLocalBuildResponse(commands ...string) (spec.Job, error) {
	localRepoURL, err := getLocalRepoURL()
	if err != nil {
		if os.IsNotExist(err) {
			panic("Local repo not found, please run `make development_setup`")
		}
		return spec.Job{}, err
	}

	return getBuildResponse(localRepoURL, commands), nil
}

func getLocalRepoURL() (string, error) {
	_, filename, _, _ := runtime.Caller(0) //nolint:dogsled

	directory := path.Dir(filename)
	if strings.Contains(directory, "_test/_obj_test") {
		pwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		directory = pwd
	}

	localRepoURL := path.Clean(directory + "/../tmp/gitlab-test/.git")

	_, err := os.Stat(localRepoURL)
	if err != nil {
		return "", err
	}

	return localRepoURL, nil
}

func RunLocalRepoGitCommand(arguments ...string) error {
	url, err := getLocalRepoURL()
	if err != nil {
		return err
	}

	cmd := exec.Command("git", arguments...)
	cmd.Dir = path.Dir(url)

	return cmd.Run()
}

func buildSnakeOilCert() (string, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return "", err
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(time.Hour)

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Snake Oil Co"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		IsCA:                  true,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return "", err
	}

	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	return string(certificate), nil
}

func getGitLabComTLSChain() (string, error) {
	if gitLabComChainFetched.Load() {
		return gitLabComChain, nil
	}

	resp, err := http.Head("https://gitlab.com/users/sign_in")
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	var buff strings.Builder
	for _, certs := range resp.TLS.VerifiedChains {
		for _, cert := range certs {
			err = pem.Encode(&buff, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
			if err != nil {
				return "", err
			}
		}
	}

	gitLabComChain = buff.String()
	gitLabComChainFetched.Store(true)

	return gitLabComChain, nil
}
