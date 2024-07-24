//go:build !windows

package os

import "runtime"

func LocalKernelVersion() string {
	panic("not imeplemented for " + runtime.GOOS)
}
