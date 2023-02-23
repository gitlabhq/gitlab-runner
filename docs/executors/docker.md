---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Docker executor **(FREE)**

GitLab Runner uses the Docker executor to run jobs on Docker images.

You can use the Docker executor to:

- Maintain the same build environment for each job.
- Use the same image to test commands locally without the requirement of running a job in the CI server.

The Docker executor uses [Docker Engine](https://www.docker.com/products/container-runtime/)
to run each job in a separate and isolated container. To connect to Docker Engine, the executor uses:

- The image and services you define in [`.gitlab-ci.yml`](https://docs.gitlab.com/ee/ci/yaml/index.html).
- The configurations you define in [`config.toml`](../commands/index.md#configuration-file).

## Docker executor workflow

The Docker executor uses a special Docker image based on [Alpine Linux](https://alpinelinux.org/) that
contains the tools to run the prepare, pre-job, and post-job steps. To view the definition of
the special Docker image, see the [GitLab Runner repository](https://gitlab.com/gitlab-org/gitlab-runner/-/tree/v13.4.1/dockerfiles/runner-helper).

The Docker executor divides the job into several steps:

1. **Prepare**: Creates and starts the [services](https://docs.gitlab.com/ee/ci/yaml/#services).
1. **Pre-job**: Clones, restores [cache](https://docs.gitlab.com/ee/ci/yaml/#cache),
   and downloads [artifacts](https://docs.gitlab.com/ee/ci/yaml/#artifacts) from previous
   stages. Runs on a special Docker image.
1. **Job**: Runs your build in the Docker image you configure for the runner.
1. **Post-job**: Create cache, upload artifacts to GitLab. Runs on
   a special Docker Image.

## Supported configurations

The Docker executor supports the following configurations.

For known issues and additional requirements of Windows configurations, see [Use Windows containers](#use-windows-containers).

| Runner is installed on:  | Executor is:     | Container is running: |
|--------------------------|------------------|------------------------|
| Windows                  | `docker-windows` | Windows                |
| Windows                  | `docker`         | Linux                  |
| Linux                    | `docker`         | Linux                  |

These configurations are **not** supported:

| Runner is installed on:  | Executor is:     | Container is running: |
|--------------------------|------------------|------------------------|
| Linux                    | `docker-windows` | Linux                  |
| Linux                    | `docker`         | Windows                |
| Linux                    | `docker-windows` | Windows                |
| Windows                  | `docker`         | Windows                |
| Windows                  | `docker-windows` | Linux                  |

NOTE:
GitLab Runner uses Docker Engine API
[v1.25](https://docs.docker.com/engine/api/v1.25/) to talk to the Docker
Engine. This means the
[minimum supported version](https://docs.docker.com/engine/api/#api-version-matrix)
of Docker on a Linux server is `1.13.0`,
[on Windows Server it needs to be more recent](#supported-docker-versions)
to identify the Windows Server version.

## Use the Docker executor

To use the Docker executor, define Docker as the executor in `config.toml`.

The following sample shows Docker defined as the executor and example
configurations. For more information about these values, see [Advanced configuration](../configuration/advanced-configuration.md)

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

To configure the Docker executor, you define the Docker images and services in [`.gitlab-ci.yml`](https://docs.gitlab.com/ee/ci/yaml/index.html) and [`config.toml`](../commands/index.md#configuration-file).

Use the following keywords:

- `image`: The name of the Docker image that the runner uses to run jobs.
  - Enter an image from the local Docker Engine, or any image in
  Docker Hub. For more information, see the [Docker documentation](https://docs.docker.com/get-started/overview/).
  - To define the image version, use a colon (`:`) to add a tag. If you don't specify a tag,
   Docker uses `latest` as the version.
- `services`: The additional image that creates another container and links to the `image`. For more information about types of services, see [Services](https://docs.gitlab.com/ee/ci/services/).

### Define images and services in `.gitlab-ci.yml`

Define an image that the runner uses for all jobs and a list of
services to use during build time.

Example:

```yaml
image: ruby:2.7

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

test:2.6:
  image: ruby:2.6
  services:
  - postgres:9.3
  script:
  - bundle exec rake spec

test:2.7:
  image: ruby:2.7
  services:
  - postgres:9.4
  script:
  - bundle exec rake spec
```

If you don't define an `image` in `.gitlab-ci.yml`, the runner uses the `image` defined in `config.toml`.

### Define images and services in `config.toml`

To add images and services to all jobs run by a runner, update `[runners.docker]` in the `config.toml`.
If you don't define an `image` in `.gitlab-ci.yml`, the runner uses the image defined in `config.toml`.

Example:

```toml
[runners.docker]
  image = "ruby:2.7"

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

- To access images from a private registry, you must [authenticate GitLab Runner](https://docs.gitlab.com/ee/ci/docker/using_docker_images.html#access-an-image-from-a-private-container-registry).

To define an image from a private registry, provide the registry name and the image in `.gitlab-ci.yml`.

Example:

```yaml
image: my.registry.tld:5000/namepace/image:tag
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
about user-defined bridge networks, see the [Docker documentation](https://docs.docker.com/network/bridge/).

To use this networking mode, enable `FF_NETWORK_PER_BUILD` in either
the feature flag or the environment variable in the`config.toml`.

Do not set the `network_mode`.

Example:

```toml
[[runners]]
  (...)
  executor = "docker"
  environment = ["FF_NETWORK_PER_BUILD = 1"]
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
[`dockerd`](https://docs.docker.com/engine/reference/commandline/dockerd/). If CIDR ranges
are already used in the network, Docker networks may conflict with other networks on the host,
including other Docker networks.

This feature works only when the Docker daemon is configured with IPv6 enabled.
To enable IPv6 support, set `enable_ipv6` to `true` in the Docker configuration.
For more information, see the [Docker documentation](https://docs.docker.com/config/daemon/ipv6/).

The runner uses the `build` alias to resolve the job container.

#### How the runner creates a network for each job

When a job starts, the runner:

1. Creates a bridge network, similar to the Docker command `docker network create <network>`.
1. Connects the service and containers to the bridge network.
1. Removes the network at the end of the job.

The container running the job and the containers running the service
resolve each other's hostnames and aliases. This functionality is
[provided by Docker](https://docs.docker.com/network/bridge/#differences-between-user-defined-bridges-and-the-default-bridge).

### Configure a network with container links

You can configure a network mode that uses Docker [legacy container links](https://docs.docker.com/network/links/) and the default Docker `bridge` to link the job container with the services. This network mode is the default
if [`FF_NETWORK_PER_BUILD`](#create-a-network-for-each-job) is not enabled.

To configure the network, specify the [networking mode](https://docs.docker.com/engine/reference/run/#network-settings) in the `config.toml` file:

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

## Restrict Docker images and services

To restrict Docker images and services, specify a wildcard pattern in the `allowed_images` and `allowed_services` parameters.

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
[Docker documentation](https://docs.docker.com/engine/reference/commandline/run/#mount-tmpfs-tmpfs).

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

- [Using PostgreSQL](https://docs.gitlab.com/ee/ci/services/postgres.html)
- [Using MySQL](https://docs.gitlab.com/ee/ci/services/mysql.html)

### How GitLab Runner performs the services health check

After the service starts, GitLab Runner waits for the service to
respond. The Docker executor tries to open a TCP connection to
the first exposed service in the service container.

To see how this is implemented, use the health check [Go command](https://gitlab.com/gitlab-org/gitlab-runner/blob/main/commands/helpers/health_check.go).

## Specify Docker driver operations

Specify arguments to supply to the Docker volume driver when you create volumes for builds.
For example, you can use these arguments to limit the space for each build to run, in addition to all other driver specific options.
The following example shows a `config.toml` where the limit that each build can consume is set to 50GB.

```toml
[runners.docker]
  [runners.docker.volume_driver_ops]
      "size" = "50G"
```

## Configure directories for the container build and cache

To define where data is stored in the container, configure `/builds` and `/cache`
directories in the `[[runners]]` section in `config.toml`.

If you modify the `/cache` storage path, to mark the path as persistent you must define it in `volumes = ["/my/cache/"]`, under the
`[runners.docker]` section in `config.toml`.

By default, the Docker executor stores builds and caches in the following directories:

- Builds in `/builds/<namespace>/<project-name>`
- Caches in `/cache` inside the container.

## Clear the Docker cache

> Introduced in GitLab Runner 13.9, [all created runner resources cleaned up](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/2310).

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

1. To remove all unused containers, networks, images (dangling and unreferenced), and untagged volumes, run [`docker system prune`](https://docs.docker.com/engine/reference/commandline/system_prune/).

## Persistent storage

The Docker executor provides persistent storage when it runs containers.
All directories defined in `volumes =` are persistent between builds.

The `volumes` directive supports the following types of storage:

- For dynamic storage, use `<path>`. The `<path>` is persistent between subsequent runs of the same concurrent job for that project. The data is attached to a custom cache volume: `runner-<short-token>-project-<id>-concurrent-<concurrency-id>-cache-<md5-of-path>`.
- For host-bound storage, use `<host-path>:<path>[:<mode>]`. The `<path>` is bound to `<host-path>` on the host system. The optional `<mode>` specifies that this storage is read-only or read-write (default).

### Persistent storage for builds

If you make the `/builds` directory a host-bound storage, your builds are stored in:
`/builds/<short-token>/<concurrent-id>/<namespace>/<project-name>`, where:

- `<short-token>` is a shortened version of the Runner's token (first 8 letters).
- `<concurrent-id>` is a unique number that identifies the local job ID of the
  particular runner in context of the project.

## IPC mode

The Docker executor supports sharing the IPC namespace of containers with other
locations. This maps to the `docker run --ipc` flag.
More details on [IPC settings in Docker documentation](https://docs.docker.com/engine/reference/run/#ipc-settings---ipc)

## Privileged mode

The Docker executor supports a number of options that allows fine-tuning of the
build container. One of these options is the [`privileged` mode](https://docs.docker.com/engine/reference/run/#runtime-privilege-and-linux-capabilities).

### Use Docker-in-Docker with privileged mode

The configured `privileged` flag is passed to the build container and all
services, thus allowing to easily use the Docker-in-Docker approach.

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

## Configure a Docker ENTRYPOINT

By default the Docker executor doesn't override the [`ENTRYPOINT` of a Docker image](https://docs.docker.com/engine/reference/run/#entrypoint-default-command-to-execute-at-runtime) and passes `sh` or `bash` as [`COMMAND`](https://docs.docker.com/engine/reference/run/#cmd-default-command-or-options) to start a container that runs the job script.

To ensure a job can run, its Docker image must:

- Provide `sh` or `bash`
- Define an `ENTRYPOINT` that starts a shell when passed `sh`/`bash` as argument

The Docker Executor runs the job's container with an equivalent of the following command:

```shell
docker run <image> sh -c "echo 'It works!'" # or bash
```

If your Docker image doesn't support this mechanism, you can [override the image's ENTRYPOINT](https://docs.gitlab.com/ee/ci/yaml/#imageentrypoint) in the project configuration as follows:

```yaml
# Equivalent of
# docker run --entrypoint "" <image> sh -c "echo 'It works!'"
image:
  name: my-image
  entrypoint: [""]
```

For more information, see [Override the Entrypoint of an image](https://docs.gitlab.com/ee/ci/docker/using_docker_images.html#override-the-entrypoint-of-an-image) and [How CMD and ENTRYPOINT interact in Docker](https://docs.docker.com/engine/reference/builder/#understand-how-cmd-and-entrypoint-interact).

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

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27119) in GitLab 15.3.

If you have GitLab Runner installed on Linux, your jobs can use Podman to replace Docker as
the container runtime in the Docker executor.

Prerequisites:

- [Podman](https://podman.io/) v4.2.0 or later.
- To run [services](#services) with Podman as an executor, enable the
  [`FF_NETWORK_PER_BUILD` feature flag](#create-a-network-for-each-job).
  [Docker container links](https://docs.docker.com/network/links/) are legacy
  and are not supported by [Podman](https://podman.io/). For services that
  create a network alias, you must install the `podman-plugins` package.

1. On your Linux host, install GitLab Runner. If you installed GitLab Runner
   by using your system's package manager, it automatically creates a `gitlab-runner` user.
1. Sign in as the user that will run GitLab Runner. You must do so in a way that
   doesn't go around [`pam_systemd`](https://www.freedesktop.org/software/systemd/man/pam_systemd.html).
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

1. Copy the socket string in the `Listen` key through which Podman's API is being accessed.
1. Make sure the Podman socket remains available after the GitLab Runner user is logged out:

   ```shell
   sudo loginctl enable-linger gitlab-runner
   ``` 

1. Edit the GitLab Runner `config.toml` file and add the socket value to the host entry in the `[[runners.docker]]` section.
   For example:

   ```toml
   [[runners]]
     name = "podman-test-runner-2022-06-07"
     url = "https://gitlab.com"
     token = "x-XxXXXXX-xxXxXxxxxx"
     executor = "docker"
     [runners.docker]
       host = "unix:///run/user/1012/podman/podman.sock"
       tls_verify = false
       image = "quay.io/podman/stable"
       privileged = true
   ```

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

## Specify which user runs the job

By default, the runner runs jobs as the `root` user within the container. To specify a different, non-root user to run the job, use the `USER` directive in the Dockerfile of the Docker image.

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

- [`always`](#set-the-always-pull-policy): Pull an image even if a local image exists. Default.
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
  a shared runner in your GitLab instance.

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

- For shared runners where different users that use the runner may have access to private images.
  For more information about security issues, see
  [Usage of private Docker images with if-not-present pull policy](../security/index.md#usage-of-private-docker-images-with-if-not-present-pull-policy).
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
    pull_policy = "if-not-present"
```

### Set multiple pull policies

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/26558) in GitLab Runner 13.8.

You can list multiple pull policies to execute if a pull fails. The runner processes pull policies
in the order listed until a pull attempt is successful or the list is exhausted. For example, if a
runner uses the `always` pull policy and the registry is not available, you can add the `if-not-present`
as a second pull policy to use a locally cached Docker image.

For information about the security implications of this pull policy, see
[Usage of private Docker images with if-not-present pull policy](../security/index.md#usage-of-private-docker-images-with-if-not-present-pull-policy).

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

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/26753) in GitLab 15.1.

In the `.gitlab-ci.yml` file, you can specify a pull policy. This policy determines how a CI/CD job
fetches images.

To restrict which pull policies can be used in the `.gitlab-ci.yml` file, use `allowed_pull_policies`.

For example, to allow only the `always` and `if-not-present` pull policies, add them to the `config.toml`:

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    allowed_pull_policies = ["always", "if-not-present"]
```

- If you don't specify `allowed_pull_policies`, the default is the value in the `pull_policy` keyword.
- If you don't specify `pull_policy`, the default is `always`.
- The existing [`pull_policy` keyword](../executors/docker.md#configure-how-runners-pull-images) must not
  include a pull policy that is not specified in `allowed_pull_policies`. If it does, the job returns an error.

### Image pull error messages

| Error message               | Description                  |
|-----------------------------|------------------------------|
| `Pulling docker image registry.tld/my/image:latest ... ERROR: Build failed: Error: image registry.tld/my/image:latest not found`  |  The runner cannot find the image. Displays when the `always` pull policy is set  |
| `Pulling docker image local_image:latest ... ERROR: Build failed: Error: image local_image:latest not found`   | The image was built locally and doesn't exist in any public or default Docker registry. Displays when the `always` pull policy is set.   |
| `Pulling docker image registry.tld/my/image:latest ... WARNING: Cannot pull the latest version of image registry.tld/my/image:latest : Error: image registry.tld/my/image:latest not found WARNING: Locally found image will be used instead.` | The runner has used a local image instead of pulling an image. Displays when the `always` pull policy is set in only [GitLab Runner 1.8 and earlier](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/1905).  |
| `Pulling docker image local_image:latest ... ERROR: Build failed: Error: image local_image:latest not found` | The image cannot be found locally. Displays when the `never` pull policy is set. |
| `WARNING: Failed to pull image with policy "always": Error response from daemon: received unexpected HTTP status: 502 Bad Gateway (docker.go:143:0s) Attempt #2: Trying "if-not-present" pull policy Using locally found image version due to "if-not-present" pull policy`| The runner failed to pull an image and attempts to pull an image by using the next listed pull policy. Displays when multiple pull policies are set. |

## Retry a failed pull

To configure a runner to retry a failed image pull, specify the same policy more than once in the
`config.toml`.

For example, this configuration retries the pull one time:

```toml
[runners.docker]
  pull_policy = ["always", "always"]
```

This setting is similar to [the `retry` directive](https://docs.gitlab.com/ee/ci/yaml/#retry)
in the `.gitlab-ci.yml` files of individual projects,
but only takes effect if specifically the Docker pull fails initially.

## Docker vs Docker-SSH (and Docker+Machine vs Docker-SSH+Machine)

WARNING:
Starting with GitLab Runner 10.0, both Docker-SSH and Docker-SSH+machine executors
are **deprecated** and will be removed in one of the upcoming releases.

We provided a support for a special type of Docker executor, namely Docker-SSH
(and the autoscaled version: Docker-SSH+Machine). Docker-SSH uses the same logic
as the Docker executor, but instead of executing the script directly, it uses an
SSH client to connect to the build container.

Docker-SSH then connects to the SSH server that is running inside the container
using its internal IP.

This executor is no longer maintained and will be removed in the near future.

## Use Windows containers

> [Introduced](https://gitlab.com/groups/gitlab-org/-/epics/535) in GitLab Runner 11.11.

To use Windows containers with the Docker executor, note the following
information about limitations, supported Windows versions, and
configuring a Windows Docker executor.

### Nanoserver support

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/2492) in GitLab Runner 13.6.

With the support for PowerShell Core introduced in the Windows helper image, it is now possible to leverage
the `nanoserver` variants for the helper image.

### Limitations of Docker executor on Windows

The following are some limitations of using Windows containers with
Docker executor:

- Docker-in-Docker is not supported, since it's
  [not supported](https://github.com/docker-library/docker/issues/49) by
  Docker itself.
- Interactive web terminals are not supported.
- Host device mounting not supported.
- When mounting a volume directory it has to exist, or Docker will fail
  to start the container, see
  [#3754](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/3754) for
  additional detail.
- `docker-windows` executor can be run only using GitLab Runner running
  on Windows.
- [Linux containers on Windows](https://learn.microsoft.com/en-us/virtualization/windowscontainers/deploy-containers/linux-containers)
  are not supported, since they are still experimental. Read
  [the relevant issue](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4373) for
  more details.
- Because of a [limitation in Docker](https://github.com/MicrosoftDocs/Virtualization-Documentation/issues/334),
  if the destination path drive letter is not `c:`, paths are not supported for:

  - [`builds_dir`](../configuration/advanced-configuration.md#the-runners-section)
  - [`cache_dir`](../configuration/advanced-configuration.md#the-runners-section)
  - [`volumes`](../configuration/advanced-configuration.md#volumes-in-the-runnersdocker-section)

  This means values such as `f:\\cache_dir` are not supported, but `f:` is supported.
  However, if the destination path is on the `c:` drive, paths are also supported
  (for example `c:\\cache_dir`).

### Supported Windows versions

GitLab Runner only supports the following versions of Windows which
follows our [support lifecycle for Windows](../install/windows.md#windows-version-support-policy):

- Windows Server 21H1/LTSC2022.
- Windows Server 20H2.
- Windows Server 2004.
- Windows Server 1809.

For future Windows Server versions, we have a
[future version support policy](../install/windows.md#windows-version-support-policy).

You can only run containers based on the same OS version that the Docker
daemon is running on. For example, the following [`Windows Server Core`](https://hub.docker.com/_/microsoft-windows-servercore) images can
be used:

- `mcr.microsoft.com/windows/servercore:ltsc2022`
- `mcr.microsoft.com/windows/servercore:ltsc2022-amd64`
- `mcr.microsoft.com/windows/servercore:20H2`
- `mcr.microsoft.com/windows/servercore:20H2-amd64`
- `mcr.microsoft.com/windows/servercore:2004`
- `mcr.microsoft.com/windows/servercore:2004-amd64`
- `mcr.microsoft.com/windows/servercore:1809`
- `mcr.microsoft.com/windows/servercore:1809-amd64`
- `mcr.microsoft.com/windows/servercore:ltsc2019`

### Supported Docker versions

A Windows Server running GitLab Runner must be running a recent version of Docker
because GitLab Runner uses Docker to detect what version of Windows Server is running.

A known version of Docker that doesn't work with GitLab Runner is `Docker 17.06`
since Docker does not identify the version of Windows Server resulting in the
following error:

```plaintext
unsupported Windows Version: Windows Server Datacenter
```

[Read more about troubleshooting this](../install/windows.md#docker-executor-unsupported-windows-version).

### Configure a Windows Docker executor

NOTE:
When a runner is registered with `c:\\cache`
as a source directory when passing the `--docker-volumes` or
`DOCKER_VOLUMES` environment variable, there is a
[known issue](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4312).

Below is an example of the configuration for a simple Docker
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

In [GitLab Runner 12.9 and later](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/1042),
you can use [services](https://docs.gitlab.com/ee/ci/services/) by
enabling [a network for each job](#create-a-network-for-each-job).
