//go:build aix || android || darwin || dragonfly || freebsd || hurd || illumos || linux || netbsd || openbsd || solaris

package runner_wrapper

import (
	"os/exec"
	"syscall"
)

func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}
