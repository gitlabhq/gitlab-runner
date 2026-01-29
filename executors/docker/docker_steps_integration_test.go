//go:build integration

package docker_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildtest"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/test"
)

var successAlwaysWantOut = []string{
	`Executing "step_run" stage of the job script`,
	"Job succeeded",
}

func Test_StepsIntegration(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	helpers.SkipIntegrationTests(t, "docker", "info")
	test.SkipIfVariable(t, "CI_SKIP_STEPS_TESTS")

	tests := map[string]struct {
		steps     string
		variables spec.Variables
		services  spec.Services
		wantOut   []string
		wantErr   bool
	}{
		"script": {
			steps: `- name: echo
  script: echo foo bar baz
- name: ls
  script: ls -lh
- name: env
  script: env`,
			wantOut: []string{
				"foo bar baz",
				"PWD=/builds/gitlab-org/ci-cd/gitlab-runner-pipeline-tests/gitlab-test",
			},
		},
		"remote step": {
			steps: `- name: echo
  step: "https://gitlab.com/gitlab-org/ci-cd/runner-tools/echo-step@v5"
  inputs:
    echo: foo bar baz`,
			wantOut: []string{"foo bar baz"},
		},
		"local step": {
			steps: `- name: localecho
  step: "./steps/echo"
  inputs:
    message: foo bar baz`,
			wantOut: []string{"foo bar baz"},
		},
		"file variable": {
			steps: `- name: cat
  script: cat ${{ job.A_FILE_VAR }}`,
			variables: spec.Variables{{Key: "A_FILE_VAR", Value: "oh this is soo secret", File: true}},
			wantOut:   []string{"oh this is soo secret"},
		},
		"job variables should not appear in environment": {
			steps: `- name: echo
  script: echo ${{ env.FLIN_FLAN_FLON }}`,
			variables: spec.Variables{{Key: "FLIN_FLAN_FLON", Value: "flin, flan, flon"}},
			wantOut: []string{
				"ERROR: Job failed:",
				`evaluating expression failed at ".FLIN_FLAN_FLON": attribute not found`,
			},
			wantErr: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			successfulBuild, err := common.GetRemoteStepsBuildResponse(tt.steps)
			assert.NoError(t, err)

			successfulBuild.Services = tt.services
			successfulBuild.Variables = append(successfulBuild.Variables, tt.variables...)
			build := &common.Build{
				Job: successfulBuild,
				Runner: &common.RunnerConfig{
					RunnerSettings: common.RunnerSettings{
						Executor: "docker",
						Docker: &common.DockerConfig{
							Image:      "fedora:latest",
							PullPolicy: common.StringOrArray{common.PullPolicyIfNotPresent},
							Privileged: true,
						},
					},
				},
			}

			wantOut := tt.wantOut
			out, err := buildtest.RunBuildReturningOutput(t, build)
			if !tt.wantErr {
				assert.NoError(t, err)
				wantOut = append(wantOut, successAlwaysWantOut...)
			} else {
				assert.Error(t, err)
			}

			for _, want := range wantOut {
				assert.Contains(t, out, want)
			}
		})
	}
}
