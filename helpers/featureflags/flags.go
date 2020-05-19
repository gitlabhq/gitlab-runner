package featureflags

import (
	"strconv"
)

const (
	CmdDisableDelayedErrorLevelExpansion string = "FF_CMD_DISABLE_DELAYED_ERROR_LEVEL_EXPANSION"
	NetworkPerBuild                      string = "FF_NETWORK_PER_BUILD"
	UseLegacyKubernetesExecutionStrategy string = "FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY"
	UseDirectDownload                    string = "FF_USE_DIRECT_DOWNLOAD"
	SkipNoOpBuildStages                  string = "FF_SKIP_NOOP_BUILD_STAGES"
)

type FeatureFlag struct {
	Name            string
	DefaultValue    string
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
		DefaultValue:    "false",
		Deprecated:      false,
		ToBeRemovedWith: "",
		Description:     "Disables [EnableDelayedExpansion](https://ss64.com/nt/delayedexpansion.html) for error checking for when using [Window Batch](../shells/index.md#windows-batch) shell",
	},
	{
		Name:            NetworkPerBuild,
		DefaultValue:    "false",
		Deprecated:      false,
		ToBeRemovedWith: "",
		Description:     "Enables creation of a docker [network per build](../executors/docker.md#networking) with the docker executor",
	},
	{
		Name:            UseLegacyKubernetesExecutionStrategy,
		DefaultValue:    "true",
		Deprecated:      false,
		ToBeRemovedWith: "",
		Description:     "When set to `false` disables execution of remote Kubernetes commands through `exec` in favor of `attach` to solve problems like [#4119](https://gitlab.com/gitlab-org/gitlab-runner/issues/4119)",
	},
	{
		Name:            UseDirectDownload,
		DefaultValue:    "false",
		Deprecated:      false,
		ToBeRemovedWith: "",
		Description:     "When set to `true` Runner tries to direct-download all artifacts instead of proxying through GitLab. Enabling might result in a download failures due to problem validating TLS certificate of Object Storage if it is enabled by GitLab",
	},
	{
		Name:            SkipNoOpBuildStages,
		DefaultValue:    "true",
		Deprecated:      false,
		ToBeRemovedWith: "",
		Description:     "When set to `false` all build stages are executed even if running them has no effect",
	},
}

func GetAll() []FeatureFlag {
	return flags
}

func IsOn(value string) (bool, error) {
	if value == "" {
		return false, nil
	}

	on, err := strconv.ParseBool(value)
	if err != nil {
		return false, err
	}

	return on, nil
}
