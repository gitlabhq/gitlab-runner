---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/engineering/ux/technical-writing/#designated-technical-writers
---

# Advanced configuration

You can change the behavior of GitLab Runner and of individual registered runners.

To do this, you modify a file called `config.toml`, which uses the [TOML](https://github.com/toml-lang/toml) format.

GitLab Runner does not require a restart when you change most options. This includes parameters
in the `[[runners]]` section and most parameters in the global section, except for `listen_address`.
If a runner was already registered, you don't need to register it again.

GitLab Runner checks for configuration modifications every 3 seconds and reloads if necessary.
GitLab Runner also reloads the configuration in response to the `SIGHUP` signal.

You can find the `config.toml` file in:

- `/etc/gitlab-runner/` on \*nix systems when GitLab Runner is
   executed as root (**this is also the path for service configuration**)
- `~/.gitlab-runner/` on \*nix systems when GitLab Runner is
   executed as non-root
- `./` on other systems

## The global section

These settings are global. They apply to all runners.

| Setting | Description |
| ------- | ----------- |
| `concurrent`     | Limits how many jobs can run concurrently. The maximum number is all defined runners. `0` **does not** mean unlimited. |
| `log_level`      | Defines the log level. Options are `debug`, `info`, `warn`, `error`, `fatal`, and `panic`. This setting has lower priority than the level set by the command-line arguments `--debug`, `-l`, or `--log-level`. |
| `log_format`     | Specifies the log format. Options are `runner`, `text`, and `json`. This setting has lower priority than the format set by command-line argument `--log-format`. The default value is `runner`. |
| `check_interval` | Defines the interval length, in seconds, between new jobs check. The default value is `3`. If set to `0` or lower, the default value is used. |
| `sentry_dsn`     | Enables tracking of all system level errors to Sentry. |
| `listen_address` | Defines an address (`<host>:<port>`) the Prometheus metrics HTTP server should listen on. |

Configuration example:

```toml
concurrent = 4
log_level = "warning"
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
{"arch":"amd64","level":"info","msg":"Runtime platform","os":"darwin","pid":38229,"revision":"HEAD","time":"2020-06-05T15:57:35+02:00","version":"development version"}
{"builds":0,"level":"info","msg":"Starting multi-runner from /etc/gitlab-runner/config.toml...","time":"2020-06-05T15:57:35+02:00"}
{"level":"warning","msg":"Running in user-mode.","time":"2020-06-05T15:57:35+02:00"}
{"level":"warning","msg":"Use sudo for system-mode:","time":"2020-06-05T15:57:35+02:00"}
{"level":"warning","msg":"$ sudo gitlab-runner...","time":"2020-06-05T15:57:35+02:00"}
{"level":"info","msg":"","time":"2020-06-05T15:57:35+02:00"}
{"builds":0,"level":"info","msg":"Configuration loaded","time":"2020-06-05T15:57:35+02:00"}
{"builds":0,"level":"info","msg":"listen_address not defined, metrics \u0026 debug endpoints disabled","time":"2020-06-05T15:57:35+02:00"}
{"builds":0,"level":"info","msg":"[session_server].listen_address not defined, session endpoints disabled","time":"2020-06-05T15:57:35+02:00"}
```

### How `check_interval` works

If more than one `[[runners]]` section exists in `config.toml`,
the interval between requests to GitLab are more frequent than you might expect. GitLab Runner
contains a loop that constantly schedules a request to the GitLab instance it's configured for.

GitLab Runner tries to ensure that subsequent requests for each runner are done in the specified interval.
To do this, it divides the value of `check_interval` by the number of `[[runners]]` sections. The loop
iterates over all sections, schedules a request for each, and sleeps for the calculated amount
of time. Things get interesting when the runners are tied to a different GitLab instance.
Consider the following example.

If you set `check_interval = 10`, and there are two runners (`runner-1` and `runner-2`),
a request is made each 10 seconds. Here is an example of the loop in this case:

1. Get `check_interval` value (`10s`).
1. Get list of runners (`runner-1`, `runner-2`).
1. Calculate the sleep interval (`10s / 2 = 5s`).
1. Start an infinite loop:
    1. Request a job for `runner-1`.
    1. Sleep for `5s`.
    1. Request a job for `runner-2`.
    1. Sleep for `5s`.
    1. Repeat.

In this example, a request from the runner's process is made every 5 seconds.
If `runner-1` and `runner-2` are connected to the same
GitLab instance, this GitLab instance also receives a new request from this runner
every 5 seconds.

Between the first request for `runner-1` and second request for `runner-1`
there are two sleep periods. Each one takes 5 seconds, so it's approximately 10 seconds between subsequent requests for `runner-1`.
The same applies for `runner-2`.

If you define more runners, the sleep interval is smaller. However, a request for a runner is
repeated after all requests for the other runners and their sleep periods are called.

## The `[session_server]` section

NOTE:
The `session_server` section is not supported by the
[`gitlab-runner` Helm chart](https://docs.gitlab.com/charts/charts/gitlab/gitlab-runner/index.html),
but [an issue exists to track this work](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/79).

The `[session_server]` section lets users interact with jobs, for example, in the
[interactive web terminal](https://docs.gitlab.com/ee/ci/interactive_web_terminal/index.html).

The `[session_server]` section should be specified at the root level, not per runner.
It should be defined outside the `[[runners]]` section.

```toml
[session_server]
  listen_address = "[::]:8093" #  listen on all available interfaces on port 8093
  advertise_address = "runner-host-name.tld:8093"
  session_timeout = 1800
```

Both `listen_address` and `advertise_address` should be in the form
of `host:port`, where `host` may be an IP address (`127.0.0.1:8093`)
or a domain (`my-runner.example.com:8093`). The runner uses this information to create
a TLS certificate, for a secure connection.

| Setting | Description |
| ------- | ----------- |
| `listen_address` | An internal URL for the session server. |
| `advertise_address`| The URL to access the session server. GitLab Runner exposes it to GitLab. If not defined, `listen_address` is used. |
| `session_timeout` | Number of seconds the session can stay active after the job completes. The timeout blocks the job from finishing. Default is `1800` (30 minutes). |

To disable the session server and terminal support, delete the `[session_server]` section.

If you are using the GitLab Runner Docker image, you must expose port `8093` by
adding `-p 8093:8093` to your [`docker run` command](../install/docker.md).

## The `[[runners]]` section

Each `[[runners]]` section defines one runner.

| Setting | Description |
| ------- | ----------- |
| `name`               | The runner's description. Informational only. |
| `url`                | GitLab instance URL. |
| `token`              | The runner's special token (not to be confused with the registration token). |
| `tls-ca-file`        | When using HTTPS, file that contains the certificates to verify the peer. See [Self-signed certificates or custom Certification Authorities documentation](tls-self-signed.md). |
| `tls-cert-file`      | When using HTTPS, file that contains the certificate to authenticate with the peer. |
| `tls-key-file`       | When using HTTPS, file that contains the private key to authenticate with the peer. |
| `limit`              | Limit how many jobs can be handled concurrently by this token. `0` (default) means do not limit. |
| `executor`           | Select how a project should be built. |
| `shell`              | Name of shell to generate the script. Default value is [platform dependent](../shells/index.md#overview). |
| `builds_dir`         | Absolute path to a directory where builds are stored in the context of the selected executor. For example, locally, Docker, or SSH. |
| `cache_dir`          | Absolute path to a directory where build caches are stored in context of selected executor. For example, locally, Docker, or SSH. If the `docker` executor is used, this directory needs to be included in its `volumes` parameter. |
| `environment`        | Append or overwrite environment variables. |
| `request_concurrency` | Limit number of concurrent requests for new jobs from GitLab. Default is `1`. |
| `output_limit`       | Maximum build log size in kilobytes. Default is `4096` (4MB). |
| `pre_clone_script`   | Commands to be executed on the runner before cloning the Git repository. Use it to adjust the Git client configuration first, for example. To insert multiple commands, use a (triple-quoted) multi-line string or `\n` character. |
| `pre_build_script`   | Commands to be executed on the runner after cloning the Git repository, but before executing the build. To insert multiple commands, use a (triple-quoted) multi-line string or `\n` character. |
| `post_build_script`  | Commands to be executed on the runner just after executing the build, but before executing `after_script`. To insert multiple commands, use a (triple-quoted) multi-line string or `\n` character. |
| `clone_url`          | Overwrite the URL for the GitLab instance. Used only if the runner can't connect to the GitLab URL. |
| `debug_trace_disabled` | Disables the `CI_DEBUG_TRACE` feature. When set to `true`, then debug log (trace) remains disabled, even if `CI_DEBUG_TRACE` is set to `true` by the user. |
| `referees` | Extra job monitoring workers that pass their results as job artifacts to GitLab. |

Example:

```toml
[[runners]]
  name = "ruby-2.6-docker"
  url = "https://CI/"
  token = "TOKEN"
  limit = 0
  executor = "docker"
  builds_dir = ""
  shell = ""
  environment = ["ENV=value", "LC_ALL=en_US.UTF-8"]
  clone_url = "http://gitlab.example.local"
```

### How `clone_url` works

When the GitLab instance is available at a URL that the runner can't use,
you can configure a `clone_url`.

For example, a firewall might prevent the runner from reaching the URL.
If the runner can reach the node on `192.168.1.23`, set the `clone_url` to `http://192.168.1.23`.

If the `clone_url` is set, the runner constructs a clone URL in the form
of `http://gitlab-ci-token:s3cr3tt0k3n@192.168.1.23/namespace/project.git`.

## The executors

The following executors are available.

| Executor | Required configuration | Where jobs run |
|-|-|-|
| `shell` |  | Local shell. The default executor. |
| `docker` | `[runners.docker]` and [Docker Engine](https://docs.docker.com/engine/) | A Docker container. |
| `docker-windows` | `[runners.docker]` and [Docker Engine](https://docs.docker.com/engine/) | A Windows Docker container. |
| `docker-ssh` | `[runners.docker]`, `[runners.ssh]`, and  [Docker Engine](https://docs.docker.com/engine/) | A Docker container, but connect with SSH.  **The Docker container runs on the local machine. This setting changes how the commands are run inside that container. If you want to run Docker commands on an external machine, change the  `host`  parameter in the  `runners.docker`  section.** |
| `ssh` | `[runners.ssh]` | SSH, remotely. |
| `parallels` | `[runners.parallels]` and `[runners.ssh]` | Parallels VM, but connect with SSH. |
| `virtualbox` | `[runners.virtualbox]` and `[runners.ssh]` | VirtualBox VM, but connect with SSH. |
| `docker+machine` | `[runners.docker]` and `[runners.machine]` | Like `docker`, but use [auto-scaled Docker machines](autoscale.md). |
| `docker-ssh+machine` | `[runners.docker]` and `[runners.machine]` | Like `docker-ssh`, but use [auto-scaled Docker machines](autoscale.md). |
| `kubernetes` | `[runners.kubernetes]` | Kubernetes pods. |

## The shells

The available shells can run on different platforms.

| Shell | Description |
| ----- | ----------- |
| `bash`        | Generate Bash (Bourne-shell) script. All commands executed in Bash context. Default for all Unix systems. |
| `sh`          | Generate Sh (Bourne-shell) script. All commands executed in Sh context. The fallback for `bash` for all Unix systems. |
| `powershell`  | Generate PowerShell script. All commands are executed in Windows PowerShell Desktop context. Default for Windows. |
| `pwsh`        | Generate PowerShell script. All commands are executed in PowerShell Core context. |

## The `[runners.docker]` section

This defines the Docker Container parameters.

| Parameter | Description |
| --------- | ----------- |
| `allowed_images`               | Wildcard list of images that can be specified in the `.gitlab-ci.yml` file. If not present, all images are allowed (equivalent to `["*/*:*"]`). |
| `allowed_services`             | Wildcard list of services that can be specified in the `.gitlab-ci.yml` file. If not present, all images are allowed (equivalent to `["*/*:*"]`). |
| `cache_dir`                    | Directory where Docker caches should be stored. This path can be absolute or relative to current working directory. See `disable_cache` for more information. |
| `cap_add`                      | Add additional Linux capabilities to the container. |
| `cap_drop`                     | Drop additional Linux capabilities from the container. |
| `cpuset_cpus`                  | The control group's `CpusetCpus`. A string. |
| `cpu_shares`                   | Number of CPU shares used to set relative CPU usage. Default is `1024`. |
| `cpus`                         | Number of CPUs (available in Docker 1.13 or later. A string.  |
| `devices`                      | Share additional host devices with the container. |
| `disable_cache`                | The Docker executor has two levels of caching: a global one (like any other executor) and a local cache based on Docker volumes. This configuration flag acts only on the local one which disables the use of automatically created (not mapped to a host directory) cache volumes. In other words, it only prevents creating a container that holds temporary files of builds, it does not disable the cache if the runner is configured in [distributed cache mode](autoscale.md#distributed-runners-caching). |
| `disable_entrypoint_overwrite` | Disable the image entrypoint overwriting. |
| `dns`                          | A list of DNS servers for the container to use. |
| `dns_search`                   | A list of DNS search domains. |
| `extra_hosts`                  | Hosts that should be defined in container environment. |
| `gpus`                         | GPU devices for Docker container. Uses the same format as the `docker` cli. View details in the [Docker documentation](https://docs.docker.com/config/containers/resource_constraints/#gpu). |
| `helper_image`                 | (Advanced) [The default helper image](#helper-image) used to clone repositories and upload artifacts. |
| `host`                         | Custom Docker endpoint. Default is `DOCKER_HOST` environment or `unix:///var/run/docker.sock`. |
| `hostname`                     | Custom hostname for the Docker container. |
| `image`                        | The image to run jobs with. |
| `links`                        | Containers that should be linked with container that runs the job. |
| `memory`                       | The memory limit. A string. |
| `memory_swap`                  | The total memory limit. A string. |
| `memory_reservation`           | The memory soft limit. A string. |
| `network_mode`                 | Add container to a custom network. |
| `oom_kill_disable`             | If an out-of-memory (OOM) error occurs, do not kill processes in a container. |
| `oom_score_adjust`             | OOM score adjustment. Positive means kill earlier. |
| `privileged`                   | Make the container run in privileged mode. Insecure. |
| `pull_policy`                  | The image pull policy: `never`, `if-not-present` or `always` (default). View details in the [pull policies documentation](../executors/docker.md#how-pull-policies-work). You can also add [multiple pull policies](../executors/docker.md#using-multiple-pull-policies). |
| `runtime`                      | The runtime for the Docker container. |
| `security_opt`                 | Security options (--security-opt in `docker run`). Takes a list of `:` separated key/values. |
| `shm_size`                     | Shared memory size for images (in bytes). |
| `sysctls`                      | The `sysctl` options. |
| `tls_cert_path`                | A directory where `ca.pem`, `cert.pem` or `key.pem` are stored and used to make a secure TLS connection to Docker. Useful in `boot2docker`. |
| `tls_verify`                   | Enable or disable TLS verification of connections to Docker daemon. Disabled by default. |
| `userns_mode`                  | The user namespace mode for the container and Docker services when user namespace remapping option is enabled. Available in Docker 1.10 or later. |
| `volumes`                      | Additional volumes that should be mounted. Same syntax as the Docker `-v` flag. |
| `volumes_from`                 | A list of volumes to inherit from another container in the form `<container name>[:<ro|rw>]`. Access level defaults to read-write, but can be manually set to `ro` (read-only) or `rw` (read-write). |
| `volume_driver`                | The volume driver to use for the container. |
| `wait_for_services_timeout`    | How long to wait for Docker services. Set to `0` to disable. Default is `30`. |

### The `[[runners.docker.services]]` section

Specify additional services that should be run with the job. Visit the
[Docker Registry](https://hub.docker.com) for the list of available applications.
Each service runs in a separate container and is linked to the job.

| Parameter | Description |
| --------- | ----------- |
| `name`  | The name of the image to be run as a service. |
| `alias` | Additional [alias name](https://docs.gitlab.com/ee/ci/docker/using_docker_images.html#available-settings-for-services) that can be used to access the service .|
| `entrypoint` | Command or script that should be executed as the container’s entrypoint. The syntax is similar to [Dockerfile’s ENTRYPOINT](https://docs.docker.com/engine/reference/builder/#entrypoint) directive, where each shell token is a separate string in the array. Introduced in [GitLab Runner 13.6](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27173). |
| `command` | Command or script that should be used as the container’s command. The syntax is similar to [Dockerfile’s CMD](https://docs.docker.com/engine/reference/builder/#cmd) directive, where each shell token is a separate string in the array. Introduced in [GitLab Runner 13.6](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27173). |

Example:

```toml
[runners.docker]
  host = ""
  hostname = ""
  tls_cert_path = "/Users/ayufan/.boot2docker/certs"
  image = "ruby:2.6"
  memory = "128m"
  memory_swap = "256m"
  memory_reservation = "64m"
  oom_kill_disable = false
  cpuset_cpus = "0,1"
  cpus = "2"
  dns = ["8.8.8.8"]
  dns_search = [""]
  privileged = false
  userns_mode = "host"
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
  [[runners.docker.services]]
    name = "registry.example.com/svc1"
    alias = "svc1"
    entrypoint = ["entrypoint.sh"]
    command = ["executable","param1","param2"]
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

[View the complete guide of Docker volume usage](https://docs.docker.com/userguide/dockervolumes/).

The following examples show how to specify volumes in the `[runners.docker]` section.

#### Example 1: Add a data volume

A data volume is a specially-designated directory in one or more containers
that bypasses the Union File System. Data volumes are designed to persist data,
independent of the container's life cycle.

```toml
[runners.docker]
  host = ""
  hostname = ""
  tls_cert_path = "/Users/ayufan/.boot2docker/certs"
  image = "ruby:2.6"
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
  image = "ruby:2.6"
  privileged = false
  disable_cache = true
  volumes = ["/path/to/bind/from/host:/path/to/bind/in/container:rw"]
```

This example uses `/path/to/bind/from/host` of the CI/CD host in the container at
`/path/to/bind/in/container`.

GitLab Runner 11.11 and later [mount the host directory](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/1261)
for the defined [services](https://docs.gitlab.com/ee/ci/services/) as
well.

### Use a private container registry

To use private registries as a source of images for your jobs,
you can set the authorization configuration in a [CI/CD variable](https://docs.gitlab.com/ee/ci/variables/)
named `DOCKER_AUTH_CONFIG`. You can set the variable in the project's CI/CD settings
or in the `config.toml` file.

Using private registries with the `if-not-present` pull policy may introduce
[security implications](../security/index.md#usage-of-private-docker-images-with-if-not-present-pull-policy).
To fully understand how pull policies work,
read the [pull policies documentation](../executors/docker.md#how-pull-policies-work).

For a detailed example, visit the [Using Docker images documentation](https://docs.gitlab.com/ee/ci/docker/using_docker_images.html#define-an-image-from-a-private-container-registry).

The steps performed by the runner can be summed up as:

1. The registry name is found from the image name.
1. If the value is not empty, the executor searches for the authentication
   configuration for this registry.
1. Finally, if an authentication corresponding to the specified registry is
   found, subsequent pulls makes use of it.

Now that the runner is set up to authenticate against your private registry,
learn [how to configure the `.gitlab-ci.yml` file](https://docs.gitlab.com/ee/ci/yaml/README.html#image) to use that
registry.

#### Support for GitLab integrated registry

GitLab sends credentials for its integrated
registry along with the job's data. These credentials are automatically
added to the registry's authorization parameters list.

After this step, authorization against the registry proceeds similarly to
configuration added with the `DOCKER_AUTH_CONFIG` variable.

In your jobs, you can use any image from your GitLab integrated
registry, even if the image is private or protected. For information on the images jobs have access to, read the
[CI/CD job token documentation](https://docs.gitlab.com/ee/api/README.html#gitlab-cicd-job-token) documentation.

#### Precedence of Docker authorization resolving

As described earlier, GitLab Runner can authorize Docker against a registry by
using credentials sent in different way. To find a proper registry, the following
precedence is taken into account:

1. Credentials configured with `DOCKER_AUTH_CONFIG`.
1. Credentials configured locally on the GitLab Runner host with `~/.docker/config.json`
   or `~/.dockercfg` files (for example, by running `docker login` on the host).
1. Credentials sent by default with a job's payload (for example, credentials for the *integrated
   registry* described earlier).

The first credentials found for the registry are used. So for example,
if you add credentials for the *integrated registry* with the
`DOCKER_AUTH_CONFIG` variable, then the default credentials are overridden.

#### Restrict `allowed_images` to private registry

You can restrict access so that your jobs run on Docker images
from your private Docker registry. For example:

```toml
[runners.docker]
  ...
  allowed_images = ["my.registry.tld:5000/*:*"]
```

## The `[runners.parallels]` section

The following parameters are for Parallels.

| Parameter | Description |
| --------- | ----------- |
| `base_name`         | Name of Parallels VM that is cloned. |
| `template_name`     | Custom name of Parallels VM linked template. Optional. |
| `disable_snapshots` | If disabled, the VMs are destroyed when the jobs are done. |

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

| Parameter | Explanation |
| --------- | ----------- |
| `base_name`         | Name of the VirtualBox VM that is cloned. |
| `base_snapshot`     | Name or UUID of a specific snapshot of the VM to create a linked clone from. If this value is empty or omitted, the current snapshot is used. If no current snapshot exists, one is created. Unless `disable_snapshots` is true, in which case a full clone of the base VM is made. |
| `base_folder`       | Folder to save the new VM in. If this value is empty or omitted, the default VM folder is used. |
| `disable_snapshots` | If disabled, the VMs are destroyed when the jobs are done. |

Example:

```toml
[runners.virtualbox]
  base_name = "my-virtualbox-image"
  base_snapshot = "my-image-snapshot"
  disable_snapshots = false
```

## The `[runners.ssh]` section

The following parameters define the SSH connection.

| Parameter  | Description |
| ---------- | ----------- |
| `host`     | Where to connect. Overridden when you use `docker-ssh`. |
| `port`     | Port. Default is `22`. |
| `user`     | Username. |
| `password` | Password. |
| `identity_file` | File path to SSH private key (`id_rsa`, `id_dsa`, or `id_edcsa`). The file must be stored unencrypted. |

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

> Added in GitLab Runner v1.1.0.

The following parameters define the Docker Machine-based autoscaling feature. More details can be
found in the separate [runner autoscale documentation](autoscale.md).

| Parameter           | Description |
|---------------------|-------------|
| `MaxGrowthRate`     | The maximum number of machines that can be added to the runner in parallel. Default is `0` (no limit). |
| `IdleCount`         | Number of machines that need to be created and waiting in _Idle_ state. |
| `IdleTime`          | Time (in seconds) for machine to be in _Idle_ state before it is removed. |
| `[[runners.machine.autoscaling]]` | Multiple sections, each containing overrides for autoscaling configuration. The last section with an expression that matches the current time is selected. |
| `OffPeakPeriods`    | Deprecated: Time periods when the scheduler is in the OffPeak mode. An array of cron-style patterns (described [below](#periods-syntax)). |
| `OffPeakTimezone`   | Deprecated: Timezone for the times given in OffPeakPeriods. A timezone string like `Europe/Berlin`. Defaults to the locale system setting of the host if omitted or empty. GitLab Runner attempts to locate the timezone database in the directory or uncompressed zip file named by the `ZONEINFO` environment variable, then looks in known installation locations on Unix systems, and finally looks in `$GOROOT/lib/time/zoneinfo.zip`. |
| `OffPeakIdleCount`  | Deprecated: Like `IdleCount`, but for _Off Peak_ time periods. |
| `OffPeakIdleTime`   | Deprecated: Like `IdleTime`, but for _Off Peak_ time periods. |
| `MaxBuilds`         | Maximum job (build) count before machine is removed. |
| `MachineName`       | Name of the machine. It **must** contain `%s`, which is replaced with a unique machine identifier. |
| `MachineDriver`     | Docker Machine `driver`. View details in the [Docker Machine configuration section](autoscale.md#supported-cloud-providers). |
| `MachineOptions`    | Docker Machine options. View details in the [Docker Machine configuration section](autoscale.md#supported-cloud-providers). |

### The `[[runners.machine.autoscaling]]` sections

| Parameter           | Description |
|---------------------|-------------|
| `Periods`           | Time periods during which this schedule is active. An array of cron-style patterns (described [below](#periods-syntax)).
| `IdleCount`         | Number of machines that need to be created and waiting in _Idle_ state. |
| `IdleTime`          | Time (in seconds) for a machine to be in _Idle_ state before it is removed. |
| `Timezone`   | Timezone for the times given in `Periods`. A timezone string like `Europe/Berlin`. Defaults to the locale system setting of the host if omitted or empty. GitLab Runner attempts to locate the timezone database in the directory or uncompressed zip file named by the `ZONEINFO` environment variable, then looks in known installation locations on Unix systems, and finally looks in `$GOROOT/lib/time/zoneinfo.zip`. |

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
| `run_exec`              | string       | **Required.** Path to an executable to run scripts in the environments. For example, the clone and build script. |
| `run_args`              | string array | First set of arguments passed to the `run_exec` executable. |
| `cleanup_exec`          | string       | Path to an executable to clean up the environment. |
| `cleanup_args`          | string array | First set of arguments passed to the `cleanup_exec` executable. |
| `cleanup_exec_timeout`  | integer      | Timeout, in seconds, for `cleanup_exec` to finish execution. Default is 3600 seconds (1 hour). |
| `graceful_kill_timeout` | integer      | Time to wait, in seconds, for `prepare_exec` and `cleanup_exec` if they are terminated (for example, during job cancellation). After this timeout, the process is killed. Default is 600 seconds (10 minutes). |
| `force_kill_timeout`    | integer      | Time to wait, in seconds, after the kill signal is sent to the script. Default is 600 seconds (10 minutes). |

## The `[runners.cache]` section

> Introduced in GitLab Runner 1.1.0.

The following parameters define the distributed cache feature. View details
in the [runner autoscale documentation](autoscale.md#distributed-runners-caching).

| Parameter        | Type             | Description |
|------------------|------------------|-------------|
| `Type`           | string           | One of: `s3`, `gcs`, `azure`. |
| `Path`           | string           | Name of the path to prepend to the cache URL. |
| `Shared`         | boolean          | Enables cache sharing between runners. Default is `false`. |

WARNING:
In GitLab Runner 11.3, the configuration parameters related to S3 were moved to a dedicated `[runners.cache.s3]` section.
The configuration with S3 configured directly in `[runners.cache]` was deprecated.
**In GitLab Runner 12.0, the configuration syntax was removed and is no longer supported**.

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

This table lists `config.toml`, CLI options, and ENV variables for `register`.

| Setting               | TOML field                               | CLI option for `register`      | ENV for `register`                | Before 12.0.0 TOML field            | Before 12.0.0 CLI option | Before 12.0.0 ENV         |
|-----------------------|------------------------------------------|--------------------------------|-----------------------------------|-------------------------------------|--------------------------|---------------------------|
| `Type`                | `[runners.cache] -> Type`                | `--cache-type`                 | `$CACHE_TYPE`                     |                                     |                          |                           |
| `Path`                | `[runners.cache] -> Path`                | `--cache-path`                 | `$CACHE_PATH`                     |                                     | `--cache-s3-cache-path`  | `$S3_CACHE_PATH`          |
| `Shared`              | `[runners.cache] -> Shared`              | `--cache-shared`               | `$CACHE_SHARED`                   |                                     | `--cache-cache-shared`   |                           |
| `S3.ServerAddress`    | `[runners.cache.s3] -> ServerAddress`    | `--cache-s3-server-address`    | `$CACHE_S3_SERVER_ADDRESS`        | `[runners.cache] -> ServerAddress`  |                          | `$S3_SERVER_ADDRESS`      |
| `S3.AccessKey`        | `[runners.cache.s3] -> AccessKey`        | `--cache-s3-access-key`        | `$CACHE_S3_ACCESS_KEY`            | `[runners.cache] -> AccessKey`      |                          | `$S3_ACCESS_KEY`          |
| `S3.SecretKey`        | `[runners.cache.s3] -> SecretKey`        | `--cache-s3-secret-key`        | `$CACHE_S3_SECRET_KEY`            | `[runners.cache] -> SecretKey`      |                          | `$S3_SECRET_KEY`          |
| `S3.BucketName`       | `[runners.cache.s3] -> BucketName`       | `--cache-s3-bucket-name`       | `$CACHE_S3_BUCKET_NAME`           | `[runners.cache] -> BucketName`     |                          | `$S3_BUCKET_NAME`         |
| `S3.BucketLocation`   | `[runners.cache.s3] -> BucketLocation`   | `--cache-s3-bucket-location`   | `$CACHE_S3_BUCKET_LOCATION`       | `[runners.cache] -> BucketLocation` |                          | `$S3_BUCKET_LOCATION`     |
| `S3.Insecure`         | `[runners.cache.s3] -> Insecure`         | `--cache-s3-insecure`          | `$CACHE_S3_INSECURE`              | `[runners.cache] -> Insecure`       |                          | `$S3_INSECURE`            |
| `GCS.AccessID`        | `[runners.cache.gcs] -> AccessID`        | `--cache-gcs-access-id`        | `$CACHE_GCS_ACCESS_ID`            |                                     |                          |                           |
| `GCS.PrivateKey`      | `[runners.cache.gcs] -> PrivateKey`      | `--cache-gcs-private-key`      | `$CACHE_GCS_PRIVATE_KEY`          |                                     |                          |                           |
| `GCS.CredentialsFile` | `[runners.cache.gcs] -> CredentialsFile` | `--cache-gcs-credentials-file` | `$GOOGLE_APPLICATION_CREDENTIALS` |                                     |                          |                           |
| `GCS.BucketName`      | `[runners.cache.gcs] -> BucketName`      | `--cache-gcs-bucket-name`      | `$CACHE_GCS_BUCKET_NAME`          |                                     |                          |                           |
| `Azure.AccountName`   | `[runners.cache.azure] -> AccountName`   | `--cache-azure-account-name`   | `$CACHE_AZURE_ACCOUNT_NAME`       |                                     |                          |                           |
| `Azure.AccountKey`    | `[runners.cache.azure] -> AccountKey`    | `--cache-azure-account-key`    | `$CACHE_AZURE_ACCOUNT_KEY`        |                                     |                          |                           |
| `Azure.ContainerName` | `[runners.cache.azure] -> ContainerName` | `--cache-azure-container-name` | `$CACHE_AZURE_CONTAINER_NAME`     |                                     |                          |                           |
| `Azure.StorageDomain` | `[runners.cache.azure] -> StorageDomain` | `--cache-azure-storage-domain` | `$CACHE_AZURE_STORAGE_DOMAIN`     |                                     |                          |                           |

### The `[runners.cache.s3]` section

The following parameters define S3 storage for cache.

In GitLab Runner 11.2 and earlier, these settings were in the global `[runners.cache]` section.

| Parameter        | Type             | Description |
|------------------|------------------|-------------|
| `ServerAddress`  | string           | A `host:port` for the S3-compatible server. If you are using a server other than AWS, consult the storage product documentation to determine the correct address. For DigitalOcean, the address must be in the format `spacename.region.digitaloceanspaces.com`. |
| `AccessKey`      | string           | The access key specified for your S3 instance. |
| `SecretKey`      | string           | The secret key specified for your S3 instance. |
| `BucketName`     | string           | Name of the storage bucket where cache is stored. |
| `BucketLocation` | string           | Name of S3 region. |
| `Insecure`       | boolean          | Set to `true` if the S3 service is available by `HTTP`. Default is `false`. |

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
```

For Amazon S3, the `ServerAddress` should always be `s3.amazonaws.com`. The MinIO S3 client
gets bucket metadata and modifies the URL to point to the valid region, for example `s3-eu-west-1.amazonaws.com`.

If `ServerAddress`, `AccessKey` or `SecretKey` aren't specified, then the S3 client uses the
IAM instance profile available to the `gitlab-runner` instance. In an
[autoscale](autoscale.md) configuration, this is not the on-demand machine
that jobs are executed on.

### The `[runners.cache.gcs]` section

> Introduced in GitLab Runner 11.3.0.

The following parameters define native support for Google Cloud Storage. To view
where these values come from, view the
[Google Cloud Storage (GCS) Authentication documentation](https://cloud.google.com/storage/docs/authentication#service_accounts).

| Parameter         | Type             | Description |
|-------------------|------------------|-------------|
| `CredentialsFile` | string           | Path to the Google JSON key file. Only the `service_account` type is supported. If configured, this value takes precedence over the `AccessID` and `PrivateKey` configured directly in `config.toml`. |
| `AccessID`        | string           | ID of GCP Service Account used to access the storage. |
| `PrivateKey`      | string           | Private key used to sign GCS requests. |
| `BucketName`      | string           | Name of the storage bucket where cache is stored. |

Examples:

**Credentials configured directly in `config.toml` file:**

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

**Credentials in JSON file downloaded from GCP:**

```toml
[runners.cache]
  Type = "gcs"
  Path = "path/to/prefix"
  Shared = false
  [runners.cache.gcs]
    CredentialsFile = "/etc/gitlab-runner/service-account.json"
    BucketName = "runners-cache"
```

### The `[runners.cache.azure]` section

> Introduced in GitLab Runner 13.4.0.

The following parameters define native support for Azure Blob Storage. To learn more, view the
[Azure Blob Storage documentation](https://docs.microsoft.com/en-us/azure/storage/blobs/storage-blobs-introduction).
While S3 and GCS use the word `bucket` for a collection of objects, Azure uses the word
`container` to denote a collection of blobs.

| Parameter         | Type             | Description |
|-------------------|------------------|-------------|
| `AccountName`     | string           | Name of the Azure Blob Storage account used to access the storage. |
| `AccountKey`      | string           | Storage account access key used to access the container. |
| `ContainerName`   | string           | Name of the [storage container](https://docs.microsoft.com/en-us/azure/storage/blobs/storage-blobs-introduction#containers) to save cache data in. |
| `StorageDomain`   | string           | Domain name [used to service Azure storage endpoints](https://docs.microsoft.com/en-us/azure/china/resources-developer-guide#check-endpoints-in-azure) (optional). Default is `blob.core.windows.net`. |

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

## The `[runners.kubernetes]` section

> Introduced in GitLab Runner v1.6.0.

The following parameters define Kubernetes behavior.
For more parameters, see the [Kubernetes executor documentation](../executors/kubernetes.md).

| Parameter        | Type    | Description |
|------------------|---------|-------------|
| `host`           | string  | Optional. Kubernetes host URL. If not specified, the runner attempts to auto-discovery it. |
| `cert_file`      | string  | Optional. Kubernetes auth certificate. |
| `key_file`       | string  | Optional. Kubernetes auth private key. |
| `ca_file`        | string  | Optional. Kubernetes auth ca certificate. |
| `image`          | string  | Default Docker image to use for jobs when none is specified. |
| `namespace`      | string  | Namespace to run Kubernetes jobs in. |
| `privileged`     | boolean | Run all containers with the privileged flag enabled. |
| `allow_privilege_escalation` | boolean | Optional. Runs all containers with the `allowPrivilegeEscalation` flag enabled. |
| `node_selector`  | table   | A `table` of `key=value` pairs of `string=string`. Limits the creation of pods to Kubernetes nodes that match all the `key=value` pairs. |
| `image_pull_secrets` | array | A list of secrets that are used to authenticate Docker image pulling. |

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
  image_pull_secrets = ["docker-registry-credentials"]
  [runners.kubernetes.node_selector]
    gitlab = "true"
```

## Helper image

When you use `docker`, `docker+machine`, or `kubernetes` executors, GitLab Runner uses a specific container
to handle Git, artifacts, and cache operations. This container is created from an image named `helper image`.

The helper image is based on Alpine Linux and is available for amd64, arm, arm64, and s390x architectures. It contains
a `gitlab-runner-helper` binary, which is a special compilation of GitLab Runner binary. It contains only a subset
of available commands, as well as Git, Git LFS, SSL certificates store, and basic configuration of Alpine.

When GitLab Runner is installed from the DEB/RPM packages, images for the supported architectures are installed on the host.
When the runner prepares to execute the job, if the image in the specified version (based on the runner's Git
revision) is not found on Docker Engine, it is automatically loaded. Both the
`docker` and `docker+machine` executors work this way.

The `kubernetes` executor and manual installations of GitLab Runner work differently.

- For manual installations, the `gitlab-runner-helper` binary is not included.
- For the `kubernetes` executor, the Kubernetes API doesn't allow the `gitlab-runner-helper` image to be loaded from a local archive.

In both cases, GitLab Runner downloads the helper image from Docker Hub, from the official GitLab repository `gitlab/gitlab-runner-helper`.
The GitLab Runner revision and architecture define which tag to download.

### Migrate helper image to `registry.gitlab.com`

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27196) in GitLab Runner 13.7.

The helper image is currently hosted in [Docker
Hub](https://hub.docker.com/r/gitlab/gitlab-runner-helper). With the new [Docker
Hub limits](https://docs.docker.com/docker-hub/download-rate-limit/), we are
migrating the helper image to
[`registry.gitlab.com`](https://gitlab.com/gitlab-org/gitlab-runner/container_registry/1472754).

By default, the helper image is pulled from Docker Hub. If the
`FF_GITLAB_REGISTRY_HELPER_IMAGE` [feature flag](feature-flags.md) is enabled
the runner pulls the image from
[`registry.gitlab.com`](https://gitlab.com/gitlab-org/gitlab-runner/container_registry/1472754)
where the limits don't apply. You can enable the feature flag for:

- A [specific pipeline](feature-flags.md#enable-feature-flag-in-pipeline-configuration).
- [Every job](feature-flags.md#enable-feature-flag-in-runner-environment-variables).

If you are a runner administrator and don't want users to override it, you can set this
feature flag by specifying
[`[runners.feature_flags]`](feature-flags.md#enable-feature-flag-in-runner-configuration).

### Override the helper image

In some cases, you may need to override the helper image. There are many reasons for doing this:

1. **To speed up jobs execution**: In environments with slower internet connection, downloading the
   same image multiple times from Docker Hub can increase the time it takes to execute a job. Downloading the helper image from
   a local registry, where the exact copy of `gitlab/gitlab-runner-helper:XYZ` is stored, can speed things up.

1. **Security concerns**: You may not want to download external dependencies that were not checked before. There
   might be a business rule to use only dependencies that were reviewed and stored in local repositories.

1. **Build environments without internet access**: In some cases, jobs are executed in an environment that has
   a dedicated, closed network. This doesn't apply to the `kubernetes` executor, where the image still needs to be downloaded
   from an external registry that is available to the Kubernetes cluster.

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

By default, GitLab Runner references a `gitlab/gitlab-runner-helper:XYZ` image, where `XYZ` is based
on the GitLab Runner architecture and Git revision. In GitLab Runner 11.3 and later, you can define the
image version by using one of the
[version variables](https://gitlab.com/gitlab-org/gitlab-runner/blob/11-3-stable/common/version.go#L48-50):

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    helper_image = "my.registry.local/gitlab/gitlab-runner-helper:x86_64-${CI_RUNNER_REVISION}"
```

With this configuration, GitLab Runner instructs the executor to use the image in version `x86_64-${CI_RUNNER_REVISION}`,
which is based on its compilation data. After updating GitLab Runner to a new version, GitLab
Runner tries to download the proper image. The image should be uploaded to the registry
before upgrading GitLab Runner, otherwise the jobs start failing with a "No such image" error.

In GitLab Runner 13.2 and later, the helper image is tagged by
`$CI_RUNNER_VERSION` in addition to `$CI_RUNNER_REVISION`. Both tags are
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

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27252) in GitLab 13.9.

An additional version of the helper image for Linux,
which contains PowerShell Core, is published with the `gitlab/gitlab-runner-helper:XYZ-pwsh` tag.

## The `[runners.custom_build_dir]` section

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/1267) in GitLab Runner 11.10.

This section defines [custom build directories](https://docs.gitlab.com/ee/ci/yaml/README.html#custom-build-directories) parameters.

This feature, if not configured explicitly, is
enabled by default for `kubernetes`, `docker`, `docker-ssh`, `docker+machine`,
and `docker-ssh+machine` executors. For all other executors, it is disabled by default.

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

## The `[runners.referees]` section

> - [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/1545) in GitLab Runner 12.7.
> - Requires [GitLab v12.6](https://about.gitlab.com/releases/2019/12/22/gitlab-12-6-released/) or later.

Use GitLab Runner referees to pass extra job monitoring data to GitLab. Referees are workers in the Runner Manager that query and collect additional data related to a job. The results
are uploaded to GitLab as job artifacts.

### Use the Metrics Runner referee

If the machine or container running the job exposes [Prometheus](https://prometheus.io) metrics, GitLab Runner can query the Prometheus server for the entirety of the job duration. After the metrics are received, they are uploaded as a job artifact that can be used for analysis later.

Only the [`docker-machine` executor](../executors/docker_machine.md) supports the referee.

### Configure the Metrics Runner Referee for GitLab Runner

Define `[runner.referees]` and `[runner.referees.metrics]` in your `config.toml` file within a `[[runner]]` section and add the following fields:

| Setting              | Description                                                                                                                         |
| -------------------- | ----------------------------------------------------------------------------------------------------------------------------------- |
| `prometheus_address` | The server that collects metrics from GitLab Runner instances. It must be accessible by the Runner Manager when the job finishes.   |
| `query_interval`     | The frequency the Prometheus instance associated with a job is queried for time series data, defined as an interval (in seconds).   |
| `queries`            | An array of [PromQL](https://prometheus.io/docs/prometheus/latest/querying/basics/) queries that are executed for each interval.    |

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

| Setting      | Description                                                                                                                   |
| ------------ | ----------------------------------------------------------------------------------------------------------------------------- |
| `{selector}` | Replaced with a `label_name=label_value` pair that selects metrics generated in Prometheus by a specific GitLab Runner instance. |
| `{interval}` | Replaced with the `query_interval` parameter from the `[runners.referees.metrics]` configuration for this referee.            |

For example, a shared GitLab Runner environment that uses the `docker-machine` executor would have a `{selector}` similar to `node=shared-runner-123`.

## Deploy to multiple servers using GitLab CI/CD

To deploy to multiple servers by using GitLab CI/CD, create a
single script that deploys to multiple servers, or create many scripts.
It depends on what you'd like to do.
