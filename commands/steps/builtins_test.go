//go:build !integration

package steps

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/step-runner/pkg/di"
	"gitlab.com/gitlab-org/step-runner/pkg/runner"
)

// The runner's builtins wrap a whole job, so step-runner must not announce
// them: "Running step name=concrete" would otherwise open every job log.
func TestBuiltinOptions_Unannounced(t *testing.T) {
	find, err := di.NewContainer(builtinOptions()...).StepFuncFinder()
	require.NoError(t, err)

	for _, name := range []string{"script_legacy", "concrete"} {
		fn, err := find(name)
		require.NoError(t, err, name)
		assert.True(t, runner.IsUnannounced(fn), "%s must be registered with the runner.Unannounced option", name)
	}
}
