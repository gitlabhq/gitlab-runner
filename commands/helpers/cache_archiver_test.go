//go:build !integration

package helpers

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
)

func TestUploadExistingArchiveIfNeeded(t *testing.T) {
	tests := map[string]struct {
		setupFile       bool
		provideCheckURL bool
		headStatus      int
		expectUpload    bool
	}{
		"local file missing": {
			setupFile:    false,
			expectUpload: false,
		},
		"file exists, remote exists": {
			setupFile:       true,
			provideCheckURL: true,
			headStatus:      http.StatusOK,
			expectUpload:    false,
		},
		"file exists, remote missing": {
			setupFile:       true,
			provideCheckURL: true,
			headStatus:      http.StatusNotFound,
			expectUpload:    true,
		},
		"file exists, no check URL": {
			setupFile:    true,
			expectUpload: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			tmpDir := t.TempDir()
			primaryFile := filepath.Join(tmpDir, "cache.zip")

			uploaded := false
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodHead:
					w.WriteHeader(tc.headStatus)
				case http.MethodPut:
					uploaded = true
					w.WriteHeader(http.StatusOK)
				}
			}))
			defer srv.Close()

			if tc.setupFile {
				require.NoError(t, os.WriteFile(primaryFile, []byte("cache content"), 0o600))
			}

			cmd := &CacheArchiverCommand{
				File: primaryFile,
				URL:  srv.URL + "/upload",
			}
			if tc.provideCheckURL {
				cmd.CheckURL = srv.URL + "/check"
			}

			cmd.uploadExistingArchiveIfNeeded()

			assert.Equal(t, tc.expectUpload, uploaded)
		})
	}
}

func TestTryRenameAlternateFile(t *testing.T) {
	tests := map[string]struct {
		setupAlternate  bool
		setupPrimary    bool
		noAlternateSet  bool // pass empty string as AlternateFile
		sameAsPrimary   bool // AlternateFile == File
		primaryInSubdir bool // primary lives in a subdirectory that doesn't exist yet
		expectRename    bool
	}{
		"no alternate file set": {
			noAlternateSet: true,
			expectRename:   false,
		},
		"alternate same as primary": {
			sameAsPrimary: true,
			expectRename:  false,
		},
		"primary exists, alternate exists": {
			setupPrimary:   true,
			setupAlternate: true,
			expectRename:   false,
		},
		"primary missing, alternate missing": {
			setupAlternate: false,
			expectRename:   false,
		},
		"primary missing, alternate exists": {
			setupAlternate: true,
			expectRename:   true,
		},
		"primary missing, alternate exists, primary dir missing": {
			setupAlternate:  true,
			primaryInSubdir: true,
			expectRename:    true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			tmpDir := t.TempDir()

			primaryFile := filepath.Join(tmpDir, "cache.zip")
			if tc.primaryInSubdir {
				primaryFile = filepath.Join(tmpDir, "newsubdir", "cache.zip")
			}

			alternateFile := filepath.Join(tmpDir, "old-cache.zip")
			switch {
			case tc.noAlternateSet:
				alternateFile = ""
			case tc.sameAsPrimary:
				alternateFile = primaryFile
			}

			if tc.setupPrimary {
				require.NoError(t, os.WriteFile(primaryFile, []byte("primary"), 0o600))
			}
			if tc.setupAlternate {
				require.NoError(t, os.WriteFile(alternateFile, []byte("alternate"), 0o600))
			}

			cmd := &CacheArchiverCommand{
				File:          primaryFile,
				AlternateFile: alternateFile,
			}
			cmd.tryRenameAlternateFile()

			if tc.expectRename {
				assert.FileExists(t, primaryFile, "primary file should exist after rename")
				assert.NoFileExists(t, alternateFile, "alternate file should be gone after rename")

				content, err := os.ReadFile(primaryFile)
				require.NoError(t, err)
				assert.Equal(t, "alternate", string(content), "primary file should contain former alternate content")
			} else {
				if tc.setupPrimary {
					content, err := os.ReadFile(primaryFile)
					require.NoError(t, err)
					assert.Equal(t, "primary", string(content), "primary file should be unchanged")
				}
				if tc.setupAlternate && alternateFile != primaryFile {
					assert.FileExists(t, alternateFile, "alternate file should be untouched")
				}
			}
		})
	}
}

// newTestCacheArchiver returns a CacheArchiverCommand wired to archive a single
// compressible file, plus the target archive path. CompressionLevel is left at
// the default (not "fastest") so the zstd method is actually exercised rather
// than falling back to Store.
func newTestCacheArchiver(t *testing.T, format string) (*CacheArchiverCommand, string) {
	t.Helper()

	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "data.txt"),
		bytes.Repeat([]byte("gitlab runner cache "), 1024), 0o644))

	// Mirror production: the archiver runs from the working directory and the
	// files map is keyed by paths relative to it (fileArchiver.add stores the
	// path as given, and the archiver resolves it against the current dir).
	t.Chdir(srcDir)
	fi, err := os.Stat("data.txt")
	require.NoError(t, err)

	cmd := &CacheArchiverCommand{CompressionFormat: format}
	cmd.wd = srcDir
	cmd.files = map[string]os.FileInfo{"data.txt": fi}

	return cmd, filepath.Join(t.TempDir(), "cache.zip")
}

// TestCacheArchiverCompressionFormatSelection guards the CACHE_COMPRESSION_FORMAT
// switch in createZipFile. The normalized c.CompressionFormat is exactly the
// archive.Format handed to archive.NewArchiver, so asserting it pins which
// archiver actually runs. zipzstd previously fell through to the default (zip),
// so this test exists specifically to stop that regression recurring.
func TestCacheArchiverCompressionFormatSelection(t *testing.T) {
	tests := map[string]struct {
		input    string
		expected spec.ArtifactFormat
	}{
		"zipzstd lowercase":    {"zipzstd", spec.ArtifactFormatZipZstd},
		"zipzstd mixed case":   {"ZipZstd", spec.ArtifactFormatZipZstd},
		"zipzstd upper case":   {"ZIPZSTD", spec.ArtifactFormatZipZstd},
		"tarzstd":              {"tarzstd", spec.ArtifactFormatTarZstd},
		"tarzstd mixed case":   {"TarZstd", spec.ArtifactFormatTarZstd},
		"zip":                  {"zip", spec.ArtifactFormatZip},
		"empty defaults zip":   {"", spec.ArtifactFormatZip},
		"unknown defaults zip": {"bogus", spec.ArtifactFormatZip},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			cmd, archiveFile := newTestCacheArchiver(t, tc.input)

			_, err := cmd.createZipFile(archiveFile)
			require.NoError(t, err)

			assert.Equal(t, string(tc.expected), cmd.CompressionFormat,
				"CACHE_COMPRESSION_FORMAT %q must select archive format %q", tc.input, tc.expected)
		})
	}
}

// TestCacheArchiverZipZstdProducesZstdZip asserts the produced archive really is
// ZIP+Zstandard — a ZIP container whose entries use the zstd method — and not a
// plain deflate zip or a tarzstd stream.
func TestCacheArchiverZipZstdProducesZstdZip(t *testing.T) {
	cmd, archiveFile := newTestCacheArchiver(t, "zipzstd")

	_, err := cmd.createZipFile(archiveFile)
	require.NoError(t, err)
	require.Equal(t, string(spec.ArtifactFormatZipZstd), cmd.CompressionFormat)

	// ZIP container: starts with the local file header magic. A tarzstd archive
	// would instead start with the zstd frame magic (0x28 0xB5 0x2F 0xFD).
	data, err := os.ReadFile(archiveFile)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(data), 4)
	assert.Equal(t, []byte("PK\x03\x04"), data[:4], "expected a ZIP container")

	// Every file entry must use the zstd method — this is what distinguishes
	// zipzstd from a plain deflate zip.
	zr, err := zip.OpenReader(archiveFile)
	require.NoError(t, err)
	defer zr.Close()

	checked := 0
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		checked++
		assert.Equalf(t, uint16(zstd.ZipMethodWinZip), f.Method,
			"entry %q should use the zstd method (%d), got %d", f.Name, uint16(zstd.ZipMethodWinZip), f.Method)
	}
	require.Positive(t, checked, "expected at least one file entry in the archive")
}
