---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: Run GitLab Runner in a Docker container.
title: Run GitLab Runner in a container
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

You can run GitLab Runner in a Docker container to execute CI/CD jobs. The GitLab Runner Docker image includes all dependencies needed to:

- Run GitLab Runner.
- Execute CI/CD jobs in containers.

The GitLab Runner Docker images use [Ubuntu or Alpine Linux](#docker-images) as their base. They wrap the standard `gitlab-runner` command, similar to installing GitLab Runner directly on the host.

The `gitlab-runner` command runs in a Docker container.
This setup delegates full control over the Docker daemon to each GitLab Runner container.
The effect is that isolation guarantees break if you run GitLab Runner inside a Docker daemon
that also runs other payloads.

In this setup, every GitLab Runner command you run has a `docker run` equivalent, like this:

- Runner command: `gitlab-runner <runner command and options...>`
- Docker command: `docker run <chosen docker options...> gitlab/gitlab-runner <runner command and options...>`

For example, to get the top-level help information for GitLab Runner, replace the `gitlab-runner` part
of the command with `docker run [docker options] gitlab/gitlab-runner`, like this:

```shell
docker run --rm -t -i gitlab/gitlab-runner --help

NAME:
   gitlab-runner - a GitLab Runner

USAGE:
   gitlab-runner [global options] command [command options] [arguments...]

VERSION:
   17.9.1 (bbf75488)

(...)
```

## Docker Engine version compatibility

The versions for the Docker Engine and GitLab Runner container image
do not have to match. The GitLab Runner images are backwards and forwards compatible.
To ensure you have the latest features and security updates,
you should always use the latest stable [Docker Engine version](https://docs.docker.com/engine/install/).

## Install the Docker image and start the container

Prerequisites:

- You have [installed Docker](https://docs.docker.com/get-started/get-docker/).
- You have read the [FAQ](../faq/_index.md) to learn about common problems in GitLab Runner.

1. Download the `gitlab-runner` Docker image by using the `docker pull gitlab/gitlab-runner:<version-tag>` command.

   For the list of available version tags, see [GitLab Runner tags](https://hub.docker.com/r/gitlab/gitlab-runner/tags).
1. Run the `gitlab-runner` Docker image by using the `docker run -d [options] <image-uri> <runner-command>` command.
1. When you run `gitlab-runner` in a Docker container, ensure the configuration is not lost when you
   restart the container. Mount a permanent volume to store the configuration. The volume can be mounted in either:

   - [A local system volume](#from-a-local-system-volume)
   - [A Docker volume](#from-a-docker-volume)

1. Optional. If using a [`session_server`](../configuration/advanced-configuration.md), expose port `8093`
   by adding `-p 8093:8093` to your `docker run` commands.
1. Optional. To use the Docker Machine executor for autoscaling, mount the Docker Machine
   storage path (`/root/.docker/machine`) by adding a volume mount to your `docker run` commands:

   - For system volume mounts, add `-v /srv/gitlab-runner/docker-machine-config:/root/.docker/machine`
   - For Docker named volumes, add `-v docker-machine-config:/root/.docker/machine`

1. [Register a new runner](../register/_index.md). The GitLab Runner container must be registered to pick up jobs.

Some available configuration options include:

- Set the container's time zone with the flag `--env TZ=<TIMEZONE>`.
  [See a list of available time zones](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones).
- For a [FIPS compliant GitLab Runner](requirements.md#fips-compliant-gitlab-runner) image, based on
  `redhat/ubi9-micro`, use the `gitlab/gitlab-runner:ubi-fips` tags.
- [Install trusted SSL server certificates](#install-trusted-ssl-server-certificates).

### From a local system volume

To use your local system for the configuration volume and other resources mounted into the `gitlab-runner` container:

1. Optional. In MacOS systems, `/srv` does not exist by default. Create `/private/srv`, or another private directory, for setup.
1. Run this command, modifying it as needed:

   ```shell
   docker run -d --name gitlab-runner --restart always \
     -v /srv/gitlab-runner/config:/etc/gitlab-runner \
     -v /var/run/docker.sock:/var/run/docker.sock \
     gitlab/gitlab-runner:latest
   ```

### From a Docker volume

To use a configuration container to mount your custom data volume:

1. Create the Docker volume:

   ```shell
   docker volume create gitlab-runner-config
   ```

1. Start the GitLab Runner container using the volume you just created:

   ```shell
   docker run -d --name gitlab-runner --restart always \
     -v /var/run/docker.sock:/var/run/docker.sock \
     -v gitlab-runner-config:/etc/gitlab-runner \
     gitlab/gitlab-runner:latest
   ```

## Update runner configuration

After you [change the runner configuration](../configuration/advanced-configuration.md) in `config.toml`,
apply your changes by restarting the container with `docker stop` and `docker run`.

## Upgrade runner version

Prerequisites:

- You must use the same method for mounting your data volume as you did originally
  (`-v /srv/gitlab-runner/config:/etc/gitlab-runner` or `-v gitlab-runner-config:/etc/gitlab-runner`).

1. Pull the latest version (or a specific tag):

   ```shell
   docker pull gitlab/gitlab-runner:latest
   ```

1. Stop and remove the existing container:

   ```shell
   docker stop gitlab-runner && docker rm gitlab-runner
   ```

1. Start the container as you did originally:

   ```shell
   docker run -d --name gitlab-runner --restart always \
     -v /var/run/docker.sock:/var/run/docker.sock \
     -v /srv/gitlab-runner/config:/etc/gitlab-runner \
     gitlab/gitlab-runner:latest
   ```

## View runner logs

Log file locations depend on how you start a runner. When you start it as a:

- **Foreground task**, either as a locally installed binary or in a Docker container,
  the logs print to `stdout`.
- **System service**, like with `systemd`, the logs are available in the system logging mechanism, like Syslog.
- **Docker-based service**, use the `docker logs` command, as the `gitlab-runner ...` command is
  the main process of the container.

For example, if you start a container with this command, its name is set to `gitlab-runner`:

```shell
docker run -d --name gitlab-runner --restart always \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /srv/gitlab-runner/config:/etc/gitlab-runner \
  gitlab/gitlab-runner:latest
```

To view its logs, run this command, replacing `gitlab-runner` with your container name:

```shell
docker logs gitlab-runner
```

For more information about handling container logs, see
[`docker container logs`](https://docs.docker.com/reference/cli/docker/container/logs/) in the Docker documentation.

## Install trusted SSL server certificates

If your GitLab CI/CD server uses self-signed SSL certificates, make sure your
runner container trusts the GitLab CI server certificate. This prevents communication failures.

Prerequisites:

- Your `ca.crt` file should contain the root certificates of all the servers you
  want GitLab Runner to trust.

1. Optional. The `gitlab/gitlab-runner` image looks for trusted SSL certificates in `/etc/gitlab-runner/certs/ca.crt`.
   To change this behavior, use the `-e "CA_CERTIFICATES_PATH=/DIR/CERT"` configuration option.
1. Copy your `ca.crt` file into the `certs` directory on the data volume (or container).
1. Optional. If your container is already running, restart it to import the `ca.crt` file on startup.

## Docker images

In GitLab Runner 18.8.0, the Docker image based on Alpine uses Alpine 3.21. These multi-platform Docker images are available:

- `gitlab/gitlab-runner:latest` based on Ubuntu, approximately 800 MB.
- `gitlab/gitlab-runner:alpine` based on Alpine, approximately 460 MB.

See the [GitLab Runner](https://gitlab.com/gitlab-org/gitlab-runner/tree/main/dockerfiles)
source for possible build instructions for both Ubuntu and Alpine images.

### Create a runner Docker image

You can upgrade your image's operating system before the update is available in the GitLab repositories.

Prerequisites:

- You are not using the IBM Z image, as it does not contain the `docker-machine` dependency. This image is
  not maintained for the Linux s390x or Linux ppc64le platforms. For the current status, see
  [issue 26551](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/26551).

To build a `gitlab-runner` Docker image for the latest Alpine version:

1. Create `alpine-upgrade/Dockerfile`.

   ```dockerfile
   ARG GITLAB_RUNNER_IMAGE_TYPE
   ARG GITLAB_RUNNER_IMAGE_TAG
   FROM gitlab/${GITLAB_RUNNER_IMAGE_TYPE}:${GITLAB_RUNNER_IMAGE_TAG}

   RUN apk update
   RUN apk upgrade
   ```

1. Create an upgraded `gitlab-runner` image.

   ```shell
   GITLAB_RUNNER_IMAGE_TYPE=gitlab-runner \
   GITLAB_RUNNER_IMAGE_TAG=alpine-v17.9.1 \
   docker build -t $GITLAB_RUNNER_IMAGE_TYPE:$GITLAB_RUNNER_IMAGE_TAG \
     --build-arg GITLAB_RUNNER_IMAGE_TYPE=$GITLAB_RUNNER_IMAGE_TYPE \
     --build-arg GITLAB_RUNNER_IMAGE_TAG=$GITLAB_RUNNER_IMAGE_TAG \
     -f alpine-upgrade/Dockerfile alpine-upgrade
   ```

1. Create an upgraded `gitlab-runner-helper` image.

   ```shell
   GITLAB_RUNNER_IMAGE_TYPE=gitlab-runner-helper \
   GITLAB_RUNNER_IMAGE_TAG=x86_64-v17.9.1 \
   docker build -t $GITLAB_RUNNER_IMAGE_TYPE:$GITLAB_RUNNER_IMAGE_TAG \
     --build-arg GITLAB_RUNNER_IMAGE_TYPE=$GITLAB_RUNNER_IMAGE_TYPE \
     --build-arg GITLAB_RUNNER_IMAGE_TAG=$GITLAB_RUNNER_IMAGE_TAG \
     -f alpine-upgrade/Dockerfile alpine-upgrade
   ```

## Use SELinux in your container

Some distributions, like CentOS, Red Hat, and Fedora use SELinux (Security-Enhanced Linux) by default to
enhance the security of the underlying system.

Use caution with this configuration.

Prerequisites:

- To use the [Docker executor](../executors/docker.md) to run builds in containers, runners need
  access to `/var/run/docker.sock`.
- If you use SELinux in enforcing mode, install [`selinux-dockersock`](https://github.com/dpw/selinux-dockersock)
  to prevent a `Permission denied` error when a runner accesses `/var/run/docker.sock`.

1. Create a persistent directory on the host: `mkdir -p /srv/gitlab-runner/config`.
1. Run Docker with `:Z` on volumes:

   ```shell
   docker run -d --name gitlab-runner --restart always \
     -v /var/run/docker.sock:/var/run/docker.sock \
     -v /srv/gitlab-runner/config:/etc/gitlab-runner:Z \
     gitlab/gitlab-runner:latest
   ```
