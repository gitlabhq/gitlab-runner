//go:build !integration

package spec

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/moa/limits"
)

func inputsWithStringInput(t *testing.T, size int) Inputs {
	t.Helper()

	data, err := json.Marshal([]map[string]any{{
		"key": "big",
		"value": map[string]any{
			"type":      "string",
			"content":   strings.Repeat("a", size),
			"sensitive": false,
		},
	}})
	require.NoError(t, err)

	inputs := Inputs{}
	require.NoError(t, json.Unmarshal(data, &inputs))

	return inputs
}

func TestJobInputs_Expand_DeepNestingIsRejected(t *testing.T) {
	t.Parallel()

	for _, depth := range []int{5_000, 1_000_000} {
		t.Run(fmt.Sprintf("depth %d", depth), func(t *testing.T) {
			t.Parallel()

			inputs := inputsWithStringInput(t, 1)
			text := "${{ " + strings.Repeat("(", depth) + "1" + strings.Repeat(")", depth) + " }}"

			_, err := inputs.Expand(text)

			var interpolationErr *InputInterpolationError
			require.ErrorAs(t, err, &interpolationErr)
			require.ErrorIs(t, err, limits.ErrDepthExceeded)
		})
	}
}

func TestJobInputs_Expand_ContainerFlatteningIsBounded(t *testing.T) {
	t.Parallel()

	const references = 40
	inputs := inputsWithStringInput(t, 512<<10)

	small, err := inputs.Expand("${{ [job.inputs.big, job.inputs.big] }}")
	require.NoError(t, err)
	assert.Greater(t, len(small), 1<<20)

	text := "${{ [" + strings.Repeat("job.inputs.big, ", references) + "] }}"

	_, err = inputs.Expand(text)

	var interpolationErr *InputInterpolationError
	require.ErrorAs(t, err, &interpolationErr)
	require.ErrorIs(t, err, limits.ErrOutputTooLarge)
}

func TestJobInputs_Expand_OutputBudgetIsSharedAcrossFields(t *testing.T) {
	t.Parallel()

	inputs := inputsWithStringInput(t, 3<<20)

	for range 2 {
		_, err := inputs.Expand(`${{ job.inputs.big + "x" }}`)
		require.NoError(t, err)
	}

	_, err := inputs.Expand(`${{ job.inputs.big + "x" }}`)

	var interpolationErr *InputInterpolationError
	require.ErrorAs(t, err, &interpolationErr)
	require.ErrorIs(t, err, limits.ErrOutputTooLarge)
}

func TestJobInputs_Expand_BudgetIsPerJob(t *testing.T) {
	t.Parallel()

	for range 2 {
		inputs := inputsWithStringInput(t, 3<<20)

		for range 2 {
			_, err := inputs.Expand(`${{ job.inputs.big + "x" }}`)
			require.NoError(t, err)
		}
	}
}

func TestJobInputs_Expand_PlainTextIsNotCharged(t *testing.T) {
	t.Parallel()

	inputs := inputsWithStringInput(t, 3<<20)
	plain := strings.Repeat("a", 1<<20)

	for range 9 {
		expanded, err := inputs.Expand(plain)
		require.NoError(t, err)
		assert.Len(t, expanded, len(plain))
	}

	_, err := inputs.Expand(`${{ job.inputs.big + "x" }}`)
	require.NoError(t, err)
}
