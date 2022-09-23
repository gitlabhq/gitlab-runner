//go:build !integration

package helpers

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive"
)

func TestCompressionLevel(t *testing.T) {
	tests := map[string]archive.CompressionLevel{
		"fastest": archive.FastestCompression,
		"fast":    archive.FastCompression,
		"slow":    archive.SlowCompression,
		"slowest": archive.SlowestCompression,
		"default": archive.DefaultCompression,
		"":        archive.DefaultCompression,
		"invalid": archive.DefaultCompression,
	}

	for name, level := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, level, GetCompressionLevel(name))
		})
	}
}

func TestZipArchiveExtract(t *testing.T) {
	small := []byte("12345678")
	large := bytes.Repeat([]byte("198273qhnjbqwdjbqwe2109u3abcdef3"), 1024*1024)

	OnEachZipArchiver(t, func(t *testing.T) {
		dir := t.TempDir()
		buf := new(bytes.Buffer)

		require.NoError(t, os.WriteFile(filepath.Join(dir, "small"), small, 0777))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "large"), large, 0777))

		archiver, err := archive.NewArchiver(archive.Zip, buf, dir, archive.DefaultCompression)
		require.NoError(t, err)

		files := make(map[string]fs.FileInfo)
		_ = filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
			if info.IsDir() {
				return nil
			}
			files[path] = info
			return nil
		})

		assert.Equal(t, 2, len(files))
		require.NoError(t, archiver.Archive(context.Background(), files))

		input := buf.Bytes()
		OnEachZipExtractor(t, func(t *testing.T) {
			out := t.TempDir()

			extractor, err := archive.NewExtractor(archive.Zip, bytes.NewReader(input), int64(len(input)), out)
			require.NoError(t, err)
			require.NoError(t, extractor.Extract(context.Background()))

			smallEq, err := os.ReadFile(filepath.Join(out, "small"))
			require.NoError(t, err)
			assert.Equal(t, small, smallEq)

			largeEq, err := os.ReadFile(filepath.Join(out, "large"))
			require.NoError(t, err)
			assert.Equal(t, large, largeEq)
		}, "fastzip")
	}, "fastzip")
}
