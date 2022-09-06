//go:build !integration

package sentry

import (
	"testing"
)

func TestNewLogHook(t *testing.T) {
	tests := []struct {
		name    string
		dsn     string
		wantErr bool
	}{
		{
			name:    "test old DSN format",
			dsn:     "https://user:password@sentry.io/project/314",
			wantErr: false,
		},
		{
			name:    "test new DSN format with HTTP",
			dsn:     "http://key@sentry.io/314",
			wantErr: false,
		},
		{
			name:    "test new DSN format with HTTPS",
			dsn:     "https://key@sentry.io/314",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewLogHook(tt.dsn)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewLogHook() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
