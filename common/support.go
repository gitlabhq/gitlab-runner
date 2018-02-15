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
	return getLocalBuildResponse("echo Hello World")
}

func GetRemoteSuccessfulBuild() (JobResponse, error) {
	return getRemoteBuildResponse("echo Hello World")
}

func GetRemoteSuccessfulBuildWithAfterScript() (JobResponse, error) {
	jobResponse, err := getRemoteBuildResponse("echo Hello World")
	jobResponse.Steps = append(jobResponse.Steps,
		Step{
			Name:   StepNameAfterScript,
			Script: []string{"echo Hello World"},
			When:   StepWhenAlways,
		},
	)
	return jobResponse, err
}

func GetRemoteSuccessfulBuildWithDumpedVariables() (response JobResponse, err error) {
	variableName := "test_dump"
	variableValue := "test"

	response, err = getRemoteBuildResponse(
		fmt.Sprintf("[[ \"${%s}\" != \"\" ]]", variableName),
		fmt.Sprintf("[[ $(cat $%s) == \"%s\" ]]", variableName, variableValue),
	)

	if err != nil {
		return
	}

	dumpedVariable := JobVariable{
		Key: variableName, Value: variableValue,
		Internal: true, Public: true, File: true,
	}
	response.Variables = append(response.Variables, dumpedVariable)

	return
}

func GetFailedBuild() (JobResponse, error) {
	return getLocalBuildResponse("exit 1")
}

func GetRemoteFailedBuild() (JobResponse, error) {
	return getRemoteBuildResponse("exit 1")
}

func GetLongRunningBuild() (JobResponse, error) {
	return getLocalBuildResponse("sleep 3600")
}

func GetRemoteLongRunningBuild() (JobResponse, error) {
	return getRemoteBuildResponse("sleep 3600")
}

func GetMultilineBashBuild() (JobResponse, error) {
	return getRemoteBuildResponse(`if true; then
	bash \
		--login \
		-c 'echo Hello World'
fi
`)
}

func GetRemoteBrokenTLSBuild() (job JobResponse, err error) {
	invalidCert, err := buildSnakeOilCert()
	if err != nil {
		return
	}

	return getRemoteCustomTLSBuild(invalidCert)
}

func GetRemoteGitLabComTLSBuild() (job JobResponse, err error) {
	cert, err := getGitLabComTLSChain()
	if err != nil {
		return
	}

	return getRemoteCustomTLSBuild(cert)
}

func getRemoteCustomTLSBuild(chain string) (job JobResponse, err error) {
	job, err = getRemoteBuildResponse("echo Hello World")
	if err != nil {
		return
	}

	job.TLSCAChain = chain
	job.Variables = append(job.Variables,
		JobVariable{Key: "GIT_STRATEGY", Value: "clone"},
		JobVariable{Key: "GIT_SUBMODULE_STRATEGY", Value: "normal"})

	return
}

func getRemoteBuildResponse(commands ...string) (response JobResponse, err error) {
	response = JobResponse{
		GitInfo: GitInfo{
			RepoURL:   repoRemoteURL,
			Sha:       repoSHA,
			BeforeSha: repoBeforeSHA,
			Ref:       repoRefName,
			RefType:   repoRefType,
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

	return
}

func getLocalBuildResponse(commands ...string) (response JobResponse, err error) {
	localRepoURL, err := getLocalRepoURL()
	if err != nil {
		return
	}

	response = JobResponse{
		GitInfo: GitInfo{
			RepoURL:   localRepoURL,
			Sha:       repoSHA,
			BeforeSha: repoBeforeSHA,
			Ref:       repoRefName,
			RefType:   repoRefType,
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

	return
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
