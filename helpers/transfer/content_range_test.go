//go:build !integration

package transfer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseContentRangeTotal(t *testing.T) {
	t.Parallel()

	n, ok := ParseContentRangeTotal("bytes 0-0/69712157")
	require.True(t, ok)
	assert.EqualValues(t, 69712157, n)

	n, ok = ParseContentRangeTotal("bytes */69712157")
	require.True(t, ok)
	assert.EqualValues(t, 69712157, n)

	n, ok = ParseContentRangeTotal("  bytes 0-0/42  ")
	require.True(t, ok)
	assert.EqualValues(t, 42, n)

	_, ok = ParseContentRangeTotal("bytes 0-0/*")
	assert.False(t, ok)

	_, ok = ParseContentRangeTotal("")
	assert.False(t, ok)

	_, ok = ParseContentRangeTotal("invalid")
	assert.False(t, ok)

	_, ok = ParseContentRangeTotal("bytes 0-0/0")
	assert.False(t, ok)
}
