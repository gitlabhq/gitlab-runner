//go:build !integration

package service_helpers

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

// truncateWriter implements a misbehaving/failing io.Writer. It will stop
// writing after `stopAfter` bytes and return an error.
type truncateWriter struct {
	sink      io.Writer
	stopAfter int
	written   int
}

func (tw *truncateWriter) Write(p []byte) (int, error) {
	stop := min(len(p), tw.stopAfter-tw.written)
	n, _ := tw.sink.Write(p[:stop])
	tw.written += n

	if n < len(p) {
		return n, fmt.Errorf("stopped writing after %d bytes", tw.written)
	}
	return n, nil
}

func min(a, b int) int {
	if a <= b {
		return a
	}
	return b
}

func TestTruncateWriter(t *testing.T) {
	buf := bytes.Buffer{}

	tests := map[string]struct {
		msg       []string
		want      string
		stopAfter int
	}{
		"all written": {
			msg:       []string{"foo bar baz bla bla bla"},
			want:      "foo bar baz bla bla bla",
			stopAfter: 100,
		},
		"stop at message end": {
			msg:       []string{"foo bar baz bla bla bla"},
			want:      "foo bar baz bla bla bla",
			stopAfter: 23,
		},
		"truncate single-part message": {
			msg:       []string{"foo bar baz bla bla bla"},
			want:      "foo bar",
			stopAfter: 7,
		},
		"truncate multi-part message": {
			msg:       []string{"foo bar baz", " bla bla bla"},
			want:      "foo bar baz bla ",
			stopAfter: 16,
		},
		"stop at  multi-part message end": {
			msg:       []string{"foo bar baz", " bla bla bla"},
			want:      "foo bar baz bla bla bla",
			stopAfter: 23,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			buf.Reset()
			tw := truncateWriter{sink: &buf, stopAfter: tt.stopAfter}
			msgLen := 0
			written := 0

			for _, line := range tt.msg {
				n, err := tw.Write([]byte(line))

				msgLen += len(line)
				written += n

				if n < len(line) {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			}
			assert.Equal(t, min(tt.stopAfter, msgLen), written)
			assert.Equal(t, tw.written, written)
			assert.Equal(t, tt.want, buf.String())
		})
	}
}
