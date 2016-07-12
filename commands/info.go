package commands

import (
	"encoding/json"
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"
	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/helpers"
)

type InfoData struct {
	BuildsCount int `json:"builds_count"`

	RunnersBuildsCounts map[string]int        `json:"runners_builds_counts"`
	VersionInfo         common.AppVersionInfo `json:"version_info"`
}

type InfoCommand struct {
	buildsHelper buildsHelper
	data         InfoData
}

func (c *InfoCommand) prepare() {
	c.data.BuildsCount = c.buildsHelper.buildsCount()
	c.data.VersionInfo = common.AppVersion

	runnersBuildsCounts := map[string]int{}
	for token, count := range c.buildsHelper.counts {
		runnersBuildsCounts[helpers.ShortenToken(token)] = count
	}
	c.data.RunnersBuildsCounts = runnersBuildsCounts
}

func (c *InfoCommand) Execute(context *cli.Context) {
	c.prepare()

	bytes, err := json.Marshal(c.data)
	if err != nil {
		log.WithError(err).Errorln("Error with InfoData marshalling to JSON")
		return
	}

	fmt.Print(string(bytes))
}

func init() {
	common.RegisterCommand2("info", "show statistic and debuging data", &InfoCommand{})
}
