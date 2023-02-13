---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# The Custom executor **(FREE)**

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/2885) in GitLab Runner 12.1

GitLab Runner provides the Custom executor for environments that it
doesn't support natively, for example, Podman or Libvirt.

This gives you the control to create your own executor by configuring
GitLab Runner to use some executable to provision, run, and clean up
your environment.

The scripts you configure for the custom executor are called `Drivers`.
For example, you could create a Podman driver, an [LXD driver](custom_examples/lxd.md) or a
[Libvirt driver](custom_examples/libvirt.md).

## Limitations

Below are some current limitations when using the Custom executor:

- No [Interactive Web Terminal](https://docs.gitlab.com/ee/ci/interactive_web_terminal/) support.

## Configuration

There are a few configuration keys that you can choose from. Some of them are optional.

Below is an example of configuration for the Custom executor using all available
configuration keys:

```toml
[[runners]]
  name = "custom"
  url = "https://gitlab.com"
  token = "TOKEN"
  executor = "custom"
  builds_dir = "/builds"
  cache_dir = "/cache"
  [runners.custom]
    config_exec = "/path/to/config.sh"
    config_args = [ "SomeArg" ]
    config_exec_timeout = 200

    prepare_exec = "/path/to/script.sh"
    prepare_args = [ "SomeArg" ]
    prepare_exec_timeout = 200

    run_exec = "/path/to/binary"
    run_args = [ "SomeArg" ]

    cleanup_exec = "/path/to/executable"
    cleanup_args = [ "SomeArg" ]
    cleanup_exec_timeout = 200

    graceful_kill_timeout = 200
    force_kill_timeout = 200
```

For field definitions and which ones are required, see
[`[runners.custom]`
section](../configuration/advanced-configuration.md#the-runnerscustom-section)
configuration.

In addition both `builds_dir` and `cache_dir` inside of the
[`[[runners]]`](../configuration/advanced-configuration.md#the-runners-section)
are required fields.

## Prerequisite software for running a Job

The user must set up the environment, including the following that must
be present in the `PATH`:

- [Git](https://git-scm.com/download) and [Git LFS](https://git-lfs.github.com/):
  see [common prerequisites](index.md#prerequisites-for-non-docker-executors).
- [GitLab Runner](../install/index.md): Used to
  download/update artifacts and cache.

## Stages

The Custom executor provides the stages for you to configure some
details of the job, prepare and clean up the environment and run the job
script within it. Each stage is responsible for specific things and has
different things to keep in mind.

Each stage executed by the Custom executor is executed at the time
a builtin GitLab Runner executor would execute them.

For each step that will be executed, specific environment variables are
exposed to the executable, which can be used to get information about
the specific Job that is running. All stages will have the following
environment variables available to them:

- Standard CI/CD [environment variables](https://docs.gitlab.com/ee/ci/variables/), including
  [predefined variables](https://docs.gitlab.com/ee/ci/variables/predefined_variables.html).
- All environment variables provided by the Custom executor Runner host system.
- All services and their [available settings](https://docs.gitlab.com/ee/ci/docker/using_docker_images.html#available-settings-for-services).
  Exposed in JSON format as `CUSTOM_ENV_CI_JOB_SERVICES`.

Both CI/CD environment variables and predefined variables are prefixed
with `CUSTOM_ENV_` to prevent conflicts with system environment
variables. For example, `CI_BUILDS_DIR` will be available as
`CUSTOM_ENV_CI_BUILDS_DIR`.

The stages run in the following sequence:

1. `config_exec`
1. `prepare_exec`
1. `run_exec`
1. `cleanup_exec`

### Services

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4358) in GitLab Runner 13.6

[Services](https://docs.gitlab.com/ee/ci/docker/using_docker_images.html#what-is-a-service) are exposed as a JSON array
as `CUSTOM_ENV_CI_JOB_SERVICES`.

Example:

```yaml
custom:
  script:
    - echo $CUSTOM_ENV_CI_JOB_SERVICES
  services:
    - redis:latest
    - name: my-postgres:9.4
      alias: pg
      entrypoint: ["path", "to", "entrypoint"]
      command: ["path", "to", "cmd"]
```

The example above will set `CUSTOM_ENV_CI_JOB_SERVICES` environment variable with the following value:

```json
[{"name":"redis:latest","alias":"","entrypoint":null,"command":null},{"name":"my-postgres:9.4","alias":"pg","entrypoint":["path","to","entrypoint"],"command":["path","to","cmd"]}]
```

### Config

The Config stage is executed by `config_exec`.

Sometimes you might want to set some settings during execution time. For
example settings a build directory depending on the project ID.
`config_exec` reads from STDOUT and expects a valid JSON string with
specific keys.

For example:

```shell
#!/usr/bin/env bash

cat << EOS
{
  "builds_dir": "/builds/${CUSTOM_ENV_CI_CONCURRENT_PROJECT_ID}/${CUSTOM_ENV_CI_PROJECT_PATH_SLUG}",
  "cache_dir": "/cache/${CUSTOM_ENV_CI_CONCURRENT_PROJECT_ID}/${CUSTOM_ENV_CI_PROJECT_PATH_SLUG}",
  "builds_dir_is_shared": true,
  "hostname": "custom-hostname",
  "driver": {
    "name": "test driver",
    "version": "v0.0.1"
  },
  "job_env" : {
    "CUSTOM_ENVIRONMENT": "example"
  }
}
EOS
```

Any additional keys inside of the JSON string will be ignored. If it's
not a valid JSON string the stage will fail and be retried two more
times.

| Parameter | Type | Required | Allowed empty | Description |
|-----------|------|----------|---------------|-------------|
| `builds_dir` | string | ✗ | ✗ | The base directory where the working directory of the job will be created. |
| `cache_dir` | string | ✗ | ✗ | The base directory where local cache will be stored. |
| `builds_dir_is_shared` | bool | ✗ | n/a | Defines whether the environment is shared between concurrent job or not. |
| `hostname` | string | ✗ | ✓ | The hostname to associate with job's "metadata" stored by the runner. If undefined, the hostname is not set. |
| `driver.name` | string | ✗ | ✓ | The user-defined name for the driver. Printed with the `Using custom executor...` line. If undefined, no information about driver is printed. |
| `driver.version` | string | ✗ | ✓ | The user-defined version for the drive. Printed with the `Using custom executor...` line. If undefined, only the name information is printed. |
| `job_env` | object | ✗ | ✓ |  Name-value pairs that are available through environment variables to all subsequent stages of the job execution. They are available for the driver, not the job. For details, see [`job_env` usage](#job_env-usage). |

The `STDERR` of the executable will print to the job log.

The user can set
[`config_exec_timeout`](../configuration/advanced-configuration.md#the-runnerscustom-section)
if they want to set a deadline for how long GitLab Runner should wait to
return the JSON string before terminating the process.

If any of the
[`config_exec_args`](../configuration/advanced-configuration.md#the-runnerscustom-section)
are defined, these will be added in order to the executable defined in
`config_exec`. For example we have the `config.toml` content below:

```toml
...
[runners.custom]
  ...
  config_exec = "/path/to/config"
  config_args = [ "Arg1", "Arg2" ]
  ...
```

GitLab Runner would execute it as `/path/to/config Arg1 Arg2`.

#### `job_env` usage

The main purpose of `job_env` configuration is to pass variables **to the context of custom executor driver calls**
for subsequent stages of the job execution.

Let's consider an example driver, where connection with the job execution environment requires preparing some
credentials and that this operation is very expensive. Let's say we need to connect to our local credentials
provider to get a temporary SSH username and password that the custom executor can next use to connect with the
job execution environment.

With Custom Executor execution flow, where each job execution [stage](#stages): `prepare`, multiple `run` calls
and `cleanup` are separate executions of the driver, the context is separate for each of them. For our credentials
resolving example, connection to the credentials provider needs to be done each time.

If this operation is expensive, we might want to do it once for a whole job execution, and then re-use the credentials
for all job execution stages. This is where the `job_env` can help. With this you can connect with the provider once,
during the `config_exec` call and then pass the received credentials with the `job_env`. They will be next added to the
list of variables that the custom executor calls for [`prepare_exec`](#prepare), [`run_exec`](#run) and [`cleanup_exec`](#cleanup) are receiving. With
this, the driver instead of connecting to the credentials provider each time may just read the variables and use the
credentials that are present.

The important thing to understand is that **the variables are not automaticaly available for the job itself**. It
fully depends on how the Custom Executor Driver is implemented and in many cases it will be not present there.

If you're considering the `job_env` setting so you can pass a set of variables to every job executed
by a particular runner, then look at the
[`environment` setting from `[[runners]]`](../configuration/advanced-configuration.md#the-runners-section).

If the variables are dynamic and it's expected that their values will change between different jobs, then you should
make sure that your driver is implemented in a way that the variables passed by `job_env` will be added to the job
execution call.

### Prepare

The Prepare stage is executed by `prepare_exec`.

At this point, GitLab Runner knows everything about the job (where and
how it's going to run). The only thing left is for the environment to be
set up so the job can run. GitLab Runner will execute the executable
that is specified in `prepare_exec`.

This is responsible for setting up the environment (for example,
creating the virtual machine or container, services or anything else). After
this is done, we expect that the environment is ready to run the job.

This stage is executed only once, in a job execution.

The user can set
[`prepare_exec_timeout`](../configuration/advanced-configuration.md#the-runnerscustom-section)
if they want to set a deadline for how long GitLab Runner
should wait to prepare the environment before terminating the process.

The `STDOUT` and `STDERR` returned from this executable will print to
the job log.

If any of the
[`prepare_exec_args`](../configuration/advanced-configuration.md#the-runnerscustom-section)
are defined, these will be added in order to the executable defined in
`prepare_exec`. For example we have the `config.toml` content below:

```toml
...
[runners.custom]
  ...
  prepare_exec = "/path/to/bin"
  prepare_args = [ "Arg1", "Arg2" ]
  ...
```

GitLab Runner would execute it as `/path/to/bin Arg1 Arg2`.

### Run

The Run stage is executed by `run_exec`.

The `STDOUT` and `STDERR` returned from this executable will print to
the job log.

Unlike the other stages, the `run_exec` stage is executed multiple
times, since it's split into sub stages listed below in sequential
order:

1. `prepare_script`
1. `get_sources`
1. `restore_cache`
1. `download_artifacts`
1. `step_*`
1. `build_script`
1. `step_*`
1. `after_script`
1. `archive_cache` OR `archive_cache_on_failure`
1. `upload_artifacts_on_success` OR `upload_artifacts_on_failure`
1. `cleanup_file_variables`

NOTE:
In GitLab Runner 14.0 and later, `build_script` will be replaced with `step_script`. For more information, see [this issue](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/26426).

For each stage mentioned above, the `run_exec` executable will be
executed with:

- The usual environment variables.
- Two arguments:
  - The path to the script that GitLab Runner creates for the Custom
    executor to run.
  - Name of the stage.

For example:

```shell
/path/to/run_exec.sh /path/to/tmp/script1 prepare_executor
/path/to/run_exec.sh /path/to/tmp/script1 prepare_script
/path/to/run_exec.sh /path/to/tmp/script1 get_sources
```

If you have `run_args` defined, they are the first set of arguments
passed to the `run_exec` executable, then GitLab Runner adds others. For
example, suppose we have the following `config.toml`:

```toml
...
[runners.custom]
  ...
  run_exec = "/path/to/run_exec.sh"
  run_args = [ "Arg1", "Arg2" ]
  ...
```

GitLab Runner will execute the executable with the following arguments:

```shell
/path/to/run_exec.sh Arg1 Arg2 /path/to/tmp/script1 prepare_executor
/path/to/run_exec.sh Arg1 Arg2 /path/to/tmp/script1 prepare_script
/path/to/run_exec.sh Arg1 Arg2 /path/to/tmp/script1 get_sources
```

This executable should be responsible for executing the scripts that are
specified in the first argument. They contain all the scripts any GitLab
Runner executor would run normally to clone, download artifacts, run
user scripts and all the other steps described below. The scripts can be
of the following shells:

- Bash
- PowerShell Desktop
- PowerShell Core
- Batch (deprecated)

We generate the script using the shell configured by `shell` inside of
[`[[runners]]`](../configuration/advanced-configuration.md#the-runners-section).
If none is provided the defaults for the OS platform are used.

The table below is a detailed explanation of what each script does and
what the main goal of that script is.

| Script Name | Script Contents |
|:-----------:|:---------------:|
| `prepare_script` | Simple debug information which machine the Job is running on. |
| `get_sources`    | Prepares the Git configuration, and clone/fetch the repository. We suggest you keep this as is since you get all of the benefits of Git strategies that GitLab provides. |
| `restore_cache` | Extract the cache if any are defined. This expects the `gitlab-runner` binary is available in `$PATH`. |
| `download_artifacts` | Download artifacts, if any are defined. This expects `gitlab-runner` binary is available in `$PATH`. |
| `step_*` | Generated by GitLab. A set of scripts to execute. It may never be sent to the custom executor. It may have multiple steps, like `step_release` and `step_accessibility`. This can be a feature from the `.gitlab-ci.yml` file. |
| `build_script` | A combination of [`before_script`](https://docs.gitlab.com/ee/ci/yaml/#before_script-and-after_script) and [`script`](https://docs.gitlab.com/ee/ci/yaml/#script). In GitLab Runner 14.0 and later, `build_script` will be replaced with `step_script`. For more information, see [this issue](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/26426). |
| `after_script` | This is the [`after_script`](https://docs.gitlab.com/ee/ci/yaml/#before_script-and-after_script) defined from the job. This is always called even if any of the previous steps failed. |
| `archive_cache` | Will create an archive of all the cache, if any are defined. Only executed when `build_script` was successful. |
| `archive_cache_on_failure` | Will create an archive of all the cache, if any are defined. Only executed when `build_script` fails. |
| `upload_artifacts_on_success` | Upload any artifacts that are defined. Only executed when `build_script` was successful. |
| `upload_artifacts_on_failure` | Upload any artifacts that are defined. Only executed when `build_script` fails. |
| `cleanup_file_variables` | Deletes all [file based](https://docs.gitlab.com/ee/ci/variables/#custom-environment-variables-of-type-file) variables from disk. |

### Cleanup

The Cleanup stage is executed by `cleanup_exec`.

This final stage is executed even if one of the previous stages failed.
The main goal for this stage is to clean up any of the environments that
might have been set up. For example, turning off VMs or deleting
containers.

The result of `cleanup_exec` does not affect job statuses. For example,
a job will be marked as successful even if the following occurs:

- Both `prepare_exec` and `run_exec` are successful.
- `cleanup_exec` fails.

The user can set
[`cleanup_exec_timeout`](../configuration/advanced-configuration.md#the-runnerscustom-section)
if they want to set some kind of deadline of how long GitLab Runner
should wait to clean up the environment before terminating the
process.

The `STDOUT` of this executable will be printed to GitLab Runner logs at a
DEBUG level. The `STDERR` will be printed to the logs at a WARN level.

If any of the
[`cleanup_exec_args`](../configuration/advanced-configuration.md#the-runnerscustom-section)
are defined, these will be added in order to the executable defined in
`cleanup_exec`. For example we have the `config.toml` content below:

```toml
...
[runners.custom]
  ...
  cleanup_exec = "/path/to/bin"
  cleanup_args = [ "Arg1", "Arg2" ]
  ...
```

GitLab Runner would execute it as `/path/to/bin Arg1 Arg2`.

## Terminating and killing executables

GitLab Runner will try to gracefully terminate an executable under any
of the following conditions:

- `config_exec_timeout`, `prepare_exec_timeout` or `cleanup_exec_timeout` are met.
- The job [times out](https://docs.gitlab.com/ee/ci/pipelines/settings.html#timeout).
- The job is cancelled.

When a timeout is reached, a `SIGTERM` is sent to the executable, and
the countdown for
[`exec_terminate_timeout`](../configuration/advanced-configuration.md#the-runnerscustom-section)
starts. The executable should listen to this signal to make sure it
cleans up any resources. If `exec_terminate_timeout` passes and the
process is still running, a `SIGKILL` is sent to kill the process and
[`exec_force_kill_timeout`](../configuration/advanced-configuration.md#the-runnerscustom-section)
will start. If the process is still running after
`exec_force_kill_timeout` has finished, GitLab Runner will abandon the
process and will not try to stop/kill anymore. If both these timeouts
are reached during `config_exec`, `prepare_exec` or `run_exec` the build
is marked as failed.

As of [GitLab 13.1](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/1743)
any child process that is spawned by the driver will also receive the
graceful termination process explained above on UNIX based systems. This
is achieved by having the main process set as a [process group](https://man7.org/linux/man-pages/man2/setpgid.2.html)
which all the child processes belong too.

## Error handling

There are two types of errors that GitLab Runner can handle differently.
These errors are only handled when the executable inside of
`config_exec`, `prepare_exec`, `run_exec`, and `cleanup_exec` exits with
these codes. If the user exits with a non-zero exit code, it should be
propagated as one of the error codes below.

If the user script exits with one of these code it has to
be propagated to the executable exit code.

### Build Failure

GitLab Runner provides `BUILD_FAILURE_EXIT_CODE` environment
variable which should be used by the executable as an exit code to
inform GitLab Runner that there is a failure on the users job. If the
executable exits with the code from
`BUILD_FAILURE_EXIT_CODE`, the build is marked as a failure
appropriately in GitLab CI.

If the script that the user defines inside of `.gitlab-ci.yml` file
exits with a non-zero code, `run_exec` should exit with
`BUILD_FAILURE_EXIT_CODE` value.

NOTE:
We strongly suggest using `BUILD_FAILURE_EXIT_CODE` to exit
instead of a hard coded value since it can change in any release, making
your binary/script future proof.

### System Failure

You can send a system failure to GitLab Runner by exiting the process with the
error code specified in the `SYSTEM_FAILURE_EXIT_CODE`. If this error
code is returned, on certain stages GitLab Runner will retry the stage, if none
of the retries are successful the job will be marked as failed.

Below is a table of what stages are retried, and by how many times.

| Stage Name           | Number of attempts                                          | Duration to wait between each retry |
|----------------------|-------------------------------------------------------------|-------------------------------------|
| `prepare_exec`       | 3                                                           | 3 seconds                           |
| `get_sources`        | Value of `GET_SOURCES_ATTEMPTS` variable. (Default 1)       | 0 seconds                           |
| `restore_cache`      | Value of `RESTORE_CACHE_ATTEMPTS` variable. (Default 1)     | 0 seconds                           |
| `download_artifacts` | Value of `ARTIFACT_DOWNLOAD_ATTEMPTS` variable. (Default 1) | 0 seconds                           |

NOTE:
We strongly suggest using `SYSTEM_FAILURE_EXIT_CODE` to exit
instead of a hard coded value since it can change in any release, making
your binary/script future proof.

## Job response

You can change job-level `CUSTOM_ENV_` variables as they observe the documented
[CI/CD variable precedence](https://docs.gitlab.com/ee/ci/variables/#cicd-variable-precedence).
Though this functionality can be desirable, when the trusted job context
is required, the full JSON job response is provided automatically. The runner
generates a temporary file, which is referenced in the `JOB_RESPONSE_FILE`
environment variable. This file exists in every stage and is automatically
removed during cleanup.

```shell
$ cat ${JOB_RESPONSE_FILE}
{"id": 123456, "token": "jobT0ken",...}
```
