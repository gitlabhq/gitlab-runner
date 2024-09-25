---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Run GitLab Runner in a container

DETAILS:
**Tier:** Free, Premium, Ultimate
**Offering:** GitLab.com, Self-managed

This page explains how to run GitLab Runner inside a Docker container.

## Docker Engine version compatibility

The version of Docker Engine and the version of the GitLab Runner container image
do not have to match. The GitLab Runner images should be backwards and forwards compatible.
To ensure you have the latest features and security updates, though,
you should always use the latest stable [Docker Engine version](https://docs.docker.com/engine/release-notes/24.0/).

## General GitLab Runner Docker image usage

GitLab Runner Docker images are based on [Ubuntu or Alpine Linux](#docker-images).
They are wrappers around the standard `gitlab-runner` command, like if you installed
GitLab Runner directly on the host.

For every GitLab Runner command that you would run like this:

```shell
gitlab-runner <runner command and options...>
```

You can use `docker run`, like this:

```shell
docker run <chosen docker options...> gitlab/gitlab-runner <runner command and options...>
```

For example, to get the top-level help information for GitLab Runner:

```shell
docker run --rm -t -i gitlab/gitlab-runner --help

NAME:
   gitlab-runner - a GitLab Runner

USAGE:
   gitlab-runner [global options] command [command options] [arguments...]

VERSION:
   16.5.0 (853330f9)

(...)
```

Replace the `gitlab-runner` part of the command with
`docker run [docker options] gitlab/gitlab-runner`, while the rest of the
command stays as described in the [register documentation](../register/index.md).
The only difference: the `gitlab-runner` command runs in a Docker container.

## Install the Docker image and start the container

Before you begin, ensure you have [installed Docker](https://docs.docker.com/get-docker/).

To run `gitlab-runner` in a Docker container, make sure the configuration isn't lost when you restart the container. To do this, there are two options, which are described below.

Make sure that you read the [FAQ](../faq/index.md) section which describes some of the most common problems with GitLab Runner.

- If you are using a [`session_server`](../configuration/advanced-configuration.md), you also
need to expose port `8093` by adding `-p 8093:8093` to your `docker run` command.
- If you want to use the Docker Machine executor for autoscaling feature, you also need to mount Docker Machine
  storage path: `/root/.docker/machine`:

  - by adding `-v /srv/gitlab-runner/docker-machine-config:/root/.docker/machine` for system volume mounts
  - by adding `-v docker-machine-config:/root/.docker/machine` for Docker named volumes

NOTE:
This setup delegates full control over the Docker daemon to each GitLab Runner container.
The effect is that isolation guarantees break if you run GitLab Runner inside a Docker daemon
that also runs other payloads.

### Option 1: Use local system volume mounts to start the Runner container

This example uses the local system for the configuration volume mounted into the `gitlab-runner` container. Use this volume for configuration and other resources:

```shell
docker run -d --name gitlab-runner --restart always \
  -v /srv/gitlab-runner/config:/etc/gitlab-runner \
  -v /var/run/docker.sock:/var/run/docker.sock \
  gitlab/gitlab-runner:latest
```

NOTE:
On macOS, `/srv` does not exist by default. Create `/private/srv`, or use another private directory.

### Option 2: Use Docker volumes to start the Runner container

In this example, you can use a configuration container to mount your custom data volume.

1. Create the Docker volume:

   ```shell
   docker volume create gitlab-runner-config
   ```

1. Start the GitLab Runner container using the volume we just created:

   ```shell
   docker run -d --name gitlab-runner --restart always \
       -v /var/run/docker.sock:/var/run/docker.sock \
       -v gitlab-runner-config:/etc/gitlab-runner \
       gitlab/gitlab-runner:latest
   ```

NOTE:
To set the container's time zone, in the `docker run` command, use the flag `--env TZ=<TIMEZONE>`. [View a list of available time zones](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones).

NOTE:
For a [FIPS compliant GitLab Runner](index.md#fips-compliant-gitlab-runner) image, based on `redhat/ubi8-minimal`, use the `gitlab/gitlab-runner:ubi-fips` tags.

### Register the runner

The final step is to [register a new runner](../register/index.md). The GitLab Runner container doesn't pick up any jobs until it's registered.

## Update configuration

If you change the configuration in `config.toml`, you might need to restart the runner to apply the change.
Use the `config.toml` configuration file to [configure runners](../configuration/advanced-configuration.md).
It's created when you register a runner.

Restart the whole container instead of using `gitlab-runner restart`:

```shell
docker restart gitlab-runner
```

## Upgrade version

Pull the latest version (or a specific tag):

```shell
docker pull gitlab/gitlab-runner:latest
```

Stop and remove the existing container:

```shell
docker stop gitlab-runner && docker rm gitlab-runner
```

Start the container as you did originally:

```shell
docker run -d --name gitlab-runner --restart always \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /srv/gitlab-runner/config:/etc/gitlab-runner \
  gitlab/gitlab-runner:latest
```

NOTE:
You need to use the same method for mounting your data volume as you
did originally (`-v /srv/gitlab-runner/config:/etc/gitlab-runner` or
`--volumes-from gitlab-runner-config`).

## Reading GitLab Runner logs

The location of the log files depend on how you start GitLab Runner. When you start it:

- **As a foreground task**, either as a locally installed binary or in a Docker container,
  the logs print to the standard output.
- **As a system service**, like with `systemd`, the logs are available in the system logging mechanism, like Syslog.
- **As a Docker-based service**, use the `docker logs` command, as the `gitlab-runner ...` command is
  the main process of the container.

For example, if you start GitLab Runner with this command:

```shell
docker run -d --name gitlab-runner --restart always \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /srv/gitlab-runner/config:/etc/gitlab-runner \
  gitlab/gitlab-runner:latest
```

Use this command to view the logs, replacing `gitlab-runner` with the name of the container:

```shell
docker logs gitlab-runner
```

The first command used the `--name` parameter to set the container name to `gitlab-runner`.

For more information about handling container logs, see
[`docker container logs`](https://docs.docker.com/reference/cli/docker/container/logs/) in the Docker documentation.

## Installing trusted SSL server certificates

If your GitLab CI/CD server is using self-signed SSL certificates, make sure your
GitLab Runner container trusts the GitLab CI server certificate. This prevents communication failures.

The `gitlab/gitlab-runner` image looks for the trusted SSL certificates in `/etc/gitlab-runner/certs/ca.crt`.
To change this behavior, use the `-e "CA_CERTIFICATES_PATH=/DIR/CERT"` configuration option.

Copy the `ca.crt` file into the `certs` directory on the data volume (or container).
The `ca.crt` file should contain the root certificates of all the servers you
want GitLab Runner to trust. The GitLab Runner container imports the `ca.crt` file on startup, so if
your container is already running, you might need to restart it for the changes to take effect.

## Docker images

The following multi-platform Docker images are available:

- `gitlab/gitlab-runner:latest` based on Ubuntu, approximately 750 MB.
- `gitlab/gitlab-runner:alpine` based on Alpine, approximately 340 MB.

See the [GitLab Runner](https://gitlab.com/gitlab-org/gitlab-runner/tree/main/dockerfiles)
source for possible build instructions for both Ubuntu and Alpine images.

### Creating a GitLab Runner Docker image

As of GitLab Runner 16.1, the GitLab Runner Docker image based on Alpine uses Alpine 3.18. However, you can upgrade the image's operating system before it's available in the GitLab repositories.

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
   GITLAB_RUNNER_IMAGE_TYPE=gitlab-runner GITLAB_RUNNER_IMAGE_TAG=alpine-v16.1.0 docker build -t $GITLAB_RUNNER_IMAGE_TYPE:$GITLAB_RUNNER_IMAGE_TAG --build-arg GITLAB_RUNNER_IMAGE_TYPE=$GITLAB_RUNNER_IMAGE_TYPE --build-arg GITLAB_RUNNER_IMAGE_TAG=$GITLAB_RUNNER_IMAGE_TAG -f alpine-upgrade/Dockerfile alpine-upgrade

   ```

1. Create an upgraded `gitlab-runner-helper` image.

   ```shell
   GITLAB_RUNNER_IMAGE_TYPE=gitlab-runner-helper GITLAB_RUNNER_IMAGE_TAG=x86_64-v16.1.0 docker build -t $GITLAB_RUNNER_IMAGE_TYPE:$GITLAB_RUNNER_IMAGE_TAG --build-arg GITLAB_RUNNER_IMAGE_TYPE=$GITLAB_RUNNER_IMAGE_TYPE --build-arg GITLAB_RUNNER_IMAGE_TAG=$GITLAB_RUNNER_IMAGE_TAG -f alpine-upgrade/Dockerfile alpine-upgrade
   ```

NOTE:
The IBM Z image does not contain the `docker-machine` dependency. It is not maintained for the Linux s390x or Linux ppc64le
platforms. For the current status, see [issue 26551](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/26551).

## SELinux

Some distributions (CentOS, Red Hat, Fedora) use SELinux by default to enhance the security of the underlying system.

Use caution with this configuration.

1. If you want to use the [Docker executor](../executors/docker.md) to run builds in containers, you need access to `/var/run/docker.sock`.
   However, if SELinux is in enforcing mode, you see a `Permission denied` error when you're accessing `/var/run/docker.sock`.
   Install [selinux-dockersock](https://github.com/dpw/selinux-dockersock) to resolve this issue.
1. Create a persistent directory on the host: `mkdir -p /srv/gitlab-runner/config`.
1. Run Docker with `:Z` on volumes:

```shell
docker run -d --name gitlab-runner --restart always \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /srv/gitlab-runner/config:/etc/gitlab-runner:Z \
  gitlab/gitlab-runner:latest
```
