//go:build !integration

package helpers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
