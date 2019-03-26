package volumes

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultRegistry(t *testing.T) {
	registry := &defaultRegistry{}
	assert.Empty(t, registry.Elements())

	registry.Append("element")
	require.Len(t, registry.Elements(), 1)
	assert.Equal(t, "element", registry.Elements()[0])
}
