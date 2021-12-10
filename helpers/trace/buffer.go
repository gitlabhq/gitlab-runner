package trace

import (
	"errors"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"sync"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/transform"

	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

const defaultBytesLimit = 4 * 1024 * 1024 // 4MB

var errLogLimitExceeded = errors.New("log limit exceeded")

type Buffer struct {
	lock sync.RWMutex
	lw   *limitWriter
	w    io.WriteCloser

	logFile  *os.File
	checksum hash.Hash32
}

type inverseLengthSort []string

func (s inverseLengthSort) Len() int {
	return len(s)
}

func (s inverseLengthSort) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s inverseLengthSort) Less(i, j int) bool {
	return len(s[i]) > len(s[j])
}

func (b *Buffer) SetMasked(values []string) {
	b.lock.Lock()
	defer b.lock.Unlock()

	// close existing writer to flush data
	if b.w != nil {
		b.w.Close()
	}

	defaultTransformers := []transform.Transformer{
		encoding.Replacement.NewEncoder(),
	}

	transformers := make([]transform.Transformer, 0, len(values)+len(defaultTransformers))

	sort.Sort(inverseLengthSort(values))
	seen := make(map[string]struct{})
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}

		transformers = append(transformers, newPhraseTransform(value))
	}

	transformers = append(transformers, defaultTransformers...)

	b.w = transform.NewWriter(b.lw, transform.Chain(transformers...))
}

func (b *Buffer) SetLimit(size int) {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.lw.limit = int64(size)
}

func (b *Buffer) Size() int {
	b.lock.RLock()
	defer b.lock.RUnlock()

	if b.lw == nil {
		return 0
	}
	return int(b.lw.written)
}

func (b *Buffer) Bytes(offset, n int) ([]byte, error) {
	return ioutil.ReadAll(io.NewSectionReader(b.logFile, int64(offset), int64(n)))
}

func (b *Buffer) Write(p []byte) (int, error) {
	b.lock.Lock()
	defer b.lock.Unlock()

	src := p
	var n int
	for len(src) > 0 {
		written, err := b.w.Write(src)
		// if we get a log limit exceeded error, we've written the log limit
		// notice out to the log and will now silently not write any additional
		// data: we return len(p), nil so the caller continues as normal.
		if err == errLogLimitExceeded {
			return len(p), nil
		}
		if err != nil {
			return n, err
		}

		// the text/transformer implementation can return n < len(p) without an
		// error. For this reason, we continue writing whatever data is left
		// unless nothing was written (therefore zero progress) on our call to
		// Write().
		if written == 0 {
			return n, io.ErrShortWrite
		}

		src = src[written:]
		n += written
	}

	return n, nil
}

func (b *Buffer) Finish() {
	b.lock.RLock()
	defer b.lock.RUnlock()

	if b.w != nil {
		_ = b.w.Close()
	}
}

func (b *Buffer) Close() {
	_ = b.logFile.Close()
	_ = os.Remove(b.logFile.Name())
}

func (b *Buffer) Checksum() string {
	b.lock.RLock()
	defer b.lock.RUnlock()

	return fmt.Sprintf("crc32:%08x", b.checksum.Sum32())
}

type limitWriter struct {
	w       io.Writer
	written int64
	limit   int64
}

func (w *limitWriter) Write(p []byte) (int, error) {
	capacity := w.limit - w.written

	if capacity <= 0 {
		return 0, errLogLimitExceeded
	}

	if int64(len(p)) >= capacity {
		p = truncateSafeUTF8(p, capacity)
		n, err := w.w.Write(p)
		if err == nil {
			err = errLogLimitExceeded
		}
		if n < 0 {
			n = 0
		}
		w.written += int64(n)
		w.writeLimitExceededMessage()

		return n, err
	}

	n, err := w.w.Write(p)
	if n < 0 {
		n = 0
	}
	w.written += int64(n)
	return n, err
}

func (w *limitWriter) writeLimitExceededMessage() {
	n, _ := fmt.Fprintf(
		w.w,
		"\n%sJob's log exceeded limit of %v bytes.\n"+
			"Job execution will continue but no more output will be collected.%s\n",
		helpers.ANSI_BOLD_YELLOW,
		w.limit,
		helpers.ANSI_RESET,
	)
	w.written += int64(n)
}

func New() (*Buffer, error) {
	logFile, err := newLogFile()
	if err != nil {
		return nil, err
	}

	buffer := &Buffer{
		logFile:  logFile,
		checksum: crc32.NewIEEE(),
	}

	buffer.lw = &limitWriter{
		w:       io.MultiWriter(buffer.logFile, buffer.checksum),
		written: 0,
		limit:   defaultBytesLimit,
	}

	buffer.SetMasked(nil)

	return buffer, nil
}

func newLogFile() (*os.File, error) {
	return ioutil.TempFile("", "trace")
}

// truncateSafeUTF8 truncates a job log at the capacity but avoids
// breaking up a multi-byte UTF-8 character.
func truncateSafeUTF8(p []byte, capacity int64) []byte {
	for i := 0; i < 4; i++ {
		r, s := utf8.DecodeLastRune(p[:capacity])
		if r == utf8.RuneError && s == 1 {
			capacity--
			continue
		}
		break
	}

	return p[:capacity]
}
