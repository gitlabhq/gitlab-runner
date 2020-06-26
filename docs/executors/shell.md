# Shell

The Shell executor is a simple executor that allows you to execute builds
locally to the machine that the Runner is installed. It supports all systems on
which the Runner can be installed. That means that it's possible to use scripts
generated for Bash, Windows PowerShell and Windows Batch (deprecated).

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

## Terminating and killing processes

The shell executor starts the script for each job in a new process. On
UNIX systems, it sets the main process as a [process
group](https://www.informit.com/articles/article.aspx?p=397655&seqNum=6).

GitLab Runner terminates processes when:

- A job [times out](https://docs.gitlab.com/ee/user/project/pipelines/settings.html#timeout).
- A job is canceled.

### GitLab 13.0 and earlier

On UNIX systems `gitlab-runner` sends a `SIGKILL` to the process to
terminate it, because the child processes belong to the same process
group the signal is also sent to them. Windows sends a `taskkill /F /T`.

### GitLab 13.1 and later

On UNIX system `gitlab-runner` sends `SIGTERM` to the process and its
child processes, and after 10 minutes sends `SIGKILL`. This allows for
graceful termination for the process. Windows don't have a `SIGTERM`
equivalent, so the kill process is sent twice. The second is sent after
10 minutes.

If for some reason this new termination process has problems with your
scripts but works with the  [old method](#gitlab-130-and-earlier) you can
set the feature flag
[`FF_SHELL_EXECUTOR_USE_LEGACY_PROCESS_KILL`](../configuration/feature-flags.md)
to `true`, and it will use the old method. Keep in mind that this
feature flag will be removed in GitLab Runner 14.0 so you still need to
fix your script to handle the new termination.
