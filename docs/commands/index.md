---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# GitLab Runner commands

DETAILS:
**Tier:** Free, Premium, Ultimate
**Offering:** GitLab.com, GitLab Self-Managed, GitLab Dedicated

GitLab Runner contains a set of commands you use to register, manage, and
run your builds.

You can check the list of commands by executing:

```shell
gitlab-runner --help
```

Append `--help` after a command to see its specific help page:

```shell
gitlab-runner <command> --help
```

## Using environment variables

Most of the commands support environment variables as a method to pass the
configuration to the command.

You can see the name of the environment variable when invoking `--help` for a
specific command. For example, you can see below the help message for the `run`
command:

```shell
gitlab-runner run --help
```

The output is similar to:

```plaintext
NAME:
   gitlab-runner run - run multi runner service

USAGE:
   gitlab-runner run [command options] [arguments...]

OPTIONS:
   -c, --config "/Users/ayufan/.gitlab-runner/config.toml"      Config file [$CONFIG_FILE]
```

## Running in debug mode

When you're looking for the cause of an undefined behavior or error, use debug mode.

To run a command in debug mode, prepend the command with `--debug`:

```shell
gitlab-runner --debug <command>
```

## Super-user permission

Commands that access the configuration of GitLab Runner behave differently when
executed as super-user (`root`). The file location depends on the user executing
the command.

When you execute `gitlab-runner` commands, you see the mode it is running in:

```shell
$ gitlab-runner run

INFO[0000] Starting multi-runner from /Users/ayufan/.gitlab-runner/config.toml ...  builds=0
WARN[0000] Running in user-mode.
WARN[0000] Use sudo for system-mode:
WARN[0000] $ sudo gitlab-runner...
```

You should use `user-mode` if you are sure this is the mode you
want to work with. Otherwise, prefix your command with `sudo`:

```shell
$ sudo gitlab-runner run

INFO[0000] Starting multi-runner from /etc/gitlab-runner/config.toml ...  builds=0
INFO[0000] Running in system-mode.
```

In the case of Windows, you may need to run the command prompt as
an administrator.

## Configuration file

GitLab Runner configuration uses the [TOML](https://github.com/toml-lang/toml) format.

You can find the file to be edited:

1. On \*nix systems when GitLab Runner is
   executed as super-user (`root`): `/etc/gitlab-runner/config.toml`
1. On \*nix systems when GitLab Runner is
   executed as non-root: `~/.gitlab-runner/config.toml`
1. On other systems: `./config.toml`

Most of the commands accept an argument to specify a custom configuration file,
so you can have a multiple different configurations on a single machine.
To specify a custom configuration file, use the `-c` or `--config` flag, or use
the `CONFIG_FILE` environment variable.

## Signals

You can use system signals to interact with GitLab Runner. The
following commands support the following signals:

| Command             | Signal                  | Action                                                                                                |
|---------------------|-------------------------|-------------------------------------------------------------------------------------------------------|
| `register`          | **SIGINT**              | Cancel runner registration and delete if it was already registered.                                   |
| `run`, `run-single` | **SIGINT**, **SIGTERM** | Abort all running builds and exit as soon as possible. Use twice to exit now (**forceful shutdown**). |
| `run`, `run-single` | **SIGQUIT**             | Stop accepting new builds. Exit as soon as the running builds finish (**graceful shutdown**).       |
| `run`               | **SIGHUP**              | Force to reload configuration file.                                                                   |

For example, to force a reload of a runner's configuration file, run:

```shell
sudo kill -SIGHUP <main_runner_pid>
```

For [graceful shutdowns](#gitlab-runner-stop-doesnt-shut-down-gracefully):

```shell
sudo kill -SIGQUIT <main_runner_pid>
```

WARNING:
Do **not** use `killall` or `pkill` for graceful shutdowns if you are using `shell`
or `docker` executors. This can cause improper handling of the signals due to sub-processes
being killed as well. Use it only on the main process handling the jobs.

Some operating systems are configured to automatically restart services when they fail (which is the default on some platforms).
If your operating system has this configuration, it might automatically restart the runner if it is shut down by the signals above.

## Commands overview

You see the following if you run `gitlab-runner` without any arguments:

```plaintext
NAME:
   gitlab-runner - a GitLab Runner

USAGE:
   gitlab-runner [global options] command [command options] [arguments...]

VERSION:
   16.5.0 (853330f9)

AUTHOR:
   GitLab Inc. <support@gitlab.com>

COMMANDS:
   list                  List all configured runners
   run                   run multi runner service
   register              register a new runner
   reset-token           reset a runner's token
   install               install service
   uninstall             uninstall service
   start                 start service
   stop                  stop service
   restart               restart service
   status                get status of a service
   run-single            start single runner
   unregister            unregister specific runner
   verify                verify all registered runners
   artifacts-downloader  download and extract build artifacts (internal)
   artifacts-uploader    create and upload build artifacts (internal)
   cache-archiver        create and upload cache artifacts (internal)
   cache-extractor       download and extract cache artifacts (internal)
   cache-init            changed permissions for cache paths (internal)
   health-check          check health for a specific address
   read-logs             reads job logs from a file, used by kubernetes executor (internal)
   help, h               Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --cpuprofile value           write cpu profile to file [$CPU_PROFILE]
   --debug                      debug mode [$RUNNER_DEBUG]
   --log-format value           Choose log format (options: runner, text, json) [$LOG_FORMAT]
   --log-level value, -l value  Log level (options: debug, info, warn, error, fatal, panic) [$LOG_LEVEL]
   --help, -h                   show help
   --version, -v                print the version
```

Below we explain what each command does in detail.

## Registration-related commands

Use the following commands to register a new runner, or list and verify
them if they are still registered.

- [`gitlab-runner register`](#gitlab-runner-register)
  - [Interactive registration](#interactive-registration)
  - [Non-interactive registration](#non-interactive-registration)
- [`gitlab-runner list`](#gitlab-runner-list)
- [`gitlab-runner verify`](#gitlab-runner-verify)
- [`gitlab-runner unregister`](#gitlab-runner-unregister)

These commands support the following arguments:

| Parameter  | Default                                                   | Description                                    |
| ---------- | --------------------------------------------------------- | ---------------------------------------------- |
| `--config` | See the [configuration file section](#configuration-file) | Specify a custom configuration file to be used |

### `gitlab-runner register`

This command registers your runner in GitLab by using the GitLab [Runners API](https://docs.gitlab.com/ee/api/runners.html#register-a-new-runner).

The registered runner is
added to the [configuration file](#configuration-file).
You can use multiple configurations in a single installation of GitLab Runner. Executing
`gitlab-runner register` adds a new configuration entry. It doesn't remove the
previous ones.

You can register a runner:

- interactively.
- non-interactively.

NOTE:
Runners can be registered directly by using the GitLab [Runners API](https://docs.gitlab.com/ee/api/runners.html#register-a-new-runner) but
configuration is not generated automatically.

#### Interactive registration

This command is usually used in interactive mode (**default**). You are
asked multiple questions during a runner's registration.

This question can be pre-filled by adding arguments when invoking the registration command:

```shell
gitlab-runner register --name my-runner --url "http://gitlab.example.com" --token my-authentication-token
```

Or by configuring the environment variable before the `register` command:

```shell
export CI_SERVER_URL=http://gitlab.example.com
export RUNNER_NAME=my-runner
export CI_SERVER_TOKEN=my-authentication-token
gitlab-runner register
```

To check all possible arguments and environments execute:

```shell
gitlab-runner register --help
```

#### Non-interactive registration

It's possible to use registration in non-interactive / unattended mode.

You can specify the arguments when invoking the registration command:

```shell
gitlab-runner register --non-interactive <other-arguments>
```

Or by configuring the environment variable before the `register` command:

```shell
<other-environment-variables>
export REGISTER_NON_INTERACTIVE=true
gitlab-runner register
```

NOTE:
Boolean parameters must be passed in the command line with `--key={true|false}`.

#### `[[runners]]` configuration template file

> - [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4228) in GitLab Runner 12.2.

Additional options can be configured during runner registration by using the
[configuration template file](../register/index.md#register-with-a-configuration-template) feature.

### `gitlab-runner list`

This command lists all runners saved in the
[configuration file](#configuration-file).

### `gitlab-runner verify`

This command verifies that the registered runners can connect to GitLab. But, it
doesn't verify if the runners are being used by the GitLab Runner service. An
example output is:

```plaintext
Verifying runner... is alive                        runner=fee9938e
Verifying runner... is alive                        runner=0db52b31
Verifying runner... is alive                        runner=826f687f
Verifying runner... is alive                        runner=32773c0f
```

To remove the old runners that have been removed from GitLab, execute the following
command.

WARNING:
This operation cannot be undone. It updates the configuration file, so
make sure to have a backup of `config.toml` before executing it.

```shell
gitlab-runner verify --delete
```

### `gitlab-runner unregister`

This command unregisters registered runners by using the GitLab [Runners API](https://docs.gitlab.com/ee/api/runners.html#delete-a-registered-runner).

It expects either:

- A full URL and the runner's token.
- The runner's name.

With the `--all-runners` option, it unregisters all the attached runners.

NOTE:
Runners can be unregistered with the GitLab [Runners API](https://docs.gitlab.com/ee/api/runners.html#delete-a-runner) but the
configuration is not modified for the user.

- If the runner was created with a runner registration token, `gitlab-runner unregister`
  with the runner authentication token deletes the runner.
- If the runner was created in the GitLab UI or with the Runners API, `gitlab-runner unregister`
  with the runner authentication token deletes the runner manager, but not the runner.
  To completely remove the runner, [delete the runner in the runners administration page](https://docs.gitlab.com/ee/ci/runners/runners_scope.html#delete-shared-runners)
  or use the [`DELETE /runners`](https://docs.gitlab.com/ee/api/runners.html#delete-a-runner) REST API endpoint.

To unregister a single runner, first get the runner's details by executing
`gitlab-runner list`:

```plaintext
test-runner     Executor=shell Token=t0k3n URL=http://gitlab.example.com
```

Then use this information to unregister it, using one of the following commands.

WARNING:
This operation cannot be undone. It updates the configuration file, so
make sure to have a backup of `config.toml` before executing it.

#### By URL and token

```shell
gitlab-runner unregister --url "http://gitlab.example.com/" --token t0k3n
```

#### By name

```shell
gitlab-runner unregister --name test-runner
```

NOTE:
If there is more than one runner with the given name, only the first one is removed.

#### All runners

```shell
gitlab-runner unregister --all-runners
```

### `gitlab-runner reset-token`

This command resets a runner's token by using the GitLab Runners API, with
either the [runner ID](https://docs.gitlab.com/ee/api/runners.html#reset-runners-authentication-token-by-using-the-runner-id)
or the [current token](https://docs.gitlab.com/ee/api/runners.html#reset-runners-authentication-token-by-using-the-current-token).

It expects the runner's name (or URL and ID), and an optional PAT if
resetting by runner ID. The PAT and runner ID are intended to be used if the
token has already expired.

With the `--all-runners` option, it resets all the attached runners' tokens.

#### With runner's current token

```shell
gitlab-runner reset-token --name test-runner
```

#### With PAT and runner name

```shell
gitlab-runner reset-token --name test-runner --pat PaT
```

#### With PAT, GitLab URL, and runner ID

```shell
gitlab-runner reset-token --url "https://gitlab.example.com/" --id 12345 --pat PaT
```

#### All runners

```shell
gitlab-runners reset-token --all-runners
```

## Service-related commands

The following commands allow you to manage the runner as a system or user
service. Use them to install, uninstall, start, and stop the runner service.

- [`gitlab-runner install`](#gitlab-runner-install)
- [`gitlab-runner uninstall`](#gitlab-runner-uninstall)
- [`gitlab-runner start`](#gitlab-runner-start)
- [`gitlab-runner stop`](#gitlab-runner-stop)
- [`gitlab-runner restart`](#gitlab-runner-restart)
- [`gitlab-runner status`](#gitlab-runner-status)
- [Multiple services](#multiple-services)
- [**Access Denied** when running the service-related commands](#access-denied-when-running-the-service-related-commands)

All service related commands accept these arguments:

| Parameter   | Default                                           | Description                                |
| ----------- | ------------------------------------------------- | ------------------------------------------ |
| `--service` | `gitlab-runner`                                   | Specify custom service name                |
| `--config`  | See the [configuration file](#configuration-file) | Specify a custom configuration file to use |

### `gitlab-runner install`

This command installs GitLab Runner as a service. It accepts different sets of
arguments depending on which system it's run on.

When run on **Windows** or as super-user, it accepts the `--user` flag which
allows you to drop privileges of builds run with the **shell** executor.

| Parameter             | Default                                           | Description                                                                                         |
| --------------------- | ------------------------------------------------- | --------------------------------------------------------------------------------------------------- |
| `--service`           | `gitlab-runner`                                   | Specify service name to use                                                                         |
| `--config`            | See the [configuration file](#configuration-file) | Specify a custom configuration file to use                                                          |
| `--syslog`            | `true` (for non systemd systems)                  | Specify if the service should integrate with system logging service                                 |
| `--working-directory` | the current directory                             | Specify the root directory where all data is stored when builds are run with the **shell** executor |
| `--user`              | `root`                                            | Specify the user that executes the builds                                                           |
| `--password`          | none                                              | Specify the password for the user that executes the builds                                          |

### `gitlab-runner uninstall`

This command stops and uninstalls GitLab Runner from being run as an
service.

### `gitlab-runner start`

This command starts the GitLab Runner service.

### `gitlab-runner stop`

This command stops the GitLab Runner service.

### `gitlab-runner restart`

This command stops and then starts the GitLab Runner service.

### `gitlab-runner status`

This command prints the status of the GitLab Runner service. The exit code is zero when the service is running and non-zero when the service is not running.

### Multiple services

By specifying the `--service` flag, it is possible to have multiple GitLab
Runner services installed, with multiple separate configurations.

## Run-related commands

This command allows to fetch and process builds from GitLab.

### `gitlab-runner run`

The `gitlab-runner run` command is the main command that is executed when GitLab Runner is started as a
service. It reads all defined runners from `config.toml` and tries to run all
of them.

The command is executed and works until it [receives a signal](#signals).

It accepts the following parameters.

| Parameter             | Default                                       | Description                                                                                     |
| --------------------- | --------------------------------------------- | ----------------------------------------------------------------------------------------------- |
| `--config`            | See [configuration-file](#configuration-file) | Specify a custom configuration file to be used                                                  |
| `--working-directory` | the current directory                         | Specify the root directory where all data is stored when builds run with the **shell** executor |
| `--user`              | the current user                              | Specify the user that executes builds                                                           |
| `--syslog`            | `false`                                       | Send all logs to SysLog (Unix) or EventLog (Windows)                                            |
| `--listen-address`    | empty                                         | Address (`<host>:<port>`) on which the Prometheus metrics HTTP server should be listening       |

### `gitlab-runner run-single`

> - Ability to use a configuration file [introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/37670) in GitLab Runner 17.1.

Use this supplementary command to run a single build from
a single GitLab instance. It can:

- Take all options either as CLI parameters or environment variables, including the GitLab URL
  and Runner token. For example, a single job with all parameters specified explicitly:

  ```shell
  gitlab-runner run-single -u http://gitlab.example.com -t my-runner-token --executor docker --docker-image ruby:2.7
  ```

- Read from a configuration file to use a specific runner's configuration. For example,
  a single job with a configuration file:

  ```shell
  gitlab-runner run-single -c ~/.gitlab-runner/config.toml -r runner-name
  ```

You can see all possible configuration options by using the `--help` flag:

```shell
gitlab-runner run-single --help
```

You can use the `--max-builds` option to control how many builds the runner executes before exiting. The
default of `0` means that the runner has no build limit and jobs run forever.

You can also use the `--wait-timeout` option to control how long the runner waits for a job before
exiting. The default of `0` means that the runner has no timeout and waits forever between jobs.

## Internal commands

GitLab Runner is distributed as a single binary and contains a few internal
commands that are used during builds.

### `gitlab-runner artifacts-downloader`

Download the artifacts archive from GitLab.

### `gitlab-runner artifacts-uploader`

Upload the artifacts archive to GitLab.

### `gitlab-runner cache-archiver`

Create a cache archive, store it locally or upload it to an external server.

### `gitlab-runner cache-extractor`

Restore the cache archive from a locally or externally stored file.

## Troubleshooting

Below are some common pitfalls.

### **Access Denied** when running the service-related commands

Usually the [service related commands](#service-related-commands) require
administrator privileges:

- On Unix (Linux, macOS, FreeBSD) systems, prefix `gitlab-runner` with `sudo`
- On Windows systems use the elevated command prompt.
  Run an `Administrator` command prompt.
  To write `Command Prompt` in the Windows search field,
  right-click and select `Run as administrator`. Confirm
  that you want to execute the elevated command prompt.

## `gitlab-runner stop` doesn't shut down gracefully

When GitLab Runner is installed on a host and runs local executors, it starts additional processes for operations
like downloading or uploading artifacts, or handling cache.
These processes are executed as `gitlab-runner` commands, which means that you can use `pkill -QUIT gitlab-runner`
or `killall QUIT gitlab-runner` to kill them. When you kill them, the operations they are responsible for fail.

Here are two ways to prevent this:

- Register the runner as a local service (like `systemd`) with `SIGQUIT` as the kill
  signal, and use `gitlab-runner stop` or `systemctl stop gitlab-runner.service`.
  Here is an example configuration to enable this behavior:

  ```ini
  ; /etc/systemd/system/gitlab-runner.service.d/kill.conf
  [Service]
  KillSignal=SIGQUIT
  TimeoutStopSec=infinity
  ```

  - To apply the configuration change, after you create this file, reload `systemd` with `systemctl daemon-reload`.

- Manually kill the process with `kill -SIGQUIT <pid>`. You have to find the `pid`
  of the main `gitlab-runner` process. You can find this by looking at logs, as
  it's displayed on startup:

  ```shell
  $ gitlab-runner run
  Runtime platform                                    arch=arm64 os=linux pid=8 revision=853330f9 version=16.5.0
  ```

### Saving system ID state file: access denied

GitLab Runner 15.7 and 15.8 might not start if it lacks write permissions
for the directory that contains the `config.toml` file.

When GitLab Runner starts, it searches for the `.runner_system_id` file in the directory that contains
the `config.toml`. If it cannot find the `.runner_system_id` file, it creates a new one. If GitLab Runner
doesn't have write permissions, it fails to start.

To resolve this issue, temporarily allow file write permissions, then run `gitlab-runner run`.
After the `.runner_system_id` file is created, you can reset the permissions to read-only.
