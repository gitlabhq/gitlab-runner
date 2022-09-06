//go:build !integration && !windows

package custom

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExecutor_Connect(t *testing.T) {
	e := new(executor)
	connection, err := e.Connect()

	assert.Nil(t, connection)
	assert.EqualError(t, err, "not yet supported")
}
