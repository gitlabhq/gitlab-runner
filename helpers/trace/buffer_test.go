package trace

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVariablesMasking(t *testing.T) {
	traceMessage := "This is the secret message cont@ining :secret duplicateValues"
	maskedValues := []string{
		"is",
		"duplicateValue",
		"duplicateValue",
		":secret",
		"cont@ining",
	}

	buffer, err := New()
	require.NoError(t, err)
	defer buffer.Close()

	buffer.SetMasked(maskedValues)

	_, err = buffer.Write([]byte(traceMessage))
	require.NoError(t, err)

	buffer.Finish()

	content, err := buffer.Bytes(0, 1000)
	require.NoError(t, err)

	assert.Equal(t, "Th[MASKED] [MASKED] the secret message [MASKED] [MASKED] [MASKED]s", string(content))
}

func TestTraceLimit(t *testing.T) {
	traceMessage := "This is the long message"

	buffer, err := New()
	require.NoError(t, err)
	defer buffer.Close()

	buffer.SetLimit(10)

	for i := 0; i < 100; i++ {
		_, err = buffer.Write([]byte(traceMessage))
		require.NoError(t, err)
	}

	buffer.Finish()

	content, err := buffer.Bytes(0, 1000)
	require.NoError(t, err)

	assert.Equal(t, "This is the\n\x1b[31;1mJob's log exceeded limit of 10 bytes.\x1b[0;m\n", string(content))
}
