package client

import (
	"log/slog"
	"net"
	"os"
)

type Option func(o *options)

type options struct {
	logger *slog.Logger
	dialer Dialer
}

func setupOptions(opts []Option) options {
	o := options{
		logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
		dialer: net.Dial,
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

func WithDialer(dialer Dialer) Option {
	return func(o *options) {
		o.dialer = dialer
	}
}
