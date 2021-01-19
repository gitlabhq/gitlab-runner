---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/engineering/ux/technical-writing/#assignments
---

# Install GitLab Runner using the official GitLab repositories

We provide packages for the currently supported versions of Debian, Ubuntu, Mint, RHEL, Fedora, and CentOS. You may be able to [install GitLab Runner as a binary](linux-manually.md#using-binary-file) on other Linux distributions.

| Distribution | Version                    | End of Life date      |
|--------------|----------------------------|-----------------------|
| Debian       | stretch                     | [June 2022](https://wiki.debian.org/LTS)             |
| Debian       | buster                      | [June 2024](https://wiki.debian.org/LTS)             |
| Ubuntu       | xenial                      | [April 2021](https://wiki.ubuntu.com/Releases)            |
| Ubuntu       | bionic                      | [April 2023](https://wiki.ubuntu.com/Releases)            |
| Ubuntu       | focal                       | [April 2025](https://wiki.ubuntu.com/Releases)            |
| Mint         | sarah, serena, sonya, sylvia| [April 2021](https://www.linuxmint.com/download_all.php)          |
| Mint         | tara, tessa, tina, tricia   | [April 2023](https://www.linuxmint.com/download_all.php)          |
| Mint         | ulyana, ulyssa              | [April 2025](https://www.linuxmint.com/download_all.php)          |
| RHEL/CentOS  | 7                           | [June 2024](https://wiki.centos.org/About/Product)             |
| CentOS       | 8                           | [December 2021](https://wiki.centos.org/About/Product)         |
| RHEL         | 8                           | [May 2029](https://access.redhat.com/product-life-cycles?product=Red%20Hat%20Enterprise%20Linux)         |
| Fedora       | 32                          | approx. May 2021      |
| Fedora       | 33                          | approx. Nov 2021      |

## Prerequisites

If you want to use the [Docker executor](../executors/docker.md), make sure to install Docker before
using GitLab Runner. [Read how to install Docker for your distribution](https://docs.docker.com/engine/installation/).

## Installing GitLab Runner

NOTE:
If you are using or upgrading from a version prior to GitLab Runner 10, read how
to [upgrade to the new version](#upgrading-to-gitlab-runner-10). If you want
to install a version prior to GitLab Runner 10, [visit the old docs](old.md).

To install GitLab Runner:

1. Add the official GitLab repository:

   ```shell
   # For Debian/Ubuntu/Mint
   curl -L "https://packages.gitlab.com/install/repositories/runner/gitlab-runner/script.deb.sh" | sudo bash

   # For RHEL/CentOS/Fedora
   curl -L "https://packages.gitlab.com/install/repositories/runner/gitlab-runner/script.rpm.sh" | sudo bash
   ```

   NOTE:
   Debian users should use [APT pinning](#apt-pinning).

1. Install the latest version of GitLab Runner, or skip to the next step to
   install a specific version:

   NOTE:
   Debian buster users should [disable skel](#disable-skel) to prevent
   [No such file or directory Job
   failures](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/1379)

   ```shell
   # For Debian/Ubuntu/Mint
   export GITLAB_RUNNER_DISABLE_SKEL=true; sudo -E apt-get install gitlab-runner

   # For RHEL/CentOS/Fedora
   export GITLAB_RUNNER_DISABLE_SKEL=true; sudo -E yum install gitlab-runner
   ```

1. To install a specific version of GitLab Runner:

   ```shell
   # for DEB based systems
   apt-cache madison gitlab-runner
   export GITLAB_RUNNER_DISABLE_SKEL=true; sudo -E apt-get install gitlab-runner=10.0.0

   # for RPM based systems
   yum list gitlab-runner --showduplicates | sort -r
   export GITLAB_RUNNER_DISABLE_SKEL=true; sudo -E yum install gitlab-runner-10.0.0-1
   ```

1. [Register a runner](../register/index.md)

After completing the step above, a runner should be started and be
ready to be used by your projects!

Make sure that you read the [FAQ](../faq/README.md) section which describes
some of the most common problems with GitLab Runner.

### APT pinning

A native package called `gitlab-ci-multi-runner` is available in
Debian Stretch. By default, when installing `gitlab-runner`, that package
from the official repositories will have a higher priority.

If you want to use our package, you should manually set the source of
the package. The best way is to add the pinning configuration file.

If you do this, the next update of the GitLab Runner package - whether it will
be done manually or automatically - will be done using the same source:

```shell
cat <<EOF | sudo tee /etc/apt/preferences.d/pin-gitlab-runner.pref
Explanation: Prefer GitLab provided packages over the Debian native ones
Package: gitlab-runner
Pin: origin packages.gitlab.com
Pin-Priority: 1001
EOF
```

## Updating GitLab Runner

Simply execute to install latest version:

```shell
# For Debian/Ubuntu/Mint
sudo apt-get update
sudo apt-get install gitlab-runner

# For RHEL/CentOS/Fedora
sudo yum update
sudo yum install gitlab-runner
```

## Manually download packages

You can [manually download and install the
packages](linux-manually.md#using-debrpm-package) if necessary.

## Disable `skel`

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/1379) in GitLab 12.10.

Sometimes the default [skeleton (`skel`) directory](https://www.thegeekdiary.com/understanding-the-etc-skel-directory-in-linux/)
causes [issues for GitLab Runner](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4449),
and it fails to run a job. When installing GitLab Runner, set the environment variable
`GITLAB_RUNNER_DISABLE_SKEL` to `true` before you install the package. This will create
the `$HOME` directory without the files inside of `skel`:

For example:

```shell
# For Debian/Ubuntu/Mint
export GITLAB_RUNNER_DISABLE_SKEL=true; sudo -E apt-get install gitlab-runner

# For RHEL/CentOS/Fedora
export GITLAB_RUNNER_DISABLE_SKEL=true; sudo -E yum install gitlab-runner
```

## Upgrading to GitLab Runner 10

To upgrade GitLab Runner from a version prior to 10.0:

1. Remove the old repository:

   ```shell
   # For Debian/Ubuntu/Mint
   sudo rm /etc/apt/sources.list.d/runner_gitlab-ci-multi-runner.list

   # For RHEL/CentOS/Fedora
   sudo rm /etc/yum.repos.d/runner_gitlab-ci-multi-runner.repo
   ```

1. Follow the same steps when [installing GitLab Runner](#installing-gitlab-runner),
   **without registering it** and using the new repository.

1. For RHEL/CentOS/Fedora, run:

   ```shell
   sudo /usr/share/gitlab-runner/post-install
   ```

   WARNING:
   If you don't run the above command, you will be left
   with no service file. Follow [issue #2786](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/2786)
   for more information.
