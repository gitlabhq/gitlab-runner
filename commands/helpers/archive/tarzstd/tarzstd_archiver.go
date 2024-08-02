package tarzstd

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/klauspost/compress/zstd"
	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive"
)

func init() {
	archive.Register(archive.TarZstd, NewArchiver, NewExtractor)
}

const irregularModes = os.ModeSocket | os.ModeDevice | os.ModeCharDevice | os.ModeNamedPipe

var levels = map[archive.CompressionLevel]int{
	archive.FastestCompression: int(zstd.SpeedFastest),
	archive.FastCompression:    int(zstd.SpeedFastest),
	archive.DefaultCompression: int(zstd.SpeedDefault),
	archive.SlowCompression:    int(zstd.SpeedBetterCompression),
	archive.SlowestCompression: int(zstd.SpeedBestCompression),
}

// archiver is a tar+zstd stream archiver.
type archiver struct {
	w     io.Writer
	dir   string
	level archive.CompressionLevel
}

// NewArchiver returns a new Tar+zstd Archiver.
func NewArchiver(w io.Writer, dir string, level archive.CompressionLevel) (archive.Archiver, error) {
	return &archiver{w: w, dir: dir, level: level}, nil
}

// Archive archives all files.
//
//nolint:gocognit
func (a *archiver) Archive(ctx context.Context, files map[string]os.FileInfo) error {
	sorted := make([]string, 0, len(files))
	for filename := range files {
		sorted = append(sorted, filename)
	}
	sort.Strings(sorted)

	zw, err := zstd.NewWriter(a.w, zstd.WithEncoderLevel(zstd.EncoderLevel(levels[a.level])))
	if err != nil {
		return err
	}
	defer zw.Close()

	tw := tar.NewWriter(zw)
	defer tw.Close()

	for _, name := range sorted {
		fi := files[name]
		if fi.Mode()&irregularModes != 0 {
			continue
		}

		path, err := filepath.Abs(name)
		if err != nil {
			return err
		}
		if !strings.HasPrefix(path, a.dir+string(filepath.Separator)) && path != a.dir {
			return fmt.Errorf("%s cannot be archived from outside of chroot (%s)", name, a.dir)
		}

		rel, err := filepath.Rel(a.dir, path)
		if err != nil {
			return err
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}

		var link string
		if fi.Mode()&os.ModeSymlink != 0 {
			link, err = os.Readlink(path)
			if err != nil {
				return err
			}
		}

		hdr, err := tar.FileInfoHeader(fi, link)
		if err != nil {
			return err
		}
		hdr.Name = rel
		if fi.IsDir() {
			hdr.Name += "/"
		}

		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}

		if !fi.Mode().IsRegular() {
			continue
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}

		if _, err = io.Copy(tw, f); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}

	if err := tw.Close(); err != nil {
		return err
	}

	return zw.Close()
}
