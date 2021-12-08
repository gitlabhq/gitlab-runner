// +build aix android darwin dragonfly freebsd hurd illumos linux netbsd openbsd solaris

package issue_28732

import (
	"syscall"
)

func AssertFixPresent() {
	// Ensure that Issue28732Fix fixed runtime is used
	syscall.Issue28732Fix()
}
