//go:build !windows

package fastzip

import (
	"os"
	"time"

	"golang.org/x/sys/unix"
)

func lchtimes(path string, mtime time.Time) error {
	tv := []unix.Timeval{unix.NsecToTimeval(time.Now().UnixNano()), unix.NsecToTimeval(mtime.UnixNano())}
	if err := unix.Lutimes(path, tv); err != nil {
		return &os.PathError{Op: "lchtimes", Path: path, Err: err}
	}

	return nil
}

func lchown(path string, uid, gid int) error {
	return os.Lchown(path, uid, gid)
}
