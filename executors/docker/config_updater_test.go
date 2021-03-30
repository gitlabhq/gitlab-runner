package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestDockerConfigUpdate(t *testing.T) {
	testCases := map[string]struct {
		gpus     string
		expected bool
	}{
		"gpus set to all": {
			gpus:     "all",
			expected: true,
		},
		"gpus blank": {
			expected: false,
		},
		"gpus set to whitepsace": {
			gpus:     " ",
			expected: false,
		},
	}

	for tn, tc := range testCases {
		t.Run(tn, func(t *testing.T) {
			config := common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{Docker: &common.DockerConfig{Gpus: tc.gpus}},
			}

			info := common.ConfigInfo{}
			configUpdater(&config, &info)
			assert.Equal(t, tc.expected, info.GpuEnabled)
		})
	}
}
