// +build linux darwin freebsd openbsd
package archives

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/unix"
)

func createTestPipe(t *testing.T) string {
	err := unix.Mkfifo("test_pipe", 0600)
	assert.NoError(t, err)
	return "test_pipe"
}
