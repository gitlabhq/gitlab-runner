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

func TestBash_CheckForErrors(t *testing.T) {
	tests := map[string]struct {
		checkForErrors bool
		expected       string
	}{
		"enabled": {
			checkForErrors: true,
			// nolint:lll
			expected: "$'echo \\'hello world\\''\n_runner_exit_code=$?; if [[ $_runner_exit_code -ne 0 ]]; then exit $_runner_exit_code; fi\n",
		},
		"disabled": {
			checkForErrors: false,
			expected:       "$'echo \\'hello world\\''\n",
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			writer := &BashWriter{checkForErrors: tc.checkForErrors}
			writer.Command("echo 'hello world'")

			assert.Equal(t, tc.expected, writer.String())
		})
	}
}
