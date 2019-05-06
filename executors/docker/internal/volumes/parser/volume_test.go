package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVolume_Definition(t *testing.T) {
	testCases := map[string]struct {
		volume         *Volume
		expectedOutput string
	}{
		"only destination": {
			volume:         &Volume{Destination: "destination"},
			expectedOutput: "destination",
		},
		"source and destination": {
			volume:         &Volume{Source: "source", Destination: "destination"},
			expectedOutput: "source:destination",
		},
		"destination and mode": {
			volume:         &Volume{Destination: "destination", Mode: "mode"},
			expectedOutput: "destination:mode",
		},
		"all values": {
			volume:         &Volume{Source: "source", Destination: "destination", Mode: "mode"},
			expectedOutput: "source:destination:mode",
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			output := testCase.volume.Definition()
			assert.Equal(t, testCase.expectedOutput, output)
		})
	}
}

func TestVolume_Len(t *testing.T) {
	testCases := map[string]struct {
		volume      *Volume
		expectedLen int
	}{
		"empty": {
			volume:      &Volume{},
			expectedLen: 0,
		},
		"only destination": {
			volume:      &Volume{Destination: "destination"},
			expectedLen: 1,
		},
		"source and destination": {
			volume:      &Volume{Source: "source", Destination: "destination"},
			expectedLen: 2,
		},
		"destination and mode": {
			volume:      &Volume{Destination: "destination", Mode: "mode"},
			expectedLen: 2,
		},
		"all values": {
			volume:      &Volume{Source: "source", Destination: "destination", Mode: "mode"},
			expectedLen: 3,
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			len := testCase.volume.Len()
			assert.Equal(t, testCase.expectedLen, len)
		})
	}
}
