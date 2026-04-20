//go:build !integration

package labels

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
)

func TestNewLabeler(t *testing.T) {
	l := NewLabeler(&common.Build{})

	assert.IsType(t, new(labeler), l)
}

func TestLabels(t *testing.T) {
	b := &common.Build{
		Job: spec.Job{
			ID: 12345,
			GitInfo: spec.GitInfo{
				Sha:       "sha",
				BeforeSha: "before-sha",
				Ref:       "ref",
				RepoURL:   "https://ci-job-token:ToKeN123@gitlab.example.com/namespace/project.git",
			},
			JobInfo: spec.JobInfo{
				ProjectID: 123456,
			},
		},
		Runner: &common.RunnerConfig{
			RunnerCredentials: common.RunnerCredentials{
				Token: "test-token",
			},
			SystemID: "some-system-ID",
		},
		RunnerID:        123,
		ProjectRunnerID: 456,
	}

	l := NewLabeler(b)

	expected := map[string]string{
		"com.gitlab.gitlab-runner.job.id":            "12345",
		"com.gitlab.gitlab-runner.job.url":           "https://gitlab.example.com/namespace/project/-/jobs/12345",
		"com.gitlab.gitlab-runner.job.sha":           "sha",
		"com.gitlab.gitlab-runner.job.before_sha":    "before-sha",
		"com.gitlab.gitlab-runner.job.ref":           "ref",
		"com.gitlab.gitlab-runner.job.timeout":       "2h0m0s",
		"com.gitlab.gitlab-runner.project.id":        "123456",
		"com.gitlab.gitlab-runner.project.runner_id": "456",
		"com.gitlab.gitlab-runner.pipeline.id":       "",
		"com.gitlab.gitlab-runner.runner.id":         "test-toke",
		"com.gitlab.gitlab-runner.runner.local_id":   "123",
		"com.gitlab.gitlab-runner.runner.system_id":  "some-system-ID",
		"com.gitlab.gitlab-runner.managed":           "true",
		"com.gitlab.gitlab-runner.other.label1":      "1",
		"com.gitlab.gitlab-runner.other.label2":      "2",
	}

	actual := l.Labels(map[string]string{"other.label1": "1", "other.label2": "2"})

	assert.Equal(t, expected, actual)
}

func TestLabels_pipelineIDSupport(t *testing.T) {
	tests := map[string]struct {
		pipelineIDInJobPayload  *int64
		pipelineIDInJobVariable *spec.Variable
		expectedPipelineIDLabel string
	}{
		"pipelineID in job payload only": {
			pipelineIDInJobPayload:  func(i int64) *int64 { return &i }(987654321),
			expectedPipelineIDLabel: "987654321",
		},
		"pipelineID in job variable only": {
			pipelineIDInJobVariable: &spec.Variable{
				Key:   "CI_PIPELINE_ID",
				Value: "123456789",
			},
			expectedPipelineIDLabel: "123456789",
		},
		"pipelineID in job variable and job payload": {
			pipelineIDInJobPayload: func(i int64) *int64 { return &i }(987654321),
			pipelineIDInJobVariable: &spec.Variable{
				Key:   "CI_PIPELINE_ID",
				Value: "123456789",
			},
			expectedPipelineIDLabel: "987654321",
		},
		"pipelineID not present at all": {
			expectedPipelineIDLabel: "",
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			b := &common.Build{
				Job: spec.Job{
					ID: 12345,
					GitInfo: spec.GitInfo{
						Sha:       "sha",
						BeforeSha: "before-sha",
						Ref:       "ref",
						RepoURL:   "https://ci-job-token:ToKeN123@gitlab.example.com/namespace/project.git",
					},
					JobInfo: spec.JobInfo{
						ProjectID: 123456,
					},
					Variables: make([]spec.Variable, 0, 1),
				},
				Runner: &common.RunnerConfig{
					RunnerCredentials: common.RunnerCredentials{
						Token: "test-token",
					},
					SystemID: "some-system-ID",
				},
				RunnerID:        123,
				ProjectRunnerID: 456,
			}

			if tt.pipelineIDInJobPayload != nil {
				b.Job.JobInfo.PipelineID = *tt.pipelineIDInJobPayload
			}

			if tt.pipelineIDInJobVariable != nil {
				b.Job.Variables = append(b.Job.Variables, *tt.pipelineIDInJobVariable)
			}

			l := NewLabeler(b)
			labels := l.Labels(map[string]string{"other": "label"})

			t.Log(labels)

			pipelineIDLabelKey := dockerLabelPrefix + ".pipeline.id"

			require.Contains(t, labels, pipelineIDLabelKey)
			assert.Equal(t, tt.expectedPipelineIDLabel, labels[pipelineIDLabelKey])
		})
	}
}
