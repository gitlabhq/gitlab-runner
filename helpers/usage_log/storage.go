package usage_log

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

var (
	ErrStorageIsClosed = errors.New("storage is closed")
	ErrEncodingJSON    = errors.New("encoding json")
	ErrStoringLog      = errors.New("storing log")
)

//go:generate mockery --name=dummyWriteCloser --inpackage --with-expecter
type dummyWriteCloser interface {
	io.WriteCloser
}

type Storage struct {
	writer io.WriteCloser
	close  chan struct{}

	timer func() time.Time
}

func NewStorage(writer io.WriteCloser) *Storage {
	return &Storage{
		writer: writer,
		close:  make(chan struct{}),
		timer:  time.Now().UTC,
	}
}

func (s *Storage) Store(record Record) error {
	select {
	case <-s.close:
		return ErrStorageIsClosed
	default:
	}

	record.Timestamp = s.timer()

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrEncodingJSON, err)
	}

	_, err = fmt.Fprintf(s.writer, "%s\n", data)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrStoringLog, err)
	}

	return nil
}

func (s *Storage) Close() error {
	close(s.close)

	return s.writer.Close()
}
