package featureflags

import (
	"strconv"
)

const (
	DockerHelperImageV2                  string = "FF_DOCKER_HELPER_IMAGE_V2"
	CmdDisableDelayedErrorLevelExpansion string = "FF_CMD_DISABLE_DELAYED_ERROR_LEVEL_EXPANSION"
	UseLegacyBuildsDirForDocker          string = "FF_USE_LEGACY_BUILDS_DIR_FOR_DOCKER"
	UseLegacyVolumesMountingOrder        string = "FF_USE_LEGACY_VOLUMES_MOUNTING_ORDER"
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
		Name:            DockerHelperImageV2,
		DefaultValue:    "false",
		Deprecated:      true,
		ToBeRemovedWith: "12.0",
		Description:     "Enable the helper image to use the new commands when [helper_image](https://docs.gitlab.com/runner/configuration/advanced-configuration.html#the-runnersdocker-section) is specified. This will start using the new API that will be used in 12.0 and stop showing the warning message in the build log",
	},
	{
		Name:            CmdDisableDelayedErrorLevelExpansion,
		DefaultValue:    "false",
		Deprecated:      false,
		ToBeRemovedWith: "",
		Description:     "Disables [EnableDelayedExpansion](https://ss64.com/nt/delayedexpansion.html) for error checking for when using [Window Batch](https://docs.gitlab.com/runner/shells/#windows-batch) shell",
	},
	{
		Name:            UseLegacyBuildsDirForDocker,
		DefaultValue:    "false",
		Deprecated:      true,
		ToBeRemovedWith: "12.3",
		Description:     "Disables the new strategy for Docker executor to cache the content of `/builds` directory instead of `/builds/group-org`",
	},
	{
		Name:            UseLegacyVolumesMountingOrder,
		DefaultValue:    "false",
		Deprecated:      true,
		ToBeRemovedWith: "12.6",
		Description:     "Disables the new ordering of volumes mounting when `docker*` executors are being used.",
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
