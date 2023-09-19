package constants

import (
	"sync"

	"github.com/magefile/mage/sh"
)

const (
	AppName = "gitlab-runner"
)

var versionOnce sync.Once
var version string

func Version() string {
	versionOnce.Do(func() {
		var err error
		version, err = sh.Output("sh", "-c", "./ci/version")
		if err != nil {
			panic(err)
		}
	})

	return version
}
