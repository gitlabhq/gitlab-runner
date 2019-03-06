package trace

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVariablesMasking(t *testing.T) {
	traceMessage := "This is the secret message containing secret duplicateValues"
	maskedValues := []string{
		"is",
		"duplicateValue",
		"duplicateValue",
		"secret",
		"containing",
	}

	buffer := New()
	buffer.SetMasked(maskedValues)

	_, err := buffer.Write([]byte(traceMessage))
	require.NoError(t, err)

	err = buffer.Close()
	require.NoError(t, err)

	assert.Equal(t, "Th[MASKED] [MASKED] the [MASKED] message [MASKED] [MASKED] [MASKED]s", buffer.String())
}

func TestTraceLimit(t *testing.T) {
	traceMessage := "This is the long message"

	buffer := New()
	buffer.SetLimit(10)

	_, err := buffer.Write([]byte(traceMessage))
	require.NoError(t, err)

	err = buffer.Close()
	require.NoError(t, err)

	assert.Contains(t, buffer.String(), "Job's log exceeded limit of")
}
