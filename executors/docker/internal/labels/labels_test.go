//go:build !integration

package labels

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestNewLabeler(t *testing.T) {
	l := NewLabeler(&common.Build{})

	assert.IsType(t, new(labeler), l)
}

func TestLabels(t *testing.T) {
	b := &common.Build{
		JobResponse: common.JobResponse{
			ID: 12345,
			GitInfo: common.GitInfo{
				Sha:       "sha",
				BeforeSha: "before-sha",
				Ref:       "ref",
				RepoURL:   "https://ci-job-token:ToKeN123@gitlab.example.com/namespace/project.git",
			},
			JobInfo: common.JobInfo{
				ProjectID: 123456,
			},
		},
		Runner: &common.RunnerConfig{
			RunnerCredentials: common.RunnerCredentials{
				Token: "test-token",
			},
		},
		RunnerID: 123,
	}

	l := NewLabeler(b)

	expected := map[string]string{
		"com.gitlab.gitlab-runner.job.id":          "12345",
		"com.gitlab.gitlab-runner.job.url":         "https://gitlab.example.com/namespace/project/-/jobs/12345",
		"com.gitlab.gitlab-runner.job.sha":         "sha",
		"com.gitlab.gitlab-runner.job.before_sha":  "before-sha",
		"com.gitlab.gitlab-runner.job.ref":         "ref",
		"com.gitlab.gitlab-runner.project.id":      "123456",
		"com.gitlab.gitlab-runner.pipeline.id":     "",
		"com.gitlab.gitlab-runner.runner.id":       "test-tok",
		"com.gitlab.gitlab-runner.runner.local_id": "123",
		"com.gitlab.gitlab-runner.managed":         "true",
		"com.gitlab.gitlab-runner.other.label1":    "1",
		"com.gitlab.gitlab-runner.other.label2":    "2",
	}

	actual := l.Labels(map[string]string{"other.label1": "1", "other.label2": "2"})

	assert.Equal(t, expected, actual)
}
