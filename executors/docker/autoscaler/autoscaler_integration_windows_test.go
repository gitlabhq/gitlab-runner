//go:build integration && windows

package autoscaler_test

import (
	"fmt"

	syswindows "golang.org/x/sys/windows"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/container/windows"
)

func getImage() string {
	v := syswindows.RtlGetVersion()
	windowsVersion := fmt.Sprintf("%v.%v.%v", v.MajorVersion, v.MinorVersion, v.BuildNumber)
	windowsVersion, _ = windows.Version(windowsVersion)

	return fmt.Sprintf(common.TestWindowsImage, "ltsc"+windowsVersion)
}
