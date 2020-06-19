package common

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildErrorIs(t *testing.T) {
	tests := map[string]struct {
		err    error
		target error
		is     bool
	}{
		"two build errors with the same failure reason": {
			err:    &BuildError{FailureReason: ScriptFailure},
			target: &BuildError{FailureReason: ScriptFailure},
			is:     true,
		},
		"different failure reasons": {
			err:    &BuildError{FailureReason: ScriptFailure},
			target: &BuildError{FailureReason: RunnerSystemFailure},
			is:     false,
		},
		"not matching errors": {
			err:    &BuildError{},
			target: errors.New("mysterious error"),
			is:     false,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			assert.Equal(t, tt.is, errors.Is(tt.err, tt.target))
		})
	}
}
