//go:build windows

package store

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// openFile is like os.OpenFile, but adds FILE_SHARE_DELETE, allowing the file
// to be deleted, even when open, on Windows.
func openFile(pathname string) (*os.File, error) {
	p, err := windows.UTF16PtrFromString(pathname)
	if err != nil {
		return nil, fmt.Errorf("converting pathname to UTF16: %w", err)
	}

	h, err := windows.CreateFile(
		p,
		windows.GENERIC_READ|windows.FILE_APPEND_DATA,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_ALWAYS,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("creating file share file: %w", err)
	}

	return os.NewFile(uintptr(h), pathname), nil
}
