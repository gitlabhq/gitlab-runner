package tarzstd

import (
	"archive/tar"
	"cmp"
	"context"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
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

	if err := os.MkdirAll(e.dir, 0777); err != nil {
		return err
	}
	root, err := os.OpenRoot(e.dir)
	if err != nil {
		return err
	}
	defer root.Close()

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

		path, err := filepath.Rel(e.dir, filepath.Join(e.dir, hdr.Name))
		if err != nil {
			return err
		}

		if err := root.MkdirAll(filepath.Dir(path), 0777); err != nil {
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

			err := root.Mkdir(path, 0777)
			if err != nil && !os.IsExist(err) {
				return err
			}

		case fi.Mode().IsRegular():
			f, err := root.Create(path)
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

			if err := e.updateFileMetadata(filepath.Join(e.dir, path), hdr); err != nil {
				return err
			}
		}
	}

	// Deepest first: creating a symlink updates its parent directory's times,
	// and a parent is always shallower, so its metadata is applied afterwards.
	paths := slices.Collect(maps.Keys(deferred))
	slices.SortFunc(paths, func(a, b string) int {
		return cmp.Or(cmp.Compare(depth(b), depth(a)), strings.Compare(a, b))
	})

	for _, path := range paths {
		hdr := deferred[path]
		fi := hdr.FileInfo()
		if fi.Mode()&os.ModeSymlink == 0 && !fi.Mode().IsDir() {
			continue
		}

		if fi.Mode()&os.ModeSymlink != 0 {
			if err := root.Symlink(hdr.Linkname, path); err != nil {
				return err
			}
		}

		// Metadata is applied by path and follows a symlinked directory, so
		// refuse one that escapes the root.
		if fi.Mode().IsDir() {
			if _, err := root.Stat(path); err != nil {
				return err
			}
		}

		if err := e.updateFileMetadata(filepath.Join(e.dir, path), hdr); err != nil {
			return err
		}
	}

	return nil
}

// depth returns how many directories path is nested in, with the root itself
// shallowest. path must be clean and relative, as returned by filepath.Rel;
// otherwise elements such as "./" are counted and the depth is wrong.
func depth(path string) int {
	if path == "." {
		return -1
	}

	return strings.Count(path, string(filepath.Separator))
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
