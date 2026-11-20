//go:build !integration

package kubernetes

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/jpillora/backoff"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"

	"gitlab.com/gitlab-org/gitlab-runner/shells"
)

type log struct {
	line   string
	offset int64
}

func (l log) String() string {
	if l.offset < 0 {
		return l.line
	}

	return fmt.Sprintf("%d %s", l.offset, l.line)
}

var errBrokenReader = errors.New("broken")

type brokenReader struct {
	err error
}

func newBrokenReader(err error) *brokenReader {
	return &brokenReader{err: err}
}

func (b *brokenReader) Read([]byte) (n int, err error) {
	return 0, b.err
}

func (b *brokenReader) Close() error {
	return nil
}

func TestNewKubernetesLogProcessor(t *testing.T) {
	client := new(kubernetes.Clientset)
	testBackoff := new(backoff.Backoff)
	logger := logrus.New()
	clientConfig := new(restclient.Config)
	p := newKubernetesLogProcessor(client, clientConfig, testBackoff, logger, kubernetesLogProcessorPodConfig{
		namespace: "namespace",
		pod:       "pod",
		container: "container",
		logPath:   "logPath",
	})

	assert.Equal(t, testBackoff, p.backoff)
	assert.Equal(t, logger, p.logger)
	require.NotNil(t, p.logStreamer)

	k, ok := p.logStreamer.(*kubernetesLogStreamer)
	assert.True(t, ok)
	assert.Equal(t, "namespace", k.namespace)
	assert.Equal(t, "pod", k.pod)
	assert.Equal(t, "container", k.container)
	assert.Equal(t, "namespace/pod/container:logPath", p.logStreamer.String())
}

func TestKubernetesLogStreamProviderLogStream(t *testing.T) {
	abortErr := errors.New("abort")

	namespace := "k8s_namespace"
	pod := "k8s_pod_name"
	container := "k8s_container_name"
	logPath := "log_path"

	client := mockKubernetesClientWithHost("", "", nil)
	cfg := new(restclient.Config)
	output := new(bytes.Buffer)
	offset := 15
	waitFileTimeout := time.Minute

	executor := NewMockRemoteExecutor(t)
	urlMatcher := mock.MatchedBy(func(url *url.URL) bool {
		query := url.Query()
		assert.Equal(t, container, query.Get("container"))
		assert.Equal(t, "true", query.Get("stdout"))
		assert.Equal(t, "true", query.Get("stderr"))
		command := query["command"]
		assert.Equal(t, []string{
			"gitlab-runner-helper",
			"read-logs",
			"--path",
			logPath,
			"--offset",
			strconv.Itoa(offset),
			"--wait-file-timeout",
			waitFileTimeout.String(),
		}, command)

		return true
	})
	executor.On("Execute", mock.Anything, http.MethodPost, urlMatcher, cfg, nil, output, output, false).Return(abortErr)

	s := kubernetesLogStreamer{}
	s.client = client
	s.clientConfig = cfg
	s.executor = executor
	s.namespace = namespace
	s.pod = pod
	s.container = container
	s.logPath = logPath
	s.waitLogFileTimeout = waitFileTimeout

	err := s.Stream(t.Context(), int64(offset), output)
	assert.ErrorIs(t, err, abortErr)
}

func TestReadLogsBrokenReader(t *testing.T) {
	proc := new(kubernetesLogProcessor)

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	proc.logger = logger

	output := make(chan string)
	err := proc.readLogs(t.Context(), newBrokenReader(errBrokenReader), output)

	assert.ErrorIs(t, err, errBrokenReader)
}

func TestProcessedOffsetSet(t *testing.T) {
	proc := new(kubernetesLogProcessor)

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	proc.logger = logger

	ch := make(chan string)
	go func() {
		for range ch {
		}
	}()

	logs := logsToReader(
		log{line: "line 1", offset: 10},
		log{line: "line 1", offset: 20},
	)
	err := proc.readLogs(t.Context(), logs, ch)
	assert.NoError(t, err)
	assert.Equal(t, int64(20), proc.logsOffset)
}

// partialLog builds a log line the way the helper's read-logs prints a chunk read
// up to EOF in the middle of a line: the offset only counts the chunk, and a '\n' is appended to it.
func partialLog(chunk string, previousOffset int64) log {
	return log{line: chunk, offset: previousOffset + int64(len(chunk))}
}

func fullLog(line string, previousOffset int64) log {
	return log{line: line, offset: previousOffset + int64(len(line)) + 1}
}

func TestReadLogsPartialLines(t *testing.T) {
	exitStatus := `{"command_exit_code": 0, "script": "/scripts-1-1/restore_cache"}`
	longJSON := `{"command_exit_code": 0, "script": "` + strings.Repeat("a", maxHeldPartialLineSize)

	tests := map[string]struct {
		chunks        func() []log
		expectedLines []string
	}{
		"exit status split in two": {
			chunks: func() []log {
				first := fullLog("line 1", 0)
				part1 := partialLog(exitStatus[:25], first.offset)
				part2 := fullLog(exitStatus[25:], part1.offset)
				return []log{first, part1, part2}
			},
			expectedLines: []string{"line 1\n", exitStatus + "\n"},
		},
		"exit status split in three": {
			chunks: func() []log {
				first := fullLog("line 1", 0)
				part1 := partialLog(exitStatus[:10], first.offset)
				part2 := partialLog(exitStatus[10:40], part1.offset)
				part3 := fullLog(exitStatus[40:], part2.offset)
				return []log{first, part1, part2, part3}
			},
			expectedLines: []string{"line 1\n", exitStatus + "\n"},
		},
		"partial non JSON line is forwarded right away": {
			chunks: func() []log {
				first := fullLog("line 1", 0)
				part1 := partialLog("progress: 10%", first.offset)
				part2 := fullLog(" 100%", part1.offset)
				return []log{first, part1, part2}
			},
			expectedLines: []string{"line 1\n", "progress: 10%", " 100%\n"},
		},
		"line without offset does not break a held exit status": {
			chunks: func() []log {
				first := fullLog("line 1", 0)
				part1 := partialLog(exitStatus[:10], first.offset)
				part2 := partialLog(exitStatus[10:40], part1.offset)
				part3 := fullLog(exitStatus[40:], part2.offset)
				return []log{first, part1, {line: "out of band", offset: -1}, part2, part3}
			},
			expectedLines: []string{"line 1\n", "out of band\n", exitStatus + "\n"},
		},
		"partial JSON line other than the exit status is forwarded right away": {
			chunks: func() []log {
				first := fullLog("line 1", 0)
				part1 := partialLog(`{"level":"info",`, first.offset)
				part2 := fullLog(`"msg":"done"}`, part1.offset)
				return []log{first, part1, part2}
			},
			expectedLines: []string{"line 1\n", `{"level":"info",`, `"msg":"done"}` + "\n"},
		},
		"partial JSON line over the size limit is forwarded right away": {
			chunks: func() []log {
				first := fullLog("line 1", 0)
				part1 := partialLog(longJSON, first.offset)
				part2 := fullLog("}", part1.offset)
				return []log{first, part1, part2}
			},
			expectedLines: []string{"line 1\n", longJSON, "}\n"},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			proc := new(kubernetesLogProcessor)
			proc.logger = logrus.New()

			ch := make(chan string)
			var received []string
			done := make(chan struct{})
			go func() {
				defer close(done)
				for line := range ch {
					received = append(received, line)
				}
			}()

			err := proc.readLogs(t.Context(), logsToReader(tt.chunks()...), ch)
			close(ch)
			<-done

			require.NoError(t, err)
			assert.Equal(t, tt.expectedLines, received)
		})
	}
}

func TestReadLogsSplitExitStatusIsDetected(t *testing.T) {
	exitStatus := `{"command_exit_code": 1, "script": "/scripts-1-1/step_script"}`

	proc := new(kubernetesLogProcessor)
	proc.logger = logrus.New()

	first := fullLog("line 1", 0)
	part1 := partialLog(exitStatus[:25], first.offset)
	part2 := fullLog(exitStatus[25:], part1.offset)

	ch := make(chan string, 3)
	require.NoError(t, proc.readLogs(t.Context(), logsToReader(first, part1, part2), ch))
	close(ch)

	var statuses []shells.StageCommandStatus
	for line := range ch {
		var status shells.StageCommandStatus
		if status.TryUnmarshal(line) {
			statuses = append(statuses, status)
		}
	}

	require.Len(t, statuses, 1)
	require.True(t, statuses[0].IsExited())
	assert.Equal(t, 1, *statuses[0].CommandExitCode)
}

func TestReadLogsPartialLineHeldAcrossReattach(t *testing.T) {
	exitStatus := `{"command_exit_code": 0, "script": "/scripts-1-1/restore_cache"}`

	proc := new(kubernetesLogProcessor)
	proc.logger = logrus.New()

	readLogs := func(logs ...log) []string {
		ch := make(chan string, len(logs))
		require.NoError(t, proc.readLogs(t.Context(), logsToReader(logs...), ch))
		close(ch)

		var lines []string
		for line := range ch {
			lines = append(lines, line)
		}
		return lines
	}

	first := fullLog("line 1", 0)
	part1 := partialLog(exitStatus[:10], first.offset)
	received := readLogs(first, part1)

	part2 := partialLog(exitStatus[10:40], proc.logsOffset)
	part3 := fullLog(exitStatus[40:], part2.offset)
	received = append(received, readLogs(part2, part3)...)

	assert.Equal(t, []string{"line 1\n", exitStatus + "\n"}, received)
}

func logsToReader(logs ...log) io.Reader {
	b := new(bytes.Buffer)
	for _, l := range logs {
		b.WriteString(l.String() + "\n")
	}

	return b
}

func TestParseLogs(t *testing.T) {
	tests := map[string]struct {
		line string

		expectedOffset int64
		expectedText   string
	}{
		"with offset": {
			line: "20 line",

			expectedOffset: 20,
			expectedText:   "line",
		},
		"with no offset": {
			line: "line",

			expectedOffset: -1,
			expectedText:   "line",
		},
		"starts with space": {
			line: " 20 line",

			expectedOffset: -1,
			expectedText:   " 20 line",
		},
		"multiple spaces after offset": {
			line: "20   line",

			expectedOffset: 20,
			expectedText:   "  line",
		},
		"empty log": {
			line: "",

			expectedOffset: -1,
			expectedText:   "",
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			p := new(kubernetesLogProcessor)

			offset, line := p.parseLogLine(tt.line)
			assert.Equal(t, tt.expectedOffset, offset)
			assert.Equal(t, tt.expectedText, line)
		})
	}
}

func TestListenReadLines(t *testing.T) {
	line1 := "line 1"
	line2 := "line 2"
	expectedLines := []string{line1 + "\n", line2 + "\n"}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	mockLogStreamer := makeMockLogStreamer(t)

	logs := []log{
		{line: line1, offset: 10},
		{line: line2, offset: 20},
	}

	var wg sync.WaitGroup
	wg.Add(len(logs))

	mockLogStreamer.On("Stream", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			writeLogs(
				args.Get(2).(io.Writer),
				logs...,
			)

			// after writing the logs, this method must wait for them to be send out through the channel
			// otherwise it will exit early and cancel the inner context responsible for receiving/sending
			wg.Wait()
			cancel()
		}).
		Return(nil).
		Once()
	mockLogStreamer.On("Stream", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			t.Log(args)
			assert.FailNow(t, "unexpected call to Stream()")
		}).
		Return(nil).
		Maybe()

	processor := newTestKubernetesLogProcessor(t)
	processor.logStreamer = mockLogStreamer

	ch, errCh := processor.Process(ctx)
	receivedLogs := make([]string, 0)
	for log := range ch {
		wg.Done()
		receivedLogs = append(receivedLogs, log)
	}

	assert.Equal(t, expectedLines, receivedLogs)
	drainProcessLogsChannels(ch, errCh)
	processor.Finalize()
}

func makeMockLogStreamer(t *testing.T) *mockLogStreamer {
	s := newMockLogStreamer(t)
	s.On("String").Return("mockLogStreamer").Maybe()

	return s
}

func writeLogs(to io.Writer, logs ...log) {
	for _, l := range logs {
		_, _ = to.Write([]byte(l.String() + "\n"))
	}
}

func newTestKubernetesLogProcessor(t *testing.T) *kubernetesLogProcessor {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	c := newMockBackoffCalculator(t)
	c.On("ForAttempt", mock.Anything).Return(50 * time.Millisecond).Maybe()

	return &kubernetesLogProcessor{
		logger:  logger,
		backoff: c,
	}
}

func TestListenCancelContext(t *testing.T) {
	mockLogStreamer := makeMockLogStreamer(t)

	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()

	mockLogStreamer.On("Stream", mock.Anything, mock.Anything, mock.Anything).
		Run(func(mock.Arguments) {
			<-ctx.Done()
		}).
		Return(io.EOF)
	mockLogStreamer.On("Stream", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			t.Log(args)
			assert.FailNow(t, "unexpected call to Stream()")
		}).
		Return(nil).
		Maybe()

	processor := newTestKubernetesLogProcessor(t)
	processor.logStreamer = mockLogStreamer

	ch, errCh := processor.Process(ctx)
	assert.NoError(t, drainProcessLogsChannels(ch, errCh), "No error should be returned!")
	processor.Finalize()
}

func TestAttachReconnectLogStream(t *testing.T) {
	const expectedConnectCount = 5
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	mockLogStreamer := makeMockLogStreamer(t)

	var connects int
	mockLogStreamer.
		On("Stream", mock.Anything, mock.Anything, mock.Anything).
		Run(func(mock.Arguments) {
			connects++
			if connects == expectedConnectCount {
				cancel()
			}
		}).
		Return(io.EOF).
		Times(expectedConnectCount)
	mockLogStreamer.On("Stream", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			t.Log(args)
			assert.FailNow(t, "unexpected call to Stream()")
		}).
		Return(nil).
		Maybe()

	processor := newTestKubernetesLogProcessor(t)
	processor.logStreamer = mockLogStreamer

	ch, errCh := processor.Process(ctx)
	_ = drainProcessLogsChannels(ch, errCh)
	processor.Finalize()
}

func TestAttachReconnectReadLogs(t *testing.T) {
	const expectedConnectCount = 5
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	mockLogStreamer := makeMockLogStreamer(t)

	var connects int
	mockLogStreamer.
		On("Stream", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			_ = args.Get(2).(*io.PipeWriter).Close()

			connects++
			if connects == expectedConnectCount {
				cancel()
			}
		}).
		Return(nil).
		Times(expectedConnectCount)
	mockLogStreamer.On("Stream", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			t.Log(args)
			assert.FailNow(t, "unexpected call to Stream()")
		}).
		Return(nil).
		Maybe()

	processor := newTestKubernetesLogProcessor(t)
	processor.logStreamer = mockLogStreamer

	ch, errCh := processor.Process(ctx)
	assert.NoError(t, drainProcessLogsChannels(ch, errCh), "No error should be returned!")
	processor.Finalize()
}

func drainProcessLogsChannels(ch <-chan string, errCh <-chan error) error {
	var firstErr error
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return firstErr
			}
		case err, ok := <-errCh:
			if !ok {
				continue
			}
			if firstErr == nil {
				firstErr = err
			}
		}
	}
}

func TestAttachCorrectOffset(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	mockLogStreamer := makeMockLogStreamer(t)

	logs := []log{
		{line: "line", offset: 10},
		{line: "line", offset: 20},
	}

	var wg sync.WaitGroup
	wg.Add(len(logs))

	mockLogStreamer.
		On("Stream", mock.Anything, int64(0), mock.Anything).
		Run(func(args mock.Arguments) {
			writeLogs(
				args.Get(2).(io.Writer),
				logs...,
			)

			// after writing the logs, this method must wait for them to be send out through the channel
			// otherwise it will exit early and cancel the inner context responsible for receiving/sending
			wg.Wait()
		}).
		Return(nil).
		Once()

	mockLogStreamer.
		On("Stream", mock.Anything, int64(20), mock.Anything).
		Run(func(mock.Arguments) {
			cancel()
		}).
		Return(errBrokenReader).
		Once()

	mockLogStreamer.On("Stream", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			t.Log(args)
			assert.FailNow(t, "unexpected call to Stream()")
		}).
		Return(nil).
		Maybe()

	processor := newTestKubernetesLogProcessor(t)
	processor.logStreamer = mockLogStreamer

	ch, errCh := processor.Process(ctx)
	for range ch {
		wg.Done()
	}

	drainProcessLogsChannels(ch, errCh)
	processor.Finalize()
}

func TestScanHandlesStreamError(t *testing.T) {
	closedErr := errors.New("closed")
	processor := new(kubernetesLogProcessor)

	tests := map[string]struct {
		readerError   error
		expectedError error
	}{
		"reader EOF": {
			readerError: io.EOF,
			// EOF is handled specially. Since it means that the stream
			// reached its end, a nil is returned by scanner.Err()
			expectedError: nil,
		},
		"custom error": {
			readerError:   closedErr,
			expectedError: closedErr,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			scanner, ch := processor.scan(ctx, newBrokenReader(tt.readerError))
			line, more := <-ch
			assert.Empty(t, line)
			assert.False(t, more)
			assert.ErrorIs(t, scanner.Err(), tt.expectedError)
		})
	}
}

func TestScanHandlesCancelledContext(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		processor := new(kubernetesLogProcessor)

		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		scanner, ch := processor.scan(ctx, logsToReader(log{}))

		// Block the channel, so there's no consumers
		time.Sleep(time.Second)

		// Assert that the channel is closed
		line, more := <-ch
		assert.Empty(t, line)
		assert.False(t, more)

		// Assert that the scanner had no error
		assert.Nil(t, scanner.Err())
	})
}
