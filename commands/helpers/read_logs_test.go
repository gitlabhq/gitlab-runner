//go:build !integration

package helpers

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/test"
)

func TestNewReadLogsCommandFileNotExist(t *testing.T) {
	cmd := newReadLogsCommand()
	cmd.logStreamProvider = &fileLogStreamProvider{
		waitFileTimeout: 2 * time.Second,
		path:            "not_exists",
	}

	err := cmd.readLogs()
	assert.ErrorIs(t, err, errWaitingFileTimeout)
}

func TestNewReadLogsCommandNoAttempts(t *testing.T) {
	cmd := newReadLogsCommand()
	cmd.WaitFileTimeout = 0

	err := cmd.execute()
	assert.ErrorIs(t, err, errNoAttemptsToOpenFile)
}

func TestNewReadLogsCommandFileSeekToInvalidLocation(t *testing.T) {
	testFile, cleanup := setupTestFile(t)
	defer cleanup()

	cmd := newReadLogsCommand()
	cmd.Path = testFile.Name()
	cmd.WaitFileTimeout = time.Minute
	cmd.Offset = -1

	err := cmd.execute()
	var expectedErr *os.PathError
	assert.ErrorAs(t, err, &expectedErr)
}

func setupTestFile(t *testing.T) (*os.File, func()) {
	f, err := os.CreateTemp("", "")
	require.NoError(t, err)

	cleanup := func() {
		_ = f.Close()
		_ = os.Remove(f.Name())
	}

	return f, cleanup
}

func TestNewReadLogsCommandFileLogStreamProviderCorrect(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)

	cmd := newReadLogsCommand()
	cmd.WaitFileTimeout = 10 * time.Second
	f, cleanup := setupTestFile(t)
	time.AfterFunc(time.Second, cleanup)
	cmd.Path = f.Name()

	err := cmd.execute()
	assert.True(t, os.IsNotExist(err), "expected err %T, but got %T", os.ErrNotExist, err)
	assert.Equal(t, &fileLogStreamProvider{
		waitFileTimeout: cmd.WaitFileTimeout,
		path:            cmd.Path,
	}, cmd.logStreamProvider)
}

func TestNewReadLogsCommandLines(t *testing.T) {
	lines := []string{"1", "2", "3"}
	f, cleanup := setupTestFile(t)
	defer cleanup()
	appendToFile(t, f, lines)

	cmd := newReadLogsCommand()

	mockLogOutputWriter := new(mockLogOutputWriter)
	defer mockLogOutputWriter.AssertExpectations(t)
	_, wg := setupMockLogOutputWriterFromLines(mockLogOutputWriter, lines, 0)
	cmd.logOutputWriter = mockLogOutputWriter

	mockLogStreamProvider := new(mockLogStreamProvider)
	defer mockLogStreamProvider.AssertExpectations(t)
	mockLogStreamProvider.On("Open").Return(f, nil)
	cmd.logStreamProvider = mockLogStreamProvider

	go func() {
		wg.Wait()
		_ = f.Close()
	}()

	err := cmd.readLogs()
	var expectedErr *os.PathError
	assert.ErrorAs(t, err, &expectedErr)
}

func appendToFile(t *testing.T, f *os.File, lines []string) {
	fw, err := os.OpenFile(f.Name(), os.O_WRONLY|os.O_APPEND, 0600)
	require.NoError(t, err)
	_, err = fw.Write([]byte(strings.Join(lines, "\n")))
	require.NoError(t, err)
	err = fw.Close()
	require.NoError(t, err)
}

func setupMockLogOutputWriterFromLines(lw *mockLogOutputWriter, lines []string, offset int) (int, *sync.WaitGroup) {
	var wg sync.WaitGroup
	wg.Add(len(lines))

	for i, l := range lines {
		offset += len(l)
		if i < len(lines)-1 {
			offset++ // account for the len of the newline character \n
		}

		lw.On("Write", fmt.Sprintf("%d %s\n", offset, l)).Run(func(mock.Arguments) {
			wg.Done()
		})
	}

	return offset, &wg
}

func TestNewReadLogsCommandWriteLinesWithDelay(t *testing.T) {
	lines1 := []string{"1", "2", "3"}
	lines2 := []string{"4", "5", "6"}

	f, cleanup := setupTestFile(t)
	defer cleanup()
	appendToFile(t, f, lines1)

	cmd := newReadLogsCommand()

	mockLogOutputWriter := new(mockLogOutputWriter)
	defer mockLogOutputWriter.AssertExpectations(t)
	offset, wg := setupMockLogOutputWriterFromLines(mockLogOutputWriter, lines1, 0)
	cmd.logOutputWriter = mockLogOutputWriter

	mockLogStreamProvider := new(mockLogStreamProvider)
	defer mockLogStreamProvider.AssertExpectations(t)
	mockLogStreamProvider.On("Open").Return(f, nil)
	cmd.logStreamProvider = mockLogStreamProvider

	go func() {
		wg.Wait()

		time.Sleep(5 * time.Second)
		_, wg = setupMockLogOutputWriterFromLines(mockLogOutputWriter, lines2, offset)
		appendToFile(t, f, lines2)

		wg.Wait()

		_ = f.Close()
	}()

	err := cmd.readLogs()
	var expectedErr *os.PathError
	assert.ErrorAs(t, err, &expectedErr)
}

func TestSplitLinesAccordingToBufferSize(t *testing.T) {
	lines := []string{strings.Repeat("1", 32), strings.Repeat("2", 32)}

	f, cleanup := setupTestFile(t)
	defer cleanup()
	appendToFile(t, f, lines)

	cmd := newReadLogsCommand()
	cmd.readerBufferSize = 16 // this is the minimum allowed buffer size by bufio.NewReader

	mockLogOutputWriter := new(mockLogOutputWriter)
	defer mockLogOutputWriter.AssertExpectations(t)

	var wg sync.WaitGroup
	wg.Add(5)
	var wgDone = func(mock.Arguments) { wg.Done() }

	mockLogOutputWriter.On("Write", fmt.Sprintf("16 %s\n", strings.Repeat("1", 16))).Run(wgDone)
	mockLogOutputWriter.On("Write", fmt.Sprintf("32 %s\n", strings.Repeat("1", 16))).Run(wgDone)
	mockLogOutputWriter.On("Write", "33 \n").Run(wgDone)
	mockLogOutputWriter.On("Write", fmt.Sprintf("49 %s\n", strings.Repeat("2", 16))).Run(wgDone)
	mockLogOutputWriter.On("Write", fmt.Sprintf("65 %s\n", strings.Repeat("2", 16))).Run(wgDone)

	cmd.logOutputWriter = mockLogOutputWriter

	mockLogStreamProvider := new(mockLogStreamProvider)
	defer mockLogStreamProvider.AssertExpectations(t)
	mockLogStreamProvider.On("Open").Return(f, nil)
	cmd.logStreamProvider = mockLogStreamProvider

	go func() {
		wg.Wait()
		_ = f.Close()
	}()

	err := cmd.readLogs()
	var expectedErr *os.PathError
	assert.ErrorAs(t, err, &expectedErr)
}

func TestSeek(t *testing.T) {
	lines := []string{strings.Repeat("1", 32)}

	f, cleanup := setupTestFile(t)
	defer cleanup()
	appendToFile(t, f, lines)

	cmd := newReadLogsCommand()
	cmd.Offset = 16
	cmd.readerBufferSize = 16 // this is the minimum allowed buffer size by bufio.NewReader

	mockLogOutputWriter := new(mockLogOutputWriter)
	defer mockLogOutputWriter.AssertExpectations(t)

	var wg sync.WaitGroup
	wg.Add(1)
	var wgDone = func(mock.Arguments) { wg.Done() }

	mockLogOutputWriter.On("Write", fmt.Sprintf("32 %s\n", strings.Repeat("1", 16))).Run(wgDone)
	cmd.logOutputWriter = mockLogOutputWriter

	mockLogStreamProvider := new(mockLogStreamProvider)
	defer mockLogStreamProvider.AssertExpectations(t)
	mockLogStreamProvider.On("Open").Return(f, nil)
	cmd.logStreamProvider = mockLogStreamProvider

	go func() {
		wg.Wait()
		_ = f.Close()
	}()

	err := cmd.readLogs()
	var expectedErr *os.PathError
	assert.ErrorAs(t, err, &expectedErr)
}
