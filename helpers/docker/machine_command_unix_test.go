//go:build !windows && !integration

package docker

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"sync"
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

func TestCreateOutputHandling(t *testing.T) {
	useFakeDockerMachine(t, `echo "$@" > "$FAKE_ARGS_FILE"
echo '{"level":"info","msg":"Creating machine...","phase":"driver_create"}'
echo 'not json'`)

	tests := map[string]struct {
		json         bool
		wantArgs     string
		wantFields   map[string]any
		wantMessages []string
	}{
		"text": {
			wantArgs:     "--bugsnag-api-token=no-report create --driver google --opt=1 name",
			wantMessages: []string{`{"level":"info","msg":"Creating machine...","phase":"driver_create"}`, "not json"},
		},
		"json": {
			json:         true,
			wantArgs:     "--bugsnag-api-token=no-report --log-format=json create --driver google --opt=1 name",
			wantFields:   map[string]any{"phase": "driver_create"},
			wantMessages: []string{"Creating machine...", "not json"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			argsFile := path.Join(t.TempDir(), "args")
			t.Setenv("FAKE_ARGS_FILE", argsFile)
			t.Setenv("MACHINE_BUGSNAG_API_TOKEN", "")

			var entries []LogEntry
			var mu sync.Mutex
			output := MachineOutput{JSONLogFormat: tc.json, OnEntry: func(e LogEntry) {
				mu.Lock()
				defer mu.Unlock()
				entries = append(entries, e)
			}}

			m := NewMachineCommand()
			require.NoError(t, m.Create(t.Context(), "google", "name", output, "opt=1"))

			args, err := os.ReadFile(argsFile)
			require.NoError(t, err)
			assert.Equal(t, tc.wantArgs, strings.TrimSpace(string(args)))

			require.Len(t, entries, 2)
			assert.Equal(t, tc.wantMessages[0], entries[0].Msg)
			assert.Equal(t, tc.wantFields, entries[0].Fields)
			assert.Equal(t, tc.wantMessages[1], entries[1].Msg)
			assert.Nil(t, entries[1].Fields)
		})
	}
}

func TestCredentials(t *testing.T) {
	fake := func(configExit int, url, authOptions string) string {
		return `case "$2" in
  config) exit ` + strconv.Itoa(configExit) + ` ;;
  url) echo "` + url + `" ;;
  inspect) echo '` + authOptions + `' ;;
esac`
	}

	tests := map[string]struct {
		script   string
		expected Credentials
		errMsg   string
	}{
		"machine with a certificate for its name": {
			script: fake(0, "tcp://10.0.0.7:2376", `{"StorePath":"/m/machines/m1","ServerName":"m1"}`),
			expected: Credentials{
				Host:          "tcp://10.0.0.7:2376",
				CertPath:      "/m/machines/m1",
				TLSVerify:     true,
				TLSServerName: "m1",
			},
		},
		"machine provisioned over SSH": {
			script: fake(0, "tcp://10.0.0.7:2376", `{"StorePath":"/m/machines/m1"}`),
			expected: Credentials{
				Host:      "tcp://10.0.0.7:2376",
				CertPath:  "/m/machines/m1",
				TLSVerify: true,
			},
		},
		"cannot connect": {
			script: fake(1, "tcp://10.0.0.7:2376", `{"StorePath":"/m/machines/m1"}`),
			errMsg: "can't connect",
		},
		"no url": {
			script: fake(0, "", `{"StorePath":"/m/machines/m1"}`),
			errMsg: "failed to get [url m1]",
		},
		"auth options without a store path": {
			script: fake(0, "tcp://10.0.0.7:2376", `{"ServerName":"m1"}`),
			errMsg: "no StorePath",
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			useFakeDockerMachine(t, tc.script)

			dc, err := NewMachineCommand().Credentials(t.Context(), "m1")
			if tc.errMsg != "" {
				require.ErrorContains(t, err, tc.errMsg)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expected, dc)
		})
	}
}
