//go:build !integration

package steps_test

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
	"gitlab.com/gitlab-org/gitlab-runner/commands/steps"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/step-runner/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	waitDeadline = 5 * time.Second
	waitTick     = 100 * time.Millisecond

	externalMode = "external-mode"
	appMode      = "app-mode"

	dontSleep       = "0"
	sleepSomeTime   = "2"
	sleepReallyLong = "300"
)

func TestMain(m *testing.M) {
	if len(os.Args) > 1 {
		cmds := map[string]func(...string) int{
			externalMode: beExternalBinary,
			appMode:      beCliApp,
		}
		mode := os.Args[1]
		if cmd, ok := cmds[mode]; ok {
			mainTmpDir := os.Getenv("_MAIN_TMP_DIR")
			fakeCoverDir, err := os.MkdirTemp(mainTmpDir, mode)
			if err != nil {
				panic("creating fake cover dir: " + err.Error())
			}
			os.Setenv("GOCOVERDIR", fakeCoverDir)
			args := slices.Clone(os.Args[2:])
			os.Exit(cmd(args...))
		}
	}

	mainTmpDir, err := os.MkdirTemp("", "")
	if err != nil {
		panic("creating main temp dir: " + err.Error())
	}
	os.Setenv("_MAIN_TMP_DIR", mainTmpDir)

	rc := m.Run()

	err = os.RemoveAll(mainTmpDir)
	if err != nil {
		panic("deleting main temp dir: " + err.Error())
	}

	os.Exit(rc)
}

func TestServe(t *testing.T) {
	t.Parallel()

	toTerminateWith := func(s string) *string { return &s }

	tests := []struct {
		name              string
		sockName          string
		cmdAndArgs        []string
		stdin             string
		explicitCancel    bool
		expectedStdout    string
		expectedStderr    string
		expectListening   bool
		expectTermination *string
	}{
		{
			name:            "valid socket name",
			sockName:        "some.sock",
			expectListening: true,
		},
		{
			name:              "invalid socket name",
			sockName:          filepath.Join("subdir", "not-existent", "fails.sock"),
			expectTermination: toTerminateWith("opening socket: listen unix .*: " + socketErrs.Get(t, "listenInvalidSocket")),
		},
		{
			name:              "with a successful command",
			sockName:          "some.sock",
			cmdAndArgs:        []string{os.Args[0], externalMode, dontSleep, "foo", "bar", "0"},
			stdin:             "some stdin",
			expectedStdout:    "stdin: some stdin\nstdout: foo\n",
			expectedStderr:    "stderr: bar\n",
			expectListening:   true,
			expectTermination: toTerminateWith(""),
		},
		{
			name:              "with a failing command",
			sockName:          "some.sock",
			cmdAndArgs:        []string{os.Args[0], externalMode, dontSleep, "foo", "bar", "42"},
			stdin:             "some stdin",
			expectedStdout:    "stdin: some stdin\nstdout: foo\n",
			expectedStderr:    "stderr: bar\n",
			expectTermination: toTerminateWith("command error: exit status 42"),
		},
		{
			name:              "with a successful longer-running command",
			sockName:          "some.sock",
			cmdAndArgs:        []string{os.Args[0], externalMode, sleepSomeTime, "foo", "bar", "0"},
			stdin:             "some stdin",
			expectedStdout:    "stdin: some stdin\nstdout: foo\n",
			expectedStderr:    "stderr: bar\n",
			expectListening:   true,
			expectTermination: toTerminateWith(""),
		},
		{
			name:              "with a failing longer-running command",
			sockName:          "some.sock",
			cmdAndArgs:        []string{os.Args[0], externalMode, sleepSomeTime, "foo", "bar", "43"},
			stdin:             "some stdin",
			expectedStdout:    "stdin: some stdin\nstdout: foo\n",
			expectedStderr:    "stderr: bar\n",
			expectTermination: toTerminateWith("command error: exit status 43"),
			expectListening:   true,
		},
		{
			name:              "with context being canceled from the outside",
			sockName:          "some.sock",
			cmdAndArgs:        []string{os.Args[0], externalMode, sleepReallyLong, "", "", "42"},
			explicitCancel:    true,
			expectListening:   true,
			expectTermination: toTerminateWith("context canceled"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sockPath := filepath.Join(shortTempDir(t), tc.sockName)
			ctx, shutDown := context.WithCancel(t.Context())
			t.Cleanup(shutDown)

			ioStreams, stdin, stdout, stderr := testIOStreams()

			serveErr := make(chan error)
			go func() {
				serveErr <- steps.Serve(ctx, sockPath, ioStreams, tc.cmdAndArgs...)
			}()

			t.Cleanup(func() {
				assert.EventuallyWithT(t, func(c *assert.CollectT) {
					assert.NoFileExists(c, sockPath)
				}, waitDeadline, waitTick, "listening socket not cleaned up")
			})

			if tc.expectListening {
				assert.EventuallyWithT(t, func(c *assert.CollectT) {
					assert.FileExists(c, sockPath)
				}, waitDeadline, waitTick, "no listening socket found")

				client := stepsClient(t, sockPath)
				status, err := client.Status(t.Context(), &proto.StatusRequest{})
				assert.NoError(t, err, "getting steps runner status")
				assert.Len(t, status.Jobs, 0, "job count")
			}

			if tc.stdin != "" {
				_, err := stdin.Write([]byte(tc.stdin))
				require.NoError(t, err, "writing to stdin pipe to external binary")
			}
			require.NoError(t, stdin.Close(), "closing stdin pipe to external binary")

			if eo := tc.expectedStdout; eo != "" {
				assert.EventuallyWithT(t, func(c *assert.CollectT) {
					assert.Equal(c, eo, stdout.String())
				}, waitDeadline, waitTick, "stdout")
			}

			if ee := tc.expectedStderr; ee != "" {
				assert.EventuallyWithT(t, func(c *assert.CollectT) {
					assert.Equal(c, ee, stderr.String())
				}, waitDeadline, waitTick, "stderr")
			}

			if tc.explicitCancel {
				shutDown()
			}

			if re := tc.expectTermination; re != nil {
				err := <-serveErr
				if *re == "" {
					assert.NoError(t, err, "expected no error")
				} else {
					assert.Regexp(t, *re, err.Error(), "expected serve error")
				}
				return
			}
		})
	}
}

func TestProxy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		skipOnOS        []string
		sockPath        string
		toSend          []string
		close           bool
		closeErr        error
		expectToReceive string
		expectShutdown  bool
		expectedErr     string
	}{
		{
			name:            "proxies",
			toSend:          []string{"hello", "there"},
			expectToReceive: "hello\nthere\n",
		},
		{
			// On windows the proxy does not shut down when the output writer is closed, it does not close the Proxy.
			skipOnOS: []string{"windows"},

			name:            "stops proxying when input is closed",
			toSend:          []string{"hello", "there"},
			close:           true,
			expectToReceive: "hello\nthere\n",
			expectShutdown:  true,
		},
		{
			// On windows the proxy does not shut down when the output writer is closed, it does not close the Proxy.
			skipOnOS: []string{"windows"},

			name:            "stops proxying when input is closed with error",
			toSend:          []string{"hello", "there"},
			close:           true,
			closeErr:        fmt.Errorf("oh no something went south"),
			expectToReceive: "hello\nthere\n",
			expectShutdown:  true,
			expectedErr:     "oh no something went south",
		},
		{
			name:           "does not proxy when socket is invalid",
			sockPath:       filepath.Join("does", "not", "exist.sock"),
			expectShutdown: true,
			expectedErr:    socketErrs.Get(t, "dialInvalidSocket"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if slices.Contains(tc.skipOnOS, runtime.GOOS) {
				t.Skipf("not supported on any of %q", tc.skipOnOS)
			}

			sockPath := cmp.Or(tc.sockPath, echoServer(t))

			ioStreams, outWriter, in, _ := testIOStreams()
			var proxyHasShutDown atomic.Bool

			go func() {
				err := steps.Proxy(sockPath, ioStreams)
				proxyHasShutDown.Store(true)
				if ee := tc.expectedErr; ee != "" {
					assert.ErrorContains(t, err, ee)
				} else {
					assert.NoError(t, err, "proxy error")
				}
			}()

			for _, msg := range tc.toSend {
				_, err := fmt.Fprintln(outWriter, msg)
				assert.NoError(t, err, "writing data")
			}

			assert.EventuallyWithT(t, func(c *assert.CollectT) {
				assert.Equal(c, tc.expectToReceive, in.String())
			}, waitDeadline, waitTick, "data received from proxy is not as expected")

			if tc.close {
				outWriter.CloseWithError(tc.closeErr)
			}

			assert.EventuallyWithT(t, func(c *assert.CollectT) {
				assert.Equal(c, tc.expectShutdown, proxyHasShutDown.Load())
			}, waitDeadline, waitTick, "proxy running state not as expected")
		})
	}
}

func TestCli(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		args             []string
		expectedStdoutRE string
	}{
		{
			name:             "steps command is hidden",
			args:             []string{"--help"},
			expectedStdoutRE: `\nCOMMANDS:\n[ ]+help[^\n]+\n\nGLOBAL OPTIONS:\n`,
		},
		{
			name:             "steps subcommands are visible",
			args:             []string{"steps", "--help"},
			expectedStdoutRE: `\nCOMMANDS:\n[ ]+serve[^\n]+\n[ ]+proxy[^\n]+\n\nOPTIONS:\n`,
		},
		{
			name:             "uses and shows the correct default socket path for serve",
			args:             []string{"steps", "serve", "--help"},
			expectedStdoutRE: `\n[ ]+--socket value[ ]+\(default: "[^"]+/step-runner.sock"\)\n`,
		},
		{
			name:             "uses and shows the correct default socket path for proxy",
			args:             []string{"steps", "proxy", "--help"},
			expectedStdoutRE: `\n[ ]+--socket value[ ]+\(default: "[^"]+/step-runner.sock"\)\n`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			stdout := &bytes.Buffer{}

			args := []string{appMode}
			args = append(args, tc.args...)

			cli := exec.Command(os.Args[0], args...)
			cli.Stdout = stdout

			err := cli.Run()
			assert.NoError(t, err, "error running CLI")

			if re := tc.expectedStdoutRE; re == "" {
				assert.Empty(t, stdout.String(), "stdout should be empty")
			} else {
				assert.Regexp(t, re, stdout.String(), "stdout not as expected")
			}
		})
	}
}

// beCliApp runs the test binary mimicking a CLI app with the steps command set up.
// With that, we can check on certain aspects of how commands are registered.
func beCliApp(args ...string) int {
	app := cli.NewApp()
	app.Commands = common.GetCommands()
	app.CommandNotFound = func(ctx *cli.Context, s string) {
		fmt.Fprintf(os.Stderr, "command not found: %s", s)
		os.Exit(-2)
	}

	runArgs := []string{"fakeArgv0"}
	runArgs = append(runArgs, args...)

	if err := app.Run(runArgs); err != nil {
		return -1
	}

	return 0
}

// beExternalBinary runs the test binary mimicking an external binary.
// It expects the following args:
//   - sleepTime (mandatory) - how long to sleep before doing anything
//   - stdout (optional) - the data to print to stdout
//   - stderr (optional) - the data to print to stderr
//   - exitCode (optional) - the code to exit with
//
// The first thing it does is to read from stdin, until that stream is closed, and only then continues. It also prints
// the data it received from stdin on stdout.
func beExternalBinary(args ...string) int {
	stdin, err := io.ReadAll(os.Stdin)
	if err != nil {
		panic("reading stdin: " + err.Error())
	}

	fmt.Fprintln(os.Stdout, "stdin: "+string(stdin))

	sleepTime, err := strconv.Atoi(args[0])
	if err != nil {
		panic("parsing sleep: " + err.Error())
	}

	time.Sleep(time.Duration(sleepTime) * time.Second)

	rc := 0
	l := len(args)
	switch {
	case l >= 4:
		var err error
		rc, err = strconv.Atoi(args[3])
		if err != nil {
			panic("parsing return code: " + err.Error())
		}
		fallthrough
	case l >= 3:
		fmt.Fprintln(os.Stderr, "stderr: "+args[2])
		fallthrough
	case l >= 2:
		fmt.Fprintln(os.Stdout, "stdout: "+args[1])
	}

	return rc
}

func testIOStreams() (steps.IOStreams, *io.PipeWriter, *syncBuffer, *syncBuffer) {
	stdinReader, stdinWriter := io.Pipe()
	stdout, stderr := &syncBuffer{}, &syncBuffer{}

	return steps.IOStreams{
		Stdin:  stdinReader,
		Stdout: stdout,
		Stderr: stderr,
	}, stdinWriter, stdout, stderr
}

// osErrs abstracts away different errors on different OSs
type osErrs map[string]map[string]string

func (oe osErrs) Get(t *testing.T, symbolicName string) string {
	errs, ok := oe[symbolicName]
	require.True(t, ok, "no errors for %q", symbolicName)

	os := runtime.GOOS

	if e, ok := errs[os]; ok {
		return e
	}
	if e, ok := errs[""]; ok {
		return e
	}

	require.FailNow(t, "no %q error for %s", symbolicName, os)
	return ""
}

var socketErrs = osErrs{
	"listenInvalidSocket": {
		"windows": "bind: A socket operation encountered a dead network.",
		"":        "bind: no such file or directory",
	},
	"dialInvalidSocket": {
		"windows": "connect: A socket operation encountered a dead network.",
		"":        "connect: no such file or directory",
	},
}

// shortTempDir is a stand-in for t.TempDir, which aims to produce shorter path names.
// Unix sockets on Windows have a max path len of 108 chars, so we need to be stingy.
func shortTempDir(t *testing.T) string {
	dir, err := os.MkdirTemp("", "glr-sr-*")
	require.NoError(t, err, "creating temp dir")
	t.Cleanup(func() {
		err := os.RemoveAll(dir)
		require.NoError(t, err, "deleting temp dir")
	})
	return dir
}

func stepsClient(t *testing.T, sockPath string) proto.StepRunnerClient {
	cliConn, err := grpc.NewClient("unix:"+sockPath, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	return proto.NewStepRunnerClient(cliConn)
}

func echoServer(t *testing.T) string {
	t.Helper()

	sockPath := filepath.Join(shortTempDir(t), "test.sock")

	l, err := net.Listen("unix", sockPath)
	require.NoError(t, err, "creating listener")
	t.Cleanup(func() {
		require.NoError(t, l.Close(), "closing listener")
	})

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				continue
			}

			go func(conn net.Conn) {
				defer conn.Close()
				_, err = io.Copy(conn, conn)
				assert.NoError(t, err, "echoing data")
			}(conn)
		}
	}()

	return sockPath
}

type syncBuffer struct {
	sync.Mutex
	buf bytes.Buffer
}

func (sb *syncBuffer) Write(p []byte) (int, error) {
	sb.Lock()
	defer sb.Unlock()
	return sb.buf.Write(p)
}

var _ io.Writer = &syncBuffer{}

func (sb *syncBuffer) String() string {
	sb.Lock()
	defer sb.Unlock()
	return sb.buf.String()
}
