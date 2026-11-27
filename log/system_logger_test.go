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

			sysLogger := newMockSystemLogger(t)
			sysService := newMockSystemService(t)
			sysService.On("SystemLogger", mock.Anything).Return(sysLogger, nil).Once()

			logger := logrus.New()
			logger.SetLevel(logrus.InfoLevel)
			logger.SetOutput(io.Discard)

			SetSystemLogger(logger, sysService)

			sysLogger.On(sysLoggerMethod, mock.Anything).Return(nil).Once().Run(func(args mock.Arguments) {
				msg := args.Get(0)
				assert.Contains(t, msg, fmt.Sprintf("msg=%q %s=%s", testMessage, key, value))
			})

			levels := map[logrus.Level]func(args ...any){
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

func TestServiceLogHookFollowsLoggerLevel(t *testing.T) {
	sysLogger := newMockSystemLogger(t)
	sysService := newMockSystemService(t)
	sysService.On("SystemLogger", mock.Anything).Return(sysLogger, nil).Once()

	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.SetOutput(io.Discard)

	SetSystemLogger(logger, sysService)

	logger.Debug("dropped at info level")

	// log_level from config.toml is applied after the hook is installed
	logger.SetLevel(logrus.DebugLevel)

	sysLogger.On("Info", mock.Anything).Return(nil).Once().Run(func(args mock.Arguments) {
		assert.Contains(t, args.Get(0), `level=debug msg="debug message"`)
	})

	logger.Debug("debug message")
}

func TestSetSystemLogger_ErrorOnInitialization(t *testing.T) {
	logger, hook := test.NewNullLogger()

	sysService := newMockSystemService(t)
	sysService.On("SystemLogger", mock.Anything).Return(nil, fmt.Errorf("test error")).Once()

	SetSystemLogger(logger, sysService)

	entry := hook.LastEntry()
	require.NotNil(t, entry)

	logrusOutput, err := entry.String()
	require.NoError(t, err)

	assert.Contains(t, logrusOutput, "Error while setting up the system logger")
	assert.Contains(t, logrusOutput, `error="test error"`)
}
