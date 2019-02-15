package trace

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"sync"

	"github.com/markelog/trie"

	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

const maskedText = "[MASKED]"
const defaultBytesLimit = 4 * 1024 * 1024 // 4MB

type Buffer struct {
	writer        io.WriteCloser
	lock          sync.RWMutex
	log           bytes.Buffer
	logMaskedSize int
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

func (b *Buffer) limitExceededMessage() string {
	return fmt.Sprintf("\n%sJob's log exceeded limit of %v bytes.%s\n", helpers.ANSI_BOLD_RED, b.bytesLimit, helpers.ANSI_RESET)
}

func (b *Buffer) Bytes() []byte {
	b.lock.RLock()
	defer b.lock.RUnlock()

	return b.log.Bytes()[0:b.logMaskedSize]
}

func (b *Buffer) String() string {
	return string(b.Bytes())
}

func (b *Buffer) Write(data []byte) (n int, err error) {
	return b.writer.Write(data)
}

func (b *Buffer) Close() error {
	// wait for trace to finish
	err := b.writer.Close()
	<-b.finish
	return err
}

func (b *Buffer) advanceAllUnsafe() {
	b.logMaskedSize = b.log.Len()
}

func (b *Buffer) advanceAll() {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.advanceAllUnsafe()
}

// advanceLogUnsafe is assumed to be run every character
func (b *Buffer) advanceLogUnsafe() error {
	// advance all if no masking is enabled
	if b.maskTree == nil {
		b.advanceAllUnsafe()
		return nil
	}

	rest := string(b.log.Bytes()[b.logMaskedSize:])

	results := b.maskTree.Search(rest)
	if len(results) == 0 {
		// we can advance as no match was found
		b.advanceAllUnsafe()
		return nil
	}

	// full match was found
	if len(results) == 1 && results[0].Key == rest {
		b.log.Truncate(b.logMaskedSize)
		b.log.WriteString(maskedText)
		b.advanceAllUnsafe()
	}

	// partial match, wait for more characters
	return nil
}

func (b *Buffer) writeRune(r rune) (int, error) {
	b.lock.Lock()
	defer b.lock.Unlock()

	n, err := b.log.WriteRune(r)
	if err != nil {
		return n, err
	}

	err = b.advanceLogUnsafe()
	if err != nil {
		return n, err
	}

	if b.log.Len() < b.bytesLimit {
		return n, nil
	}

	b.log.WriteString(b.limitExceededMessage())
	return n, io.EOF
}

func (b *Buffer) process(pipe *io.PipeReader) {
	defer pipe.Close()

	stopped := false
	reader := bufio.NewReader(pipe)

	for {
		r, s, err := reader.ReadRune()
		if s <= 0 {
			break
		} else if stopped {
			// ignore symbols if job log exceeded limit
			continue
		} else if err == nil {
			_, err = b.writeRune(r)
			if err == io.EOF {
				stopped = true
			}
		} else {
			// ignore invalid characters
			continue
		}
	}

	b.advanceAll()
	close(b.finish)
}

func New() *Buffer {
	reader, writer := io.Pipe()
	buffer := &Buffer{
		writer:     writer,
		bytesLimit: defaultBytesLimit,
		finish:     make(chan struct{}),
	}
	go buffer.process(reader)
	return buffer
}
