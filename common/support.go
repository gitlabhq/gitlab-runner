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
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync"
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
	// These fixtures exercise the TLS chain to gitlab.com, so they take no
	// test git server redirect.
	job := getBuildResponse(repoRemoteURL, []string{"echo Hello World"})

	job.TLSData.CAChain = chain
	job.Variables = append(
		job.Variables,
		spec.Variable{Key: "GIT_STRATEGY", Value: "clone"},
		spec.Variable{Key: "GIT_SUBMODULE_STRATEGY", Value: "normal"},
	)

	return job, nil
}

// Remote fixtures clone from gitlab.com. When RUNNER_TEST_GIT_SERVER_URL
// names a git-server (base-images, dockerfiles/tests) they also carry git
// insteadOf rules that send the mirrored repositories there: the clone,
// the submodules and LFS. The job otherwise still looks like a gitlab.com
// job, so URL-derived values such as the job URL are unchanged.
const TestGitServerEnv = "RUNNER_TEST_GIT_SERVER_URL"

// testGitServerRepos are the repositories the git-server image mirrors.
// Only these are redirected; tests that use other gitlab.com repositories,
// such as the submodule fixtures and the private project they reference,
// keep reaching gitlab.com with the runner's own credential handling.
var testGitServerRepos = []string{
	"gitlab-org/ci-cd/gitlab-runner-pipeline-tests/gitlab-test",
	"gitlab-org/ci-cd/gitlab-runner-pipeline-tests/gitlab-grack",
	"gitlab-org/ci-cd/gitlab-runner-pipeline-tests/simplegit",
	"gitlab-org/ci-cd/tests/lfs",
	"gitlab-org/ci-cd/tests/file-permissions",
}

var (
	testGitServerOnce sync.Once
	testGitServerBase string
	errTestGitServer  error
)

// testGitServerBaseURL returns the configured server URL with a trailing
// slash, or "" when none is configured. The host is pinned to an address
// once: jobs may run in containers that cannot resolve the CI service alias.
// A configured server that cannot be resolved is an error for every fixture,
// not just the first, so the redirect never drops out silently.
func testGitServerBaseURL() (string, error) {
	testGitServerOnce.Do(func() {
		raw := os.Getenv(TestGitServerEnv)
		if raw == "" {
			return
		}

		testGitServerBase, errTestGitServer = resolveTestGitServer(raw, net.LookupIP)
	})
	if errTestGitServer != nil {
		return "", fmt.Errorf("%s: %w", TestGitServerEnv, errTestGitServer)
	}

	return testGitServerBase, nil
}

// resolveTestGitServer normalises the server URL to "scheme://addr[:port]/".
// A name is replaced by its first IPv4 address: the job containers have IPv4
// networking, so a name with only IPv6 addresses is an error rather than a
// redirect to nowhere. Address literals are kept as written.
func resolveTestGitServer(raw string, lookupIP func(string) ([]net.IP, error)) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.Scheme == "" {
		return "", fmt.Errorf("%q is not a URL", raw)
	}

	if host := u.Hostname(); net.ParseIP(host) == nil {
		ips, err := lookupIP(host)
		if err != nil {
			return "", fmt.Errorf("resolving %q: %w", host, err)
		}

		addr := ""
		for _, ip := range ips {
			if ip4 := ip.To4(); ip4 != nil {
				addr = ip4.String()
				break
			}
		}
		if addr == "" {
			return "", fmt.Errorf("resolving %q: no IPv4 address", host)
		}
		if port := u.Port(); port != "" {
			addr = net.JoinHostPort(addr, port)
		}
		u.Host = addr
	}
	u.Path, u.RawQuery, u.Fragment = strings.TrimSuffix(u.Path, "/")+"/", "", ""

	return u.String(), nil
}

// gitServerVariablesFn supplies the redirect variables for a remote fixture.
// The fixtures take it as a parameter so the tests can drive them without
// the process-wide environment.
type gitServerVariablesFn func() (spec.Variables, error)

// testGitServerVariables returns the job variables that redirect git to the
// configured test git server, or nil when none is configured.
func testGitServerVariables() (spec.Variables, error) {
	base, err := testGitServerBaseURL()
	if err != nil || base == "" {
		return nil, err
	}

	return testGitServerRules(base), nil
}

// testGitServerRules builds the redirect for a server base URL of the form
// "scheme://addr[:port]/". GIT_CONFIG_* is read by every git invocation in
// the job, which is what makes the redirect reach submodules and LFS as well
// as the clone. One rule per mirrored repository, each longer than the
// runner's own credential rule for https://gitlab.com/, so git prefers it
// for exactly those URLs.
func testGitServerRules(base string) spec.Variables {
	vars := spec.Variables{
		spec.Variable{Key: "GIT_CONFIG_COUNT", Value: strconv.Itoa(len(testGitServerRepos))},
	}
	for i, repo := range testGitServerRepos {
		vars = append(vars,
			spec.Variable{Key: fmt.Sprintf("GIT_CONFIG_KEY_%d", i), Value: "url." + base + repo + ".git.insteadOf"},
			spec.Variable{Key: fmt.Sprintf("GIT_CONFIG_VALUE_%d", i), Value: "https://gitlab.com/" + repo + ".git"},
		)
	}

	return vars
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
	return remoteStepsBuildResponse(testGitServerVariables, stepsYAML)
}

func remoteStepsBuildResponse(gitServer gitServerVariablesFn, stepsYAML string) (spec.Job, error) {
	job, err := getStepsBuildResponse(repoRemoteURL, stepsYAML)
	if err != nil {
		return spec.Job{}, err
	}

	return withTestGitServer(gitServer, job)
}

func GetRemoteBuildResponse(commands ...string) (spec.Job, error) {
	return remoteBuildResponse(testGitServerVariables, commands)
}

func remoteBuildResponse(gitServer gitServerVariablesFn, commands []string) (spec.Job, error) {
	return withTestGitServer(gitServer, getBuildResponse(repoRemoteURL, commands))
}

// withTestGitServer adds the redirect to a job that clones from gitlab.com.
func withTestGitServer(gitServer gitServerVariablesFn, job spec.Job) (spec.Job, error) {
	vars, err := gitServer()
	if err != nil {
		return spec.Job{}, err
	}
	job.Variables = append(job.Variables, vars...)

	return job, nil
}

// GetLocalBuildResponse returns the gitlab-test job for tests that should
// not depend on gitlab.com: the git-server serves it when one is configured,
// otherwise the clone `make development_setup` makes under tmp/gitlab-test.
// With neither it is the remote job, so tests that never run the job, such
// as unit tests, need no setup.
func GetLocalBuildResponse(commands ...string) (spec.Job, error) {
	return localBuildResponse(testGitServerVariables, getLocalRepoURL, commands)
}

func localBuildResponse(
	gitServer gitServerVariablesFn,
	localRepo func() (string, error),
	commands []string,
) (spec.Job, error) {
	vars, err := gitServer()
	if err != nil {
		return spec.Job{}, err
	}
	if len(vars) > 0 {
		job := getBuildResponse(repoRemoteURL, commands)
		job.Variables = append(job.Variables, vars...)

		return job, nil
	}

	localRepoURL, err := localRepo()
	switch {
	case err == nil:
		return getBuildResponse(localRepoURL, commands), nil
	case os.IsNotExist(err):
		return getBuildResponse(repoRemoteURL, commands), nil
	default:
		return spec.Job{}, err
	}
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
