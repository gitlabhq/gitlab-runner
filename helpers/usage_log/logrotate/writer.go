package logrotate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	fileNameTimeFormat = "2006-01-02-15-04-05.000"
	fileNamePrefix     = "usage-log-"
	fileNameExt        = ".json"
)

var (
	ErrCreationFailure = errors.New("creating log file")
	ErrRotationFailure = errors.New("rotating log file")

	fileNameFormat = fileNamePrefix + fileNameTimeFormat + fileNameExt
)

type logfileInfo struct {
	name      string
	timestamp time.Time
}

type Writer struct {
	options options

	f  *os.File
	ts time.Time

	mu sync.RWMutex

	runCleanup chan struct{}
}

func New(o ...Option) *Writer {
	w := &Writer{
		options:    setupOptions(o...),
		runCleanup: make(chan struct{}),
	}

	return w
}

func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.f == nil {
		err := w.reCreateFile()
		if err != nil {
			return 0, fmt.Errorf("%w: %v", ErrCreationFailure, err)
		}
	}

	err := w.rotate()
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrRotationFailure, err)
	}

	wrote, err := w.f.Write(p)
	if err != nil {
		err = fmt.Errorf("writing log: %w", err)
	}

	go w.cleanup()

	return wrote, err
}

func (w *Writer) reCreateFile() error {
	logDir := w.options.LogDirectory

	err := os.MkdirAll(logDir, 0755)
	if err != nil {
		return fmt.Errorf("creating log directory: %w", err)
	}

	w.ts = time.Now().UTC()

	fileName := w.ts.Format(fileNameFormat)
	logFile := filepath.Join(logDir, fileName)

	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening log file: %w", err)
	}

	w.f = file

	return nil
}

func (w *Writer) rotate() error {
	if w.f == nil {
		return nil
	}

	if time.Since(w.ts) < w.options.MaxRotationAge {
		return nil
	}

	err := w.f.Close()
	if err != nil {
		return fmt.Errorf("closing log file: %w", err)
	}

	return w.reCreateFile()
}

func (w *Writer) cleanup() {
	w.mu.RLock()
	select {
	case <-w.runCleanup:
		w.mu.RUnlock()
		return
	default:
	}
	w.mu.RUnlock()

	w.mu.Lock()
	defer w.mu.Unlock()
	close(w.runCleanup)

	defer func() {
		w.runCleanup = make(chan struct{})
	}()

	logFiles := w.allLogFiles()

	if int64(len(logFiles)) <= w.options.MaxBackupFiles {
		return
	}

	w.timesortLogFiles(logFiles)

	toRemove := logFiles[w.options.MaxBackupFiles:]
	for _, file := range toRemove {
		_ = os.Remove(filepath.Join(w.options.LogDirectory, file.name))
	}
}

func (w *Writer) allLogFiles() []logfileInfo {
	files, _ := os.ReadDir(w.options.LogDirectory)

	var logFiles []logfileInfo
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filename := file.Name()

		if !strings.HasPrefix(filename, fileNamePrefix) {
			continue
		}

		if !strings.HasSuffix(filename, fileNameExt) {
			continue
		}

		timestamp := filename[len(fileNamePrefix) : len(filename)-len(fileNameExt)]
		ts, err := time.Parse(fileNameTimeFormat, timestamp)
		if err != nil {
			continue
		}

		logFiles = append(logFiles, logfileInfo{
			name:      filename,
			timestamp: ts,
		})
	}

	return logFiles
}

func (w *Writer) timesortLogFiles(files []logfileInfo) {
	slices.SortFunc(files, func(a, b logfileInfo) int {
		if a.timestamp.After(b.timestamp) {
			return -1
		}

		if a.timestamp.Equal(b.timestamp) {
			return 0
		}

		return 1
	})
}

func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.f == nil {
		return nil
	}

	return w.f.Close()
}
