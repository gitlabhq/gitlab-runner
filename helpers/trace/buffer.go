package trace

import (
	"bufio"
	"errors"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"os"
	"sync"
	"unicode/utf8"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/trace/internal/masker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/trace/internal/tokensanitizer"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/trace/internal/urlsanitizer"
	"golang.org/x/text/encoding"
	"golang.org/x/text/transform"
)

const defaultBytesLimit = 4 * 1024 * 1024 // 4MB

var errLogLimitExceeded = errors.New("log limit exceeded")

type Buffer struct {
	lock sync.RWMutex
	lw   *limitWriter
	w    io.WriteCloser

	logFile  *os.File
	bufw     *bufio.Writer
	checksum hash.Hash32

	opts options

	// failedFlush indicates that a read which subsequentialy attempted to
	// flush data to the underlying writer failed. In this scenario, calls to
	// Write() will immediately attempt to flush and return any error on a
	// failure.
	failedFlush bool
}

type options struct {
	urlParamMasking bool
}

type Option func(*options) error

func WithURLParamMasking(enabled bool) Option {
	return func(o *options) error {
		o.urlParamMasking = enabled
		return nil
	}
}

func (b *Buffer) SetMasked(opts common.MaskOptions) {
	b.lock.Lock()
	defer b.lock.Unlock()

	// close existing writer to flush data
	if b.w != nil {
		b.w.Close()
	}

	// convert bytes to utf-8
	b.w = transform.NewWriter(b.lw, encoding.Replacement.NewEncoder())

	// mask values
	b.w = masker.New(b.w, opts.Phrases)

	// prefixes values
	b.w = tokensanitizer.New(b.w, opts.TokenPrefixes)

	// mask urls if enabled
	if b.opts.urlParamMasking {
		b.w = urlsanitizer.New(b.w)
	}
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
	b.lock.Lock()
	defer b.lock.Unlock()

	// For simplicity, we read only from the file, rather than also the bufio.Writer.
	// To ensure the underlying file has the data requested, we always flush the
	// buffer.
	//
	// If a failure occurs on flushing the data, we store that an error occurred so
	// buffer.Write() can retry and additionally return any error on the write side.
	if err := b.bufw.Flush(); err != nil {
		b.failedFlush = true
		return nil, fmt.Errorf("flushing log buffer: %w", err)
	}

	size := int(b.lw.written - int64(offset))
	if n > size {
		n = size
	}

	buf := make([]byte, n)
	_, err := b.logFile.ReadAt(buf, int64(offset))
	if err == io.EOF {
		err = nil
	}

	return buf, err
}

func (b *Buffer) Write(p []byte) (int, error) {
	b.lock.Lock()
	defer b.lock.Unlock()

	n, err := b.w.Write(p)
	// if we get a log limit exceeded error, we've written the log limit
	// notice out to the log and will now silently not write any additional
	// data: we return len(p), nil so the caller continues as normal.
	if err == errLogLimitExceeded {
		return len(p), nil
	}

	// if we previously failed to flush to the underlying writer, try again
	// and return any failure immediately.
	if b.failedFlush {
		if err := b.bufw.Flush(); err != nil {
			return n, err
		}
		b.failedFlush = false
	}

	return n, err
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

func New(opts ...Option) (*Buffer, error) {
	logFile, err := newLogFile()
	if err != nil {
		return nil, err
	}

	options := options{
		urlParamMasking: true,
	}

	for _, o := range opts {
		err := o(&options)
		if err != nil {
			return nil, err
		}
	}

	buffer := &Buffer{
		logFile:  logFile,
		bufw:     bufio.NewWriter(logFile),
		checksum: crc32.NewIEEE(),
		opts:     options,
	}

	buffer.lw = &limitWriter{
		w:       io.MultiWriter(buffer.bufw, buffer.checksum),
		written: 0,
		limit:   defaultBytesLimit,
	}

	buffer.SetMasked(common.MaskOptions{})

	return buffer, nil
}

func newLogFile() (*os.File, error) {
	return os.CreateTemp("", "trace")
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
