package common

import (
	"fmt"
	"runtime"
	"time"

	"github.com/codegangsta/cli"
)

var NAME = "gitlab-ci-multi-runner"
var VERSION = "dev"
var REVISION = "HEAD"
var BRANCH = "HEAD"
var BUILT = "now"

type AppVersionInfo struct {
	Name         string    `json:"name"`
	Version      string    `json:"version"`
	Revision     string    `json:"revision"`
	Branch       string    `json:"branch"`
	GOVersion    string    `json:"go_version"`
	BuiltAt      time.Time `json:"built_at"`
	OS           string    `json:"os"`
	Architecture string    `json:"architecture"`
}

var AppVersion AppVersionInfo

func VersionPrinter(c *cli.Context) {
	fmt.Print(ExtendedVersion())
}

func VersionLine() string {
	return fmt.Sprintf("%s %s (%s)", AppVersion.Name, AppVersion.Version, AppVersion.Revision)
}

func VersionShortLine() string {
	return fmt.Sprintf("%s (%s)", AppVersion.Version, AppVersion.Revision)
}

func VersionUserAgent() string {
	return fmt.Sprintf("%s %s (%s; %s; %s/%s)", AppVersion.Name, AppVersion.Version, AppVersion.Branch,
		AppVersion.GOVersion, AppVersion.OS, AppVersion.Architecture)
}

func ExtendedVersion() string {
	version := fmt.Sprintf("Version:      %s\n", AppVersion.Version)
	version += fmt.Sprintf("Git revision: %s\n", AppVersion.Revision)
	version += fmt.Sprintf("Git branch:   %s\n", AppVersion.Branch)
	version += fmt.Sprintf("GO version:   %s\n", AppVersion.GOVersion)
	version += fmt.Sprintf("Built:        %s\n", AppVersion.BuiltAt.Format(time.RFC1123Z))
	version += fmt.Sprintf("OS/Arch:      %s/%s\n", AppVersion.OS, AppVersion.Architecture)

	return version
}

func init() {
	builtAt := time.Now()
	if BUILT != "now" {
		builtAt, _ = time.Parse(time.RFC3339, BUILT)
	}

	AppVersion = AppVersionInfo{
		Name:         NAME,
		Version:      VERSION,
		Revision:     REVISION,
		Branch:       BRANCH,
		GOVersion:    runtime.Version(),
		BuiltAt:      builtAt,
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
	}
}
