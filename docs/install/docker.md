# Run GitLab Runner in a container

This is how you can run GitLab Runner inside a Docker container.

## General GitLab Runner Docker image usage

GitLab Runner Docker images (based on [Ubuntu or Alpine Linux](#docker-images))
are designed as wrappers around the standard `gitlab-runner` command, like if
GitLab Runner was installed directly on the host.

The general rule is that every GitLab Runner command that normally would be executed
as:

```bash
gitlab-runner [Runner command and options...]
```

can be executed with:

```bash
docker run [chosen docker options...] gitlab/gitlab-runner [Runner command and options...]
```

For example, getting the top-level help information for GitLab Runner command could be
executed as:

```bash
docker run --rm -t -i gitlab/gitlab-runner --help

NAME:
   gitlab-runner - a GitLab Runner

USAGE:
   gitlab-runner [global options] command [command options] [arguments...]

VERSION:
   10.7.0 (7c273476)

(...)
```

In short, the `gitlab-runner` part of the command is replaced with
`docker run [docker options] gitlab/gitlab-runner`, while the rest of Runner's
command stays as it is described in the [register documentation](../register/index.md).
The only difference is that the `gitlab-runner` command is executed inside of a
Docker container.

## Docker image installation and configuration

1. Install Docker first:

   ```bash
   curl -sSL https://get.docker.com/ | sh
   ```

1. You need to mount a config volume into the `gitlab-runner` container to
   be used for configs and other resources:

   ```bash
   docker run -d --name gitlab-runner --restart always \
     -v /srv/gitlab-runner/config:/etc/gitlab-runner \
     -v /var/run/docker.sock:/var/run/docker.sock \
     gitlab/gitlab-runner:latest
   ```

   TIP: **Tip:**
   On macOS, use `/Users/Shared` instead of `/srv`.

   Or, you can use a config container to mount your custom data volume:

   ```bash
   docker run -d --name gitlab-runner-config \
       -v /etc/gitlab-runner \
       busybox:latest \
       /bin/true
   ```

   And then, run the Runner:

   ```bash
   docker run -d --name gitlab-runner --restart always \
       -v /var/run/docker.sock:/var/run/docker.sock \
       --volumes-from gitlab-runner-config \
       gitlab/gitlab-runner:latest
   ```

1. Register the runner you just launched by following the instructions in the
   [Docker section of Registering Runners](../register/index.md#docker).
   The runner won't pick up any jobs until it's registered.

Make sure that you read the [FAQ](../faq/README.md) section which describes
some of the most common problems with GitLab Runner.

## Update

Pull the latest version:

```bash
docker pull gitlab/gitlab-runner:latest
```

Stop and remove the existing container:

```bash
docker stop gitlab-runner && docker rm gitlab-runner
```

Start the container as you did originally:

```bash
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

```bash
docker run -d --name gitlab-runner --restart always \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /srv/gitlab-runner/config:/etc/gitlab-runner \
  gitlab/gitlab-runner:latest
```

you may get the logs with:

```bash
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

The following Docker images are available:

- `gitlab/gitlab-runner:latest` based on Ubuntu.
- `gitlab/gitlab-runner:alpine` based on Alpine with much a smaller footprint
  (~160/350 MB Ubuntu vs ~45/130 MB Alpine compressed/decompressed).

TIP: **Tip:**
See [gitlab-org/gitlab-runner](https://gitlab.com/gitlab-org/gitlab-runner/tree/master/dockerfiles)
source for possible build instructions for both Ubuntu and Alpine images.

## SELinux

Some distributions (CentOS, RedHat, Fedora) use SELinux by default to enhance the security of the underlying system.

The special care must be taken when dealing with such configuration.

1. If you want to use Docker executor to run builds in containers you need to access the `/var/run/docker.sock`.
   However, if you have a SELinux in enforcing mode, you will see the `Permission denied` when accessing the `/var/run/docker.sock`.
   Install the `selinux-dockersock` and to resolve the issue: <https://github.com/dpw/selinux-dockersock>.
1. Make sure that persistent directory is created on host: `mkdir -p /srv/gitlab-runner/config`.
1. Run docker with `:Z` on volumes:

```bash
docker run -d --name gitlab-runner --restart always \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /srv/gitlab-runner/config:/etc/gitlab-runner:Z \
  gitlab/gitlab-runner:latest
```

More information about the cause and resolution can be found here:
<http://www.projectatomic.io/blog/2015/06/using-volumes-with-docker-can-cause-problems-with-selinux/>
