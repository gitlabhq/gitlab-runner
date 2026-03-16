//go:build !windows

package store

import "os"

func openFile(pathname string) (*os.File, error) {
	// Check if file exists before opening
	_, err := os.Stat(pathname)
	isNewFile := os.IsNotExist(err)

	f, err := os.OpenFile(pathname, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return nil, err
	}

	// Only chmod if we just created the file
	if isNewFile {
		if err := os.Chmod(pathname, 0666); err != nil {
			f.Close()
			return nil, err
		}
	}

	return f, nil
}
