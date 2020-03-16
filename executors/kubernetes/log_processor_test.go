package kubernetes

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/jpillora/backoff"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest/fake"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

const testDate1 = "2020-01-30T16:28:25.479904159Z"
const testDate2 = "2021-01-30T16:28:25.479904159Z"

type log struct {
	line      string
	timestamp *time.Time
}

func (t log) String() string {
	if t.timestamp == nil {
		return t.line
	}

	return fmt.Sprintf("%s %s", t.timestamp.Format(time.RFC3339Nano), t.line)
}

func mustParseTimestamp(t *testing.T, s string) *time.Time {
	parsedTime, err := time.Parse(time.RFC3339, s)
	require.NoError(t, err)
	return &parsedTime
}

func logsToReadCloser(logs ...log) io.ReadCloser {
	b := &bytes.Buffer{}
	for _, l := range logs {
		b.WriteString(l.String() + "\n")
	}

	return ioutil.NopCloser(b)
}

type brokenReaderError struct{}

func (e *brokenReaderError) Error() string {
	return "broken"
}

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
	client := &kubernetes.Clientset{}
	testBackoff := backoff.Backoff{}
	logger := &common.BuildLogger{}
	p := newKubernetesLogProcessor(client, testBackoff, logger, kubernetesLogProcessorPodConfig{
		namespace:  "namespace",
		pod:        "pod",
		containers: []string{"container"},
	}).(*kubernetesLogProcessor)

	assert.Equal(t, testBackoff, p.backoff)
	assert.Equal(t, logger, p.logger)
	require.Len(t, p.logProviders, 1)

	k, ok := p.logProviders[0].(*kubernetesLogStreamProvider)
	assert.True(t, ok)
	assert.Equal(t, "namespace", k.namespace)
	assert.Equal(t, "pod", k.pod)
	assert.Equal(t, "container", k.container)
	assert.Equal(t, "namespace/pod/container", p.logProviders[0].String())
}

func TestKubernetesLogStreamProviderLogStream(t *testing.T) {
	abortErr := errors.New("abort")

	namespace := "k8s_namespace"
	pod := "k8s_pod_name"
	container := "k8s_container_name"

	tests := map[string]struct {
		sinceTime *time.Time
	}{
		"existing since time": {
			sinceTime: mustParseTimestamp(t, testDate1),
		},
		"non-existing since time": {
			sinceTime: nil,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			client := mockKubernetesClientWithHost("", "", fake.CreateHTTPClient(func(request *http.Request) (*http.Response, error) {
				assert.Equal(t, fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/log", namespace, pod), request.URL.Path)
				query := request.URL.Query()
				assert.Equal(t, container, query.Get("container"))

				follow, err := strconv.ParseBool(query.Get("follow"))
				assert.NoError(t, err)
				assert.True(t, follow)

				timestamps, err := strconv.ParseBool(query.Get("timestamps"))
				assert.NoError(t, err)
				assert.True(t, timestamps)

				sinceTimeQuery := query.Get("sinceTime")
				if tt.sinceTime != nil {
					sinceTime, err := time.Parse(time.RFC3339, sinceTimeQuery)
					require.NoError(t, err)
					assert.True(t, tt.sinceTime.Round(time.Second).Equal(sinceTime))
				} else {
					assert.Equal(t, "", sinceTimeQuery)
				}

				return nil, abortErr
			}))

			p := kubernetesLogStreamProvider{
				client:    client,
				namespace: namespace,
				pod:       pod,
				container: container,
			}

			_, err := p.LogStream(tt.sinceTime)
			assert.True(t, errors.Is(err, abortErr))
		})
	}
}

func TestReadLogsBrokenReader(t *testing.T) {
	proc := kubernetesLogProcessor{}
	expectedTime := time.Time{}.Add(1 * time.Hour)
	receivedTime, err := proc.readLogs(context.Background(), newBrokenReader(&brokenReaderError{}), timestampsSet{}, expectedTime, nil)

	assert.True(t, errors.Is(err, new(brokenReaderError)))
	assert.Equal(t, expectedTime, receivedTime)
}

func TestProcessedTimestampsPopulated(t *testing.T) {
	proc := kubernetesLogProcessor{}
	ts := mustParseTimestamp(t, testDate1)
	logs := logsToReadCloser(
		log{line: "line 1", timestamp: ts},
		log{line: "line 1", timestamp: ts},
	)
	timestamps := timestampsSet{}

	ch := make(chan string)
	defer close(ch)

	go func() {
		for range ch {
		}
	}()

	_, err := proc.readLogs(context.Background(), logs, timestamps, time.Time{}, ch)

	assert.NoError(t, err)

	_, ok := timestamps[ts.UnixNano()]
	assert.True(t, ok)
	assert.Len(t, timestamps, 1)
}

func TestParseLogs(t *testing.T) {
	tests := map[string]struct {
		log         log
		verifyLogFn func(t *testing.T, parsedLog log, testLog log)

		assertErrorFn func(t *testing.T, err error)
	}{
		"parse log with date correct": {
			log: log{
				line:      "log line 1",
				timestamp: mustParseTimestamp(t, testDate1),
			},
			verifyLogFn: func(t *testing.T, parsedLog log, testLog log) {
				assert.Equal(t, testLog.line, parsedLog.line)
				assert.Equal(t, *testLog.timestamp, *parsedLog.timestamp)
			},
		},
		"invalid log with no date space": {
			log: log{line: testDate1},
			verifyLogFn: func(t *testing.T, parsedLog log, testLog log) {
				assert.Equal(t, testDate1, parsedLog.line)
				assert.Equal(t, *mustParseTimestamp(t, testDate1), *parsedLog.timestamp)
			},
		},
		"parse log with date not in RFC3339": {
			log: log{
				line: "2019-03-12 log",
			},
			assertErrorFn: func(t *testing.T, err error) {
				var parseError *time.ParseError
				assert.True(t, errors.As(err, &parseError))
			},
		},
		"invalid log with invalid date and no space": {
			log: log{
				line: "invalid",
			},
			assertErrorFn: func(t *testing.T, err error) {
				var parseError *time.ParseError
				assert.True(t, errors.As(err, &parseError))
			},
		},
		"invalid log empty": {
			log: log{
				line: "",
			},
			assertErrorFn: func(t *testing.T, err error) {
				assert.True(t, errors.Is(err, io.EOF))
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			p := kubernetesLogProcessor{}

			timestamp, line, err := p.parseLogLine(tt.log.String())
			if tt.assertErrorFn != nil {
				tt.assertErrorFn(t, err)
				return
			}

			assert.NoError(t, err)
			tt.verifyLogFn(t, log{
				line:      line,
				timestamp: &timestamp,
			}, tt.log)
		})
	}
}

func TestListenReadLines(t *testing.T) {
	expectedLines := []string{"line 1", "line 2"}

	stream := logsToReadCloser(
		log{line: expectedLines[0], timestamp: mustParseTimestamp(t, testDate1)},
		log{line: expectedLines[1], timestamp: mustParseTimestamp(t, testDate2)},
	)

	mockStreamProvider := &mockLogStreamProvider{}
	defer mockStreamProvider.AssertExpectations(t)
	mockStreamProvider.On("LogStream", mock.Anything).Return(stream, nil).Once()

	receivedLogs := make([]string, 0)
	processor := &kubernetesLogProcessor{
		logger:       &common.BuildLogger{},
		logProviders: []logStreamProvider{mockStreamProvider},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := processor.Listen(ctx)
	for log := range ch {
		receivedLogs = append(receivedLogs, log)
		if len(receivedLogs) == len(expectedLines) {
			break
		}
	}

	assert.Equal(t, expectedLines, receivedLogs)
}

func TestListenCancelContext(t *testing.T) {
	mockStreamProvider := &mockLogStreamProvider{}
	defer mockStreamProvider.AssertExpectations(t)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	mockStreamProvider.On("LogStream", mock.Anything).
		Run(func(args mock.Arguments) {
			<-ctx.Done()
		}).
		Return(nil, io.EOF)

	processor := &kubernetesLogProcessor{
		logger:       &common.BuildLogger{},
		logProviders: []logStreamProvider{mockStreamProvider},
	}

	<-processor.Listen(ctx)
}

func TestAttachReconnect(t *testing.T) {
	const expectedReconnectCount = 3

	tests := map[string]struct {
		logStreamReturnReaderCloser io.ReadCloser
		logStreamReturnError        error
	}{
		"request error": {
			logStreamReturnReaderCloser: nil,
			logStreamReturnError:        io.EOF,
		},
		"broken stream error": {
			logStreamReturnReaderCloser: newBrokenReader(io.EOF),
			logStreamReturnError:        nil,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())

			mockStreamProvider := &mockLogStreamProvider{}
			defer mockStreamProvider.AssertExpectations(t)

			var reconnects int
			mockStreamProvider.
				On("LogStream", mock.Anything).
				Run(func(args mock.Arguments) {
					reconnects++
					if reconnects == expectedReconnectCount {
						cancel()
					}
				}).
				Return(tt.logStreamReturnReaderCloser, tt.logStreamReturnError).
				Times(expectedReconnectCount)

			minBackoff := 100 * time.Millisecond
			processor := &kubernetesLogProcessor{
				logProviders: []logStreamProvider{mockStreamProvider},
				logger:       &common.BuildLogger{},
				backoff:      backoff.Backoff{Min: minBackoff},
			}

			started := time.Now()

			<-processor.Listen(ctx)
			assert.True(t, time.Now().Sub(started) > minBackoff*expectedReconnectCount)
		})
	}
}

func TestAttachCorrectSinceTime(t *testing.T) {
	stream := logsToReadCloser(
		log{line: "line", timestamp: mustParseTimestamp(t, testDate1)},
		log{line: "line", timestamp: mustParseTimestamp(t, testDate2)},
	)

	ctx, cancel := context.WithCancel(context.Background())

	mockStreamProvider := &mockLogStreamProvider{}
	defer mockStreamProvider.AssertExpectations(t)

	mockStreamProvider.
		On("LogStream", mock.MatchedBy(func(sinceTime *time.Time) bool {
			require.NotNil(t, sinceTime)
			return sinceTime.Equal(time.Time{})
		})).
		Return(stream, nil).
		Once()

	mockStreamProvider.
		On("LogStream", mock.MatchedBy(func(sinceTime *time.Time) bool {
			require.NotNil(t, sinceTime)
			return mustParseTimestamp(t, testDate2).Equal(*sinceTime)
		})).
		Run(func(args mock.Arguments) {
			cancel()
		}).
		Return(&brokenReader{}, nil).
		Once()

	processor := &kubernetesLogProcessor{
		logProviders: []logStreamProvider{mockStreamProvider},
		logger:       &common.BuildLogger{},
	}

	ch := processor.Listen(ctx)
	for range ch {
	}
}

func TestAttachCloseStream(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	r, w := io.Pipe()
	go w.Write([]byte(log{line: "line", timestamp: mustParseTimestamp(t, testDate1)}.String() + "\n"))
	go w.Write([]byte(log{line: "line", timestamp: mustParseTimestamp(t, testDate2)}.String() + "\n"))

	mockStreamProvider := &mockLogStreamProvider{}
	defer mockStreamProvider.AssertExpectations(t)

	mockStreamProvider.
		On("LogStream", mock.Anything).
		Return(r, nil).
		Once()

	mockStreamProvider.
		On("LogStream", mock.Anything).
		Run(func(args mock.Arguments) {
			cancel()
		}).
		Return(newBrokenReader(io.EOF), nil).
		Once()

	processor := &kubernetesLogProcessor{
		logProviders: []logStreamProvider{mockStreamProvider},
		logger:       &common.BuildLogger{},
	}

	ch := processor.Listen(ctx)
	<-ch

	_ = r.CloseWithError(errors.New("closed"))

	for range ch {
	}
}

func TestAttachReconnectWhenStreamEOF(t *testing.T) {
	line := log{line: "line", timestamp: mustParseTimestamp(t, testDate1)}

	logStreamError := errors.New("log stream err")

	tests := map[string]struct {
		mockStreamProviderAssertions func(*mockLogStreamProvider)
	}{
		"stream EOF": {
			mockStreamProviderAssertions: func(m *mockLogStreamProvider) {
				m.On("LogStream", mock.Anything).
					Return(newBrokenReader(io.EOF), nil).
					Once()

				m.On("LogStream", mock.Anything).
					Return(logsToReadCloser(line), nil).
					Once()

				m.On("LogStream", mock.Anything).
					Return(newBrokenReader(io.EOF), nil).
					Maybe()
			},
		},
		"log stream error": {
			mockStreamProviderAssertions: func(m *mockLogStreamProvider) {
				m.On("LogStream", mock.Anything).
					Return(nil, logStreamError).
					Once()

				m.On("LogStream", mock.Anything).
					Return(logsToReadCloser(line), nil).
					Once()

				m.On("LogStream", mock.Anything).
					Return(nil, logStreamError).
					Maybe()
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			mockStreamProvider := &mockLogStreamProvider{}
			defer mockStreamProvider.AssertExpectations(t)

			tt.mockStreamProviderAssertions(mockStreamProvider)

			processor := &kubernetesLogProcessor{
				logProviders: []logStreamProvider{mockStreamProvider},
				logger:       &common.BuildLogger{},
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			received := <-processor.Listen(ctx)
			assert.Equal(t, line.line, received)
		})
	}
}

func TestResumesFromCorrectSinceTimeAfterSuccessThenFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	expectedLineContents := "line"

	r, w := io.Pipe()
	go w.Write([]byte(log{line: expectedLineContents, timestamp: mustParseTimestamp(t, testDate1)}.String() + "\n"))
	go w.Write([]byte("\n"))

	mockStreamProvider := &mockLogStreamProvider{}
	defer mockStreamProvider.AssertExpectations(t)

	mockStreamProvider.
		On("LogStream", mock.Anything).
		Return(r, nil).
		Once()

	mockStreamProvider.
		On("LogStream", mock.MatchedBy(func(sinceTime *time.Time) bool {
			require.NotNil(t, sinceTime)
			return mustParseTimestamp(t, testDate1).Equal(*sinceTime)
		})).
		Run(func(args mock.Arguments) {
			cancel()
		}).
		Return(newBrokenReader(io.EOF), nil).
		Once()

	processor := &kubernetesLogProcessor{
		logProviders: []logStreamProvider{mockStreamProvider},
		logger:       &common.BuildLogger{},
	}

	ch := processor.Listen(ctx)
	line := <-ch
	assert.Equal(t, expectedLineContents, line)

	<-ch
}

func TestScanHandlesStreamError(t *testing.T) {
	closedErr := errors.New("closed")
	processor := &kubernetesLogProcessor{}

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
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			scanner, ch := processor.scan(ctx, newBrokenReader(tt.readerError))
			line, more := <-ch
			assert.Empty(t, line)
			assert.False(t, more)
			assert.True(t, errors.Is(scanner.Err(), tt.expectedError))
		})
	}
}

func TestScanHandlesCancelledContext(t *testing.T) {
	processor := &kubernetesLogProcessor{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	scanner, ch := processor.scan(ctx, logsToReadCloser(log{}))
	var wg sync.WaitGroup
	go func() {
		defer wg.Done()

		// Block the channel, so there's no consumers
		time.Sleep(1 * time.Second)

		// Assert that the channel is closed
		line, more := <-ch
		assert.Empty(t, line)
		assert.False(t, more)

		// Assert that the scanner had no error
		assert.Nil(t, scanner.Err())
	}()

	wg.Wait()
}
