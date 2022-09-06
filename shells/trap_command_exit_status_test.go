//go:build !integration

package shells

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTryDecode(t *testing.T) {
	exitCode := 0
	script := "script"

	correct, err := json.Marshal(trapCommandExitStatusImpl{
		CommandExitCode: &exitCode,
		Script:          &script,
	})
	require.NoError(t, err)

	missingCommandExitCode, err := json.Marshal(trapCommandExitStatusImpl{
		CommandExitCode: nil,
		Script:          &script,
	})
	require.NoError(t, err)

	missingScripts, err := json.Marshal(trapCommandExitStatusImpl{
		CommandExitCode: &exitCode,
		Script:          nil,
	})
	require.NoError(t, err)

	noFields, err := json.Marshal(trapCommandExitStatusImpl{
		CommandExitCode: nil,
		Script:          nil,
	})
	require.NoError(t, err)

	tests := map[string]struct {
		from                    string
		verifyCommandExitFn     func(t *testing.T, err error, c trapCommandExitStatusImpl)
		verifyTrapCommandExitFn func(t *testing.T, decoded bool, c TrapCommandExitStatus)
	}{
		"TryUnmarshal correct": {
			from: string(correct),
			verifyCommandExitFn: func(t *testing.T, err error, c trapCommandExitStatusImpl) {
				assert.NoError(t, err)
				assert.Equal(t, exitCode, *c.CommandExitCode)
				assert.Equal(t, script, *c.Script)
			},
			verifyTrapCommandExitFn: func(t *testing.T, decoded bool, c TrapCommandExitStatus) {
				assert.True(t, decoded)
				assert.Equal(t, exitCode, c.CommandExitCode)
			},
		},
		"TryUnmarshal only json prefix incorrect": {
			from:                    string(correct[:len(correct)-1]),
			verifyCommandExitFn:     verifyDecodingError,
			verifyTrapCommandExitFn: verifyNotDecoded,
		},
		"TryUnmarshal no json prefix incorrect": {
			from:                    string(correct[1:]),
			verifyCommandExitFn:     verifyDecodingError,
			verifyTrapCommandExitFn: verifyNotDecoded,
		},
		"TryUnmarshal empty": {
			from:                    "",
			verifyCommandExitFn:     verifyDecodingError,
			verifyTrapCommandExitFn: verifyNotDecoded,
		},
		"TryUnmarshal missing exit code": {
			from: string(missingCommandExitCode),
			verifyCommandExitFn: func(t *testing.T, err error, c trapCommandExitStatusImpl) {
				assert.NoError(t, err)
				assert.Nil(t, c.CommandExitCode)
				assert.Equal(t, script, *c.Script)
			},
			verifyTrapCommandExitFn: func(t *testing.T, decoded bool, c TrapCommandExitStatus) {
				assert.False(t, decoded)
				assert.Zero(t, c.CommandExitCode)
			},
		},
		"TryUnmarshal missing scripts": {
			from: string(missingScripts),
			verifyCommandExitFn: func(t *testing.T, err error, c trapCommandExitStatusImpl) {
				assert.NoError(t, err)
				assert.Equal(t, exitCode, *c.CommandExitCode)
				assert.Nil(t, c.Script)
			},
			verifyTrapCommandExitFn: func(t *testing.T, decoded bool, c TrapCommandExitStatus) {
				assert.True(t, decoded)
				assert.Equal(t, exitCode, c.CommandExitCode)
			},
		},
		"TryUnmarshal no fields": {
			from: string(noFields),
			verifyCommandExitFn: func(t *testing.T, err error, c trapCommandExitStatusImpl) {
				assert.NoError(t, err)
				assert.Nil(t, c.CommandExitCode)
				assert.Nil(t, c.Script)
			},
			verifyTrapCommandExitFn: func(t *testing.T, decoded bool, c TrapCommandExitStatus) {
				assert.False(t, decoded)
				assert.Zero(t, c.CommandExitCode)
			},
		},
		"TryUnmarshal hand crafted json with all fields": {
			from: `{"command_exit_code": 0, "script": "script"}`,
			verifyCommandExitFn: func(t *testing.T, err error, c trapCommandExitStatusImpl) {
				assert.NoError(t, err)
				assert.Equal(t, exitCode, *c.CommandExitCode)
				assert.Equal(t, script, *c.Script)
			},
			verifyTrapCommandExitFn: func(t *testing.T, decoded bool, c TrapCommandExitStatus) {
				assert.True(t, decoded)
				assert.Equal(t, exitCode, c.CommandExitCode)
			},
		},
		"TryUnmarshal hand crafted json missing exit code": {
			from: `{"script": "script"}`,
			verifyCommandExitFn: func(t *testing.T, err error, c trapCommandExitStatusImpl) {
				assert.NoError(t, err)
				assert.Nil(t, c.CommandExitCode)
				assert.Equal(t, script, *c.Script)
			},
			verifyTrapCommandExitFn: func(t *testing.T, decoded bool, c TrapCommandExitStatus) {
				assert.False(t, decoded)
				assert.Zero(t, c.CommandExitCode)
			},
		},
		"TryUnmarshal hand crafted json missing script": {
			from: `{"command_exit_code": 0}`,
			verifyCommandExitFn: func(t *testing.T, err error, c trapCommandExitStatusImpl) {
				assert.NoError(t, err)
				assert.Equal(t, exitCode, *c.CommandExitCode)
				assert.Nil(t, c.Script)
			},
			verifyTrapCommandExitFn: func(t *testing.T, decoded bool, c TrapCommandExitStatus) {
				assert.True(t, decoded)
				assert.Equal(t, exitCode, c.CommandExitCode)
			},
		},
		"TryUnmarshal hand crafted empty json": {
			from: "{}",
			verifyCommandExitFn: func(t *testing.T, err error, c trapCommandExitStatusImpl) {
				assert.NoError(t, err)
				assert.Nil(t, c.CommandExitCode)
				assert.Nil(t, c.Script)
			},
			verifyTrapCommandExitFn: func(t *testing.T, decoded bool, c TrapCommandExitStatus) {
				assert.False(t, decoded)
				assert.Zero(t, c.CommandExitCode)
			},
		},
		"TryUnmarshal hand crafted invalid json": {
			from: "{invalid json",
			verifyCommandExitFn: func(t *testing.T, err error, c trapCommandExitStatusImpl) {
				assert.Error(t, err)
				assert.Nil(t, c.CommandExitCode)
				assert.Nil(t, c.Script)
			},
			verifyTrapCommandExitFn: func(t *testing.T, decoded bool, c TrapCommandExitStatus) {
				assert.False(t, decoded)
				assert.Zero(t, c.CommandExitCode)
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			var cmd trapCommandExitStatusImpl
			err := cmd.tryUnmarshal(tt.from)
			tt.verifyCommandExitFn(t, err, cmd)

			var c TrapCommandExitStatus
			decoded := c.TryUnmarshal(tt.from)
			tt.verifyTrapCommandExitFn(t, decoded, c)
		})
	}
}

func verifyDecodingError(t *testing.T, err error, c trapCommandExitStatusImpl) {
	t.Helper()

	assert.Error(t, err)
	assert.Nil(t, c.CommandExitCode)
	assert.Nil(t, c.Script)
}

func verifyNotDecoded(t *testing.T, decoded bool, c TrapCommandExitStatus) {
	t.Helper()

	assert.False(t, decoded)
	assert.Zero(t, c.CommandExitCode)
}
