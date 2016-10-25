package shells

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPowershell_CommandShellEscapes(t *testing.T) {
	writer := &PsWriter{}
	writer.Command("foo", "x&(y)")

	assert.Equal(t, "& \"foo\" \"x&(y)\"\r\nif(!$?) { Exit $LASTEXITCODE }\r\n\r\n", writer.String())
}

func TestPowershell_IfCmdShellEscapes(t *testing.T) {
	writer := &PsWriter{}
	writer.IfCmd("foo", "x&(y)")

	assert.Equal(t, "& \"foo\" \"x&(y)\" 2>$null\r\nif($?) {\r\n", writer.String())
}
