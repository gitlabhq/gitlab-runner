//go:build !integration

package archive_test

import (
	"archive/tar"
	"bytes"
	"io"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/klauspost/compress/zip"
	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive"

	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive/fastzip"
	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive/gziplegacy"
	_ "gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive/raw"
	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive/tarzstd"
	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive/ziplegacy"
)

func TestDefaultRegistration(t *testing.T) {
	tests := map[archive.Format]struct {
		hasArchiver, hasExtractor bool
	}{
		archive.Raw:     {hasArchiver: true, hasExtractor: false},
		archive.Gzip:    {hasArchiver: true, hasExtractor: false},
		archive.Zip:     {hasArchiver: true, hasExtractor: true},
		archive.ZipZstd: {hasArchiver: true, hasExtractor: true},
		archive.TarZstd: {hasArchiver: true, hasExtractor: true},
	}

	for tn, tc := range tests {
		t.Run(string(tn), func(t *testing.T) {
			_, err := archive.NewArchiver(tn, nil, "", archive.DefaultCompression)

			if tc.hasArchiver {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, archive.ErrUnsupportedArchiveFormat)
			}

			_, err = archive.NewExtractor(tn, nil, 0, "")

			if tc.hasExtractor {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, archive.ErrUnsupportedArchiveFormat)
			}
		})
	}
}

func TestRegister(t *testing.T) {
	format := archive.Format("new-format")

	archive.Register(format, ziplegacy.NewArchiver, ziplegacy.NewExtractor)

	_, err := archive.NewArchiver(format, nil, "", archive.DefaultCompression)
	assert.NoError(t, err)

	_, err = archive.NewExtractor(format, nil, 0, "")
	assert.NoError(t, err)
}

func TestRegisterOverride(t *testing.T) {
	existingGzipArchiver, err := gziplegacy.NewArchiver(io.Discard, "", archive.DefaultCompression)
	assert.NoError(t, err)

	existingZipArchiver, err := ziplegacy.NewArchiver(io.Discard, "", archive.DefaultCompression)
	assert.NoError(t, err)

	existingZipExtractor, err := ziplegacy.NewExtractor(nil, 0, "")
	assert.NoError(t, err)

	// assert existing archiver
	archiver, err := archive.NewArchiver(archive.Gzip, nil, "", archive.DefaultCompression)
	assert.NoError(t, err)
	assert.IsType(t, existingGzipArchiver, archiver)

	_, err = archive.NewExtractor(archive.Gzip, nil, 0, "")
	assert.Error(t, err)

	// override
	archive.Register(archive.Gzip, ziplegacy.NewArchiver, ziplegacy.NewExtractor)

	archiver, err = archive.NewArchiver(archive.Gzip, nil, "", archive.DefaultCompression)
	assert.NoError(t, err)
	assert.IsType(t, existingZipArchiver, archiver)

	extractor, err := archive.NewExtractor(archive.Gzip, nil, 0, "")
	assert.NoError(t, err)
	assert.IsType(t, existingZipExtractor, extractor)
}

type entry struct {
	name  string
	link  string
	mtime time.Time
}

func (e entry) mode() os.FileMode {
	switch {
	case e.link != "":
		return os.ModeSymlink | 0o777
	case strings.HasSuffix(e.name, "/"):
		return os.ModeDir | 0o777
	default:
		return 0o644
	}
}

func buildTarZst(t *testing.T, entries []entry) []byte {
	var buf bytes.Buffer
	zw, err := zstd.NewWriter(&buf)
	require.NoError(t, err)
	tw := tar.NewWriter(zw)
	for _, e := range entries {
		hdr := &tar.Header{Name: e.name, Typeflag: tar.TypeReg, Linkname: e.link, Mode: int64(e.mode().Perm()), ModTime: e.mtime}
		switch {
		case e.link != "":
			hdr.Typeflag = tar.TypeSymlink
		case e.mode().IsDir():
			hdr.Typeflag = tar.TypeDir
		}
		require.NoError(t, tw.WriteHeader(hdr))
	}
	require.NoError(t, tw.Close())
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func buildZip(t *testing.T, entries []entry) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		fh := &zip.FileHeader{Name: e.name, Modified: e.mtime}
		fh.SetMode(e.mode())
		w, err := zw.CreateHeader(fh)
		require.NoError(t, err)
		_, err = io.WriteString(w, e.link)
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

var extractors = map[string]struct {
	build func(*testing.T, []entry) []byte
	new   func(io.ReaderAt, int64, string) (archive.Extractor, error)
}{
	"tarzstd": {build: buildTarZst, new: tarzstd.NewExtractor},
	"fastzip": {build: buildZip, new: fastzip.NewExtractor},
}

func TestExtractSymlinkContainment(t *testing.T) {
	tests := map[string]struct {
		// "OUTSIDE" in a link is replaced by the outside directory.
		onDisk  map[string]string
		entries []entry
		// nil wantErr: only containment is asserted.
		wantErr assert.ErrorAssertionFunc
		want    string
	}{
		"file through escaping symlink on disk": {
			onDisk:  map[string]string{"escape": "OUTSIDE"},
			entries: []entry{{name: "escape/pwned"}},
			wantErr: assert.Error,
		},
		"file over escaping symlink on disk": {
			onDisk:  map[string]string{"victim": "OUTSIDE/victim"},
			entries: []entry{{name: "victim"}},
		},
		"file over dangling escaping symlink on disk": {
			onDisk:  map[string]string{"pwned": "OUTSIDE/pwned"},
			entries: []entry{{name: "pwned"}},
		},
		"nested file over escaping symlink on disk": {
			onDisk:  map[string]string{"dir/": "", "dir/victim": "OUTSIDE/victim"},
			entries: []entry{{name: "dir/"}, {name: "dir/victim"}},
		},
		// p only resolves once x exists, and then resolves to the parent of
		// the root.
		"relative symlink on disk resolved through a directory in archive": {
			onDisk:  map[string]string{"p": "x/../.."},
			entries: []entry{{name: "x/"}, {name: "p/pwned"}},
			wantErr: assert.Error,
		},
		"relative symlink on disk resolved through a later directory in archive": {
			onDisk:  map[string]string{"p": "x/../.."},
			entries: []entry{{name: "p/pwned"}, {name: "x/"}},
			wantErr: assert.Error,
		},
		"directory over escaping symlink on disk": {
			onDisk:  map[string]string{"escape": "OUTSIDE"},
			entries: []entry{{name: "escape/"}},
			wantErr: assert.Error,
		},
		"symlink under symlink in archive": {
			entries: []entry{{name: "s", link: "OUTSIDE"}, {name: "s/victim", link: "planted"}},
			wantErr: assert.Error,
		},
		"symlink under differently cased symlink in archive": {
			entries: []entry{{name: "S", link: "OUTSIDE"}, {name: "s/victim", link: "planted"}},
		},
		"symlink under differently normalized symlink in archive": {
			entries: []entry{{name: "é", link: "OUTSIDE"}, {name: "é/victim", link: "planted"}},
		},
		"symlink under in-root symlink on disk replaced in archive": {
			onDisk:  map[string]string{"real/": "", "s": "real"},
			entries: []entry{{name: "s", link: "OUTSIDE"}, {name: "s/victim", link: "planted"}},
		},
		"directory and symlink with the same name in archive": {
			entries: []entry{{name: "d/"}, {name: "d", link: "OUTSIDE"}},
		},
		"file through in-root symlink on disk": {
			onDisk:  map[string]string{"real/": "", "inroot": "real"},
			entries: []entry{{name: "inroot/file"}},
			wantErr: assert.NoError,
			want:    "real/file",
		},
		"outward symlink over existing outward symlink": {
			onDisk:  map[string]string{"venv/": "", "venv/python": "OUTSIDE/victim"},
			entries: []entry{{name: "venv/"}, {name: "venv/python", link: "OUTSIDE/victim"}},
			// tarzstd has never replaced existing symlinks and fails with EEXIST.
			want: "venv/python",
		},
		"leading slash extracts inside root": {
			entries: []entry{{name: "/abs/file"}},
			wantErr: assert.NoError,
			want:    "abs/file",
		},
	}

	for extractorName, ex := range extractors {
		for name, tc := range tests {
			t.Run(extractorName+"/"+name, func(t *testing.T) {
				// The root is inside the outside directory, so that relative
				// links leading out of the root also end up where it's checked.
				outside := t.TempDir()
				root := filepath.Join(outside, "root")
				require.NoError(t, os.Mkdir(root, 0o777))
				require.NoError(t, os.Chmod(outside, 0o700))
				before, err := os.Stat(outside)
				require.NoError(t, err)
				victim := filepath.Join(outside, "victim")
				require.NoError(t, os.WriteFile(victim, []byte("original"), 0o600))
				expand := func(s string) string {
					if rest, ok := strings.CutPrefix(s, "OUTSIDE"); ok {
						return outside + filepath.FromSlash(rest)
					}
					return s
				}

				// Directories first, so that on Windows symlinks to them are
				// created as directory symlinks.
				keys := slices.Sorted(maps.Keys(tc.onDisk))
				for _, p := range keys {
					if strings.HasSuffix(p, "/") {
						require.NoError(t, os.MkdirAll(filepath.Join(root, p), 0o777))
					}
				}
				for _, p := range keys {
					if !strings.HasSuffix(p, "/") {
						require.NoError(t, os.Symlink(expand(tc.onDisk[p]), filepath.Join(root, p)))
					}
				}

				entries := make([]entry, len(tc.entries))
				for i, e := range tc.entries {
					entries[i] = entry{name: e.name, link: expand(e.link)}
				}

				data := ex.build(t, entries)
				e, err := ex.new(bytes.NewReader(data), int64(len(data)), root)
				require.NoError(t, err)
				err = e.Extract(t.Context())
				if tc.wantErr != nil {
					tc.wantErr(t, err)
				}

				assert.NoFileExists(t, filepath.Join(outside, "pwned"))
				fi, err := os.Stat(outside)
				require.NoError(t, err)
				assert.Equal(t, before.Mode().Perm(), fi.Mode().Perm(), "mode of directory outside root changed")
				got, err := os.ReadFile(victim)
				require.NoError(t, err, "file outside root was replaced")
				assert.Equal(t, "original", string(got))

				if tc.want != "" {
					_, err := os.Lstat(filepath.Join(root, tc.want))
					assert.NoError(t, err)
				}
			})
		}
	}
}

func TestExtractKeepsTimesAroundSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink times are not set on Windows")
	}

	dirTime := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	linkTime := time.Date(2021, 1, 2, 3, 4, 5, 0, time.UTC)

	// Enough directories with symlinks inside them that restoring times in
	// the wrong order is caught regardless of iteration order.
	entries := []entry{{name: "./", mtime: dirTime}}
	for _, dir := range []string{"a/", "a/b/", "c/", "d/", "e/", "f/", "g/", "h/"} {
		entries = append(entries,
			entry{name: dir, mtime: dirTime},
			entry{name: dir + "python", link: "/usr/bin/python3", mtime: linkTime},
		)
	}

	for name, ex := range extractors {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			data := ex.build(t, entries)
			e, err := ex.new(bytes.NewReader(data), int64(len(data)), root)
			require.NoError(t, err)
			require.NoError(t, e.Extract(t.Context()))

			for _, en := range entries {
				fi, err := os.Lstat(filepath.Join(root, en.name))
				require.NoError(t, err)
				assert.True(t, fi.ModTime().Equal(en.mtime), "%s mtime %v, want %v", en.name, fi.ModTime(), en.mtime)
			}
		})
	}
}

func TestFastzipReplacesDirectoryWithSymlink(t *testing.T) {
	tests := map[string]struct {
		onDisk  []string
		wantErr bool
	}{
		"empty directory": {
			onDisk: []string{"node_modules/.bin/"},
		},
		"non-empty directory": {
			onDisk:  []string{"node_modules/.bin/", "node_modules/.bin/tool"},
			wantErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			for _, p := range tc.onDisk {
				if strings.HasSuffix(p, "/") {
					require.NoError(t, os.MkdirAll(filepath.Join(root, p), 0o777))
				} else {
					require.NoError(t, os.WriteFile(filepath.Join(root, p), nil, 0o600))
				}
			}

			data := buildZip(t, []entry{{name: "node_modules/.bin", link: "../lib"}})
			e, err := fastzip.NewExtractor(bytes.NewReader(data), int64(len(data)), root)
			require.NoError(t, err)
			err = e.Extract(t.Context())

			path := filepath.Join(root, "node_modules", ".bin")
			if tc.wantErr {
				assert.Error(t, err)
				fi, err := os.Lstat(path)
				require.NoError(t, err)
				assert.True(t, fi.IsDir(), "non-empty directory was replaced")
				return
			}

			require.NoError(t, err)
			target, err := os.Readlink(path)
			require.NoError(t, err)
			// The target is stored as given, also on Windows.
			assert.Equal(t, "../lib", target)
		})
	}
}
