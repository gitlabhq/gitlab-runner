//go:build windows

package archives

import (
	"os"
)

func lchmod(name string, mode os.FileMode) error {
	if mode&os.ModeSymlink != 0 {
		return nil
	}
	return os.Chmod(name, mode.Perm())
}
