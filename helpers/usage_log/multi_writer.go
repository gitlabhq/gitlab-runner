package usage_log

import "errors"

// MultiWriter fans out Store calls to multiple writers.
// Close closes all writers. Errors from all writers are joined.
type MultiWriter struct {
	writers []Storage
}

func NewMultiWriter(writers ...Storage) *MultiWriter {
	return &MultiWriter{writers: writers}
}

func (mw *MultiWriter) Store(record Record) error {
	var errs []error
	for _, w := range mw.writers {
		if err := w.Store(record); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (mw *MultiWriter) Close() error {
	var errs []error
	for _, w := range mw.writers {
		if err := w.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
