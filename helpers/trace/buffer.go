package trace

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sync"

	"github.com/markelog/trie"

	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

const maskedText = "[MASKED]"
const defaultBytesLimit = 4 * 1024 * 1024 // 4MB

type Buffer struct {
	writer        io.WriteCloser
	lock          sync.RWMutex
	logFile       *os.File
	logSize       int
	logWriter     *bufio.Writer
	advanceBuffer bytes.Buffer
	bytesLimit    int
	finish        chan struct{}

	maskTree *trie.Trie
}

func (b *Buffer) SetMasked(values []string) {
	if len(values) == 0 {
		b.maskTree = nil
		return
	}

	maskTree := trie.New()
	for _, value := range values {
		maskTree.Add(value, nil)
	}
	b.maskTree = maskTree
}

func (b *Buffer) SetLimit(size int) {
	b.bytesLimit = size
}

func (b *Buffer) Size() int {
	return b.logSize
}

func (b *Buffer) Reader(offset, n int) (io.ReadSeeker, error) {
	b.lock.Lock()
	defer b.lock.Unlock()

	err := b.logWriter.Flush()
	if err != nil {
		return nil, err
	}

	return io.NewSectionReader(b.logFile, int64(offset), int64(n)), nil
}

func (b *Buffer) Bytes(offset, n int) ([]byte, error) {
	reader, err := b.Reader(offset, n)
	if err != nil {
		return nil, err
	}

	return ioutil.ReadAll(reader)
}

func (b *Buffer) Write(data []byte) (n int, err error) {
	return b.writer.Write(data)
}

func (b *Buffer) Finish() {
	// wait for trace to finish
	_ = b.writer.Close()
	<-b.finish
}

func (b *Buffer) Close() {
	_ = b.logFile.Close()
	_ = os.Remove(b.logFile.Name())
}

func (b *Buffer) advanceAllUnsafe() error {
	n, err := b.advanceBuffer.WriteTo(b.logWriter)
	b.logSize += int(n)
	return err
}

func (b *Buffer) advanceAll() {
	b.lock.Lock()
	defer b.lock.Unlock()

	_ = b.advanceAllUnsafe()
}

// advanceLogUnsafe is assumed to be run every character
func (b *Buffer) advanceLogUnsafe() error {
	// advance all if no masking is enabled
	if b.maskTree == nil {
		return b.advanceAllUnsafe()
	}

	rest := b.advanceBuffer.String()
	results := b.maskTree.Search(rest)
	if len(results) == 0 {
		// we can advance as no match was found
		return b.advanceAllUnsafe()
	}

	// full match was found
	if len(results) == 1 && results[0].Key == rest {
		b.advanceBuffer.Reset()
		b.advanceBuffer.WriteString(maskedText)
		return b.advanceAllUnsafe()
	}

	// partial match, wait for more characters
	return nil
}

func (b *Buffer) limitExceededMessage() string {
	return fmt.Sprintf(
		"\n%sJob's log exceeded limit of %v bytes.%s\n",
		helpers.ANSI_BOLD_RED,
		b.bytesLimit,
		helpers.ANSI_RESET,
	)
}

func (b *Buffer) writeRune(r rune) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	// over trace limit
	if b.logSize > b.bytesLimit {
		return io.EOF
	}

	if _, err := b.advanceBuffer.WriteRune(r); err != nil {
		return err
	}

	if err := b.advanceLogUnsafe(); err != nil {
		return err
	}

	// under trace limit
	if b.logSize <= b.bytesLimit {
		return nil
	}

	b.advanceBuffer.Reset()
	b.advanceBuffer.WriteString(b.limitExceededMessage())
	return b.advanceAllUnsafe()
}

func (b *Buffer) process(pipe *io.PipeReader) {
	defer func() { _ = pipe.Close() }()

	reader := bufio.NewReader(pipe)

	for {
		r, s, err := reader.ReadRune()
		if s <= 0 {
			break
		}

		if err == nil {
			// only write valid characters
			_ = b.writeRune(r)
		}
	}

	b.advanceAll()
	close(b.finish)
}

func New() (*Buffer, error) {
	logFile, err := ioutil.TempFile("", "trace")
	if err != nil {
		return nil, err
	}

	reader, writer := io.Pipe()
	buffer := &Buffer{
		writer:     writer,
		bytesLimit: defaultBytesLimit,
		finish:     make(chan struct{}),
		logFile:    logFile,
		logWriter:  bufio.NewWriter(logFile),
	}
	go buffer.process(reader)
	return buffer, nil
}
