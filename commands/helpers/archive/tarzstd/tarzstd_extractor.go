package tarzstd

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive"
)

// extractor is a tar+zstd stream extractor.
type extractor struct {
	r    io.ReaderAt
	size int64
	dir  string
}

// NewExtractor returns a new tar+zstd extractor.
func NewExtractor(r io.ReaderAt, size int64, dir string) (archive.Extractor, error) {
	return &extractor{r: r, size: size, dir: dir}, nil
}

// Extract extracts files from the reader to the directory passed to
// NewZipExtractor.
//
//nolint:gocognit
func (e *extractor) Extract(ctx context.Context) error {
	zr, err := zstd.NewReader(io.NewSectionReader(e.r, 0, e.size), zstd.WithDecoderLowmem(true))
	if err != nil {
		return err
	}
	defer zr.Close()

	tr := tar.NewReader(zr)

	deferred := map[string]*tar.Header{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		fi := hdr.FileInfo()
		if fi.Mode()&irregularModes != 0 {
			continue
		}

		var path string
		path, err = filepath.Abs(filepath.Join(e.dir, hdr.Name))
		if err != nil {
			return err
		}
		if !strings.HasPrefix(path, e.dir+string(filepath.Separator)) && path != e.dir {
			return fmt.Errorf("%s cannot be extracted outside of chroot (%s)", path, e.dir)
		}

		if err := os.MkdirAll(filepath.Dir(path), 0777); err != nil {
			return err
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}

		switch {
		case fi.Mode()&os.ModeSymlink != 0:
			deferred[path] = hdr
			continue

		case fi.Mode().IsDir():
			deferred[path] = hdr

			err := os.Mkdir(path, 0777)
			if err != nil && !os.IsExist(err) {
				return err
			}

		case fi.Mode().IsRegular():
			f, err := os.Create(path)
			if err != nil {
				return err
			}

			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}

			if err := e.updateFileMetadata(path, hdr); err != nil {
				return err
			}
		}
	}

	for path, hdr := range deferred {
		fi := hdr.FileInfo()
		if fi.Mode()&os.ModeSymlink == 0 && !fi.Mode().IsDir() {
			continue
		}

		if fi.Mode()&os.ModeSymlink != 0 {
			if err := os.Symlink(hdr.Linkname, path); err != nil {
				return err
			}
		}

		if err := e.updateFileMetadata(path, hdr); err != nil {
			return err
		}
	}

	return nil
}

func (e *extractor) updateFileMetadata(path string, hdr *tar.Header) error {
	fi := hdr.FileInfo()

	if err := lchtimes(path, fi.Mode(), time.Now(), fi.ModTime()); err != nil {
		return err
	}

	if err := lchmod(path, fi.Mode()); err != nil {
		return err
	}

	_ = lchown(path, hdr.Uid, hdr.Gid)
	return nil
}
