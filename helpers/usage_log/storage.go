package usage_log

import (
	"errors"
)

var (
	ErrStorageIsClosed = errors.New("storage is closed")
	ErrStoringLog      = errors.New("storing log")
)

type Storage interface {
	Store(record Record) error
	Close() error
}
