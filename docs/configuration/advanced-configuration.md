---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: Advanced configuration
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

To change the behavior of GitLab Runner and individual registered runners, modify the `config.toml` file.

You can find the `config.toml` file in:

- `/etc/gitlab-runner/` on \*nix systems when GitLab Runner is executed as root. This directory is also the path for
  service configuration.
- `~/.gitlab-runner/` on \*nix systems when GitLab Runner is executed as non-root.
- `./` on other systems.

GitLab Runner does not require a restart when you change most options. This includes parameters
in the `[[runners]]` section and most parameters in the global section, except for `listen_address`.
If a runner was already registered, you don't need to register it again.

GitLab Runner checks for configuration modifications every 3 seconds and reloads if necessary.
GitLab Runner also reloads the configuration in response to the `SIGHUP` signal.

## Configuration validation

{{< history >}}

- [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/3924) in GitLab Runner 15.10

{{< /history >}}

Configuration validation is a process that checks the structure of the `config.toml` file. The output from the configuration
validator provides only `info` level messages.

The configuration validation process is for informational purposes only. You can use the output to
identify potential issues with your runner configuration. The configuration validation might not catch all possible problems,
and the absence of messages does not guarantee that the `config.toml` file is flawless.

## The global section

These settings are global. They apply to all runners.

| Setting              | Description |
|----------------------|-------------|
| `concurrent`         | Limits how many jobs can run concurrently, across all registered runners. Each `[[runners]]` section can define its own limit, but this value sets a maximum for all of those values combined. For example, a value of `10` means no more than 10 jobs can run concurrently. `0` is forbidden. If you use this value, the runner process exits with a critical error. View how this setting works with the [Docker Machine executor](autoscale.md#limit-the-number-of-vms-created-by-the-docker-machine-executor), [Instance executor](../executors/instance.md), [Docker Autoscaler executor](../executors/docker_autoscaler.md#example-aws-autoscaling-for-1-job-per-instance), and [`runners.custom_build_dir` configuration](#the-runnerscustom_build_dir-section). |
| `log_level`          | Defines the log level. Options are `debug`, `info`, `warn`, `error`, `fatal`, and `panic`. This setting has lower priority than the level set by the command-line arguments `--debug`, `-l`, or `--log-level`. |
| `log_format`         | Specifies the log format. Options are `runner`, `text`, and `json`. This setting has lower priority than the format set by command-line argument `--log-format`. The default value is `runner`, which contains ANSI escape codes for coloring. |
| `check_interval`     | Defines the interval length, in seconds, between the runner checking for new jobs. The default value is `3`. If set to `0` or lower, the default value is used. |
| `sentry_dsn`         | Enables tracking of all system level errors to Sentry. |
| `connection_max_age` | The maximum duration a TLS keepalive connection to the GitLab server should remain open before reconnecting. The default value is `15m` for 15 minutes. If set to `0` or lower, the connection persists as long as possible. |
| `listen_address`     | Defines an address (`<host>:<port>`) the Prometheus metrics HTTP server should listen on. |
| `shutdown_timeout`   | Number of seconds until the [forceful shutdown operation](../commands/_index.md#signals) times out and exits the process. The default value is `30`. If set to `0` or lower, the default value is used. |

### Configuration warnings

#### Long polling issues

GitLab Runner can experience long polling issues in several configuration scenarios when GitLab
long polling is turned on through GitLab Workhorse. These range from performance bottlenecks to severe processing delays, depending on the configuration. GitLab Runner workers can get stuck in long polling requests for extended periods (matches the GitLab Workhorse configuration `-apiCiLongPollingDuration`, which defaults to 50 seconds), preventing other jobs from being processed promptly.

This issue is related to GitLab CI/CD long polling feature, which is controlled by
the GitLab Workhorse `-apiCiLongPollingDuration` setting. When turned on, job requests
can block for up to the configured duration while they wait for jobs to become available.

The default GitLab Workhorse long polling configuration value is 50 seconds (turned on by default in recent GitLab versions).

The following are some configuration examples:

- Omnibus: `gitlab_workhorse['api_ci_long_polling_duration'] = "50s"` in `/etc/gitlab/gitlab.rb`
- Helm chart: Use the `gitlab.webservice.workhorse.extraArgs` setting
- CLI: `gitlab-workhorse -apiCiLongPollingDuration 50s`

For more information, see:

- [Long polling for runners](https://docs.gitlab.com/ci/runners/long_polling/)
- [Workhorse configuration](https://docs.gitlab.com/development/workhorse/configuration/)

Symptoms:

- Jobs from some projects experience delays before starting (duration matches your GitLab instance long polling timeout)
- Jobs from other projects run immediately
- Warning message in runner logs: `CONFIGURATION: Long polling issues detected`

Common problematic scenarios:

- Worker starvation bottleneck: The `concurrent` setting is less than the number of runners (severe bottleneck)
- Request bottleneck: Runners with `request_concurrency=1` cause job delays during long polling
- Build limit bottleneck: Runners with low `limit` settings (≤2) combined with `request_concurrency=1`

GitLab Runner automatically detects the problem scenarios and provides tailored solutions in the
warning messages. Common solutions include:

- Increase the `concurrent` setting to exceed the number of runners.
- Set the `request_concurrency` value for high-volume runners to a value higher than 1 (default is 1).
  Consider turning on [runner monitoring](../monitoring/_index.md) to understand the state of your system and find the best
  value for the setting. Consider using the `FF_USE_ADAPTIVE_REQUEST_CONCURRENCY` feature flag to automatically
  adjust `request_concurrency` based on workload. For information about adaptive concurrency,
  see the [feature flags documentation](feature-flags.md).
- Balance `limit` settings with expected job volume.

##### Example problematic configurations

Scenario 1: Worker starvation bottleneck:

```toml
concurrent = 2  # Only 2 concurrent workers

[[runners]]
  name = "runner-1"
[[runners]]
  name = "runner-2"
[[runners]]
  name = "runner-3"  # 3 runners, only 2 workers - severe bottleneck
```

Scenario 2: Request bottleneck:

```toml
concurrent = 4  # 4 workers available

[[runners]]
  name = "high-volume-runner"
  request_concurrency = 1  # Default: only 1 request at a time
  limit = 10               # Can handle 10 jobs, but only 1 request slot
```

Scenario 3: Build limit bottleneck:

```toml
concurrent = 4

[[runners]]
  name = "limited-runner"
  limit = 2                # Only 2 builds allowed
  request_concurrency = 1  # Only 1 request at a time
  # Creates severe bottleneck: builds at capacity + request slot blocked by long polling
```

##### Example corrected configuration

```toml
concurrent = 4  # Adequate worker capacity

[[runners]]
  name = "high-volume-runner"
  request_concurrency = 3  # Allow multiple simultaneous requests
  limit = 10

[[runners]]
  name = "balanced-runner"
  request_concurrency = 2
  limit = 5
```

Here's a configuration example:

```toml

# Example `config.toml` file

concurrent = 100 # A global setting for job concurrency that applies to all runner sections defined in this `config.toml` file
log_level = "warning"
log_format = "text"
check_interval = 3 # Value in seconds

[[runners]]
  name = "first"
  url = "Your Gitlab instance URL (for example, `https://gitlab.com`)"
  executor = "shell"
  (...)

[[runners]]
  name = "second"
  url = "Your Gitlab instance URL (for example, `https://gitlab.com`)"
  executor = "docker"
  (...)

[[runners]]
  name = "third"
  url = "Your Gitlab instance URL (for example, `https://gitlab.com`)"
  executor = "docker-autoscaler"
  (...)

```

### `log_format` examples (truncated)

#### `runner`

```shell
Runtime platform                                    arch=amd64 os=darwin pid=37300 revision=HEAD version=development version
Starting multi-runner from /etc/gitlab-runner/config.toml...  builds=0
WARNING: Running in user-mode.
WARNING: Use sudo for system-mode:
WARNING: $ sudo gitlab-runner...

Configuration loaded                                builds=0
listen_address not defined, metrics & debug endpoints disabled  builds=0
[session_server].listen_address not defined, session endpoints disabled  builds=0
```

#### `text`

```shell
INFO[0000] Runtime platform                              arch=amd64 os=darwin pid=37773 revision=HEAD version="development version"
INFO[0000] Starting multi-runner from /etc/gitlab-runner/config.toml...  builds=0
WARN[0000] Running in user-mode.
WARN[0000] Use sudo for system-mode:
WARN[0000] $ sudo gitlab-runner...
INFO[0000]
INFO[0000] Configuration loaded                          builds=0
INFO[0000] listen_address not defined, metrics & debug endpoints disabled  builds=0
INFO[0000] [session_server].listen_address not defined, session endpoints disabled  builds=0
```

#### `json`

```shell
{"arch":"amd64","level":"info","msg":"Runtime platform","os":"darwin","pid":38229,"revision":"HEAD","time":"2025-06-05T15:57:35+02:00","version":"development version"}
{"builds":0,"level":"info","msg":"Starting multi-runner from /etc/gitlab-runner/config.toml...","time":"2025-06-05T15:57:35+02:00"}
{"level":"warning","msg":"Running in user-mode.","time":"2025-06-05T15:57:35+02:00"}
{"level":"warning","msg":"Use sudo for system-mode:","time":"2025-06-05T15:57:35+02:00"}
{"level":"warning","msg":"$ sudo gitlab-runner...","time":"2025-06-05T15:57:35+02:00"}
{"level":"info","msg":"","time":"2025-06-05T15:57:35+02:00"}
{"builds":0,"level":"info","msg":"Configuration loaded","time":"2025-06-05T15:57:35+02:00"}
{"builds":0,"level":"info","msg":"listen_address not defined, metrics \u0026 debug endpoints disabled","time":"2025-06-05T15:57:35+02:00"}
{"builds":0,"level":"info","msg":"[session_server].listen_address not defined, session endpoints disabled","time":"2025-06-05T15:57:35+02:00"}
```

### How `check_interval` works

If `config.toml` has more than one `[[runners]]` section, GitLab Runner contains a loop that
constantly schedules job requests to the GitLab instance where GitLab Runner is configured.

The following example has `check_interval` of 10 seconds and two `[[runners]]` sections
(`runner-1` and `runner-2`). GitLab Runner sends a request every 10 seconds and sleeps for five seconds:

1. Get `check_interval` value (`10s`).
1. Get list of runners (`runner-1`, `runner-2`).
1. Calculate the sleep interval (`10s / 2 = 5s`).
1. Start an infinite loop:
   1. Request a job for `runner-1`.
   1. Sleep for `5s`.
   1. Request a job for `runner-2`.
   1. Sleep for `5s`.
   1. Repeat.

Here's a `check_interval` configuration example:

```toml
# Example `config.toml` file

concurrent = 100 # A global setting for job concurrency that applies to all runner sections defined in this `config.toml` file.
log_level = "warning"
log_format = "json"
check_interval = 10 # Value in seconds

[[runners]]
  name = "runner-1"
  url = "Your Gitlab instance URL (for example, `https://gitlab.com`)"
  executor = "shell"
  (...)

[[runners]]
  name = "runner-2"
  url = "Your Gitlab instance URL (for example, `https://gitlab.com`)"
  executor = "docker"
  (...)
```

In this example, a job request from the runner's process is made every five seconds.
If `runner-1` and `runner-2` are connected to the same
GitLab instance, this GitLab instance also receives a new request from this runner
every five seconds.

Two sleep periods occur between the first and second requests for `runner-1`.
Each period takes five seconds, so it's approximately 10 seconds between subsequent requests for `runner-1`.
The same applies for `runner-2`.

If you define more runners, the sleep interval is smaller. However, a request for a runner is
repeated after all requests for the other runners and their sleep periods are called.

## The `[session_server]` section

To interact with jobs, specify the `[session_server]` section
at the root level, outside the `[[runners]]` section.
Configure this section once for all runners, not for each individual runner.

```toml
# Example `config.toml` file with session server configured

concurrent = 100 # A global setting for job concurrency that applies to all runner sections defined in this `config.toml` file
log_level = "warning"
log_format = "runner"
check_interval = 3 # Value in seconds

[session_server]
  listen_address = "[::]:8093" # Listen on all available interfaces on port `8093`
  advertise_address = "runner-host-name.tld:8093"
  session_timeout = 1800
```

When you configure the `[session_server]` section:

- For `listen_address` and `advertise_address`, use the format `host:port`, where `host`
  is the IP address (`127.0.0.1:8093`) or domain (`my-runner.example.com:8093`). The
  runner uses this information to create a TLS certificate for a secure connection.
- Ensure that GitLab can connect to the IP address and port defined in `listen_address` or `advertise_address`.
- Ensure that `advertise_address` is a public IP address, unless you have enabled the application setting, [`allow_local_requests_from_web_hooks_and_services`](https://docs.gitlab.com/api/settings/#available-settings).

| Setting             | Description |
|---------------------|-------------|
| `listen_address`    | An internal URL for the session server. |
| `advertise_address` | The URL to access the session server. GitLab Runner exposes it to GitLab. If not defined, `listen_address` is used. |
| `session_timeout`   | Number of seconds the session can stay active after the job completes. The timeout blocks the job from finishing. Default is `1800` (30 minutes). |

To disable the session server and terminal support, delete the `[session_server]` section.

{{< alert type="note" >}}

When your runner instance is already running, you might need to execute `gitlab-runner restart` for the changes in the `[session_server]` section to be take effect.

{{< /alert >}}

If you are using the GitLab Runner Docker image, you must expose port `8093` by
adding `-p 8093:8093` to your [`docker run` command](../install/docker.md).

## The `[[runners]]` section

Each `[[runners]]` section defines one runner.

| Setting                               | Description                                                                                                                                                                                                                                                                                                                                                                                                 |
| ------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `name`                                | The runner's description. Informational only.                                                                                                                                                                                                                                                                                                                                                               |
| `url`                                 | GitLab instance URL.                                                                                                                                                                                                                                                                                                                                                                                        |
| `token`                               | The runner's authentication token, which is obtained during runner registration. [Not the same as the registration token](https://docs.gitlab.com/api/runners/#registration-and-authentication-tokens).                                                                                                                                                                                                     |
| `tls-ca-file`                         | When using HTTPS, file that contains the certificates to verify the peer. See [Self-signed certificates or custom Certification Authorities documentation](tls-self-signed.md).                                                                                                                                                                                                                             |
| `tls-cert-file`                       | When using HTTPS, file that contains the certificate to authenticate with the peer.                                                                                                                                                                                                                                                                                                                         |
| `tls-key-file`                        | When using HTTPS, file that contains the private key to authenticate with the peer.                                                                                                                                                                                                                                                                                                                         |
| `limit`                               | Limit how many jobs can be handled concurrently by this registered runner. `0` (default) means do not limit. View how this setting works with the [Docker Machine](autoscale.md#limit-the-number-of-vms-created-by-the-docker-machine-executor), [Instance](../executors/instance.md), and [Docker Autoscaler](../executors/docker_autoscaler.md#example-aws-autoscaling-for-1-job-per-instance) executors. |
| `executor`                            | The environment or command processor on the host operating system that the runner uses to run a CI/CD job. For more information, see [executors](../executors/_index.md).                                                                                                                                                                                                                                   |
| `shell`                               | Name of shell to generate the script. Default value is [platform dependent](../shells/_index.md).                                                                                                                                                                                                                                                                                                           |
| `builds_dir`                          | Absolute path to a directory where builds are stored in the context of the selected executor. For example, locally, Docker, or SSH.                                                                                                                                                                                                                                                                         |
| `cache_dir`                           | Absolute path to a directory where build caches are stored in context of selected executor. For example, locally, Docker, or SSH. If the `docker` executor is used, this directory needs to be included in its `volumes` parameter.                                                                                                                                                                         |
| `environment`                         | Append or overwrite environment variables.                                                                                                                                                                                                                                                                                                                                                                  |
| `request_concurrency`                 | Limit number of concurrent requests for new jobs from GitLab. Default is `1`. For more information about how `concurrency` , `limit`, and `request_concurrency` interact to control job flow, see the [KB article on GitLab Runner concurrency tuning](https://support.gitlab.com/hc/en-us/articles/21324350882076-GitLab-Runner-Concurrency-Tuning-Understanding-request-concurrency).                     |
| `strict_check_interval`               | Under normal operation, when a runner polls for jobs and receives a job, it immediately re-polls for jobs until the number of jobs being processed matches `concurrent` or `limit`, or until no jobs are available. When you turn on `strict_check_interval`, the runner disables this faster-than-`check_interval` re-polling loop and strictly respects `check_interval`. Default is `false`.                    |
| `output_limit`                        | Maximum build log size in kilobytes. Default is `4096` (4 MB).                                                                                                                                                                                                                                                                                                                                              |
| `pre_get_sources_script`              | Commands to be executed on the runner before updating the Git repository and updating submodules. Use it to adjust the Git client configuration first, for example. To insert multiple commands, use a (triple-quoted) multi-line string or `\n` character.                                                                                                                                                 |
| `post_get_sources_script`             | Commands to be executed on the runner after updating the Git repository and updating submodules. To insert multiple commands, use a (triple-quoted) multi-line string or `\n` character.                                                                                                                                                                                                                    |
| `pre_build_script`                    | Commands to be executed on the runner before executing the job. To insert multiple commands, use a (triple-quoted) multi-line string or `\n` character.                                                                                                                                                                                                                                                     |
| `post_build_script`                   | Commands to be executed on the runner just after executing the job, but before executing `after_script`. To insert multiple commands, use a (triple-quoted) multi-line string or `\n` character.                                                                                                                                                                                                            |
| `clone_url`                           | Overwrite the URL for the GitLab instance. Used only if the runner can't connect to the GitLab URL.                                                                                                                                                                                                                                                                                                         |
| `debug_trace_disabled`                | Disables [debug tracing](https://docs.gitlab.com/ci/variables/#enable-debug-logging). When set to `true`, the debug log (trace) remains disabled even if `CI_DEBUG_TRACE` is set to `true`.                                                                                                                                                                                                                 |
| `clean_git_config`                    | Cleans the Git configuration. For more information, see [Cleaning Git configuration](#cleaning-git-configuration).                                                                                                                                                                                                                                                                                          |
| `referees`                            | Extra job monitoring workers that pass their results as job artifacts to GitLab.                                                                                                                                                                                                                                                                                                                            |
| `unhealthy_requests_limit`            | The number of `unhealthy` responses to new job requests after which a runner worker is disabled.                                                                                                                                                                                                                                                                                                            |
| `unhealthy_interval`                  | Duration that a runner worker is disabled for after it exceeds the unhealthy requests limit. Supports syntax like `3600 s`, `1 h 30 min`, and similar.                                                                                                                                                                                                                                                      |
| `job_status_final_update_retry_limit` | The maximum number of times GitLab Runner can retry to push the final job status to the GitLab instance.                                                                                                                                                                                                                                                                                                    |

Example:

```toml
[[runners]]
  name = "example-runner"
  url = "http://gitlab.example.com/"
  token = "TOKEN"
  limit = 0
  executor = "docker"
  builds_dir = ""
  shell = ""
  environment = ["ENV=value", "LC_ALL=en_US.UTF-8"]
  clone_url = "http://gitlab.example.local"
```

### Legacy `/ci` URL suffix

{{< history >}}

- Deprecated in [GitLab Runner 1.0.0](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/289).
- Warning added in GitLab Runner 18.7.0.

{{< /history >}}

In versions of GitLab Runner before 1.0.0, the runner URL was configured with a `/ci` suffix,
such as `url = "https://gitlab.example.com/ci"`. This suffix is no longer required and should be removed
from your configuration.

If your `config.toml` contains a URL with the `/ci` suffix, GitLab Runner automatically strips it when
processing the configuration. However, you should update your configuration file to remove the suffix to
avoid potential issues.

#### Known issues

- Git submodule authentication failures: When `GIT_SUBMODULE_FORCE_HTTPS=true` is set, submodules might fail
  to clone with authentication errors like `fatal: could not read Username for 'https://gitlab.example.com': terminal prompts disabled`.
  This issue occurs because the `/ci` suffix interferes with Git URL rewriting rules. For more details, see
  [issue 581678](https://gitlab.com/gitlab-org/gitlab/-/work_items/581678#note_2934077238).

**Problematic configuration**:

```toml
[[runners]]
  name = "legacy-runner"
  url = "https://gitlab.example.com/ci"  # Remove the /ci suffix
  token = "TOKEN"
  executor = "docker"
```

**Corrected configuration**:

```toml
[[runners]]
  name = "legacy-runner"
  url = "https://gitlab.example.com"  # /ci suffix removed
  token = "TOKEN"
  executor = "docker"
```

When GitLab Runner starts with a URL containing the `/ci` suffix, it logs a warning message:

```plaintext
WARNING: The runner URL contains a legacy '/ci' suffix. This suffix is deprecated and should be
removed from the configuration. Git submodules may fail to clone with authentication errors if this
suffix is present. Please update the 'url' field in your config.toml to remove the '/ci' suffix.
See https://docs.gitlab.com/runner/configuration/advanced-configuration/#legacy-ci-url-suffix for more information.
```

To resolve this warning, edit your `config.toml` file and remove the `/ci` suffix from the `url` field.

### How `clone_url` works

When the GitLab instance is available at a URL that the runner can't use,
you can configure a `clone_url`.

For example, a firewall might prevent the runner from reaching the URL.
If the runner can reach the node on `192.168.1.23`, set the `clone_url` to `http://192.168.1.23`.

If the `clone_url` is set, the runner constructs a clone URL in the form
of `http://gitlab-ci-token:s3cr3tt0k3n@192.168.1.23/namespace/project.git`.

{{< alert type="note" >}}

`clone_url` does not affect Git LFS endpoints or artifact uploads or downloads.

{{< /alert >}}

#### Modify Git LFS endpoints

To modify [Git LFS](https://docs.gitlab.com/topics/git/lfs/) endpoints, set `pre_get_sources_script` in one of the following files:

- `config.toml`:

  ```toml
  pre_get_sources_script = "mkdir -p $RUNNER_TEMP_PROJECT_DIR/git-template; git config -f $RUNNER_TEMP_PROJECT_DIR/git-template/config lfs.url https://<alternative-endpoint>"
  ```

- `.gitlab-ci.yml`:

  ```yaml
  default:
    hooks:
      pre_get_sources_script:
        - mkdir -p $RUNNER_TEMP_PROJECT_DIR/git-template
        - git config -f $RUNNER_TEMP_PROJECT_DIR/git-template/config lfs.url https://localhost
  ```

### How `unhealthy_requests_limit` and `unhealthy_interval` works

When a GitLab instance is unavailable for a long time (for example, during a
version upgrade), its runners become idle. The runners
do not resume job processing for 30-60 minutes after
the GitLab instance is available again.

To increase or decrease the duration that runners are idle, change the `unhealthy_interval` setting.

To change runner's number of connection attempts to the GitLab server and
receive an unhealthy sleep before becoming idle, change the `unhealthy_requests_limit` setting.
For more information, see [How `check_interval` works](advanced-configuration.md#how-check_interval-works).

## The executors

The following executors are available.

| Executor            | Required configuration                                                  | Where jobs run |
|---------------------|-------------------------------------------------------------------------|----------------|
| `shell`             |                                                                         | Local shell. The default executor. |
| `docker`            | `[runners.docker]` and [Docker Engine](https://docs.docker.com/engine/) | A Docker container. |
| `docker-windows`    | `[runners.docker]` and [Docker Engine](https://docs.docker.com/engine/) | A Windows Docker container. |
| `ssh`               | `[runners.ssh]`                                                         | SSH, remotely. |
| `parallels`         | `[runners.parallels]` and `[runners.ssh]`                               | Parallels VM, but connect with SSH. |
| `virtualbox`        | `[runners.virtualbox]` and `[runners.ssh]`                              | VirtualBox VM, but connect with SSH. |
| `docker+machine`    | `[runners.docker]` and `[runners.machine]`                              | Like `docker`, but use [auto-scaled Docker machines](autoscale.md). |
| `kubernetes`        | `[runners.kubernetes]`                                                  | Kubernetes pods. |
| `docker-autoscaler` | `[docker-autoscaler]` and `[runners.autoscaler]`                        | Like `docker`, but uses autoscaled instances to run CI/CD jobs in containers. |
| `instance`          | `[docker-autoscaler]` and `[runners.autoscaler]`                        | Like `shell`, but uses autoscaled instances to run CI/CD jobs directly on the host instance. |

## The shells

CI/CD jobs run locally on the host machine when configured to use the shell executor. The supported operating system shells are:

| Shell        | Description |
|--------------|-------------|
| `bash`       | Generate Bash (Bourne-shell) script. All commands executed in Bash context. Default for all Unix systems. |
| `sh`         | Generate Sh (Bourne-shell) script. All commands executed in Sh context. The fallback for `bash` for all Unix systems. |
| `powershell` | Generate PowerShell script. All commands are executed in PowerShell Desktop context. |
| `pwsh`       | Generate PowerShell script. All commands are executed in PowerShell Core context. This is the default shell for Windows. |

When the `shell` option is set to `bash` or `sh`, Bash's [ANSI-C quoting](https://www.gnu.org/software/bash/manual/html_node/ANSI_002dC-Quoting.html) is used
to shell escape job scripts.

### Use a POSIX-compliant shell

In GitLab Runner 14.9 and later, [enable the feature flag](feature-flags.md) named
`FF_POSIXLY_CORRECT_ESCAPES` to use a POSIX-compliant shell (like `dash`).
When enabled, ["Double Quotes"](https://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html#tag_18_02),
which is POSIX-compliant shell escaping mechanism, is used.

## The `[runners.docker]` section

The following settings define the Docker container parameters. These settings are applicable when the runner is configured to use the Docker executor.

[Docker-in-Docker](https://docs.gitlab.com/ci/docker/using_docker_build/#use-docker-in-docker) as a service, or any container runtime configured inside a job, does not inherit these parameters.

| Parameter                          | Example                                          | Description |
|------------------------------------|--------------------------------------------------|-------------|
| `allowed_images`                   | `["ruby:*", "python:*", "php:*"]`                | Wildcard list of images that can be specified in the `.gitlab-ci.yml` file. If not present, all images are allowed (equivalent to `["*/*:*"]`). Use with the [Docker](../executors/docker.md#restrict-docker-images-and-services) or [Kubernetes](../executors/kubernetes/_index.md#restrict-docker-images-and-services) executors. |
| `allowed_privileged_images`        |                                                  | Wildcard subset of `allowed_images` that runs in privileged mode when `privileged` is enabled. If not present, all images are allowed (equivalent to `["*/*:*"]`). Use with the [Docker](../executors/docker.md#restrict-docker-images-and-services) executors. |
| `allowed_pull_policies`            |                                                  | List of pull policies that can be specified in the `.gitlab-ci.yml` file or the `config.toml` file. If not specified, only the pull policies specified in `pull-policy` are allowed. Use with the [Docker](../executors/docker.md#allow-docker-pull-policies) executor. |
| `allowed_services`                 | `["postgres:9", "redis:*", "mysql:*"]`           | Wildcard list of services that can be specified in the `.gitlab-ci.yml` file. If not present, all images are allowed (equivalent to `["*/*:*"]`). Use with the [Docker](../executors/docker.md#restrict-docker-images-and-services) or [Kubernetes](../executors/kubernetes/_index.md#restrict-docker-images-and-services) executors. |
| `allowed_privileged_services`      |                                                  | Wildcard subset of `allowed_services` that is allowed to run in privileged mode, when `privileged` or `services_privileged` is enabled. If not present, all images are allowed (equivalent to `["*/*:*"]`). Use with the [Docker](../executors/docker.md#restrict-docker-images-and-services) executors. |
| `cache_dir`                        |                                                  | Directory where Docker caches should be stored. This path can be absolute or relative to current working directory. See `disable_cache` for more information. |
| `cap_add`                          | `["NET_ADMIN"]`                                  | Add additional Linux capabilities to the container. |
| `cap_drop`                         | `["DAC_OVERRIDE"]`                               | Drop additional Linux capabilities from the container. |
| `cpuset_cpus`                      | `"0,1"`                                          | The control group's `CpusetCpus`. A string. |
| `cpuset_mems`                      | `"0,1"`                                          | The control group's `CpusetMems`. A string. |
| `cpu_shares`                       |                                                  | Number of CPU shares used to set relative CPU usage. Default is `1024`. |
| `cpus`                             | `"2"`                                            | Number of CPUs (available in Docker 1.13 or later). A string. |
| `devices`                          | `["/dev/net/tun"]`                               | Share additional host devices with the container. |
| `device_cgroup_rules`              |                                                  | Custom device `cgroup` rules (available in Docker 1.28 or later). |
| `disable_cache`                    |                                                  | The Docker executor has two levels of caching: a global one (like any other executor) and a local cache based on Docker volumes. This configuration flag acts only on the local one which disables the use of automatically created (not mapped to a host directory) cache volumes. In other words, it only prevents creating a container that holds temporary files of builds, it does not disable the cache if the runner is configured in [distributed cache mode](autoscale.md#distributed-runners-caching). |
| `disable_entrypoint_overwrite`     |                                                  | Disable the image entrypoint overwriting. |
| `dns`                              | `["8.8.8.8"]`                                    | A list of DNS servers for the container to use. |
| `dns_search`                       |                                                  | A list of DNS search domains. |
| `extra_hosts`                      | `["other-host:127.0.0.1"]`                       | Hosts that should be defined in container environment. |
| `gpus`                             |                                                  | GPU devices for Docker container. Uses the same format as the `docker` CLI. View details in the [Docker documentation](https://docs.docker.com/engine/containers/resource_constraints/#gpu). Requires [configuration to enable GPUs](gpus.md#docker-executor). |
| `group_add`                        | `["docker"]`                                     | Add additional groups for the container process to run. |
| `helper_image`                     |                                                  | (Advanced) [The default helper image](#helper-image) used to clone repositories and upload artifacts. |
| `helper_image_flavor`              |                                                  | Sets the helper image flavor (`alpine`, `alpine3.21`, `alpine-latest`, `ubi-fips` or `ubuntu`). Defaults to `alpine`. The `alpine` flavor uses the same version as `alpine-latest`. |
| `helper_image_autoset_arch_and_os` |                                                  | Uses the underlying OS to set the Helper Image architecture and OS. |
| `host`                             |                                                  | Custom Docker endpoint. Default is `DOCKER_HOST` environment or `unix:///var/run/docker.sock`. |
| `hostname`                         |                                                  | Custom hostname for the Docker container. |
| `image`                            | `"ruby:3.3"`                                     | The image to run jobs with. |
| `links`                            | `["mysql_container:mysql"]`                      | Containers that should be linked with container that runs the job. |
| `memory`                           | `"128m"`                                         | The memory limit. A string. |
| `memory_swap`                      | `"256m"`                                         | The total memory limit. A string. |
| `memory_reservation`               | `"64m"`                                          | The memory soft limit. A string. |
| `network_mode`                     |                                                  | Add container to a custom network. |
| `mac_address`                      | `92:d0:c6:0a:29:33`                              | Container MAC address |
| `oom_kill_disable`                 |                                                  | If an out-of-memory (`OOM`) error occurs, do not terminate processes in a container. |
| `oom_score_adjust`                 |                                                  | `OOM` score adjustment. Positive means terminate the processes earlier. |
| `privileged`                       | `false`                                          | Make the container run in privileged mode. Insecure. |
| `services_privileged`              |                                                  | Allow services to run in privileged mode. If unset (default) `privileged` value is used instead. Use with the [Docker](../executors/docker.md#allow-docker-pull-policies) executor. Insecure. |
| `pull_policy`                      |                                                  | The image pull policy: `never`, `if-not-present` or `always` (default). View details in the [pull policies documentation](../executors/docker.md#configure-how-runners-pull-images). You can also add [multiple pull policies](../executors/docker.md#set-multiple-pull-policies), [retry a failed pull](../executors/docker.md#retry-a-failed-pull), or [restrict pull policies](../executors/docker.md#allow-docker-pull-policies). |
| `runtime`                          |                                                  | The runtime for the Docker container. |
| `isolation`                        |                                                  | Container isolation technology (`default`, `hyperv` and `process`). Windows only. |
| `security_opt`                     |                                                  | Security options (--security-opt in `docker run`). Takes a list of `:` separated key/values. `systempaths` specification is not supported. For more information, see [issue 36810](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/36810). |
| `shm_size`                         | `300000`                                         | Shared memory size for images (in bytes). |
| `sysctls`                          |                                                  | The `sysctl` options. |
| `tls_cert_path`                    | On macOS `/Users/<username>/.boot2docker/certs`. | A directory where `ca.pem`, `cert.pem` or `key.pem` are stored and used to make a secure TLS connection to Docker. Use this setting with `boot2docker`. |
| `tls_verify`                       |                                                  | Enable or disable TLS verification of connections to the Docker daemon. Disabled by default. By default, GitLab Runner connects to the Docker Unix socket over SSH. The Unix socket does not support RTLS and communicates over HTTP with SSH to provide encryption and authentication. Enabling `tls_verify` is not typically needed and requires additional configuration. To enable `tls_verify`, the daemon must listen on a port (rather than the default Unix socket) and the GitLab Runner Docker host must use the address the daemon is listening on. |
| `user`                             |                                                  | Run all commands in the container as the specified user. |
| `userns_mode`                      |                                                  | The user namespace mode for the container and Docker services when user namespace remapping option is enabled. Available in Docker 1.10 or later. For details, see [Docker documentation](https://docs.docker.com/engine/security/userns-remap/#disable-namespace-remapping-for-a-container). |
| `ulimit`                           |                                                  | Ulimit values that are passed to the container. Uses the same syntax as the Docker `--ulimit` flag. |
| `volumes`                          | `["/data", "/home/project/cache"]`               | Additional volumes that should be mounted. Same syntax as the Docker `-v` flag. |
| `volumes_from`                     | `["storage_container:ro"]`                       | A list of volumes to inherit from another container in the form `<container name>[:<access_level>]`. Access level defaults to read-write, but can be manually set to `ro` (read-only) or `rw` (read-write). |
| `volume_driver`                    |                                                  | The volume driver to use for the container. |
| `wait_for_services_timeout`        | `30`                                             | How long to wait for Docker services. Set to `-1` to disable. Default is `30`. |
| `container_labels`                 |                                                  | A set of labels to add to each container created by the runner. The label value can include environment variables for expansion. |
| `services_limit`                   |                                                  | Set the maximum allowed services per job. `-1` (default) means there is no limit. |
| `service_cpuset_cpus`              |                                                  | String value containing the `cgroups CpusetCpus` to use for a service. |
| `service_cpu_shares`               |                                                  | Number of CPU shares used to set a service's relative CPU usage (default: [`1024`](https://docs.docker.com/engine/containers/resource_constraints/#cpu)). |
| `service_cpus`                     |                                                  | String value of the number of CPUs for a service. Available in Docker 1.13 or later. |
| `service_gpus`                     |                                                  | GPU devices for Docker container. Uses the same format as the `docker` CLI. View details in the [Docker documentation](https://docs.docker.com/engine/containers/resource_constraints/#gpu). Requires [configuration to enable GPUs](gpus.md#docker-executor). |
| `service_memory`                   |                                                  | String value of the memory limit for a service. |
| `service_memory_swap`              |                                                  | String value of the total memory limit for a service. |
| `service_memory_reservation`       |                                                  | String value of the memory soft limit for a service. |

### The `[[runners.docker.services]]` section

Specify additional [services](https://docs.gitlab.com/ci/services/) to run with the job. For a list of available images, see the
[Docker Registry](https://hub.docker.com).
Each service runs in a separate container and is linked to the job.

| Parameter     | Example                            | Description |
|---------------|------------------------------------|-------------|
| `name`        | `"registry.example.com/svc1"`      | The name of the image to be run as a service. |
| `alias`       | `"svc1"`                           | Additional [alias name](https://docs.gitlab.com/ci/services/#available-settings-for-services) that can be used to access the service. |
| `entrypoint`  | `["entrypoint.sh"]`                | Command or script that should be executed as the container's entrypoint. The syntax is similar to the [Dockerfile ENTRYPOINT](https://docs.docker.com/reference/dockerfile/#entrypoint) directive, where each shell token is a separate string in the array. Introduced in [GitLab Runner 13.6](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27173). |
| `command`     | `["executable","param1","param2"]` | Command or script that should be used as the container's command. The syntax is similar to the [Dockerfile CMD](https://docs.docker.com/reference/dockerfile/#cmd) directive, where each shell token is a separate string in the array. Introduced in [GitLab Runner 13.6](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27173). |
| `environment` | `["ENV1=value1", "ENV2=value2"]`   | Append or overwrite environment variables for the service container. |

Example:

```toml
[runners.docker]
  host = ""
  hostname = ""
  tls_cert_path = "/Users/ayufan/.boot2docker/certs"
  image = "ruby:3.3"
  memory = "128m"
  memory_swap = "256m"
  memory_reservation = "64m"
  oom_kill_disable = false
  cpuset_cpus = "0,1"
  cpuset_mems = "0,1"
  cpus = "2"
  dns = ["8.8.8.8"]
  dns_search = [""]
  service_memory = "128m"
  service_memory_swap = "256m"
  service_memory_reservation = "64m"
  service_cpuset_cpus = "0,1"
  service_cpus = "2"
  services_limit = 5
  privileged = false
  group_add = ["docker"]
  cap_add = ["NET_ADMIN"]
  cap_drop = ["DAC_OVERRIDE"]
  devices = ["/dev/net/tun"]
  disable_cache = false
  wait_for_services_timeout = 30
  cache_dir = ""
  volumes = ["/data", "/home/project/cache"]
  extra_hosts = ["other-host:127.0.0.1"]
  shm_size = 300000
  volumes_from = ["storage_container:ro"]
  links = ["mysql_container:mysql"]
  allowed_images = ["ruby:*", "python:*", "php:*"]
  allowed_services = ["postgres:9", "redis:*", "mysql:*"]
  [runners.docker.ulimit]
    "rtprio" = "99"
  [[runners.docker.services]]
    name = "registry.example.com/svc1"
    alias = "svc1"
    entrypoint = ["entrypoint.sh"]
    command = ["executable","param1","param2"]
    environment = ["ENV1=value1", "ENV2=value2"]
  [[runners.docker.services]]
    name = "redis:2.8"
    alias = "cache"
  [[runners.docker.services]]
    name = "postgres:9"
    alias = "postgres-db"
  [runners.docker.sysctls]
    "net.ipv4.ip_forward" = "1"
```

### Volumes in the `[runners.docker]` section

For more information about volumes, see the [Docker documentation](https://docs.docker.com/engine/storage/volumes/).

The following examples show how to specify volumes in the `[runners.docker]` section.

#### Example 1: Add a data volume

A data volume is a specially-designated directory in one or more containers
that bypasses the Union File System. Data volumes are designed to persist data,
independent of the container's lifecycle.

```toml
[runners.docker]
  host = ""
  hostname = ""
  tls_cert_path = "/Users/ayufan/.boot2docker/certs"
  image = "ruby:3.3"
  privileged = false
  disable_cache = true
  volumes = ["/path/to/volume/in/container"]
```

This example creates a new volume in the container at `/path/to/volume/in/container`.

#### Example 2: Mount a host directory as a data volume

When you want to store directories outside the container, you can mount
a directory from your Docker daemon's host into a container:

```toml
[runners.docker]
  host = ""
  hostname = ""
  tls_cert_path = "/Users/ayufan/.boot2docker/certs"
  image = "ruby:3.3"
  privileged = false
  disable_cache = true
  volumes = ["/path/to/bind/from/host:/path/to/bind/in/container:rw"]
```

This example uses `/path/to/bind/from/host` of the CI/CD host in the container at
`/path/to/bind/in/container`.

GitLab Runner 11.11 and later [mount the host directory](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/1261)
for the defined [services](https://docs.gitlab.com/ci/services/) as
well.

### Use a private container registry

To use private registries as a source of images for your jobs, configure authorization
with the [CI/CD variable](https://docs.gitlab.com/ci/variables/) `DOCKER_AUTH_CONFIG`. You can set the variable in one of the following:

- The CI/CD settings of the project as the [`file` type](https://docs.gitlab.com/ci/variables/#use-file-type-cicd-variables)
- The `config.toml` file

Using private registries with the `if-not-present` pull policy may introduce
[security implications](../security/_index.md#usage-of-private-docker-images-with-if-not-present-pull-policy).
For more information about how pull policies work, see [Configure how runners pull images](../executors/docker.md#configure-how-runners-pull-images).

For more information about using private container registries, see:

- [Access an image from a private container registry](https://docs.gitlab.com/ci/docker/using_docker_images/#access-an-image-from-a-private-container-registry)
- [`.gitlab-ci.yml` keyword reference](https://docs.gitlab.com/ci/yaml/#image)

The steps performed by the runner can be summed up as:

1. The registry name is found from the image name.
1. If the value is not empty, the executor searches for the authentication
   configuration for this registry.
1. Finally, if an authentication corresponding to the specified registry is
   found, subsequent pulls makes use of it.

#### Support for GitLab integrated registry

GitLab sends credentials for its integrated
registry along with the job's data. These credentials are automatically
added to the registry's authorization parameters list.

After this step, authorization against the registry proceeds similarly to
configuration added with the `DOCKER_AUTH_CONFIG` variable.

In your jobs, you can use any image from your GitLab integrated
registry, even if the image is private or protected. For information on the images jobs have access to, read the
[CI/CD job token documentation](https://docs.gitlab.com/ci/jobs/ci_job_token/) documentation.

#### Precedence of Docker authorization resolving

As described earlier, GitLab Runner can authorize Docker against a registry by
using credentials sent in different way. To find a proper registry, the following
precedence is taken into account:

1. Credentials configured with `DOCKER_AUTH_CONFIG`.
1. Credentials configured locally on the GitLab Runner host with `~/.docker/config.json`
   or `~/.dockercfg` files (for example, by running `docker login` on the host).
1. Credentials sent by default with a job's payload (for example, credentials for the integrated
   registry described earlier).

The first credentials found for the registry are used. So for example,
if you add credentials for the integrated registry with the
`DOCKER_AUTH_CONFIG` variable, then the default credentials are overridden.

## The `[runners.parallels]` section

The following parameters are for Parallels.

| Parameter           | Description |
|---------------------|-------------|
| `base_name`         | Name of Parallels VM that is cloned. |
| `template_name`     | Custom name of Parallels VM linked template. Optional. |
| `disable_snapshots` | If disabled, the VMs are destroyed when the jobs are done. |
| `allowed_images`    | List of allowed `image`/`base_name` values, represented as regular expressions. See the [Overriding the base VM image](#overriding-the-base-vm-image) section for more details. |

Example:

```toml
[runners.parallels]
  base_name = "my-parallels-image"
  template_name = ""
  disable_snapshots = false
```

## The `[runners.virtualbox]` section

The following parameters are for VirtualBox. This executor relies on the
`vboxmanage` executable to control VirtualBox machines, so you have to adjust
your `PATH` environment variable on Windows hosts:
`PATH=%PATH%;C:\Program Files\Oracle\VirtualBox`.

| Parameter           | Explanation |
|---------------------|-------------|
| `base_name`         | Name of the VirtualBox VM that is cloned. |
| `base_snapshot`     | Name or UUID of a specific snapshot of the VM to create a linked clone from. If this value is empty or omitted, the current snapshot is used. If no current snapshot exists, one is created. Unless `disable_snapshots` is true, in which case a full clone of the base VM is made. |
| `base_folder`       | Folder to save the new VM in. If this value is empty or omitted, the default VM folder is used. |
| `disable_snapshots` | If disabled, the VMs are destroyed when the jobs are done. |
| `allowed_images`    | List of allowed `image`/`base_name` values, represented as regular expressions. See the [Overriding the base VM image](#overriding-the-base-vm-image) section for more details. |
| `start_type`        | Graphical front-end type when starting the VM. |

Example:

```toml
[runners.virtualbox]
  base_name = "my-virtualbox-image"
  base_snapshot = "my-image-snapshot"
  disable_snapshots = false
  start_type = "headless"
```

The `start_type` parameter determines the graphical front end used when starting the virtual image. Valid values are `headless` (default), `gui` or `separate` as supported by the host and guest combination.

## Overriding the base VM image

For both the Parallels and VirtualBox executors, you can override the base VM name specified by `base_name`.
To do this, use the [image](https://docs.gitlab.com/ci/yaml/#image) parameter in the `.gitlab-ci.yml` file.

For backward compatibility, you cannot override this value by default. Only the image specified by `base_name` is allowed.

To allow users to select a VM image by using the `.gitlab-ci.yml` [image](https://docs.gitlab.com/ci/yaml/#image) parameter:

```toml
[runners.virtualbox]
  ...
  allowed_images = [".*"]
```

In the example, any existing VM image can be used.

The `allowed_images` parameter is a list of regular expressions. Configuration can be as precise as required.
For instance, if you want to allow only certain VM images, you can use regex like:

```toml
[runners.virtualbox]
  ...
  allowed_images = ["^allowed_vm[1-2]$"]
```

In this example, only `allowed_vm1` and `allowed_vm2` are allowed. Any other attempts result in an error.

## The `[runners.ssh]` section

The following parameters define the SSH connection.

| Parameter                          | Description |
|------------------------------------|-------------|
| `host`                             | Where to connect. |
| `port`                             | Port. Default is `22`. |
| `user`                             | Username.   |
| `password`                         | Password.   |
| `identity_file`                    | File path to SSH private key (`id_rsa`, `id_dsa`, or `id_edcsa`). The file must be stored unencrypted. |
| `disable_strict_host_key_checking` | This value determines if the runner should use strict host key checking. Default is `true`. In GitLab 15.0, the default value, or the value if it's not specified, is `false`. |

Example:

```toml
[runners.ssh]
  host = "my-production-server"
  port = "22"
  user = "root"
  password = "production-server-password"
  identity_file = ""
```

## The `[runners.machine]` section

The following parameters define the Docker Machine-based autoscaling feature. For more information, see [Docker Machine Executor autoscale configuration](autoscale.md).

| Parameter                         | Description |
|-----------------------------------|-------------|
| `MaxGrowthRate`                   | The maximum number of machines that can be added to the runner in parallel. Default is `0` (no limit). |
| `IdleCount`                       | Number of machines that need to be created and waiting in _Idle_ state. |
| `IdleScaleFactor`                 | The number of _Idle_ machines as a factor of the number of machines in use. Must be in float number format. See [the autoscale documentation](autoscale.md#the-idlescalefactor-strategy) for more details. Defaults to `0.0`. |
| `IdleCountMin`                    | Minimal number of machines that need to be created and waiting in _Idle_ state when the `IdleScaleFactor` is in use. Default is 1. |
| `IdleTime`                        | Time (in seconds) for machine to be in _Idle_ state before it is removed. |
| `[[runners.machine.autoscaling]]` | Multiple sections, each containing overrides for autoscaling configuration. The last section with an expression that matches the current time is selected. |
| `OffPeakPeriods`                  | Deprecated: Time periods when the scheduler is in the OffPeak mode. An array of cron-style patterns (described [below](#periods-syntax)). |
| `OffPeakTimezone`                 | Deprecated: Time zone for the times given in OffPeakPeriods. A time zone string like `Europe/Berlin`. Defaults to the locale system setting of the host if omitted or empty. GitLab Runner attempts to locate the time zone database in the directory or uncompressed zip file named by the `ZONEINFO` environment variable, then looks in known installation locations on Unix systems, and finally looks in `$GOROOT/lib/time/zoneinfo.zip`. |
| `OffPeakIdleCount`                | Deprecated: Like `IdleCount`, but for _Off Peak_ time periods. |
| `OffPeakIdleTime`                 | Deprecated: Like `IdleTime`, but for _Off Peak_ time periods. |
| `MaxBuilds`                       | Maximum job (build) count before machine is removed. |
| `MachineName`                     | Name of the machine. It **must** contain `%s`, which is replaced with a unique machine identifier. |
| `MachineDriver`                   | Docker Machine `driver`. View details in the [Cloud Providers Section in the Docker Machine configuration](autoscale.md#supported-cloud-providers). |
| `MachineOptions`                  | Docker Machine options for the MachineDriver. For more information, see [Supported Cloud Providers](autoscale.md#supported-cloud-providers). For more information about all options for AWS, see the [AWS](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/aws.md) and [GCP](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/gce.md) projects in the Docker Machine repository. |

### The `[[runners.machine.autoscaling]]` sections

The following parameters define the configuration available when using the [Instance](../executors/instance.md) or [Docker Autoscaler](../executors/docker_autoscaler.md#example-aws-autoscaling-for-1-job-per-instance) executor.

| Parameter         | Description |
|-------------------|-------------|
| `Periods`         | Time periods during which this schedule is active. An array of cron-style patterns (described [below](#periods-syntax)). |
| `IdleCount`       | Number of machines that need to be created and waiting in _Idle_ state. |
| `IdleScaleFactor` | (Experiment) The number of _Idle_ machines as a factor of the number of machines in use. Must be in float number format. See [the autoscale documentation](autoscale.md#the-idlescalefactor-strategy) for more details. Defaults to `0.0`. |
| `IdleCountMin`    | Minimal number of machines that need to be created and waiting in _Idle_ state when the `IdleScaleFactor` is in use. Default is 1. |
| `IdleTime`        | Time (in seconds) for a machine to be in _Idle_ state before it is removed. |
| `Timezone`        | Time zone for the times given in `Periods`. A time zone string like `Europe/Berlin`. Defaults to the locale system setting of the host if omitted or empty. GitLab Runner attempts to locate the time zone database in the directory or uncompressed zip file named by the `ZONEINFO` environment variable, then looks in known installation locations on Unix systems, and finally looks in `$GOROOT/lib/time/zoneinfo.zip`. |

Example:

```toml
[runners.machine]
  IdleCount = 5
  IdleTime = 600
  MaxBuilds = 100
  MachineName = "auto-scale-%s"
  MachineDriver = "google" # Refer to Docker Machine docs on how to authenticate: https://docs.docker.com/machine/drivers/gce/#credentials
  MachineOptions = [
      # Additional machine options can be added using the Google Compute Engine driver.
      # If you experience problems with an unreachable host (ex. "Waiting for SSH"),
      # you should remove optional parameters to help with debugging.
      # https://docs.docker.com/machine/drivers/gce/
      "google-project=GOOGLE-PROJECT-ID",
      "google-zone=GOOGLE-ZONE", # e.g. 'us-central1-a', full list in https://cloud.google.com/compute/docs/regions-zones/
  ]
  [[runners.machine.autoscaling]]
    Periods = ["* * 9-17 * * mon-fri *"]
    IdleCount = 50
    IdleCountMin = 5
    IdleScaleFactor = 1.5 # Means that current number of Idle machines will be 1.5*in-use machines,
                          # no more than 50 (the value of IdleCount) and no less than 5 (the value of IdleCountMin)
    IdleTime = 3600
    Timezone = "UTC"
  [[runners.machine.autoscaling]]
    Periods = ["* * * * * sat,sun *"]
    IdleCount = 5
    IdleTime = 60
    Timezone = "UTC"
```

### Periods syntax

The `Periods` setting contains an array of string patterns of
time periods represented in a cron-style format. The line contains
following fields:

```plaintext
[second] [minute] [hour] [day of month] [month] [day of week] [year]
```

Like in the standard cron configuration file, the fields can contain single
values, ranges, lists, and asterisks. View [a detailed description of the syntax](https://github.com/gorhill/cronexpr#implementation).

## The `[runners.instance]` section

| Parameter        | Type   | Description |
|------------------|--------|-------------|
| `allowed_images` | string | When VM Isolation is enabled, `allowed_images` controls which images a job is allowed to specify. |

## The `[runners.autoscaler]` section

{{< history >}}

- Introduced in GitLab Runner v15.10.0.

{{< /history >}}

The following parameters configure the autoscaler feature. You can only use these parameters with the
[Instance](../executors/instance.md) and [Docker Autoscaler](../executors/docker_autoscaler.md) executors.

| Parameter                        | Description |
|----------------------------------|-------------|
| `capacity_per_instance`          | The number of jobs that can be executed concurrently by a single instance. |
| `max_use_count`                  | The maximum number of times an instance can be used before it is scheduled for removal. |
| `max_instances`                  | The maximum number of instances that are allowed, this is regardless of the instance state (pending, running, deleting). Default: `0` (unlimited). |
| `plugin`                         | The [fleeting](https://gitlab.com/gitlab-org/fleeting/fleeting) plugin to use. For more information about how to install and reference a plugin, see [Install the fleeting plugin](../fleet_scaling/fleeting.md#install-a-fleeting-plugin). |
| `delete_instances_on_shutdown`   | Specifies if all provision instances are deleted when GitLab Runner is shutting down. Default: `false`. Introduced in [GitLab Runner 15.11](https://gitlab.com/gitlab-org/fleeting/taskscaler/-/merge_requests/24) |
| `instance_ready_command`         | Executes this command on each instance provisioned by the autoscaler to ensure that it is ready for use. A failure results in the instance being removed. Introduced in [GitLab Runner 16.11](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/37473). |
| `instance_acquire_timeout`       | The maximum duration the runner waits to acquire an instance before it times out. Default: `15m` (15 minutes). You can adjust this value to better suit your environment. Introduced in [GitLab Runner 18.1](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5563). |
| `update_interval`                | The interval to check with the fleeting plugin for instance updates. Default: `1m` (1 minute). Introduced in [GitLab Runner 16.11](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4722). |
| `update_interval_when_expecting` | The interval to check with the fleeting plugin for instance updates when expecting a state change. For example, when an instance has provisioned an instance and the runner is waiting to transition from `pending` to `running`. Default: `2s` (2 seconds). Introduced in [GitLab Runner 16.11](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4722). |
| `deletion_retry_interval` | The interval that the fleeting plugin waits before it retries deletion when a previous deletion attempt had no effect. Default: `1m` (1 minute). Introduced in [GitLab Runner 18.4](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5777). |
| `shutdown_deletion_interval`| The interval used by the fleeting plugin between removing instances and checking their status during shutdown. Default: `10s` (10 seconds). Introduced in [GitLab Runner 18.4](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5777). |
| `shutdown_deletion_retries` | The maximum number of attempts made by the fleeting plugin to ensure that the instances finish deletion before shutdown. Default: `3`. Introduced in [GitLab Runner 18.4](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5777). |
| `failure_threshold` | The maximum number of consecutive health failures before the fleeting plugin replaces an instance. See also the heartbeat feature. Default: `3`. Introduced in [GitLab Runner 18.4](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5777). |
| `log_internal_ip`                | Specifies whether the CI/CD output logs the internal IP address of the VM. Default: `false`. Introduced in [GitLab Runner 18.1](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5519). |
| `log_external_ip`                | Specifies whether the CI/CD output logs the external IP address of the VM. Default: `false`. Introduced in [GitLab Runner 18.1](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5519). |

{{< alert type="note" >}}

If the `instance_ready_command` frequently fails with idle scale rules, instances might be removed and created
faster than the runner accepts jobs. To support scale throttling, an exponential backoff was added in
[GitLab 17.0](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/37497).

{{< /alert >}}

{{< alert type="note" >}}

Autoscaler configuration options don't reload with configuration changes. However, in
GitLab 17.5.0 or later, `[[runners.autoscaler.policy]]` entries reload when configurations change.

{{< /alert >}}

## The `[runners.autoscaler.plugin_config]` section

This hash table is re-encoded to JSON and passed directly to the configured plugin.

[fleeting](https://gitlab.com/gitlab-org/fleeting/fleeting) plugins typically have accompanying documentation on
the supported configuration.

## The `[runners.autoscaler.scale_throttle]` section

{{< history >}}

- Introduced in GitLab Runner v17.0.0.

{{< /history >}}

| Parameter | Description |
|-----------|-------------|
| `limit`   | The rate limit of new instances per second that can provisioned. `-1` is infinite. The default (`0`), sets the limit to `100`. |
| `burst`   | The burst limit of new instances. Defaults to `max_instances` or `limit` when `max_instances` is not set. If `limit` is infinite, `burst` is ignored. |

### Relationship between `limit` and `burst`

The scale throttle uses a token quota system to create instances. This system is defined by two values:

- `burst`: The maximum size of the quota.
- `limit`: The rate at which the quota refreshes per second.

The number of instances you can create at once depends on your remaining quota.
If you have sufficient quota, you can create instances up to that amount.
If the quota is depleted, you can create `limit` instances per second.
When instance creation stops, the quota increases by `limit` per second
until it reaches the `burst` value.

For example, if `limit` is `1` and `burst` is `60`:

- You can create 60 instances instantly, but you're throttled.
- If you wait 60 seconds, you can instantly create another 60 instances.
- If you do not wait, you can create 1 instance every second.

## The `[runners.autoscaler.connector_config]` section

[fleeting](https://gitlab.com/gitlab-org/fleeting/fleeting) plugins typically have accompanying documentation on
the supported connection options.

Plugins automatically update the connector configuration. You can use the `[runners.autoscaler.connector_config]`
to override automatic update of the connector configuration, or to fill in
the empty values that the plugin cannot determine.

| Parameter                | Description |
|--------------------------|-------------|
| `os`                     | The operating system of the instance. |
| `arch`                   | The architecture of the instance. |
| `protocol`               | `ssh`, `winrm`, or `winrm+https`. `winrm` is used by default if Windows is detected. |
| `protocol_port`          | The port used to establish connection based on the specified protocol. Defaults to `ssh:22`, `winrm+http:5985`, `winrm+https:5986`. |
| `username`               | The username used to connect with. |
| `password`               | The password used to connect with. |
| `key_path`               | The TLS key used to connect with or dynamically provision credentials with. |
| `use_static_credentials` | Disabled automatic credential provisioning. Default: `false`. |
| `keepalive`              | The connection keepalive duration. |
| `timeout`                | The connection timeout duration. |
| `use_external_addr`      | Whether to use the external address provided by the plugin. If the plugin only returns an internal address, it is used regardless of this setting. Default: `false`. |

## The `[runners.autoscaler.state_storage]` section

{{< details >}}

- Status: Beta

{{< /details >}}

{{< history >}}

- Introduced in GitLab Runner 17.5.0.

{{< /history >}}

If GitLab Runner starts when state storage is disabled (default), the existing fleeting instances
are removed immediately for safety reasons. For example, when `max_use_count` is set to `1`,
we might inadvertently assign a job to an instance that's already been used if we don't
know its usage status.

Enabling the state storage feature allows an instance's state to persist on the local disk.
In this case, if an instance exists when GitLab Runner starts, it is not deleted. Its
cached connection details, use count, and other configurations are restored.

Consider the following information when enabling the state storage feature:

- The authentication details for an instance (username, password, keys)
  remain in the disk.
- If an instance is restored when it is actively running a job, GitLab Runner removes it by
  default. This behavior ensures safety, as GitLab Runner cannot resume jobs. To keep the
  instance, set `keep_instance_with_acquisitions` to `true`.

  Setting `keep_instance_with_acquisitions` to `true` helps when you're not concerned about ongoing jobs
  on the instance. You can also use the `instance_ready_command`
  configuration option to clean the environment to keep the instance. This might involve stopping all
  executing commands or forcefully removing Docker containers.

| Parameter                         | Description |
|-----------------------------------|-------------|
| `enabled`                         | Whether state storage is enabled. Default: `false`. |
| `dir`                             | The state store directory. Each runner configuration entry has a subdirectory here. Default: `.taskscaler` in the GitLab Runner configuration file directory. |
| `keep_instance_with_acquisitions` | Whether instances with active jobs are removed. Default: `false`. |

## The `[[runners.autoscaler.policy]]` sections

**Note** - `idle_count` in this context refers to the number of jobs, not the number of autoscaled machines as in the legacy autoscaling method.

| Parameter            | Description |
|----------------------|-------------|
| `periods`            | An array of unix-cron formatted strings to denote the period this policy is enabled for. Default: `* * * * *` |
| `timezone`           | The time zone used when evaluating the unix-cron period. Default: The system's local time zone. |
| `idle_count`         | The target idle capacity we want to be immediately available for jobs. |
| `idle_time`          | The amount of time that an instance can be idle before it is terminated. |
| `scale_factor`       | The target idle capacity we want to be immediately available for jobs, on top of the `idle_count`, as a factor of the current in use capacity. Defaults to `0.0`. |
| `scale_factor_limit` | The maximum capacity the `scale_factor` calculation can yield. |
| `preemptive_mode`    | With preemptive mode turned on, jobs are requested only when an instance is confirmed to be available. This action allows jobs to start almost immediately without provisioning delays. When preemptive mode is turned off, jobs are requested first, and then the system attempts to find or provision the necessary capacity. |

To decide whether to remove an idle instance, the taskscaler compares `idle_time` against the instance's idle duration.
The idle period of each instance is calculated from the time the instance:

- Last completed a job (if the instance is previously used).
- Is provisioned (if never used).

This check occurs during scaling events. Instances that exceed the configured `idle_time` are removed, unless needed to maintain the required `idle_count` job capacity.

When `scale_factor` is set, `idle_count` becomes the minimum `idle` capacity and the `scaler_factor_limit` the maximum `idle` capacity.

You can define multiple policies. The last matching policy is the one used.

In the following example, the idle count `1` is used between 08:00 and 15:59, Monday through Friday. Otherwise, the idle count is 0.

```toml
[[runners.autoscaler.policy]]
  idle_count        = 0
  idle_time         = "0s"
  periods           = ["* * * * *"]

[[runners.autoscaler.policy]]
  idle_count        = 1
  idle_time         = "30m0s"
  periods           = ["* 8-15 * * mon-fri"]
```

### Periods syntax

The `periods` setting contains an array of unix-cron formatted strings to denote the period a policy is enabled for. The
cron format consists of 5 fields:

```plaintext
 ┌────────── minute (0 - 59)
 │ ┌──────── hour (0 - 23)
 │ │ ┌────── day of month (1 - 31)
 │ │ │ ┌──── month (1 - 12)
 │ │ │ │ ┌── day of week (1 - 7 or MON-SUN, 0 is an alias for Sunday)
 * * * * *
```

- `-` can be used between two numbers to specify a range.
- `*` can be used to represent the whole range of valid values for that field.
- `/` followed by a number or can be used after a range to skip that number through the range. For example, 0-12/2 for the hour field would activate the period every 2 hours between the hours of 00:00 and 00:12.
- `,` can be used to separate a list of valid numbers or ranges for the field. For example, `1,2,6-9`.

It's worth keeping in mind that this cron job represents a range in time. For example:

| Period               | Affect |
|----------------------|--------|
| `1 * * * * *`        | Rule enabled for the period of 1 minute every hour (unlikely to be very effective) |
| `* 0-12 * * *`       | Rule enabled for the period of 12 hours at the beginning of each day |
| `0-30 13,16 * * SUN` | Rule enabled for the period of each Sunday for 30 minutes at 1pm and 30 minutes at 4pm. |

## The `[runners.autoscaler.vm_isolation]` section

VM Isolation uses [`nesting`](../executors/instance.md#nested-virtualization), which is only supported on macOS.

| Parameter        | Description |
|------------------|-------------|
| `enabled`        | Specifies if VM Isolation is enabled or not. Default: `false`. |
| `nesting_host`   | The `nesting` daemon host. |
| `nesting_config` | The `nesting` configuration, which is serialized to JSON and sent to the `nesting` daemon. |
| `image`          | The default image used by the nesting daemon if no job image is specified. |

## The `[runners.autoscaler.vm_isolation.connector_config]` section

The parameters for the `[runners.autoscaler.vm_isolation.connector_config]` section are identical to the
[`[runners.autoscaler.connector_config]`](#the-runnersautoscalerconnector_config-section) section,
but are used to connect to the `nesting` provisioned virtual machine, rather than the autoscaled instance.

## The `[runners.custom]` section

The following parameters define configuration for the [custom executor](../executors/custom.md).

| Parameter               | Type         | Description |
|-------------------------|--------------|-------------|
| `config_exec`           | string       | Path to an executable, so a user can override some configuration settings before the job starts. These values override the ones set in the [`[[runners]]`](#the-runners-section) section. [The custom executor documentation](../executors/custom.md#config) has the full list. |
| `config_args`           | string array | First set of arguments passed to the `config_exec` executable. |
| `config_exec_timeout`   | integer      | Timeout, in seconds, for `config_exec` to finish execution. Default is 3600 seconds (1 hour). |
| `prepare_exec`          | string       | Path to an executable to prepare the environment. |
| `prepare_args`          | string array | First set of arguments passed to the `prepare_exec` executable. |
| `prepare_exec_timeout`  | integer      | Timeout, in seconds, for `prepare_exec` to finish execution. Default is 3600 seconds (1 hour). |
| `run_exec`              | string       | **Required**. Path to an executable to run scripts in the environments. For example, the clone and build script. |
| `run_args`              | string array | First set of arguments passed to the `run_exec` executable. |
| `cleanup_exec`          | string       | Path to an executable to clean up the environment. |
| `cleanup_args`          | string array | First set of arguments passed to the `cleanup_exec` executable. |
| `cleanup_exec_timeout`  | integer      | Timeout, in seconds, for `cleanup_exec` to finish execution. Default is 3600 seconds (1 hour). |
| `graceful_kill_timeout` | integer      | Time to wait, in seconds, for `prepare_exec` and `cleanup_exec` if they are terminated (for example, during job cancellation). After this timeout, the process is killed. Default is 600 seconds (10 minutes). |
| `force_kill_timeout`    | integer      | Time to wait, in seconds, after the kill signal is sent to the script. Default is 600 seconds (10 minutes). |

## The `[runners.cache]` section

The following parameters define the distributed cache feature. View details
in the [runner autoscale documentation](autoscale.md#distributed-runners-caching).

| Parameter                | Type    | Description |
|--------------------------|---------|-------------|
| `Type`                   | string  | One of: `s3`, `gcs`, `azure`. |
| `Path`                   | string  | Name of the path to prepend to the cache URL. |
| `Shared`                 | boolean | Enables cache sharing between runners. Default is `false`. |
| `MaxUploadedArchiveSize` | int64   | Limit, in bytes, of the cache archive being uploaded to cloud storage. A malicious actor can work around this limit so the GCS adapter enforces it through the X-Goog-Content-Length-Range header in the signed URL. You should also set the limit on your cloud storage provider. |

You can use the following environment variables to configure cache compression:

| Variable                   | Description                           | Default   | Values                                          |
|----------------------------|---------------------------------------|-----------|-------------------------------------------------|
| `CACHE_COMPRESSION_FORMAT` | Compression format for cache archives | `zip`     | `zip`, `tarzstd`                                |
| `CACHE_COMPRESSION_LEVEL`  | Compression level for cache archives  | `default` | `fastest`, `fast`, `default`, `slow`, `slowest` |

The `tarzstd` format uses TAR with Zstandard compression, which provides better compression ratios than `zip`.
The compression levels range from `fastest` (minimal compression for maximum speed) to `slowest` (maximum compression for smallest file size).
The `default` level provides a balanced trade-off between compression ratio and speed.

Example:

```yaml
job:
  variables:
    CACHE_COMPRESSION_FORMAT: tarzstd
    CACHE_COMPRESSION_LEVEL: fast
```

The cache mechanism uses pre-signed URLs to upload and download cache. URLs are signed by GitLab Runner on its own instance.
It does not matter if the job's script (including the cache upload/download script) are executed on local or external
machines. For example, `shell` or `docker` executors run their scripts on the same
machine where the GitLab Runner process is running. At the same time, `virtualbox` or `docker+machine`
connects to a separate VM to execute the script. This process is for security reasons:
minimizing the possibility of leaking the cache adapter's credentials.

If the [S3 cache adapter](#the-runnerscaches3-section) is configured to use
an IAM instance profile, the adapter uses the profile attached to the GitLab Runner machine.
Similarly for [GCS cache adapter](#the-runnerscachegcs-section), if configured to
use the `CredentialsFile`. The file needs to be present on the GitLab Runner machine.

This table lists `config.toml`, CLI options, and environment variables
for `register`. When you define these environment variables, the values
are saved in `config.toml` after you register a new GitLab Runner.

If you want to omit S3 credentials from `config.toml` and load static
credentials from the environment, you can define `AWS_ACCESS_KEY_ID` and
`AWS_SECRET_ACCESS_KEY`. For more information, see
[AWS SDK default credential chain section](#aws-sdk-default-credential-chain).

| Setting                        | TOML field                                        | CLI option for `register`                  | Environment variable for `register` |
|--------------------------------|---------------------------------------------------|--------------------------------------------|-------------------------------------|
| `Type`                         | `[runners.cache] -> Type`                         | `--cache-type`                             | `$CACHE_TYPE`                       |
| `Path`                         | `[runners.cache] -> Path`                         | `--cache-path`                             | `$CACHE_PATH`                       |
| `Shared`                       | `[runners.cache] -> Shared`                       | `--cache-shared`                           | `$CACHE_SHARED`                     |
| `S3.ServerAddress`             | `[runners.cache.s3] -> ServerAddress`             | `--cache-s3-server-address`                | `$CACHE_S3_SERVER_ADDRESS`          |
| `S3.AccessKey`                 | `[runners.cache.s3] -> AccessKey`                 | `--cache-s3-access-key`                    | `$CACHE_S3_ACCESS_KEY`              |
| `S3.SecretKey`                 | `[runners.cache.s3] -> SecretKey`                 | `--cache-s3-secret-key`                    | `$CACHE_S3_SECRET_KEY`              |
| `S3.SessionToken`              | `[runners.cache.s3] -> SessionToken`              | `--cache-s3-session-token`                 | `$CACHE_S3_SESSION_TOKEN`           |
| `S3.BucketName`                | `[runners.cache.s3] -> BucketName`                | `--cache-s3-bucket-name`                   | `$CACHE_S3_BUCKET_NAME`             |
| `S3.BucketLocation`            | `[runners.cache.s3] -> BucketLocation`            | `--cache-s3-bucket-location`               | `$CACHE_S3_BUCKET_LOCATION`         |
| `S3.Insecure`                  | `[runners.cache.s3] -> Insecure`                  | `--cache-s3-insecure`                      | `$CACHE_S3_INSECURE`                |
| `S3.AuthenticationType`        | `[runners.cache.s3] -> AuthenticationType`        | `--cache-s3-authentication_type`           | `$CACHE_S3_AUTHENTICATION_TYPE`     |
| `S3.ServerSideEncryption`      | `[runners.cache.s3] -> ServerSideEncryption`      | `--cache-s3-server-side-encryption`        | `$CACHE_S3_SERVER_SIDE_ENCRYPTION`  |
| `S3.ServerSideEncryptionKeyID` | `[runners.cache.s3] -> ServerSideEncryptionKeyID` | `--cache-s3-server-side-encryption-key-id` | `$CACHE_S3_SERVER_SIDE_ENCRYPTION_KEY_ID` |
| `S3.DualStack`                 | `[runners.cache.s3] -> DualStack`                 | `--cache-s3-dual-stack`                    | `$CACHE_S3_DUAL_STACK`              |
| `S3.Accelerate`                | `[runners.cache.s3] -> Accelerate`                | `--cache-s3-accelerate`                    | `$CACHE_S3_ACCELERATE`              |
| `S3.PathStyle`                 | `[runners.cache.s3] -> PathStyle`                 | `--cache-s3-path-style`                    | `$CACHE_S3_PATH_STYLE`              |
| `S3.RoleARN`                   | `[runners.cache.s3] -> RoleARN`                   | `--cache-s3-role-arn`                      | `$CACHE_S3_ROLE_ARN`                |
| `S3.UploadRoleARN`             | `[runners.cache.s3] -> UploadRoleARN`             | `--cache-s3-upload-role-arn`               | `$CACHE_S3_UPLOAD_ROLE_ARN`         |
| `GCS.AccessID`                 | `[runners.cache.gcs] -> AccessID`                 | `--cache-gcs-access-id`                    | `$CACHE_GCS_ACCESS_ID`              |
| `GCS.PrivateKey`               | `[runners.cache.gcs] -> PrivateKey`               | `--cache-gcs-private-key`                  | `$CACHE_GCS_PRIVATE_KEY`            |
| `GCS.CredentialsFile`          | `[runners.cache.gcs] -> CredentialsFile`          | `--cache-gcs-credentials-file`             | `$GOOGLE_APPLICATION_CREDENTIALS`   |
| `GCS.BucketName`               | `[runners.cache.gcs] -> BucketName`               | `--cache-gcs-bucket-name`                  | `$CACHE_GCS_BUCKET_NAME`            |
| `Azure.AccountName`            | `[runners.cache.azure] -> AccountName`            | `--cache-azure-account-name`               | `$CACHE_AZURE_ACCOUNT_NAME`         |
| `Azure.AccountKey`             | `[runners.cache.azure] -> AccountKey`             | `--cache-azure-account-key`                | `$CACHE_AZURE_ACCOUNT_KEY`          |
| `Azure.ContainerName`          | `[runners.cache.azure] -> ContainerName`          | `--cache-azure-container-name`             | `$CACHE_AZURE_CONTAINER_NAME`       |
| `Azure.StorageDomain`          | `[runners.cache.azure] -> StorageDomain`          | `--cache-azure-storage-domain`             | `$CACHE_AZURE_STORAGE_DOMAIN`       |

### Cache key handling

{{< history >}}

- [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5751) in GitLab Runner v18.4.0.

{{< /history >}}

In GitLab Runner 18.4.0 and later, you can hash cache keys with the
`FF_HASH_CACHE_KEYS` [feature flag](feature-flags.md).

When `FF_HASH_CACHE_KEYS` is turned off (default), GitLab Runner sanitizes the
cache key before using it to build the path for both the local cache file and
the object in the storage bucket. If the sanitization changes the cache key,
GitLab Runner logs this change. If GitLab Runner cannot sanitize the cache key,
it also logs this, and does not use this specific cache.

When you turn on this feature flag, GitLab Runner hashes the cache key before using
it to build the path for the local cache artifact and the object in the remote storage
bucket. GitLab Runner does not sanitize the cache key. To help you understand which
cache key created a specific cache artifact, GitLab Runner attaches metadata to it:

- For local cache artifacts, GitLab Runner places a `metadata.json` file next to
  the cache artifact `cache.zip`, with the following content:

  ```json
  {"cachekey": "the human readable cache key"}
  ```

- For cache artifacts on distributed caches, GitLab Runner attaches the metadata directly to the storage object blob,
  with the key `cachekey`. You can query it using the cloud provider's mechanisms. For an example, see the
  [user-defined object metadata](https://docs.aws.amazon.com/AmazonS3/latest/userguide/UsingMetadata.html#UserMetadata)
  for AWS S3.

{{< alert type="warning" >}}

When you change `FF_HASH_CACHE_KEYS`, GitLab Runner ignores existing cache artifacts
because hashing the cache key changes the cache artifact's name and location.
This change applies in both directions, from `FF_HASH_CACHE_KEYS=true` to
`FF_HASH_CACHE_KEYS=false` and vice versa.

If you run multiple runners that share a distributed cache but have different
settings for `FF_HASH_CACHE_KEYS`, they do not share cache artifacts.

Therefore, best practice is:

- Keep `FF_HASH_CACHE_KEYS` in sync across runners which share distributed
  caches.

- Expect cache misses, cache artifacts rebuild, and longer first job runs after
  you change `FF_HASH_CACHE_KEYS`.

{{< /alert >}}

{{< alert type="warning" >}}

If you turn on `FF_HASH_CACHE_KEYS` but run an older version of the helper binary
(for example, because you pinned the helper image to an older version), hashing the
cache key and uploading or downloading caches still works. However, GitLab Runner
does not maintain the metadata of cache artifacts.

{{< /alert >}}

### The `[runners.cache.s3]` section

The following parameters define S3 storage for cache.

| Parameter                   | Type    | Description |
|-----------------------------|---------|-------------|
| `ServerAddress`             | string  | A `host:port` for the S3-compatible server. If you are using a server other than AWS, consult the storage product documentation to determine the correct address. For DigitalOcean, the address must be in the format `spacename.region.digitaloceanspaces.com`. |
| `AccessKey`                 | string  | The access key specified for your S3 instance. |
| `SecretKey`                 | string  | The secret key specified for your S3 instance. |
| `SessionToken`              | string  | The session token specified for your S3 instance when temporary credentials are used. |
| `BucketName`                | string  | Name of the storage bucket where cache is stored. |
| `BucketLocation`            | string  | Name of S3 region. |
| `Insecure`                  | boolean | Set to `true` if the S3 service is available by `HTTP`. Default is `false`. |
| `AuthenticationType`        | string  | Set to `iam` or `access-key`. Default is `access-key` if `ServerAddress`, `AccessKey`, and `SecretKey` are all provided. Defaults to `iam` if `ServerAddress`, `AccessKey`, or `SecretKey` are missing. |
| `ServerSideEncryption`      | string  | The server-side encryption type to use with S3. In GitLab 15.3 and later, available types are `S3`, or `KMS`. In GitLab 17.5 and later, [`DSSE-KMS`](https://docs.aws.amazon.com/AmazonS3/latest/userguide/UsingDSSEncryption.html) is supported. |
| `ServerSideEncryptionKeyID` | string  | The alias, ID, or ARN of a KMS key used for encryption when you use KMS. If you use an alias, prefix it with `alias/`. Use ARN format for cross-account scenarios. Available in GitLab 15.3 and later. |
| `DualStack`                 | boolean | Enables IPv4 and IPv6 endpoints. Default is `true`. Disable this setting if you are using AWS S3 Express. GitLab ignores this setting if you set `ServerAddress`. Available in GitLab 17.5 and later. |
| `Accelerate`                | boolean | Enables AWS S3 Transfer Acceleration. GitLab sets this to `true` automatically if `ServerAddress` is configured as an Accelerated endpoint. Available in GitLab 17.5 and later. |
| `PathStyle`                 | boolean | Enables path-style access. By default, GitLab automatically detects this setting based on the `ServerAddress` value. Available in GitLab 17.5 and later. |
| `UploadRoleARN`             | string  | Deprecated. Use `RoleARN` instead. Specifies an AWS role ARN that can be used with `AssumeRole` to generate time-limited `PutObject` S3 requests. Enables S3 multipart uploads. Available in GitLab 17.5 and later. |
| `RoleARN`                   | string  | Specifies an AWS role ARN that can be used with `AssumeRole` to generate time-limited `GetObject` and `PutObject` S3 requests. Enables S3 multipart transfers. Available in GitLab 17.8 and later. |

Example:

```toml
[runners.cache]
  Type = "s3"
  Path = "path/to/prefix"
  Shared = false
  [runners.cache.s3]
    ServerAddress = "s3.amazonaws.com"
    AccessKey = "AWS_S3_ACCESS_KEY"
    SecretKey = "AWS_S3_SECRET_KEY"
    BucketName = "runners-cache"
    BucketLocation = "eu-west-1"
    Insecure = false
    ServerSideEncryption = "KMS"
    ServerSideEncryptionKeyID = "alias/my-key"
```

## Authentication

GitLab Runner uses different authentication methods for S3 based on
your configuration.

### Static credentials

The runner uses static access key authentication when:

- `ServerAddress`, `AccessKey`, and `SecretKey` parameters are specified but `AuthenticationType` is not provided.
- `AuthenticationType = "access-key"` is explicitly set.

### AWS SDK default credential chain

The runner uses the [AWS SDK default credential chain](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html#specifying-credentials) when:

- Any of `ServerAddress`, `AccessKey`, or `SecretKey` are omitted and `AuthenticationType` is not provided.
- `AuthenticationType = "iam"` is explicitly set.

The credential chain attempts authentication in the following order:

1. Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`)
1. Shared credentials file (`~/.aws/credentials`)
1. IAM instance profile (for EC2 instances)
1. Other AWS credential sources supported by the SDK

If `RoleARN` is not specified, the default credential chain is executed
by the runner manager, which is often not necessarily on the same
machine where the build runs. For example, in an
[autoscale](autoscale.md) configuration, the job runs on a different
machine. Similarly, with the Kubernetes executor, the build pod can also
run on a different node than the runner manager. This behavior makes it possible
to grant bucket-level access only to the runner manager.

If `RoleARN` is specified, the credentials are resolved within the
execution context of the helper image. For more information, see
[RoleARN](#enable-multipart-transfers-with-rolearn).

When you use Helm charts to install GitLab Runner, and `rbac.create` is set to `true`
in the `values.yaml` file, a service account is created. The service account's annotations are retrieved from the
`rbac.serviceAccountAnnotations` section.

For runners on Amazon EKS, you can specify an IAM role to
assign to the service account. The specific annotation needed is:
`eks.amazonaws.com/role-arn: arn:aws:iam::<ACCOUNT_ID>:role/<IAM_ROLE_NAME>`.

The IAM policy for this role must have permissions to do the following actions for the specified bucket:

- `s3:PutObject`
- `s3:GetObjectVersion`
- `s3:GetObject`
- `s3:DeleteObject`
- `s3:ListBucket`

If you use `ServerSideEncryption` of type `KMS`, this role must also have permission to do the following actions for the specified AWS KMS Key:

- `kms:Encrypt`
- `kms:Decrypt`
- `kms:ReEncrypt*`
- `kms:GenerateDataKey*`
- `kms:DescribeKey`

`ServerSideEncryption` of type `SSE-C` is not supported.
`SSE-C` requires that the headers, which contain the user-supplied key, are provided for the download request, in addition to the pre-signed URL.
This would mean passing the key material to the job, where the key can't be kept safe. This does have the potential to leak the decryption key.
A discussion about this issue is in [this merge request](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/3295).

{{< alert type="note" >}}

The maximum size of a single file that can be uploaded to AWS S3 cache is 5 GB.
A discussion about potential workarounds for this behavior is in [this issue](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/26921).

{{< /alert >}}

#### Use KMS key encryption in S3 bucket for runner cache

The `GenerateDataKey` API uses the KMS symmetric key to create a data key for client-side encryption (<https://docs.aws.amazon.com/kms/latest/APIReference/API_GenerateDataKey.html>). KMS key configuration must be as follows:

| Attribute | Description |
|-----------|-------------|
| Key Type  | Symmetric   |
| Origin    | `AWS_KMS`   |
| Key Spec  | `SYMMETRIC_DEFAULT` |
| Key Usage | Encrypt and decrypt |

The IAM policy for the role assigned to the ServiceAccount defined in `rbac.serviceAccountName` must have permissions to do the following actions for the KMS Key:

- `kms:GetPublicKey`
- `kms:Decrypt`
- `kms:Encrypt`
- `kms:DescribeKey`
- `kms:GenerateDataKey`

#### Enable multipart transfers with `RoleARN`

To limit access to the cache, the runner manager generates
timed-limited, [pre-signed URLs](https://docs.aws.amazon.com/AmazonS3/latest/userguide/using-presigned-url.html) for jobs to download from and upload to
the cache. However, AWS S3 limits a [single PUT request to 5 GB](https://docs.aws.amazon.com/AmazonS3/latest/userguide/upload-objects.html).
For files larger than 5 GB, you must use the multipart upload API.

Multipart transfers are only supported with AWS S3 and not for other S3
providers. Because the runner manager handles jobs for different
projects, the runner manager cannot pass around S3 credentials that have
bucket-wide permissions. Instead, the runner manger uses time-limited
pre-signed URLs and narrowly-scoped credentials to restrict access to one
specific object.

To use S3 multipart transfers with AWS, specify an IAM role in
`RoleARN` in the `arn:aws:iam:::<ACCOUNT ID>:<YOUR ROLE NAME>`
format. This role generates time-limited AWS credentials that are
narrowly scoped to write to a specific blob in the bucket. Ensure that
your original S3 credentials can access `AssumeRole` for the
specified `RoleARN`.

The IAM role specified in `RoleARN` must have the following
permissions:

- `s3:GetObject` access to the bucket specified in `BucketName`.
- `s3:PutObject` access to the bucket specified in `BucketName`.
- `s3:ListBucket` access to the bucket specified in `BucketName`.
- `kms:Decrypt` and `kms:GenerateDataKey` if server side encryption with KMS or DSSE-KMS is enabled.

For example, suppose you have an IAM role called `my-instance-role`
attached to an EC2 instance with the ARN `arn:aws:iam::1234567890123:role/my-instance-role`.

You can create a new role `arn:aws:iam::1234567890123:role/my-upload-role`
that only has `s3:PutObject` permissions for `BucketName`. In the AWS settings for `my-instance-role`,
the `Trust relationships` might look similar to this:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": {
                "AWS": "arn:aws:iam::1234567890123:role/my-upload-role"
            },
            "Action": "sts:AssumeRole"
        }
    ]
}
```

You can also reuse `my-instance-role` as the `RoleARN` and avoid
creating a new role. Make sure that `my-instance-role` has the
`AssumeRole` permission. For example, an IAM profile associated with an
EC2 instance might have the following `Trust relationships`:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": {
                "Service": "ec2.amazonaws.com",
                "AWS": "arn:aws:iam::1234567890123:role/my-instance-role"
            },
            "Action": "sts:AssumeRole"
        }
    ]
}
```

You can use the AWS command-line interface to verify that your instance has the
`AssumeRole` permission. For example:

```shell
aws sts assume-role --role-arn arn:aws:iam::1234567890123:role/my-upload-role --role-session-name gitlab-runner-test1
```

##### How uploads work with `RoleARN`

If `RoleARN` is present, every time the runner uploads to the cache:

1. The runner manager retrieves the original S3 credentials (specified through `AuthenticationType`, `AccessKey`, and `SecretKey`).
1. With the S3 credentials, the runner manager sends a request to the Amazon Security Token Service (STS) for [`AssumeRole`](https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRole.html) with `RoleARN`.
   The policy request looks similar to this:

   ```json
   {
       "Version": "2012-10-17",
       "Statement": [
           {
               "Effect": "Allow",
               "Action": ["s3:PutObject"],
               "Resource": "arn:aws:s3:::<YOUR-BUCKET-NAME>/<CACHE-FILENAME>"
           }
       ]
   }
   ```

1. If the request is successful, the runner manager obtains temporary AWS credentials with a restricted session.
1. The runner manager passes these credentials and URL in the `s3://<bucket name>/<filename>` format to
   the cache archiver, which then uploads the file.

#### Enable IAM roles for Kubernetes ServiceAccount resources

To use IAM roles for service accounts, an IAM OIDC provider [must exist for your cluster](https://docs.aws.amazon.com/eks/latest/userguide/enable-iam-roles-for-service-accounts.html). After an IAM OIDC provider is associated with your cluster, you can create an IAM role to associate to the service account of the runner.

1. On the **Create Role** window, under **Select type of trusted entity**, select **Web Identity**.
1. On the **Trusted Relationships tab** of the role:

   - The **Trusted entities** section must have the format:
     `arn:aws:iam::<ACCOUNT_ID>:oidc-provider/oidc.eks.<AWS_REGION>.amazonaws.com/id/<OIDC_ID>`.
     The **OIDC ID** can be found on EKS cluster's **Configuration** tab.

   - The **Condition** section must have the GitLab Runner service account
     defined in `rbac.serviceAccountName` or the default service account
     created if `rbac.create` is set to `true`:

     | Condition      | Key                                                    | Value |
     |----------------|--------------------------------------------------------|-------|
     | `StringEquals` | `oidc.eks.<AWS_REGION>.amazonaws.com/id/<OIDC_ID>:sub` | `system:serviceaccount:<GITLAB_RUNNER_NAMESPACE>:<GITLAB_RUNNER_SERVICE_ACCOUNT>` |

#### Use S3 Express One Zone buckets

{{< history >}}

- Introduced in GitLab Runner 17.5.0.

{{< /history >}}

{{< alert type="note" >}}

[S3 Express One Zone directory buckets do not work with `RoleARN`](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/38484#note_2313111840) because the runner manager cannot restrict access to one specific object.

{{< /alert >}}

1. Set up an S3 Express One Zone bucket by following the [Amazon tutorial](https://docs.aws.amazon.com/AmazonS3/latest/userguide/s3-express-getting-started.html).
1. Configure `config.toml` with `BucketName` and `BucketLocation`.
1. Set `DualStack` to `false` as S3 Express does not support dual-stack endpoints.

Example `config.toml`:

```toml
[runners.cache]
  Type = "s3"
  [runners.cache.s3]
    BucketName = "example-express--usw2-az1--x-s3"
    BucketLocation = "us-west-2"
    DualStack = false
```

### The `[runners.cache.gcs]` section

The following parameters define native support for Google Cloud Storage. For more information
about these values, see the
[Google Cloud Storage (GCS) authentication documentation](https://docs.cloud.google.com/storage/docs/authentication#service_accounts).

| Parameter         | Type   | Description |
|-------------------|--------|-------------|
| `CredentialsFile` | string | Path to the Google JSON key file. Only the `service_account` type is supported. If configured, this value takes precedence over the `AccessID` and `PrivateKey` configured directly in `config.toml`. |
| `AccessID`        | string | ID of GCP Service Account used to access the storage. |
| `PrivateKey`      | string | Private key used to sign GCS requests. |
| `BucketName`      | string | Name of the storage bucket where cache is stored. |

Examples:

**Credentials configured directly in `config.toml` file**:

```toml
[runners.cache]
  Type = "gcs"
  Path = "path/to/prefix"
  Shared = false
  [runners.cache.gcs]
    AccessID = "cache-access-account@test-project-123456.iam.gserviceaccount.com"
    PrivateKey = "-----BEGIN PRIVATE KEY-----\nXXXXXX\n-----END PRIVATE KEY-----\n"
    BucketName = "runners-cache"
```

**Credentials in JSON file downloaded from GCP**:

```toml
[runners.cache]
  Type = "gcs"
  Path = "path/to/prefix"
  Shared = false
  [runners.cache.gcs]
    CredentialsFile = "/etc/gitlab-runner/service-account.json"
    BucketName = "runners-cache"
```

**Application Default Credentials (ADC) from the metadata server in GCP**:

When you use GitLab Runner with Google Cloud ADC, you typically use the default service account. Then you don't need to supply credentials for the instance:

```toml
[runners.cache]
  Type = "gcs"
  Path = "path/to/prefix"
  Shared = false
  [runners.cache.gcs]
    BucketName = "runners-cache"
```

If you use ADC, be sure that the service account that you use has the `iam.serviceAccounts.signBlob` permission. Typically this is done by granting the [Service Account Token Creator role](https://docs.cloud.google.com/iam/docs/service-account-permissions#token-creator-role) to the service account.

#### Workload Identity Federation for GKE

Workload Identity Federation for GKE is supported with application default credentials (ADC).
If you have issues getting workload identities to work:

- Check the runner pod logs (not the build log) for the message `ERROR: generating signed URL`.
  This error might indicate a permission issue, such as:

  ```plaintext
  IAM returned 403 Forbidden: Permission 'iam.serviceAccounts.getAccessToken' denied on resource (or it may not exist).
  ```

- Try the following `curl` commands from within the runner pod:

  ```shell
  curl -H "Metadata-Flavor: Google" http://169.254.169.254/computeMetadata/v1/instance/service-accounts/default/email
  ```

   This command should return the correct Kubernetes service account. Next, try to obtain an access token:

  ```shell
  curl -H "Metadata-Flavor: Google" http://169.254.169.254/computeMetadata/v1/instance/service-accounts/default/token?scopes=https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fcloud-platform
  ```

   If the command succeeds, the result returns a JSON payload with an access token. If it fails, check the service account permissions.

### The `[runners.cache.azure]` section

The following parameters define native support for Azure Blob Storage. To learn more, view the
[Azure Blob Storage documentation](https://learn.microsoft.com/en-us/azure/storage/blobs/storage-blobs-introduction).
While S3 and GCS use the word `bucket` for a collection of objects, Azure uses the word
`container` to denote a collection of blobs.

| Parameter       | Type   | Description |
|-----------------|--------|-------------|
| `AccountName`   | string | Name of the Azure Blob Storage account used to access the storage. |
| `AccountKey`    | string | Storage account access key used to access the container. To omit `AccountKey` from the configuration, use [Azure workload or managed identities](#azure-workload-and-managed-identities). |
| `ContainerName` | string | Name of the [storage container](https://learn.microsoft.com/en-us/azure/storage/blobs/storage-blobs-introduction#containers) to save cache data in. |
| `StorageDomain` | string | Domain name [used to service Azure storage endpoints](https://learn.microsoft.com/en-us/azure/china/resources-developer-guide#check-endpoints-in-azure) (optional). Default is `blob.core.windows.net`. |

Example:

```toml
[runners.cache]
  Type = "azure"
  Path = "path/to/prefix"
  Shared = false
  [runners.cache.azure]
    AccountName = "<AZURE STORAGE ACCOUNT NAME>"
    AccountKey = "<AZURE STORAGE ACCOUNT KEY>"
    ContainerName = "runners-cache"
    StorageDomain = "blob.core.windows.net"
```

#### Azure workload and managed identities

{{< history >}}

- [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27303) in GitLab Runner v17.5.0.

{{< /history >}}

To use Azure workload or managed identities, omit `AccountKey` from the
configuration. When `AccountKey` is blank, the runner attempts to:

1. Obtain temporary credentials by using [`DefaultAzureCredential`](https://github.com/Azure/azure-sdk-for-go/blob/main/sdk/azidentity/README.md#defaultazurecredential).
1. Get a [User Delegation Key](https://learn.microsoft.com/en-us/rest/api/storageservices/get-user-delegation-key).
1. Generate a SAS token with that key to access a Storage Account blob.

Ensure that the instance has the `Storage Blob Data Contributor`
role assigned to it. If the instance does not have access
to perform the actions above, GitLab Runner reports an
`AuthorizationPermissionMismatch` error.

To use Azure workload identities, add the `service_account` associated
with the identity and the pod label `azure.workload.identity/use` in the
`runner.kubernetes` section. For example, if `service_account` is
`gitlab-runner`:

```toml
  [runners.kubernetes]
    service_account = "gitlab-runner"
    [runners.kubernetes.pod_labels]
      "azure.workload.identity/use" = "true"
```

Ensure that the `service_account` has the `azure.workload.identity/client-id` annotation associated with it:

```yaml
serviceAccount:
  annotations:
    azure.workload.identity/client-id: <YOUR CLIENT ID HERE>
```

For GitLab 17.7 and later, this configuration is sufficient to set up workload identities.

However, for GitLab Runner 17.5 and 17.6, you must also configure the runner manager with:

- The `azure.workload.identity/use` pod label
- A service account to use with the workload identity

For example, with the GitLab Runner Helm chart:

```yaml
serviceAccount:
  name: "gitlab-runner"
podLabels:
  azure.workload.identity/use: "true"
```

The label is needed because the credentials are retrieved from different sources.
For cache downloads, the credentials are retrieved from the runner manager.
For cache uploads, credentials are retrieved from the pod that runs the [helper image](#helper-image).

For more details, see [issue 38330](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/38330).

## The `[runners.kubernetes]` section

The following table lists configuration parameters available for the Kubernetes executor.
For more parameters, see the [documentation for the Kubernetes executor](../executors/kubernetes/_index.md).

| Parameter                    | Type    | Description |
|------------------------------|---------|-------------|
| `host`                       | string  | Optional. Kubernetes host URL. If not specified, the runner attempts to auto-discovery it. |
| `cert_file`                  | string  | Optional. Kubernetes auth certificate. |
| `key_file`                   | string  | Optional. Kubernetes auth private key. |
| `ca_file`                    | string  | Optional. Kubernetes auth ca certificate. |
| `image`                      | string  | Default container image to use for jobs when none is specified. |
| `allowed_images`             | array   | Wildcard list of container images that are allowed in `.gitlab-ci.yml`. If not present all images are allowed (equivalent to `["*/*:*"]`). Use with the [Docker](../executors/docker.md#restrict-docker-images-and-services) or [Kubernetes](../executors/kubernetes/_index.md#restrict-docker-images-and-services) executors. |
| `allowed_services`           | array   | Wildcard list of services that are allowed in `.gitlab-ci.yml`. If not present all images are allowed (equivalent to `["*/*:*"]`). Use with the [Docker](../executors/docker.md#restrict-docker-images-and-services) or [Kubernetes](../executors/kubernetes/_index.md#restrict-docker-images-and-services) executors. |
| `namespace`                  | string  | Namespace to run Kubernetes jobs in. |
| `privileged`                 | boolean | Run all containers with the privileged flag enabled. |
| `allow_privilege_escalation` | boolean | Optional. Runs all containers with the `allowPrivilegeEscalation` flag enabled. |
| `node_selector`              | table   | A `table` of `key=value` pairs of `string=string`. Limits the creation of pods to Kubernetes nodes that match all the `key=value` pairs. |
| `image_pull_secrets`         | array   | An array of items containing the Kubernetes `docker-registry` secret names used to authenticate container images pulling from private registries. |
| `logs_base_dir`              | string  | Base directory to be prepended to the generated path to store build logs. [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/37760) in GitLab Runner 17.2. |
| `scripts_base_dir`           | string  | Base directory to be prepended to the generated path to store build scripts. [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/37760) in GitLab Runner 17.2. |
| `service_account`            | string  | Default service account that job/executor pods use to communicate with the Kubernetes API. |

Example:

```toml
[runners.kubernetes]
  host = "https://45.67.34.123:4892"
  cert_file = "/etc/ssl/kubernetes/api.crt"
  key_file = "/etc/ssl/kubernetes/api.key"
  ca_file = "/etc/ssl/kubernetes/ca.crt"
  image = "golang:1.8"
  privileged = true
  allow_privilege_escalation = true
  image_pull_secrets = ["docker-registry-credentials", "optional-additional-credentials"]
  allowed_images = ["ruby:*", "python:*", "php:*"]
  allowed_services = ["postgres:9.4", "postgres:latest"]
  logs_base_dir = "/tmp"
  scripts_base_dir = "/tmp"
  [runners.kubernetes.node_selector]
    gitlab = "true"
```

## Helper image

When you use `docker`, `docker+machine`, or `kubernetes` executors, GitLab Runner uses a specific container
to handle Git, artifacts, and cache operations. This container is created from an image named `helper image`.

The helper image is available for amd64, arm, arm64, s390x, ppc64le, and riscv64 architectures. It contains
a `gitlab-runner-helper` binary, which is a special compilation of GitLab Runner binary. It contains only a subset
of available commands, and Git, Git LFS, and SSL certificates store.

The helper image has a few flavors: `alpine`, `alpine3.21`, `alpine-latest`, `ubi-fips` and `ubuntu`.
The `alpine` image is the default due to its small footprint.
Using `helper_image_flavor = "ubuntu"` selects the `ubuntu` flavor of the helper image.

In GitLab Runner 16.1 to 17.1, the `alpine` flavor is an alias for `alpine3.18`. In GitLab Runner 17.2 to 17.6, it's an alias for `alpine3.19`. In GitLab Runner 17.7 and later, it's an alias for `alpine3.21`.
In GitLab Runner 18.4 and later, it's an alias for `alpine-latest`.

The `alpine-latest` flavor uses `alpine:latest` as its base image, and will naturally increment versions as new upstream
versions are released.

When GitLab Runner is installed from the `DEB` or `RPM` packages, images for the supported architectures are installed on the host.
If Docker Engine can't find the specified image version, the runner automatically downloads it before running the job. Both the
`docker` and `docker+machine` executors work this way.

For the `alpine` flavors, only the default `alpine` flavor image is included in the package. All other flavors are downloaded from the registry.

The `kubernetes` executor and manual installations of GitLab Runner work differently.

- For manual installations, the `gitlab-runner-helper` binary is not included.
- For the `kubernetes` executor, the Kubernetes API doesn't allow the `gitlab-runner-helper` image to be loaded from a local archive.

In both cases, GitLab Runner [downloads the helper image](#helper-image-registry).
The GitLab Runner revision and architecture define which tag to download.

### Helper image configuration for Kubernetes on Arm

By default, the correct [helper image for your architecture](../executors/kubernetes/_index.md#operating-system-architecture-and-windows-kernel-version)
is selected. If you need to set a custom `helper_image` path to use the `arm64` helper image on `arm64` Kubernetes clusters, set the following values
in your [configuration file](../executors/kubernetes/_index.md#configuration-settings):

```toml
[runners.kubernetes]
  helper_image = "my.registry.local/gitlab/gitlab-runner-helper:arm64-v${CI_RUNNER_VERSION}"
```

### Runner images that use an old version of Alpine Linux

{{< history >}}

- [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/3122) in GitLab Runner 14.5.

{{< /history >}}

Images are built with multiple versions of Alpine Linux. You can use a newer version of Alpine, but at the same time use older versions as well.

For the helper image, change the `helper_image_flavor` or read the [Helper image](#helper-image) section.

For the GitLab Runner image, follow the same logic, where `alpine`, `alpine3.19`, `alpine3.21`, or `alpine-latest`
is used as a prefix in the image, before the version:

```shell
docker pull gitlab/gitlab-runner:alpine3.19-v16.1.0
```

### Alpine `pwsh` images

As of GitLab Runner 16.1 and later, all `alpine` helper images have a `pwsh` variant. The only exception is `alpine-latest` because the
[`powershell` Docker images](https://learn.microsoft.com/en-us/powershell/scripting/install/powershell-in-docker?view=powershell-7.4) on which the GitLab Runner helper images are based do not support `alpine:latest`.

Example:

```shell
docker pull registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:alpine3.21-x86_64-v17.7.0-pwsh
```

### Helper image registry

In GitLab 15.0 and earlier, you configure helper images to use images from Docker Hub.

In GitLab 15.1 and later, the helper image is pulled from the GitLab Container Registry on GitLab.com at `registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:x86_64-v${CI_RUNNER_VERSION}`.
GitLab Self-Managed instances also pull the helper image from the GitLab Container Registry on GitLab.com by default.
To check the status of the GitLab Container Registry on GitLab.com, see [GitLab System Status](https://status.gitlab.com/).

### Override the helper image

In some cases, you might need to override the helper image for the following reasons:

1. **Speed up jobs execution**: In environments with slower internet connection, downloading the
   same image multiple times can increase the time it takes to execute a job. Downloading the helper image from
   a local registry, where the exact copy of `registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:XYZ` is stored, can speed things up.

1. **Security concerns**: You may not want to download external dependencies that were not checked before. There
   might be a business rule to use only dependencies that were reviewed and stored in local repositories.

1. **Build environments without internet access**: If you have [Kubernetes clusters installed in an offline environment](../install/operator.md#install-gitlab-runner-operator-on-kubernetes-clusters-in-offline-environments), you can use a local image registry or package repository to pull images used in CI/CD jobs.

1. **Additional software**: You may want to install some additional software to the helper image, like
   `openssh` to support submodules accessible with `git+ssh` instead of `git+http`.

In these cases, you can configure a custom image by using the `helper_image` configuration field,
which is available for the `docker`, `docker+machine`, and `kubernetes` executors:

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    helper_image = "my.registry.local/gitlab/gitlab-runner-helper:tag"
```

The version of the helper image should be considered to be strictly coupled with the version of GitLab Runner.
One of the main reasons for providing these images is that GitLab Runner is using the
`gitlab-runner-helper` binary. This binary is compiled from part of the GitLab Runner source. This binary uses an internal
API that is expected to be the same in both binaries.

By default, GitLab Runner references a `registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:XYZ` image, where `XYZ` is based
on the GitLab Runner architecture and Git revision. You can define the
image version by using one of the
[version variables](https://gitlab.com/gitlab-org/gitlab-runner/blob/main/common/version.go#L60-61):

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    helper_image = "my.registry.local/gitlab/gitlab-runner-helper:x86_64-v${CI_RUNNER_VERSION}"
```

With this configuration, GitLab Runner instructs the executor to use the image in version `x86_64-v${CI_RUNNER_VERSION}`,
which is based on its compilation data. After updating GitLab Runner to a new version, GitLab
Runner tries to download the proper image. The image should be uploaded to the registry
before upgrading GitLab Runner, otherwise the jobs start failing with a "No such image" error.

The helper image is tagged by `$CI_RUNNER_VERSION` in addition to `$CI_RUNNER_REVISION`. Both tags are
valid and point to the same image.

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    helper_image = "my.registry.local/gitlab/gitlab-runner-helper:x86_64-v${CI_RUNNER_VERSION}"
```

#### When using PowerShell Core

An additional version of the helper image for Linux,
which contains PowerShell Core, is published with the `registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:XYZ-pwsh` tag.

## The `[runners.custom_build_dir]` section

{{< history >}}

- [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/1267) in GitLab Runner 11.10.

{{< /history >}}

This section defines [custom build directories](https://docs.gitlab.com/ci/runners/configure_runners/#custom-build-directories) parameters.

This feature, if not configured explicitly, is
enabled by default for `kubernetes`, `docker`, `docker+machine`, `docker autoscaler`, and `instance`
executors. For all other executors, it is disabled by default.

This feature requires that `GIT_CLONE_PATH` is in a path defined
in `runners.builds_dir`. To use the `builds_dir`, use the
`$CI_BUILDS_DIR` variable.

By default, this feature is enabled only for `docker` and `kubernetes` executors,
because they provide a good way to separate resources. This feature can be
explicitly enabled for any executor, but use caution when you use it
with executors that share `builds_dir` and have `concurrent > 1`.

| Parameter | Type    | Description |
|-----------|---------|-------------|
| `enabled` | boolean | Allow user to define a custom build directory for a job. |

Example:

```toml
[runners.custom_build_dir]
  enabled = true
```

### Default Build Directory

GitLab Runner clones the repository to a path that exists under a
base path better known as the _Builds Directory_. The default location
of this base directory depends on the executor. For:

- [Kubernetes](../executors/kubernetes/_index.md),
  [Docker](../executors/docker.md) and [Docker Machine](../executors/docker_machine.md) executors, it is
  `/builds` inside of the container.
- [Instance](../executors/instance.md), it is
  `~/builds` in the home directory of the user configured to handle the
  SSH or WinRM connection to the target machine.
- [Docker Autoscaler](../executors/docker_autoscaler.md), it is
  `/builds` inside of the container.
- [Shell](../executors/shell.md) executor, it is `$PWD/builds`.
- [SSH](../executors/ssh.md), [VirtualBox](../executors/virtualbox.md)
  and [Parallels](../executors/parallels.md) executors, it is
  `~/builds` in the home directory of the user configured to handle the
  SSH connection to the target machine.
- [Custom](../executors/custom.md) executors, no default is provided and
  it must be explicitly configured, otherwise, the job fails.

The used _Builds Directory_ may be defined explicitly by the user with the
[`builds_dir`](#the-runners-section)
setting.

{{< alert type="note" >}}

You can also specify
[`GIT_CLONE_PATH`](https://docs.gitlab.com/ci/runners/configure_runners/#custom-build-directories)
if you want to clone to a custom directory, and the guideline below
doesn't apply.

{{< /alert >}}

GitLab Runner uses the _Builds Directory_ for all the jobs that it
runs, but nests them using a specific pattern
`{builds_dir}/$RUNNER_TOKEN_KEY/$CONCURRENT_PROJECT_ID/$NAMESPACE/$PROJECT_NAME`.
For example: `/builds/2mn-ncv-/0/user/playground`.

GitLab Runner does not stop you from storing things inside of the
_Builds Directory_. For example, you can store tools inside of
`/builds/tools` that can be used during CI execution. We **HIGHLY**
discourage this, you should never store anything inside of the _Builds
Directory_. GitLab Runner should have total control over it and does not
provide stability in such cases. If you have dependencies that are
required for your CI, you must install them in some other
place.

## Cleaning Git configuration

{{< history >}}

- [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5438) in GitLab Runner 17.10.

{{< /history >}}

At the beginning and end of every build, GitLab Runner removes the following
files from the repository and its submodules:

- Git lock files (`{index,shallow,HEAD,config}.lock`)
- Post-checkout hooks (`hooks/post-checkout`)

If you enable `clean_git_config`, the following additional files or directories
are removed from the repository, its submodules, and the Git template directory:

- `.git/config` file
- `.git/hooks` directory

This cleanup prevents custom, ephemeral, or potentially malicious Git configuration
from caching between jobs.

Before GitLab Runner 17.10, cleanups behaved differently:

- Git lock files and Post-checkout hooks cleanup only occurred at the
  beginning of a job and not at the end.
- Other Git configurations (now controlled by `clean_git_config`) were not removed unless
  `FF_ENABLE_JOB_CLEANUP` was set. When you set this flag, only the main repository's
  `.git/config` was deleted but not submodule configurations.

The `clean_git_config` setting defaults to `true`. But, it defaults to `false` when:

- [Shell executor](../executors/shell.md) is used.
- [Git strategy](https://docs.gitlab.com/ci/runners/configure_runners/#git-strategy)
  is set to `none`.

Explicit `clean_git_config` configuration takes precedence over the default
setting.

## The `[runners.referees]` section

Use GitLab Runner referees to pass extra job monitoring data to GitLab. Referees are workers in the runner manager that query and collect additional data related to a job. The results
are uploaded to GitLab as job artifacts.

### Use the Metrics Runner referee

If the machine or container running the job exposes [Prometheus](https://prometheus.io) metrics, GitLab Runner can query the Prometheus server for the entirety of the job duration. After the metrics are received, they are uploaded as a job artifact that can be used for analysis later.

Only the [`docker-machine` executor](../executors/docker_machine.md) supports the referee.

### Configure the Metrics Runner Referee for GitLab Runner

Define `[runner.referees]` and `[runner.referees.metrics]` in your `config.toml` file in a `[[runner]]` section and add the following fields:

| Setting              | Description |
|----------------------|-------------|
| `prometheus_address` | The server that collects metrics from GitLab Runner instances. It must be accessible by the runner manager when the job finishes. |
| `query_interval`     | The frequency the Prometheus instance associated with a job is queried for time series data, defined as an interval (in seconds). |
| `queries`            | An array of [PromQL](https://prometheus.io/docs/prometheus/latest/querying/basics/) queries that are executed for each interval. |

Here is a complete configuration example for `node_exporter` metrics:

```toml
[[runners]]
  [runners.referees]
    [runners.referees.metrics]
      prometheus_address = "http://localhost:9090"
      query_interval = 10
      metric_queries = [
        "arp_entries:rate(node_arp_entries{{selector}}[{interval}])",
        "context_switches:rate(node_context_switches_total{{selector}}[{interval}])",
        "cpu_seconds:rate(node_cpu_seconds_total{{selector}}[{interval}])",
        "disk_read_bytes:rate(node_disk_read_bytes_total{{selector}}[{interval}])",
        "disk_written_bytes:rate(node_disk_written_bytes_total{{selector}}[{interval}])",
        "memory_bytes:rate(node_memory_MemTotal_bytes{{selector}}[{interval}])",
        "memory_swap_bytes:rate(node_memory_SwapTotal_bytes{{selector}}[{interval}])",
        "network_tcp_active_opens:rate(node_netstat_Tcp_ActiveOpens{{selector}}[{interval}])",
        "network_tcp_passive_opens:rate(node_netstat_Tcp_PassiveOpens{{selector}}[{interval}])",
        "network_receive_bytes:rate(node_network_receive_bytes_total{{selector}}[{interval}])",
        "network_receive_drops:rate(node_network_receive_drop_total{{selector}}[{interval}])",
        "network_receive_errors:rate(node_network_receive_errs_total{{selector}}[{interval}])",
        "network_receive_packets:rate(node_network_receive_packets_total{{selector}}[{interval}])",
        "network_transmit_bytes:rate(node_network_transmit_bytes_total{{selector}}[{interval}])",
        "network_transmit_drops:rate(node_network_transmit_drop_total{{selector}}[{interval}])",
        "network_transmit_errors:rate(node_network_transmit_errs_total{{selector}}[{interval}])",
        "network_transmit_packets:rate(node_network_transmit_packets_total{{selector}}[{interval}])"
      ]
```

Metrics queries are in `canonical_name:query_string` format. The query string supports two variables that are replaced during execution:

| Setting      | Description |
|--------------|-------------|
| `{selector}` | Replaced with a `label_name=label_value` pair that selects metrics generated in Prometheus by a specific GitLab Runner instance. |
| `{interval}` | Replaced with the `query_interval` parameter from the `[runners.referees.metrics]` configuration for this referee. |

For example, a shared GitLab Runner environment that uses the `docker-machine` executor would have a `{selector}` similar to `node=shared-runner-123`.
