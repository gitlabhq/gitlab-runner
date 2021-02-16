package trace

import (
	"bufio"
	"bytes"
	"fmt"
	"hash"
	"hash/crc32"
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
	lock          sync.RWMutex
	logSize       int
	logWriter     *bufio.Writer
	advanceBuffer bytes.Buffer

	logFile    *os.File
	bytesLimit int
	checksum   hash.Hash32

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
	b.lock.Lock()
	b.bytesLimit = size
	b.lock.Unlock()
}

func (b *Buffer) Size() int {
	b.lock.RLock()
	defer b.lock.RUnlock()

	return b.logSize
}

func (b *Buffer) Bytes(offset, n int) ([]byte, error) {
	b.lock.Lock()
	defer b.lock.Unlock()

	err := b.logWriter.Flush()
	if err != nil {
		return nil, err
	}

	return ioutil.ReadAll(io.NewSectionReader(b.logFile, int64(offset), int64(n)))
}

func (b *Buffer) Write(data []byte) (n int, err error) {
	b.lock.Lock()
	defer b.lock.Unlock()

	for _, r := range bytes.Runes(data) {
		_ = b.writeRuneUnsafe(r)
	}
	_ = b.advanceAllUnsafe()

	return len(data), nil
}

func (b *Buffer) Finish() {
	b.lock.Lock()
	defer b.lock.Unlock()

	_ = b.advanceAllUnsafe()
}

func (b *Buffer) Close() {
	_ = b.logFile.Close()
	_ = os.Remove(b.logFile.Name())
}

func (b *Buffer) Checksum() string {
	return fmt.Sprintf("crc32:%08x", b.checksum.Sum32())
}

func (b *Buffer) advanceAllUnsafe() error {
	n, err := b.advanceBuffer.WriteTo(b.logWriter)
	b.logSize += int(n)
	return err
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
		"\n%sJob's log exceeded limit of %v bytes.\n"+
			"Job execution will continue but no more output will be collected.%s\n",
		helpers.ANSI_BOLD_YELLOW,
		b.bytesLimit,
		helpers.ANSI_RESET,
	)
}

func (b *Buffer) writeRuneUnsafe(r rune) error {
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
	if b.logSize < b.bytesLimit {
		return nil
	}

	b.advanceBuffer.Reset()
	b.advanceBuffer.WriteString(b.limitExceededMessage())
	return b.advanceAllUnsafe()
}

func New() (*Buffer, error) {
	logFile, err := ioutil.TempFile("", "trace")
	if err != nil {
		return nil, err
	}

	checksum := crc32.NewIEEE()

	buffer := &Buffer{
		bytesLimit: defaultBytesLimit,
		logFile:    logFile,
		checksum:   checksum,
		logWriter:  bufio.NewWriter(io.MultiWriter(logFile, checksum)),
	}

	return buffer, nil
}
