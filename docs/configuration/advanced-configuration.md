---
table_display_block: true
---

# Advanced configuration

GitLab Runner configuration uses the [TOML](https://github.com/toml-lang/toml) format.

The file to be edited can be found in:

1. `/etc/gitlab-runner/config.toml` on \*nix systems when GitLab Runner is
   executed as root (**this is also path for service configuration**)
1. `~/.gitlab-runner/config.toml` on \*nix systems when GitLab Runner is
   executed as non-root
1. `./config.toml` on other systems

## The global section

This defines global settings of GitLab Runner.

| Setting | Description |
| ------- | ----------- |
| `concurrent`     | limits how many jobs globally can be run concurrently. The most upper limit of jobs using all defined runners. `0` **does not** mean unlimited |
| `log_level`      | Log level (options: `debug`, `info`, `warn`, `error`, `fatal`, `panic`). Note that this setting has lower priority than level set by command line argument `--debug`, `-l`, or `--log-level` |
| `log_format`     | Log format (options: `runner`, `text`, `json`). Note that this setting has lower priority than format set by command line argument `--log-format` The default value is `runner`. |
| `check_interval` | defines the interval length, in seconds, between new jobs check. The default value is `3`; if set to `0` or lower, the default value will be used. |
| `sentry_dsn`     | enable tracking of all system level errors to Sentry |
| `listen_address` | address (`<host>:<port>`) on which the Prometheus metrics HTTP server should be listening |

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

If there is more than one `[[runners]]` section in `config.toml` (let's call them workers),
the interval between requests to GitLab are more frequent than one could expect. GitLab Runner
contains a loop that constantly schedules a request that should be made for a worker against
the GitLab instance it's configured for.

GitLab Runner tries to ensure that subsequent requests for one worker will be done in the specified interval,
so the value of `check_interval` is divided by the number of the `[[runners]]` sections. The loop will next
iterate over all sections, schedule a request for each of them, and will sleep for the calculated amount
of time. Things get interesting when the workers are tied to a different GitLab instance.
Consider the following example.

If one would set `check_interval = 10`, and there were 2 workers in total (`runner-1` and `runner-2`),
a subsequent request would be made each 10 seconds. The loop would look like:

1. Get `check_interval` value (`10s`).
1. Get list of workers (`runner-1`, `runner-2`).
1. Calculate the sleep interval (`10s / 2 = 5s`).
1. Start an infinite loop:
    1. Request a job for `runner-1`.
    1. Sleep for `5s`.
    1. Request a job for `runner-2`.
    1. Sleep for `5s`.
    1. Repeat.

So, a request from the Runner's process is made each 5s. If `runner-1` and `runner-2` are connected to the same
GitLab instance, it means that the request to this GitLab instance will receive a new request from this Runner
also each 5s. But as you can see, between the first request for `runner-1` and second request for `runner-1`
there are two sleeps taking 5s, so finally it's ~10s between subsequent requests for `runner-1`. The same goes
for `runner-2`. If you define more workers, the sleep interval will be smaller, but a request for a worker will
be repeated after all requests for the other workers + their sleeps are called.

## The `[session_server]` section

NOTE: **Note:**
`session_server` is not yet supported by
[`gitlab-runner` Helm chart](https://docs.gitlab.com/charts/charts/gitlab/gitlab-runner/index.html),
but support [is planned](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/79).

The section `[session_server]` is a system runner level configuration, so it should be specified at the root level,
not per executor i.e. it should be outside `[[runners]]` section. The session server allows the user to interact
with jobs that the Runner is responsible for. A good example of this is the
[interactive web terminal](https://docs.gitlab.com/ee/ci/interactive_web_terminal/index.html).

Both `listen_address` and `advertise_address` should be provided in the form
of `host:port`, where `host` may be an IP address (e.g., `127.0.0.1:8093`)
or a domain (e.g., `my-runner.example.com:8093`). The Runner will create a
TLS certificate automatically to have a secure connection.

If you want to disable the session server, just delete the `[session_server]`
section and terminal support will be disabled.

| Setting | Description |
| ------- | ----------- |
| `listen_address` | An internal URL to be used for the session server. |
| `advertise_address`| The URL that the Runner will expose to GitLab to be used to access the session server. Fallbacks to `listen_address` if not defined.   |
| `session_timeout` | How long in seconds the session can stay active after the job completes (which will block the job from finishing), defaults to `1800` (30 minutes). |

Example:

```toml
[session_server]
  listen_address = "[::]:8093" #  listen on all available interfaces on port 8093
  advertise_address = "runner-host-name.tld:8093"
  session_timeout = 1800
```

NOTE: **Note:**
If using the GitLab Runner Docker image, you will also need to expose port 8093 by
adding `-p 8093:8093` to your [`docker run` command](../install/docker.md).

## The `[[runners]]` section

This defines one runner entry.

| Setting | Description |
| ------- | ----------- |
| `name`               | The Runner's description, just informatory |
| `url`                | GitLab URL |
| `token`              | The Runner's special token (not to be confused with the registration token) |
| `tls-ca-file`        | File containing the certificates to verify the peer when using HTTPS |
| `tls-cert-file`      | File containing the certificate to authenticate with the peer when using HTTPS |
| `tls-key-file`       | File containing the private key to authenticate with the peer when using HTTPS |
| `limit`              | Limit how many jobs can be handled concurrently by this token. `0` (default) simply means don't limit |
| `executor`           | Select how a project should be built, see next section |
| `shell`              | Name of shell to generate the script. Default value is [platform dependent](../shells/index.md#overview). |
| `builds_dir`         | Absolute path to a directory where builds will be stored in context of selected executor (Locally, Docker, SSH) |
| `cache_dir`          | Absolute path to a directory where build caches will be stored in context of selected executor (locally, Docker, SSH). If the `docker` executor is used, this directory needs to be included in its `volumes` parameter. |
| `environment`        | Append or overwrite environment variables |
| `request_concurrency` | Limit number of concurrent requests for new jobs from GitLab (default 1) |
| `output_limit`       | Set maximum build log size in kilobytes, by default set to 4096 (4MB) |
| `pre_clone_script`   | Commands to be executed on the Runner before cloning the Git repository. this can be used to adjust the Git client configuration first, for example. To insert multiple commands, use a (triple-quoted) multi-line string or "\n" character. |
| `pre_build_script`   | Commands to be executed on the Runner after cloning the Git repository, but before executing the build. To insert multiple commands, use a (triple-quoted) multi-line string or "\n" character. |
| `post_build_script`  | Commands to be executed on the Runner just after executing the build, but before executing `after_script`. To insert multiple commands, use a (triple-quoted) multi-line string or "\n" character. |
| `clone_url`          | Overwrite the URL for the GitLab instance. Used if the Runner can't connect to GitLab on the URL GitLab exposes itself. |
| `debug_trace_disabled` | Disables the `CI_DEBUG_TRACE` feature. When set to true, then debug log (trace) will remain disabled even if `CI_DEBUG_TRACE` will be set to `true` by the user. |
| `referees` | Extra job monitoring workers that pass their results as job artifacts to GitLab |

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

In cases where the GitLab instance is exposed to a URL which can't be used
by the runner, a `clone_url` can be configured. For example; GitLab is exposed
to `https://gitlab.example.com`, but the runner can't reach that because of
a firewall setup. If the runner can reach the node on `192.168.1.23`,
the `clone_url` should be set to `http://192.168.1.23`.

Only if the `clone_url` is set, the runner will construct a clone URL in the form
of `http://gitlab-ci-token:s3cr3tt0k3n@192.168.1.23/namespace/project.git`.

## The EXECUTORS

There are a couple of available executors currently.

| Executor | Description |
| -------- | ----------- |
| `shell`       | run build locally, default |
| `docker`      | run build using Docker container. This requires the presence of `[runners.docker]` and [Docker Engine](https://docs.docker.com/engine/) installed on a system that the Runner will run the job on. |
| `docker-windows` | run build using Windows Docker container. This requires the presence of `[runners.docker]` and [Docker Engine](https://docs.docker.com/engine/) installed on a Windows system. |
| `docker-ssh`  | run build using Docker container, but connect to it with SSH - this requires the presence of `[runners.docker]` , `[runners.ssh]` and [Docker Engine](https://docs.docker.com/engine/) installed on the system that the Runner runs. **Note: This will run the Docker container on the local machine, it just changes how the commands are run inside that container. If you want to run Docker commands on an external machine, then you should change the `host` parameter in the `runners.docker` section.**|
| `ssh`         | run build remotely with SSH - this requires the presence of `[runners.ssh]` |
| `parallels`   | run build using Parallels VM, but connect to it with SSH - this requires the presence of `[runners.parallels]` and `[runners.ssh]` |
| `virtualbox`  | run build using VirtualBox VM, but connect to it with SSH - this requires the presence of `[runners.virtualbox]` and `[runners.ssh]` |
| `docker+machine` | like `docker`, but uses [auto-scaled Docker machines](autoscale.md) - this requires the presence of `[runners.docker]` and `[runners.machine]` |
| `docker-ssh+machine` | like `docker-ssh`, but uses [auto-scaled Docker machines](autoscale.md) - this requires the presence of `[runners.docker]` and `[runners.machine]` |
| `kubernetes` | run build using Kubernetes Pods - this requires the presence of `[runners.kubernetes]` |

## The SHELLS

There are a couple of available shells that can be run on different platforms.

| Shell | Description |
| ----- | ----------- |
| `bash`        | generate Bash (Bourne-shell) script. All commands executed in Bash context (default for all Unix systems) |
| `sh`          | generate Sh (Bourne-shell) script. All commands executed in Sh context (fallback for `bash` for all Unix systems) |
| `powershell`  | generate PowerShell script. All commands are executed in Windows PowerShell Desktop context (default for Windows) |
| `pwsh`        | generate PowerShell script. All commands are executed in PowerShell Core context |

## The `[runners.docker]` section

This defines the Docker Container parameters.

| Parameter | Description |
| --------- | ----------- |
| `host`                         | Specify custom Docker endpoint, by default `DOCKER_HOST` environment is used or `unix:///var/run/docker.sock` |
| `hostname`                     | Specify custom hostname for Docker container |
| `runtime`                      | Specify a runtime for Docker container |
| `tls_cert_path`                | When set it will use `ca.pem`, `cert.pem` and `key.pem` from that folder to make secure TLS connection to Docker (useful in boot2docker) |
| `tls_verify`                   | Enable or disable TLS verification of connections to Docker daemon. Disabled by default. |
| `image`                        | Use this image to run builds |
| `memory`                       | String value containing the memory limit |
| `memory_swap`                  | String value containing the total memory limit |
| `memory_reservation`           | String value containing the memory soft limit |
| `oom_kill_disable`             | Do not kill processes in a container if an out-of-memory (OOM) error occurs |
| `oom_score_adjust`             | OOM score adjustment, positive means kill earlier |
| `cpuset_cpus`                  | String value containing the cgroups CpusetCpus to use |
| `cpu_shares`                   | Number of CPU shares used to set relative cpu usage, default: 1024 |
| `cpus`                         | String value of number of CPUs (available in Docker 1.13 or later) |
| `dns`                          | A list of DNS servers for the container to use |
| `dns_search`                   | A list of DNS search domains |
| `privileged`                   | Make container run in Privileged mode (insecure) |
| `disable_entrypoint_overwrite` | Disable the image entrypoint overwriting |
| `userns_mode`                  | Sets the usernamespace mode for the container when usernamespace remapping option is enabled. (available in Docker 1.10 or later) |
| `cap_add`                      | Add additional Linux capabilities to the container |
| `cap_drop`                     | Drop additional Linux capabilities from the container |
| `security_opt`                 | Set security options (--security-opt in `docker run`), takes a list of ':' separated key/values |
| `devices`                      | Share additional host devices with the container |
| `cache_dir`                    | Specify where Docker caches should be stored (this can be absolute or relative to current working directory). See `disable_cache` for more information. |
| `disable_cache`                | The Docker executor has 2 levels of caching: a global one (like any other executor) and a local cache based on Docker volumes. This configuration flag acts only on the local one which disables the use of automatically created (not mapped to a host directory) cache volumes. In other words, it only prevents creating a container that holds temporary files of builds, it does not disable the cache if the Runner is configured in [distributed cache mode](autoscale.md#distributed-runners-caching). |
| `network_mode`              | Add container to a custom network |
| `wait_for_services_timeout` | Specify how long to wait for Docker services, set to 0 to disable, default: 30 |
| `volumes`                   | Specify additional volumes that should be mounted (same syntax as Docker's `-v` flag) |
| `extra_hosts`               | Specify hosts that should be defined in container environment |
| `shm_size`                  | Specify shared memory size for images (in bytes) |
| `volumes_from`              | Specify a list of volumes to inherit from another container in the form `<container name>[:<ro|rw>]`. Access level defaults to read-write, but can be manually set to `ro` (read-only) or `rw` (read-write). |
| `volume_driver`             | Specify the volume driver to use for the container |
| `links`                     | Specify containers which should be linked with building container |
| `allowed_images`            | Specify wildcard list of images that can be specified in `.gitlab-ci.yml`. If not present all images are allowed (equivalent to `["*/*:*"]`) |
| `allowed_services`          | Specify wildcard list of services that can be specified in `.gitlab-ci.yml`. If not present all images are allowed (equivalent to `["*/*:*"]`) |
| `pull_policy`               | Specify the image pull policy: `never`, `if-not-present` or `always` (default); read more in the [pull policies documentation](../executors/docker.md#how-pull-policies-work) |
| `sysctls`                   | specify the sysctl options |
| `helper_image`              | (Advanced) [Override the default helper image](#helper-image) used to clone repos and upload artifacts. |

### The `[[runners.docker.services]]` section

Specify additional services that should be run with the build. Please visit the
[Docker Registry](https://hub.docker.com) for the list of available applications.
Each service will be run in a separate container and linked to the build.

| Parameter | Description |
| --------- | ----------- |
| `name`  | The name of the image to be run as a service |
| `alias` | Additional [alias name](https://docs.gitlab.com/ee/ci/docker/using_docker_images.html#available-settings-for-services) that can be used to access the service |

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
    name = "mysql"
    alias = "db"
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

You can find the complete guide of Docker volume usage
[here](https://docs.docker.com/userguide/dockervolumes/).

Let's use some examples to explain how it work (assuming you have a working
runner).

#### Example 1: adding a data volume

A data volume is a specially-designated directory within one or more containers
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

This will create a new volume inside the container at `/path/to/volume/in/container`.

#### Example 2: mount a host directory as a data volume

In addition to creating a volume using a data volume, you can also mount
a directory from your Docker daemon's host into a container. It's useful
when you want to store directories outside the container.

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

This will use `/path/to/bind/from/host` of the CI host inside the container at
`/path/to/bind/in/container`.

NOTE: **Note:**
GitLab Runner 11.11 and newer [will mount the host
directory](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/1261)
for the defined [services](https://docs.gitlab.com/ee/ci/services/) as
well.

### Using a private container registry

> **Notes:**
>
>- This feature requires GitLab Runner **1.8** or higher
>- For GitLab Runner versions **>= 0.6, <1.8** there was a partial
>  support for using private registries, which required manual configuration
>  of credentials on runner's host. We recommend to upgrade your Runner to
>  at least version **1.8** if you want to use private registries.
>- Using private registries with the `if-not-present` pull policy may introduce
>  [security implications](../security/index.md#usage-of-private-docker-images-with-if-not-present-pull-policy). To fully understand how pull policies work,
>  read the [pull policies documentation](../executors/docker.md#how-pull-policies-work).

If you want to use private registries as a source of images for your builds,
you can set the authorization configuration in the `DOCKER_AUTH_CONFIG`
[variable](https://docs.gitlab.com/ee/ci/variables/#variables). It can be set in both GitLab Variables section of
a project and in the `config.toml` file.

For a detailed example, visit the [Using Docker images documentation](https://docs.gitlab.com/ee/ci/docker/using_docker_images.html#define-an-image-from-a-private-container-registry).

The steps performed by the Runner can be summed up to:

1. The registry name is found from the image name.
1. If the value is not empty, the executor will search for the authentication
   configuration for this registry.
1. Finally, if an authentication corresponding to the specified registry is
   found, subsequent pulls will make use of it.

Now that the Runner is set up to authenticate against your private registry,
learn [how to configure `.gitlab-ci.yml`](https://docs.gitlab.com/ee/ci/yaml/README.html#image-and-services) in order to use that
registry.

#### Support for GitLab integrated registry

> **Note:**
To work automatically with private/protected images from
GitLab integrated registry it needs at least GitLab CE/EE **8.14**
and GitLab Runner **1.8**.

Starting with GitLab CE/EE 8.14, GitLab will send credentials for its integrated
registry along with the build data. These credentials will be automatically
added to registries authorization parameters list.

After this authorization against the registry will be proceed like for
configuration added with `DOCKER_AUTH_CONFIG` variable.

Thanks to this, in your builds you can use any image from you GitLab integrated
registry, even if the image is private/protected. To fully understand for
which images the builds will have access, read the
[New CI build permissions model](https://docs.gitlab.com/ee/user/project/new_ci_build_permissions_model.html) documentation.

#### Precedence of Docker authorization resolving

As described above, GitLab Runner can authorize Docker against a registry by
using credentials sent in different way. To find a proper registry, the following
precedence is taken into account:

1. Credentials configured with `DOCKER_AUTH_CONFIG`.
1. Credentials configured locally on Runner's host with `~/.docker/config.json`
   or `~/.dockercfg` files (e.g., by running `docker login` on the host).
1. Credentials sent by default with job's payload (e.g., credentials for _integrated
   registry_ described above).

The first found credentials for the registry will be used. So for example,
if you add some credentials for the _integrated registry_ with the
`DOCKER_AUTH_CONFIG` variable, then the default credentials will be overridden.

#### Restrict `allowed_images` to private registry

For certain setups you will restrict access of the build jobs to Docker images
which comes from your private Docker registry. In that case set

```toml
[runners.docker]
  ...
  allowed_images = ["my.registry.tld:5000/*:*"]
```

## The `[runners.parallels]` section

This defines the Parallels parameters.

| Parameter | Description |
| --------- | ----------- |
| `base_name`         | name of Parallels VM which will be cloned |
| `template_name`     | custom name of Parallels VM linked template (optional) |
| `disable_snapshots` | if disabled the VMs will be destroyed after build |

Example:

```toml
[runners.parallels]
  base_name = "my-parallels-image"
  template_name = ""
  disable_snapshots = false
```

## The `[runners.virtualbox]` section

This defines the VirtualBox parameters. This executor relies on
`vboxmanage` as executable to control VirtualBox machines so you have to adjust
your `PATH` environment variable on Windows hosts:
`PATH=%PATH%;C:\Program Files\Oracle\VirtualBox`.

| Parameter | Explanation |
| --------- | ----------- |
| `base_name`         | name of VirtualBox VM which will be cloned |
| `base_snapshot`     | name or UUID of a specific snapshot of the VM from which to create a linked clone. If this is empty or omitted, the current snapshot will be used. If there is no current snapshot, one will be created unless `disable_snapshots` is true, in which case a full clone of the base VM will be made. |
| `disable_snapshots` | if disabled the VMs will be destroyed after build |

Example:

```toml
[runners.virtualbox]
  base_name = "my-virtualbox-image"
  base_snapshot = "my-image-snapshot"
  disable_snapshots = false
```

## The `[runners.ssh]` section

This defines the SSH connection parameters.

| Parameter  | Description |
| ---------- | ----------- |
| `host`     | where to connect (overridden when using `docker-ssh`) |
| `port`     | specify port, default: 22 |
| `user`     | specify user |
| `password` | specify password |
| `identity_file` | specify file path to SSH private key (id_rsa, id_dsa or id_edcsa). The file needs to be stored unencrypted |

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

This defines the Docker Machine based autoscaling feature. More details can be
found in the separate [runners autoscale documentation](autoscale.md).

| Parameter           | Description |
|---------------------|-------------|
| `IdleCount`         | Number of machines, that need to be created and waiting in _Idle_ state. |
| `IdleTime`          | Time (in seconds) for machine to be in _Idle_ state before it is removed. |
| `[[runners.machine.autoscaling]]` | Multiple sections each containing overrides for autoscaling configuration. The last section with the expression matching the current time is selected. |
| `OffPeakPeriods`    | Deprecated: Time periods when the scheduler is in the OffPeak mode. An array of cron-style patterns (described [below](#periods-syntax)). |
| `OffPeakTimezone`   | Deprecated: Timezone for the times given in OffPeakPeriods. A timezone string like `Europe/Berlin`. Defaults to the locale system setting of the host if omitted or empty. GitLab Runner attempts to locate the timezone database in the directory or uncompressed zip file named by the `ZONEINFO` environment variable, then looks in known installation locations on Unix systems, and finally looks in `$GOROOT/lib/time/zoneinfo.zip`. |
| `OffPeakIdleCount`  | Deprecated: Like `IdleCount`, but for _Off Peak_ time periods. |
| `OffPeakIdleTime`   | Deprecated: Like `IdleTime`, but for _Off Peak_ time periods. |
| `MaxBuilds`         | Builds count after which machine will be removed. |
| `MachineName`       | Name of the machine. It **must** contain `%s`, which will be replaced with a unique machine identifier. |
| `MachineDriver`     | Docker Machine `driver` to use. More details can be found in the [Docker Machine configuration section](autoscale.md#supported-cloud-providers). |
| `MachineOptions`    | Docker Machine options. More details can be found in the [Docker Machine configuration section](autoscale.md#supported-cloud-providers). |

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
  MachineDriver = "digitalocean"
  MachineOptions = [
      "digitalocean-image=coreos-stable",
      "digitalocean-ssh-user=core",
      "digitalocean-access-token=DO_ACCESS_TOKEN",
      "digitalocean-region=nyc2",
      "digitalocean-size=4gb",
      "digitalocean-private-networking",
      "engine-registry-mirror=http://10.11.12.13:12345"
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
values, ranges, lists and asterisks. A detailed description of the syntax
can be found [here](https://github.com/gorhill/cronexpr#implementation).

## The `[runners.custom]` section

Define configuration for the [custom executor](../executors/custom.md).

| Parameter               | Type         | Required | Description                                                                                                                                                                                                                                                                                         |
|-------------------------|--------------|----------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `config_exec`           | string       | ✗        | Path to an executable to allow the user to override some configuration settings before the build starts. These values will override the ones set inside of the [`[[runners]]`](#the-runners-section) section. [The custom executor documentation](../executors/custom.md#config) has the full list. |
| `config_args`           | string array | ✗        | First set of arguments passed to the `config_exec` executable.                                                                                                                                                                                                                                      |
| `config_exec_timeout`   | integer      | ✗        | Timeout in seconds for `config_exec` to finish execution. Default to 1 hour.                                                                                                                                                                                                                        |
| `prepare_exec`          | string       | ✗        | Path to an executable to prepare the environment.                                                                                                                                                                                                                                                   |
| `prepare_args`          | string array | ✗        | First set of arguments passed to the `prepare_exec` executable.                                                                                                                                                                                                                                     |
| `prepare_exec_timeout`  | integer      | ✗        | Timeout in seconds for `prepare_exec` to finish execution. Default to 1 hour.                                                                                                                                                                                                                       |
| `run_exec`              | string       | ✓        | Path to an executable to run scripts in the environments. For example, the clone and build script.                                                                                                                                                                                                  |
| `run_args`              | string array | ✗        | First set of arguments passed to the `run_exec` executable.                                                                                                                                                                                                                                         |
| `cleanup_exec`          | string       | ✗        | Path to an executable to clean up the environment.                                                                                                                                                                                                                                                  |
| `cleanup_args`          | string array | ✗        | First set of arguments passed to the `cleanup_exec` executable.                                                                                                                                                                                                                                     |
| `cleanup_exec_timeout`  | integer      | ✗        | Timeout in seconds for `cleanup_exec` to finish execution. Default to 1 hour.                                                                                                                                                                                                                       |
| `graceful_kill_timeout` | integer      | ✗        | Time to wait in seconds for `prepare_exec` and `cleanup_exec` if they are terminated (for example, during build cancellation). After this timeout, the process is killed. Defaults to 10 minutes.                                                                                                   |
| `force_kill_timeout`    | integer      | ✗        | Time to wait in seconds after the kill signal is sent to the script. Defaults to 10 minutes.                                                                                                                                                                                                        |

## The `[runners.cache]` section

> Introduced in GitLab Runner 1.1.0.

This defines the distributed cache feature. More details can be found
in the [runners autoscale documentation](autoscale.md#distributed-runners-caching).

| Parameter        | Type             | Description |
|------------------|------------------|-------------|
| `Type`           | string           | One of: `s3`, `gcs`. |
| `Path`           | string           | Name of the path to prepend to the cache URL. |
| `Shared`         | boolean          | Enables cache sharing between runners, `false` by default. |

CAUTION: **Important:**
With GitLab Runner 11.3.0, the configuration parameters related to S3 were moved to a dedicated `[runners.cache.s3]` section.
The old format of the configuration with S3 configured directly in `[runners.cache]` was deprecated with GitLab Runner 11.3.0.
**With GitLab Runner 12.0.0 the old configuration syntax was removed and is no longer supported**.

NOTE: **Note:**
The cache mechanism uses pre-signed URLs to upload and download cache. URLs are being signed by GitLab Runner on its **own instance**.
No matter if the job's script - so also the cache upload/download script - are being executed on local or external
machines (e.g. `shell` or `docker` executors are running their scripts on the same
machine where GitLab Runner process is running, while `virtualbox` or `docker+machine`
connects to a separate VM to execute the script). This is done for security reasons:
minimizing the possibility of leaking the cache adapter's credentials.

NOTE: **Note:**
Previous note implies [S3 cache adapter](#the-runnerscaches3-section), if configured to use
IAM instance profile, will use the profile attached with GitLab Runner's machine.
Similarly for [GCS cache adapter](#the-runnerscachegcs-section), if configured to
use the `CredentialsFile`, the file needs to be present on GitLab Runner's machine.

Below is a table containing a summary of `config.toml`, cli options and ENV variables for `register`:

| Setting             | TOML field                               | CLI option for `register`      | ENV for `register`                | Before 12.0.0 TOML field            | Before 12.0.0 CLI option | Before 12.0.0 ENV         |
|---------------------|------------------------------------------|--------------------------------|-----------------------------------|-------------------------------------|--------------------------|---------------------------|
| Type                | `[runners.cache] -> Type`                | `--cache-type`                 | `$CACHE_TYPE`                     |                                     |                          |                           |
| Path                | `[runners.cache] -> Path`                | `--cache-path`                 | `$CACHE_PATH`                     |                                     | `--cache-s3-cache-path`  | `$S3_CACHE_PATH`          |
| Shared              | `[runners.cache] -> Shared`              | `--cache-shared`               | `$CACHE_SHARED`                   |                                     | `--cache-cache-shared`   |                           |
| S3.ServerAddress    | `[runners.cache.s3] -> ServerAddress`    | `--cache-s3-server-address`    | `$CACHE_S3_SERVER_ADDRESS`        | `[runners.cache] -> ServerAddress`  |                          | `$S3_SERVER_ADDRESS`      |
| S3.AccessKey        | `[runners.cache.s3] -> AccessKey`        | `--cache-s3-access-key`        | `$CACHE_S3_ACCESS_KEY`            | `[runners.cache] -> AccessKey`      |                          | `$S3_ACCESS_KEY`          |
| S3.SecretKey        | `[runners.cache.s3] -> SecretKey`        | `--cache-s3-secret-key`        | `$CACHE_S3_SECRET_KEY`            | `[runners.cache] -> SecretKey`      |                          | `$S3_SECRET_KEY`          |
| S3.BucketName       | `[runners.cache.s3] -> BucketName`       | `--cache-s3-bucket-name`       | `$CACHE_S3_BUCKET_NAME`           | `[runners.cache] -> BucketName`     |                          | `$S3_BUCKET_NAME`         |
| S3.BucketLocation   | `[runners.cache.s3] -> BucketLocation`   | `--cache-s3-bucket-location`   | `$CACHE_S3_BUCKET_LOCATION`       | `[runners.cache] -> BucketLocation` |                          | `$S3_BUCKET_LOCATION`     |
| S3.Insecure         | `[runners.cache.s3] -> Insecure`         | `--cache-s3-insecure`          | `$CACHE_S3_INSECURE`              | `[runners.cache] -> Insecure`       |                          | `$S3_INSECURE`            |
| GCS.AccessID        | `[runners.cache.gcs] -> AccessID`        | `--cache-gcs-access-id`        | `$CACHE_GCS_ACCESS_ID`            |                                     |                          |                           |
| GCS.PrivateKey      | `[runners.cache.gcs] -> PrivateKey`      | `--cache-gcs-private-key`      | `$CACHE_GCS_PRIVATE_KEY`          |                                     |                          |                           |
| GCS.CredentialsFile | `[runners.cache.gcs] -> CredentialsFile` | `--cache-gcs-credentials-file` | `$GOOGLE_APPLICATION_CREDENTIALS` |                                     |                          |                           |
| GCS.BucketName      | `[runners.cache.gcs] -> BucketName`      | `--cache-gcs-bucket-name`      | `$CACHE_GCS_BUCKET_NAME`          |                                     |                          |                           |

### The `[runners.cache.s3]` section

NOTE: **Note:**
Moved from the `[runners.cache]` section in GitLab Runner 11.3.0.

Configure S3 storage for cache. This section contains settings related to S3, that previously were
present globally in the `[runners.cache]` section.

| Parameter        | Type             | Description |
|------------------|------------------|-------------|
| `ServerAddress`  | string           | A `host:port` for the S3-compatible server. If you are using a server other than AWS, consult the storage product documentation to determine the correct address. For DigitalOcean, the address must be in the format `spacename.region.digitaloceanspaces.com`. |
| `AccessKey`      | string           | The access key specified for your S3 instance. |
| `SecretKey`      | string           | The secret key specified for your S3 instance. |
| `BucketName`     | string           | Name of the storage bucket where cache will be stored. |
| `BucketLocation` | string           | Name of S3 region. |
| `Insecure`       | boolean          | Set to `true` if the S3 service is available by `HTTP`. Set to `false` by default. |

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

NOTE: **Note:**
For Amazon's S3 service, the `ServerAddress` should always be `s3.amazonaws.com`. The MinIO S3 client will
get bucket metadata and modify the URL to point to the valid region (eg. `s3-eu-west-1.amazonaws.com`) itself.

NOTE: **Note:**
If any of `ServerAddress`, `AccessKey` or `SecretKey` aren't specified, then the S3 client will use the
IAM instance profile available to the `gitlab-runner` instance. In an
[autoscale](autoscale.md) configuration, this is *NOT* the machine created on
demand that jobs are executed on.

### The `[runners.cache.gcs]` section

> Introduced in GitLab Runner 11.3.0.

Configure native support for Google Cloud Storage. Read the
[Google Cloud Storage Authentication documentation](https://cloud.google.com/storage/docs/authentication#service_accounts)
to check where these values come from.

| Parameter         | Type             | Description |
|-------------------|------------------|-------------|
| `CredentialsFile` | string           | Path to the Google JSON key file. Currently only the `service_account` type is supported. If configured, takes precedence over `AccessID` and `PrivateKey` configured directly in `config.toml`. |
| `AccessID`        | string           | ID of GCP Service Account used to access the storage. |
| `PrivateKey`      | string           | Private key used to sign GCS requests. |
| `BucketName`      | string           | Name of the storage bucket where cache will be stored. |

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

## The `[runners.kubernetes]` section

> Introduced in GitLab Runner v1.6.0.

This defines the Kubernetes parameters.
See [Kubernetes executor](../executors/kubernetes.md) for additional parameters.

| Parameter        | Type    | Description |
|------------------|---------|-------------|
| `host`           | string  | Optional Kubernetes master host URL (auto-discovery attempted if not specified) |
| `cert_file`      | string  | Optional Kubernetes master auth certificate |
| `key_file`       | string  | Optional Kubernetes master auth private key |
| `ca_file`        | string  | Optional Kubernetes master auth ca certificate |
| `image`          | string  | Default Docker image to use for builds when none is specified |
| `namespace`      | string  | Namespace to run Kubernetes jobs in |
| `privileged`     | boolean | Run all containers with the privileged flag enabled |
| `node_selector`  | table   | A `table` of `key=value` pairs of `string=string`. Setting this limits the creation of pods to Kubernetes nodes matching all the `key=value` pairs |
| `image_pull_secrets` | array | A list of secrets that are used to authenticate Docker image pulling |

Example:

```toml
[runners.kubernetes]
  host = "https://45.67.34.123:4892"
  cert_file = "/etc/ssl/kubernetes/api.crt"
  key_file = "/etc/ssl/kubernetes/api.key"
  ca_file = "/etc/ssl/kubernetes/ca.crt"
  image = "golang:1.8"
  privileged = true
  image_pull_secrets = ["docker-registry-credentials"]
  [runners.kubernetes.node_selector]
    gitlab = "true"
```

## Helper image

When one of `docker`, `docker+machine` or `kubernetes` executors is used, GitLab Runner uses a specific container
to handle Git, artifacts and cache operations. This container is created from a special image, named `helper image`.

The helper image is based on Alpine Linux and it's provided for amd64 and arm architectures. It contains
a `gitlab-runner-helper` binary which is a special compilation of GitLab Runner binary, that contains only a subset
of available commands, as well as Git, Git LFS, SSL certificates store, and basic configuration of Alpine.

When GitLab Runner is installed from the DEB/RPM packages, both images (`amd64` and `arm` based) are installed on the host.
When the Runner prepares the environment for the job execution, if the image in specified version (based on Runner's Git
revision) is not found on Docker Engine, it is automatically loaded. It works like that for both
`docker` and `docker+machine` executors.

Things work a little different for the `kubernetes` executor or when GitLab Runner is installed manually. For manual
installations, the `gitlab-runner-helper` binary is not included and for the `kubernetes` executor, the API of Kubernetes
doesn't allow loading the `gitlab-runner-helper` image from a local archive. In both cases, GitLab Runner will download
the helper image from Docker Hub, from GitLab's official repository `gitlab/gitlab-runner-helper` by using the Runner's
revision and architecture for defining which tag should be downloaded.

### Overriding the helper image

In some cases, you may need to override the helper image. There are many reasons for doing this:

1. **To speed up jobs execution**: In environments with slower internet connection, downloading over and over again the
   same image from Docker Hub may generate a significant increase of a job's timings. Downloading the helper image from
   a local registry (where the exact copy of `gitlab/gitlab-runner-helper:XYZ` is stored) may speed things up.

1. **Security concerns**: Many people don't like to download external dependencies that were not checked before. There
   might be a business rule to use only dependencies that were reviewed and stored in local repositories.

1. **Build environments without internet access**: In some cases, jobs are being executed in an environment which has
   a dedicated, closed network (this doesn't apply to the `kubernetes` executor where the image still needs to be downloaded
   from an external registry that is available at least to the Kubernetes cluster).

1. **Additional software**: Some users may want to install some additional software to the helper image, like
   `openssh` to support submodules accessible via `git+ssh` instead of `git+http`.

In any of the cases described above, it's possible to configure a custom image using the `helper_image` configuration field,
that is available for the `docker`, `docker+machine` and `kubernetes` executors:

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    helper_image = "my.registry.local/gitlab/gitlab-runner-helper:tag"
```

Note that the version of the helper image should be considered as strictly coupled with the version of GitLab Runner.
As it was described above, one of the main reasons of providing such images is that Runner is using the
`gitlab-runner-helper` binary, and this binary is compiled from part of GitLab Runner sources which is using an internal
API that is expected to be the same in both binaries.

The Runner by default references to a `gitlab/gitlab-runner-helper:XYZ` image, where `XYZ` is based
on the Runner's architecture and Git revision. Starting with **GitLab Runner 11.3** it's possible to define the version
of used image automatically, by using one of the
[version variables](https://gitlab.com/gitlab-org/gitlab-runner/blob/11-3-stable/common/version.go#L48-50):

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    helper_image = "my.registry.local/gitlab/gitlab-runner-helper:x86_64-${CI_RUNNER_REVISION}"
```

With that configuration, GitLab Runner will instruct the executor to use the image in version `x86_64-${CI_RUNNER_REVISION}`,
which is based on its compilation data. After updating the Runner to a new version, this will ensure that the
Runner will try to download the proper image. This of course means that the image should be uploaded to the registry
before upgrading the Runner, otherwise the jobs will start failing with a "No such image" error.

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

## The `[runners.custom_build_dir]` section

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/1267) in GitLab Runner 11.10.

This section defines [custom build directories](https://docs.gitlab.com/ee/ci/yaml/README.html#custom-build-directories) parameters.

Please notice, that the feature - if not configured explicitly - will be
enabled by default for `kubernetes`, `docker`, `docker-ssh`, `docker+machine`
and `docker-ssh+machine` executors. It will be disabled by default for all other
executors.

This feature requires that `GIT_CLONE_PATH` is within a path defined
within `runners.builds_dir`. For the ease of using `builds_dir` the
`$CI_BUILDS_DIR` variable can be used.

The feature is by default enabled only for `docker` and `kubernetes` executors
as they provide a good way to separate resources. This feature can be
explicitly enabled for any executor, but special care should be taken when using
with executors that share `builds_dir` and have `concurrent > 1`.

| Parameter | Type    | Description |
|-----------|---------|-------------|
| `enabled` | boolean | Allow user to define a custom build directory for a job |

Example:

```toml
[runners.custom_build_dir]
  enabled = true
```

## The `[runners.referees]` section

> - [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/1545) in GitLab Runner 12.7.
> - Requires [GitLab v12.6](https://about.gitlab.com/releases/2019/12/22/gitlab-12-6-released/) or later.

Use Runner Referees to pass extra job monitoring data to GitLab. Runner referees are special workers within the Runner manager that query and collect additional data related to a job and upload their results to GitLab as job artifacts.

### Using the Metrics Runner Referee

If the machine/container that is running the job exposes [Prometheus](https://prometheus.io) metrics that are gathered by a Prometheus server, GitLab Runner can query the Prometheus server for the entirety of the job duration. After the metrics are received, they are uploaded as a job artifact which can be used for analysis later.

Currently, only the [`docker-machine` executor](../executors/docker_machine.md) supports the referee.

### Configuring the Metrics Runner Referee for a Runner

Define `[runner.referees]` and `[runner.referees.metrics]` in your `config.toml` file within a `[[runner]]` section and add the following fields:

| Setting              | Description                                                                                                                         |
| -------------------- | ----------------------------------------------------------------------------------------------------------------------------------- |
| `prometheus_address` | The server that collects metrics from Runner instances. It must be accessible by the Runner manager when the job finishes.          |
| `query_interval`     | The frequency the Prometheus instance associated with a job is queried for time series data, defined as an interval (in seconds).   |
| `queries`            | An array of [PromQL](https://prometheus.io/docs/prometheus/latest/querying/basics/) queries that will be executed for each interval. |

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
| `{selector}` | Replaced with a `label_name=label_value` pair that selects metrics generated by a specific Runner instance within Prometheus. |
| `{interval}` | Replaced with the `query_interval` parameter from the `[runners.referees.metrics]` configuration for this referee.            |

For example, a shared Runner environment using the `docker-machine` executor would have a `{selector}` similar to `node=shared-runner-123`.

## Note

If you'd like to deploy to multiple servers using GitLab CI, you can create a
single script that deploys to multiple servers or you can create many scripts.
It depends on what you'd like to do.
