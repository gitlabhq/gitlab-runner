//go:build !integration

package docker

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestDockerConfigUpdate(t *testing.T) {
	testCases := map[string]struct {
		gpus string
	}{
		"gpus set to all": {
			gpus: "all",
		},
		"gpus with trailing space": {
			gpus: " ",
		},
	}

	for tn, tc := range testCases {
		t.Run(tn, func(t *testing.T) {
			config := common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{Docker: &common.DockerConfig{Gpus: tc.gpus}},
			}

			info := common.ConfigInfo{}
			configUpdater(&config, &info)
			assert.Equal(t, strings.Trim(tc.gpus, " "), info.Gpus)
		})
	}
}
