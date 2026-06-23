//go:build !integration

package usage_log

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockStorage struct {
	records  []Record
	closed   bool
	storeErr error
	closeErr error
}

func (m *mockStorage) Store(record Record) error {
	if m.storeErr != nil {
		return m.storeErr
	}
	m.records = append(m.records, record)
	return nil
}

func (m *mockStorage) Close() error {
	m.closed = true
	return m.closeErr
}

func TestMultiWriter_Store(t *testing.T) {
	w1 := &mockStorage{}
	w2 := &mockStorage{}
	mw := NewMultiWriter(w1, w2)

	record := Record{UUID: "test-1"}
	err := mw.Store(record)
	require.NoError(t, err)

	assert.Len(t, w1.records, 1)
	assert.Len(t, w2.records, 1)
	assert.Equal(t, "test-1", w1.records[0].UUID)
	assert.Equal(t, "test-1", w2.records[0].UUID)
}

func TestMultiWriter_Store_PartialFailure(t *testing.T) {
	w1 := &mockStorage{storeErr: errors.New("w1 failed")}
	w2 := &mockStorage{}
	mw := NewMultiWriter(w1, w2)

	record := Record{UUID: "test-1"}
	err := mw.Store(record)

	// Error from w1, but w2 still got the record
	assert.Error(t, err)
	assert.ErrorContains(t, err, "w1 failed")
	assert.Len(t, w2.records, 1)
}

func TestMultiWriter_Close(t *testing.T) {
	w1 := &mockStorage{}
	w2 := &mockStorage{}
	mw := NewMultiWriter(w1, w2)

	err := mw.Close()
	require.NoError(t, err)

	assert.True(t, w1.closed)
	assert.True(t, w2.closed)
}

func TestMultiWriter_Close_PartialFailure(t *testing.T) {
	w1 := &mockStorage{closeErr: errors.New("w1 close failed")}
	w2 := &mockStorage{}
	mw := NewMultiWriter(w1, w2)

	err := mw.Close()
	assert.Error(t, err)
	// Both still closed
	assert.True(t, w1.closed)
	assert.True(t, w2.closed)
}
