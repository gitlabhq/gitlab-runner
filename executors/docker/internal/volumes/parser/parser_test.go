package parser

import (
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker/errors"
)

func TestNew(t *testing.T) {
	testCases := map[string]struct {
		expectedParserType interface{}
		expectedError      error
	}{
		OSTypeLinux:   {expectedParserType: &linuxParser{}, expectedError: nil},
		OSTypeWindows: {expectedParserType: &windowsParser{}, expectedError: nil},
		"unsupported": {expectedParserType: nil, expectedError: errors.NewErrOSNotSupported("unsupported")},
	}

	for osType, testCase := range testCases {
		t.Run(osType, func(t *testing.T) {
			parser, err := New(types.Info{OSType: osType})

			assert.IsType(t, testCase.expectedParserType, parser)

			if testCase.expectedError == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, testCase.expectedError.Error())
			}
		})
	}
}
