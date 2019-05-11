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

	assert.Equal(t, "Set-Variable -Name cmdErr -Value $false\r\nTry {\r\n  & \"foo\" \"x&(y)\" 2>$null\r\n  if(!$?) { throw $LASTEXITCODE }\r\n} Catch {\r\n  Set-Variable -Name cmdErr -Value $true\r\n}\r\nif(!$cmdErr) {\r\n", writer.String())
}
