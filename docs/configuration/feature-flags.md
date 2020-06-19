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

- Activate a feature flag, set the corresponding environment variable to `"true"` or `1`.
- Deactivate a feature flag, set the corresponding environment variable to `"false"` or `0`.

## Available feature flags

<!--
The list of feature flags is created automatically.
If you need to update it, call `make update_feature_flags_docs` in the
root directory of this project.
The flags are defined in `./helpers/featureflags/flags.go` file.
-->

<!-- feature_flags_list_start -->

| Feature flag | Default value | Deprecated | To be removed with | Description |
|--------------|---------------|------------|--------------------|-------------|
| `FF_CMD_DISABLE_DELAYED_ERROR_LEVEL_EXPANSION` | `false` | ✗ |  | Disables [EnableDelayedExpansion](https://ss64.com/nt/delayedexpansion.html) for error checking for when using [Window Batch](../shells/index.md#windows-batch) shell |
| `FF_NETWORK_PER_BUILD` | `false` | ✗ |  | Enables creation of a Docker [network per build](../executors/docker.md#networking) with the `docker` executor |
| `FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY` | `true` | ✗ |  | When set to `false` disables execution of remote Kubernetes commands through `exec` in favor of `attach` to solve problems like [#4119](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4119) |
| `FF_USE_DIRECT_DOWNLOAD` | `true` | ✗ |  | When set to `true` Runner tries to direct-download all artifacts instead of proxying through GitLab on a first try. Enabling might result in a download failures due to problem validating TLS certificate of Object Storage if it is enabled by GitLab |
| `FF_SKIP_NOOP_BUILD_STAGES` | `true` | ✗ |  | When set to `false` all build stages are executed even if running them has no effect |
| `FF_SHELL_EXECUTOR_USE_LEGACY_PROCESS_KILL` | `false` | ✓ | 14.0 | Use the old process termination that was used prior to GitLab 13.1 where only `SIGKILL` was sent |

<!-- feature_flags_list_end -->

## Enable feature flag in pipeline configuration

You can use [CI variables](https://docs.gitlab.com/ee/ci/variables/) to
enable feature flags:

- For all jobs in the pipeline (globally):

  ```yaml
  variables:
    FEATURE_FLAG_NAME: 1
  ```

- For a single job:

  ```yaml
  job:
    stage: test
    variables:
      FEATURE_FLAG_NAME: 1
    script:
    - echo "Hello"
  ```

## Enable feature flag for Runner

To enable the feature for every job a Runner runs, specify the feature
flag as an
[`environment`](advanced-configuration.md#the-runners-section) variable
in the [Runner configuration](advanced-configuration.md):

```toml
[[runners]]
  name = "ruby-2.6-docker"
  url = "https://CI/"
  token = "TOKEN"
  limit = 0
  executor = "docker"
  builds_dir = ""
  shell = ""
  environment = ["FEATURE_FLAG_NAME=1"]
```
