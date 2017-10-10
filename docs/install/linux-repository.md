# Install GitLab Runner using the official GitLab repositories

We provide packages for the currently supported versions of Debian, Ubuntu, Mint, RHEL, Fedora, and CentOS.

| Distribution | Version | End of Life date   |
| ------------ | ------- | ------------------ |
| Debian       | buster  |                    |
| Debian       | stretch | approx. 2022       |
| Debian       | jessie  | June 2020          |
| Debian       | wheezy  | May 2018           |
| Ubuntu       | artful  |                    |
| Ubuntu       | zesty   | January 2018       |
| Ubuntu       | xenial  | April 2021         |
| Ubuntu       | trusty  | April 2019         |
| Mint         | sonya   | approx. 2021       |
| Mint         | serena  | approx. 2021       |
| Mint         | sarah   | approx. 2021       |
| Mint         | rosa    | April 2019         |
| Mint         | rafaela | April 2019         |
| Mint         | rebecca | April 2019         |
| Mint         | qiana   | April 2019         |
| REHL/CentOS  | 7       | June 2024          |
| REHL/CentOS  | 6       | November 2020      |
| Fedora       | 25      |                    |
| Fedora       | 26      |                    |

If you want to use the [Docker executor], install it before using the Runner:

```bash
curl -sSL https://get.docker.com/ | sh
```

## Installing the Runner

To install the Runner:

1. Add GitLab's official repository:

    **For GitLab Runner 10.0 and newer**

    ```bash
    # For Debian/Ubuntu
    curl -L https://packages.gitlab.com/install/repositories/runner/gitlab-runner/script.deb.sh | sudo bash

    # For RHEL/CentOS
    curl -L https://packages.gitlab.com/install/repositories/runner/gitlab-runner/script.rpm.sh | sudo bash
    ```

    **For versions older than 10.0, please use**

    ```bash
    # For Debian/Ubuntu
    curl -L https://packages.gitlab.com/install/repositories/runner/gitlab-ci-multi-runner/script.deb.sh | sudo bash

    # For RHEL/CentOS
    curl -L https://packages.gitlab.com/install/repositories/runner/gitlab-ci-multi-runner/script.rpm.sh | sudo bash
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

1. Install `gitlab-runner`:

    **For GitLab Runner 10.0 and newer**

    ```bash
    # For Debian/Ubuntu
    sudo apt-get install gitlab-runner

    # For RHEL/CentOS
    sudo yum install gitlab-runner
    ```

    **For versions older than 10.0, please use**

    ```bash
    # For Debian/Ubuntu
    sudo apt-get install gitlab-ci-multi-runner

    # For RHEL/CentOS
    sudo yum install gitlab-ci-multi-runner
    ```

1. [Register the Runner](../register/index.md)

After completing the step above, the Runner should be started already being
ready to be used by your projects!

Make sure that you read the [FAQ](../faq/README.md) section which describes
some of the most common problems with GitLab Runner.

## Updating the Runner

Simply execute to install latest version:

**For GitLab Runner 10.0 and newer**

```bash
# For Debian/Ubuntu
sudo apt-get update
sudo apt-get install gitlab-runner

# For RHEL/CentOS
sudo yum update
sudo yum install gitlab-runner
```

**For versions older than 10.0, please use**

```bash
# For Debian/Ubuntu
sudo apt-get update
sudo apt-get install gitlab-ci-multi-runner

# For RHEL/CentOS
sudo yum update
sudo yum install gitlab-ci-multi-runner
```

## Manually download packages

You can manually download the packages from the following URL:
<https://packages.gitlab.com/runner/gitlab-runner>

[docker executor]: ../executors/docker.md
