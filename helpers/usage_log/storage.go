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

type dummyWriteCloser interface {
	io.WriteCloser
}

type Storage struct {
	writer io.WriteCloser
	close  chan struct{}

	timer func() time.Time

	options options
}

func NewStorage(writer io.WriteCloser, o ...Option) *Storage {
	return &Storage{
		writer:  writer,
		close:   make(chan struct{}),
		timer:   time.Now,
		options: setupOptions(o...),
	}
}

func (s *Storage) Store(record Record) error {
	select {
	case <-s.close:
		return ErrStorageIsClosed
	default:
	}

	data, err := json.Marshal(s.setupRecord(record))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrEncodingJSON, err)
	}

	_, err = fmt.Fprintf(s.writer, "%s\n", data)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrStoringLog, err)
	}

	return nil
}

func (s *Storage) setupRecord(record Record) Record {
	record.Timestamp = s.timer().UTC()

	if record.Labels == nil {
		record.Labels = make(map[string]string)
	}

	if s.options.Labels != nil {
		for key, value := range s.options.Labels {
			record.Labels[key] = value
		}
	}

	return record
}

func (s *Storage) Close() error {
	close(s.close)

	return s.writer.Close()
}
