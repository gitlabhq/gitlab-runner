# Run GitLab Runner in a container

This is how you can run GitLab Runner inside a Docker container.

## General GitLab Runner Docker image usage

GitLab Runner Docker images (based on [Ubuntu or Alpine Linux](#docker-images))
are designed as wrappers around the standard `gitlab-runner` command, like if
GitLab Runner was installed directly on the host.

The general rule is that every GitLab Runner command that normally would be executed
as:

```shell
gitlab-runner <Runner command and options...>
```

can be executed with:

```shell
docker run <chosen docker options...> gitlab/gitlab-runner <Runner command and options...>
```

For example, getting the top-level help information for GitLab Runner command could be
executed as:

```shell
docker run --rm -t -i gitlab/gitlab-runner --help

NAME:
   gitlab-runner - a GitLab Runner

USAGE:
   gitlab-runner <global options> command <command options> <arguments...>

VERSION:
   10.7.0 (7c273476)

(...)
```

In short, the `gitlab-runner` part of the command is replaced with
`docker run [docker options] gitlab/gitlab-runner`, while the rest of Runner's
command stays as it is described in the [register documentation](../register/index.md).
The only difference is that the `gitlab-runner` command is executed inside of a
Docker container.

## Install the Docker image and start the container

Before you begin, ensure [Docker is installed](https://docs.docker.com/get-docker/).

To run `gitlab-runner` inside a Docker container, you need to make sure that the configuration is not lost when the container is restarted. To do this, there are two options, which are described below.

Make sure that you read the [FAQ](../faq/README.md) section which describes some of the most common problems with GitLab Runner.

NOTE: **Note:**
If you are using a [`session_server`](../configuration/advanced-configuration.md), you will also need to expose port `8093` by adding `-p 8093:8093` to your `docker run` command.

### Option 1: Use local system volume mounts to start the Runner container

This example uses the local system for the configuration volume that is mounted into the `gitlab-runner` container. This volume is used for configs and other resources.

   ```shell
   docker run -d --name gitlab-runner --restart always \
     -v /srv/gitlab-runner/config:/etc/gitlab-runner \
     -v /var/run/docker.sock:/var/run/docker.sock \
     gitlab/gitlab-runner:latest
   ```

   TIP: **Tip:** On macOS, use `/Users/Shared` instead of `/srv`.

### Option 2: Use Docker volumes to start the Runner container

In this example, you can use a configuration container to mount your custom data volume.

1. Create the Docker volume:

   ```shell
   docker volume create gitlab-runner-config
   ```

1. Start the Runner container using the volume we just created:

   ```shell
   docker run -d --name gitlab-runner --restart always \
       -v /var/run/docker.sock:/var/run/docker.sock \
       -v gitlab-runner-config:/etc/gitlab-runner \
       gitlab/gitlab-runner:latest
   ```

### Register the Runner

The final step is to [register a new Runner](../register/index.md#docker). The GitLab Runner Container won't pick up any jobs until it's registered.

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

NOTE: **Note:**
You need to use the same method for mounting you data volume as you
did originally (`-v /srv/gitlab-runner/config:/etc/gitlab-runner` or
`--volumes-from gitlab-runner-config`).

## Reading GitLab Runner logs

When GitLab Runner is started as a foreground task (whether it's a locally installed binary or
inside of a Docker Container), the logs are printed to the standard output. When
GitLab Runner is started as a system service (e.g. with Systemd), the logs are in most
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

You may find more information about handling container logs at the [Docker documentation
page](https://docs.docker.com/engine/reference/commandline/logs/).

## Installing trusted SSL server certificates

If your GitLab CI server is using self-signed SSL certificates then you should
make sure the GitLab CI server certificate is trusted by the GitLab Runner
container for them to be able to talk to each other.

The `gitlab/gitlab-runner` image is configured to look for the trusted SSL
certificates at `/etc/gitlab-runner/certs/ca.crt`, this can however be changed using the
`-e "CA_CERTIFICATES_PATH=/DIR/CERT"` configuration option.

Copy the `ca.crt` file into the `certs` directory on the data volume (or container).
The `ca.crt` file should contain the root certificates of all the servers you
want GitLab Runner to trust. The GitLab Runner container will
import the `ca.crt` file on startup so if your container is already running you
may need to restart it for the changes to take effect.

## Docker images

The following multi-platform Docker images are available:

- `gitlab/gitlab-runner:latest` based on Ubuntu.
- `gitlab/gitlab-runner:alpine` based on Alpine with much a smaller footprint
  (~160/350 MB Ubuntu vs ~45/130 MB Alpine compressed/decompressed).

TIP: **Tip:**
See [GitLab Runner](https://gitlab.com/gitlab-org/gitlab-runner/tree/master/dockerfiles)
source for possible build instructions for both Ubuntu and Alpine images.

NOTE: **Note:**
The IBM Z image does not contain the `docker-machine` dependency, as it is not yet maintained for the Linux s390x
platform. See [issue](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/26551) for current status.

## SELinux

Some distributions (CentOS, RedHat, Fedora) use SELinux by default to enhance the security of the underlying system.

Special care must be taken when dealing with such a configuration.

1. If you want to use the [Docker executor](../executors/docker.md) to run builds in containers, you'll need access to `/var/run/docker.sock`.
   However, if SELinux is in enforcing mode, you will see a `Permission denied` error when you're accessing `/var/run/docker.sock`.
   Install [selinux-dockersock](https://github.com/dpw/selinux-dockersock) to resolve this issue.
1. Make sure that a persistent directory is created on host: `mkdir -p /srv/gitlab-runner/config`.
1. Run Docker with `:Z` on volumes:

```shell
docker run -d --name gitlab-runner --restart always \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /srv/gitlab-runner/config:/etc/gitlab-runner:Z \
  gitlab/gitlab-runner:latest
```

More information about the cause and resolution can be found here:
<http://www.projectatomic.io/blog/2015/06/using-volumes-with-docker-can-cause-problems-with-selinux/>
