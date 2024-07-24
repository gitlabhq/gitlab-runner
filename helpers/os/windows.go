//go:build windows

package os

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func LocalKernelVersion() string {
	major, minor, build := windows.RtlGetNtVersionNumbers()
	return fmt.Sprintf("%d.%d.%d", major, minor, build)
}
