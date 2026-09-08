//go:build !windows && !integration

package docker

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func useFakeDockerMachine(t *testing.T, script string) {
	executable := path.Join(t.TempDir(), "docker-machine")
	require.NoError(t, os.WriteFile(executable, []byte("#!/bin/sh\n"+script+"\n"), 0o755))
	current := dockerMachineExecutable
	dockerMachineExecutable = executable
	t.Cleanup(func() { dockerMachineExecutable = current })
}

func TestDockerMachineRunsInOwnProcessGroup(t *testing.T) {
	useFakeDockerMachine(t, `sleep 30`)

	ctx, cancel := context.WithCancel(t.Context())
	cmd := newDockerMachineCommand(ctx, "inspect", "x")
	require.NoError(t, cmd.Start())

	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	require.NoError(t, err)
	assert.Equal(t, cmd.Process.Pid, pgid, "docker-machine should lead its own process group")
	assert.NotEqual(t, syscall.Getpgrp(), pgid, "must not share the runner's process group")

	cancel()
	var exitErr *exec.ExitError
	require.ErrorAs(t, cmd.Wait(), &exitErr)
}

func TestDockerMachineCancelKillsProcessGroup(t *testing.T) {
	// The background sleep stands in for the driver plugin. It inherits
	// the stdout pipe, so the read end only sees EOF once it is dead too.
	useFakeDockerMachine(t, `sleep 30 & echo started; wait`)

	r, w, err := os.Pipe()
	require.NoError(t, err)
	defer r.Close()

	ctx, cancel := context.WithCancel(t.Context())
	cmd := newDockerMachineCommand(ctx, "create", "x")
	cmd.Stdout = w
	require.NoError(t, cmd.Start())
	require.NoError(t, w.Close())

	buf := make([]byte, 8)
	_, err = io.ReadFull(r, buf)
	require.NoError(t, err)

	cancel()
	var exitErr *exec.ExitError
	require.ErrorAs(t, cmd.Wait(), &exitErr)

	require.NoError(t, r.SetReadDeadline(time.Now().Add(5*time.Second)))
	_, err = io.ReadAll(r)
	assert.NoError(t, err, "child still holds the pipe; the group kill did not reach it")
}

func TestDockerMachineCancelAfterExitIsNotAnError(t *testing.T) {
	useFakeDockerMachine(t, `exit 0`)

	cmd := newDockerMachineCommand(t.Context(), "inspect", "x")
	require.NoError(t, cmd.Start())
	require.NoError(t, cmd.Wait())

	assert.ErrorIs(t, cmd.Cancel(), os.ErrProcessDone)
}

func TestExist(t *testing.T) {
	tests := map[string]struct {
		script   string
		ctx      func(t *testing.T) context.Context
		expected bool
	}{
		"inspect succeeds":                       {script: "exit 0", expected: true},
		"inspect rejects the store":              {script: "exit 1", expected: false},
		"inspect crashes (SIGQUIT dump exits 2)": {script: "exit 2", expected: true},
		"inspect killed by a signal":             {script: "kill -TERM $$", expected: true},
		"inspect times out": {
			script: "sleep 10",
			ctx: func(t *testing.T) context.Context {
				ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
				t.Cleanup(cancel)
				return ctx
			},
			expected: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Setenv("MACHINE_STORAGE_PATH", t.TempDir())
			require.NoError(t, os.MkdirAll(path.Join(getMachineDir(), "machine-1"), 0o755))
			require.NoError(t, os.WriteFile(path.Join(getMachineDir(), "machine-1", "config.json"), []byte("{}"), 0o600))
			useFakeDockerMachine(t, tc.script)

			ctx := t.Context()
			if tc.ctx != nil {
				ctx = tc.ctx(t)
			}

			m := &machineCommand{}
			assert.Equal(t, tc.expected, m.Exist(ctx, "machine-1"))
		})
	}
}

func TestExistWithoutStoreEntry(t *testing.T) {
	t.Setenv("MACHINE_STORAGE_PATH", t.TempDir())

	m := &machineCommand{}
	assert.False(t, m.Exist(t.Context(), "machine-1"))
}
