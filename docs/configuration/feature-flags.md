---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# GitLab Runner feature flags

> - Introduced in GitLab 11.4.

WARNING:
Data corruption, stability degradation, performance degradation, and security issues may occur if you enable a feature that's disabled by default. Before you enable feature flags, you should be aware of the risks involved. For more information, see [Risks when enabling features still in development](https://docs.gitlab.com/ee/administration/feature_flags.html#risks-when-enabling-features-still-in-development).

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
| `FF_CMD_DISABLE_DELAYED_ERROR_LEVEL_EXPANSION` | `false` | **{dotted-circle}** No |  | Disables [EnableDelayedExpansion](https://ss64.com/nt/delayedexpansion.html) for error checking for when using [Window Batch](../shells/index.md#windows-batch) shell |
| `FF_NETWORK_PER_BUILD` | `false` | **{dotted-circle}** No |  | Enables creation of a Docker [network per build](../executors/docker.md#network-configurations) with the `docker` executor |
| `FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY` | `false` | **{dotted-circle}** No |  | When set to `false` disables execution of remote Kubernetes commands through `exec` in favor of `attach` to solve problems like [#4119](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4119) |
| `FF_USE_DIRECT_DOWNLOAD` | `true` | **{dotted-circle}** No |  | When set to `true` Runner tries to direct-download all artifacts instead of proxying through GitLab on a first try. Enabling might result in a download failures due to problem validating TLS certificate of Object Storage if it is enabled by GitLab. See [Self-signed certificates or custom Certification Authorities](tls-self-signed.md) |
| `FF_SKIP_NOOP_BUILD_STAGES` | `true` | **{dotted-circle}** No |  | When set to `false` all build stages are executed even if running them has no effect |
| `FF_USE_FASTZIP` | `false` | **{dotted-circle}** No |  | Fastzip is a performant archiver for cache/artifact archiving and extraction |
| `FF_DISABLE_UMASK_FOR_DOCKER_EXECUTOR` | `false` | **{dotted-circle}** No |  | If enabled will remove the usage of `umask 0000` call for jobs executed with `docker` executor. Instead Runner will try to discover the UID and GID of the user configured for the image used by the build container and will change the ownership of the working directory and files by running the `chmod` command in the predefined container (after updating sources, restoring cache and downloading artifacts). POSIX utility `id` must be installed and operational in the build image for this feature flag. Runner will execute `id` with options `-u` and `-g` to retrieve the UID and GID. |
| `FF_ENABLE_BASH_EXIT_CODE_CHECK` | `false` | **{dotted-circle}** No |  | If enabled, bash scripts don't rely solely on `set -e`, but check for a non-zero exit code after each script command is executed. |
| `FF_USE_WINDOWS_LEGACY_PROCESS_STRATEGY` | `false` | **{dotted-circle}** No |  | In GitLab Runner 16.10 and later, the default is `false`. In GitLab Runner 16.9 and earlier, the default is `true`. When disabled, processes that Runner creates on Windows (shell and custom executor) will be created with additional setup that should improve process termination. When set to `true`, legacy process setup is used. To successfully and gracefully drain a Windows Runner, this feature flag should be set to `false`. |
| `FF_USE_NEW_BASH_EVAL_STRATEGY` | `false` | **{dotted-circle}** No |  | When set to `true`, the Bash `eval` call is executed in a subshell to help with proper exit code detection of the script executed. |
| `FF_USE_POWERSHELL_PATH_RESOLVER` | `false` | **{dotted-circle}** No |  | When enabled, PowerShell resolves pathnames rather than Runner using OS-specific filepath functions that are specific to where Runner is hosted. |
| `FF_USE_DYNAMIC_TRACE_FORCE_SEND_INTERVAL` | `false` | **{dotted-circle}** No |  | When enabled, the trace force send interval for logs is dynamically adjusted based on the trace update interval. |
| `FF_SCRIPT_SECTIONS` | `false` | **{dotted-circle}** No |  | When enabled, each script line from the `.gitlab-ci.yml` file is in a collapsible section in the job output, and shows the duration of each line. When the command spans multiple lines, the complete command is displayed within the job log output terminal. |
| `FF_USE_NEW_SHELL_ESCAPE` | `false` | **{dotted-circle}** No |  | When enabled, a faster implementation of shell escape is used. |
| `FF_ENABLE_JOB_CLEANUP` | `false` | **{dotted-circle}** No |  | When enabled, the project directory will be cleaned up at the end of the build. If `GIT_CLONE` is used, the whole project directory will be deleted. If `GIT_FETCH` is used, a series of Git `clean` commands will be issued. |
| `FF_KUBERNETES_HONOR_ENTRYPOINT` | `false` | **{dotted-circle}** No |  | When enabled, the Docker entrypoint of an image will be honored if `FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY` is not set to true |
| `FF_POSIXLY_CORRECT_ESCAPES` | `false` | **{dotted-circle}** No |  | When enabled, [POSIX shell escapes](https://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html#tag_18_02) are used rather than [`bash`-style ANSI-C quoting](https://www.gnu.org/software/bash/manual/html_node/Quoting.html). This should be enabled if the job environment uses a POSIX-compliant shell. |
| `FF_RESOLVE_FULL_TLS_CHAIN` | `false` | **{dotted-circle}** No |  | In GitLab Runner 16.4 and later, the default is `false`. In GitLab Runner 16.3 and earlier, the default is `true`. When enabled, the runner resolves a full TLS chain all the way down to a self-signed root certificate for `CI_SERVER_TLS_CA_FILE`. This was previously [required to make Git HTTPS clones work](tls-self-signed.md#git-cloning) for a Git client built with libcurl prior to v7.68.0 and OpenSSL. However, the process to resolve certificates might fail on some operating systems, such as macOS, that reject root certificates signed with older signature algorithms. If certificate resolution fails, you might need to disable this feature. This feature flag can only be disabled in the [`[runners.feature_flags]` configuration](#enable-feature-flag-in-runner-configuration). |
| `FF_DISABLE_POWERSHELL_STDIN` | `false` | **{dotted-circle}** No |  | When enabled, PowerShell scripts for shell and custom executors are passed by file, rather than passed and executed via stdin. This is required for jobs' `allow_failure:exit_codes` keywords to work correctly. |
| `FF_USE_POD_ACTIVE_DEADLINE_SECONDS` | `true` | **{dotted-circle}** No |  | When enabled, the [pod `activeDeadlineSeconds`](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#lifecycle) is set to the CI/CD job timeout. This flag affects the [pod's lifecycle](../executors/kubernetes/index.md#pod-lifecycle). |
| `FF_USE_ADVANCED_POD_SPEC_CONFIGURATION` | `false` | **{dotted-circle}** No |  | When enabled, the user can set an entire whole pod specification in the `config.toml` file. For more information, see [Overwrite generated pod specifications (Experiment)](../executors/kubernetes/index.md#overwrite-generated-pod-specifications). |
| `FF_SET_PERMISSIONS_BEFORE_CLEANUP` | `true` | **{dotted-circle}** No |  | When enabled, permissions on directories and files in the project directory are set first, to ensure that deletions during cleanup are successful. |
| `FF_SECRET_RESOLVING_FAILS_IF_MISSING` | `true` | **{dotted-circle}** No |  | When enabled, secret resolving fails if the value cannot be found. |
| `FF_RETRIEVE_POD_WARNING_EVENTS` | `false` | **{dotted-circle}** No |  | When enabled, all warning events associated with the Pod are retrieved when the job fails. |
| `FF_PRINT_POD_EVENTS` | `false` | **{dotted-circle}** No |  | When enabled, all events associated with the build pod will be printed until it's started. |
| `FF_USE_GIT_BUNDLE_URIS` | `true` | **{dotted-circle}** No |  | When enabled, the Git `transfer.bundleURI` configuration option is set to `true`. This FF is enabled by default. Set to `false` to disable Git bundle support. |
| `FF_USE_DUMB_INIT_WITH_KUBERNETES_EXECUTOR` | `false` | **{dotted-circle}** No |  | When enabled, `dumb-init` is used to execute all the scripts. This allows `dumb-init` to run as the first process in the helper and build container. |
| `FF_USE_INIT_WITH_DOCKER_EXECUTOR` | `false` | **{dotted-circle}** No |  | When enabled, the Docker executor starts the service and build containers with the `--init` option, which runs `tini-init` as PID 1. |
| `FF_LOG_IMAGES_CONFIGURED_FOR_JOB` | `false` | **{dotted-circle}** No |  | When enabled, the runner logs names of the image and service images defined for each received job. |
| `FF_USE_DOCKER_AUTOSCALER_DIAL_STDIO` | `true` | **{dotted-circle}** No |  | When enabled (the default), `docker system stdio` is used to tunnel to the remote Docker daemon. When disabled, for SSH connections a native SSH tunnel is used, and for WinRM connections a 'fleeting-proxy' helper binary is first deployed. |
| `FF_CLEAN_UP_FAILED_CACHE_EXTRACT` | `false` | **{dotted-circle}** No |  | When enabled, commands are inserted into build scripts to detect a failed cache extraction and clean up partial cache contents left behind. |
| `FF_USE_WINDOWS_JOB_OBJECT` | `false` | **{dotted-circle}** No |  | When enabled, a job object is created for each process that the runner creates on Windows with the shell and custom executors. To force-kill the processes, the runner closes the job object. This should improve the termination of difficult-to-kill processes. |
| `FF_TIMESTAMPS` | `false` | **{dotted-circle}** No |  | When enabled timestamps are added to the beginning of each log trace line. |

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

## Enable feature flag in runner environment variables

To enable the feature for every job a Runner runs, specify the feature
flag as an
[`environment`](advanced-configuration.md#the-runners-section) variable
in the [Runner configuration](advanced-configuration.md):

```toml
[[runners]]
  name = "ruby-2.7-docker"
  url = "https://CI/"
  token = "TOKEN"
  limit = 0
  executor = "docker"
  builds_dir = ""
  shell = ""
  environment = ["FEATURE_FLAG_NAME=1"]
```

## Enable feature flag in runner configuration

> - [Introduced in](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/2811) GitLab Runner 13.11.

You can enable feature flags by specifying them under `[runners.feature_flags]`. This
setting prevents any job from overriding the feature flag values.

Some feature flags are also only usable when you configure this setting, because
they don't deal with how the job is executed.

```toml
[[runners]]
  name = "ruby-2.7-docker"
  url = "https://CI/"
  token = "TOKEN"
  executor = "docker"
  [runners.feature_flags]
    FF_USE_DIRECT_DOWNLOAD = true
```
