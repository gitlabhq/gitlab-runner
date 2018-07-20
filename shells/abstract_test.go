package shells

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/stretchr/testify/require"
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

func TestWriteWritingArtifactsOnSuccess(t *testing.T) {
	gitlabURL := "https://example.com:3443"

	shell := AbstractShell{}
	build := &common.Build{
		JobResponse: common.JobResponse{
			ID:    1000,
			Token: "token",
			Artifacts: common.Artifacts{
				common.Artifact{
					Paths: []string{"default"},
				},
				common.Artifact{
					Paths: []string{"on-success"},
					When:  common.ArtifactWhenOnSuccess,
				},
				common.Artifact{
					Paths: []string{"on-failure"},
					When:  common.ArtifactWhenOnFailure,
				},
				common.Artifact{
					Paths: []string{"always"},
					When:  common.ArtifactWhenAlways,
				},
				common.Artifact{
					Paths:  []string{"zip-archive"},
					When:   common.ArtifactWhenAlways,
					Format: common.ArtifactFormatZip,
					Type:   "archive",
				},
				common.Artifact{
					Paths:  []string{"gzip-junit"},
					When:   common.ArtifactWhenAlways,
					Format: common.ArtifactFormatGzip,
					Type:   "junit",
				},
			},
		},
		Runner: &common.RunnerConfig{
			RunnerCredentials: common.RunnerCredentials{
				URL: gitlabURL,
			},
		},
	}
	info := common.ShellScriptInfo{
		RunnerCommand: "gitlab-runner-helper",
		Build:         build,
	}

	mockWriter := new(mocks.ShellWriter)
	defer mockWriter.AssertExpectations(t)
	mockWriter.On("Variable", mock.Anything)
	mockWriter.On("Cd", mock.Anything)
	mockWriter.On("IfCmd", "gitlab-runner-helper", "--version")
	mockWriter.On("Notice", mock.Anything)
	mockWriter.On("Command", "gitlab-runner-helper", "artifacts-uploader",
		"--url", gitlabURL,
		"--token", "token",
		"--id", "1000",
		"--path", "default").Once()
	mockWriter.On("Command", "gitlab-runner-helper", "artifacts-uploader",
		"--url", gitlabURL,
		"--token", "token",
		"--id", "1000",
		"--path", "on-success").Once()
	mockWriter.On("Command", "gitlab-runner-helper", "artifacts-uploader",
		"--url", gitlabURL,
		"--token", "token",
		"--id", "1000",
		"--path", "always").Once()
	mockWriter.On("Command", "gitlab-runner-helper", "artifacts-uploader",
		"--url", gitlabURL,
		"--token", "token",
		"--id", "1000",
		"--path", "zip-archive",
		"--artifact-format", "zip",
		"--artifact-type", "archive").Once()
	mockWriter.On("Command", "gitlab-runner-helper", "artifacts-uploader",
		"--url", gitlabURL,
		"--token", "token",
		"--id", "1000",
		"--path", "gzip-junit",
		"--artifact-format", "gzip",
		"--artifact-type", "junit").Once()
	mockWriter.On("Else")
	mockWriter.On("Warning", mock.Anything, mock.Anything, mock.Anything)
	mockWriter.On("EndIf")

	err := shell.writeScript(mockWriter, common.BuildStageUploadOnSuccessArtifacts, info)
	require.NoError(t, err)
}

func TestWriteWritingArtifactsOnFailure(t *testing.T) {
	gitlabURL := "https://example.com:3443"

	shell := AbstractShell{}
	build := &common.Build{
		JobResponse: common.JobResponse{
			ID:    1000,
			Token: "token",
			Artifacts: common.Artifacts{
				common.Artifact{
					Paths: []string{"default"},
				},
				common.Artifact{
					Paths: []string{"on-success"},
					When:  common.ArtifactWhenOnSuccess,
				},
				common.Artifact{
					Paths: []string{"on-failure"},
					When:  common.ArtifactWhenOnFailure,
				},
				common.Artifact{
					Paths: []string{"always"},
					When:  common.ArtifactWhenAlways,
				},
				common.Artifact{
					Paths:  []string{"zip-archive"},
					When:   common.ArtifactWhenAlways,
					Format: common.ArtifactFormatZip,
					Type:   "archive",
				},
				common.Artifact{
					Paths:  []string{"gzip-junit"},
					When:   common.ArtifactWhenAlways,
					Format: common.ArtifactFormatGzip,
					Type:   "junit",
				},
			},
		},
		Runner: &common.RunnerConfig{
			RunnerCredentials: common.RunnerCredentials{
				URL: gitlabURL,
			},
		},
	}
	info := common.ShellScriptInfo{
		RunnerCommand: "gitlab-runner-helper",
		Build:         build,
	}

	mockWriter := new(mocks.ShellWriter)
	defer mockWriter.AssertExpectations(t)
	mockWriter.On("Variable", mock.Anything)
	mockWriter.On("Cd", mock.Anything)
	mockWriter.On("IfCmd", "gitlab-runner-helper", "--version")
	mockWriter.On("Notice", mock.Anything)
	mockWriter.On("Command", "gitlab-runner-helper", "artifacts-uploader",
		"--url", gitlabURL,
		"--token", "token",
		"--id", "1000",
		"--path", "on-failure").Once()
	mockWriter.On("Command", "gitlab-runner-helper", "artifacts-uploader",
		"--url", gitlabURL,
		"--token", "token",
		"--id", "1000",
		"--path", "always").Once()
	mockWriter.On("Command", "gitlab-runner-helper", "artifacts-uploader",
		"--url", gitlabURL,
		"--token", "token",
		"--id", "1000",
		"--path", "zip-archive",
		"--artifact-format", "zip",
		"--artifact-type", "archive").Once()
	mockWriter.On("Command", "gitlab-runner-helper", "artifacts-uploader",
		"--url", gitlabURL,
		"--token", "token",
		"--id", "1000",
		"--path", "gzip-junit",
		"--artifact-format", "gzip",
		"--artifact-type", "junit").Once()
	mockWriter.On("Else")
	mockWriter.On("Warning", mock.Anything, mock.Anything, mock.Anything)
	mockWriter.On("EndIf")

	err := shell.writeScript(mockWriter, common.BuildStageUploadOnFailureArtifacts, info)
	require.NoError(t, err)
}
