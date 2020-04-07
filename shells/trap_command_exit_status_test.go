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

	correct, err := json.Marshal(TrapCommandExitStatus{
		CommandExitCode: &exitCode,
		Script:          &script,
	})
	require.NoError(t, err)

	missingCommandExitCode, err := json.Marshal(TrapCommandExitStatus{
		CommandExitCode: nil,
		Script:          &script,
	})
	require.NoError(t, err)

	missingScripts, err := json.Marshal(TrapCommandExitStatus{
		CommandExitCode: &exitCode,
		Script:          nil,
	})
	require.NoError(t, err)

	noFields, err := json.Marshal(TrapCommandExitStatus{
		CommandExitCode: nil,
		Script:          nil,
	})
	require.NoError(t, err)

	tests := map[string]struct {
		from string

		verifyFn func(t *testing.T, decoded bool, c TrapCommandExitStatus)
	}{
		"TryUnmarshal correct": {
			from: string(correct),
			verifyFn: func(t *testing.T, decoded bool, c TrapCommandExitStatus) {
				assert.True(t, decoded)
				assert.Equal(t, exitCode, *c.CommandExitCode)
				assert.Equal(t, script, *c.Script)
			},
		},
		"TryUnmarshal only json prefix incorrect": {
			from:     string(correct[:len(correct)-1]),
			verifyFn: verifyNotDecoded,
		},
		"TryUnmarshal no json prefix incorrect": {
			from:     string(correct[1:]),
			verifyFn: verifyNotDecoded,
		},
		"TryUnmarshal empty": {
			from:     "",
			verifyFn: verifyNotDecoded,
		},
		"TryUnmarshal missing exit code": {
			from: string(missingCommandExitCode),
			verifyFn: func(t *testing.T, decoded bool, c TrapCommandExitStatus) {
				assert.False(t, decoded)
				assert.Nil(t, c.CommandExitCode)
				assert.Equal(t, script, *c.Script)
			},
		},
		"TryUnmarshal missing scripts": {
			from: string(missingScripts),
			verifyFn: func(t *testing.T, decoded bool, c TrapCommandExitStatus) {
				assert.False(t, decoded)
				assert.Nil(t, c.Script)
				assert.Equal(t, exitCode, *c.CommandExitCode)
			},
		},
		"TryUnmarshal no fields": {
			from: string(noFields),
			verifyFn: func(t *testing.T, decoded bool, c TrapCommandExitStatus) {
				assert.False(t, decoded)
				assert.Nil(t, c.Script)
				assert.Nil(t, c.CommandExitCode)
			},
		},
		"TryUnmarshal hand crafted json with all fields": {
			from: `{"command_exit_code": 0, "script": "script"}`,
			verifyFn: func(t *testing.T, decoded bool, c TrapCommandExitStatus) {
				assert.True(t, decoded)
				assert.Equal(t, exitCode, *c.CommandExitCode)
				assert.Equal(t, script, *c.Script)
			},
		},
		"TryUnmarshal hand crafted json missing exit code": {
			from: `{"script": "script"}`,
			verifyFn: func(t *testing.T, decoded bool, c TrapCommandExitStatus) {
				assert.False(t, decoded)
				assert.Nil(t, c.CommandExitCode)
				assert.Equal(t, script, *c.Script)
			},
		},
		"TryUnmarshal hand crafted json missing script": {
			from: `{"command_exit_code": 0}`,
			verifyFn: func(t *testing.T, decoded bool, c TrapCommandExitStatus) {
				assert.False(t, decoded)
				assert.Equal(t, exitCode, *c.CommandExitCode)
				assert.Nil(t, c.Script)
			},
		},
		"TryUnmarshal hand crafted empty json": {
			from: "{}",
			verifyFn: func(t *testing.T, decoded bool, c TrapCommandExitStatus) {
				assert.False(t, decoded)
				assert.Nil(t, c.CommandExitCode)
				assert.Nil(t, c.Script)
			},
		},
		"TryUnmarshal hand crafted invalid json": {
			from: "{invalid json",
			verifyFn: func(t *testing.T, decoded bool, c TrapCommandExitStatus) {
				assert.False(t, decoded)
				assert.Nil(t, c.CommandExitCode)
				assert.Nil(t, c.Script)
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			var c TrapCommandExitStatus
			decoded := c.TryUnmarshal(tt.from)
			tt.verifyFn(t, decoded, c)
		})
	}
}

func verifyNotDecoded(t *testing.T, decoded bool, c TrapCommandExitStatus) {
	t.Helper()

	assert.False(t, decoded)
	assert.Nil(t, c.CommandExitCode)
	assert.Nil(t, c.Script)
}
