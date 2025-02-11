package client

import (
	"log/slog"
	"os"
)

type Option func(o *options)

type options struct {
	logger *slog.Logger
}

func setupOptions(opts []Option) options {
	o := options{
		logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}

	for _, opt := range opts {
		opt(&o)
	}

	return o
}

func WithLogger(logger *slog.Logger) Option {
	return func(o *options) {
		o.logger = logger
	}
}
