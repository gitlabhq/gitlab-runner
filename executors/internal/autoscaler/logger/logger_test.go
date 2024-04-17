//go:build !integration

package logger

import (
	"bytes"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestLogger(t *testing.T) {
	buf := new(bytes.Buffer)

	l := logrus.StandardLogger()
	l.Out = buf

	logger := New(logrus.NewEntry(l))
	logger.SetLevel(hclog.Trace)
	require.Equal(t, hclog.Trace, logger.GetLevel())

	logger.Trace("trace", "one", "two")
	logger.Debug("debug", "three", "four")
	logger.Info("info", "five", "six")
	logger.Warn("warn", "seven", "eight")
	logger.Error("error", "nine", "ten")

	subsystem := logger.Named("newname")
	subsystem.Info("info")
	subsystem.Named("another").Info("info")

	logger.Info("unbalanced", "key")

	require.Contains(t, buf.String(), "level=trace msg=trace one=two")
	require.Contains(t, buf.String(), "level=debug msg=debug three=four")
	require.Contains(t, buf.String(), "level=info msg=info five=six")
	require.Contains(t, buf.String(), "level=warning msg=warn seven=eight")
	require.Contains(t, buf.String(), "level=error msg=error nine=ten")
	require.Contains(t, buf.String(), "level=info msg=info subsystem=newname")
	require.Contains(t, buf.String(), "level=info msg=info subsystem=newname.another")
	require.Contains(t, buf.String(), "level=info msg=unbalanced key=\"<unknown>\"")
}
