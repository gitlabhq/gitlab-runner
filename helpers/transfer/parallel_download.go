package transfer

import (
	"fmt"
	"io"
	"sync"
)

// FetchChunk returns a reader for the byte range [offset, offset+length). The caller closes the reader.
type FetchChunk func(offset, length int64) (io.ReadCloser, error)

type byteRange struct {
	offset, length int64
}

func normalizeParallelDownloadInputs(contentLength int64, chunkSize int64, concurrency int) (int64, int, error) {
	if chunkSize <= 0 {
		return 0, 0, fmt.Errorf("transfer: chunk size must be positive")
	}
	if chunkSize > contentLength {
		chunkSize = contentLength
	}
	if concurrency < 1 {
		concurrency = 1
	}
	return chunkSize, concurrency, nil
}

func parallelDownloadRanges(contentLength, chunkSize int64) []byteRange {
	var chunks []byteRange
	for offset := int64(0); offset < contentLength; offset += chunkSize {
		length := chunkSize
		if offset+length > contentLength {
			length = contentLength - offset
		}
		chunks = append(chunks, byteRange{offset, length})
	}
	return chunks
}

type parallelRangeWorker struct {
	dest       io.WriterAt
	fetchChunk FetchChunk
	firstErr   error
	once       sync.Once
}

func (w *parallelRangeWorker) recordFirstErr(err error) {
	w.once.Do(func() { w.firstErr = err })
}

func (w *parallelRangeWorker) downloadChunk(offset, length int64) {
	reader, err := w.fetchChunk(offset, length)
	if err != nil {
		w.recordFirstErr(err)
		return
	}
	defer func() { _ = reader.Close() }()

	chunkLen := int(length)
	if int64(chunkLen) != length {
		w.recordFirstErr(fmt.Errorf("chunk length overflows int: %d", length))
		return
	}
	buf := make([]byte, chunkLen)
	_, err = io.ReadFull(io.LimitReader(reader, length), buf)
	if err != nil {
		w.recordFirstErr(fmt.Errorf("chunk read at offset %d: %w", offset, err))
		return
	}
	n, err := w.dest.WriteAt(buf, offset)
	if err != nil {
		w.recordFirstErr(err)
		return
	}
	if int64(n) != length {
		w.recordFirstErr(fmt.Errorf("chunk write size mismatch at offset %d: wrote %d bytes, want %d", offset, n, length))
	}
}

// ParallelRangeDownload fetches content in parallel via range requests and writes each chunk at its
// byte offset using dest.WriteAt. Memory use stays on the order of concurrency×chunkSize because a
// chunk buffer is released as soon as it is written, unlike a full-file reordering buffer.
// Each chunk read is capped with io.LimitReader so a server that ignores Range length cannot cause
// unbounded buffering; io.ReadFull requires exactly length bytes (short reads fail).
//
// dest must support concurrent non-overlapping WriteAt calls (for example *os.File on Unix).
// fetchChunk is called for each chunk; the caller closes each returned reader. dest is never closed.
// chunkSize must be positive (callers that treat 0 as "default" must substitute a default before calling).
// concurrency is raised to at least 1 if lower.
func ParallelRangeDownload(contentLength, chunkSize int64, concurrency int, dest io.WriterAt, fetchChunk FetchChunk) error {
	chunkSize, concurrency, err := normalizeParallelDownloadInputs(contentLength, chunkSize, concurrency)
	if err != nil {
		return err
	}
	chunks := parallelDownloadRanges(contentLength, chunkSize)

	worker := &parallelRangeWorker{dest: dest, fetchChunk: fetchChunk}
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for _, cnk := range chunks {
		wg.Add(1)
		sem <- struct{}{}
		go func(offset, length int64) {
			defer wg.Done()
			defer func() { <-sem }()
			worker.downloadChunk(offset, length)
		}(cnk.offset, cnk.length)
	}
	wg.Wait()
	return worker.firstErr
}
