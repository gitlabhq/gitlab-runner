package fastzip

import "time"

// fastzip does not set symlink times or owners on Windows.

func lchtimes(string, time.Time) error { return nil }

func lchown(string, int, int) error { return nil }
