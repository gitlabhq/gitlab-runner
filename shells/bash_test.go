//go:build !integration

package shells

import (
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

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
		writer := &BashWriter{}
		writer.Command(tc.command, tc.args...)

		assert.Equal(t, tc.expected, writer.String())
	}
}

func TestBash_IfCmdShellEscapes(t *testing.T) {
	writer := &BashWriter{}
	writer.IfCmd("foo", "x&(y)")

	assert.Equal(t, "if foo $'x&(y)' >/dev/null 2>&1 ; then\n", writer.String())
}

func TestBash_GenerateSaveScript(t *testing.T) {
	info := common.ShellScriptInfo{
		Shell: "bash",
		Build: &common.Build{Runner: &common.RunnerConfig{}},
	}

	script, err := common.GetShell("bash").GenerateSaveScript(info, "path", "echo hi")
	require.NoError(t, err)
	assert.Equal(t, "echo $'echo hi' > path.tmp\nchmod 777 path.tmp\nmv path.tmp path\n", script)
}

// TestBashWriter_IfFileReadable pins the guard emitted around optional git
// config includes: [ -r ] is exactly the access(R_OK) probe git itself
// uses for optional config files, and %q quoting must keep a runtime $HOME
// reference unescaped so it expands in the executor's environment.
func TestBashWriter_IfFileReadable(t *testing.T) {
	tests := map[string]struct {
		path     string
		expected string
	}{
		"runtime HOME reference": {
			path:     "$HOME/.gitconfig",
			expected: `if [ -r "$HOME/.gitconfig" ]; then` + "\n",
		},
		"path with spaces": {
			path:     "/home/user name/.gitconfig",
			expected: `if [ -r "/home/user name/.gitconfig" ]; then` + "\n",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			writer := &BashWriter{}
			writer.IfFileReadable(tc.path)

			assert.Equal(t, tc.expected, writer.String())
		})
	}
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
		variable spec.Variable
		writer   BashWriter
		want     string
	}{
		"file var, relative path": {
			variable: spec.Variable{Key: "KEY", Value: "the secret", File: true},
			writer:   BashWriter{TemporaryPath: "foo/bar"},
			// nolint:lll
			want: "mkdir -p \"foo/bar\"\nprintf '%s' $'the secret' > \"$PWD/foo/bar/KEY\"\nexport KEY=\"$PWD/foo/bar/KEY\"\n",
		},
		"file var, absolute path": {
			variable: spec.Variable{Key: "KEY", Value: "the secret", File: true},
			writer:   BashWriter{TemporaryPath: "/foo/bar"},
			// nolint:lll
			want: "mkdir -p \"/foo/bar\"\nprintf '%s' $'the secret' > \"/foo/bar/KEY\"\nexport KEY=\"/foo/bar/KEY\"\n",
		},
		"tmp file var, relative path": {
			variable: spec.Variable{Key: "KEY", Value: "foo/bar/KEY2"},
			writer:   BashWriter{TemporaryPath: "foo/bar"},
			want:     "export KEY=$'$PWD/foo/bar/KEY2'\n",
		},
		"tmp file var, absolute path": {
			variable: spec.Variable{Key: "KEY", Value: "/foo/bar/KEY2"},
			writer:   BashWriter{TemporaryPath: "/foo/bar"},
			want:     "export KEY=/foo/bar/KEY2\n",
		},
		"regular var": {
			variable: spec.Variable{Key: "KEY", Value: "VALUE"},
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

func Test_BashWriter_DotEnvVariables(t *testing.T) {
	tests := map[string]struct {
		variables map[string]string
		writer    BashWriter
		want      string
	}{
		"single key": {
			variables: map[string]string{
				"KEY": "the secret",
			},
			writer: BashWriter{TemporaryPath: "foo/bar"},
			want: `cat << EOF > $PWD/foo/bar/test
KEY="the secret"
EOF

`,
		},
		"multiline key": {
			variables: map[string]string{
				"KEY": "one\ntwo",
				"FOO": "test",
			},
			writer: BashWriter{TemporaryPath: "foo/bar"},
			want: `cat << EOF > $PWD/foo/bar/test
FOO="test"
KEY="one\ntwo"
EOF

`,
		},
		"key with special characters": {
			variables: map[string]string{
				"BAR":  `the $DOLLAR_STORE`,
				"BAZ":  "some\t\r\nthing",
				"FOO":  `the "right" stuff`,
				"TEST": `some \ / thing`,
			},
			writer: BashWriter{TemporaryPath: "foo/bar"},
			want: `cat << EOF > $PWD/foo/bar/test
BAR="the $DOLLAR_STORE"
BAZ="some	\r\nthing"
FOO="the \"right\" stuff"
TEST="some \\ / thing"
EOF

`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			tt.writer.DotEnvVariables("test", tt.variables)
			assert.Equal(t, tt.want, tt.writer.String())
		})
	}
}

func TestBashEntrypointCommand(t *testing.T) {
	tests := map[string]struct {
		probeFile       string
		expectedCommand []string
		shellType       common.ShellType
	}{
		"normal shell/no probe": {
			shellType:       common.NormalShell,
			expectedCommand: []string{"sh", "-c", "if [ -x /usr/local/bin/bash ]; then\n\texec /usr/local/bin/bash \nelif [ -x /usr/bin/bash ]; then\n\texec /usr/bin/bash \nelif [ -x /bin/bash ]; then\n\texec /bin/bash \nelif [ -x /usr/local/bin/sh ]; then\n\texec /usr/local/bin/sh \nelif [ -x /usr/bin/sh ]; then\n\texec /usr/bin/sh \nelif [ -x /bin/sh ]; then\n\texec /bin/sh \nelif [ -x /busybox/sh ]; then\n\texec /busybox/sh \nelse\n\techo shell not found\n\texit 1\nfi\n\n"},
		},
		"normal shell/with probe": {
			shellType:       common.NormalShell,
			probeFile:       "someFile",
			expectedCommand: []string{"sh", "-c", ">'someFile'; if [ -x /usr/local/bin/bash ]; then\n\texec /usr/local/bin/bash \nelif [ -x /usr/bin/bash ]; then\n\texec /usr/bin/bash \nelif [ -x /bin/bash ]; then\n\texec /bin/bash \nelif [ -x /usr/local/bin/sh ]; then\n\texec /usr/local/bin/sh \nelif [ -x /usr/bin/sh ]; then\n\texec /usr/bin/sh \nelif [ -x /bin/sh ]; then\n\texec /bin/sh \nelif [ -x /busybox/sh ]; then\n\texec /busybox/sh \nelse\n\techo shell not found\n\texit 1\nfi\n\n"},
		},
		"login shell/no probe": {
			shellType:       common.LoginShell,
			expectedCommand: []string{"sh", "-c", "if [ -x /usr/local/bin/bash ]; then\n\texec /usr/local/bin/bash -l\nelif [ -x /usr/bin/bash ]; then\n\texec /usr/bin/bash -l\nelif [ -x /bin/bash ]; then\n\texec /bin/bash -l\nelif [ -x /usr/local/bin/sh ]; then\n\texec /usr/local/bin/sh -l\nelif [ -x /usr/bin/sh ]; then\n\texec /usr/bin/sh -l\nelif [ -x /bin/sh ]; then\n\texec /bin/sh -l\nelif [ -x /busybox/sh ]; then\n\texec /busybox/sh -l\nelse\n\techo shell not found\n\texit 1\nfi\n\n"},
		},
		"login shell/with probe": {
			shellType:       common.LoginShell,
			probeFile:       "someFile",
			expectedCommand: []string{"sh", "-c", ">'someFile'; if [ -x /usr/local/bin/bash ]; then\n\texec /usr/local/bin/bash -l\nelif [ -x /usr/bin/bash ]; then\n\texec /usr/bin/bash -l\nelif [ -x /bin/bash ]; then\n\texec /bin/bash -l\nelif [ -x /usr/local/bin/sh ]; then\n\texec /usr/local/bin/sh -l\nelif [ -x /usr/bin/sh ]; then\n\texec /usr/bin/sh -l\nelif [ -x /bin/sh ]; then\n\texec /bin/sh -l\nelif [ -x /busybox/sh ]; then\n\texec /busybox/sh -l\nelse\n\techo shell not found\n\texit 1\nfi\n\n"},
		},
		"interactive shell/no probe": {
			shellType:       common.InteractiveShell,
			expectedCommand: []string{"sh", "-c", "if [ -x /usr/local/bin/bash ]; then\n\texec /usr/local/bin/bash -l\nelif [ -x /usr/bin/bash ]; then\n\texec /usr/bin/bash -l\nelif [ -x /bin/bash ]; then\n\texec /bin/bash -l\nelif [ -x /usr/local/bin/sh ]; then\n\texec /usr/local/bin/sh -l\nelif [ -x /usr/bin/sh ]; then\n\texec /usr/bin/sh -l\nelif [ -x /bin/sh ]; then\n\texec /bin/sh -l\nelif [ -x /busybox/sh ]; then\n\texec /busybox/sh -l\nelse\n\techo shell not found\n\texit 1\nfi\n\n"},
		},
		"interactive shell/with probe": {
			shellType:       common.InteractiveShell,
			probeFile:       "someFile",
			expectedCommand: []string{"sh", "-c", ">'someFile'; if [ -x /usr/local/bin/bash ]; then\n\texec /usr/local/bin/bash -l\nelif [ -x /usr/bin/bash ]; then\n\texec /usr/bin/bash -l\nelif [ -x /bin/bash ]; then\n\texec /bin/bash -l\nelif [ -x /usr/local/bin/sh ]; then\n\texec /usr/local/bin/sh -l\nelif [ -x /usr/bin/sh ]; then\n\texec /usr/bin/sh -l\nelif [ -x /bin/sh ]; then\n\texec /bin/sh -l\nelif [ -x /busybox/sh ]; then\n\texec /busybox/sh -l\nelse\n\techo shell not found\n\texit 1\nfi\n\n"},
		},
	}
	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			shell := common.GetShell("bash")
			shellScriptInfo := common.ShellScriptInfo{Type: tc.shellType}

			actualCommand := shell.GetEntrypointCommand(shellScriptInfo, tc.probeFile)
			assert.Equal(t, tc.expectedCommand, actualCommand)
		})
	}
}

func TestBash_RmDir(t *testing.T) {
	tests := map[string]struct {
		path     string
		expected string
	}{
		"simple path": {
			path: "/path/to/dir",
			expected: "if [ -d \"/path/to/dir\" ]; then\n" +
				"  chmod \"-R\" \"u+rwX\" \"/path/to/dir\"\n" +
				"fi\n" +
				"rm \"-r\" \"-f\" \"/path/to/dir\"\n",
		},
		"path with spaces": {
			path: "/path/with spaces/dir",
			expected: "if [ -d \"/path/with spaces/dir\" ]; then\n" +
				"  chmod \"-R\" \"u+rwX\" \"/path/with spaces/dir\"\n" +
				"fi\n" +
				"rm \"-r\" \"-f\" \"/path/with spaces/dir\"\n",
		},
		"path with special characters": {
			path: "/path/$VAR/dir",
			expected: "if [ -d \"/path/$VAR/dir\" ]; then\n" +
				"  chmod \"-R\" \"u+rwX\" \"/path/$VAR/dir\"\n" +
				"fi\n" +
				"rm \"-r\" \"-f\" \"/path/$VAR/dir\"\n",
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			writer := &BashWriter{}
			writer.RmDir(tc.path)

			assert.Equal(t, tc.expected, writer.String())
		})
	}
}

func TestBash_RmFile(t *testing.T) {
	tests := map[string]struct {
		path     string
		expected string
	}{
		"simple path": {
			path:     "/path/to/file.txt",
			expected: "rm \"-f\" \"/path/to/file.txt\"\n",
		},
		"path with spaces": {
			path:     "/path/with spaces/file.txt",
			expected: "rm \"-f\" \"/path/with spaces/file.txt\"\n",
		},
		"path with variable": {
			path:     "/path/$VAR/file.txt",
			expected: "rm \"-f\" \"/path/$VAR/file.txt\"\n",
		},
		"path with special characters": {
			path:     "/path/to/file-name_123.txt",
			expected: "rm \"-f\" \"/path/to/file-name_123.txt\"\n",
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			writer := &BashWriter{}
			writer.RmFile(tc.path)

			assert.Equal(t, tc.expected, writer.String())
		})
	}
}

func TestBash_CommandArgExpand(t *testing.T) {
	tests := map[string]struct {
		command  string
		args     []string
		expected string
	}{
		"simple command": {
			command:  "echo",
			args:     []string{"hello"},
			expected: "echo \"hello\"\n",
		},
		"command with variable": {
			command:  "chmod",
			args:     []string{"-R", "u+rwX", "$BUILD_DIR"},
			expected: "chmod \"-R\" \"u+rwX\" \"$BUILD_DIR\"\n",
		},
		"rm command with path": {
			command:  "rm",
			args:     []string{"-r", "-f", "/path/to/dir"},
			expected: "rm \"-r\" \"-f\" \"/path/to/dir\"\n",
		},
		"path with spaces": {
			command:  "rm",
			args:     []string{"-f", "/path/with spaces/file"},
			expected: "rm \"-f\" \"/path/with spaces/file\"\n",
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			writer := &BashWriter{}
			writer.CommandArgExpand(tc.command, tc.args...)

			assert.Equal(t, tc.expected, writer.String())
		})
	}
}

func TestBash_Finish_EvalForm(t *testing.T) {
	tests := map[string]struct {
		useLegacyBashEval bool
		wantSubstr        string
		notWantSubstr     string
	}{
		"default emits trapped subshell with stdin from /dev/null": {
			useLegacyBashEval: false,
			wantSubstr:        `(trap 'exit 1' TERM; eval $'echo hi\n') < /dev/null`,
			notWantSubstr:     ": | eval",
		},
		"FF_USE_LEGACY_BASH_EVAL=true emits legacy pipeline form": {
			useLegacyBashEval: true,
			wantSubstr:        `: | eval $'echo hi\n'`,
			notWantSubstr:     "< /dev/null",
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			w := &BashWriter{Shell: "bash", useLegacyBashEval: tc.useLegacyBashEval}
			w.Line("echo hi")

			out := w.Finish(false)

			assert.Contains(t, out, tc.wantSubstr,
				"generated script should contain expected eval form")
			assert.NotContains(t, out, tc.notWantSubstr,
				"generated script should not contain the other eval form")
		})
	}
}

func TestBash_PrepareStageCPUInfoMessage(t *testing.T) {
	tests := map[string]struct {
		shell      string
		stage      common.BuildStage
		wantCPUMsg bool
	}{
		"sh prepare stage prints CPU info": {
			shell:      "sh",
			stage:      common.BuildStagePrepare,
			wantCPUMsg: true,
		},
		"bash prepare stage prints CPU info": {
			shell:      "bash",
			stage:      common.BuildStagePrepare,
			wantCPUMsg: true,
		},
		"bash cleanup stage does not print CPU info": {
			shell:      "bash",
			stage:      common.BuildStageCleanup,
			wantCPUMsg: false,
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			info := common.ShellScriptInfo{
				Shell: tc.shell,
				Build: &common.Build{Runner: &common.RunnerConfig{}},
			}

			script, err := common.GetShell(tc.shell).GenerateScript(t.Context(), tc.stage, info)
			require.NoError(t, err)

			if tc.wantCPUMsg {
				assert.Contains(t, script, "Running on CPU: ")
			} else {
				assert.NotContains(t, script, "Running on CPU: ")
			}
		})
	}
}

// TestBash_CPUInfoMessage runs the whole prepare stage script, as the writer escapes it, with
// a real shell. It reads a cpuinfo fixture instead of /proc/cpuinfo, and stubs lscpu and
// hostname, so the CPU and tools of the host running the test don't affect the result.
func TestBash_CPUInfoMessage(t *testing.T) {
	if runtime.GOOS == OSWindows {
		t.Skip("requires a POSIX shell")
	}

	const x86CPUInfo = "processor\t: 0\n" +
		"model name\t: AMD EPYC 9B45\n" +
		"flags\t\t: fpu sse4_2 avx avx2 avx512f avx512vl\n"
	const arm64CPUInfo = "processor\t: 0\nFeatures\t: fp asimd\n"

	tests := map[string]struct {
		cpuinfo  string // no cpuinfo file is created when empty
		lscpu    string // lscpu fails when empty
		expected string
	}{
		"model and extensions from cpuinfo": {
			cpuinfo:  x86CPUInfo,
			lscpu:    "Model name:            Ignored\n",
			expected: "Running on CPU: AMD EPYC 9B45 (avx, avx2, avx512f)",
		},
		"model from lscpu": {
			cpuinfo:  arm64CPUInfo,
			lscpu:    "Architecture:          aarch64\nModel name:            Neoverse-N1\n",
			expected: "Running on CPU: Neoverse-N1",
		},
		"lscpu model name is a dash": {
			cpuinfo:  arm64CPUInfo,
			lscpu:    "Architecture:          aarch64\nModel name:            -\n",
			expected: "Running on CPU: unknown",
		},
		"no cpuinfo and no lscpu": {
			expected: "Running on CPU: unknown",
		},
	}

	shells := map[string]struct {
		shell        string
		path         string
		posixEscapes bool
	}{
		// sh on dash can't run the default $'...' escapes, so it needs POSIX escapes.
		"sh":   {shell: "sh", path: "/bin/sh", posixEscapes: true},
		"bash": {shell: "bash", path: "bash"},
	}

	for sn, sc := range shells {
		shellPath, err := exec.LookPath(sc.path)
		if err != nil {
			t.Logf("skipping %s: %v", sn, err)
			continue
		}

		info := common.ShellScriptInfo{
			Shell: sc.shell,
			Build: &common.Build{Runner: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					FeatureFlags: map[string]bool{featureflags.PosixlyCorrectEscapes: sc.posixEscapes},
				},
			}},
		}
		generated, err := common.GetShell(sc.shell).GenerateScript(t.Context(), common.BuildStagePrepare, info)
		require.NoError(t, err)
		require.Contains(t, generated, "/proc/cpuinfo")

		for tn, tc := range tests {
			t.Run(sn+"/"+tn, func(t *testing.T) {
				dir := t.TempDir()
				cpuinfo := filepath.Join(dir, "cpuinfo")
				if tc.cpuinfo != "" {
					require.NoError(t, os.WriteFile(cpuinfo, []byte(tc.cpuinfo), 0o644))
				}

				lscpu := "#!/bin/sh\nexit 1\n"
				if tc.lscpu != "" {
					lscpu = "#!/bin/sh\nprintf '" + tc.lscpu + "'\n"
				}

				bin := filepath.Join(dir, "bin")
				require.NoError(t, os.Mkdir(bin, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(bin, "lscpu"), []byte(lscpu), 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(bin, "hostname"), []byte("#!/bin/sh\necho test\n"), 0o755))

				script := filepath.Join(dir, "script")
				require.NoError(t, os.WriteFile(script, []byte(strings.ReplaceAll(generated, "/proc/cpuinfo", cpuinfo)), 0o755))

				var stderr strings.Builder
				cmd := exec.Command(shellPath, script)
				cmd.Dir = dir
				cmd.Env = []string{"PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH")}
				cmd.Stderr = &stderr
				out, err := cmd.Output()
				require.NoError(t, err, "stderr: %s", stderr.String())
				assert.Empty(t, stderr.String(), "the probe must not write to stderr")
				assert.Equal(t, "Running on test...\n"+tc.expected+"\n", string(out))
			})
		}
	}
}

func TestBash_InteractiveShellHasNoEffectIn(t *testing.T) {
	tests := map[string]struct {
		shellType common.ShellType
		expected  string
	}{
		"bash login shell produces -l": {
			shellType: common.LoginShell,
			expected:  "bash -l",
		},
		"bash interactive shell produces -l": {
			shellType: common.InteractiveShell,
			expected:  "bash -l",
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			shell := common.GetShell("bash")
			require.NotNil(t, shell)

			info := common.ShellScriptInfo{
				Shell: "bash",
				Type:  tc.shellType,
				Build: &common.Build{
					Runner: &common.RunnerConfig{},
				},
			}

			config, err := shell.GetConfiguration(info)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, config.CmdLine)
		})
	}
}

func Test_BashWriter_IfGitVersionIsAtLeast(t *testing.T) {
	tests := map[string]struct {
		version string
		// nolint:lll
		want string
	}{
		"version 2.49": {
			version: "2.49",
			want: `current_ver="$(git version|cut -d ' ' -f 3)"
required_ver="2.49"
minimum_ver="$(printf '%s\n%s' "$required_ver" "$current_ver" | sort -t '.' -k 1,1n -k 2,2n -k 3,3n | sed -n '1p')"
if [ "$minimum_ver" = "$required_ver" ]; then
echo $'\x1b[0;mGit version at least "2.49"'
`,
		},
		"version 2.0": {
			version: "2.0",
			want: `current_ver="$(git version|cut -d ' ' -f 3)"
required_ver="2.0"
minimum_ver="$(printf '%s\n%s' "$required_ver" "$current_ver" | sort -t '.' -k 1,1n -k 2,2n -k 3,3n | sed -n '1p')"
if [ "$minimum_ver" = "$required_ver" ]; then
echo $'\x1b[0;mGit version at least "2.0"'
`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			writer := &BashWriter{}
			writer.IfGitVersionIsAtLeast(tt.version)

			assert.Equal(t, tt.want, writer.String())
			// Ensure the generated script does not use `head -n1`, which causes
			// SIGPIPE under `set -o pipefail` + `set -o errexit` and silently
			// aborts the get_sources script. See:
			// https://gitlab.com/gitlab-org/gitlab-runner/-/work_items/39680
			assert.NotContains(t, writer.String(), "head -n1")
		})
	}
}
