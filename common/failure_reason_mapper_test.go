//go:build !integration

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFailureReasonMapper_Map(t *testing.T) {
	const (
		frOne   JobFailureReason = "fr_one"
		frTwo   JobFailureReason = "fr_two"
		frThree JobFailureReason = "fr_three"
		frFour  JobFailureReason = "fr_four"
		frFive  JobFailureReason = "fr_five"
		frSix   JobFailureReason = "fr_six"
		frSeven JobFailureReason = "fr_seven"
		frEight JobFailureReason = "fr_eight"

		frLoopOne   JobFailureReason = "fr_loop_one"
		frLoopTwo   JobFailureReason = "fr_loop_two"
		frLoopThree JobFailureReason = "fr_loop_three"
		frLoopFour  JobFailureReason = "fr_loop_four"

		frTotallyUnknown JobFailureReason = "fr_totally_unknown"

		maxDepth = 3
	)

	supported := []JobFailureReason{frOne, frTwo}
	compatibilityMap := map[JobFailureReason]JobFailureReason{
		frThree: frOne,
		frFive:  frFour,
		frFour:  frTwo,
		frSeven: frSix,
		frEight: frSeven,

		frLoopOne:   frLoopOne,
		frLoopFour:  frLoopThree,
		frLoopThree: frLoopTwo,
		frLoopTwo:   frLoopThree,
	}

	tests := map[string]struct {
		run func(t *testing.T, f *failureReasonMapper)
	}{
		"default failure": {
			run: func(t *testing.T, f *failureReasonMapper) {
				assert.Equal(t, ScriptFailure, f.Map(""))
				assert.NoError(t, f.err)
			},
		},

		"always supported by GitLab": {
			run: func(t *testing.T, f *failureReasonMapper) {
				assert.Equal(t, ScriptFailure, f.Map(ScriptFailure))
				assert.Equal(t, RunnerSystemFailure, f.Map(RunnerSystemFailure))
				assert.Equal(t, JobExecutionTimeout, f.Map(JobExecutionTimeout))
				assert.NoError(t, f.err)
			},
		},

		"optionally supported by GitLab": {
			run: func(t *testing.T, f *failureReasonMapper) {
				assert.Equal(t, frOne, f.Map(frOne))
				assert.Equal(t, frTwo, f.Map(frTwo))
				assert.NoError(t, f.err)
			},
		},

		"unsupported by GitLab": {
			run: func(t *testing.T, f *failureReasonMapper) {
				assert.Equal(t, UnknownFailure, f.Map(frSix))
				assert.NoError(t, f.err)
			},
		},

		"new directly mapped to older supported by GitLab": {
			run: func(t *testing.T, f *failureReasonMapper) {
				assert.Equal(t, frOne, f.Map(frThree))
				assert.Equal(t, frTwo, f.Map(frFour))
				assert.NoError(t, f.err)
			},
		},

		"new indirectly mapped to older supported by GitLab": {
			run: func(t *testing.T, f *failureReasonMapper) {
				assert.Equal(t, frTwo, f.Map(frFive))
				assert.NoError(t, f.err)
			},
		},

		"directly mapped to unsupported by GitLab": {
			run: func(t *testing.T, f *failureReasonMapper) {
				assert.Equal(t, UnknownFailure, f.Map(frSeven))
				assert.NoError(t, f.err)
			},
		},

		"indirectly mapped to unsupported by GitLab": {
			run: func(t *testing.T, f *failureReasonMapper) {
				assert.Equal(t, UnknownFailure, f.Map(frEight))
				assert.NoError(t, f.err)
			},
		},

		"totally unknown reason": {
			run: func(t *testing.T, f *failureReasonMapper) {
				assert.Equal(t, UnknownFailure, f.Map(frTotallyUnknown))
				assert.NoError(t, f.err)
			},
		},

		"endless direct loop": {
			run: func(t *testing.T, f *failureReasonMapper) {
				assert.Equal(t, UnknownFailure, f.Map(frLoopOne))
				assert.ErrorIs(t, f.err, errMaxMappingDepthExceeded)
			},
		},

		"endless indirect loop": {
			run: func(t *testing.T, f *failureReasonMapper) {
				assert.Equal(t, UnknownFailure, f.Map(frLoopFour))
				assert.ErrorIs(t, f.err, errMaxMappingDepthExceeded)
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			f := newFailureReasonMapper(supported)
			f.compatibilityMap = compatibilityMap
			f.maxMappingDepth = maxDepth

			tt.run(t, f)
		})
	}
}

// This tests checks if the hardcoded compatibility map introduces
// mapping loops or exceeds mapping depth. In case of failures, mapping
// should be fixed before introducing the change to the main branch
// and releasing.
func TestFailureReasonsCompatibilityMap(t *testing.T) {
	f := newFailureReasonMapper(nil)
	require.Equal(t, failureReasonsCompatibilityMap, f.compatibilityMap)

	for _, r := range allFailureReasons {
		t.Run(string(r), func(t *testing.T) {
			f.Map(r)
			assert.NoError(t, f.err)
		})
	}
}
