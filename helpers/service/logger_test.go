//go:build !integration

package service_helpers

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/trace"
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

func TestInlineServiceLogWriter(t *testing.T) {
	buf := bytes.Buffer{}
	slw := NewInlineServiceLogWriter("foo", &buf)
	pre := string(slw.prefix)
	suf := string(slw.suffix)
	newLine := suf + pre
	emptyLine := pre + suf

	tests := map[string]struct {
		msg  string
		want string
	}{
		"no newlines": {
			msg:  "bar baz bla bla bla",
			want: pre + "bar baz bla bla bla" + suf,
		},
		"leading newline": {
			msg:  "\nbar baz bla bla bla",
			want: emptyLine + pre + "bar baz bla bla bla" + suf,
		},
		"trailing newline": {
			msg:  "bar baz bla bla bla\n",
			want: pre + "bar baz bla bla bla" + suf,
		},
		"inner newlines": {
			msg:  "bar\nbaz\nbla bla bla",
			want: pre + "bar" + newLine + "baz" + newLine + "bla bla bla" + suf,
		},
		"all the newlines": {
			msg:  "\nbar\nbaz\nbla bla bla\n",
			want: emptyLine + pre + "bar" + newLine + "baz" + newLine + "bla bla bla" + suf,
		},
		"consecutive newlines": {
			msg:  "bar\n\nbaz bla\n\nbla bla",
			want: pre + "bar" + newLine + newLine + "baz bla" + newLine + newLine + "bla bla" + suf,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			defer buf.Reset()

			n, err := slw.Write([]byte(tt.msg))

			assert.NoError(t, err)
			assert.Equal(t, tt.want, buf.String())
			assert.Equal(t, len(tt.msg), n)
		})
	}
}

// Ensure that inlineServiceLogWriter respects the `io.Writer` contract.
// Specifically, if the number of bytes written returned by `Write()` is less
// than the message length, `Write() must return an error.
func TestInlineServiceLogWriter_Err(t *testing.T) {
	buf := bytes.Buffer{}
	tw := truncateWriter{sink: &buf}
	slw := NewInlineServiceLogWriter("foo", &tw)

	plen, slen := len(slw.prefix), len(slw.suffix)

	tests := map[string]struct {
		msg         string
		stopAfter   int
		wantWritten int
	}{
		"none of original message written": {
			msg:         "bar baz bla",
			stopAfter:   6,
			wantWritten: 0,
		},
		"some of original message written": {
			msg:         "bar baz bla",
			stopAfter:   plen + 7,
			wantWritten: 7,
		},
		"all of original message written": {
			msg:         "bar baz bla",
			stopAfter:   plen + 11 + slen - 2,
			wantWritten: 11,
		},
		"some of original message (with newlines) written": {
			msg:         "\nbar baz\nbla\n",
			stopAfter:   plen*2 + slen*2 - 1 + 7,
			wantWritten: 9,
		},
		"some more of original message (with newlines) written": {
			msg:         "\nbar baz\nbla\n",
			stopAfter:   plen*3 + slen*2 + 9,
			wantWritten: 11,
		},
		"all of original message (with newlines) written": {
			msg:         "\nbar baz\nbla\n",
			stopAfter:   plen*3 + slen*3 - 1 + 10,
			wantWritten: 13,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			defer func() {
				buf.Reset()
				tw.written = 0
			}()

			tw.stopAfter = tt.stopAfter

			n, err := slw.Write([]byte(tt.msg))

			assert.Equal(t, tt.wantWritten, n)
			assert.Error(t, err)
		})
	}
}

func BenchmarkServiceLog(b *testing.B) {
	var payloads [][]byte
	var size int
	for i := 0; i < 64; i++ {
		payloads = append(payloads, append(bytes.Repeat([]byte{'a' + byte(i%26)}, i*1024), '\n'))
		size += i*1024 + 1
	}

	benchmarks := map[string]func() io.Writer{
		"discard": func() io.Writer { return io.Discard },
		"trace buffer": func() io.Writer {
			buf, err := trace.New()
			buf.SetMasked(common.MaskOptions{Phrases: []string{"bench", "mark"}})
			require.NoError(b, err)
			return buf
		},
	}

	b.ResetTimer()
	for name, bufFn := range benchmarks {
		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(size))

			for n := 0; n < b.N; n++ {
				buf := bufFn()
				if c, ok := buf.(io.Closer); ok {
					defer c.Close()
				}

				slw := NewInlineServiceLogWriter("foo", buf)
				for _, payload := range payloads {
					_, err := slw.Write(payload)
					require.NoError(b, err)
				}
				require.NoError(b, slw.Close())
			}
		})
	}
}
