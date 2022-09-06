//go:build !integration

package log

import (
	"fmt"
	"io"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestServiceLogHook(t *testing.T) {
	key := "key"
	value := "value"
	testMessage := "test message"

	tests := map[logrus.Level]string{
		logrus.InfoLevel:  "Info",
		logrus.WarnLevel:  "Warning",
		logrus.ErrorLevel: "Error",
		// Fatal is skipped by purpose
		//
		// There is no way to disable or overwrite the `Exit(1)` called by logrus
		// at the end of `Fatal` logger. We have our helpers.MakeFatalToPanic
		// hook, but it doesn't work reliable here.
		//
		// We just need to assume, that if all other levels are working properly, then
		// `Fatal` will also work. In the end, it's just another entry in the systemLogger
		// method selector.
		logrus.PanicLevel: "Error",
	}

	for level, sysLoggerMethod := range tests {
		t.Run(fmt.Sprintf("level-%s", level), func(t *testing.T) {
			defer func() {
				_ = recover()
			}()

			sysLogger := new(mockSystemLogger)
			defer sysLogger.AssertExpectations(t)

			sysService := new(mockSystemService)
			defer sysService.AssertExpectations(t)
			sysService.On("SystemLogger", mock.Anything).Return(sysLogger, nil).Once()

			logger := logrus.New()
			logger.SetLevel(logrus.InfoLevel)
			logger.SetOutput(io.Discard)

			SetSystemLogger(logger, sysService)

			sysLogger.On(sysLoggerMethod, mock.Anything).Return(nil).Once().Run(func(args mock.Arguments) {
				msg := args.Get(0)
				assert.Contains(t, msg, fmt.Sprintf("msg=%q %s=%s", testMessage, key, value))
			})

			levels := map[logrus.Level]func(args ...interface{}){
				logrus.PanicLevel: logger.WithField(key, value).Panic,
				logrus.ErrorLevel: logger.WithField(key, value).Error,
				logrus.WarnLevel:  logger.WithField(key, value).Warning,
				logrus.InfoLevel:  logger.WithField(key, value).Info,
				logrus.DebugLevel: logger.WithField(key, value).Debug,
			}

			levelLogger, ok := levels[level]
			require.True(t, ok, "Unknown level %v used", level)

			levelLogger(testMessage)
		})
	}
}

func TestServiceLogHookWithSpecifiedLevel(t *testing.T) {
	// Disable colors to avoid any OS specific formatting.
	formatter := &logrus.TextFormatter{DisableColors: true}

	logger := logrus.New()
	logger.Formatter = formatter

	entry := logrus.NewEntry(logger)
	entry.Message = "test message"

	sysLogger := new(mockSystemLogger)
	defer sysLogger.AssertExpectations(t)

	assertSysLoggerMethod := func(args mock.Arguments) {
		msg := args.Get(0)
		assert.Contains(t, msg, `msg="test message"`)
	}

	sysLogger.On("Error", mock.Anything).Return(nil).Once().Run(assertSysLoggerMethod)
	sysLogger.On("Warning", mock.Anything).Return(nil).Once().Run(assertSysLoggerMethod)

	hook := new(SystemServiceLogHook)
	hook.systemLogger = sysLogger
	hook.Level = logrus.WarnLevel

	for _, level := range []logrus.Level{
		logrus.ErrorLevel,
		logrus.WarnLevel,
		logrus.InfoLevel,
	} {
		entry.Level = level
		err := hook.Fire(entry)

		assert.NoError(t, err)
	}
}

func TestSetSystemLogger_ErrorOnInitialization(t *testing.T) {
	logger, hook := test.NewNullLogger()

	sysService := new(mockSystemService)
	defer sysService.AssertExpectations(t)
	sysService.On("SystemLogger", mock.Anything).Return(nil, fmt.Errorf("test error")).Once()

	SetSystemLogger(logger, sysService)

	entry := hook.LastEntry()
	require.NotNil(t, entry)

	logrusOutput, err := entry.String()
	require.NoError(t, err)

	assert.Contains(t, logrusOutput, "Error while setting up the system logger")
	assert.Contains(t, logrusOutput, `error="test error"`)
}
