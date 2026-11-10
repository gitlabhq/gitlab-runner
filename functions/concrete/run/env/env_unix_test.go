//go:build !windows && !integration

package env

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const lookPathChildEnv = "ENV_TEST_LOOKPATH_CHILD"

func TestLookPath_SkipsFileOwnerCannotExecute(t *testing.T) {
	if os.Geteuid() == 0 && os.Getenv(lookPathChildEnv) == "" {
		runAsNobody(t, "^TestLookPath_SkipsFileOwnerCannotExecute$")
		return
	}

	// Only the "other" execute bit is set, so the owner cannot run it.
	otherExec := t.TempDir()
	tool := filepath.Join(otherExec, "tool")
	require.NoError(t, os.WriteFile(tool, nil, 0o601))
	require.NoError(t, os.Chmod(tool, 0o601))

	execDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(execDir, "tool"), nil, 0o755))

	e := &Env{Env: map[string]string{"PATH": otherExec + ":" + execDir}}
	got, err := e.LookPath("tool", nil)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(execDir, "tool"), got)
}

// runAsNobody runs the named test again in a copy of the test binary as uid
// and gid 65534, because root may execute a file that has any execute bit set.
func runAsNobody(t *testing.T, run string) {
	t.Helper()

	// t.TempDir's parent is 0700, so uid 65534 could not reach the binary.
	dir, err := os.MkdirTemp("", "lookpath")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	require.NoError(t, os.Chmod(dir, 0o755))

	bin := filepath.Join(dir, "env.test")
	src, err := os.Open(os.Args[0])
	require.NoError(t, err)
	defer src.Close()
	dst, err := os.OpenFile(bin, os.O_CREATE|os.O_WRONLY, 0o755)
	require.NoError(t, err)
	defer dst.Close()
	_, err = io.Copy(dst, src)
	require.NoError(t, err)
	require.NoError(t, dst.Close())
	// OpenFile applies the umask, and uid 65534 must be able to run the copy.
	require.NoError(t, os.Chmod(bin, 0o755))

	tmp := filepath.Join(dir, "tmp")
	require.NoError(t, os.Mkdir(tmp, 0o700))
	require.NoError(t, os.Chown(tmp, 65534, 65534))

	cmd := exec.Command(bin, "-test.run", run, "-test.count=1", "-test.v")
	cmd.Dir = dir
	cmd.Env = []string{"TMPDIR=" + tmp, "PATH=" + os.Getenv("PATH"), lookPathChildEnv + "=1"}
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 65534, Gid: 65534}}
	out, err := cmd.CombinedOutput()
	if errors.Is(err, syscall.EPERM) {
		t.Skipf("cannot run as uid 65534: %v", err)
	}
	require.NoError(t, err, "test as uid 65534:\n%s", out)
	// The test binary prints PASS even when -test.run matches no test.
	require.Contains(t, string(out), "--- PASS: ")
}
