//go:build !integration

package fastzip

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive"
)

func TestOptionFromEnvValidation(t *testing.T) {
	t.Run("archiver", func(t *testing.T) {
		for _, option := range []string{archiverBufferSize, archiverConcurrency} {
			defer tempEnvOption(option, "invalid")()

			_, err := getArchiverOptionsFromEnvironment()
			assert.Error(t, err)
		}
	})

	t.Run("extractor", func(t *testing.T) {
		for _, option := range []string{extractorConcurrency} {
			defer tempEnvOption(option, "invalid")()

			_, err := getExtractorOptionsFromEnvironment()
			assert.Error(t, err)
		}
	})
}

func TestArchiverOptionFromEnv(t *testing.T) {
	tests := map[string]struct {
		value string
		err   string
	}{
		archiverStagingDir:  {"/dev/null", "fastzip archiver unable to create temporary directory"},
		archiverConcurrency: {"-1", "concurrency must be at least 1"},
	}

	for option, tc := range tests {
		t.Run(fmt.Sprintf("%s=%s", option, tc.value), func(t *testing.T) {
			defer tempEnvOption(option, tc.value)()

			archiveTestDir(t, func(_ string, _ string, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.err)
			})
		})
	}
}

func TestExtractorOptionFromEnv(t *testing.T) {
	tests := map[string]struct {
		value string
		err   string
	}{
		extractorConcurrency: {"-1", "concurrency must be at least 1"},
	}

	for option, tc := range tests {
		t.Run(fmt.Sprintf("%s=%s", option, tc.value), func(t *testing.T) {
			defer tempEnvOption(option, tc.value)()

			archiveTestDir(t, func(archiveFile string, dir string, err error) {
				require.NoError(t, err)

				f, err := os.Open(archiveFile)
				require.NoError(t, err)
				defer f.Close()

				fi, err := f.Stat()
				require.NoError(t, err)

				extractor, err := NewExtractor(f, fi.Size(), dir)
				require.NoError(t, err)

				err = extractor.Extract(context.Background())
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.err)
			})
		})
	}
}

func archiveTestDir(t *testing.T, fn func(string, string, error)) {
	dir := t.TempDir()

	pathname := filepath.Join(dir, "test_file")
	require.NoError(t, os.WriteFile(pathname, []byte("foobar"), 0o777))
	fi, err := os.Stat(pathname)
	require.NoError(t, err)

	f, err := os.CreateTemp(dir, "fastzip")
	require.NoError(t, err)
	defer f.Close()

	archiver, err := NewArchiver(f, dir, archive.DefaultCompression)
	require.NoError(t, err)

	err = archiver.Archive(context.Background(), map[string]os.FileInfo{pathname: fi})
	require.NoError(t, f.Close())

	fn(f.Name(), dir, err)
}

func tempEnvOption(option, value string) func() {
	existing := os.Getenv(option)
	os.Setenv(option, value)

	return func() {
		os.Setenv(option, existing)
	}
}
