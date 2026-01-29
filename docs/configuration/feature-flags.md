---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: GitLab Runner feature flags
---

{{< alert type="warning" >}}

Data corruption, stability degradation, performance degradation, and security issues may occur if you enable a feature that's disabled by default. Before you enable feature flags, you should be aware of the risks involved. For more information, see [Risks when enabling features still in development](https://docs.gitlab.com/administration/feature_flags/#risks-when-enabling-features-still-in-development).

{{< /alert >}}

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
| `FF_NETWORK_PER_BUILD` | `false` | {{< icon name="dotted-circle" >}} No |  | Enables creation of a Docker [network per build](../executors/docker.md#network-configurations) with the `docker` executor |
| `FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY` | `false` | {{< icon name="dotted-circle" >}} No |  | When set to `false` disables execution of remote Kubernetes commands through `exec` in favor of `attach` to solve problems like [#4119](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4119) |
| `FF_USE_DIRECT_DOWNLOAD` | `true` | {{< icon name="dotted-circle" >}} No |  | When set to `true` Runner tries to direct-download all artifacts instead of proxying through GitLab on a first try. Enabling might result in a download failures due to problem validating TLS certificate of Object Storage if it is enabled by GitLab. See [Self-signed certificates or custom Certification Authorities](tls-self-signed.md) |
| `FF_SKIP_NOOP_BUILD_STAGES` | `true` | {{< icon name="dotted-circle" >}} No |  | When set to `false` all build stages are executed even if running them has no effect |
| `FF_USE_FASTZIP` | `false` | {{< icon name="dotted-circle" >}} No |  | Fastzip is a performant archiver for cache/artifact archiving and extraction |
| `FF_DISABLE_UMASK_FOR_DOCKER_EXECUTOR` | `false` | {{< icon name="dotted-circle" >}} No |  | If enabled will remove the usage of `umask 0000` call for jobs executed with `docker` executor. Instead Runner will try to discover the UID and GID of the user configured for the image used by the build container and will change the ownership of the working directory and files by running the `chmod` command in the predefined container (after updating sources, restoring cache and downloading artifacts). POSIX utility `id` must be installed and operational in the build image for this feature flag. Runner will execute `id` with options `-u` and `-g` to retrieve the UID and GID. |
| `FF_ENABLE_BASH_EXIT_CODE_CHECK` | `false` | {{< icon name="dotted-circle" >}} No |  | If enabled, bash scripts don't rely solely on `set -e`, but check for a non-zero exit code after each script command is executed. |
| `FF_USE_WINDOWS_LEGACY_PROCESS_STRATEGY` | `false` | {{< icon name="dotted-circle" >}} No |  | In GitLab Runner 16.10 and later, the default is `false`. In GitLab Runner 16.9 and earlier, the default is `true`. When disabled, processes that Runner creates on Windows (shell and custom executor) will be created with additional setup that should improve process termination. When set to `true`, legacy process setup is used. To successfully and gracefully drain a Windows Runner, this feature flag should be set to `false`. |
| `FF_USE_NEW_BASH_EVAL_STRATEGY` | `false` | {{< icon name="dotted-circle" >}} No |  | When set to `true`, the Bash `eval` call is executed in a subshell to help with proper exit code detection of the script executed. |
| `FF_USE_POWERSHELL_PATH_RESOLVER` | `false` | {{< icon name="dotted-circle" >}} No |  | When enabled, PowerShell resolves pathnames rather than Runner using OS-specific filepath functions that are specific to where Runner is hosted. |
| `FF_USE_DYNAMIC_TRACE_FORCE_SEND_INTERVAL` | `false` | {{< icon name="dotted-circle" >}} No |  | When enabled, the trace force send interval for logs is dynamically adjusted based on the trace update interval. |
| `FF_SCRIPT_SECTIONS` | `false` | {{< icon name="dotted-circle" >}} No |  | When enabled, each script line from the `.gitlab-ci.yml` file is in a collapsible section in the job output, and shows the duration of each line. When the command spans multiple lines, the complete command is displayed within the job log output terminal. |
| `FF_ENABLE_JOB_CLEANUP` | `false` | {{< icon name="dotted-circle" >}} No |  | When enabled, the project directory will be cleaned up at the end of the build. If `GIT_CLONE` is used, the whole project directory will be deleted. If `GIT_FETCH` is used, a series of Git `clean` commands will be issued. |
| `FF_KUBERNETES_HONOR_ENTRYPOINT` | `false` | {{< icon name="dotted-circle" >}} No |  | When enabled, the Docker entrypoint of an image will be honored if `FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY` is not set to true |
| `FF_POSIXLY_CORRECT_ESCAPES` | `false` | {{< icon name="dotted-circle" >}} No |  | When enabled, [POSIX shell escapes](https://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html#tag_18_02) are used rather than [`bash`-style ANSI-C quoting](https://www.gnu.org/software/bash/manual/html_node/Quoting.html). This should be enabled if the job environment uses a POSIX-compliant shell. |
| `FF_RESOLVE_FULL_TLS_CHAIN` | `false` | {{< icon name="dotted-circle" >}} No |  | In GitLab Runner 16.4 and later, the default is `false`. In GitLab Runner 16.3 and earlier, the default is `true`. When enabled, the runner resolves a full TLS chain all the way down to a self-signed root certificate for `CI_SERVER_TLS_CA_FILE`. This was previously [required to make Git HTTPS clones work](tls-self-signed.md#git-cloning) for a Git client built with libcurl prior to v7.68.0 and OpenSSL. However, the process to resolve certificates might fail on some operating systems, such as macOS, that reject root certificates signed with older signature algorithms. If certificate resolution fails, you might need to disable this feature. This feature flag can only be disabled in the [`[runners.feature_flags]` configuration](#enable-feature-flag-in-runner-configuration). |
| `FF_DISABLE_POWERSHELL_STDIN` | `false` | {{< icon name="dotted-circle" >}} No |  | When enabled, PowerShell scripts for shell and custom executors are passed by file, rather than passed and executed via stdin. This is required for jobs' `allow_failure:exit_codes` keywords to work correctly. |
| `FF_USE_POD_ACTIVE_DEADLINE_SECONDS` | `true` | {{< icon name="dotted-circle" >}} No |  | When enabled, the [pod `activeDeadlineSeconds`](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#lifecycle) is set to the CI/CD job timeout. This flag affects the [pod's lifecycle](../executors/kubernetes/_index.md#pod-lifecycle). |
| `FF_USE_ADVANCED_POD_SPEC_CONFIGURATION` | `false` | {{< icon name="dotted-circle" >}} No |  | When enabled, the user can set an entire whole pod specification in the `config.toml` file. For more information, see [Overwrite generated pod specifications (Experiment)](../executors/kubernetes/_index.md#overwrite-generated-pod-specifications). |
| `FF_SET_PERMISSIONS_BEFORE_CLEANUP` | `true` | {{< icon name="dotted-circle" >}} No |  | When enabled, permissions on directories and files in the project directory are set first, to ensure that deletions during cleanup are successful. |
| `FF_SECRET_RESOLVING_FAILS_IF_MISSING` | `true` | {{< icon name="dotted-circle" >}} No |  | When enabled, secret resolving fails if the value cannot be found. |
| `FF_PRINT_POD_EVENTS` | `false` | {{< icon name="dotted-circle" >}} No |  | When enabled, all events associated with the build pod will be printed until it's started. |
| `FF_USE_GIT_BUNDLE_URIS` | `true` | {{< icon name="dotted-circle" >}} No |  | When enabled, the Git `transfer.bundleURI` configuration option is set to `true`. This FF is enabled by default. Set to `false` to disable Git bundle support. |
| `FF_USE_GIT_NATIVE_CLONE` | `false` | {{< icon name="dotted-circle" >}} No |  | When enabled and `GIT_STRATEGY=clone`, the `git-clone(1)` command is used instead of `git-init(1)` + `git-fetch(1)` to clone the project. This requires Git version 2.49 and later, and falls back to `init` + `fetch` if not available. |
| `FF_USE_DUMB_INIT_WITH_KUBERNETES_EXECUTOR` | `false` | {{< icon name="dotted-circle" >}} No |  | When enabled, `dumb-init` is used to execute all the scripts. This allows `dumb-init` to run as the first process in the helper and build container. |
| `FF_USE_INIT_WITH_DOCKER_EXECUTOR` | `false` | {{< icon name="dotted-circle" >}} No |  | When enabled, the Docker executor starts the service and build containers with the `--init` option, which runs `tini-init` as PID 1. |
| `FF_LOG_IMAGES_CONFIGURED_FOR_JOB` | `false` | {{< icon name="dotted-circle" >}} No |  | When enabled, the runner logs names of the image and service images defined for each received job. |
| `FF_USE_DOCKER_AUTOSCALER_DIAL_STDIO` | `true` | {{< icon name="dotted-circle" >}} No |  | When enabled (the default), `docker system stdio` is used to tunnel to the remote Docker daemon. When disabled, for SSH connections a native SSH tunnel is used, and for WinRM connections a 'fleeting-proxy' helper binary is first deployed. |
| `FF_CLEAN_UP_FAILED_CACHE_EXTRACT` | `false` | {{< icon name="dotted-circle" >}} No |  | When enabled, commands are inserted into build scripts to detect a failed cache extraction and clean up partial cache contents left behind. |
| `FF_USE_WINDOWS_JOB_OBJECT` | `false` | {{< icon name="dotted-circle" >}} No |  | When enabled, a job object is created for each process that the runner creates on Windows with the shell and custom executors. To force-kill the processes, the runner closes the job object. This should improve the termination of difficult-to-kill processes. |
| `FF_TIMESTAMPS` | `true` | {{< icon name="dotted-circle" >}} No |  | When disabled timestamps are not added to the beginning of each log trace line. |
| `FF_DISABLE_AUTOMATIC_TOKEN_ROTATION` | `false` | {{< icon name="dotted-circle" >}} No |  | When enabled, it restricts automatic token rotation and logs a warning when the token is about to expire. |
| `FF_USE_LEGACY_GCS_CACHE_ADAPTER` | `false` | {{< icon name="dotted-circle" >}} No |  | When enabled, the legacy GCS Cache adapter is used. When disabled (default), a newer GCS Cache adapter is used which uses Google Cloud Storage's SDK for authentication. This should resolve authentication problems in environments that the legacy adapter struggled with, such as workload identity configurations in GKE. |
| `FF_DISABLE_UMASK_FOR_KUBERNETES_EXECUTOR` | `false` | {{< icon name="dotted-circle" >}} No |  | When enabled, removes the `umask 0000` call for jobs executed with the Kubernetes executor. Instead, the runner tries to discover the user ID (UID) and group ID (GID) of the user the build container runs as. The runner also changes the ownership of the working directory and files by running the `chown` command in the predefined container (after updating sources, restoring cache, and downloading artifacts). |
| `FF_USE_LEGACY_S3_CACHE_ADAPTER` | `false` | {{< icon name="dotted-circle" >}} No |  | When enabled, the legacy S3 Cache adapter is used. When disabled (default), a newer S3 Cache adapter is used which uses Amazon's S3 SDK for authentication. This should resolve authentication problems in environments that the legacy adapter struggled with, such as custom STS endpoints. |
| `FF_GIT_URLS_WITHOUT_TOKENS` | `false` | {{< icon name="dotted-circle" >}} No |  | When enabled, GitLab Runner doesn't embed the job token anywhere during Git configuration or command execution. Instead, it sets up a Git credential helper that uses the environment variable to obtain the job token. This approach limits token storage and reduces the risk of token leaks. |
| `FF_WAIT_FOR_POD_TO_BE_REACHABLE` | `false` | {{< icon name="dotted-circle" >}} No |  | When enabled, the runner waits for the Pod status to be 'Running', and for the Pod to be ready with its certificates attached. |
| `FF_MASK_ALL_DEFAULT_TOKENS` | `true` | {{< icon name="dotted-circle" >}} No |  | When enabled, GitLab Runner automatically masks all default tokens patterns. |
| `FF_EXPORT_HIGH_CARDINALITY_METRICS` | `false` | {{< icon name="dotted-circle" >}} No |  | When enabled, the runner exports the metrics with high cardinality. Special care should be taken when enabling this feature flag to avoid ingesting large amounts of data. For more information, see [Fleet scaling](../fleet_scaling/_index.md). |
| `FF_USE_FLEETING_ACQUIRE_HEARTBEATS` | `false` | {{< icon name="dotted-circle" >}} No |  | When enabled, fleeting instance connectivity is checked before a job is assigned to an instance. |
| `FF_USE_EXPONENTIAL_BACKOFF_STAGE_RETRY` | `true` | {{< icon name="dotted-circle" >}} No |  | When enabled, the retries for `GET_SOURCES_ATTEMPTS`, `ARTIFACT_DOWNLOAD_ATTEMPTS`, `RESTORE_CACHE_ATTEMPTS`, and `EXECUTOR_JOB_SECTION_ATTEMPTS` use exponential backoff (5 sec - 5 min). |
| `FF_USE_ADAPTIVE_REQUEST_CONCURRENCY` | `true` | {{< icon name="dotted-circle" >}} No |  | When enabled, the `request_concurrency` setting becomes the maximum concurrency value, and the number of concurrent requests adjusts based on the rate of successful job requests. |
| `FF_USE_GITALY_CORRELATION_ID` | `true` | {{< icon name="dotted-circle" >}} No |  | When enabled, the `X-Gitaly-Correlation-ID` header is added to all Git HTTP requests. When disabled, the Git operations execute without Gitaly Correlation ID headers. |
| `FF_HASH_CACHE_KEYS` | `false` | {{< icon name="dotted-circle" >}} No |  | When GitLab Runner creates or extracts caches, it hashes the cache keys (SHA256) before using them, both for local and distributed caches (for example, S3). For more information, see [cache key handling](advanced-configuration.md#cache-key-handling). |
| `FF_ENABLE_JOB_INPUTS_INTERPOLATION` | `false` | {{< icon name="dotted-circle" >}} No |  | When enabled, job inputs are interpolated. For more information, see [&17833](https://gitlab.com/groups/gitlab-org/-/epics/17833). |
| `FF_USE_JOB_ROUTER` | `false` | {{< icon name="dotted-circle" >}} No |  | Makes GitLab Runner fetch jobs by connecting to Job Router rather than GitLab directly. |
| `FF_SCRIPT_TO_STEP_MIGRATION` | `false` | {{< icon name="dotted-circle" >}} No |  | When enabled, user scripts are migrated to steps and executed with the step-runner. |

<!-- feature_flags_list_end -->

## Enable feature flag in pipeline configuration

You can use [CI/CD variables](https://docs.gitlab.com/ci/variables/) to
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

## Enable feature flag in runner environment variables

To enable the feature for every job a Runner runs, specify the feature
flag as an
[`environment`](advanced-configuration.md#the-runners-section) variable
in the [Runner configuration](advanced-configuration.md):

```toml
[[runners]]
  name = "example-runner"
  url = "https://gitlab.com/"
  token = "TOKEN"
  limit = 0
  executor = "docker"
  builds_dir = ""
  shell = ""
  environment = ["FEATURE_FLAG_NAME=1"]
```

## Enable feature flag in runner configuration

You can enable feature flags by specifying them under `[runners.feature_flags]`. This
setting prevents any job from overriding the feature flag values.

Some feature flags are also only usable when you configure this setting, because
they don't deal with how the job is executed.

```toml
[[runners]]
  name = "example-runner"
  url = "https://gitlab.com/"
  token = "TOKEN"
  executor = "docker"
  [runners.feature_flags]
    FF_USE_DIRECT_DOWNLOAD = true
```
