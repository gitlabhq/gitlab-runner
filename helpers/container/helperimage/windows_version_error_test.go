package helperimage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestError(t *testing.T) {
	for _, expectedVersion := range []string{"1803", "1809"} {
		err := NewUnsupportedWindowsVersionError(expectedVersion)
		assert.Equal(t, expectedVersion, err.version)
	}
}
