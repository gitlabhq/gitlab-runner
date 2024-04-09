package main

import (
	"os"
	"path/filepath"

	"github.com/KimMachineGun/automemlimit/memlimit"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"go.uber.org/automaxprocs/maxprocs"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/log"

	_ "gitlab.com/gitlab-org/gitlab-runner/commands/helpers"
)

func init() {
	_, _ = maxprocs.Set()
	memlimit.SetGoMemLimitWithEnv()
}

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

	app := cli.NewApp()
	app.Name = filepath.Base(os.Args[0])
	app.Usage = "a GitLab Runner Helper"
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
		logrus.Fatalln("Command", command, "not found")
	}

	log.ConfigureLogging(app)

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}
