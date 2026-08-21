//go:build !integration

package featureflags

import (
	"testing"

	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

func mockFlags(newFlags ...FeatureFlag) func() {
	oldFlags := flags
	flags = newFlags

	return func() {
		flags = oldFlags
	}
}

func TestGetAll(t *testing.T) {
	testFlag := FeatureFlag{Name: "TEST_FLAG", DefaultValue: true}

	defer mockFlags(testFlag)()

	f := GetAll()
	assert.Len(t, f, 1)
	assert.Contains(t, f, testFlag)
}

func TestIsOn(t *testing.T) {
	testCases := map[string]struct {
		testValue      string
		expectedResult bool
		expectedLog    bool
	}{
		"empty value": {
			testValue:      "",
			expectedResult: false,
		},
		"non boolean value": {
			testValue:      "a",
			expectedResult: false,
			expectedLog:    true,
		},
		"true value": {
			testValue:      "1",
			expectedResult: true,
		},
		"false value": {
			testValue:      "f",
			expectedResult: false,
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			logger, hook := logrustest.NewNullLogger()
			result := IsOn(logger, testCase.testValue)
			assert.Equal(t, testCase.expectedResult, result)
			if testCase.expectedLog {
				assert.NotNil(t, hook.LastEntry())
				assert.Contains(t, hook.LastEntry().Message, "Error while parsing the value of feature flag")
				return
			}

			assert.Nil(t, hook.LastEntry())
		})
	}
}

func TestIsOnFromEnv(t *testing.T) {
	const (
		defaultOffFlag = "FF_TEST_DEFAULT_OFF"
		defaultOnFlag  = "FF_TEST_DEFAULT_ON"
		unknownFlag    = "FF_TEST_UNKNOWN"
	)

	defer mockFlags(
		FeatureFlag{Name: defaultOffFlag, DefaultValue: false},
		FeatureFlag{Name: defaultOnFlag, DefaultValue: true},
	)()

	testCases := map[string]struct {
		flag           string
		envValue       string
		setEnv         bool
		expectedResult bool
		expectedLog    bool
	}{
		"default off, env unset": {
			flag:           defaultOffFlag,
			expectedResult: false,
		},
		"default on, env unset": {
			flag:           defaultOnFlag,
			expectedResult: true,
		},
		"default on, env empty": {
			flag:           defaultOnFlag,
			setEnv:         true,
			envValue:       "",
			expectedResult: true,
		},
		"default off, env true": {
			flag:           defaultOffFlag,
			setEnv:         true,
			envValue:       "true",
			expectedResult: true,
		},
		"default on, env false": {
			flag:           defaultOnFlag,
			setEnv:         true,
			envValue:       "false",
			expectedResult: false,
		},
		"default on, env malformed": {
			flag:           defaultOnFlag,
			setEnv:         true,
			envValue:       "a",
			expectedResult: true,
			expectedLog:    true,
		},
		"default off, env malformed": {
			flag:           defaultOffFlag,
			setEnv:         true,
			envValue:       "a",
			expectedResult: false,
			expectedLog:    true,
		},
		"unknown flag, env unset": {
			flag:           unknownFlag,
			expectedResult: false,
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			if testCase.setEnv {
				t.Setenv(testCase.flag, testCase.envValue)
			}

			logger, hook := logrustest.NewNullLogger()

			result := IsOnFromEnv(logger, testCase.flag)
			assert.Equal(t, testCase.expectedResult, result)

			if testCase.expectedLog {
				assert.NotNil(t, hook.LastEntry())
				assert.Contains(t, hook.LastEntry().Message, "Error while parsing the value of feature flag")
				return
			}

			assert.Nil(t, hook.LastEntry())
		})
	}
}
