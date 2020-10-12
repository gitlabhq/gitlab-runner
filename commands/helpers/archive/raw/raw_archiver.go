package raw

import (
	"context"
	"errors"
	"io"
	"os"

	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive"
)

func init() {
	archive.Register(archive.Raw, NewArchiver, nil)
}

// ErrTooManyRawFiles is returned if more than one file is passed to the
// RawArchiver.
var ErrTooManyRawFiles = errors.New("only one file can be sent as raw")

// archiver is a raw archiver. It doesn't support compression nor multiple
// files.
type archiver struct {
	w   io.Writer
	dir string
}

// NewArchiver returns a new Raw Archiver.
func NewArchiver(w io.Writer, dir string, level archive.CompressionLevel) (archive.Archiver, error) {
	return &archiver{w: w, dir: dir}, nil
}

// Archive opens and copies a single file to the writer passed to
// NewRawArchiver. If more than one file is passed, ErrTooManyRawFiles is
// returned.
func (a *archiver) Archive(ctx context.Context, files map[string]os.FileInfo) error {
	if len(files) > 1 {
		return ErrTooManyRawFiles
	}

	for pathname := range files {
		f, err := os.Open(pathname)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(a.w, f)
		return err
	}

	return nil
}
