// +build aix android darwin dragonfly freebsd hurd illumos linux netbsd openbsd solaris

package fd0

import (
	"syscall"
)

func AssertFixPresent() {
	// Ensure that Fd-0 fixed runtime is used
	syscall.Fd0Fix()
}
