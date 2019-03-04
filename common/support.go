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

const repoRemoteURL = "https://gitlab.com/gitlab-org/gitlab-test.git"
const repoSHA = "6907208d755b60ebeacb2e9dfea74c92c3449a1f"
const repoBeforeSHA = "c347ca2e140aa667b968e51ed0ffe055501fe4f4"
const repoRefName = "master"
const repoRefType = RefTypeBranch

var (
	gitLabComChain        string
	gitLabComChainFetched *abool.AtomicBool
)

func init() {
	gitLabComChainFetched = abool.New()
}

func GetSuccessfulBuild() (JobResponse, error) {
	return GetLocalBuildResponse("echo Hello World")
}

func GetRemoteSuccessfulBuild() (JobResponse, error) {
	return GetRemoteBuildResponse("echo Hello World")
}

func GetRemoteSuccessfulBuildWithAfterScript() (JobResponse, error) {
	jobResponse, err := GetRemoteBuildResponse("echo Hello World")
	jobResponse.Steps = append(jobResponse.Steps,
		Step{
			Name:   StepNameAfterScript,
			Script: []string{"echo Hello World"},
			When:   StepWhenAlways,
		},
	)
	return jobResponse, err
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

func GetMultilineBashBuild() (JobResponse, error) {
	return GetRemoteBuildResponse(`if true; then
	bash \
		--login \
		-c 'echo Hello World'
fi
`)
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
	job.Variables = append(job.Variables,
		JobVariable{Key: "GIT_STRATEGY", Value: "clone"},
		JobVariable{Key: "GIT_SUBMODULE_STRATEGY", Value: "normal"})

	return job, nil
}

func getBuildResponse(repoURL string, commands []string) JobResponse {
	return JobResponse{
		GitInfo: GitInfo{
			RepoURL:   repoURL,
			Sha:       repoSHA,
			BeforeSha: repoBeforeSHA,
			Ref:       repoRefName,
			RefType:   repoRefType,
			Refspecs:  []string{"+refs/heads/*:refs/origin/heads/*", "+refs/tags/*:refs/tags/*"},
		},
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
		return JobResponse{}, err
	}

	return getBuildResponse(localRepoURL, commands), nil
}

func getLocalRepoURL() (string, error) {
	_, filename, _, _ := runtime.Caller(0)

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
