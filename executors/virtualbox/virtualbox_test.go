//go:build !integration

package virtualbox

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestVirtualBoxExecutorRegistered(t *testing.T) {
	executorNames := common.GetExecutorNames()
	assert.Contains(t, executorNames, "virtualbox")
}

func TestVirtualBoxCreateExecutor(t *testing.T) {
	executor := common.NewExecutor("virtualbox")
	assert.NotNil(t, executor)
}
