---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Run GitLab Runner in a container **(FREE)**

This is how you can run GitLab Runner inside a Docker container.

## Docker Engine version compatibility

In general, the version of Docker Engine and the version of the GitLab Runner container image
do not have to match. The GitLab Runner images should be backwards and forwards compatible.
However, to ensure you have the latest features and security updates,
you should always use the latest stable [Docker Engine version](https://docs.docker.com/engine/release-notes/).

## General GitLab Runner Docker image usage

GitLab Runner Docker images (based on [Ubuntu or Alpine Linux](#docker-images))
are designed as wrappers around the standard `gitlab-runner` command, like if
GitLab Runner was installed directly on the host.

The general rule is that every GitLab Runner command that normally would be executed
as:

```shell
gitlab-runner <runner command and options...>
```

can be executed with:

```shell
docker run <chosen docker options...> gitlab/gitlab-runner <runner command and options...>
```

For example, getting the top-level help information for GitLab Runner command could be
executed as:

```shell
docker run --rm -t -i gitlab/gitlab-runner --help

NAME:
   gitlab-runner - a GitLab Runner

USAGE:
   gitlab-runner [global options] command [command options] [arguments...]

VERSION:
   15.1.0 (76984217)

(...)
```

In short, the `gitlab-runner` part of the command is replaced with
`docker run [docker options] gitlab/gitlab-runner`, while the rest of the
command stays as it is described in the [register documentation](../register/index.md).
The only difference is that the `gitlab-runner` command is executed inside of a
Docker container.

## Install the Docker image and start the container

Before you begin, ensure [Docker is installed](https://docs.docker.com/get-docker/).

To run `gitlab-runner` inside a Docker container, you need to make sure that the configuration is not lost when the container is restarted. To do this, there are two options, which are described below.

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

This example uses the local system for the configuration volume that is mounted into the `gitlab-runner` container. This volume is used for configs and other resources.

```shell
docker run -d --name gitlab-runner --restart always \
  -v /srv/gitlab-runner/config:/etc/gitlab-runner \
  -v /var/run/docker.sock:/var/run/docker.sock \
  gitlab/gitlab-runner:latest
```

NOTE:
On macOS, use `/Users/Shared` instead of `/srv`.

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
To set the container's timezone, in the `docker run` command, use the flag `--env TZ=<TIMEZONE>`. [View a list of available time zones](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones).

NOTE:
For a [FIPS compliant GitLab Runner](index.md#fips-compliant-gitlab-runner) image, based on `redhat/ubi8`, use the `gitlab/gitlab-runner:ubi-fips` tags.

### Register the runner

The final step is to [register a new runner](../register/index.md#docker). The GitLab Runner container doesn't pick up any jobs until it's registered.

## Update configuration

If you change the configuration in `config.toml`, you might need to restart the runner to apply the change.
Make sure to restart the whole container instead of using `gitlab-runner restart`:

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

When GitLab Runner is started as a foreground task (whether it's a locally installed binary or
inside of a Docker Container), the logs are printed to the standard output. When
GitLab Runner is started as a system service (for example, with Systemd), the logs are in most
cases logged through Syslog or other system logging mechanism.

With GitLab Runner started as a Docker based service, since the `gitlab-runner ...` command is
the main process of the container, the logs can be read using the `docker logs` command.

For example, if GitLab Runner was started with the following command:

```shell
docker run -d --name gitlab-runner --restart always \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /srv/gitlab-runner/config:/etc/gitlab-runner \
  gitlab/gitlab-runner:latest
```

you may get the logs with:

```shell
docker logs gitlab-runner
```

where `gitlab-runner` is the name of the container, set with `--name gitlab-runner` by
the first command.

You may find more information about handling container logs at the
[Docker documentation page](https://docs.docker.com/engine/reference/commandline/logs/).

## Installing trusted SSL server certificates

If your GitLab CI server is using self-signed SSL certificates then you should
make sure the GitLab CI server certificate is trusted by the GitLab Runner
container for them to be able to talk to each other.

The `gitlab/gitlab-runner` image is configured to look for the trusted SSL
certificates at `/etc/gitlab-runner/certs/ca.crt`, this can however be changed using the
`-e "CA_CERTIFICATES_PATH=/DIR/CERT"` configuration option.

Copy the `ca.crt` file into the `certs` directory on the data volume (or container).
The `ca.crt` file should contain the root certificates of all the servers you
want GitLab Runner to trust. The GitLab Runner container imports the `ca.crt` file on startup so if
your container is already running you may need to restart it for the changes to take effect.

## Docker images

The following multi-platform Docker images are available:

- `gitlab/gitlab-runner:latest` based on Ubuntu.
- `gitlab/gitlab-runner:alpine` based on Alpine with much a smaller footprint
  (~160/350 MB Ubuntu vs ~45/130 MB Alpine compressed/decompressed).

See [GitLab Runner](https://gitlab.com/gitlab-org/gitlab-runner/tree/main/dockerfiles)
source for possible build instructions for both Ubuntu and Alpine images.

### Creating a GitLab Runner Docker image

As of 2021-08-03, the GitLab Runner Docker image based on Alpine uses Alpine 3.12.0. However, you can upgrade the image's OS before it is available in the GitLab repositories.

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
   GITLAB_RUNNER_IMAGE_TYPE=gitlab-runner GITLAB_RUNNER_IMAGE_TAG=alpine-v13.12.0 docker build -t $GITLAB_RUNNER_IMAGE_TYPE:$GITLAB_RUNNER_IMAGE_TAG --build-arg GITLAB_RUNNER_IMAGE_TYPE=$GITLAB_RUNNER_IMAGE_TYPE --build-arg GITLAB_RUNNER_IMAGE_TAG=$GITLAB_RUNNER_IMAGE_TAG -f alpine-upgrade/Dockerfile alpine-upgrade
   ```

1. Create an upgraded `gitlab-runner-helper` image.

   ```shell
   GITLAB_RUNNER_IMAGE_TYPE=gitlab-runner-helper GITLAB_RUNNER_IMAGE_TAG=x86_64-v13.12.0 docker build -t $GITLAB_RUNNER_IMAGE_TYPE:$GITLAB_RUNNER_IMAGE_TAG --build-arg GITLAB_RUNNER_IMAGE_TYPE=$GITLAB_RUNNER_IMAGE_TYPE --build-arg GITLAB_RUNNER_IMAGE_TAG=$GITLAB_RUNNER_IMAGE_TAG -f alpine-upgrade/Dockerfile alpine-upgrade
   ```

NOTE:
The IBM Z image does not contain the `docker-machine` dependency, as it is not yet maintained for the Linux s390x or Linux ppc64le
platforms. See [issue](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/26551) for current status.

## SELinux

Some distributions (CentOS, Red Hat, Fedora) use SELinux by default to enhance the security of the underlying system.

Special care must be taken when dealing with such a configuration.

1. If you want to use the [Docker executor](../executors/docker.md) to run builds in containers, you need access to `/var/run/docker.sock`.
   However, if SELinux is in enforcing mode, you see a `Permission denied` error when you're accessing `/var/run/docker.sock`.
   Install [selinux-dockersock](https://github.com/dpw/selinux-dockersock) to resolve this issue.
1. Make sure that a persistent directory is created on host: `mkdir -p /srv/gitlab-runner/config`.
1. Run Docker with `:Z` on volumes:

```shell
docker run -d --name gitlab-runner --restart always \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /srv/gitlab-runner/config:/etc/gitlab-runner:Z \
  gitlab/gitlab-runner:latest
```
