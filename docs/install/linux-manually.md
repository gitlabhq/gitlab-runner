---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Install GitLab Runner manually on GNU/Linux **(FREE)**

If you can't use the [deb/rpm repository](linux-repository.md) to
install GitLab Runner, or your GNU/Linux OS is not among the supported
ones, you can install it manually using one of the methods below, as a
last resort.

If you want to use the [Docker executor](../executors/docker.md),
you must [install Docker](https://docs.docker.com/engine/install/centos/#install-docker-ce)
before using GitLab Runner.

Make sure that you read the [FAQ](../faq/index.md) section which describes
some of the most common problems with GitLab Runner.

## Using deb/rpm package

It is possible to download and install via a `deb` or `rpm` package, if necessary.

### Download

To download the appropriate package for your system:

1. Find the latest file name and options at
   <https://gitlab-runner-downloads.s3.amazonaws.com/latest/index.html>.
1. Choose a version and download a binary, as described in the
   documentation for [downloading any other tagged releases](bleeding-edge.md#download-any-other-tagged-release) for
   bleeding edge GitLab Runner releases.

For example, for Debian or Ubuntu:

```shell
# Replace ${arch} with any of the supported architectures, e.g. amd64, arm, arm64
# A full list of architectures can be found here https://gitlab-runner-downloads.s3.amazonaws.com/latest/index.html
curl -LJO "https://gitlab-runner-downloads.s3.amazonaws.com/latest/deb/gitlab-runner_${arch}.deb"
```

For example, for CentOS or Red Hat Enterprise Linux:

```shell
# Replace ${arch} with any of the supported architectures, e.g. amd64, arm, arm64
# A full list of architectures can be found here https://gitlab-runner-downloads.s3.amazonaws.com/latest/index.html
curl -LJO "https://gitlab-runner-downloads.s3.amazonaws.com/latest/rpm/gitlab-runner_${arch}.rpm"
```

For example, for [FIPS compliant GitLab Runner](index.md#fips-compliant-gitlab-runner) on RHEL:

```shell
# Currently only amd64 is a supported arch
# A full list of architectures can be found here https://gitlab-runner-downloads.s3.amazonaws.com/latest/index.html
curl -LJO "https://gitlab-runner-downloads.s3.amazonaws.com/latest/rpm/gitlab-runner_amd64-fips.rpm"
```

### Install

1. Install the package for your system as follows.

   For example, for Debian or Ubuntu:

   ```shell
   dpkg -i gitlab-runner_<arch>.deb
   ```

   For example, for CentOS or Red Hat Enterprise Linux:

   ```shell
   rpm -i gitlab-runner_<arch>.rpm
   ```

1. [Register a runner](../register/index.md#linux)

### Update

Download the latest package for your system then upgrade as follows:

For example, for Debian or Ubuntu:

```shell
dpkg -i gitlab-runner_<arch>.deb
```

For example, for CentOS or Red Hat Enterprise Linux:

```shell
rpm -Uvh gitlab-runner_<arch>.rpm
```

## Using binary file

It is possible to download and install via binary file, if necessary.

### Install

WARNING:
With GitLab Runner 10, the executable was renamed to `gitlab-runner`.

1. Simply download one of the binaries for your system:

   ```shell
   # Linux x86-64
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://gitlab-runner-downloads.s3.amazonaws.com/latest/binaries/gitlab-runner-linux-amd64"

   # Linux x86
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://gitlab-runner-downloads.s3.amazonaws.com/latest/binaries/gitlab-runner-linux-386"

   # Linux arm
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://gitlab-runner-downloads.s3.amazonaws.com/latest/binaries/gitlab-runner-linux-arm"

   # Linux arm64
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://gitlab-runner-downloads.s3.amazonaws.com/latest/binaries/gitlab-runner-linux-arm64"

   # Linux s390x
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://gitlab-runner-downloads.s3.amazonaws.com/latest/binaries/gitlab-runner-linux-s390x"

   # Linux ppc64le
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://gitlab-runner-downloads.s3.amazonaws.com/latest/binaries/gitlab-runner-linux-ppc64le"

   # Linux x86-64 FIPS Compliant
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://gitlab-runner-downloads.s3.amazonaws.com/latest/binaries/gitlab-runner-linux-amd64-fips"
   ```

   You can download a binary for every available version as described in
   [Bleeding Edge - download any other tagged release](bleeding-edge.md#download-any-other-tagged-release).

1. Give it permissions to execute:

   ```shell
   sudo chmod +x /usr/local/bin/gitlab-runner
   ```

1. Create a GitLab CI user:

   ```shell
   sudo useradd --comment 'GitLab Runner' --create-home gitlab-runner --shell /bin/bash
   ```

1. Install and run as service:

   ```shell
   sudo gitlab-runner install --user=gitlab-runner --working-directory=/home/gitlab-runner
   sudo gitlab-runner start
   ```

   Ensure you have `/usr/local/bin/` in `$PATH` for root or you might get a `command not found` error.
   Alternately, you can install `gitlab-runner` in a different location, like `/usr/bin/`.

1. [Register a runner](../register/index.md)

NOTE:
If `gitlab-runner` is installed and run as service (what is described
in this page), it will run as root, but will execute jobs as user specified by
the `install` command. This means that some of the job functions like cache and
artifacts will need to execute `/usr/local/bin/gitlab-runner` command,
therefore the user under which jobs are run, needs to have access to the executable.

### Update

1. Stop the service (you need elevated command prompt as before):

   ```shell
   sudo gitlab-runner stop
   ```

1. Download the binary to replace the GitLab Runner executable. For example:

   ```shell
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://gitlab-runner-downloads.s3.amazonaws.com/latest/binaries/gitlab-runner-linux-amd64"
   ```

   You can download a binary for every available version as described in
   [Bleeding Edge - download any other tagged release](bleeding-edge.md#download-any-other-tagged-release).

1. Give it permissions to execute:

   ```shell
   sudo chmod +x /usr/local/bin/gitlab-runner
   ```

1. Start the service:

   ```shell
   sudo gitlab-runner start
   ```
