package fastzip

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/klauspost/compress/zip"
	"github.com/saracen/fastzip"
	"github.com/saracen/zipextra"

	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive"
)

const (
	extractorConcurrency = "FASTZIP_EXTRACTOR_CONCURRENCY"
)

// irregularModes matches the entries fastzip skips.
const irregularModes = os.ModeSocket | os.ModeDevice | os.ModeCharDevice | os.ModeNamedPipe

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
//
// fastzip's file operations follow symlinks, so symlinks are created here
// through os.Root after fastzip has extracted everything else.
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

	dir, err := filepath.Abs(e.dir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0777); err != nil {
		return err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()

	if err := checkContainment(root, dir, extractor.Files()); err != nil {
		return err
	}

	symlinks, err := deferSymlinks(extractor)
	if err != nil {
		return err
	}

	if err := extractor.Extract(ctx); err != nil {
		return err
	}

	for _, file := range symlinks {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := createSymlink(root, dir, file); err != nil {
			return err
		}
	}

	if len(symlinks) == 0 {
		return nil
	}

	// Creating symlinks updated their parent directories' times, which
	// fastzip had already restored.
	return restoreDirTimes(root, dir, extractor.Files())
}

func restoreDirTimes(root *os.Root, dir string, files []*zip.File) error {
	for _, file := range files {
		if !file.Mode().IsDir() {
			continue
		}

		path, err := relPath(dir, file.Name)
		if err != nil {
			return err
		}

		if err := root.Chtimes(path, time.Now(), file.Modified); err != nil {
			return err
		}
	}

	return nil
}

// checkContainment rejects entries that fastzip would write through a symlink
// on disk that escapes dir, and creates the directories fastzip writes into so
// that every one of them is a real directory inside dir.
func checkContainment(root *os.Root, dir string, files []*zip.File) error {
	for _, file := range files {
		if file.Mode()&(irregularModes|os.ModeSymlink) != 0 {
			continue
		}

		path, err := relPath(dir, file.Name)
		if err != nil {
			return err
		}

		// Directory metadata is applied by path, which follows a symlink.
		check := filepath.Dir(path)
		if file.Mode().IsDir() {
			check = path
		}

		// Create the directory through the root now: a symlink that doesn't
		// resolve yet could resolve outside dir once fastzip creates other
		// directories, so a missing path can't be treated as safe.
		if err := root.MkdirAll(check, 0777); err != nil {
			return fmt.Errorf("%s cannot be extracted: %w", file.Name, err)
		}
	}

	return nil
}

// deferSymlinks hides symlink entries from fastzip by marking them irregular,
// which fastzip skips, and returns the original entries.
func deferSymlinks(extractor *fastzip.Extractor) ([]*zip.File, error) {
	var symlinks []*zip.File

	files := extractor.Files()
	for i, file := range files {
		if file.Mode()&os.ModeSymlink == 0 {
			continue
		}

		hidden := *file
		hidden.SetMode(os.ModeSocket)
		files[i] = &hidden
		symlinks = append(symlinks, file)

		// Fail closed if fastzip stops exposing its own file list.
		if extractor.Files()[i] != &hidden {
			return nil, errors.New("fastzip: cannot defer symlink creation")
		}
	}

	return symlinks, nil
}

func createSymlink(root *os.Root, dir string, file *zip.File) error {
	path, err := relPath(dir, file.Name)
	if err != nil {
		return err
	}

	if err := root.MkdirAll(filepath.Dir(path), 0777); err != nil {
		return err
	}

	// Like fastzip, replace whatever is there; Remove fails for a non-empty
	// directory.
	if err := root.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	r, err := file.Open()
	if err != nil {
		return err
	}
	defer r.Close()

	target, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	if err := root.Symlink(string(target), path); err != nil {
		return err
	}

	return updateSymlinkMetadata(filepath.Join(dir, path), file)
}

// updateSymlinkMetadata applies the archived times and owner to a symlink, as
// fastzip does. Like fastzip without a chown error handler, a failed chown is
// ignored.
func updateSymlinkMetadata(path string, file *zip.File) error {
	if err := lchtimes(path, file.Modified); err != nil {
		return err
	}

	fields, err := zipextra.Parse(file.Extra)
	if err != nil {
		return err
	}

	field, ok := fields[zipextra.ExtraFieldUnixN]
	if !ok {
		return nil
	}

	owner, err := field.InfoZIPNewUnix()
	if err != nil {
		return err
	}

	_ = lchown(path, int(owner.Uid.Int64()), int(owner.Gid.Int64()))

	return nil
}

// relPath resolves name the way fastzip does and returns it relative to dir.
func relPath(dir, name string) (string, error) {
	path, err := filepath.Rel(dir, filepath.Join(dir, name))
	if err != nil {
		return "", err
	}

	if path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s cannot be extracted outside of chroot (%s)", name, dir)
	}

	return path, nil
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
