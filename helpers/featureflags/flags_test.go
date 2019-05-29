package featureflags

import (
	"testing"

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
	testFlag := FeatureFlag{Name: "TEST_FLAG", DefaultValue: "value"}

	defer mockFlags(testFlag)()

	f := GetAll()
	assert.Len(t, f, 1)
	assert.Contains(t, f, testFlag)
}

func TestIsOn(t *testing.T) {
	testCases := map[string]struct {
		testValue      string
		expectedResult bool
		expectedError  string
	}{
		"empty value": {
			testValue:      "",
			expectedResult: false,
		},
		"non boolean value": {
			testValue:      "a",
			expectedResult: false,
			expectedError:  `strconv.ParseBool: parsing "a": invalid syntax`,
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
			result, err := IsOn(testCase.testValue)
			assert.Equal(t, testCase.expectedResult, result)
			if testCase.expectedError != "" {
				assert.EqualError(t, err, testCase.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
