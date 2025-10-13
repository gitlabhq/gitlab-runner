package usage_log

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
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
	// Let's use RFC-9562 UUIDv7 compatible id if only possible. If a random
	// error happens on reading the random value inside, we shouldn't block
	// storing the usage event and instead should provide a UUID compatible
	// value whose "randomness" should be simulated enough by assigning
	// values unique to a specific job and hashing that with SHA-256.
	uid, err := uuid.NewV7()
	if err != nil {
		hash := sha256.Sum256([]byte(record.Timestamp.Format(time.RFC3339) + record.Runner.ID + record.Runner.SystemID + record.Job.URL))
		uid = uuid.UUID(hash[:16])
	}

	record.UUID = uid.String()
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
