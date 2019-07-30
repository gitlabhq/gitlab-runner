package fslocker

import (
	"github.com/gofrs/flock"
)

type locker interface {
	TryLock() (bool, error)
	Unlock() error
}

var defaultLocker = func(path string) locker {
	return flock.New(path)
}

func InLock(filePath string, fn func()) (err error) {
	locker := defaultLocker(filePath)

	lock, err := locker.TryLock()
	if err != nil {
		return errCantAcquireLock(err, filePath)
	}

	if !lock {
		return errFileInUse(filePath)
	}

	defer func() {
		unlockErr := locker.Unlock()
		if unlockErr != nil {
			err = errCantReleaseLock(unlockErr, filePath)
		}
	}()

	fn()

	return
}
