package archive

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
)

var (
	// ErrUnsupportedArchiveFormat is returned if an archiver or extractor format
	// requested has not been registered.
	ErrUnsupportedArchiveFormat = errors.New("unsupported archive format")
)

// CompressionLevel type for specifying a compression level.
type CompressionLevel int

// Compression levels from fastest (low/zero compression ratio) to slowest
// (high compression ratio).
const (
	FastestCompression CompressionLevel = -2
	FastCompression    CompressionLevel = -1
	DefaultCompression CompressionLevel = 0
	SlowCompression    CompressionLevel = 1
	SlowestCompression CompressionLevel = 2
)

// Format type for specifying format.
type Format string

// Formats supported by GitLab.
const (
	Raw  Format = "raw"
	Gzip Format = "gzip"
	Zip  Format = "zip"
)

var (
	archivers  = make(map[Format]NewArchiverFunc)
	extractors = make(map[Format]NewExtractorFunc)
)

// Archiver is an interface for the Archive method.
//
//go:generate mockery --name=Archiver --inpackage
type Archiver interface {
	Archive(ctx context.Context, files map[string]os.FileInfo) error
}

// Extractor is an interface for the Extract method.
type Extractor interface {
	Extract(ctx context.Context) error
}

// NewArchiverFunc is a function that can be registered (with Register()) and
// used to instantiate a new archiver (with NewArchiver()).
type NewArchiverFunc func(w io.Writer, dir string, level CompressionLevel) (Archiver, error)

// NewExtractorFunc is a function that can be registered (with Register()) and
// used to instantiate a new extractor (with NewExtractor()).
type NewExtractorFunc func(r io.ReaderAt, size int64, dir string) (Extractor, error)

// Register registers a new archiver, overriding the archiver and/or extractor
// for the format provided.
func Register(
	format Format,
	archiver NewArchiverFunc,
	extractor NewExtractorFunc,
) (
	prevArchiver NewArchiverFunc,
	prevExtractor NewExtractorFunc,
) {
	if archiver != nil {
		prevArchiver = archivers[format]
		archivers[format] = archiver
	}
	if extractor != nil {
		prevExtractor = extractors[format]
		extractors[format] = extractor
	}
	return
}

// NewArchiver returns a new Archiver of the specified format.
//
// The archiver will ensure that files to be archived are children of the
// directory provided.
func NewArchiver(format Format, w io.Writer, dir string, level CompressionLevel) (Archiver, error) {
	fn := archivers[format]
	if fn == nil {
		return nil, fmt.Errorf("%q format: %w", format, ErrUnsupportedArchiveFormat)
	}

	return fn(w, dir, level)
}

// NewExtractor returns a new Extractor of the specified format.
//
// The extractor will extract files to the directory provided.
func NewExtractor(format Format, r io.ReaderAt, size int64, dir string) (Extractor, error) {
	fn := extractors[format]
	if fn == nil {
		return nil, fmt.Errorf("%q format: %w", format, ErrUnsupportedArchiveFormat)
	}

	return fn(r, size, dir)
}
