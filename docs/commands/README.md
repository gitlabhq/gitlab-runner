# GitLab Runner commands

GitLab Runner contains a set of commands with which you register, manage and
run your builds.

You can check a recent list of commands by executing:

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

The output would be similar to:

```plaintext
NAME:
   gitlab-runner run - run multi runner service

USAGE:
   gitlab-runner run [command options] [arguments...]

OPTIONS:
   -c, --config "/Users/ayufan/.gitlab-runner/config.toml"      Config file [$CONFIG_FILE]
```

## Running in debug mode

Debug mode is especially useful when looking for the cause of some undefined
behavior or error.

To run a command in debug mode, prepend the command with `--debug`:

```shell
gitlab-runner --debug <command>
```

## Super-user permission

Commands that access the configuration of GitLab Runner behave differently when
executed as super-user (`root`). The file location depends on the user executing
the command.

Be aware of the notice that is written when executing the commands that are
used for running builds, registering services or managing registered runners:

```shell
$ gitlab-runner run

INFO[0000] Starting multi-runner from /Users/ayufan/.gitlab-runner/config.toml ...  builds=0
WARN[0000] Running in user-mode.
WARN[0000] Use sudo for system-mode:
WARN[0000] $ sudo gitlab-runner...
```

You should use `user-mode` if you are really sure that this is a mode that you
want to work with. Otherwise, prefix your command with `sudo`:

```shell
$ sudo gitlab-runner run

INFO[0000] Starting multi-runner from /etc/gitlab-runner/config.toml ...  builds=0
INFO[0000] Running in system-mode.
```

In the case of **Windows** you may need to run the **Command Prompt** in
**Administrative Mode**.

## Configuration file

GitLab Runner configuration uses the [TOML](https://github.com/toml-lang/toml) format.

The file to be edited can be found in:

1. `/etc/gitlab-runner/config.toml` on \*nix systems when GitLab Runner is
   executed as super-user (`root`)
1. `~/.gitlab-runner/config.toml` on \*nix systems when GitLab Runner is
   executed as non-root
1. `./config.toml` on other systems

Most of the commands accept an argument to specify a custom configuration file,
allowing you to have a multiple different configurations on a single machine.
To specify a custom configuration file use the `-c` or `--config` flag, or use
the `CONFIG_FILE` environment variable.

## Signals

It is possible to use system signals to interact with GitLab Runner. The
following commands support the following signals:

| Command | Signal | Action |
|---------|--------|--------|
| `register` | **SIGINT** | Cancel runner registration and delete if it was already registered |
| `run`, `exec`, `run-single` | **SIGINT**, **SIGTERM** | Abort all running builds and exit as soon as possible. Use twice to exit now (**forceful shutdown**). |
| `run`, `exec`, `run-single` | **SIGQUIT** | Stop accepting a new builds. Exit as soon as currently running builds do finish (**graceful shutdown**). |
| `run` | **SIGHUP** | Force to reload configuration file |

For example, to force a reload of the Runner's configuration file, run

```shell
sudo kill -SIGHUP <main_runner_pid>
```

For [graceful shutdowns](../best_practice/index.md#graceful-shutdown):

```shell
sudo kill -SIGQUIT <main_runner_pid>
```

CAUTION: **Warning**:
Do **not** use `killall` or `pkill` for graceful shutdowns if you are using `shell`
or `docker` executors. This can cause improper handling of the signals due to subprocessess
being killed as well. Use it only on the main process handling the jobs.

If your operating system is configured to automatically restart the service if it fails (which is the default on some platforms) it may automatically restart the runner if it's shut down by the signals above.

## Commands overview

This is what you see if you run `gitlab-runner` without any arguments:

```plaintext
NAME:
   gitlab-runner - a GitLab Runner

USAGE:
   gitlab-runner [global options] command [command options] [arguments...]

VERSION:
   13.0.0 (HEAD)

AUTHOR:
   GitLab Inc. <support@gitlab.com>

COMMANDS:
     exec                  execute a build locally
     list                  List all configured runners
     run                   run multi runner service
     register              register a new runner
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
     help, h               Shows a list of commands or help for one command
```

Below we will explain what each command does in detail.

## Registration-related commands

The following commands allow you to register a new runner, or list and verify
them if they are still registered.

- [`gitlab-runner register`](#gitlab-runner-register)
  - [Interactive registration](#interactive-registration)
  - [Non-interactive registration](#non-interactive-registration)
- [`gitlab-runner list`](#gitlab-runner-list)
- [`gitlab-runner verify`](#gitlab-runner-verify)
- [`gitlab-runner unregister`](#gitlab-runner-unregister)

The above commands support the following arguments:

| Parameter   | Default | Description |
|-------------|---------|-------------|
| `--config`  | See the [configuration file section](#configuration-file) | Specify a custom configuration file to be used |

### `gitlab-runner register`

This command registers your GitLab Runner in GitLab using the GitLab [Runners API](https://docs.gitlab.com/ee/api/runners.html#register-a-new-runner).

The registered runner is
added to the [configuration file](#configuration-file).
You can use multiple configurations in a single GitLab Runner. Executing
`gitlab-runner register` adds a new configuration entry, it doesn't remove the
previous ones.

There are two options to register a Runner:

- interactive.
- non-interactive.

NOTE: **Note:**
Runners can be registered directly using the GitLab [Runners API](https://docs.gitlab.com/ee/api/runners.html#register-a-new-runner) but
configuration will not be generated automatically.

#### Interactive registration

This command is usually used in interactive mode (**default**). You will be
asked multiple questions during a Runner's registration.

This question can be pre-filled by adding arguments when invoking the registration command:

```shell
gitlab-runner register --name my-runner --url http://gitlab.example.com --registration-token my-registration-token
```

Or by configuring the environment variable before the `register` command:

```shell
export CI_SERVER_URL=http://gitlab.example.com
export RUNNER_NAME=my-runner
export REGISTRATION_TOKEN=my-registration-token
export REGISTER_NON_INTERACTIVE=true
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

> **Note:** Boolean parameters must be passed in the command line with `--key={true|false}`.

#### `[[runners]]` configuration template file

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4228) in GitLab Runner 12.2.

Additional options can be easily configured during Runner registration using the
[configuration template file](../register/index.md#runners-configuration-template-file) feature.

### `gitlab-runner list`

This command lists all runners saved in the
[configuration file](#configuration-file).

### `gitlab-runner verify`

This command checks if the registered runners can connect to GitLab, but it
doesn't verify if the runners are being used by the GitLab Runner service. An
example output is:

```plaintext
Verifying runner... is alive                        runner=fee9938e
Verifying runner... is alive                        runner=0db52b31
Verifying runner... is alive                        runner=826f687f
Verifying runner... is alive                        runner=32773c0f
```

To delete the old and removed from GitLab runners, execute the following
command.

>**Warning:**
This operation cannot be undone, it will update the configuration file, so
make sure to have a backup of `config.toml` before executing it.

```shell
gitlab-runner verify --delete
```

### `gitlab-runner unregister`

This command unregisters registered runners using the GitLab [Runners API](https://docs.gitlab.com/ee/api/runners.html#delete-a-registered-runner).

It expects either:

- A full URL and the runner's token.
- The runner's name.

With the `--all-runners` option, it will unregister all the attached runners.

NOTE: **Note:**
Runners can be unregistered directly using the GitLab [Runners API](https://docs.gitlab.com/ee/api/runners.html#delete-a-registered-runner) but
configuration will not be modified for the user.

To unregister a specific runner, first get the runner's details by executing
`gitlab-runner list`:

```plaintext
test-runner     Executor=shell Token=t0k3n URL=http://gitlab.example.com
```

Then use this information to unregister it, using one of the following commands.

>**Warning:**
This operation cannot be undone, it will update the configuration file, so
make sure to have a backup of `config.toml` before executing it.

#### By URL and token

```shell
gitlab-runner unregister --url http://gitlab.example.com/ --token t0k3n
```

#### By name

> **Note:** If there is more than one runner with the given name, only the first one will be removed

```shell
gitlab-runner unregister --name test-runner
```

#### All Runners

```shell
gitlab-runner unregister --all-runners
```

## Service-related commands

The following commands allow you to manage the runner as a system or user
service. Use them to install, uninstall, start and stop the runner service.

- [`gitlab-runner install`](#gitlab-runner-install)
- [`gitlab-runner uninstall`](#gitlab-runner-uninstall)
- [`gitlab-runner start`](#gitlab-runner-start)
- [`gitlab-runner stop`](#gitlab-runner-stop)
- [`gitlab-runner restart`](#gitlab-runner-restart)
- [`gitlab-runner status`](#gitlab-runner-status)
- [Multiple services](#multiple-services)
- [**Access Denied** when running the service-related commands](#access-denied-when-running-the-service-related-commands)

All service related commands accept these arguments:

| Parameter   | Default                                           | Description |
|-------------|---------------------------------------------------|-------------|
| `--service` | `gitlab-runner`                                   | Specify custom service name |
| `--config`  | See the [configuration file](#configuration-file) | Specify a custom configuration file to use |

### `gitlab-runner install`

This command installs GitLab Runner as a service. It accepts different sets of
arguments depending on which system it's run on.

When run on **Windows** or as super-user, it accepts the `--user` flag which
allows you to drop privileges of builds run with the **shell** executor.

| Parameter             | Default               | Description |
|-----------------------|-----------------------|-------------|
| `--service`           | `gitlab-runner`       | Specify service name to use |
| `--config`           | See the [configuration file](#configuration-file) | Specify a custom configuration file to use |
| `--syslog`            | `true`                | Specify if the service should integrate with system logging service |
| `--working-directory` | the current directory | Specify the root directory where all data will be stored when builds will be run with the **shell** executor |
| `--user`              | `root`                | Specify the user which will be used to execute builds |
| `--password`          | none                  | Specify the password for the user that will be used to execute the builds |

### `gitlab-runner uninstall`

This command stops and uninstalls the GitLab Runner from being run as an
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

This is main command that is executed when GitLab Runner is started as a
service. It reads all defined Runners from `config.toml` and tries to run all
of them.

The command is executed and works until it [receives a signal](#signals).

It accepts the following parameters.

| Parameter | Default | Description |
|-----------|---------|-------------|
| `--config`  | See [configuration-file](#configuration-file) | Specify a custom configuration file to be used |
| `--working-directory` | the current directory | Specify the root directory where all data will be stored when builds will be run with the **shell** executor |
| `--user`    | the current user | Specify the user that will be used to execute builds |
| `--syslog`  | `false` | Send all logs to SysLog (Unix) or EventLog (Windows) |
| `--listen-address` | empty | Address (`<host>:<port>`) on which the Prometheus metrics HTTP server should be listening |

### `gitlab-runner run-single`

This is a supplementary command that can be used to run only a single build
from a single GitLab instance. It doesn't use any configuration file and
requires to pass all options either as parameters or environment variables.
The GitLab URL and Runner token need to be specified too.

For example:

```shell
gitlab-runner run-single -u http://gitlab.example.com -t my-runner-token --executor docker --docker-image ruby:2.6
```

You can see all possible configuration options by using the `--help` flag:

```shell
gitlab-runner run-single --help
```

You can use the `--max-builds` option to control how many builds the runner will execute before exiting. The
default of `0` means that the runner has no build limit and will run jobs forever.

You can also use the `--wait-timeout` option to control how long the runner will wait for a job before
exiting. The default of `0` means that the runner has no timeout and will wait forever between jobs.

### `gitlab-runner exec`

> Notice: Not all features of `.gitlab-ci.yml` are supported by `exec`. Please
check what exactly is supported in the [limitations of `gitlab-runner exec`](#limitations-of-gitlab-runner-exec)
section.

This command allows you to run builds locally, trying to replicate the CI
environment as much as possible. It doesn't need to connect to GitLab, instead
it reads the local `.gitlab-ci.yml` and creates a new build environment in
which all the build steps are executed.

This command is useful for fast checking and verifying `.gitlab-ci.yml` as well
as debugging broken builds since everything is run locally.

When executing `exec` you need to specify the executor and the job name that is
present in `.gitlab-ci.yml`. The command should be executed from the root
directory of your Git repository that contains `.gitlab-ci.yml`.

`gitlab-runner exec` will clone the current state of the local Git repository.
Make sure you have committed any changes you want to test beforehand.

For example, the following command will execute the job named **tests** locally
using a shell executor:

```shell
gitlab-runner exec shell tests
```

To see a list of available executors, run:

```shell
gitlab-runner exec
```

To see a list of all available options for the `shell` executor, run:

```shell
gitlab-runner exec shell
```

If you want to use the `docker` executor with the `exec` command, use that in
context of `docker-machine shell` or `boot2docker shell`. This is required to
properly map your local directory to the directory inside the Docker container.

#### Limitations of `gitlab-runner exec`

With current implementation of `exec` some of the features of GitLab CI will
not work or may work partially.

We're currently thinking about how to replace current `exec` implementation,
to make fully compatible with all features. Please track [the issue](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/2797)
for more details.

**Compatibility table - features based on `.gitlab-ci.yml`**

| GitLab CI feature | Available with `exec` | Comments |
|-------------------|-----------------------|----------|
| image             | yes                   | extended configuration (`name`, `entrypoint`) are also supported |
| services          | yes                   | extended configuration (`name`, `alias`, `entrypoint`, `command`) are also supported |
| stages            | no                    | `exec` can only run one job, independently from others |
| before_script     | yes                   | supports both global and job-level `before_script` |
| after_script      | partially             | global `after_script` is not supported, only job-level `after_script`; only commands are taken in consideration, `when` is hardcoded to `always` |
| variables         | yes                   | Supports default (partially), global and job-level variables; default variables are pre-set as can be seen in <https://gitlab.com/gitlab-org/gitlab-runner/blob/master/helpers/gitlab_ci_yaml_parser/parser.go#L147> |
| artifacts         | no                    |          |
| cache             | partially             | Regarding the specific configuration it may or may not work as expected |
| cache:policy      | no                    |          |
| environment       | no                    |          |
| only              | no                    |          |
| except            | no                    |          |
| `allow_failure`   | no                    | `exec` just exits with the result of job; it's callers responsibility to decide if failure is OK or not |
| tags              | no                    |          |
| when              | no                    |          |
| dependencies      | no                    |          |
| coverage          | no                    |          |
| retry             | no                    |          |
| hidden keys       | no                    | If explicitly asked to run, `exec` will try to run such job |
| YAML features     | yes                   | Anchors (`&`), aliases (`*`), map merging (`<<`) are part of YAML specification and are handled by the parser |
| pages             | partially             | Job's script will be executed if explicitly asked, but it doesn't affect pages state, which is managed by GitLab |

**Compatibility table - features based on variables**

| GitLab CI feature          | Available with `exec` | Comments |
|----------------------------|-----------------------|----------|
| GIT_STRATEGY               | yes                   |          |
| GIT_CHECKOUT               | yes                   |          |
| GIT_SUBMODULE_STRATEGY     | yes                   |          |
| GET_SOURCES_ATTEMPTS       | yes                   |          |
| ARTIFACT_DOWNLOAD_ATTEMPTS | no                    | artifacts are not supported |
| RESTORE_CACHE_ATTEMPTS     | yes                   |          |
| GIT_DEPTH                  | yes                   |          |

**Compatibility table - other features**

| GitLab CI feature | Available with `exec` | Comments |
|-------------------|-----------------------|----------|
| Secret Variables  | no                    |          |
| triggers          | no                    |          |
| schedules         | no                    |          |
| job timeout       | no                    | hardcoded to 1 hour |
| `[ci skip]`       | no                    |          |

**Other requirements and limitations**

`gitlab-runner exec docker` can only be used when Docker is installed locally.
This is needed because GitLab Runner is using host-bind volumes to access the
Git sources.

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
  The simplest way is to write `Command Prompt` in the Windows search field,
  right click and select `Run as administrator`. You will be asked to confirm
  that you want to execute the elevated command prompt.
