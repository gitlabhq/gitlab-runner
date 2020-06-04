package helpers

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

const (
	defaultReaderBufferSize        = 16 * 1024
	defaultCheckFileExistsInterval = time.Second
	pollFileContentsTimeout        = 500 * time.Millisecond
)

var (
	errWaitingFileTimeout   = errors.New("timeout waiting for file to be created")
	errNoAttemptsToOpenFile = errors.New("no attempts to open log file configured")
)

type logStreamProvider interface {
	Open() (readSeekCloser, error)
}

type readSeekCloser interface {
	io.ReadSeeker
	io.Closer
}

type fileLogStreamProvider struct {
	cmd *ReadLogsCommand
}

func (p *fileLogStreamProvider) Open() (readSeekCloser, error) {
	attempts := int(p.cmd.WaitFileTimeout / defaultCheckFileExistsInterval)
	if attempts < 1 {
		return nil, errNoAttemptsToOpenFile
	}

	for i := 0; i < attempts; i++ {
		f, err := os.Open(p.cmd.Path)
		if os.IsNotExist(err) {
			time.Sleep(defaultCheckFileExistsInterval)
			continue
		}

		return f, err
	}

	return nil, errWaitingFileTimeout
}

type logOutputWriter interface {
	Write(string)
}

type streamLogOutputWriter struct {
	stream io.Writer
}

func (s *streamLogOutputWriter) Write(data string) {
	_, _ = fmt.Fprint(s.stream, data)
}

type ReadLogsCommand struct {
	Path            string        `long:"path"`
	Offset          int64         `long:"offset"`
	WaitFileTimeout time.Duration `long:"wait-file-timeout"`

	logStreamProvider logStreamProvider
	logOutputWriter   logOutputWriter
	readerBufferSize  int
}

func newReadLogsCommand() *ReadLogsCommand {
	cmd := new(ReadLogsCommand)
	cmd.logStreamProvider = &fileLogStreamProvider{
		cmd: cmd,
	}
	cmd.logOutputWriter = &streamLogOutputWriter{stream: os.Stdout}
	cmd.readerBufferSize = defaultReaderBufferSize
	// by default check if the file exists at least once
	cmd.WaitFileTimeout = defaultCheckFileExistsInterval

	return cmd
}

func (c *ReadLogsCommand) Execute(*cli.Context) {
	if err := c.readLogs(); err != nil {
		c.logOutputWriter.Write(fmt.Sprintf("error reading logs %v\n", err))
		os.Exit(1)
	}
}

func (c *ReadLogsCommand) readLogs() error {
	s, r, err := c.openFileReader()
	if err != nil {
		return err
	}
	defer s.Close()

	offset := c.Offset
	for {
		buf, err := r.ReadSlice('\n')
		if len(buf) > 0 {
			offset += int64(len(buf))
			// if the buffer was filled by a message larger than the
			// buffer size we must make sure that it ends with a new line
			// so it gets properly handled by the executor which splits by new lines
			if buf[len(buf)-1] != '\n' {
				buf = append(buf, '\n')
			}

			c.logOutputWriter.Write(fmt.Sprintf("%d %s", offset, buf))
		}

		// io.EOF means that we reached the end of the file
		// we try reading from it again to see if there are new contents
		// bufio.ErrBufferFull means that the message was larger than the buffer
		// we print the message so far along with a new line character
		// and continue reading the rest of it from the stream
		if errors.Is(err, io.EOF) {
			time.Sleep(pollFileContentsTimeout)
		} else if err != nil && !errors.Is(err, bufio.ErrBufferFull) {
			return err
		}
	}
}

func (c *ReadLogsCommand) openFileReader() (readSeekCloser, *bufio.Reader, error) {
	s, err := c.logStreamProvider.Open()
	if err != nil {
		return nil, nil, err
	}

	_, err = s.Seek(c.Offset, 0)
	if err != nil {
		_ = s.Close()
		return nil, nil, err
	}

	return s, bufio.NewReaderSize(s, c.readerBufferSize), nil
}

func init() {
	common.RegisterCommand2("read-logs", "reads job logs from a file, used by kubernetes executor (internal)", newReadLogsCommand())
}
