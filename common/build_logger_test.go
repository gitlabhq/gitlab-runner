// +build !integration

package common

import (
	"bytes"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type fakeJobTrace struct {
	*MockJobTrace
	buffer *bytes.Buffer
}

func (fjt *fakeJobTrace) Read() string {
	return fjt.buffer.String()
}

func newFakeJobTrace() fakeJobTrace {
	e := new(MockJobTrace)
	buf := new(bytes.Buffer)

	e.On("IsStdout").Return(false).Maybe()
	call := e.On("Write", mock.Anything).Maybe()

	call.RunFn = func(args mock.Arguments) {
		n, err := buf.Write(args.Get(0).([]byte))
		call.ReturnArguments = mock.Arguments{n, err}
	}

	return fakeJobTrace{
		MockJobTrace: e,
		buffer:       buf,
	}
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
				assert.Contains(t, output, "print\n")
				assert.Contains(t, output, "info\n")
				assert.Contains(t, output, "WARNING: warning\n")
				assert.Contains(t, output, "ERROR: softerror\n")
				assert.Contains(t, output, "ERROR: error\n")
			},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			trace := newFakeJobTrace()
			defer trace.AssertExpectations(t)

			logger := NewBuildLogger(trace, tc.entry)

			logger.Println("print")
			logger.Infoln("info")
			logger.Warningln("warning")
			logger.SoftErrorln("softerror")
			logger.Errorln("error")

			tc.assertion(t, trace.Read())
		})
	}
}
