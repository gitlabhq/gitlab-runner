//go:build !integration && windows
// +build !integration,windows

package archives

import (
	"testing"
)

func createTestPipe(t *testing.T, csb charsetByte) string {
	panic("unsupported - this should not be called")
}
