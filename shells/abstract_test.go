package shells

import (
	"fmt"
	"testing"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/tls"
	"gitlab.com/gitlab-org/gitlab-runner/shells/mocks"
)

func TestWriteGitSSLConfig(t *testing.T) {
	gitlabURL := "https://example.com:3443"
	runnerURL := gitlabURL + "/ci/"

	shell := AbstractShell{}
	build := &common.Build{
		Runner: &common.RunnerConfig{
			RunnerCredentials: common.RunnerCredentials{
				URL: runnerURL,
			},
		},
		JobResponse: common.JobResponse{
			TLSAuthCert: "TLS_CERT",
			TLSAuthKey:  "TLS_KEY",
			TLSCAChain:  "CA_CHAIN",
		},
	}

	mockWriter := new(mocks.ShellWriter)
	mockWriter.On("TmpFile", tls.VariableCAFile).Return(tls.VariableCAFile).Once()
	mockWriter.On("TmpFile", tls.VariableCertFile).Return(tls.VariableCertFile).Once()
	mockWriter.On("TmpFile", tls.VariableKeyFile).Return(tls.VariableKeyFile).Once()

	mockWriter.On("Command", "git", "config", fmt.Sprintf("http.%s.%s", gitlabURL, "sslCAInfo"), tls.VariableCAFile).Once()
	mockWriter.On("Command", "git", "config", fmt.Sprintf("http.%s.%s", gitlabURL, "sslCert"), tls.VariableCertFile).Once()
	mockWriter.On("Command", "git", "config", fmt.Sprintf("http.%s.%s", gitlabURL, "sslKey"), tls.VariableKeyFile).Once()

	shell.writeGitSSLConfig(mockWriter, build, nil)

	mockWriter.AssertExpectations(t)
}
