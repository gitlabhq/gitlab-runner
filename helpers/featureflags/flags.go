package featureflags

import (
	"strconv"

	"github.com/sirupsen/logrus"
)

const (
	CmdDisableDelayedErrorLevelExpansion        string = "FF_CMD_DISABLE_DELAYED_ERROR_LEVEL_EXPANSION"
	NetworkPerBuild                             string = "FF_NETWORK_PER_BUILD"
	UseLegacyKubernetesExecutionStrategy        string = "FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY"
	UseDirectDownload                           string = "FF_USE_DIRECT_DOWNLOAD"
	SkipNoOpBuildStages                         string = "FF_SKIP_NOOP_BUILD_STAGES"
	ShellExecutorUseLegacyProcessKill           string = "FF_SHELL_EXECUTOR_USE_LEGACY_PROCESS_KILL"
	ResetHelperImageEntrypoint                  string = "FF_RESET_HELPER_IMAGE_ENTRYPOINT"
	UseGoCloudWithCacheArchiver                 string = "FF_USE_GO_CLOUD_WITH_CACHE_ARCHIVER"
	UseFastzip                                  string = "FF_USE_FASTZIP"
	GitLabRegistryHelperImage                   string = "FF_GITLAB_REGISTRY_HELPER_IMAGE"
	DisableUmaskForDockerExecutor               string = "FF_DISABLE_UMASK_FOR_DOCKER_EXECUTOR"
	EnableBashExitCodeCheck                     string = "FF_ENABLE_BASH_EXIT_CODE_CHECK"
	UseWindowsLegacyProcessStrategy             string = "FF_USE_WINDOWS_LEGACY_PROCESS_STRATEGY"
	SkipDockerMachineProvisionOnCreationFailure string = "FF_SKIP_DOCKER_MACHINE_PROVISION_ON_CREATION_FAILURE"
)

type FeatureFlag struct {
	Name            string
	DefaultValue    bool
	Deprecated      bool
	ToBeRemovedWith string
	Description     string
}

// REMEMBER to update the documentation after adding or removing a feature flag
//
// Please use `make update_feature_flags_docs` to make the update automatic and
// properly formatted. It will replace the existing table with the new one, computed
// basing on the values below
var flags = []FeatureFlag{
	{
		Name:            CmdDisableDelayedErrorLevelExpansion,
		DefaultValue:    false,
		Deprecated:      false,
		ToBeRemovedWith: "",
		Description: "Disables [EnableDelayedExpansion](https://ss64.com/nt/delayedexpansion.html) for " +
			"error checking for when using [Window Batch](../shells/index.md#windows-batch) shell",
	},
	{
		Name:            NetworkPerBuild,
		DefaultValue:    false,
		Deprecated:      false,
		ToBeRemovedWith: "",
		Description: "Enables creation of a Docker [network per build](../executors/docker.md#networking) with " +
			"the `docker` executor",
	},
	{
		Name:            UseLegacyKubernetesExecutionStrategy,
		DefaultValue:    true,
		Deprecated:      false,
		ToBeRemovedWith: "",
		Description: "When set to `false` disables execution of remote Kubernetes commands through `exec` in " +
			"favor of `attach` to solve problems like " +
			"[#4119](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4119)",
	},
	{
		Name:            UseDirectDownload,
		DefaultValue:    true,
		Deprecated:      false,
		ToBeRemovedWith: "",
		Description: "When set to `true` Runner tries to direct-download all artifacts instead of proxying " +
			"through GitLab on a first try. Enabling might result in a download failures due to problem validating " +
			"TLS certificate of Object Storage if it is enabled by GitLab. " +
			"See [Self-signed certificates or custom Certification Authorities](tls-self-signed.md)",
	},
	{
		Name:            SkipNoOpBuildStages,
		DefaultValue:    true,
		Deprecated:      false,
		ToBeRemovedWith: "",
		Description:     "When set to `false` all build stages are executed even if running them has no effect",
	},
	{
		Name:            ShellExecutorUseLegacyProcessKill,
		DefaultValue:    false,
		Deprecated:      true,
		ToBeRemovedWith: "14.0",
		Description: "Use the old process termination that was used prior to GitLab 13.1 where only `SIGKILL`" +
			" was sent",
	},
	{
		Name:            ResetHelperImageEntrypoint,
		DefaultValue:    true,
		Deprecated:      true,
		ToBeRemovedWith: "14.0",
		Description: "Enables adding an ENTRYPOINT layer for Helper images imported from local Docker archives " +
			"by the `docker` executor, in order to enable [importing of user certificate roots]" +
			"(tls-self-signed.md#trusting-the-certificate-for-the-other-cicd-stages)",
	},
	{
		Name:            UseGoCloudWithCacheArchiver,
		DefaultValue:    true,
		Deprecated:      true,
		ToBeRemovedWith: "14.0",
		Description: "Enables the use of Go Cloud to write cache archives to object storage. " +
			"This mode is only used by Azure Blob storage.",
	},
	{
		Name:            UseFastzip,
		DefaultValue:    false,
		Deprecated:      false,
		ToBeRemovedWith: "",
		Description:     "Fastzip is a performant archiver for cache/artifact archiving and extraction",
	},
	{
		Name:            GitLabRegistryHelperImage,
		DefaultValue:    false,
		Deprecated:      false,
		ToBeRemovedWith: "",
		Description: "Use GitLab Runner helper image for the Docker and " +
			"Kubernetes executors from `registry.gitlab.com` instead of Docker Hub",
	},
	{
		Name:            DisableUmaskForDockerExecutor,
		DefaultValue:    false,
		Deprecated:      false,
		ToBeRemovedWith: "",
		Description: "If enabled will remove the usage of `umask 0000` call for jobs executed with `docker` " +
			"executor. Instead Runner will try to discover the UID and GID of the user configured for the image used " +
			"by the build container and will change the ownership of the working directory and files by running the " +
			"`chmod` command in the predefined container (after updating sources, restoring cache and " +
			"downloading artifacts). POSIX utility `id` must be installed and operational in the build image " +
			"for this feature flag. Runner will execute `id` with options `-u` and `-g` to retrieve the UID and GID.",
	},
	{
		Name:            EnableBashExitCodeCheck,
		DefaultValue:    false,
		Deprecated:      false,
		ToBeRemovedWith: "",
		Description: "If enabled, bash scripts don't rely solely on `set -e`, but check for a non-zero exit code " +
			"after each script command is executed.",
	},
	{
		Name:            UseWindowsLegacyProcessStrategy,
		DefaultValue:    true,
		Deprecated:      false,
		ToBeRemovedWith: "",
		Description: "When disabled, processes that Runner creates on Windows (shell and custom executor) will be " +
			"created with additional setup that should improve process termination. This is currently experimental " +
			"and how we setup these processes may change as we continue to improve this. When set to `true`, legacy " +
			"process setup is used. To successfully and gracefully drain a Windows Runner, this feature flag should" +
			"be set to `false`.",
	},
	{
		Name:            SkipDockerMachineProvisionOnCreationFailure,
		DefaultValue:    false,
		Deprecated:      false,
		ToBeRemovedWith: "",
		Description: "With the `docker+machine` executor, when a machine is " +
			"not created, `docker-machine provision` runs for X amount of times. When " +
			"this feature flag is set to `true`, it skips `docker-machine provision` " +
			"removes the machine, and creates another machine instead.",
	},
}

func GetAll() []FeatureFlag {
	return flags
}

func IsOn(logger logrus.FieldLogger, value string) bool {
	if value == "" {
		return false
	}

	on, err := strconv.ParseBool(value)
	if err != nil {
		logger.WithError(err).
			WithField("value", value).
			Error("Error while parsing the value of feature flag")

		return false
	}

	return on
}
