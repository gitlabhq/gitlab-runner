---
last_updated: 2017-10-09
---

# Install GitLab Runner manually on GNU/Linux

If you don't want to use a [deb/rpm repository](linux-repository.md) to install
GitLab Runner, or your GNU/Linux OS is not among the supported ones, you can
install it manually.

Make sure that you read the [FAQ](../faq/README.md) section which describes
some of the most common problems with GitLab Runner.

## Install

CAUTION: **Important:**
With GitLab Runner 10, the executable was renamed to `gitlab-runner`. If you
want to install a version prior to GitLab Runner 10, [visit the old docs](old.md).

1. Simply download one of the binaries for your system:

   ```sh
   # Linux x86-64
   sudo curl -L --output /usr/local/bin/gitlab-runner https://gitlab-runner-downloads.s3.amazonaws.com/latest/binaries/gitlab-runner-linux-amd64

   # Linux x86
   sudo curl -L --output /usr/local/bin/gitlab-runner https://gitlab-runner-downloads.s3.amazonaws.com/latest/binaries/gitlab-runner-linux-386

   # Linux arm
   sudo curl -L --output /usr/local/bin/gitlab-runner https://gitlab-runner-downloads.s3.amazonaws.com/latest/binaries/gitlab-runner-linux-arm
   ```

   You can download a binary for every available version as described in
   [Bleeding Edge - download any other tagged release](bleeding-edge.md#download-any-other-tagged-release).

1. Give it permissions to execute:

   ```sh
   sudo chmod +x /usr/local/bin/gitlab-runner
   ```

1. Optionally, if you want to use Docker, install Docker with:

   ```sh
   curl -sSL https://get.docker.com/ | sh
   ```

1. Create a GitLab CI user:

   ```sh
   sudo useradd --comment 'GitLab Runner' --create-home gitlab-runner --shell /bin/bash
   ```

1. Install and run as service:

   ```sh
   sudo gitlab-runner install --user=gitlab-runner --working-directory=/home/gitlab-runner
   sudo gitlab-runner start
   ```

1. [Register the Runner](../register/index.md)

NOTE: **Note**
If `gitlab-runner` is installed and run as service (what is described
in this page), it will run as root, but will execute jobs as user specified by
the `install` command. This means that some of the job functions like cache and
artifacts will need to execute `/usr/local/bin/gitlab-runner` command,
therefore the user under which jobs are run, needs to have access to the executable.

## Update

1. Stop the service (you need elevated command prompt as before):

   ```sh
   sudo gitlab-runner stop
   ```

1. Download the binary to replace Runner's executable. For example:

   ```sh
   sudo curl -L --output /usr/local/bin/gitlab-runner https://gitlab-runner-downloads.s3.amazonaws.com/latest/binaries/gitlab-runner-linux-amd64
   ```

   You can download a binary for every available version as described in
   [Bleeding Edge - download any other tagged release](bleeding-edge.md#download-any-other-tagged-release).

1. Give it permissions to execute:

   ```sh
   sudo chmod +x /usr/local/bin/gitlab-runner
   ```

1. Start the service:

   ```sh
   sudo gitlab-runner start
   ```
