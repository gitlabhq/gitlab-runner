package shells

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBash_CommandShellEscapes(t *testing.T) {
	writer := &BashWriter{}
	writer.Command("foo", "x&(y)")

	assert.Equal(t, `$'foo' "x&(y)"`+"\n", writer.String())
}

func TestBash_IfCmdShellEscapes(t *testing.T) {
	writer := &BashWriter{}
	writer.IfCmd("foo", "x&(y)")

	assert.Equal(t, `if $'foo' "x&(y)" >/dev/null 2>/dev/null; then`+"\n", writer.String())
}
