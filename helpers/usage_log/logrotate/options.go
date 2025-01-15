package logrotate

import (
	"os"
	"path/filepath"
	"time"
)

const (
	defaultMaxBackupFiles = 14
	defaultMaxRotationAge = 24 * time.Hour
)

type options struct {
	// LogDirectory directory in which the log file will be stored
	LogDirectory string

	// MaxBackupFiles how many older files to leave after rotation
	MaxBackupFiles int64

	// MaxRotationAge duration after which the file should be force rotated.
	// Default is 24 hours
	MaxRotationAge time.Duration
}

type Option func(*options)

func setupOptions(o ...Option) options {
	opts := options{
		LogDirectory:   filepath.Join(os.TempDir(), "usage-logger"),
		MaxBackupFiles: defaultMaxBackupFiles,
		MaxRotationAge: defaultMaxRotationAge,
	}

	for _, opt := range o {
		opt(&opts)
	}

	return opts
}

func WithLogDirectory(dir string) Option {
	return func(o *options) {
		o.LogDirectory = dir
	}
}

func WithMaxBackupFiles(maxBackupFiles int64) Option {
	return func(o *options) {
		o.MaxBackupFiles = maxBackupFiles
	}
}

func WithMaxRotationAge(maxRotationAge time.Duration) Option {
	return func(o *options) {
		o.MaxRotationAge = maxRotationAge
	}
}
