//go:build !integration

package meter

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFormatByteRate(t *testing.T) {
	tests := map[string]struct {
		size     uint64
		d        time.Duration
		expected string
	}{
		"format bytes":     {1, time.Second, "1 B/s"},
		"format kilobytes": {1000, time.Second, "1.0 KB/s"},
		"format megabytes": {1000000, time.Second, "1.0 MB/s"},
		"format gigabytes": {1000000000, time.Second, "1.0 GB/s"},
		"format terabytes": {1000000000000, time.Second, "1.0 TB/s"},
		"format petabytes": {1000000000000000, time.Second, "1.0 PB/s"},
		"format exabytes":  {1000000000000000000, time.Second, "1.0 EB/s"},

		"format kilobytes under": {1490, time.Second, "1.5 KB/s"},
		"format megabytes under": {1490000, time.Second, "1.5 MB/s"},
		"format gigabytes under": {1490000000, time.Second, "1.5 GB/s"},
		"format terabytes under": {1490000000000, time.Second, "1.5 TB/s"},
		"format petabytes under": {1490000000000000, time.Second, "1.5 PB/s"},
		"format exabytes under":  {1490000000000000000, time.Second, "1.5 EB/s"},

		"format kilobytes over": {1510, time.Second, "1.5 KB/s"},
		"format megabytes over": {1510000, time.Second, "1.5 MB/s"},
		"format gigabytes over": {1510000000, time.Second, "1.5 GB/s"},
		"format terabytes over": {1510000000000, time.Second, "1.5 TB/s"},
		"format petabytes over": {1510000000000000, time.Second, "1.5 PB/s"},
		"format exabytes over":  {1510000000000000000, time.Second, "1.5 EB/s"},

		"format kilobytes exact": {1300, time.Second, "1.3 KB/s"},
		"format megabytes exact": {1300000, time.Second, "1.3 MB/s"},
		"format gigabytes exact": {1300000000, time.Second, "1.3 GB/s"},
		"format terabytes exact": {1300000000000, time.Second, "1.3 TB/s"},
		"format petabytes exact": {1300000000000000, time.Second, "1.3 PB/s"},
		"format exabytes exact":  {1300000000000000000, time.Second, "1.3 EB/s"},

		"format bytes (non-second)":  {10, 2 * time.Second, "5 B/s"},
		"format bytes (zero-second)": {10, 0, "10.0 GB/s"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expected, FormatByteRate(tc.size, tc.d))
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := map[string]struct {
		size     uint64
		expected string
	}{
		"format bytes":     {1, "1 B"},
		"format kilobytes": {1100, "1.10 KB"},
		"format megabytes": {1110000, "1.11 MB"},
		"format gigabytes": {1111000000, "1.11 GB"},
		"format terabytes": {1111100000000, "1.11 TB"},
		"format petabytes": {1111110000000000, "1.11 PB"},
		"format exabytes":  {1111110000000000000, "1.11 EB"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expected, FormatBytes(tc.size))
		})
	}
}

func TestLabelledRateFormat(t *testing.T) {
	commonOutput := func(t *testing.T, line string, _ int64) {
		assert.Contains(t, line, "\rTesting formatter 10 B")
		assert.Contains(t, line, "(10 B/s)")
	}
	unknownTotalSizeOutput := func(t *testing.T, line string, total int64) {
		assert.NotContains(t, line, fmt.Sprintf("/%s", FormatBytes(uint64(total))))
	}
	knownTotalSizeOutput := func(t *testing.T, line string, total int64) {
		assert.Contains(t, line, fmt.Sprintf("/%s", FormatBytes(uint64(total))))
	}
	undoneOutput := func(t *testing.T, line string, _ int64) {
		assert.NotContains(t, line, "\n")
	}
	doneOutput := func(t *testing.T, line string, _ int64) {
		assert.Contains(t, line, "\n")
	}

	tests := map[string]struct {
		total        int64
		done         bool
		assertOutput func(t *testing.T, line string, total int64)
	}{
		"unknown total size undone": {
			total: UnknownTotalSize,
			done:  false,
			assertOutput: func(t *testing.T, line string, total int64) {
				commonOutput(t, line, total)
				unknownTotalSizeOutput(t, line, total)
				undoneOutput(t, line, total)
			},
		},
		"unknown total size done": {
			total: UnknownTotalSize,
			done:  true,
			assertOutput: func(t *testing.T, line string, total int64) {
				commonOutput(t, line, total)
				unknownTotalSizeOutput(t, line, total)
				doneOutput(t, line, total)
			},
		},
		"known total size undone": {
			total: 10,
			done:  false,
			assertOutput: func(t *testing.T, line string, total int64) {
				commonOutput(t, line, total)
				knownTotalSizeOutput(t, line, total)
				undoneOutput(t, line, total)
			},
		},
		"known total size done": {
			total: 10,
			done:  true,
			assertOutput: func(t *testing.T, line string, total int64) {
				commonOutput(t, line, total)
				knownTotalSizeOutput(t, line, total)
				doneOutput(t, line, total)
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			buf := new(bytes.Buffer)

			fn := LabelledRateFormat(buf, "Testing formatter", tt.total)
			fn(10, 1*time.Second, tt.done)

			tt.assertOutput(t, buf.String(), tt.total)
		})
	}
}
