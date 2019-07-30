package fslocker

import (
	"github.com/pkg/errors"
)

func errFileInUse(path string) error {
	return errors.Errorf("file %q is locked by another process", path)
}

func errCantAcquireLock(inner error, path string) error {
	return errors.Wrapf(inner, "can't acquire file lock on %q file", path)
}

func errCantReleaseLock(inner error, path string) error {
	return errors.Wrapf(inner, "can't release file lock on %q file", path)
}
