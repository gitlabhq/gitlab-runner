//go:build !integration && !windows

package archives

import (
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

func createTestPipe(t *testing.T, csb charsetByte) string {
	name := "test_pipe"
	if csb == multiBytes {
		name = "テストパイプ"
	}

	err := syscall.Mkfifo(name, 0600)
	assert.NoError(t, err)
	return name
}
