package ziplegacy

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"sort"

	"github.com/klauspost/compress/zstd"
	"github.com/saracen/fastzip"
	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/archives"
)

func init() {
	zip.RegisterDecompressor(zstd.ZipMethodWinZip, fastzip.ZstdDecompressor())

	archive.Register(archive.Zip, NewArchiver, NewExtractor)
	archive.Register(archive.ZipZstd, nil, NewExtractor)
}

// archiver is a zip stream archiver.
type archiver struct {
	w   io.Writer
	dir string
}

// NewArchiver returns a new Zip Archiver.
func NewArchiver(w io.Writer, dir string, level archive.CompressionLevel) (archive.Archiver, error) {
	return &archiver{w: w, dir: dir}, nil
}

// Archive archives all files as new gzip streams.
func (a *archiver) Archive(ctx context.Context, files map[string]os.FileInfo) error {
	sorted := make([]string, 0, len(files))
	for filename := range files {
		sorted = append(sorted, filename)
	}
	sort.Strings(sorted)

	return archives.CreateZipArchive(a.w, sorted)
}
