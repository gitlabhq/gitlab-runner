package shells

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"
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


var noUserShellScriptInfo = common.ShellScriptInfo{}
var nonExistingUserShellScriptInfo = common.ShellScriptInfo{
	User: "non-existing-user",
}
var existingUserShellScriptInfo = common.ShellScriptInfo{
	User: "test-user",
}

func TestBash_CommandCredentials(t *testing.T) {
	shell := &BashShell{Shell: "bash"}

	script, err := shell.GetConfiguration(noUserShellScriptInfo)
	assert.NoError(t, err)
	require.NotNil(t, script)
	assert.Nil(t, script.CommandCredential)

	script, err = shell.GetConfiguration(nonExistingUserShellScriptInfo)
	assert.Error(t, err)
	assert.Nil(t, script)

	script, err = shell.GetConfiguration(existingUserShellScriptInfo)
	assert.NoError(t, err)
	require.NotNil(t, script)
	require.NotNil(t, script.CommandCredential)
	assert.Equal(t, 60000, script.CommandCredential.UID)
	assert.Equal(t, 60000, script.CommandCredential.GID)
}