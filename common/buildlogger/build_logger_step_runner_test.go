//go:build !integration

package buildlogger

import (
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wireLine builds one full-line entry in step-runner's timestamper
// wire format: 28-byte timestamp + 2-hex stream id + stream type +
// line type + body. Only the full-line type (' ') is exercised; the
// partial-line continuation marker ('+') has no consumer here.
// Body should end with '\n'.
func wireLine(streamType byte, body string) string {
	return "2026-05-05T12:00:00.000000Z 01" + string(streamType) + " " + body
}

func newStepRunnerLogger(t *testing.T, jt Trace, timestamping bool, maskPhrases ...string) Logger {
	t.Helper()
	return New(jt, logrus.WithField("test", t.Name()), Options{
		Timestamping: timestamping,
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
		timestamping bool
		maskPhrases  []string
		input        string
		assertion    func(t *testing.T, output string)
	}{
		"pre-stamped passes through when Timestamping is on": {
			timestamping: true,
			input:        wireLine('O', "hello-from-step-runner\n"),
			assertion: func(t *testing.T, output string) {
				// Inner header preserved verbatim, no second stamp added.
				assert.Equal(t, wireLine('O', "hello-from-step-runner\n"), output)
			},
		},
		"pre-stamped is stripped when Timestamping is off": {
			input: wireLine('O', "hello-from-step-runner\n"),
			assertion: func(t *testing.T, output string) {
				assert.NotContains(t, output, "2026-05-05T12:00:00.000000Z")
				assert.Contains(t, output, "hello-from-step-runner")
				assert.NotContains(t, output, "Z ")
			},
		},
		"pre-stamped passthrough does not mask (step-runner masked upstream)": {
			timestamping: true,
			maskPhrases:  []string{"secret-token"},
			input:        wireLine('O', "contains secret-token literally\n"),
			assertion: func(t *testing.T, output string) {
				assert.Contains(t, output, "secret-token")
			},
		},
		"pre-stamped strip applies the wrap chain masker": {
			maskPhrases: []string{"secret-token"},
			input:       wireLine('O', "contains secret-token literally\n"),
			assertion: func(t *testing.T, output string) {
				assert.Contains(t, output, "[MASKED]")
				assert.NotContains(t, output, "secret-token")
			},
		},
		"plain data is wrapped and stamped": {
			timestamping: true,
			input:        "plain output\n",
			assertion: func(t *testing.T, output string) {
				assert.NotEqual(t, "plain output\n", output)
				assert.Contains(t, output, "plain output")
				assert.Contains(t, output, "Z ")
			},
		},
		"plain data is masked by the wrap chain": {
			timestamping: true,
			maskPhrases:  []string{"secret-token"},
			input:        "contains secret-token literally\n",
			assertion: func(t *testing.T, output string) {
				assert.Contains(t, output, "[MASKED]")
				assert.NotContains(t, output, "secret-token")
			},
		},
		"short first write falls back to wrapped": {
			// Shorter than 28 bytes: isPreStamped must not panic.
			timestamping: true,
			input:        "short\n",
			assertion: func(t *testing.T, output string) {
				assert.Contains(t, output, "short")
				assert.Contains(t, output, "Z ")
			},
		},
		"lookalike with Z at byte 26 falls back to wrapped": {
			timestamping: true,
			input:        lookalikeLine,
			assertion: func(t *testing.T, output string) {
				assert.Contains(t, output, "rest")
				assert.Contains(t, output, "Z ")
			},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			jt := newFakeJobTrace()
			l := newStepRunnerLogger(t, jt, tc.timestamping, tc.maskPhrases...)

			w := l.StepRunnerStream(StreamWorkLevel, Stdout)
			_, err := w.Write([]byte(tc.input))
			require.NoError(t, err)
			require.NoError(t, w.Close())

			tc.assertion(t, jt.Read())
		})
	}
}

// First write locks in the mode; subsequent writes follow the same path
// even if they happen not to look pre-stamped.
func TestStepRunnerStream_ChoicePersists(t *testing.T) {
	jt := newFakeJobTrace()
	l := newStepRunnerLogger(t, jt, true)

	w := l.StepRunnerStream(StreamWorkLevel, Stdout)
	_, err := w.Write([]byte(wireLine('O', "first\n")))
	require.NoError(t, err)
	_, err = w.Write([]byte("trailing-without-stamp\n"))
	require.NoError(t, err)
	require.NoError(t, w.Close())

	out := jt.Read()
	assert.Contains(t, out, "first")
	assert.Contains(t, out, "trailing-without-stamp\n")
	// Only one 'Z ' (the inner stamp on the first write); the trailing
	// write wasn't re-stamped because the stream is in passthrough.
	assert.Equal(t, 1, strings.Count(out, "Z "))
}

// In strip mode, stderr-typed lines reach the trace via the stderr wrap chain.
func TestStepRunnerStream_DemuxesStderr(t *testing.T) {
	jt := newFakeJobTrace()
	l := newStepRunnerLogger(t, jt, false)

	w := l.StepRunnerStream(StreamWorkLevel, Stdout)
	_, err := w.Write([]byte(wireLine('E', "diagnostic\n")))
	require.NoError(t, err)
	require.NoError(t, w.Close())

	assert.Contains(t, jt.Read(), "diagnostic")
}

func TestStepRunnerStream_CloseWithoutWrite(t *testing.T) {
	jt := newFakeJobTrace()
	l := newStepRunnerLogger(t, jt, true)

	w := l.StepRunnerStream(StreamWorkLevel, Stdout)
	require.NoError(t, w.Close())
	assert.Empty(t, jt.Read())
}
