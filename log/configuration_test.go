package log

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

func prepareFakeConfiguration() func() {
	oldConfiguration := configuration
	configuration = NewConfig()

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

	app.Run(args)
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

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			defer prepareFakeConfiguration()()
			defer helpers.MakeFatalToPanic()()

			testFunc := func() {
				testCommandRun(test.args...)
				if test.expectedError == "" {
					assert.Equal(t, test.expectedLevel, Configuration().level)
					assert.Equal(t, test.expectedFormatter, Configuration().format)
					assert.Equal(t, test.expectedLevelSetWithCli, Configuration().IsLevelSetWithCli())
					assert.Equal(t, test.expectedFormatSetWithCli, Configuration().IsFormatSetWithCli())

					if test.goroutinesDumpStopChExists {
						assert.NotNil(t, Configuration().goroutinesDumpStopCh)
					} else {
						assert.Nil(t, Configuration().goroutinesDumpStopCh)
					}
				}
			}

			if test.expectedError != "" {
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
				assert.Contains(t, panicMessage, test.expectedError)

			} else {
				assert.NotPanics(t, testFunc)
			}
		})
	}
}

func TestGoroutinesDumpDisabling(t *testing.T) {
	config := new(Config)
	config.level = logrus.DebugLevel
	config.ReloadConfiguration()
	config.ReloadConfiguration()

	assert.NotNil(t, config.goroutinesDumpStopCh)

	config.level = logrus.InfoLevel
	config.ReloadConfiguration()
	config.ReloadConfiguration()

	assert.Nil(t, config.goroutinesDumpStopCh)
}
