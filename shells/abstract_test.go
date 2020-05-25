package shells

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/tls"
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

	mockWriter := new(MockShellWriter)
	mockWriter.On("EnvVariableKey", tls.VariableCAFile).Return("VariableCAFile").Once()
	mockWriter.On("EnvVariableKey", tls.VariableCertFile).Return("VariableCertFile").Once()
	mockWriter.On("EnvVariableKey", tls.VariableKeyFile).Return("VariableKeyFile").Once()

	mockWriter.On("Command", "git", "config", fmt.Sprintf("http.%s.%s", gitlabURL, "sslCAInfo"), "VariableCAFile").Once()
	mockWriter.On("Command", "git", "config", fmt.Sprintf("http.%s.%s", gitlabURL, "sslCert"), "VariableCertFile").Once()
	mockWriter.On("Command", "git", "config", fmt.Sprintf("http.%s.%s", gitlabURL, "sslKey"), "VariableKeyFile").Once()

	shell.writeGitSSLConfig(mockWriter, build, nil)

	mockWriter.AssertExpectations(t)
}

func getJobResponseWithMultipleArtifacts(t *testing.T) common.JobResponse {
	return common.JobResponse{
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
	}
}

func TestWriteWritingArtifactsOnSuccess(t *testing.T) {
	gitlabURL := "https://example.com:3443"

	shell := AbstractShell{}
	build := &common.Build{
		JobResponse: getJobResponseWithMultipleArtifacts(t),
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

	mockWriter := new(MockShellWriter)
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
		JobResponse: getJobResponseWithMultipleArtifacts(t),
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

	mockWriter := new(MockShellWriter)
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

func TestWriteWritingArtifactsWithExcludedPaths(t *testing.T) {
	shell := AbstractShell{}

	build := &common.Build{
		JobResponse: common.JobResponse{
			ID:    1001,
			Token: "token",
			Artifacts: common.Artifacts{
				common.Artifact{
					Paths:   []string{"include/**"},
					Exclude: []string{"include/exclude/*"},
					When:    common.ArtifactWhenAlways,
					Format:  common.ArtifactFormatZip,
					Type:    "archive",
				},
			},
		},
		Runner: &common.RunnerConfig{
			RunnerCredentials: common.RunnerCredentials{
				URL: "https://gitlab.example.com",
			},
		},
	}

	info := common.ShellScriptInfo{
		RunnerCommand: "gitlab-runner-helper",
		Build:         build,
	}

	mockWriter := new(MockShellWriter)
	defer mockWriter.AssertExpectations(t)
	mockWriter.On("Variable", mock.Anything)
	mockWriter.On("Cd", mock.Anything).Once()
	mockWriter.On("IfCmd", "gitlab-runner-helper", "--version").Once()
	mockWriter.On("Notice", mock.Anything).Once()
	mockWriter.On("Command", "gitlab-runner-helper", "artifacts-uploader",
		"--url", "https://gitlab.example.com",
		"--token", "token",
		"--id", "1001",
		"--path", "include/**",
		"--exclude", "include/exclude/*",
		"--artifact-format", "zip",
		"--artifact-type", "archive").Once()
	mockWriter.On("Else").Once()
	mockWriter.On("Warning", mock.Anything, mock.Anything, mock.Anything).Once()
	mockWriter.On("EndIf").Once()

	err := shell.writeScript(mockWriter, common.BuildStageUploadOnSuccessArtifacts, info)
	require.NoError(t, err)
}

func TestGitCleanFlags(t *testing.T) {
	tests := map[string]struct {
		value string

		expectedGitClean      bool
		expectedGitCleanFlags []interface{}
	}{
		"empty clean flags": {
			value:                 "",
			expectedGitClean:      true,
			expectedGitCleanFlags: []interface{}{"-ffdx"},
		},
		"use custom flags": {
			value:                 "custom-flags",
			expectedGitClean:      true,
			expectedGitCleanFlags: []interface{}{"custom-flags"},
		},
		"use custom flags with multiple arguments": {
			value:                 "-ffdx -e cache/",
			expectedGitClean:      true,
			expectedGitCleanFlags: []interface{}{"-ffdx", "-e", "cache/"},
		},
		"disabled": {
			value:            "none",
			expectedGitClean: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			shell := AbstractShell{}

			const dummySha = "01234567abcdef"
			const dummyRef = "master"

			build := &common.Build{
				Runner: &common.RunnerConfig{},
				JobResponse: common.JobResponse{
					GitInfo: common.GitInfo{Sha: dummySha, Ref: dummyRef},
					Variables: common.JobVariables{
						{Key: "GIT_CLEAN_FLAGS", Value: test.value},
					},
				},
			}

			mockWriter := new(MockShellWriter)
			defer mockWriter.AssertExpectations(t)

			mockWriter.On("Notice", "Checking out %s as %s...", dummySha[0:8], dummyRef).Once()
			mockWriter.On("Command", "git", "checkout", "-f", "-q", dummySha).Once()

			if test.expectedGitClean {
				command := []interface{}{"git", "clean"}
				command = append(command, test.expectedGitCleanFlags...)
				mockWriter.On("Command", command...).Once()
			}

			shell.writeCheckoutCmd(mockWriter, build)
		})
	}
}

func TestGitFetchFlags(t *testing.T) {
	tests := map[string]struct {
		value string

		expectedGitFetchFlags []interface{}
	}{
		"empty fetch flags": {
			value:                 "",
			expectedGitFetchFlags: []interface{}{"--prune", "--quiet"},
		},
		"use custom flags": {
			value:                 "--prune",
			expectedGitFetchFlags: []interface{}{"--prune"},
		},
		"disabled": {
			value: "none",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			shell := AbstractShell{}

			const dummySha = "01234567abcdef"
			const dummyRef = "master"
			const dummyProjectDir = "./"
			const dummyGitDir = "./.git"

			build := &common.Build{
				Runner: &common.RunnerConfig{},
				JobResponse: common.JobResponse{
					GitInfo: common.GitInfo{Sha: dummySha, Ref: dummyRef, Depth: 0},
					Variables: common.JobVariables{
						{Key: "GIT_FETCH_EXTRA_FLAGS", Value: test.value},
					},
				},
			}

			mockWriter := new(MockShellWriter)
			defer mockWriter.AssertExpectations(t)

			mockWriter.On("Notice", "Fetching changes...").Once()
			mockWriter.On("MkTmpDir", mock.Anything).Return(mock.Anything).Once()
			mockWriter.On("Command", "git", "config", "-f", mock.Anything, "fetch.recurseSubmodules", "false").Once()
			mockWriter.On("Command", "git", "init", dummyProjectDir, "--template", mock.Anything).Once()
			mockWriter.On("Cd", mock.Anything)
			mockWriter.On("IfCmd", "git", "remote", "add", "origin", mock.Anything)
			mockWriter.On("RmFile", mock.Anything)
			mockWriter.On("Notice", "Created fresh repository.").Once()
			mockWriter.On("Else")
			mockWriter.On("Command", "git", "remote", "set-url", "origin", mock.Anything)
			mockWriter.On("EndIf")

			command := []interface{}{"git", "fetch", "origin"}
			command = append(command, test.expectedGitFetchFlags...)
			mockWriter.On("Command", command...)

			shell.writeRefspecFetchCmd(mockWriter, build, dummyProjectDir, dummyGitDir)
		})
	}
}

func TestAbstractShell_writeSubmoduleUpdateCmdRecursive(t *testing.T) {
	shell := AbstractShell{}
	mockWriter := new(MockShellWriter)
	defer mockWriter.AssertExpectations(t)

	mockWriter.On("Notice", "Updating/initializing submodules recursively...").Once()
	mockWriter.On("Command", "git", "submodule", "sync", "--recursive").Once()
	mockWriter.On("Command", "git", "submodule", "update", "--init", "--recursive").Once()
	mockWriter.On("Command", "git", "submodule", "foreach", "--recursive", "git clean -ffxd").Once()
	mockWriter.On("Command", "git", "submodule", "foreach", "--recursive", "git reset --hard").Once()
	mockWriter.On("IfCmd", "git", "lfs", "version").Once()
	mockWriter.On("Command", "git", "submodule", "foreach", "--recursive", "git lfs pull").Once()
	mockWriter.On("EndIf").Once()

	shell.writeSubmoduleUpdateCmd(mockWriter, &common.Build{}, true)
}

func TestAbstractShell_writeSubmoduleUpdateCmd(t *testing.T) {
	shell := AbstractShell{}
	mockWriter := new(MockShellWriter)
	defer mockWriter.AssertExpectations(t)

	mockWriter.On("Notice", "Updating/initializing submodules...").Once()
	mockWriter.On("Command", "git", "submodule", "sync").Once()
	mockWriter.On("Command", "git", "submodule", "update", "--init").Once()
	mockWriter.On("Command", "git", "submodule", "foreach", "git clean -ffxd").Once()
	mockWriter.On("Command", "git", "submodule", "foreach", "git reset --hard").Once()
	mockWriter.On("IfCmd", "git", "lfs", "version").Once()
	mockWriter.On("Command", "git", "submodule", "foreach", "git lfs pull").Once()
	mockWriter.On("EndIf").Once()

	shell.writeSubmoduleUpdateCmd(mockWriter, &common.Build{}, false)
}

func TestSkipBuildStage(t *testing.T) {
	stageTests := map[common.BuildStage]map[string]struct {
		JobResponse common.JobResponse
		Runner      common.RunnerConfig
	}{
		common.BuildStageRestoreCache: {
			"don't skip if cache has paths": {
				common.JobResponse{
					Cache: common.Caches{
						common.Cache{
							Paths: []string{"default"},
						},
					},
				},
				common.RunnerConfig{},
			},
			"don't skip if cache uses untracked files": {
				common.JobResponse{
					Cache: common.Caches{
						common.Cache{
							Untracked: true,
						},
					},
				},
				common.RunnerConfig{},
			},
		},

		common.BuildStageDownloadArtifacts: {
			"don't skip if job has any dependencies": {
				common.JobResponse{
					Dependencies: common.Dependencies{
						common.Dependency{
							ID:            1,
							ArtifactsFile: common.DependencyArtifactsFile{Filename: "dependency.txt"},
						},
					},
				},
				common.RunnerConfig{},
			},
		},

		common.BuildStageUserScript: {
			"don't skip if user script is defined": {
				common.JobResponse{
					Steps: common.Steps{
						common.Step{
							Name: common.StepNameScript,
						},
					},
				},
				common.RunnerConfig{},
			},
		},

		common.BuildStageAfterScript: {
			"don't skip if an after script is defined and has content": {
				common.JobResponse{
					Steps: common.Steps{
						common.Step{
							Name:   common.StepNameAfterScript,
							Script: common.StepScript{"echo 'hello world'"},
						},
					},
				},
				common.RunnerConfig{},
			},
		},

		common.BuildStageArchiveCache: {
			"don't skip if cache has paths": {
				common.JobResponse{
					Cache: common.Caches{
						common.Cache{
							Paths: []string{"default"},
						},
					},
				},
				common.RunnerConfig{},
			},
			"don't skip if cache uses untracked files": {
				common.JobResponse{
					Cache: common.Caches{
						common.Cache{
							Untracked: true,
						},
					},
				},
				common.RunnerConfig{},
			},
		},

		common.BuildStageUploadOnSuccessArtifacts: {
			"don't skip if artifact has paths and URL defined": {
				common.JobResponse{
					Artifacts: common.Artifacts{
						common.Artifact{
							When:  common.ArtifactWhenOnSuccess,
							Paths: []string{"default"},
						},
					},
				},
				common.RunnerConfig{
					RunnerCredentials: common.RunnerCredentials{
						URL: "https://example.com",
					},
				},
			},
			"don't skip if artifact uses untracked files and URL defined": {
				common.JobResponse{
					Artifacts: common.Artifacts{
						common.Artifact{
							When:      common.ArtifactWhenOnSuccess,
							Untracked: true,
						},
					},
				},
				common.RunnerConfig{
					RunnerCredentials: common.RunnerCredentials{
						URL: "https://example.com",
					},
				},
			},
		},

		common.BuildStageUploadOnFailureArtifacts: {
			"don't skip if artifact has paths and URL defined": {
				common.JobResponse{
					Artifacts: common.Artifacts{
						common.Artifact{
							When:  common.ArtifactWhenOnFailure,
							Paths: []string{"default"},
						},
					},
				},
				common.RunnerConfig{
					RunnerCredentials: common.RunnerCredentials{
						URL: "https://example.com",
					},
				},
			},
			"don't skip if artifact uses untracked files and URL defined": {
				common.JobResponse{
					Artifacts: common.Artifacts{
						common.Artifact{
							When:      common.ArtifactWhenOnFailure,
							Untracked: true,
						},
					},
				},
				common.RunnerConfig{
					RunnerCredentials: common.RunnerCredentials{
						URL: "https://example.com",
					},
				},
			},
		},
	}

	shell := AbstractShell{}
	for stage, tests := range stageTests {
		t.Run(string(stage), func(t *testing.T) {
			for tn, tc := range tests {
				t.Run(tn, func(t *testing.T) {
					build := &common.Build{
						JobResponse: common.JobResponse{},
						Runner:      &common.RunnerConfig{},
					}
					info := common.ShellScriptInfo{
						RunnerCommand: "gitlab-runner-helper",
						Build:         build,
					}

					// empty stages should always be skipped
					err := shell.writeScript(&BashWriter{}, stage, info)
					assert.True(t, errors.Is(err, common.ErrSkipBuildStage),
						"expected err %T, but got %T", common.ErrSkipBuildStage, err)

					// stages with bare minimum requirements should not be skipped
					build.JobResponse = tc.JobResponse
					build.Runner = &tc.Runner
					err = shell.writeScript(&BashWriter{}, stage, info)
					assert.NoError(t, err, "stage %v should not have been skipped", stage)
				})
			}
		})
	}
}
