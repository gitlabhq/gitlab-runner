//go:build !integration

package log

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

func prepareFakeConfiguration(logger *logrus.Logger) func() {
	oldConfiguration := configuration
	configuration = NewConfig(logger)

	return func() {
		configuration = oldConfiguration
		configuration.ReloadConfiguration()
	}
}

func testCommandRun(args ...string) {
	app := cli.NewApp()
	app.Commands = []cli.Command{
		{
			Name:   "logtest",
			Action: func(cliCtx *cli.Context) {},
		},
	}

	ConfigureLogging(app)

	args = append([]string{"binary"}, args...)
	args = append(args, "logtest")

	_ = app.Run(args)
}

type handleCliCtxTestCase struct {
	args                       []string
	expectedError              string
	expectedLevel              logrus.Level
	expectedFormatter          logrus.Formatter
	expectedLevelSetWithCli    bool
	expectedFormatSetWithCli   bool
	goroutinesDumpStopChExists bool
}

func TestHandleCliCtx(t *testing.T) {
	tests := map[string]handleCliCtxTestCase{
		"no configuration specified": {
			expectedLevel:     logrus.InfoLevel,
			expectedFormatter: new(RunnerTextFormatter),
		},
		"--log-level specified": {
			args:                    []string{"--log-level", "error"},
			expectedLevel:           logrus.ErrorLevel,
			expectedFormatter:       new(RunnerTextFormatter),
			expectedLevelSetWithCli: true,
		},
		"--debug specified": {
			args:                       []string{"--debug"},
			expectedLevel:              logrus.DebugLevel,
			expectedFormatter:          new(RunnerTextFormatter),
			expectedLevelSetWithCli:    true,
			goroutinesDumpStopChExists: true,
		},
		"--log-level and --debug specified": {
			args:                       []string{"--log-level", "error", "--debug"},
			expectedLevel:              logrus.DebugLevel,
			expectedFormatter:          new(RunnerTextFormatter),
			expectedLevelSetWithCli:    true,
			goroutinesDumpStopChExists: true,
		},
		"invalid --log-level specified": {
			args:          []string{"--log-level", "test"},
			expectedError: "failed to parse log level",
		},
		"--log-format specified": {
			args:                     []string{"--log-format", "json"},
			expectedLevel:            logrus.InfoLevel,
			expectedFormatter:        new(logrus.JSONFormatter),
			expectedFormatSetWithCli: true,
		},
		"invalid --log-format specified": {
			args:          []string{"--log-format", "test"},
			expectedError: "unknown log format",
		},
	}

	for name, testCase := range tests {
		t.Run(name, func(t *testing.T) {
			logger, _ := test.NewNullLogger()

			defer prepareFakeConfiguration(logger)()
			defer helpers.MakeFatalToPanic()()

			testFunc := func() {
				testCommandRun(testCase.args...)
				if testCase.expectedError == "" {
					assert.Equal(t, testCase.expectedLevel, Configuration().level)
					assert.Equal(t, testCase.expectedFormatter, Configuration().format)
					assert.Equal(t, testCase.expectedLevelSetWithCli, Configuration().IsLevelSetWithCli())
					assert.Equal(t, testCase.expectedFormatSetWithCli, Configuration().IsFormatSetWithCli())

					if testCase.goroutinesDumpStopChExists {
						assert.NotNil(t, Configuration().goroutinesDumpStopCh)
					} else {
						assert.Nil(t, Configuration().goroutinesDumpStopCh)
					}
				}
			}

			if testCase.expectedError != "" {
				var message *logrus.Entry
				var ok bool

				func() {
					defer func() {
						message, ok = recover().(*logrus.Entry)
					}()

					testFunc()
				}()

				require.True(t, ok)

				panicMessage, err := message.String()
				require.NoError(t, err)

				assert.Contains(t, panicMessage, "Error while setting up logging configuration")
				assert.Contains(t, panicMessage, testCase.expectedError)
			} else {
				assert.NotPanics(t, testFunc)
			}
		})
	}
}

func TestGoroutinesDumpDisabling(t *testing.T) {
	logger, _ := test.NewNullLogger()

	config := NewConfig(logger)
	config.level = logrus.DebugLevel
	config.ReloadConfiguration()
	config.ReloadConfiguration()

	assert.NotNil(t, config.goroutinesDumpStopCh)

	config.level = logrus.InfoLevel
	config.ReloadConfiguration()
	config.ReloadConfiguration()

	assert.Nil(t, config.goroutinesDumpStopCh)
}
