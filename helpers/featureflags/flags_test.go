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
				assert.Contains(t, "Error while parsing the value of feature flag", hook.LastEntry().Message)
				return
			}

			assert.Nil(t, hook.LastEntry())
		})
	}
}
