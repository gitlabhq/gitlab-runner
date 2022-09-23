//go:build !integration

package meter

import (
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestReader_New_NoUpdateFrequency(t *testing.T) {
	// the original io.ReadCloser is returned if the meter update frequency
	// is zero.
	reader := io.NopCloser(nil)
	m := NewReader(reader, 0, func(uint64, time.Duration, bool) {})
	assert.Equal(t, reader, m)
}

func TestReader_New(t *testing.T) {
	complete := new(sync.WaitGroup)
	complete.Add(1)

	m := NewReader(
		io.NopCloser(strings.NewReader("foobar")),
		50*time.Millisecond,
		func(written uint64, since time.Duration, done bool) {
			if done {
				assert.Equal(t, uint64(6), written)
				complete.Done()
			}
		},
	)

	_, err := io.Copy(io.Discard, m)
	assert.NoError(t, err)
	assert.NoError(t, m.Close())
	complete.Wait()

	// another close shouldn't be a problem
	assert.NoError(t, m.Close())
}
