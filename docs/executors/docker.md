---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Docker executor
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

GitLab Runner uses the Docker executor to run jobs on Docker images.

You can use the Docker executor to:

- Maintain the same build environment for each job.
- Use the same image to test commands locally without the requirement of running a job in the CI server.

The Docker executor uses [Docker Engine](https://www.docker.com/products/container-runtime/)
to run each job in a separate and isolated container. To connect to Docker Engine, the executor uses:

- The image and services you define in [`.gitlab-ci.yml`](https://docs.gitlab.com/ci/yaml/).
- The configurations you define in [`config.toml`](../commands/_index.md#configuration-file).

You can't register a runner and its Docker executor without defining a default image in `config.toml`.
The image defined in `config.toml` can be used when none is defined in `.gitlab-ci.yml`.
If an image is defined in `.gitlab-ci.yml`, it overrides the one defined in `config.toml`.

Prerequisites:

- [Install Docker](https://docs.docker.com/engine/install/).

## Docker executor workflow

The Docker executor uses a Docker image based on [Alpine Linux](https://alpinelinux.org/) that
contains the tools to run the prepare, pre-job, and post-job steps. To view the definition of
the special Docker image, see the [GitLab Runner repository](https://gitlab.com/gitlab-org/gitlab-runner/-/tree/v13.4.1/dockerfiles/runner-helper).

The Docker executor divides the job into several steps:

1. **Prepare**: Creates and starts the [services](https://docs.gitlab.com/ci/yaml/#services).
1. **Pre-job**: Clones, restores [cache](https://docs.gitlab.com/ci/yaml/#cache),
   and downloads [artifacts](https://docs.gitlab.com/ci/yaml/#artifacts) from previous
   stages. Runs on a special Docker image.
1. **Job**: Runs your build in the Docker image you configure for the runner.
1. **Post-job**: Create cache, upload artifacts to GitLab. Runs on
   a special Docker Image.

## Supported configurations

The Docker executor supports the following configurations.

For known issues and additional requirements of Windows configurations, see [Use Windows containers](#use-windows-containers).

| Runner is installed on: | Executor is:     | Container is running: |
|-------------------------|------------------|-----------------------|
| Windows                 | `docker-windows` | Windows               |
| Windows                 | `docker`         | Linux                 |
| Linux                   | `docker`         | Linux                 |
| macOS                   | `docker`         | Linux                 |

These configurations are **not** supported:

| Runner is installed on: | Executor is:     | Container is running: |
|-------------------------|------------------|-----------------------|
| Linux                   | `docker-windows` | Linux                 |
| Linux                   | `docker`         | Windows               |
| Linux                   | `docker-windows` | Windows               |
| Windows                 | `docker`         | Windows               |
| Windows                 | `docker-windows` | Linux                 |

{{< alert type="note" >}}

GitLab Runner uses Docker Engine API
[v1.25](https://docs.docker.com/reference/api/engine/version/v1.25/) to talk to the Docker
Engine. This means the
[minimum supported version](https://docs.docker.com/reference/api/engine/#api-version-matrix)
of Docker on a Linux server is `1.13.0`.
On Windows Server, [it needs to be more recent](#supported-docker-versions)
to identify the Windows Server version.

{{< /alert >}}

## Use the Docker executor

To use the Docker executor, manually define Docker as the executor in `config.toml` or use the
[`gitlab-runner register --executor "docker"`](../register/_index.md#register-with-a-runner-authentication-token)
command to automatically define it.

The following sample configuration shows Docker defined as the executor. For more information about these values, see [Advanced configuration](../configuration/advanced-configuration.md)

```toml
concurrent = 4

[[runners]]
name = "myRunner"
url = "https://gitlab.com/ci"
token = "......"
executor = "docker"
[runners.docker]
  tls_verify = true
  image = "my.registry.tld:5000/alpine:latest"
  privileged = false
  disable_entrypoint_overwrite = false
  oom_kill_disable = false
  disable_cache = false
  volumes = [
    "/cache",
  ]
  shm_size = 0
  allowed_pull_policies = ["always", "if-not-present"]
  allowed_images = ["my.registry.tld:5000/*:*"]
  allowed_services = ["my.registry.tld:5000/*:*"]
  [runners.docker.volume_driver_ops]
    "size" = "50G"
```

## Configure images and services

Prerequisites:

- The image where your job runs must have a working shell in its operating system `PATH`. Supported shells are:
  - For Linux:
    - `sh`
    - `bash`
    - PowerShell Core (`pwsh`). [Introduced in 13.9](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4021).
  - For Windows:
    - PowerShell (`powershell`)
    - PowerShell Core (`pwsh`). [Introduced in 13.6](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/13139).

To configure the Docker executor, you define the Docker images and services in [`.gitlab-ci.yml`](https://docs.gitlab.com/ci/yaml/) and [`config.toml`](../commands/_index.md#configuration-file).

Use the following keywords:

- `image`: The name of the Docker image that the runner uses to run jobs.
  - Enter an image from the local Docker Engine, or any image in
    Docker Hub. For more information, see the [Docker documentation](https://docs.docker.com/get-started/introduction/).
  - To define the image version, use a colon (`:`) to add a tag. If you don't specify a tag,
    Docker uses `latest` as the version.
- `services`: The additional image that creates another container and links to the `image`. For more information about types of services, see [Services](https://docs.gitlab.com/ci/services/).

### Define images and services in `.gitlab-ci.yml`

Define an image that the runner uses for all jobs and a list of
services to use during build time.

Example:

```yaml
image: ruby:3.3

services:
  - postgres:9.3

before_script:
  - bundle install

test:
  script:
  - bundle exec rake spec
```

To define different images and services per job:

```yaml
before_script:
  - bundle install

test:3.3:
  image: ruby:3.3
  services:
  - postgres:9.3
  script:
  - bundle exec rake spec

test:3.4:
  image: ruby:3.4
  services:
  - postgres:9.4
  script:
  - bundle exec rake spec
```

If you don't define an `image` in `.gitlab-ci.yml`, the runner uses the `image` defined in `config.toml`.

### Define images and services in `config.toml`

To add images and services to all jobs run by a runner, update `[runners.docker]` in the `config.toml`.

By default, the Docker executer uses the `image` defined in `.gitlab-ci.yml`. If you don't define one in `.gitlab-ci.yml`, the runner uses the image defined in `config.toml`.

Example:

```toml
[runners.docker]
  image = "ruby:3.3"

[[runners.docker.services]]
  name = "mysql:latest"
  alias = "db"

[[runners.docker.services]]
  name = "redis:latest"
  alias = "cache"
```

This example uses the [array of tables syntax](https://toml.io/en/v0.4.0#array-of-tables).

### Define an image from a private registry

Prerequisites:

- To access images from a private registry, you must [authenticate GitLab Runner](https://docs.gitlab.com/ci/docker/using_docker_images/#access-an-image-from-a-private-container-registry).

To define an image from a private registry, provide the registry name and the image in `.gitlab-ci.yml`.

Example:

```yaml
image: my.registry.tld:5000/namespace/image:tag
```

In this example, GitLab Runner searches the registry `my.registry.tld:5000` for the
image `namespace/image:tag`.

## Network configurations

You must configure a network to connect services to a CI/CD job.

To configure a network, you can either:

- Recommended. Configure the runner to create a network for each job.
- Define container links. Container links are a legacy feature of Docker.

### Create a network for each job

You can configure the runner to create a network for each job.

When you enable this networking mode, the runner creates and uses a
user-defined Docker bridge network for each job. Docker environment
variables are not shared across the containers. For more information
about user-defined bridge networks, see the [Docker documentation](https://docs.docker.com/engine/network/drivers/bridge/).

To use this networking mode, enable `FF_NETWORK_PER_BUILD` in either
the feature flag or the environment variable in the `config.toml`.

Do not set the `network_mode`.

Example:

```toml
[[runners]]
  (...)
  executor = "docker"
  environment = ["FF_NETWORK_PER_BUILD=1"]
```

Or:

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.feature_flags]
    FF_NETWORK_PER_BUILD = true
```

To set the default Docker address pool, use `default-address-pool` in
[`dockerd`](https://docs.docker.com/reference/cli/dockerd/). If CIDR ranges
are already used in the network, Docker networks may conflict with other networks on the host,
including other Docker networks.

This feature works only when the Docker daemon is configured with IPv6 enabled.
To enable IPv6 support, set `enable_ipv6` to `true` in the Docker configuration.
For more information, see the [Docker documentation](https://docs.docker.com/engine/daemon/ipv6/).

The runner uses the `build` alias to resolve the job container.

{{< alert type="note" >}}

DNS might not work correctly with a Docker-in-Docker (`dind`) service when you use this feature.

This behavior is due to an issue with [Docker/Moby](https://github.com/moby/moby/issues/20037#issuecomment-181659049),
where `dind` containers don't inherit custom DNS entries when you specify a network.

As a workaround, manually provide the custom DNS settings to the `dind` service. For example,
if your custom DNS server is `1.1.1.1`, you can use `127.0.0.11`, which is Docker's internal DNS service:

```yaml
  services:
    - name: docker:dind
      command: [--dns=127.0.0.11, --dns=1.1.1.1]
```

This approach also allows containers to resolve services on the same network.

{{< /alert >}}

#### How the runner creates a network for each job

When a job starts, the runner:

1. Creates a bridge network, similar to the Docker command `docker network create <network>`.
1. Connects the service and containers to the bridge network.
1. Removes the network at the end of the job.

The container running the job and the containers running the service
resolve each other's hostnames and aliases. This functionality is
[provided by Docker](https://docs.docker.com/engine/network/drivers/bridge/#differences-between-user-defined-bridges-and-the-default-bridge).

### Configure a network with container links

GitLab Runner before 18.7.0 uses the default Docker `bridge` along with [legacy container links](https://docs.docker.com/engine/network/links/) to link the job container with the services. Because Docker deprecated the links functionality, in GitLab Runner 18.7.0 and later, the legacy container link behavior is emulated by allowing service aliases to be resolved using Docker's `extra_hosts` functionality. This network mode is the default if [`FF_NETWORK_PER_BUILD`](#create-a-network-for-each-job) is disabled.

{{< alert type="note" >}}

GitLab Runner's emulated links behavior differs slightly from [legacy container links](https://docs.docker.com/engine/network/links/):

- Disabling `icc` disables inter-container communication and containers cannot communicate with each other.
- Environment variables for the linked containers are no longer present (`<name>_PORT_<port>_<protocol>`).

{{< /alert >}}

To configure the network, specify the [networking mode](https://docs.docker.com/engine/containers/run/#network-settings) in the `config.toml` file:

- `bridge`: Use the bridge network. Default.
- `host`: Use the host's network stack inside the container.
- `none`: No networking. Not recommended.

Example:

```toml
[[runners]]
  (...)
  executor = "docker"
[runners.docker]
  network_mode = "bridge"
```

If you use any other `network_mode` value, these are taken as the name of an already existing
Docker network, which the build container connects to.

During name resolution, Docker updates the `/etc/hosts` file in the
container with the service container hostname and alias. However,
the service container is **not** able to resolve the container
name. To resolve the container name, you must create a network for each job.

Linked containers share their environment variables.

#### Overriding the MTU of the created network

For some environments, like virtual machines in OpenStack, a custom MTU is necessary.
The Docker daemon does not respect the MTU in `docker.json` (see [Moby issue 34981](https://github.com/moby/moby/issues/34981)).
You can set `network_mtu` in your `config.toml` to any valid value so
the Docker daemon can use the correct MTU for the newly created network.
You must also enable [`FF_NETWORK_PER_BUILD`](#create-a-network-for-each-job) for the override to take effect.

The following configuration sets the MTU to `1402` for the network created for each job.
Make sure to adjust the value to your specific environment requirements.

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    network_mtu = 1402
    [runners.feature_flags]
      FF_NETWORK_PER_BUILD = true
```

## Restrict Docker images and services

To restrict Docker images and services, specify a wildcard pattern in the `allowed_images` and `allowed_services` parameters. For more details on syntax, see [doublestar documentation](https://github.com/bmatcuk/doublestar).

For example, to allow images from your private Docker registry only:

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    allowed_images = ["my.registry.tld:5000/*:*"]
    allowed_services = ["my.registry.tld:5000/*:*"]
```

To restrict to a list of images from your private Docker registry:

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    allowed_images = ["my.registry.tld:5000/ruby:*", "my.registry.tld:5000/node:*"]
    allowed_services = ["postgres:9.4", "postgres:latest"]
```

To exclude specific images like Kali:

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    allowed_images = ["**", "!*/kali*"]
```

## Access services hostnames

To access a service hostname, add the service to `services` in `.gitlab-ci.yml`.

For example, to use a Wordpress instance to test an API integration with your application,
use [tutum/wordpress](https://hub.docker.com/r/tutum/wordpress/) as the service image:

```yaml
services:
- tutum/wordpress:latest
```

When the job runs, the `tutum/wordpress` service starts. You can
access it from your build container under the hostname `tutum__wordpress`
and `tutum-wordpress`.

In addition to the specified service aliases, the runner assigns the name of the service image as an alias to the service container. You can use any of these aliases.

The runner uses the following rules to create the alias based on the image name:

- Everything after `:` is stripped.
- For the first alias, the slash (`/`) is replaced with double underscores (`__`).
- For the second alias, the slash (`/`) is replaced with a single dash (`-`).

If you use a private service image, the runner strips any specified port and applies the rules.
The service `registry.gitlab-wp.com:4999/tutum/wordpress` results in the hostname
`registry.gitlab-wp.com__tutum__wordpress` and `registry.gitlab-wp.com-tutum-wordpress`.

## Configuring services

To change database names or set account names, you can define environment variables
for the service.

When the runner passes variables:

- Variables are passed to all containers. The runner cannot pass variables to specific
  containers.
- Secure variables are passed to the build container.

For more information about configuration variables, see the documentation of each image
provided in their corresponding Docker Hub page.

### Mount a directory in RAM

You can use the `tmpfs` option to mount a directory in RAM. This speeds up the time
required to test if there is a lot of I/O related work, such as with databases.

If you use the `tmpfs` and `services_tmpfs` options in the runner configuration,
you can specify multiple paths, each with its own options. For more information, see the
[Docker documentation](https://docs.docker.com/reference/cli/docker/container/run/#tmpfs).

For example, to mount the data directory for the official MySQL container in RAM,
configure the `config.toml`:

```toml
[runners.docker]
  # For the main container
  [runners.docker.tmpfs]
      "/var/lib/mysql" = "rw,noexec"

  # For services
  [runners.docker.services_tmpfs]
      "/var/lib/mysql" = "rw,noexec"
```

### Building a directory in a service

GitLab Runner mounts a `/builds` directory to all shared services.

For more information about using different services see:

- [Using PostgreSQL](https://docs.gitlab.com/ci/services/postgres/)
- [Using MySQL](https://docs.gitlab.com/ci/services/mysql/)

### How GitLab Runner performs the services health check

{{< history >}}

- [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4079) multiple port checks in GitLab 16.0.

{{< /history >}}

After the service starts, GitLab Runner waits for the service to
respond. The Docker executor tries to open a TCP connection to the
exposed service port in the service container.

- In GitLab 15.11 and earlier, only the first exposed port is checked.
- In GitLab 16.0 and later, the first 20 exposed ports are checked.

The `HEALTHCHECK_TCP_PORT` service variable can be used to perform the health check on a specific port:

```yaml
job:
  services:
    - name: mongo
      variables:
        HEALTHCHECK_TCP_PORT: "27017"
```

To see how this is implemented, use the health check [Go command](https://gitlab.com/gitlab-org/gitlab-runner/blob/main/commands/helpers/health_check.go).

## Specify Docker driver operations

Specify arguments to supply to the Docker volume driver when you create volumes for builds.
For example, you can use these arguments to limit the space for each build to run, in addition to all other driver specific options.
The following example shows a `config.toml` where the limit that each build can consume is set to 50 GB.

```toml
[runners.docker]
  [runners.docker.volume_driver_ops]
      "size" = "50G"
```

## Using host devices

{{< history >}}

- [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/6208) in GitLab 17.10.

{{< /history >}}

You can expose hardware devices on the GitLab Runner host to the container that runs the job.
To do this, configure the runner's `devices` and `services_devices` options.

- To expose devices to `build` and
  [helper](../configuration/advanced-configuration.md#helper-image) containers, use the `devices` option.
- To expose devices to services containers, use the `services_devices` option.
  To restrict a service container's device access to specific images, use exact image names or glob patterns.
  This action prevents direct access to host system devices.

For more information on device access, see [Docker documentation](https://docs.docker.com/reference/cli/docker/container/run/#device).

### Build container example

In this example, the `config.toml` section exposes `/dev/bus/usb` to build containers.
This configuration allows pipelines to access USB devices attached to the host
machine, such as Android smartphones controlled over the
[Android Debug Bridge (`adb`)](https://developer.android.com/tools/adb).

Since build job containers can directly access host USB devices, simultaneous
pipeline executions may conflict with each other when accessing the same hardware.
To prevent these conflicts, use [`resource_group`](https://docs.gitlab.com/ci/yaml/#resource_group).

```toml
[[runners]]
  name = "hardware-runner"
  url = "https://gitlab.com"
  token = "__REDACTED__"
  executor = "docker"
  [runners.docker]
    # All job containers may access the host device
    devices = ["/dev/bus/usb"]
```

### Private registry example

This example shows how to expose `/dev/kvm` and `/dev/dri` devices to container images from a private
Docker registry. These devices are commonly used for hardware-accelerated virtualization and rendering.
To mitigate risks involved with providing users direct access to hardware resources,
restrict device access to trusted images in the `myregistry:5000/emulator/*` namespace:

```toml
[runners.docker]
  [runners.docker.services_devices]
    # Only images from an internal registry may access the host devices
    "myregistry:5000/emulator/*" = ["/dev/kvm", "/dev/dri"]
```

> [!warning]
> The image name `**/*` might expose devices to any image.

## Configure directories for the container build and cache

To define where data is stored in the container, configure `/builds` and `/cache`
directories in the `[[runners]]` section in `config.toml`.

If you modify the `/cache` storage path, to mark the path as persistent you must define it in `volumes = ["/my/cache/"]`, under the
`[runners.docker]` section in `config.toml`.

By default, the Docker executor stores builds and caches in the following directories:

- Builds in `/builds/<namespace>/<project-name>`
- Caches in `/cache` inside the container.

## Clear the Docker cache

Use [`clear-docker-cache`](https://gitlab.com/gitlab-org/gitlab-runner/blob/main/packaging/root/usr/share/gitlab-runner/clear-docker-cache) to remove unused containers and volumes created by the runner.

For a list of options, run the script with the `help` option:

```shell
clear-docker-cache help
```

The default option is `prune-volumes`, which removes all unused containers (dangling and unreferenced)
and volumes.

To manage cache storage efficiently, you should:

- Run `clear-docker-cache` with `cron` regularly (for example, once a week).
- Maintain some recent containers in the cache for performance while you
  reclaim disk space.

The `FILTER_FLAG` environment variable controls which objects are pruned. For example usage, see the
[Docker image prune](https://docs.docker.com/reference/cli/docker/image/prune/#filter) documentation.

## Clear Docker build images

The [`clear-docker-cache`](https://gitlab.com/gitlab-org/gitlab-runner/blob/main/packaging/root/usr/share/gitlab-runner/clear-docker-cache) script does not remove Docker images because they are not tagged by the GitLab Runner.

To clear Docker build images:

1. Confirm what disk space can be reclaimed:

   ```shell
   clear-docker-cache space

   Show docker disk usage
   ----------------------

   TYPE            TOTAL     ACTIVE    SIZE      RECLAIMABLE
   Images          14        9         1.306GB   545.8MB (41%)
   Containers      19        18        115kB     0B (0%)
   Local Volumes   0         0         0B        0B
   Build Cache     0         0         0B        0B
   ```

1. To remove all unused containers, networks, images (dangling and unreferenced), and untagged volumes, run [`docker system prune`](https://docs.docker.com/reference/cli/docker/system/prune/).

## Persistent storage

The Docker executor provides persistent storage when it runs containers.
All directories defined in `volumes =` are persistent between builds.

The `volumes` directive supports the following types of storage:

- For dynamic storage, use `<path>`. The `<path>` is persistent between
  subsequent runs of the same concurrent job for that project. If you
  don't set `runners.docker.cache_dir`, the data persists in Docker volumes.
  Otherwise, it persists in the configured directory on the host (mounted into
  the build container).

  Volume names for volume-based persistent storage:

  - For GitLab Runner before 18.4.0: `runner-<short-token>-project-<project-id>-concurrent-<concurrency-id>-cache-<md5-of-path>`
  - For GitLab Runner 18.4.0 and later: `runner-<runner-id-hash>-cache-<md5-of-path><protection>`

    Data that is no longer human readable in the volume name is moved to the volume's labels.

  Host directories for host-based persistent storage:

  - For GitLab Runner before 18.4.0: `<cache-dir>/runner-<short-token>-project-<project-id>-concurrent-<concurrency-id>/<md5-of-path>`
  - For GitLab Runner 18.4.0 and later: `<cache-dir>/runner-<runner-id-hash>/<md5-of-path><protection>`

  Description of the variable parts:

  - `<short-token>`: The shortened version of the runner's token (first 8 letters)
  - `<project-id>`: The ID of the GitLab project
  - `<concurrency-id>`: The index of the runner (from the list of all runners running a build for the same project concurrently)
  - `<md5-of-path>`: The MD5 sum of the path within the container
  - `<runner-id-hash>`: The hash for the following data:
    - Runner's token
    - Runner's system ID
    - `<project-id>`
    - `<concurrency-id>`
  - `<protection>`: The value is empty for builds on unprotected branches, and `-protected` for protected branch builds
  - `<cache-dir>`: The configuration in `runners.docker.cache_dir`

- For host-bound storage, use `<host-path>:<path>[:<mode>]`. GitLab Runner binds the `<path>`
  to `<host-path>` on the host system. The optional `<mode>` specifies whether this storage
  is read-only or read-write (default).

{{< alert type="warning" >}}

With GitLab Runner 18.4.0, the naming of sources for dynamic storage (see above) changed
for both Docker volume-based and host directory-based persistent storage. When you upgrade
to 18.4.0, GitLab Runner ignores the cached data from previous runner versions and creates
new dynamic storage on-demand, either through new Docker volumes or new host directories.

Host-bound storage (with a `<host-path>` configuration), in contrast to dynamic
storage, is not affected.

{{< /alert >}}

### Persistent storage for builds

If you make the `/builds` directory a host-bound storage, your builds are stored in:
`/builds/<short-token>/<concurrent-id>/<namespace>/<project-name>`, where:

- `<short-token>` is a shortened version of the Runner's token (first 8 letters).
- `<concurrent-id>` is a unique number that identifies the local job ID of the
  particular runner in context of the project.

## IPC mode

The Docker executor supports sharing the IPC namespace of containers with other
locations. This maps to the `docker run --ipc` flag.
More details on [IPC settings in Docker documentation](https://docs.docker.com/engine/containers/run/#ipc-settings---ipc)

## Privileged mode

The Docker executor supports several options that allows fine-tuning of the
build container. One of these options is the [`privileged` mode](https://docs.docker.com/engine/containers/run/#runtime-privilege-and-linux-capabilities).

### Use Docker-in-Docker with privileged mode

The configured `privileged` flag is passed to the build container and all
services. With this flag, you can use the Docker-in-Docker approach.

First, configure your runner (`config.toml`) to run in `privileged` mode:

```toml
[[runners]]
  executor = "docker"
  [runners.docker]
    privileged = true
```

Then, make your build script (`.gitlab-ci.yml`) to use Docker-in-Docker
container:

```yaml
image: docker:git
services:
- docker:dind

build:
  script:
  - docker build -t my-image .
  - docker push my-image
```

{{< alert type="warning" >}}

Containers that run in privileged mode have security risks.
When your containers run in privileged mode, you disable the
container security mechanisms and expose your host to privilege escalation.
Running containers in privileged mode can lead to container breakout. For more information,
see the Docker documentation about
[runtime privilege and Linux capabilities](https://docs.docker.com/engine/containers/run/#runtime-privilege-and-linux-capabilities).

{{< /alert >}}

You might need to
[configure Docker in Docker with TLS, or disable TLS](https://docs.gitlab.com/ci/docker/using_docker_build/#use-the-docker-executor-with-docker-in-docker)
to avoid an error similar to the following:

```plaintext
Cannot connect to the Docker daemon at tcp://docker:2375. Is the docker daemon running?
```

### Use rootless Docker-in-Docker with restricted privileged mode

In this version, only Docker-in-Docker rootless images are allowed to run as services in privileged mode.

The `services_privileged` and `allowed_privileged_services` configuration parameters
limit which containers are allowed to run in privileged mode.

To use rootless Docker-in-Docker with restricted privileged mode:

1. In the `config.toml`, configure the runner to use `services_privileged` and `allowed_privileged_services`:

   ```toml
   [[runners]]
     executor = "docker"
     [runners.docker]
       services_privileged = true
       allowed_privileged_services = ["docker.io/library/docker:*-dind-rootless", "docker.io/library/docker:dind-rootless", "docker:*-dind-rootless", "docker:dind-rootless"]
   ```

1. In `.gitlab-ci.yml`, edit your build script to use Docker-in-Docker rootless container:

   ```yaml
   image: docker:git
   services:
   - docker:dind-rootless

   build:
     script:
     - docker build -t my-image .
     - docker push my-image
   ```

Only the Docker-in-Docker rootless images you list in `allowed_privileged_services` are allowed to run in privileged mode.
All other containers for jobs and services run in unprivileged mode.

Because they run as non-root, it's _almost safe_ to use with privileged mode
images like Docker-in-Docker rootless or BuildKit rootless.

For more information about security issues,
see [Security risks for Docker executors](../security/_index.md#usage-of-docker-executor).

## Configure a Docker ENTRYPOINT

By default, the Docker executor doesn't override the [`ENTRYPOINT` of a Docker image](https://docs.docker.com/engine/containers/run/#entrypoint-default-command-to-execute-at-runtime). It passes `sh` or `bash` as [`COMMAND`](https://docs.docker.com/engine/containers/run/#cmd-default-command-or-options) to start a container that runs the job script.

To ensure a job can run, its Docker image must:

- Provide `sh` or `bash` and `grep`
- Define an `ENTRYPOINT` that starts a shell when passed `sh`/`bash` as argument

The Docker Executor runs the job's container with an equivalent of the following command:

```shell
docker run <image> sh -c "echo 'It works!'" # or bash
```

If your Docker image doesn't support this mechanism, you can [override the image's ENTRYPOINT](https://docs.gitlab.com/ci/yaml/#imageentrypoint) in the project configuration as follows:

```yaml
# Equivalent of
# docker run --entrypoint "" <image> sh -c "echo 'It works!'"
image:
  name: my-image
  entrypoint: [""]
```

For more information, see [Override the Entrypoint of an image](https://docs.gitlab.com/ci/docker/using_docker_images/#override-the-entrypoint-of-an-image) and [How `CMD` and `ENTRYPOINT` interact in Docker](https://docs.docker.com/reference/dockerfile/#understand-how-cmd-and-entrypoint-interact).

### Job script as ENTRYPOINT

You can use `ENTRYPOINT` to create a Docker image that
runs the build script in a custom environment, or in secure mode.

For example, you can create a Docker image that uses an `ENTRYPOINT` that doesn't
execute the build script. Instead, the Docker image executes a predefined set of commands
to build the Docker image from your directory. You run
the build container in [privileged mode](#privileged-mode), and secure
the build environment of the runner.

1. Create a new Dockerfile:

   ```dockerfile
   FROM docker:dind
   ADD / /entrypoint.sh
   ENTRYPOINT ["/bin/sh", "/entrypoint.sh"]
   ```

1. Create a bash script (`entrypoint.sh`) that is used as the `ENTRYPOINT`:

   ```shell
   #!/bin/sh

   dind docker daemon
       --host=unix:///var/run/docker.sock \
       --host=tcp://0.0.0.0:2375 \
       --storage-driver=vf &

   docker build -t "$BUILD_IMAGE" .
   docker push "$BUILD_IMAGE"
   ```

1. Push the image to the Docker registry.

1. Run Docker executor in `privileged` mode. In `config.toml` define:

   ```toml
   [[runners]]
     executor = "docker"
     [runners.docker]
       privileged = true
   ```

1. In your project use the following `.gitlab-ci.yml`:

   ```yaml
   variables:
     BUILD_IMAGE: my.image
   build:
     image: my/docker-build:image
     script:
     - Dummy Script
   ```

## Use Podman to run Docker commands

{{< history >}}

- [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27119) in GitLab 15.3.

{{< /history >}}

If you have GitLab Runner installed on Linux, your jobs can use Podman to replace Docker as
the container runtime in the Docker executor.

Prerequisites:

- [Podman](https://podman.io/) v4.2.0 or later.
- To run [services](#services) with Podman as an executor, enable the
  [`FF_NETWORK_PER_BUILD` feature flag](#create-a-network-for-each-job).
  [Docker container links](https://docs.docker.com/engine/network/links/) are legacy
  and are not supported by [Podman](https://podman.io/). For services that
  create a network alias, you must install the `podman-plugins` package.

{{< alert type="note" >}}

Podman uses `aardvark-dns` as the DNS server for containers.
The `aardvark-dns` versions 1.10.0 and earlier cause sporadic DNS resolution failures in CI/CD jobs.
Make sure that you have installed a newer version.
For more information, see [GitHub issue 389](https://github.com/containers/aardvark-dns/issues/389).

{{< /alert >}}

1. On your Linux host, install GitLab Runner. If you installed GitLab Runner
   by using your system's package manager, it automatically creates a `gitlab-runner` user.
1. Sign in as the user who runs GitLab Runner. You must do so in a way that
   doesn't go around [`pam_systemd`](https://www.freedesktop.org/software/systemd/man/latest/pam_systemd.html).
   You can use SSH with the correct user. This ensures you can run `systemctl` as this user.
1. Make sure that your system fulfills the prerequisites for
   [a rootless Podman setup](https://github.com/containers/podman/blob/main/docs/tutorials/rootless_tutorial.md).
   Specifically, make sure your user has
   [correct entries in `/etc/subuid` and `/etc/subgid`](https://github.com/containers/podman/blob/main/docs/tutorials/rootless_tutorial.md#etcsubuid-and-etcsubgid-configuration).
1. On the Linux host, [install Podman](https://podman.io/getting-started/installation).
1. Enable and start the Podman socket:

   ```shell
   systemctl --user --now enable podman.socket
   ```

1. Verify the Podman socket is listening:

   ```shell
   systemctl status --user podman.socket
   ```

1. Copy the socket string in the `Listen` key through which the Podman API is being accessed.
1. Make sure the Podman socket remains available after the GitLab Runner user is logged out:

   ```shell
   sudo loginctl enable-linger gitlab-runner
   ```

1. Edit the GitLab Runner `config.toml` file and add the socket value to the host entry in the `[runners.docker]` section.
   For example:

   ```toml
   [[runners]]
     name = "podman-test-runner-2025-06-07"
     url = "https://gitlab.com"
     token = "TOKEN"
     executor = "docker"
     [runners.docker]
       host = "unix:///run/user/1012/podman/podman.sock"
       tls_verify = false
       image = "quay.io/podman/stable"
       privileged = false
   ```

   {{< alert type="note" >}}

   Set `privileged = false` for standard Podman usage. Set `privileged = true` only if you need to run
   [Docker-in-Docker services](#use-docker-in-docker-with-privileged-mode) within your jobs.

   {{< /alert >}}

### Use Podman to build container images from a Dockerfile

The following example uses Podman to build a container image and push the image to the GitLab Container registry.

The default container image in the Runner `config.toml` is set to `quay.io/podman/stable`, so that the CI job uses that image to execute the included commands.

```yaml
variables:
  IMAGE_TAG: $CI_REGISTRY_IMAGE:$CI_COMMIT_REF_SLUG

before_script:
  - podman login -u "$CI_REGISTRY_USER" -p "$CI_REGISTRY_PASSWORD" $CI_REGISTRY

oci-container-build:
  stage: build
  script:
    - podman build -t $IMAGE_TAG .
    - podman push $IMAGE_TAG
  when: manual
```

### Use Buildah to build container images from a Dockerfile

The following example shows how to use Buildah to build a container image and push the image to the GitLab Container registry.

```yaml
image: quay.io/buildah/stable

variables:
  IMAGE_TAG: $CI_REGISTRY_IMAGE:$CI_COMMIT_REF_SLUG

before_script:
  - buildah login -u "$CI_REGISTRY_USER" -p "$CI_REGISTRY_PASSWORD" $CI_REGISTRY

oci-container-build:
  stage: build
  script:
    - buildah bud -t $IMAGE_TAG .
    - buildah push $IMAGE_TAG
  when: manual
```

### Known issues

Unlike Docker, Podman enforces SELinux policies by default. While many pipelines run without issues, some may fail due to SELinux context inheritance when tools use temporary directories.

For example, the following pipeline fails under Podman:

```yaml
testing:
  image: alpine:3.20
  script:
    - apk add --no-cache python3 py3-pip
    - pip3 install --target $CI_PROJECT_DIR requests==2.28.2
```

The failure occurs because pip uses `/tmp` as a working directory. Files created in `/tmp` inherit its SELinux context, which prevents the container from modifying these files when they're moved to `$CI_PROJECT_DIR`.

**Solution:** Add `/tmp` to the volumes in the runner's `config.toml` under the `runners.docker` section:

```toml
[[runners]]
  [runners.docker]
    volumes = ["/cache", "/tmp"]
```

This addition ensures consistent SELinux contexts across the mounted directories.

#### Troubleshooting SELinux Issues

Other Podman/SELinux issues may require additional troubleshooting to identify the necessary configuration changes.

To test whether a Podman runner issue is SELinux-related, temporarily add the following directive to the runner's `config.toml` under the `runners.docker` section:

```toml
[[runners]]
  [runners.docker]
    security_opt = ["label:disable"]
```

> [!warning]
> This addition turns off SELinux enforcement in the container (which is Docker's default behavior).
> Use this configuration only for testing purposes and not as a permanent solution because it can have security implications.

#### Configure SELinux MCS

If SELinux blocks some write operations (such as reinitializing an existing Git repository), you can force a Multi-Category Security (MCS) on all containers launched by the runner:

```toml
[[runners]]
  [runners.docker]
    security_opt = ["label=level:s0:c1000"]
```

This option does not disable SELinux, but sets the container's MCS level. This approach is more secure than using `label:disable`.

> [!warning]
> Multiple containers that use the same MCS category can access the same files tagged with that category.

## Specify which user runs the job

By default, the runner runs jobs as the `root` user in the container. To specify a different, non-root user to run the job, use the `USER` directive in the Dockerfile of the Docker image.

```dockerfile
FROM amazonlinux
RUN ["yum", "install", "-y", "nginx"]
RUN ["useradd", "www"]
USER "www"
CMD ["/bin/bash"]
```

When you use that Docker image to execute your job, it runs as the specified user:

```yaml
build:
  image: my/docker-build:image
  script:
  - whoami   # www
```

## Configure how runners pull images

Configure the pull policy in the `config.toml` to define how runners pull Docker images from registries. You can set a single policy, [a list of policies](#set-multiple-pull-policies), or [allow specific pull policies](#allow-docker-pull-policies).

Use the following values for the `pull_policy`:

- [`always`](#set-the-always-pull-policy): Default. Pull an image even if a local image exists. This pull policy does not apply to images specified by their `SHA256` that already exist on disk.
- [`if-not-present`](#set-the-if-not-present-pull-policy): Pull an image only when a local version does not exist.
- [`never`](#set-the-never-pull-policy): Never pull an image and use only local images.

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    pull_policy = "always" # available: always, if-not-present, never
```

### Set the `always` pull policy

The `always` option, which is on by default, always initiates a pull before
creating the container. This option makes sure the image is up-to-date, and
prevents you from using outdated images even if a local image exists.

Use this pull policy if:

- Runners must always pull the most recent images.
- Runners are publicly available and configured for [auto-scale](../configuration/autoscale.md) or as
  an instance runner in your GitLab instance.

**Do not use** this policy if runners must use locally stored images.

Set `always` as the `pull policy` in the `config.toml`:

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    pull_policy = "always"
```

### Set the `if-not-present` pull policy

When you set the pull policy to `if-not-present`, the runner first checks
if a local image exists. If there is no local image, the runner pulls
an image from the registry.

Use the `if-not-present` policy to:

- Use local images but also pull images if a local image does not exist.
- Reduce time that runners analyze the difference in image layers for heavy and rarely updated images.
  In this case, you must manually remove the image regularly from the local Docker Engine store to
  force the image update.

**Do not use** this policy:

- For instance runners where different users that use the runner may have access to private images.
  For more information about security issues, see
  [Usage of private Docker images with if-not-present pull policy](../security/_index.md#usage-of-private-docker-images-with-if-not-present-pull-policy).
- If jobs are frequently updated and must be run in the most recent image
  version. This may result in a network load reduction that outweighs the value of frequent deletion
  of local images.

Set the `if-not-present` policy in the `config.toml`:

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    pull_policy = "if-not-present"
```

### Set the `never` pull policy

Prerequisites:

- Local images must contain an installed Docker Engine and a local copy of used images.

When you set the pull policy to `never`, image pulling is disabled. Users can only use images
that have been manually pulled on the Docker host where the runner runs.

Use the `never` pull policy:

- To control the images used by runner users.
- For private runners that are dedicated to a project that can only use specific images
  that are not publicly available on any registries.

**Do not use** the `never` pull policy for [auto-scaled](../configuration/autoscale.md)
Docker executors. The `never` pull policy is usable only when using a pre-defined cloud instance
images for chosen cloud provider.

Set the `never` policy in the `config.toml`:

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    pull_policy = "never"
```

### Set multiple pull policies

You can list multiple pull policies to execute if a pull fails. The runner processes pull policies
in the order listed until a pull attempt is successful or the list is exhausted. For example, if a
runner uses the `always` pull policy and the registry is not available, you can add the `if-not-present`
as a second pull policy. This configuration lets the runner use a locally cached Docker image.

For information about the security implications of this pull policy, see
[Usage of private Docker images with if-not-present pull policy](../security/_index.md#usage-of-private-docker-images-with-if-not-present-pull-policy).

To set multiple pull policies, add them as a list in the `config.toml`:

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    pull_policy = ["always", "if-not-present"]
```

### Allow Docker pull policies

{{< history >}}

- [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/26753) in GitLab 15.1.

{{< /history >}}

In the `.gitlab-ci.yml` file, you can specify a pull policy. This policy determines how a CI/CD job
fetches images.

To restrict which pull policies can be used from those specified in the `.gitlab-ci.yml` file, use `allowed_pull_policies`.

For example, to allow only the `always` and `if-not-present` pull policies, add them to the `config.toml`:

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    allowed_pull_policies = ["always", "if-not-present"]
```

- If you don't specify `allowed_pull_policies`, the list matches the values specified in the `pull_policy` keyword.
- If you don't specify `pull_policy`, the default is `always`.
- The job uses only the pull policies that are listed in both `pull_policy` and `allowed_pull_policies`.
  The effective pull policy is determined by comparing the policies specified in
  [`pull_policy` keyword](#configure-how-runners-pull-images)
  and `allowed_pull_policies`. GitLab uses the [intersection](https://en.wikipedia.org/wiki/Intersection_(set_theory))
  of these two policy lists.
  For example, if `pull_policy` is `["always", "if-not-present"]` and `allowed_pull_policies`
  is `["if-not-present"]`, then the job uses only `if-not-present` because it's the only pull policy defined in both lists.
- The existing `pull_policy` keyword must include at least one pull policy specified in `allowed_pull_policies`.
  The job fails if none of the `pull_policy` values match `allowed_pull_policies`.

### Image pull error messages

| Error message                                                                                                                                                                                                                                                               | Description |
|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-------------|
| `Pulling docker image registry.tld/my/image:latest ... ERROR: Build failed: Error: image registry.tld/my/image:latest not found`                                                                                                                                            | The runner cannot find the image. Displays when the `always` pull policy is set |
| `Pulling docker image local_image:latest ... ERROR: Build failed: Error: image local_image:latest not found`                                                                                                                                                                | The image was built locally and doesn't exist in any public or default Docker registry. Displays when the `always` pull policy is set. |
| `Pulling docker image registry.tld/my/image:latest ... WARNING: Cannot pull the latest version of image registry.tld/my/image:latest : Error: image registry.tld/my/image:latest not found WARNING: Locally found image will be used instead.`                              | The runner has used a local image instead of pulling an image. |
| `Pulling docker image local_image:latest ... ERROR: Build failed: Error: image local_image:latest not found`                                                                                                                                                                | The image cannot be found locally. Displays when the `never` pull policy is set. |
| `WARNING: Failed to pull image with policy "always": Error response from daemon: received unexpected HTTP status: 502 Bad Gateway (docker.go:143:0s) Attempt #2: Trying "if-not-present" pull policy Using locally found image version due to "if-not-present" pull policy` | The runner failed to pull an image and attempts to pull an image by using the next listed pull policy. Displays when multiple pull policies are set. |

## Retry a failed pull

To configure a runner to retry a failed image pull, specify the same policy more than once in the
`config.toml`.

For example, this configuration retries the pull one time:

```toml
[runners.docker]
  pull_policy = ["always", "always"]
```

This setting is similar to [the `retry` directive](https://docs.gitlab.com/ci/yaml/#retry)
in the `.gitlab-ci.yml` files of individual projects,
but only takes effect if specifically the Docker pull fails initially.

## Use Windows containers

To use Windows containers with the Docker executor, note the following
information about limitations, supported Windows versions, and
configuring a Windows Docker executor.

### Nanoserver support

With the support for PowerShell Core introduced in the Windows helper image, it is now possible to leverage
the `nanoserver` variants for the helper image.

### Known issues with Docker executor on Windows

The following are some limitations of using Windows containers with
Docker executor:

- Docker-in-Docker is not supported, because it's
  [not supported](https://github.com/docker-library/docker/issues/49) by
  Docker itself.
- Interactive web terminals are not supported.
- Host device mounting not supported.
- When mounting a volume directory it has to exist, or Docker fails
  to start the container, see
  [#3754](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/3754) for
  additional detail.
- `docker-windows` executor can be run only using GitLab Runner running
  on Windows.
- [Linux containers on Windows](https://learn.microsoft.com/en-us/virtualization/windowscontainers/deploy-containers/set-up-linux-containers)
  are not supported, because they are still experimental. Read
  [the relevant issue](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4373) for
  more details.
- Because of a [limitation in Docker](https://github.com/MicrosoftDocs/Virtualization-Documentation/pull/331),
  if the destination path drive letter is not `c:`, paths are not supported for:

  - [`builds_dir`](../configuration/advanced-configuration.md#the-runners-section)
  - [`cache_dir`](../configuration/advanced-configuration.md#the-runners-section)
  - [`volumes`](../configuration/advanced-configuration.md#volumes-in-the-runnersdocker-section)

  This means values such as `f:\\cache_dir` are not supported, but `f:` is supported.
  However, if the destination path is on the `c:` drive, paths are also supported
  (for example `c:\\cache_dir`).

  To configure where the Docker daemon keeps images and containers, update
  the `data-root` parameter in the `daemon.json` file of the Docker daemon.

  For more information, see [Configure Docker with a configuration file](https://learn.microsoft.com/en-us/virtualization/windowscontainers/manage-docker/configure-docker-daemon#configure-docker-with-a-configuration-file).

### Supported Windows versions

GitLab Runner only supports the following versions of Windows which
follows our [support lifecycle for Windows](../install/support-policy.md#windows-version-support):

- Windows Server 2022 LTSC (21H2)
- Windows Server 2019 LTSC (1809)

For future Windows Server versions, we have a
[future version support policy](../install/support-policy.md#windows-version-support).

You can only run containers based on the same OS version that the Docker
daemon is running on. For example, the following [`Windows Server Core`](https://hub.docker.com/r/microsoft/windows-servercore) images can
be used:

- `mcr.microsoft.com/windows/servercore:ltsc2022`
- `mcr.microsoft.com/windows/servercore:ltsc2022-amd64`
- `mcr.microsoft.com/windows/servercore:1809`
- `mcr.microsoft.com/windows/servercore:1809-amd64`
- `mcr.microsoft.com/windows/servercore:ltsc2019`

### Supported Docker versions

GitLab Runner uses Docker to detect what version of Windows Server is running.
Hence, a Windows Server running GitLab Runner must be running a recent version of Docker.

A known version of Docker that doesn't work with GitLab Runner is `Docker 17.06`.
Docker does not identify the version of Windows Server resulting in the
following error:

```plaintext
unsupported Windows Version: Windows Server Datacenter
```

[Read more about troubleshooting this](../install/windows.md#docker-executor-unsupported-windows-version).

### Configure a Windows Docker executor

{{< alert type="note" >}}

When a runner is registered with `c:\\cache`
as a source directory when passing the `--docker-volumes` or
`DOCKER_VOLUMES` environment variable, there is a
[known issue](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4312).

{{< /alert >}}

Below is an example of the configuration for a Docker
executor running Windows.

```toml
[[runners]]
  name = "windows-docker-2019"
  url = "https://gitlab.com/"
  token = "xxxxxxx"
  executor = "docker-windows"
  [runners.docker]
    image = "mcr.microsoft.com/windows/servercore:1809_amd64"
    volumes = ["c:\\cache"]
```

For other configuration options for the Docker executor, see the
[advanced configuration](../configuration/advanced-configuration.md#the-runnersdocker-section)
section.

### Services

You can use [services](https://docs.gitlab.com/ci/services/) by
enabling [a network for each job](#create-a-network-for-each-job).

## Native Step Runner Integration

{{< history >}}

- [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5069) in GitLab 17.6.0 behind the
  feature-flag `FF_USE_NATIVE_STEPS`, which is disabled by default.
- [Updated](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5322) in GitLab 17.9.0. GitLab Runner
  injects the `step-runner` binary into the build container and adjusts the `$PATH` environment variable accordingly.
  This enhancement makes it possible to use any image as the build image.

{{< /history >}}

The Docker executor supports running the [CI/CD steps](https://docs.gitlab.com/ci/steps/) natively by using the
`gRPC` API provided by [`step-runner`](https://gitlab.com/gitlab-org/step-runner).

To enable this mode of execution, you must specify CI/CD jobs using the `run` keyword instead of the legacy `script`
keyword. Additionally, you must enable the `FF_USE_NATIVE_STEPS` feature flag. You can enable this feature flag at
either the job or pipeline level.

```yaml
step job:
  stage: test
  variables:
    FF_USE_NATIVE_STEPS: true
  image:
    name: alpine:latest
  run:
    - name: step1
      script: pwd
    - name: step2
      script: env
    - name: step3
      script: ls -Rlah ../
```

### Known Issues

- In GitLab 17.9 and later, the build image must have the `ca-certificates` package installed or the `step-runner` will fail to pull the steps
  defined in the job. Debian-based Linux distribution for example do not install `ca-certificates` by default.

- In GitLab versions before 17.9, the build image must include a `step-runner` binary in `$PATH`. To achieve this, you can either:

  - Create your own custom build image and include the `step-runner` binary in it.
  - Use the `registry.gitlab.com/gitlab-org/step-runner:v0` image if it includes the dependencies you need to run your
    job.

- Running a step that runs a Docker container must adhere to the same configuration parameters and constraints as
  traditional `scripts`. For example, you must use [Docker-in-Docker](#use-docker-in-docker-with-privileged-mode).
- This mode of execution does not yet support running [`Github Actions`](https://gitlab.com/components/action-runner).
