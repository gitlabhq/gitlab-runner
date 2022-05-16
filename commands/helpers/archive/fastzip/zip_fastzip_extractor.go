package fastzip

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/saracen/fastzip"

	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive"
)

const (
	extractorConcurrency = "FASTZIP_EXTRACTOR_CONCURRENCY"
)

// extractor is a zip stream extractor.
type extractor struct {
	r    io.ReaderAt
	size int64
	dir  string
}

// NewExtractor returns a new Zip Extractor.
func NewExtractor(r io.ReaderAt, size int64, dir string) (archive.Extractor, error) {
	return &extractor{r: r, size: size, dir: dir}, nil
}

// Extract extracts files from the reader to the directory passed to
// NewExtractor.
func (e *extractor) Extract(ctx context.Context) error {
	opts, err := getExtractorOptionsFromEnvironment()
	if err != nil {
		return err
	}

	extractor, err := fastzip.NewExtractorFromReader(e.r, e.size, e.dir, opts...)
	if err != nil {
		return err
	}
	defer extractor.Close()

	return extractor.Extract(ctx)
}

func getExtractorOptionsFromEnvironment() ([]fastzip.ExtractorOption, error) {
	var opts []fastzip.ExtractorOption

	if val := os.Getenv(extractorConcurrency); val != "" {
		concurrency, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("fastzip extractor concurrency: %w", err)
		}

		opts = append(opts, fastzip.WithExtractorConcurrency(int(concurrency)))
	}

	return opts, nil
}
