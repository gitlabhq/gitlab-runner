package fs

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type ScannerWalkFn func(f FileEntry) error
type SelectFileFn func(path string, fileInfo os.FileInfo) bool

type Scanner interface {
	Walk(fn ScannerWalkFn) error
	Scan(selectFile SelectFileFn) error
	AddFile(path string) error
}

type directoryScanner struct {
	entries []FileEntry

	workDir string
}

func NewDirectoryScanner(workDir string) Scanner {
	return &directoryScanner{
		workDir: workDir,
	}
}

func (fs *directoryScanner) Walk(fn ScannerWalkFn) error {
	for _, entry := range fs.entries {
		err := fn(entry)
		if err != nil {
			return fmt.Errorf("error while walking the files: %w", err)
		}
	}

	return nil
}

func (fs *directoryScanner) Scan(selectFile SelectFileFn) error {
	return filepath.Walk(fs.workDir, func(path string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error while entering the path %q: %w", path, err)
		}

		if fileInfo.IsDir() || !selectFile(path, fileInfo) {
			return nil
		}

		err = fs.createFileEntry(path, fileInfo)
		if err != nil {
			return fmt.Errorf("couldn't create file entry for %q: %w", path, err)
		}

		return nil
	})
}

func (fs *directoryScanner) createFileEntry(path string, fileInfo os.FileInfo) error {
	if fileInfo.IsDir() {
		return fmt.Errorf("can't handle %q because it's a directory", path)
	}

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open the file %q: %w", path, err)
	}
	defer f.Close()

	hasher := sha256.New()

	_, err = io.Copy(hasher, f)
	if err != nil {
		return fmt.Errorf("failed to copy %q content to the SHA256 hash calculator: %w", path, err)
	}

	relativePath, err := filepath.Rel(fs.workDir, path)
	if err != nil {
		return fmt.Errorf("couldn't compute the relative path for %q and %q: %w", fs.workDir, path, err)
	}

	entry := FileEntry{
		FileName:     fileInfo.Name(),
		FullPath:     path,
		RelativePath: relativePath,
		Checksum:     fmt.Sprintf("%x", hasher.Sum(nil)),
		SizeMb:       float64(fileInfo.Size()) / 1048576,
	}

	fs.entries = append(fs.entries, entry)

	return nil
}

func (fs *directoryScanner) AddFile(path string) error {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("couldn't stat %q: %w", path, err)
	}

	return fs.createFileEntry(path, fileInfo)
}
