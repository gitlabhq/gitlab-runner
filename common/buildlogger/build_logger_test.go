//go:build !integration

package buildlogger

import (
	"bytes"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

type fakeJobTrace struct {
	buffer *bytes.Buffer
}

func (fjt *fakeJobTrace) Write(p []byte) (int, error) {
	return fjt.buffer.Write(p)
}

func (fjt *fakeJobTrace) IsStdout() bool {
	return false
}

func (fjt *fakeJobTrace) Read() string {
	return fjt.buffer.String()
}

func newFakeJobTrace() *fakeJobTrace {
	buf := new(bytes.Buffer)

	return &fakeJobTrace{
		buffer: buf,
	}
}

func newBuildLogger(testName string, jt Trace) Logger {
	return New(jt, logrus.WithField("test", testName))
}

func runOnHijackedLogrusOutput(t *testing.T, handler func(t *testing.T, output *bytes.Buffer)) {
	oldOutput := logrus.StandardLogger().Out
	defer func() { logrus.StandardLogger().Out = oldOutput }()

	buf := bytes.NewBuffer([]byte{})
	logrus.StandardLogger().Out = buf

	handler(t, buf)
}

func TestLogLineWithoutSecret(t *testing.T) {
	runOnHijackedLogrusOutput(t, func(t *testing.T, output *bytes.Buffer) {
		jt := newFakeJobTrace()

		l := newBuildLogger("log-line-without-secret", jt)

		l.Errorln("Fatal: Get http://localhost/?id=123")
		assert.Contains(t, jt.Read(), `Get http://localhost/?id=123`)
		assert.Contains(t, output.String(), `Get http://localhost/?id=123`)
	})
}

func TestLogLineWithSecret(t *testing.T) {
	runOnHijackedLogrusOutput(t, func(t *testing.T, output *bytes.Buffer) {
		jt := newFakeJobTrace()

		l := newBuildLogger("log-line-with-secret", jt)

		l.Errorln("Get http://localhost/?id=123&X-Amz-Signature=abcd1234&private_token=abcd1234")
		assert.Contains(
			t,
			jt.Read(),
			`Get http://localhost/?id=123&X-Amz-Signature=[FILTERED]&private_token=[FILTERED]`,
		)
		assert.Contains(
			t,
			output.String(),
			`Get http://localhost/?id=123&X-Amz-Signature=abcd1234&private_token=abcd1234`,
		)
	})
}

func TestLogPrinters(t *testing.T) {
	tests := map[string]struct {
		entry     *logrus.Entry
		assertion func(t *testing.T, output string)
	}{
		"null writer": {
			entry: nil,
			assertion: func(t *testing.T, output string) {
				assert.Empty(t, output)
			},
		},
		"with entry": {
			entry: logrus.WithField("printer", "test"),
			assertion: func(t *testing.T, output string) {
				assert.Contains(t, output, "print\033[0;m\n")
				assert.Contains(t, output, "info\033[0;m\n")
				assert.Contains(t, output, "WARNING: warning\033[0;m\n")
				assert.Contains(t, output, "ERROR: softerror\033[0;m\n")
				assert.Contains(t, output, "ERROR: error\033[0;m\n")
			},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			trace := newFakeJobTrace()

			logger := New(trace, tc.entry)

			logger.Println("print")
			logger.Infoln("info")
			logger.Warningln("warning")
			logger.SoftErrorln("softerror")
			logger.Errorln("error")

			tc.assertion(t, trace.Read())
		})
	}
}
