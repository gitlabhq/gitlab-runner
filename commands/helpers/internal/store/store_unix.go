//go:build !windows

package store

import "os"

func openFile(pathname string) (*os.File, error) {
	return os.OpenFile(pathname, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0666)
}
