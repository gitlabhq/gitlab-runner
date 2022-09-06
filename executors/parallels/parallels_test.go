//go:build !integration

package parallels

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestParallelsExecutorRegistered(t *testing.T) {
	executorNames := common.GetExecutorNames()
	assert.Contains(t, executorNames, "parallels")
}

func TestParallelsCreateExecutor(t *testing.T) {
	executor := common.NewExecutor("parallels")
	assert.NotNil(t, executor)
}
