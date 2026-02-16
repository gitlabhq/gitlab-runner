---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: Manually download and install the GitLab Runner binary on Linux.
title: Install GitLab Runner manually on GNU/Linux
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

You can install GitLab Runner manually by using a `deb` or `rpm` package or a binary file.
Use this approach as a last resort if:

- You can't use the deb/rpm repository to install GitLab Runner
- Your GNU/Linux OS is not supported

## Prerequisites

Before running GitLab Runner manually:

- If you plan to use the Docker executor, install Docker first.
- Review the FAQ section for common problems and solutions.

## Using deb/rpm package

You can download and install GitLab Runner by using a `deb` or `rpm` package.

### Download

To download the appropriate package for your system:

1. Find the latest filename and options at
   <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/index.html>.
1. Download the runner-helper version that matches your package manager or architecture.
1. Choose a version and download a binary, as described in the
   documentation for [downloading any other tagged releases](bleeding-edge.md#download-any-other-tagged-release) for
   bleeding edge GitLab Runner releases.

For example, for Debian or Ubuntu:

```shell
# Replace ${arch} with any of the supported architectures, e.g. amd64, arm, arm64
# A full list of architectures can be found here https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/index.html
curl -LJO "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/deb/gitlab-runner-helper-images.deb"
curl -LJO "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/deb/gitlab-runner_${arch}.deb"
```

For example, for CentOS or Red Hat Enterprise Linux:

```shell
# Replace ${arch} with any of the supported architectures, e.g. amd64, arm, arm64
# A full list of architectures can be found here https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/index.html
curl -LJO "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/rpm/gitlab-runner-helper-images.rpm"
curl -LJO "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/rpm/gitlab-runner_${arch}.rpm"
```

For example, for FIPS compliant GitLab Runner on RHEL:

```shell
# Currently only amd64 is a supported arch
# The FIPS compliant GitLab Runner version continues to include the helper images in one package.
# A full list of architectures can be found here https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/index.html
curl -LJO "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/rpm/gitlab-runner_amd64-fips.rpm"
```

### Install

1. Install the package for your system as follows.

   For example, for Debian or Ubuntu:

   ```shell
   dpkg -i gitlab-runner-helper-images.deb gitlab-runner_<arch>.deb
   ```

   For example, for CentOS or Red Hat Enterprise Linux:

   ```shell
   dnf install -y gitlab-runner-helper-images.rpm gitlab-runner_<arch>.rpm
   ```

### Upgrade

Download the latest package for your system then upgrade as follows:

For example, for Debian or Ubuntu:

```shell
dpkg -i gitlab-runner_<arch>.deb
```

For example, for CentOS or Red Hat Enterprise Linux:

```shell
dnf install -y gitlab-runner-helper-images.rpm gitlab-runner_<arch>.rpm
```

## Using binary file

You can download and install GitLab Runner by using a binary file.

### Install

1. Download one of the binaries for your system:

   ```shell
   # Linux x86-64
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-amd64"

   # Linux x86
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-386"

   # Linux arm
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-arm"

   # Linux arm64
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-arm64"

   # Linux s390x
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-s390x"

   # Linux ppc64le
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-ppc64le"

   # Linux riscv64
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-riscv64"

   # Linux loong64
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-loong64"

   # Linux x86-64 FIPS Compliant
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-amd64-fips"
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

{{< alert type="note" >}}

If `gitlab-runner` is installed and run as a service, it runs as root,
but executes jobs as a user specified by the `install` command.
This means that some of the job functions like cache and
artifacts must execute the `/usr/local/bin/gitlab-runner` command.
Therefore, the user under which jobs are run needs to have access to the executable.

{{< /alert >}}

### Upgrade

1. Stop the service (you need elevated command prompt as before):

   ```shell
   sudo gitlab-runner stop
   ```

1. Download the binary to replace the GitLab Runner executable. For example:

   ```shell
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-amd64"
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

## Next steps

After installation, [register a runner](../register/_index.md) to complete the setup.

The runner binary doesn't include pre-built helper images. You can use these commands to download the corresponding version of the helper image archive and copy it to the appropriate location:

```shell
mkdir -p /usr/local/bin/out/helper-images
cd /usr/local/bin/out/helper-images
```

Choose the appropriate helper image for your architecture:

<details>
<summary>Ubuntu helper images</summary>

```shell
# Linux x86-64 ubuntu
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-ubuntu-x86_64.tar.xz

# Linux x86-64 ubuntu pwsh
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-ubuntu-x86_64-pwsh.tar.xz

# Linux s390x ubuntu
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-ubuntu-s390x.tar.xz

# Linux ppc64le ubuntu
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-ubuntu-ppc64le.tar.xz

# Linux arm64 ubuntu
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-ubuntu-arm64.tar.xz

# Linux arm ubuntu
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-ubuntu-arm.tar.xz

# Linux x86-64 ubuntu specific version - v17.10.0
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/v17.10.0/helper-images/prebuilt-ubuntu-x86_64.tar.xz
```

</details>

<details>
<summary>Alpine helper images</summary>

```shell
# Linux x86-64 alpine
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-alpine-x86_64.tar.xz

# Linux x86-64 alpine pwsh
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-alpine-x86_64-pwsh.tar.xz

# Linux s390x alpine
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-alpine-s390x.tar.xz

# Linux riscv64 alpine edge
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-alpine-edge-riscv64.tar.xz

# Linux arm64 alpine
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-alpine-arm64.tar.xz

# Linux arm alpine
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-alpine-arm.tar.xz
```

</details>

## Additional resources

- [Docker executor documentation](../executors/docker.md)
- [Install Docker](https://docs.docker.com/engine/install/centos/#install-docker-ce)
- [Download other GitLab Runner versions](bleeding-edge.md#download-any-other-tagged-release)
- [FIPS compliant GitLab Runner information](requirements.md#fips-compliant-gitlab-runner)
- [GitLab Runner FAQ](../faq/_index.md)
- [deb/rpm repository installation](linux-repository.md)
