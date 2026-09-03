package usage_log

import (
	"errors"
)

var (
	ErrStorageIsClosed = errors.New("storage is closed")
	ErrStoringLog      = errors.New("storing log")
)

type Storage interface {
	// Store implementations must not modify the record or its maps because
	// MultiWriter passes the same value to every storage.
	Store(record Record) error
	Close() error
}
