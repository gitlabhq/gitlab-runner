//go:build !integration

package buildlogger

import (
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// preStampedLine is a line in the runner timestamper's wire format: a
// 28-byte timestamp (YYYY-MM-DDTHH:MM:SS.UUUUUUZ<space>) followed by the
// body. stepRunnerStream validates the full shape before picking the
// passthrough writer.
const preStampedLine = "2026-05-05T12:00:00.000000Z hello-from-step-runner\n"

func newStepRunnerLogger(t *testing.T, jt Trace, maskPhrases ...string) Logger {
	t.Helper()
	return New(jt, logrus.WithField("test", t.Name()), Options{
		Timestamping: true,
		MaskPhrases:  maskPhrases,
	})
}

func TestStepRunnerStream(t *testing.T) {
	// Lookalike: 28+ bytes with 'Z' at byte 26 and ' ' at byte 27 but the
	// wrong separators elsewhere. isPreStamped must reject it.
	const lookalikeLine = "AAAAxAAxAAxAAxAAxAAxAAAAAAZ rest\n"
	require.Equal(t, byte('Z'), lookalikeLine[26])
	require.Equal(t, byte(' '), lookalikeLine[27])

	tests := map[string]struct {
		maskPhrases []string
		input       string
		assertion   func(t *testing.T, output string)
	}{
		"pre-stamped passes through byte-for-byte": {
			input: preStampedLine,
			assertion: func(t *testing.T, output string) {
				assert.Equal(t, preStampedLine, output)
			},
		},
		"plain data is wrapped and stamped": {
			input: "plain output\n",
			assertion: func(t *testing.T, output string) {
				assert.NotEqual(t, "plain output\n", output)
				assert.Contains(t, output, "plain output")
				assert.Contains(t, output, "Z ") // runner timestamp suffix
			},
		},
		"pre-stamped skips the masker": {
			// Passthrough bypasses the wrap chain entirely; step-runner
			// is expected to have applied masking already.
			maskPhrases: []string{"secret-token"},
			input:       "2026-05-05T12:00:00.000000Z contains secret-token literally\n",
			assertion: func(t *testing.T, output string) {
				assert.Equal(t, "2026-05-05T12:00:00.000000Z contains secret-token literally\n", output)
			},
		},
		"plain data is masked by the wrap chain": {
			maskPhrases: []string{"secret-token"},
			input:       "contains secret-token literally\n",
			assertion: func(t *testing.T, output string) {
				assert.Contains(t, output, "[MASKED]")
				assert.NotContains(t, output, "secret-token")
			},
		},
		"short first write falls back to wrapped": {
			// Shorter than 28 bytes — isPreStamped must not panic.
			input: "short\n",
			assertion: func(t *testing.T, output string) {
				assert.Contains(t, output, "short")
				assert.Contains(t, output, "Z ") // got stamped
			},
		},
		"lookalike with Z at byte 26 falls back to wrapped": {
			input: lookalikeLine,
			assertion: func(t *testing.T, output string) {
				assert.Contains(t, output, "rest")
				assert.Contains(t, output, "Z ") // got stamped by the wrap chain
			},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			jt := newFakeJobTrace()
			l := newStepRunnerLogger(t, jt, tc.maskPhrases...)

			w := l.StepRunnerStream(StreamWorkLevel, Stdout)
			_, err := w.Write([]byte(tc.input))
			require.NoError(t, err)
			require.NoError(t, w.Close())

			tc.assertion(t, jt.Read())
		})
	}
}

func TestStepRunnerStream_ChoicePersists(t *testing.T) {
	jt := newFakeJobTrace()
	l := newStepRunnerLogger(t, jt)

	w := l.StepRunnerStream(StreamWorkLevel, Stdout)
	// First write is pre-stamped, locking in the passthrough writer.
	_, err := w.Write([]byte(preStampedLine))
	require.NoError(t, err)
	// A subsequent write that happens NOT to start with a timestamp must
	// still go through passthrough — the choice is made once.
	_, err = w.Write([]byte("trailing-without-stamp\n"))
	require.NoError(t, err)
	require.NoError(t, w.Close())

	out := jt.Read()
	assert.Contains(t, out, preStampedLine)
	assert.Contains(t, out, "trailing-without-stamp\n")
	// And no second wrapping — only one 'Z ' suffix (the one in
	// preStampedLine), not a fresh timestamp on the trailing write.
	assert.Equal(t, 1, strings.Count(out, "Z "))
}

func TestStepRunnerStream_CloseWithoutWrite(t *testing.T) {
	jt := newFakeJobTrace()
	l := newStepRunnerLogger(t, jt)

	w := l.StepRunnerStream(StreamWorkLevel, Stdout)
	// Close before any write should not panic and should pick a writer.
	require.NoError(t, w.Close())
	assert.Empty(t, jt.Read())
}
