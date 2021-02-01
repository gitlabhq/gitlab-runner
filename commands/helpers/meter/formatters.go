package meter

import (
	"fmt"
	"io"
	"math"
	"time"
)

func FormatByteRate(b uint64, d time.Duration) string {
	b = uint64(float64(b) / math.Max(time.Nanosecond.Seconds(), d.Seconds()))
	rate, prefix := formatBytes(b)
	if prefix == 0 {
		return fmt.Sprintf("%d B/s", int(rate))
	}

	return fmt.Sprintf("%.1f %cB/s", rate, prefix)
}

func FormatBytes(b uint64) string {
	size, prefix := formatBytes(b)
	if prefix == 0 {
		return fmt.Sprintf("%d B", int(size))
	}

	return fmt.Sprintf("%.2f %cB", size, prefix)
}

func formatBytes(b uint64) (float64, byte) {
	const (
		unit   = 1000
		prefix = "KMGTPE"
	)

	if b < unit {
		return float64(b), 0
	}

	div := int64(unit)
	exp := 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return float64(b) / float64(div), prefix[exp]
}

func LabelledRateFormat(w io.Writer, label string, totalSize int64) UpdateCallback {
	return func(written uint64, since time.Duration, done bool) {
		known := ""
		if totalSize > UnknownTotalSize {
			known = "/" + FormatBytes(uint64(totalSize))
		}

		line := fmt.Sprintf(
			"\r%s %s%s (%s)                ",
			label,
			FormatBytes(written),
			known,
			FormatByteRate(written, since),
		)

		if done {
			_, _ = fmt.Fprintln(w, line)
			return
		}
		_, _ = fmt.Fprint(w, line)
	}
}
