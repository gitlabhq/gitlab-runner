package fastzip

import (
	"context"
	"io"

	"github.com/saracen/fastzip"

	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive"
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
	extractor, err := fastzip.NewExtractorFromReader(e.r, e.size, e.dir)
	if err != nil {
		return err
	}
	defer extractor.Close()

	return extractor.Extract(ctx)
}
