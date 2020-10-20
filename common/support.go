package common

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/tevino/abool"
)

const (
	repoRemoteURL = "https://gitlab.com/gitlab-org/ci-cd/tests/gitlab-test.git"

	repoRefType = RefTypeBranch

	repoSHA       = "91956efe32fb7bef54f378d90c9bd74c19025872"
	repoBeforeSHA = "ca50079dac5293292f83a4d454922ba8db44e7a3"
	repoRefName   = "master"

	repoLFSSHA       = "2371dd05e426fca09b0d2ec5d9ed757559035e2f"
	repoLFSBeforeSHA = "91956efe32fb7bef54f378d90c9bd74c19025872"
	repoLFSRefName   = "add-lfs-object"

	repoSubmoduleLFSSHA       = "d0cb7ff49b5c4fcf159e860fd6b30ef40731c435"
	repoSubmoduleLFSBeforeSHA = "dcbc4f0c93cb1731eeac4e3a70a55a991838e137"
	repoSubmoduleLFSRefName   = "add-lfs-submodule"

	FilesLFSFile1LFSsize = int64(2097152)
)

var (
	gitLabComChain        string
	gitLabComChainFetched *abool.AtomicBool
)

func init() {
	gitLabComChainFetched = abool.New()
}

func GetGitInfo(url string) GitInfo {
	return GitInfo{
		RepoURL:   url,
		Sha:       repoSHA,
		BeforeSha: repoBeforeSHA,
		Ref:       repoRefName,
		RefType:   repoRefType,
		Refspecs:  []string{"+refs/heads/*:refs/origin/heads/*", "+refs/tags/*:refs/tags/*"},
	}
}

func GetLFSGitInfo(url string) GitInfo {
	return GitInfo{
		RepoURL:   url,
		Sha:       repoLFSSHA,
		BeforeSha: repoLFSBeforeSHA,
		Ref:       repoLFSRefName,
		RefType:   repoRefType,
		Refspecs:  []string{"+refs/heads/*:refs/origin/heads/*", "+refs/tags/*:refs/tags/*"},
	}
}

func GetSubmoduleLFSGitInfo(url string) GitInfo {
	return GitInfo{
		RepoURL:   url,
		Sha:       repoSubmoduleLFSSHA,
		BeforeSha: repoSubmoduleLFSBeforeSHA,
		Ref:       repoSubmoduleLFSRefName,
		RefType:   repoRefType,
		Refspecs:  []string{"+refs/heads/*:refs/origin/heads/*", "+refs/tags/*:refs/tags/*"},
	}
}

func GetSuccessfulBuild() (JobResponse, error) {
	return GetLocalBuildResponse("echo Hello World")
}

func GetRemoteSuccessfulBuild() (JobResponse, error) {
	return GetRemoteBuildResponse("echo Hello World")
}

func GetRemoteSuccessfulLFSBuild() (JobResponse, error) {
	response, err := GetRemoteBuildResponse("echo Hello World")
	response.GitInfo = GetLFSGitInfo(repoRemoteURL)

	return response, err
}

func GetRemoteSuccessfulBuildWithAfterScript() (JobResponse, error) {
	jobResponse, err := GetRemoteBuildResponse("echo Hello World")
	jobResponse.Steps = append(
		jobResponse.Steps,
		Step{
			Name:   StepNameAfterScript,
			Script: []string{"echo Hello World"},
			When:   StepWhenAlways,
		},
	)
	return jobResponse, err
}

func GetRemoteSuccessfulMultistepBuild() (JobResponse, error) {
	jobResponse, err := GetRemoteBuildResponse("echo Hello World")
	if err != nil {
		return JobResponse{}, err
	}

	jobResponse.Steps = append(
		jobResponse.Steps,
		Step{
			Name:   "release",
			Script: []string{"echo Release"},
			When:   StepWhenOnSuccess,
		},
		Step{
			Name:   StepNameAfterScript,
			Script: []string{"echo After Script"},
			When:   StepWhenAlways,
		},
	)

	return jobResponse, nil
}

func GetRemoteFailingMultistepBuild(failingStepName StepName) (JobResponse, error) {
	jobResponse, err := GetRemoteSuccessfulMultistepBuild()
	if err != nil {
		return JobResponse{}, err
	}

	for i, step := range jobResponse.Steps {
		if step.Name == failingStepName {
			jobResponse.Steps[i].Script = append(step.Script, "exit 1")
		}
	}

	return jobResponse, nil
}

func GetRemoteFailingMultistepBuildWithEnvs(shell string, fail bool) (JobResponse, error) {
	jobResponse, err := GetRemoteBuildResponse("echo 'Hello World'")
	if err != nil {
		return JobResponse{}, err
	}

	var envCommand string
	switch shell {
	case "cmd":
		envCommand = "set"
	case "powershell", "pwsh":
		envCommand = `dir env: | %{"{0}={1}" -f $_.Name,$_.Value}`
	default:
		envCommand = "env"
	}

	exitCommand := "exit 0"
	if fail {
		exitCommand = "exit 1"
	}

	jobResponse.Steps = append(
		jobResponse.Steps,
		Step{
			Name:   "env",
			Script: []string{envCommand, exitCommand},
			When:   StepWhenOnSuccess,
		},
		Step{
			Name:   StepNameAfterScript,
			Script: []string{envCommand},
			When:   StepWhenAlways,
		},
	)

	return jobResponse, nil
}

func GetRemoteSuccessfulBuildWithDumpedVariables() (JobResponse, error) {
	variableName := "test_dump"
	variableValue := "test"

	response, err := GetRemoteBuildResponse(
		fmt.Sprintf("[[ \"${%s}\" != \"\" ]]", variableName),
		fmt.Sprintf("[[ $(cat $%s) == \"%s\" ]]", variableName, variableValue),
	)

	if err != nil {
		return JobResponse{}, err
	}

	dumpedVariable := JobVariable{
		Key: variableName, Value: variableValue,
		Internal: true, Public: true, File: true,
	}
	response.Variables = append(response.Variables, dumpedVariable)

	return response, nil
}

func GetFailedBuild() (JobResponse, error) {
	return GetLocalBuildResponse("exit 1")
}

func GetRemoteFailedBuild() (JobResponse, error) {
	return GetRemoteBuildResponse("exit 1")
}

func GetLongRunningBuild() (JobResponse, error) {
	return GetLocalBuildResponse("sleep 3600")
}

func GetRemoteLongRunningBuild() (JobResponse, error) {
	return GetRemoteBuildResponse("sleep 3600")
}

func GetRemoteLongRunningBuildCMD() (JobResponse, error) {
	// Can't use TIMEOUT since it requires input redirection,
	// https://knowledge.broadcom.com/external/article/29524/the-timeout-command-in-batch-script-job.html
	return GetLocalBuildResponse("ping 127.0.0.1 -n 3600 > nul")
}

func GetRemoteLongRunningBuildWithAfterScript() (JobResponse, error) {
	jobResponse, err := GetRemoteLongRunningBuild()
	if err != nil {
		return JobResponse{}, err
	}

	addAfterScript(&jobResponse)

	return jobResponse, nil
}

func GetRemoteLongRunningBuildWithAfterScriptCMD() (JobResponse, error) {
	jobResponse, err := GetRemoteLongRunningBuildCMD()
	if err != nil {
		return JobResponse{}, err
	}

	addAfterScript(&jobResponse)

	return jobResponse, nil
}

func addAfterScript(jobResponse *JobResponse) {
	jobResponse.Steps = append(
		jobResponse.Steps,
		Step{
			Name: StepNameAfterScript,
			Script: []string{
				"echo Hello World from after_script",
			},
			When: StepWhenAlways,
		},
	)
}

func GetMultilineBashBuild() (JobResponse, error) {
	return GetRemoteBuildResponse(`if true; then
	echo 'Hello World'
fi
`)
}

func GetMultilineBashBuildPowerShell() (JobResponse, error) {
	return GetRemoteBuildResponse("if (0 -eq 0) {\n\recho \"Hello World\"\n\r}")
}

func GetMultilineBashBuildCmd() (JobResponse, error) {
	return GetRemoteBuildResponse(`IF 0==0 (
  echo Hello World
)`)
}

func GetRemoteBrokenTLSBuild() (JobResponse, error) {
	invalidCert, err := buildSnakeOilCert()
	if err != nil {
		return JobResponse{}, err
	}

	return getRemoteCustomTLSBuild(invalidCert)
}

func GetRemoteGitLabComTLSBuild() (JobResponse, error) {
	cert, err := getGitLabComTLSChain()
	if err != nil {
		return JobResponse{}, err
	}

	return getRemoteCustomTLSBuild(cert)
}

func getRemoteCustomTLSBuild(chain string) (JobResponse, error) {
	job, err := GetRemoteBuildResponse("echo Hello World")
	if err != nil {
		return JobResponse{}, err
	}

	job.TLSCAChain = chain
	job.Variables = append(
		job.Variables,
		JobVariable{Key: "GIT_STRATEGY", Value: "clone"},
		JobVariable{Key: "GIT_SUBMODULE_STRATEGY", Value: "normal"},
	)

	return job, nil
}

func getBuildResponse(repoURL string, commands []string) JobResponse {
	return JobResponse{
		GitInfo: GetGitInfo(repoURL),
		Steps: Steps{
			Step{
				Name:         StepNameScript,
				Script:       commands,
				When:         StepWhenAlways,
				AllowFailure: false,
			},
		},
	}
}

func GetRemoteBuildResponse(commands ...string) (JobResponse, error) {
	return getBuildResponse(repoRemoteURL, commands), nil
}

func GetLocalBuildResponse(commands ...string) (JobResponse, error) {
	localRepoURL, err := getLocalRepoURL()
	if err != nil {
		if os.IsNotExist(err) {
			panic("Local repo not found, please run `make development_setup`")
		}
		return JobResponse{}, err
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
	if gitLabComChainFetched.IsSet() {
		return gitLabComChain, nil
	}

	resp, err := http.Head("https://gitlab.com/users/sign_in")
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	var buff bytes.Buffer
	for _, certs := range resp.TLS.VerifiedChains {
		for _, cert := range certs {
			err = pem.Encode(&buff, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
			if err != nil {
				return "", err
			}
		}
	}

	gitLabComChain = buff.String()
	gitLabComChainFetched.Set()

	return gitLabComChain, nil
}
