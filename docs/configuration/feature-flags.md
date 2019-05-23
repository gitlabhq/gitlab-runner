# Feature flags

Starting with GitLab Runner 11.4 we added a base support for feature flags in GitLab Runner.

These flags may be used:

1. For beta features that we want to make available for volunteers but don't want to enable publicly yet.

    A user who wants to use such feature and accepts the risk, can enable it explicitly while the wide
    group of users will be unaffected by potential bugs and regressions.

1. For breaking changes that need a deprecation and removal after few releases.

    While the product evolves some features may be removed or changed. Sometimes it may be even something
    that is generally considered as a bug, but users already managed to find some workarounds for it
    and a fix could affect their configurations.

    In that cases the feature flag is used to switch from old behavior to the new wan on demand. Such
    fix such ensure that the old behavior is deprecated and marked for removal together with the feature
    flag that protects the new behavior.

At this moment feature flags mechanism is based on environment variables. To make the change hidden behind
the feature flag active a corresponding environment variable should be set to `true` or `1`. To make the
change hidden behind the feature flag disabled a corresponding environment variable should be set to
`false` or `0`.

## Available feature flags

<!--
The list of feature flags is created automatically.
If you need to update it, call `make update_feature_flags_docs` in the
root directory of this project.
The flags are defined in `./helpers/feature_flags/flags.go` file.
-->

<!-- feature_flags_list_start -->

| Feature flag | Default value | Deprecated | To be removed with | Description |
|--------------|---------------|------------|--------------------|-------------|
| `FF_K8S_USE_ENTRYPOINT_OVER_COMMAND` | `true` | ✓ | 12.0 | Enables [the fix](https://gitlab.com/gitlab-org/gitlab-runner/merge_requests/1010) for entrypoint configuration when `kubernetes` executor is used |
| `FF_DOCKER_HELPER_IMAGE_V2` | `false` | ✓ | 12.0 | Enable the helper image to use the new commands when [helper_image](https://docs.gitlab.com/runner/configuration/advanced-configuration.html#the-runnersdocker-section) is specified. This will start using the new API that will be used in 12.0 and stop showing the warning message in the build log |
| `FF_CMD_DISABLE_DELAYED_ERROR_LEVEL_EXPANSION` | `false` | ✗ |  | Disables [EnableDelayedExpansion](https://ss64.com/nt/delayedexpansion.html) for error checking for when using [Window Batch](https://docs.gitlab.com/runner/shells/#windows-batch) shell |
| `FF_USE_LEGACY_BUILDS_DIR_FOR_DOCKER` | `false` | ✓ | 12.3 | Disables the new strategy for Docker executor to cache the content of `/builds` directory instead of `/builds/group-org` |
| `FF_USE_LEGACY_VOLUMES_MOUNTING_ORDER` | `false` | ✓ | 12.6 | Disables the new ordering of volumes mounting when `docker*` executors are being used. |

<!-- feature_flags_list_end -->
