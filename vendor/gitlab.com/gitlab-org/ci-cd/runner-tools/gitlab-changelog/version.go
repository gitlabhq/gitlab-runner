package gitlab_changelog

import (
	"fmt"
	"runtime"
	"time"
)

var (
	NAME     = "gitlab-changelog"
	VERSION  = "dev"
	REVISION = "HEAD"
	BRANCH   = "HEAD"
	BUILT    = "now"

	AuthorName  = "GitLab Inc."
	AuthorEmail = "support@gitlab.com"
)

type VersionInfo struct {
	Name         string
	Version      string
	Revision     string
	Branch       string
	GOVersion    string
	BuiltAt      string
	OS           string
	Architecture string
}

func (v *VersionInfo) SimpleLine() string {
	return fmt.Sprintf("%s (%s)", v.Version, v.Revision)
}

func (v *VersionInfo) Extended() string {
	version := fmt.Sprintln(v.Name)
	version += fmt.Sprintf("Version:      %s\n", v.Version)
	version += fmt.Sprintf("Git revision: %s\n", v.Revision)
	version += fmt.Sprintf("Git branch:   %s\n", v.Branch)
	version += fmt.Sprintf("GO version:   %s\n", v.GOVersion)
	version += fmt.Sprintf("Built:        %s\n", v.BuiltAt)
	version += fmt.Sprintf("OS/Arch:      %s/%s\n", v.OS, v.Architecture)

	return version
}

var version *VersionInfo

func Version() *VersionInfo {
	if version != nil {
		return version
	}

	built := BUILT
	if built == "now" {
		built = time.Now().UTC().Format(time.RFC3339)
	}

	version = &VersionInfo{
		Name:         NAME,
		Version:      VERSION,
		Revision:     REVISION,
		Branch:       BRANCH,
		GOVersion:    runtime.Version(),
		BuiltAt:      built,
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
	}

	return version
}
