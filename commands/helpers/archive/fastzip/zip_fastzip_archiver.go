package fastzip

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/saracen/fastzip"

	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive"
)

var flateLevels = map[archive.CompressionLevel]int{
	archive.FastestCompression: 0,
	archive.FastCompression:    1,
	archive.DefaultCompression: 5,
	archive.SlowCompression:    7,
	archive.SlowestCompression: 9,
}

// archiver is a zip stream archiver.
type archiver struct {
	w     io.Writer
	dir   string
	level archive.CompressionLevel
}

// NewArchiver returns a new Zip Archiver.
func NewArchiver(w io.Writer, dir string, level archive.CompressionLevel) (archive.Archiver, error) {
	return &archiver{
		w:     w,
		dir:   dir,
		level: level,
	}, nil
}

// Archive archives all files provided.
func (a *archiver) Archive(ctx context.Context, files map[string]os.FileInfo) error {
	tmpDir, err := ioutil.TempDir("", "fastzip")
	if err != nil {
		return fmt.Errorf("fastzip archiver unable to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	opts := []fastzip.ArchiverOption{
		fastzip.WithStageDirectory(tmpDir),
	}
	if a.level == archive.FastestCompression {
		opts = append(opts, fastzip.WithArchiverMethod(zip.Store))
	}

	fa, err := fastzip.NewArchiver(a.w, a.dir, opts...)
	if err != nil {
		return err
	}

	if a.level != archive.FastestCompression {
		fa.RegisterCompressor(zip.Deflate, fastzip.FlateCompressor(flateLevels[a.level]))
	}

	err = fa.Archive(ctx, files)

	if cerr := fa.Close(); err == nil && cerr != nil {
		return cerr
	}

	return err
}
