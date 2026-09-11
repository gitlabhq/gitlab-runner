//go:build !integration

package steps

import (
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSigExitCode(t *testing.T) {
	// 128+N is the convention the runner and allow_failure:exit_codes match
	// against; a signal that carries no number falls back to a generic failure.
	assert.Equal(t, 143, sigExitCode(syscall.SIGTERM))
	assert.Equal(t, 130, sigExitCode(syscall.SIGINT))
	assert.Equal(t, 1, sigExitCode(numberlessSignal{}))
}

// numberlessSignal is an os.Signal that is not a syscall.Signal.
type numberlessSignal struct{}

func (numberlessSignal) String() string { return "numberless" }
func (numberlessSignal) Signal()        {}
