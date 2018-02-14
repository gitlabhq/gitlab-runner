package archives

import "testing"

func createTestPipe(t *testing.T) string {
	t.Skip("mkfifo not implemented on windows")
	return ""
}
