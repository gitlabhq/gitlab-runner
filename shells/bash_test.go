package shells

import (
	"fmt"
	"testing"

	"github.com/opencontainers/runc/libcontainer/user"
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

var nonExistingUser = user.User{
	Name: "non-existing-user",
}
var existingUser = user.User{
	Name: "test-user",
	Uid:  60000,
	Gid:  60000,
}

var noUserShellScriptInfo = common.ShellScriptInfo{}
var nonExistingUserShellScriptInfo = common.ShellScriptInfo{
	User: nonExistingUser.Name,
}
var existingUserShellScriptInfo = common.ShellScriptInfo{
	User: existingUser.Name,
}

type fakeUserResolver struct {
	supported bool
}

func (ur *fakeUserResolver) getCredentials(userName string) (*common.CommandCredential, error) {
	if !ur.supported {
		return nil, user.ErrUnsupported
	}

	if userName == nonExistingUser.Name {
		return nil, fmt.Errorf("no matching entries in passwd file")
	}

	return &common.CommandCredential{UID: uint32(existingUser.Uid), GID: uint32(existingUser.Gid)}, nil
}

func TestBash_CommandCredentials(t *testing.T) {
	shell := &BashShell{Shell: "bash", userResolver: &fakeUserResolver{true}}

	script, err := shell.GetConfiguration(noUserShellScriptInfo)
	assert.NoError(t, err)
	require.NotNil(t, script)
	assert.Nil(t, script.CommandCredential)

	script, err = shell.GetConfiguration(nonExistingUserShellScriptInfo)
	assert.EqualError(t, err, "no matching entries in passwd file")
	assert.Nil(t, script)

	script, err = shell.GetConfiguration(existingUserShellScriptInfo)
	assert.NoError(t, err)
	require.NotNil(t, script)
	require.NotNil(t, script.CommandCredential)
	assert.Equal(t, uint32(existingUser.Uid), script.CommandCredential.UID)
	assert.Equal(t, uint32(existingUser.Gid), script.CommandCredential.GID)
}

func TestBash_CommandCredentialsUnsupported(t *testing.T) {
	shell := &BashShell{Shell: "bash", userResolver: &fakeUserResolver{false}}

	script, err := shell.GetConfiguration(noUserShellScriptInfo)
	assert.NoError(t, err)
	require.NotNil(t, script)
	assert.Nil(t, script.CommandCredential)

	script, err = shell.GetConfiguration(nonExistingUserShellScriptInfo)
	assert.NoError(t, err)
	require.NotNil(t, script)
	assert.Nil(t, script.CommandCredential)

	script, err = shell.GetConfiguration(existingUserShellScriptInfo)
	assert.NoError(t, err)
	require.NotNil(t, script)
	assert.Nil(t, script.CommandCredential)
}
