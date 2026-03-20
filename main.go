package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/KimMachineGun/automemlimit/memlimit"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/commands"
	"gitlab.com/gitlab-org/gitlab-runner/commands/fleeting"
	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/commands/steps"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/executors/custom"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/autoscaler"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/machine"
	"gitlab.com/gitlab-org/gitlab-runner/executors/instance"
	"gitlab.com/gitlab-org/gitlab-runner/executors/kubernetes"
	"gitlab.com/gitlab-org/gitlab-runner/executors/parallels"
	"gitlab.com/gitlab-org/gitlab-runner/executors/shell"
	"gitlab.com/gitlab-org/gitlab-runner/executors/ssh"
	"gitlab.com/gitlab-org/gitlab-runner/executors/virtualbox"
	cli_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/cli"
	"gitlab.com/gitlab-org/gitlab-runner/log"
	"gitlab.com/gitlab-org/gitlab-runner/network"
	"gitlab.com/gitlab-org/gitlab-runner/router"
	"gitlab.com/gitlab-org/labkit/fips"

	_ "gitlab.com/gitlab-org/gitlab-runner/cache/azure"
	_ "gitlab.com/gitlab-org/gitlab-runner/cache/gcs"
	_ "gitlab.com/gitlab-org/gitlab-runner/cache/gcsv2"
	_ "gitlab.com/gitlab-org/gitlab-runner/cache/s3"
	_ "gitlab.com/gitlab-org/gitlab-runner/cache/s3v2"
	_ "gitlab.com/gitlab-org/gitlab-runner/helpers/secrets/resolvers/aws"
	_ "gitlab.com/gitlab-org/gitlab-runner/helpers/secrets/resolvers/azure_key_vault"
	_ "gitlab.com/gitlab-org/gitlab-runner/helpers/secrets/resolvers/gcp_secret_manager"
	_ "gitlab.com/gitlab-org/gitlab-runner/helpers/secrets/resolvers/gitlab_secrets_manager"
	_ "gitlab.com/gitlab-org/gitlab-runner/helpers/secrets/resolvers/vault"
	_ "gitlab.com/gitlab-org/gitlab-runner/shells"
)

func init() {
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

	executorProviders := newExecutorProviders()
	for name, provider := range executorProviders.All() {
		err := common.ValidateExecutorProvider(provider)
		if err != nil {
			panic(fmt.Sprintf("Executor %s cannot be registered: %v", name, err))
		}
	}

	fips.Check()
	gitLabClient, clientShutdown, apiRequestsCollector := newClient(executorProviders)
	defer clientShutdown()

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
	app.Commands = newCommands(gitLabClient, apiRequestsCollector, executorProviders)
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

func newCommands(n common.Network, apiRequestsCollector *network.APIRequestsCollector, executorProviders executors.Providers) []cli.Command {
	cmds := []cli.Command{
		commands.NewListCommand(),
		commands.NewRegisterCommand(n, executorProviders),
		commands.NewResetTokenCommand(n),
		commands.NewRunCommand(n, apiRequestsCollector, executorProviders),
		commands.NewRunSingleCommand(n, executorProviders),
		commands.NewRunnerWrapperCommand(),
		commands.NewUnregisterCommand(n),
		commands.NewVerifyCommand(n),
		fleeting.NewCommand(),
		helpers.NewArtifactsDownloaderCommand(),
		helpers.NewArtifactsUploaderCommand(),
		helpers.NewCacheArchiverCommand(),
		helpers.NewCacheExtractorCommand(),
		helpers.NewCacheInitCommand(),
		helpers.NewHealthCheckCommand(),
		helpers.NewProxyExecCommand(),
		helpers.NewReadLogsCommand(),
		steps.NewCommand(),
	}
	cmds = append(cmds, commands.NewServiceCommands()...)
	return cmds
}

func newClient(executorProviders executors.Providers) (common.Network, func(), *network.APIRequestsCollector) {
	apiRequestsCollector := network.NewAPIRequestsCollector()
	certDir := commands.GetDefaultCertificateDirectory()

	mainClient := network.NewGitLabClient(
		network.WithAPIRequestsCollector(apiRequestsCollector),
		network.WithCertificateDirectory(certDir),
		network.WithExecutorProviderFunc(executorProviders.GetByName),
	)
	rc := router.NewClient(
		mainClient,
		certDir,
		common.AppVersion.UserAgent(),
	)
	return rc, rc.Shutdown, apiRequestsCollector
}

func newExecutorProviders() *executors.ProviderRegistry {
	runnerCommand, err := os.Executable()
	if err != nil {
		panic(err)
	}
	dockerProvider := docker.NewProvider()

	return executors.NewProviderRegistry(map[string]common.ExecutorProvider{
		"shell":                   shell.NewProvider(runnerCommand),
		"custom":                  custom.NewProvider("gitlab-runner"),
		"instance":                instance.NewProvider("gitlab-runner"),
		"docker":                  dockerProvider,
		"docker-windows":          docker.NewWindowsProvider(),
		"docker-autoscaler":       autoscaler.NewProvider(dockerProvider),
		"docker+machine":          machine.NewProvider(dockerProvider),
		common.ExecutorKubernetes: kubernetes.NewProvider(),
		"ssh":                     ssh.NewProvider(),
		"parallels":               parallels.NewProvider(),
		"virtualbox":              virtualbox.NewProvider(),
	})
}
