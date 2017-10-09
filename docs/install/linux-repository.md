---
last_updated: 2017-10-09
---

# Install GitLab Runner using the official GitLab repositories

The currently supported distributions are:

- Debian
- Ubuntu
- RHEL
- CentOS
- Fedora (added in 10.0)

## Prerequisites

If you want to use the [Docker executor], make sure to install Docker before
using the Runner. [Read how to install Docker for your distribution](https://docs.docker.com/engine/installation/).

## Installing the Runner

CAUTION: **Important:**
If you are using or upgrading from a version prior to GitLab Runner 10, read how
to [upgrade to the new version](#upgrading-to-gitlab-runner-10). If you want
to install a version older than GitLab Runner 10, [visit the old docs](old.md).

To install the Runner:

1. Add GitLab's official repository:

    ```bash
    # For Debian/Ubuntu
    curl -L https://packages.gitlab.com/install/repositories/runner/gitlab-runner/script.deb.sh | sudo bash

    # For RHEL/CentOS/Fedora
    curl -L https://packages.gitlab.com/install/repositories/runner/gitlab-runner/script.rpm.sh | sudo bash
    ```

    >**Note:**
    _Debian users should use APT pinning_
    >
    Since Debian Stretch, Debian maintainers added their native package
    with the same name as is used by our package, and by default the official
    repositories will have a higher priority.
    >
    If you want to use our package you should manually set the source of
    the package. The best would be to add the pinning configuration file.
    Thanks to this every next update of the Runner's package - whether it will
    be done manually or automatically - will be done using the same source:
    >
    ```bash
    cat > /etc/apt/preferences.d/pin-gitlab-runner.pref <<EOF
    Explanation: Prefer GitLab provided packages over the Debian native ones
    Package: gitlab-runner
    Pin: origin packages.gitlab.com
    Pin-Priority: 1001
    EOF
    ```

1. Install the latest version of GitLab Runner, or skip to the next step to
   install a specific version:

    ```bash
    # For Debian/Ubuntu
    sudo apt-get install gitlab-runner

    # For RHEL/CentOS/Fedora
    sudo yum install gitlab-runner
    ```

1. To install a specific version of GitLab Runner:

    ```bash
    # for DEB based systems
    apt-cache madison gitlab-runner
    sudo apt-get install gitlab-runner=10.0.0

    # for RPM based systems
    yum list gitlab-runner --showduplicates | sort -r
    sudo yum install gitlab-runner-10.0.0-1
    ```

1. [Register the Runner](../register/index.md)

After completing the step above, the Runner should be started already being
ready to be used by your projects!

Make sure that you read the [FAQ](../faq/README.md) section which describes
some of the most common problems with GitLab Runner.

## Updating the Runner

Simply execute to install latest version:

```bash
# For Debian/Ubuntu
sudo apt-get update
sudo apt-get install gitlab-runner

# For RHEL/CentOS/Fedora
sudo yum update
sudo yum install gitlab-runner
```
## Manually download packages

You can manually download the packages from the following URL:
<https://packages.gitlab.com/runner/gitlab-runner>

## Upgrading to GitLab Runner 10

To upgrade GitLab Runner from a version older than 10.0:

1. Remove the old repository:

    ```
    # For Debian/Ubuntu
    sudo rm /etc/apt/sources.list.d/runner_gitlab-ci-multi-runner.list

    # For RHEL/CentOS
    sudo rm /etc/yum.repos.d/runner_gitlab-ci-multi-runner.repo
    ```

1. Follow the same steps when [installing the Runner](#installing-the-runner),
   **without registering it** and using the new repository.

[docker executor]: ../executors/docker.md
