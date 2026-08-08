//go:build !integration

package docker

import (
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestNewPullManagerConfigWiresServerAPIVersion(t *testing.T) {
	e := new(executor)
	e.Config.Docker = &common.DockerConfig{}
	e.Build = &common.Build{Runner: &common.RunnerConfig{}}
	e.serverAPIVersion = version.Must(version.NewVersion("1.44"))

	config := newPullManagerConfig(e)

	assert.Same(t, e.serverAPIVersion, config.APIVersion, "pull manager should use the negotiated API version")
}
