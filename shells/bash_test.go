//go:build !integration

package shells

import (
	"path"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestBash_CommandShellEscapesLegacy(t *testing.T) {
	writer := &BashWriter{useNewEscape: false}
	writer.Command("foo", "x&(y)")

	assert.Equal(t, `$'foo' $'x&(y)'`+"\n", writer.String())
}

func TestBash_IfCmdShellEscapesLegacy(t *testing.T) {
	writer := &BashWriter{useNewEscape: false}
	writer.IfCmd("foo", "x&(y)")

	assert.Equal(t, `if $'foo' $'x&(y)' >/dev/null 2>&1; then`+"\n", writer.String())
}

func TestBash_CommandShellEscapes(t *testing.T) {
	tests := []struct {
		command  string
		args     []string
		expected string
	}{
		{
			command:  "foo",
			args:     []string{"x&(y)"},
			expected: "foo $'x&(y)'\n",
		},
		{
			command:  "echo",
			args:     []string{"c:\\windows"},
			expected: "echo $'c:\\\\windows'\n",
		},
		{
			command:  "echo",
			args:     []string{"'$HOME'"},
			expected: "echo $'\\'$HOME\\''\n",
		},
	}

	for _, tc := range tests {
		writer := &BashWriter{useNewEscape: true}
		writer.Command(tc.command, tc.args...)

		assert.Equal(t, tc.expected, writer.String())
	}
}

func TestBash_IfCmdShellEscapes(t *testing.T) {
	writer := &BashWriter{useNewEscape: true}
	writer.IfCmd("foo", "x&(y)")

	assert.Equal(t, "if foo $'x&(y)' >/dev/null 2>&1; then\n", writer.String())
}

func TestBash_CheckForErrors(t *testing.T) {
	tests := map[string]struct {
		checkForErrors bool
		expected       string
	}{
		"enabled": {
			checkForErrors: true,
			// nolint:lll
			expected: "$'echo \\'hello world\\''\n_runner_exit_code=$?; if [ $_runner_exit_code -ne 0 ]; then exit $_runner_exit_code; fi\n",
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

func TestBash_GetConfiguration(t *testing.T) {
	tests := map[string]struct {
		info common.ShellScriptInfo
		cmd  string
		args []string
		os   string
	}{
		`bash`: {
			info: common.ShellScriptInfo{Shell: "bash", Type: common.NormalShell},
			cmd:  "bash",
		},
		`bash -l`: {
			info: common.ShellScriptInfo{Shell: "bash", Type: common.LoginShell},
			cmd:  "bash",
			args: []string{"-l"},
		},
		`su -s /bin/bash foobar -c bash`: {
			info: common.ShellScriptInfo{Shell: "bash", User: "foobar", Type: common.NormalShell},
			cmd:  "su",
			args: []string{"-s", "/bin/bash", "foobar", "-c", "bash"},
			os:   OSLinux,
		},
		`su -s /bin/bash foobar -c $'bash -l'`: {
			info: common.ShellScriptInfo{Shell: "bash", User: "foobar", Type: common.LoginShell},
			cmd:  "su",
			args: []string{"-s", "/bin/bash", "foobar", "-c", "bash -l"},
			os:   OSLinux,
		},
		`su -s /bin/sh foobar -c $'sh -l'`: {
			info: common.ShellScriptInfo{Shell: "sh", User: "foobar", Type: common.LoginShell},
			cmd:  "su",
			args: []string{"-s", "/bin/sh", "foobar", "-c", "sh -l"},
			os:   OSLinux,
		},
		`su foobar -c $'bash -l'`: {
			info: common.ShellScriptInfo{Shell: "bash", User: "foobar", Type: common.LoginShell},
			cmd:  "su",
			args: []string{"foobar", "-c", "bash -l"},
			os:   "darwin",
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			if tc.os != "" && tc.os != runtime.GOOS {
				t.Skipf("test only runs on %s", tc.os)
			}

			sh := BashShell{Shell: tc.info.Shell}
			config, err := sh.GetConfiguration(tc.info)
			require.NoError(t, err)

			assert.Equal(t, tc.cmd, config.Command)
			assert.Equal(t, tc.args, config.Arguments)
			assert.Equal(t, tn, config.CmdLine)
		})
	}
}

func Test_BashWriter_isTmpFile(t *testing.T) {
	tmpDir := "/foo/bar"
	bw := BashWriter{TemporaryPath: tmpDir}

	tests := map[string]struct {
		path string
		want bool
	}{
		"tmp file var":     {path: path.Join(tmpDir, "BAZ"), want: true},
		"not tmp file var": {path: "bla bla bla", want: false},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.want, bw.isTmpFile(tt.path))
		})
	}
}

func Test_BashWriter_cleanPath(t *testing.T) {
	tests := map[string]struct {
		path, want string
	}{
		"relative path": {
			path: "foo/bar/KEY",
			want: "$PWD/foo/bar/KEY",
		},
		"absolute path": {
			path: "/foo/bar/KEY",
			want: "/foo/bar/KEY",
		},
		"idempotent": {
			path: "$PWD/foo/bar/KEY",
			want: "$PWD/foo/bar/KEY",
		},
	}

	bw := BashWriter{TemporaryPath: "foo/bar"}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := bw.cleanPath(tt.path)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_BashWriter_Variable(t *testing.T) {
	tests := map[string]struct {
		variable common.JobVariable
		writer   BashWriter
		want     string
	}{
		"file var, relative path": {
			variable: common.JobVariable{Key: "KEY", Value: "the secret", File: true},
			writer:   BashWriter{TemporaryPath: "foo/bar"},
			// nolint:lll
			want: "mkdir -p \"foo/bar\"\nprintf '%s' $'the secret' > \"$PWD/foo/bar/KEY\"\nexport KEY=\"$PWD/foo/bar/KEY\"\n",
		},
		"file var, absolute path": {
			variable: common.JobVariable{Key: "KEY", Value: "the secret", File: true},
			writer:   BashWriter{TemporaryPath: "/foo/bar"},
			// nolint:lll
			want: "mkdir -p \"/foo/bar\"\nprintf '%s' $'the secret' > \"/foo/bar/KEY\"\nexport KEY=\"/foo/bar/KEY\"\n",
		},
		"tmp file var, relative path": {
			variable: common.JobVariable{Key: "KEY", Value: "foo/bar/KEY2"},
			writer:   BashWriter{TemporaryPath: "foo/bar"},
			want:     "export KEY=$'$PWD/foo/bar/KEY2'\n",
		},
		"tmp file var, absolute path": {
			variable: common.JobVariable{Key: "KEY", Value: "/foo/bar/KEY2"},
			writer:   BashWriter{TemporaryPath: "/foo/bar"},
			want:     "export KEY=$'/foo/bar/KEY2'\n",
		},
		"regular var": {
			variable: common.JobVariable{Key: "KEY", Value: "VALUE"},
			writer:   BashWriter{TemporaryPath: "/foo/bar"},
			want:     "export KEY=VALUE\n",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			tt.writer.Variable(tt.variable)
			assert.Equal(t, tt.want, tt.writer.String())
		})
	}
}
