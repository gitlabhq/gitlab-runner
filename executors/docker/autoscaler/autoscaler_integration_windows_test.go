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
	res, _ := windows.Resolve(windowsVersion)
	windowsVersion = res.Version

	return fmt.Sprintf(common.TestWindowsImage, "ltsc"+windowsVersion)
}
