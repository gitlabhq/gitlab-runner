//go:build !integration

package env

import (
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandValue_GitLabEnvTakesPriorityOverEnv(t *testing.T) {
	e := &Env{
		Env:       map[string]string{"MY_VAR": "static"},
		GitLabEnv: map[string]string{"MY_VAR": "dynamic"},
	}

	assert.Equal(t, "dynamic", e.ExpandValue("$MY_VAR"),
		"GitLabEnv overlay must shadow Env for the same key")
}

// TestExpandValue_ShellEscapes covers `$$` escape and shell special
// parameter handling, per
// https://docs.gitlab.com/ci/variables/#use-variables-in-other-variables.
func TestExpandValue_ShellEscapes(t *testing.T) {
	e := &Env{Env: map[string]string{
		// Names matching shell special params must resolve to empty,
		// not leak the same-keyed env entry.
		"$": "should-not-leak-dollar",
		"*": "should-not-leak-star",
		"0": "should-not-leak-zero",
		"?": "should-not-leak-qm",
	}}

	cases := map[string]struct {
		in   string
		want string
	}{
		"$$ expands to literal $":                     {"$$", "$"},
		"$$ adjacent to text preserves $":             {"PRICE=$$100", "PRICE=$100"},
		"$$ inside ${} braces":                        {"${PATH:-$$}", ""}, // ${...} with default not supported; just guard no panic + no leak
		"$* resolves to empty":                        {"$*", ""},
		"$0 resolves to empty":                        {"$0", ""},
		"$? resolves to empty":                        {"$?", ""},
		"unknown var resolves to empty":               {"$NO_SUCH_VAR", ""},
		"plain literal passes through":                {"literal text", "literal text"},
		"empty string in -> empty string out":         {"", ""},
		"escape followed by var leaves var intact":    {"$$VAR_NAME", "$VAR_NAME"},
		"escape inside ${} should not eat the braces": {"$${BRACED}", "${BRACED}"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, e.ExpandValue(tc.in))
		})
	}
}

func TestLookPath_UsesJobPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX executable bits")
	}

	dirs := map[string]string{}
	for _, n := range []string{"process", "env", "gitlab", "extra"} {
		dirs[n] = t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dirs[n], "tool"), nil, 0o755))
	}
	t.Setenv("PATH", dirs["process"])

	nonExec := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(nonExec, "tool"), nil, 0o644))

	tests := map[string]struct {
		env, gitlab, extra string
		want               string
	}{
		"process PATH when the job sets none": {want: "process"},
		"job variable PATH":                   {env: dirs["env"], want: "env"},
		"GITLAB_ENV PATH beats job variable":  {env: dirs["env"], gitlab: dirs["gitlab"], want: "gitlab"},
		"extra PATH beats both":               {env: dirs["env"], gitlab: dirs["gitlab"], extra: dirs["extra"], want: "extra"},
		"non-executable file is skipped":      {env: nonExec + ":" + dirs["env"], want: "env"},
		"relative entries are skipped":        {env: ".", want: ""},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			e := &Env{Env: map[string]string{}, GitLabEnv: map[string]string{}}
			var extra map[string]string
			if tc.env != "" {
				e.Env["PATH"] = tc.env
			}
			if tc.gitlab != "" {
				e.GitLabEnv["PATH"] = tc.gitlab
			}
			if tc.extra != "" {
				extra = map[string]string{"PATH": tc.extra}
			}

			got, err := e.LookPath("tool", extra)
			if tc.want == "" {
				require.ErrorIs(t, err, exec.ErrNotFound)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, filepath.Join(dirs[tc.want], "tool"), got)
		})
	}
}

func TestLookPath_Windows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows Path key and PATHEXT")
	}

	procDir, jobDir := t.TempDir(), t.TempDir()
	for _, d := range []string{procDir, jobDir} {
		require.NoError(t, os.WriteFile(filepath.Join(d, "tool.exe"), nil, 0o644))
	}
	t.Setenv("Path", procDir)

	tests := map[string]struct {
		jobKey string
		want   string
	}{
		"process Path when the job sets none": {want: procDir},
		"job PATH beats process Path":         {jobKey: "PATH", want: jobDir},
		"job key matched case-insensitively":  {jobKey: "path", want: jobDir},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			e := &Env{Env: map[string]string{}, GitLabEnv: map[string]string{}}
			if tc.jobKey != "" {
				e.Env[tc.jobKey] = jobDir
			}

			got, err := e.LookPath("tool", nil)
			require.NoError(t, err)
			assert.Equal(t, filepath.Join(tc.want, "tool.exe"), got)
		})
	}
}

func TestHelperEnvs_UsesJobPath(t *testing.T) {
	processPath := t.TempDir()
	t.Setenv("PATH", processPath)
	jobPath, gitlabPath, extraPath := t.TempDir(), t.TempDir(), t.TempDir()
	pathKey := "PATH"
	if runtime.GOOS == "windows" {
		pathKey = "Path"
	}

	tests := map[string]struct {
		env, gitlab, extra map[string]string
		want               string
		noBundle           bool
	}{
		"process PATH": {want: processPath},
		"job PATH":     {env: map[string]string{pathKey: jobPath}, want: jobPath},
		"other job variables are not included": {
			env: map[string]string{pathKey: jobPath, "SECRET": "value"}, want: jobPath,
		},
		"GITLAB_ENV overrides job PATH": {
			env: map[string]string{"PATH": jobPath}, gitlab: map[string]string{pathKey: gitlabPath}, want: gitlabPath,
		},
		"extra overrides job and GITLAB_ENV PATH": {
			env: map[string]string{"PATH": jobPath}, gitlab: map[string]string{"PATH": gitlabPath},
			extra: map[string]string{pathKey: extraPath, "OTHER": "value"}, want: extraPath,
		},
		"empty job PATH":   {env: map[string]string{pathKey: ""}},
		"empty extra PATH": {env: map[string]string{"PATH": jobPath}, extra: map[string]string{pathKey: ""}},
		"PATH unchanged without bundled Git": {
			extra: map[string]string{"PATH": extraPath}, want: extraPath, noBundle: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			e := &Env{Env: tc.env, GitLabEnv: tc.gitlab}
			// Supply a bundle without relying on the test executable's installation layout.
			e.resolveBundleOnce.Do(func() {})
			e.bundledGit = filepath.Join(t.TempDir(), "git")
			if tc.noBundle {
				e.bundledGit = "git"
			}
			original := make(map[string]string)
			maps.Copy(original, tc.extra)

			got := e.HelperEnvs(tc.extra)
			wantPath := filepath.Dir(e.bundledGit) + string(os.PathListSeparator) + tc.want
			if tc.noBundle {
				wantPath = tc.want
			}
			assert.Equal(t, wantPath, got["PATH"])
			assert.NotContains(t, got, "SECRET")
			for k, v := range original {
				assert.Equal(t, v, tc.extra[k], "input must not be mutated")
				if isPathKey(k) {
					if k != "PATH" {
						assert.NotContains(t, got, k, "avoid duplicate Windows PATH keys")
					}
				} else {
					assert.Equal(t, v, got[k])
				}
			}
		})
	}
}
