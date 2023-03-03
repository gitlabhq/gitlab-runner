package main

import (
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	cli_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/cli"
	"gitlab.com/gitlab-org/gitlab-runner/log"
	"gitlab.com/gitlab-org/labkit/fips"

	_ "gitlab.com/gitlab-org/gitlab-runner/cache/azure"
	_ "gitlab.com/gitlab-org/gitlab-runner/cache/gcs"
	_ "gitlab.com/gitlab-org/gitlab-runner/cache/s3"
	_ "gitlab.com/gitlab-org/gitlab-runner/commands"
	_ "gitlab.com/gitlab-org/gitlab-runner/commands/helpers"
	_ "gitlab.com/gitlab-org/gitlab-runner/executors/custom"
	_ "gitlab.com/gitlab-org/gitlab-runner/executors/docker"
	_ "gitlab.com/gitlab-org/gitlab-runner/executors/docker/machine"
	_ "gitlab.com/gitlab-org/gitlab-runner/executors/instance"
	_ "gitlab.com/gitlab-org/gitlab-runner/executors/kubernetes"
	_ "gitlab.com/gitlab-org/gitlab-runner/executors/parallels"
	_ "gitlab.com/gitlab-org/gitlab-runner/executors/shell"
	_ "gitlab.com/gitlab-org/gitlab-runner/executors/ssh"
	_ "gitlab.com/gitlab-org/gitlab-runner/executors/virtualbox"
	_ "gitlab.com/gitlab-org/gitlab-runner/helpers/secrets/resolvers/vault"
	_ "gitlab.com/gitlab-org/gitlab-runner/shells"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			// log panics forces exit
			if _, ok := r.(*logrus.Entry); ok {
				os.Exit(1)
			}
			panic(r)
		}
	}()

	fips.Check()

	app := cli.NewApp()
	app.Name = filepath.Base(os.Args[0])
	app.Usage = "a GitLab Runner"
	app.Version = common.AppVersion.ShortLine()
	cli.VersionPrinter = common.AppVersion.Printer
	app.Authors = []cli.Author{
		{
			Name:  "GitLab Inc.",
			Email: "support@gitlab.com",
		},
	}
	app.Commands = common.GetCommands()
	app.CommandNotFound = func(context *cli.Context, command string) {
		logrus.Fatalln("Command", command, "not found.")
	}

	cli_helpers.InitCli()
	cli_helpers.LogRuntimePlatform(app)
	cli_helpers.SetupCPUProfile(app)
	cli_helpers.FixHOME(app)
	cli_helpers.WarnOnBool(os.Args)

	log.ConfigureLogging(app)

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}
