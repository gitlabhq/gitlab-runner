# Shell

The Shell executor is a simple executor that allows you to execute builds
locally to the machine that the Runner is installed. It supports all systems on
which the Runner can be installed. That means that it's possible to use scripts
generated for Bash, Windows PowerShell and Windows Batch.

NOTE: **Note:**
Always use the latest version of Git available. Additionally, GitLab Runner will use
the `git lfs` command if [Git LFS](https://git-lfs.github.com) is installed on the machine,
so ensure Git LFS is up-to-date when GitLab Runner will run using the shell executor.

## Overview

The scripts can be run as unprivileged user if the `--user` is added to the
[`gitlab-runner run` command](../commands/README.md#gitlab-runner-run). This feature is only supported by Bash.

The source project is checked out to:
`<working-directory>/builds/<short-token>/<concurrent-id>/<namespace>/<project-name>`.

The caches for project are stored in
`<working-directory>/cache/<namespace>/<project-name>`.

Where:

- `<working-directory>` is the value of `--working-directory` as passed to the
  `gitlab-runner run` command or the current directory where the Runner is
  running
- `<short-token>` is a shortened version of the Runner's token (first 8 letters)
- `<concurrent-id>` is a unique number, identifying the local job ID on the
  particular Runner in context of the project
- `<namespace>` is the namespace where the project is stored on GitLab
- `<project-name>` is the name of the project as it is stored on GitLab

To overwrite the `<working-directory>/builds` and `<working-directory/cache`
specify the `builds_dir` and `cache_dir` options under the `[[runners]]` section
in [`config.toml`](../configuration/advanced-configuration.md).

## Running as unprivileged user

If GitLab Runner is installed on Linux from the [official `.deb` or `.rpm`
packages](https://packages.gitlab.com/runner/gitlab-runner), the installer will try to use the `gitlab_ci_multi_runner`
user if found. If it is not found, it will create a `gitlab-runner` user and use
this instead.

All shell builds will be then executed as either the `gitlab-runner` or
`gitlab_ci_multi_runner` user.

In some testing scenarios, your builds may need to access some privileged
resources, like Docker Engine or VirtualBox. In that case you need to add the
`gitlab-runner` user to the respective group:

```shell
usermod -aG docker gitlab-runner
usermod -aG vboxusers gitlab-runner
```

## Selecting your shell

GitLab Runner [supports certain shells](../shells/index.md). To select a shell, specify it in your `config.toml` file. For example:

```toml
...
[[runners]]
  name = "shell executor runner"
  executor = "shell"
  shell = "powershell"
...
```

## Security

Generally it's unsafe to run tests with shell executors. The jobs are run with
the user's permissions (`gitlab-runner`) and can "steal" code from other
projects that are run on this server. Use it only for running builds on a
server you trust and own.
