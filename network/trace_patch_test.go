package network

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

var traceContent = "test content"

func TestNewTracePatch(t *testing.T) {
	trace := bytes.NewBufferString(traceContent)
	tp, err := newTracePatch(*trace, 0)
	assert.NoError(t, err)

	assert.Equal(t, 0, tp.Offset())
	assert.Equal(t, len(traceContent), tp.TotalSize())
	assert.Equal(t, []byte(traceContent), tp.Patch())
}

func TestInvalidTracePatchInitialOffsetValue(t *testing.T) {
	trace := bytes.NewBufferString("test")
	_, err := newTracePatch(*trace, trace.Len()+10)
	assert.EqualError(t, err, "Range is invalid, limit can't be less than offset")
}

func TestTracePatch_PatchAfterSetNewOffset(t *testing.T) {
	trace := bytes.NewBufferString(traceContent)
	tp, err := newTracePatch(*trace, 0)
	assert.NoError(t, err)

	tp.SetNewOffset(5)
	assert.Equal(t, []byte("content"), tp.Patch())
}

func TestTracePatchEmptyPatch(t *testing.T) {
	trace := bytes.NewBufferString(traceContent)
	tp, err := newTracePatch(*trace, len(traceContent))
	assert.NoError(t, err)

	assert.Empty(t, tp.Patch())
}
