package featureflags

import (
	"strconv"

	"github.com/sirupsen/logrus"
)

const (
	CmdDisableDelayedErrorLevelExpansion string = "FF_CMD_DISABLE_DELAYED_ERROR_LEVEL_EXPANSION"
	NetworkPerBuild                      string = "FF_NETWORK_PER_BUILD"
	UseLegacyKubernetesExecutionStrategy string = "FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY"
	UseDirectDownload                    string = "FF_USE_DIRECT_DOWNLOAD"
	SkipNoOpBuildStages                  string = "FF_SKIP_NOOP_BUILD_STAGES"
	UseFastzip                           string = "FF_USE_FASTZIP"
	DisableUmaskForDockerExecutor        string = "FF_DISABLE_UMASK_FOR_DOCKER_EXECUTOR"
	EnableBashExitCodeCheck              string = "FF_ENABLE_BASH_EXIT_CODE_CHECK"
	UseWindowsLegacyProcessStrategy      string = "FF_USE_WINDOWS_LEGACY_PROCESS_STRATEGY"
	UseNewEvalStrategy                   string = "FF_USE_NEW_BASH_EVAL_STRATEGY"
	UsePowershellPathResolver            string = "FF_USE_POWERSHELL_PATH_RESOLVER"
	UseDynamicTraceForceSendInterval     string = "FF_USE_DYNAMIC_TRACE_FORCE_SEND_INTERVAL"
	ScriptSections                       string = "FF_SCRIPT_SECTIONS"
	UseNewShellEscape                    string = "FF_USE_NEW_SHELL_ESCAPE"
	EnableJobCleanup                     string = "FF_ENABLE_JOB_CLEANUP"
	KubernetesHonorEntrypoint            string = "FF_KUBERNETES_HONOR_ENTRYPOINT"
	PosixlyCorrectEscapes                string = "FF_POSIXLY_CORRECT_ESCAPES"
	UseImprovedURLMasking                string = "FF_USE_IMPROVED_URL_MASKING"
	ResolveFullTLSChain                  string = "FF_RESOLVE_FULL_TLS_CHAIN"
	DisablePowershellStdin               string = "FF_DISABLE_POWERSHELL_STDIN"
	UsePodActiveDeadlineSeconds          string = "FF_USE_POD_ACTIVE_DEADLINE_SECONDS"
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
		//nolint:lll
		Description: "Enables creation of a Docker [network per build](../executors/docker.md#network-configurations) with " +
			"the `docker` executor",
	},
	{
		Name:            UseLegacyKubernetesExecutionStrategy,
		DefaultValue:    false,
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
		Name:            UseFastzip,
		DefaultValue:    false,
		Deprecated:      false,
		ToBeRemovedWith: "",
		Description:     "Fastzip is a performant archiver for cache/artifact archiving and extraction",
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
			"process setup is used. To successfully and gracefully drain a Windows Runner, this feature flag should " +
			"be set to `false`.",
	},
	{
		Name:            UseNewEvalStrategy,
		DefaultValue:    false,
		Deprecated:      false,
		ToBeRemovedWith: "",
		Description: "When set to `true`, the Bash `eval` call is executed in a subshell to help with proper exit " +
			"code detection of the script executed.",
	},
	{
		Name:            UsePowershellPathResolver,
		DefaultValue:    false,
		Deprecated:      false,
		ToBeRemovedWith: "",
		Description: "When enabled, PowerShell resolves pathnames rather than Runner using OS-specific filepath " +
			"functions that are specific to where Runner is hosted.",
	},
	{
		Name:            UseDynamicTraceForceSendInterval,
		DefaultValue:    false,
		Deprecated:      false,
		ToBeRemovedWith: "",
		Description: "When enabled, the trace force send interval is dynamically adjusted based on the trace " +
			"update interval.",
	},
	{
		Name:            ScriptSections,
		DefaultValue:    false,
		Deprecated:      false,
		ToBeRemovedWith: "",
		Description: "When enabled, each script line from the `.gitlab-ci.yml` file will be in a collapsible " +
			"section in the job output and show the duration of each line.",
	},
	{
		Name:            UseNewShellEscape,
		DefaultValue:    false,
		Deprecated:      false,
		ToBeRemovedWith: "",
		Description:     "When enabled, a faster implementation of shell escape is used.",
	},
	{
		Name:            EnableJobCleanup,
		DefaultValue:    false,
		Deprecated:      false,
		ToBeRemovedWith: "",
		Description: "When enabled, the project directory will be cleaned up at the end of the build. " +
			"If `GIT_CLONE` is used, the whole project directory will be deleted. If `GIT_FETCH` is used, " +
			"a series of Git `clean` commands will be issued.",
	},
	{
		Name:            KubernetesHonorEntrypoint,
		DefaultValue:    false,
		Deprecated:      false,
		ToBeRemovedWith: "",
		Description: "When enabled, the Docker entrypoint of an image will be honored if " +
			"`FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY` is not set to true",
	},
	{
		Name:            PosixlyCorrectEscapes,
		DefaultValue:    false,
		Deprecated:      false,
		ToBeRemovedWith: "",
		//nolint:lll
		Description: "When enabled, [POSIX shell escapes](https://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html#tag_18_02) " +
			"are used rather than [`bash`-style ANSI-C quoting](https://www.gnu.org/software/bash/manual/html_node/Quoting.html). " +
			"This should be enabled if the job environment uses a POSIX-compliant shell.",
	},
	{
		Name:         UseImprovedURLMasking,
		DefaultValue: false,
		Deprecated:   false,
		Description: "When enabled, any sensitive URL parameters are masked no matter where they appear in the trace " +
			"log output. When this is disabled, sensitive URL parameters are only masked in select places and can " +
			"[occasionally be revealed](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4625). This feature " +
			"flag can only be configured via Runner's config and not from a job.",
	},
	{
		Name:         ResolveFullTLSChain,
		DefaultValue: true,
		Deprecated:   false,
		Description: "When enabled, the runner resolves a full TLS " +
			"chain all the way down to a self-signed root certificate " +
			"for `CI_SERVER_TLS_CA_FILE`. This was previously " +
			"[required to make Git HTTPS clones to work](tls-self-signed.md#git-cloning) " +
			"for a Git client built with libcurl prior to v7.68.0 and OpenSSL. " +
			"However, the process to resolve certificates may fail on " +
			"some operating systems, such as macOS, that reject root certificates " +
			"signed with older signature algorithms. " +
			"If certificate resolution fails, you may need to disable this feature. " +
			"This feature flag can only be disabled in the " +
			"[`[runners.feature_flags]` configuration](#enable-feature-flag-in-runner-configuration).",
	},
	{
		Name:         DisablePowershellStdin,
		DefaultValue: false,
		Deprecated:   false,
		Description: "When enabled, PowerShell scripts for shell and custom executors are passed by " +
			"file, rather than passed and executed via stdin.",
	},
	//nolint:lll
	{
		Name:            UsePodActiveDeadlineSeconds,
		DefaultValue:    false,
		Deprecated:      false,
		ToBeRemovedWith: "",
		Description: "When enabled, the [pod `activeDeadlineSeconds`]" +
			"(https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#lifecycle)" +
			" is set to the CI/CD job timeout. This flag affects the " +
			"[pod's lifecycle](../executors/kubernetes.md#pod-lifecycle).",
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
