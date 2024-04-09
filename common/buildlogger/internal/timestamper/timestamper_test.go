//go:build !integration

package timestamper

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func setupDummyTime() func() {
	oldNow := now

	pretend, _ := time.Parse(time.RFC3339, "2021-01-01T00:00:00.000000Z")
	now = func() time.Time {
		pretend = pretend.Add(time.Hour)
		return pretend.UTC()
	}

	return func() {
		now = oldNow
	}
}

// nolint:errcheck
func writeLines(w io.Writer) {
	w.Write([]byte("PREFIX This is the beginning of a new line\n"))
	w.Write([]byte("PREFIX This is a split "))
	w.Write([]byte("up "))
	w.Write([]byte("line\n"))
	w.Write([]byte("PREFIX Progress bar: "))

	for i := 0; i < 10; i++ {
		w.Write([]byte(".\r"))
	}

	w.Write([]byte("Done.\r\n"))
	w.Write([]byte("PREFIX Another windows new-line\r\n"))

	w.Write([]byte("PREFIX multiple\nnew\nlines\nin\none\n"))

	w.Write([]byte("\nstart"))
	w.Write([]byte("\nend\n"))

	w.Write([]byte("PREFIX Eat carriages\r\r\r\r\r\r\r\n"))
	w.Write([]byte("PREFIX This is across\ntwo lines\n"))
	w.Write([]byte("PREFIX The end"))
}

func TestWithTimestamps(t *testing.T) {
	buf := new(bytes.Buffer)

	defer setupDummyTime()()

	w := New(buf, StderrType, 255, true)
	writeLines(w)
	w.Close()

	expected := []string{
		"2021-01-01T01:00:00.000000Z ffE PREFIX This is the beginning of a new line\n",
		"2021-01-01T02:00:00.000000Z ffE PREFIX This is a split up line\n",
		"2021-01-01T03:00:00.000000Z ffE PREFIX Progress bar: .\r\n",
		"2021-01-01T04:00:00.000000Z ffE+.\r\n",
		"2021-01-01T05:00:00.000000Z ffE+.\r\n",
		"2021-01-01T06:00:00.000000Z ffE+.\r\n",
		"2021-01-01T07:00:00.000000Z ffE+.\r\n",
		"2021-01-01T08:00:00.000000Z ffE+.\r\n",
		"2021-01-01T09:00:00.000000Z ffE+.\r\n",
		"2021-01-01T10:00:00.000000Z ffE+.\r\n",
		"2021-01-01T11:00:00.000000Z ffE+.\r\n",
		"2021-01-01T12:00:00.000000Z ffE+.\r\n",
		"2021-01-01T13:00:00.000000Z ffE+Done.\r\n",
		"2021-01-01T14:00:00.000000Z ffE PREFIX Another windows new-line\r\n",
		"2021-01-01T15:00:00.000000Z ffE PREFIX multiple\n",
		"2021-01-01T16:00:00.000000Z ffE new\n",
		"2021-01-01T17:00:00.000000Z ffE lines\n",
		"2021-01-01T18:00:00.000000Z ffE in\n",
		"2021-01-01T19:00:00.000000Z ffE one\n",
		"2021-01-01T20:00:00.000000Z ffE \n",
		"2021-01-01T21:00:00.000000Z ffE start\n",
		"2021-01-01T22:00:00.000000Z ffE end\n",
		"2021-01-01T23:00:00.000000Z ffE PREFIX Eat carriages\r\r\r\r\r\r\r\n",
		"2021-01-02T00:00:00.000000Z ffE PREFIX This is across\n",
		"2021-01-02T01:00:00.000000Z ffE two lines\n",
		"2021-01-02T02:00:00.000000Z ffE PREFIX The end\n",
	}

	assert.Equal(t, strings.Join(expected, ""), buf.String())
}

func TestWithoutTimestamp(t *testing.T) {
	buf := new(bytes.Buffer)

	defer setupDummyTime()()

	w := New(buf, StderrType, 255, false)
	writeLines(w)
	w.Close()

	expected := []string{
		"ffE PREFIX This is the beginning of a new line\n",
		"ffE PREFIX This is a split up line\n",
		"ffE PREFIX Progress bar: .\r\n",
		"ffE+.\r\n",
		"ffE+.\r\n",
		"ffE+.\r\n",
		"ffE+.\r\n",
		"ffE+.\r\n",
		"ffE+.\r\n",
		"ffE+.\r\n",
		"ffE+.\r\n",
		"ffE+.\r\n",
		"ffE+Done.\r\n",
		"ffE PREFIX Another windows new-line\r\n",
		"ffE PREFIX multiple\n",
		"ffE new\n",
		"ffE lines\n",
		"ffE in\n",
		"ffE one\n",
		"ffE \n",
		"ffE start\n",
		"ffE end\n",
		"ffE PREFIX Eat carriages\r\r\r\r\r\r\r\n",
		"ffE PREFIX This is across\n",
		"ffE two lines\n",
		"ffE PREFIX The end\n",
	}

	assert.Equal(t, strings.Join(expected, ""), buf.String())
}

// nolint:errcheck
func TestForcedFlush(t *testing.T) {
	buf := new(bytes.Buffer)

	defer setupDummyTime()()

	w := New(buf, StderrType, 255, true)
	w.Write([]byte("PREFIX This is the beginning of a new line\n"))
	w.Write([]byte("We have no new line character in this write"))
	w.Write([]byte("... The line is now flushed.\n"))
	w.Write([]byte("large continuous write incoming"))
	w.Write(bytes.Repeat([]byte{'.'}, bufSize))
	w.Write(bytes.Repeat([]byte{'.'}, bufSize+1))
	w.Write([]byte("ended\n"))
	w.Close()

	expected := []string{
		"2021-01-01T01:00:00.000000Z ffE PREFIX This is the beginning of a new line\n",
		"2021-01-01T02:00:00.000000Z ffE We have no new line character in this write... The line is now flushed.\n",
		"2021-01-01T03:00:00.000000Z ffE large continuous write incoming" + strings.Repeat(".", bufSize) + "\n",
		"2021-01-01T04:00:00.000000Z ffE+" + strings.Repeat(".", bufSize+1) + "\n",
		"2021-01-01T05:00:00.000000Z ffE+ended\n",
	}

	assert.Equal(t, strings.Join(expected, ""), buf.String())
}

func BenchmarkWithTimestamps(b *testing.B) {
	defer setupDummyTime()()

	w := New(io.Discard, StderrType, 255, true)

	headerSize := len(time.Now().Format(time.RFC3339)) + fracs + additionalBytes + 4

	line := []byte("This is the beginning of a new line\n")
	b.SetBytes(int64((headerSize + len(line)) * 200))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for j := 0; j < 200; j++ {
			_, _ = w.Write(line)
		}
	}
}
