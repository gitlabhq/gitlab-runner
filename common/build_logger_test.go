package common

import (
	"bytes"
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

type fakeJobTrace struct {
	buffer *bytes.Buffer
}

func (fjt *fakeJobTrace) Success()                                       {}
func (fjt *fakeJobTrace) Fail(err error, failureReason JobFailureReason) {}
func (fjt *fakeJobTrace) SetCancelFunc(context.CancelFunc)               {}
func (fjt *fakeJobTrace) Cancel() bool                                   { return false }
func (fjt *fakeJobTrace) SetAbortFunc(context.CancelFunc)                {}
func (fjt *fakeJobTrace) Abort() bool                                    { return false }
func (fjt *fakeJobTrace) SetFailuresCollector(fc FailuresCollector)      {}
func (fjt *fakeJobTrace) SetMasked(masked []string)                      {}
func (fjt *fakeJobTrace) IsStdout() bool                                 { return false }

func (fjt *fakeJobTrace) Write(p []byte) (n int, err error) {
	return fjt.buffer.Write(p)
}

func (fjt *fakeJobTrace) Read() string {
	return fjt.buffer.String()
}

func newFakeJobTrace() *fakeJobTrace {
	fjt := &fakeJobTrace{
		buffer: bytes.NewBuffer([]byte{}),
	}

	return fjt
}

func newBuildLogger(testName string, jt JobTrace) BuildLogger {
	return BuildLogger{
		log:   jt,
		entry: logrus.WithField("test", testName),
	}
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
