//go:build !integration

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
			if tt.is {
				assert.ErrorIs(t, tt.err, tt.target)
				return
			}

			assert.NotErrorIs(t, tt.err, tt.target)
		})
	}
}

func TestUnwrapBuildError(t *testing.T) {
	err := &BuildError{Inner: assert.AnError}
	// Unwraps inner error
	assert.ErrorIs(t, err, assert.AnError)

	// Stop unwrapping until BuildError is found.
	assert.ErrorIs(t, err, &BuildError{})
	var buildErr *BuildError
	assert.ErrorAs(t, err, &buildErr)

	err = &BuildError{}
	// Unwraps inner error
	assert.NotErrorIs(t, err, assert.AnError)

	// Stop unwrapping until BuildError is found.
	assert.ErrorIs(t, err, &BuildError{})
	assert.ErrorAs(t, err, &buildErr)
}
