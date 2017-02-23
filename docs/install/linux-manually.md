# Manual installation and configuration on GNU/Linux

>**Note:**
Make sure that you read the [FAQ](../faq/README.md) section which describes
some of the most common problems with GitLab Runner.

If you don't want to use a [deb/rpm repository](linux-repository.md) to install
GitLab Runner, or your OS is not among the supported ones, you can install it
manually.

## Install

Simply download one of the binaries for your system:

```bash
sudo wget -O /usr/local/bin/gitlab-ci-multi-runner https://gitlab-ci-multi-runner-downloads.s3.amazonaws.com/latest/binaries/gitlab-ci-multi-runner-linux-386
sudo wget -O /usr/local/bin/gitlab-ci-multi-runner https://gitlab-ci-multi-runner-downloads.s3.amazonaws.com/latest/binaries/gitlab-ci-multi-runner-linux-amd64
sudo wget -O /usr/local/bin/gitlab-ci-multi-runner https://gitlab-ci-multi-runner-downloads.s3.amazonaws.com/latest/binaries/gitlab-ci-multi-runner-linux-arm
```

Give it permissions to execute:

```bash
sudo chmod +x /usr/local/bin/gitlab-ci-multi-runner
```

Optionally, if you want to use Docker, install Docker with:

```bash
curl -sSL https://get.docker.com/ | sh
```

Create a GitLab CI user:

```
sudo useradd --comment 'GitLab Runner' --create-home gitlab-runner --shell /bin/bash
```

Register the Runner (look into [Runners documentation](https://docs.gitlab.com/ce/ci/runners/) to learn how to obtain a token):

```bash
sudo gitlab-ci-multi-runner register
```

Install and run as service:

```bash
sudo gitlab-ci-multi-runner install --user=gitlab-runner --working-directory=/home/gitlab-runner
sudo gitlab-ci-multi-runner start
```

>**Note**
If `gitlab-ci-multi-runner` is installed and run as service (what is described
in this page), it will run as root, but will execute jobs as user specified by
the `install` command. This means that some of the job functions like cache and
artifacts will need to execute `/usr/local/bin/gitlab-ci-multi-runner` command,
therefore the user under which jobs are run, needs to have access to the executable.

## Update

Stop the service (you need elevated command prompt as before):

```bash
sudo gitlab-ci-multi-runner stop
```

Download the binary to replace Runner's executable:

```bash
sudo wget -O /usr/local/bin/gitlab-ci-multi-runner https://gitlab-ci-multi-runner-downloads.s3.amazonaws.com/latest/binaries/gitlab-ci-multi-runner-linux-386
sudo wget -O /usr/local/bin/gitlab-ci-multi-runner https://gitlab-ci-multi-runner-downloads.s3.amazonaws.com/latest/binaries/gitlab-ci-multi-runner-linux-amd64
```

Give it permissions to execute:

```bash
sudo chmod +x /usr/local/bin/gitlab-ci-multi-runner
```

Start the service:

```bash
sudo gitlab-ci-multi-runner start
```
