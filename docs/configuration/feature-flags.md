# Feature flags

> Introduced in GitLab 11.4.

Feature flags are toggles that allow you to enable or disable specific features. These flags are typically used:

- For beta features that are made available for volunteers to test, but that are not ready to be enabled for all users.

    Beta features are sometimes incomplete or need further testing. A user who wants to use a beta feature
    can choose to accept the risk and explicitly enable the feature with a feature flag. Other users who
    do not need the feature or who are not willing to accept the risk on their system have the
    feature disabled by default and are not impacted by possible bugs and regressions.

- For breaking changes that result in functionality deprecation or feature removal in the near future.

    As the product evolves, features are sometimes changed or removed entirely. Known bugs are often fixed,
    but in some cases, users have already found a workaround for a bug that affected them; forcing users
    to adopt the standardized bug fix might cause other problems with their customized configurations.

    In such cases, the feature flag is used to switch from the old behavior to the new one on demand. This
    allows users to adopt new versions of the product while giving them time to plan for a smooth, permanent
    transition from the old behavior to the new behavior.

Feature flags are toggled using environment variables. To:

- Activate a feature flag, set the corresponding environment variable to `true` or `1`.
- Deactivate a feature flag, set the corresponding environment variable to `false` or `0`.

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
| `FF_CMD_DISABLE_DELAYED_ERROR_LEVEL_EXPANSION` | `false` | ✗ |  | Disables [EnableDelayedExpansion](https://ss64.com/nt/delayedexpansion.html) for error checking for when using [Window Batch](https://docs.gitlab.com/runner/shells/#windows-batch) shell |
| `FF_USE_LEGACY_BUILDS_DIR_FOR_DOCKER` | `false` | ✓ | 12.3 | Disables the new strategy for Docker executor to cache the content of `/builds` directory instead of `/builds/group-org` |
| `FF_USE_LEGACY_VOLUMES_MOUNTING_ORDER` | `false` | ✓ | 12.6 | Disables the new ordering of volumes mounting when `docker*` executors are being used. |

<!-- feature_flags_list_end -->
