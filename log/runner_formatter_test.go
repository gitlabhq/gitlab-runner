//go:build !integration

package log

import (
	"fmt"
	"io"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

func newNullLogger(formatter logrus.Formatter, level logrus.Level) *logrus.Logger {
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	logger.SetFormatter(formatter)
	logger.SetLevel(level)

	return logger
}

type colorsAndPrefixesTestCase struct {
	expectedPrefix    string
	expectedColorCode string
}

func TestRunnerTextFormatter_ColorsAndPrefixes(t *testing.T) {
	logrus.RegisterExitHandler(func() {
		panic("Fatal logged")
	})

	key := "key"
	value := "value"
	fields := logrus.Fields{
		key: value,
	}

	tests := map[logrus.Level]colorsAndPrefixesTestCase{
		logrus.PanicLevel: {
			expectedPrefix:    "PANIC: ",
			expectedColorCode: helpers.ANSI_BOLD_RED,
		},
		// Fatal is skipped by purpose
		//
		// There is no way to disable or overwrite the `Exit(1)` called by logrus
		// at the end of `Fatal` logger. We have our helpers.MakeFatalToPanic
		// hook, but in this case it is unusable: hooks are fired before the formatting
		// is done, and this is what we would like to test.
		//
		// We just need to assume, that if all other levels are working properly, then
		// `Fatal` will also work. In the end, it's just another entry in the prefix/color
		// choosing method.
		logrus.ErrorLevel: {
			expectedPrefix:    "ERROR: ",
			expectedColorCode: helpers.ANSI_BOLD_RED,
		},
		logrus.WarnLevel: {
			expectedPrefix:    "WARNING: ",
			expectedColorCode: helpers.ANSI_YELLOW,
		},
		logrus.InfoLevel: {},
		logrus.DebugLevel: {
			expectedColorCode: helpers.ANSI_BOLD_WHITE,
		},
	}

	for level, testCase := range tests {
		for _, colored := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s-level colored-%v", level.String(), colored), func(t *testing.T) {
				formatter := new(RunnerTextFormatter)
				formatter.DisableColors = !colored

				logger := newNullLogger(formatter, logrus.DebugLevel)

				hook := test.NewLocal(logger)

				defer testOutputColoringAndPrefix(t, key, value, testCase, colored, hook)

				levels := map[logrus.Level]func(args ...interface{}){
					logrus.PanicLevel: logger.WithFields(fields).Panic,
					logrus.ErrorLevel: logger.WithFields(fields).Error,
					logrus.WarnLevel:  logger.WithFields(fields).Warning,
					logrus.InfoLevel:  logger.WithFields(fields).Info,
					logrus.DebugLevel: logger.WithFields(fields).Debug,
				}

				levelLogger, ok := levels[level]
				require.True(t, ok, "Unknown level %v used", level)

				levelLogger("test message")
			})
		}
	}
}

func testOutputColoringAndPrefix(
	t *testing.T,
	key string,
	value string,
	testCase colorsAndPrefixesTestCase,
	colored bool,
	hook *test.Hook,
) {
	_ = recover()

	entry := hook.LastEntry()
	require.NotNil(t, entry)

	logrusOutput, err := entry.String()
	require.NoError(t, err)

	if testCase.expectedPrefix != "" {
		assert.Contains(t, logrusOutput, testCase.expectedPrefix)
	}

	if colored {
		if testCase.expectedColorCode != "" {
			assert.Contains(t, logrusOutput, testCase.expectedColorCode, "Should contain color code")
		}
		assert.Contains(t, logrusOutput, helpers.ANSI_RESET, "Should contain reset color code")
		assert.Contains(
			t,
			logrusOutput,
			fmt.Sprintf("%s%s%s=%s", testCase.expectedColorCode, key, helpers.ANSI_RESET, value),
			"Should color field key",
		)
	} else {
		if testCase.expectedColorCode != "" {
			assert.NotContains(t, logrusOutput, testCase.expectedColorCode, "Shouldn't contain color code")
		}
		assert.NotContains(t, logrusOutput, helpers.ANSI_RESET, "Shouldn't contain reset color code")
		assert.Contains(t, logrusOutput, fmt.Sprintf("%s=%s", key, value), "Shouldn't color field key")
	}
}

func TestRunnerTextFormatter_KeysSorting(t *testing.T) {
	fields := logrus.Fields{
		"aza": "v",
		"zzz": "v",
		"zaz": "v",
		"aaa": "v",
	}

	formatter := new(RunnerTextFormatter)
	formatter.DisableColors = true
	formatter.DisableSorting = false

	logger := newNullLogger(formatter, logrus.InfoLevel)
	hook := test.NewLocal(logger)

	for i := 0; i <= 2; i++ {
		logger.WithFields(fields).Info("test message")

		entry := hook.LastEntry()
		require.NotNil(t, entry)

		logrusOutput, err := entry.String()
		require.NoError(t, err)

		assert.Contains(t, logrusOutput, " aaa=v aza=v zaz=v zzz=v")
	}
}
