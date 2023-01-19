---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/product/ux/technical-writing/#assignments
comments: false
---

# Registering runners (deprecated) **(FREE)**

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/3414) in GitLab Runner 15.0, a change to the registration request format prevents the GitLab Runner from communicating with GitLab versions lower than 14.8. You must use a Runner version that is appropriate for the GitLab version, or upgrade the GitLab application.

WARNING:
The ability to pass a runner registration token was [deprecated](https://gitlab.com/gitlab-org/gitlab/-/issues/380872) in GitLab 15.6 and is
planned for removal in 17.0, along with support for certain configuration arguments. This change is a breaking change. GitLab plans to introduce a new
[GitLab Runner token architecture](https://docs.gitlab.com/ee/architecture/blueprints/runner_tokens/), which introduces
a new method for registering runners and eliminates the legacy
[runner registration token](https://docs.gitlab.com/ee/security/token_overview.html#runner-registration-tokens).

Registering a runner is the process that binds the runner with one or more GitLab instances.

You can register multiple runners on the same host machine,
each with a different configuration, by repeating the `register` command.

## Requirements

Before registering a runner, you must first:

- [Install it](../install/index.md) on a server separate than where GitLab
  is installed
- Obtain a token:
  - For a [shared runner](https://docs.gitlab.com/ee/ci/runners/runners_scope.html#shared-runners),
    have an administrator go to the GitLab Admin Area and select **Overview > Runners**
  - For a [group runner](https://docs.gitlab.com/ee/ci/runners/runners_scope.html#group-runners),
    go to **CI/CD > Runners**
  - For a [project-specific runner](https://docs.gitlab.com/ee/ci/runners/runners_scope.html#specific-runners),
    go to **Settings > CI/CD** and expand the **Runners** section

NOTE:
When registering a runner on GitLab.com, the `gitlab-ci coordinator URL`
is `https://gitlab.com`.

## Docker

The instructions in this section are meant to be used *after* you
[install GitLab Runner in a container](../install/docker.md).

The following steps describe launching a short-lived `gitlab-runner` container to
register the container you created during install. After you finish
registration, the resulting configuration is written to your chosen
configuration volume (for example, `/srv/gitlab-runner/config`) and is
loaded by the runner using that configuration volume.

To register a runner using a Docker container:

1. Run the register command based on the mount type:

   For local system volume mounts:

   ```shell
   docker run --rm -it -v /srv/gitlab-runner/config:/etc/gitlab-runner gitlab/gitlab-runner register
   ```

   NOTE:
   If you used a configuration volume other than `/srv/gitlab-runner/config`
   during install, be sure to update the command with the correct volume.

   For Docker volume mounts:

   ```shell
   docker run --rm -it -v gitlab-runner-config:/etc/gitlab-runner gitlab/gitlab-runner:latest register
   ```

1. Enter your GitLab instance URL (also known as the `gitlab-ci coordinator URL`).
1. Enter the token you obtained to register the runner.
1. Enter a description for the runner. You can change this value later in the
   GitLab user interface.
1. Enter the [tags associated with the runner](https://docs.gitlab.com/ee/ci/runners/configure_runners.html#use-tags-to-control-which-jobs-a-runner-can-run),
   separated by commas. You can change this value later in the GitLab user
   interface.
1. Enter any optional maintenance note for the runner.
1. Provide the [runner executor](../executors/index.md). For most use cases, enter
   `docker`.
1. If you entered `docker` as your executor, you are asked for the default
   image to be used for projects that do not define one in `.gitlab-ci.yml`.

## Linux

To register a runner under Linux:

1. Run the following command:

   ```shell
   sudo gitlab-runner register
   ```

   If you are behind a proxy, add an environment variable and then run the
   registration command:

   ```shell
   export HTTP_PROXY=http://yourproxyurl:3128
   export HTTPS_PROXY=http://yourproxyurl:3128

   sudo -E gitlab-runner register
   ```

1. Enter your GitLab instance URL (also known as the `gitlab-ci coordinator URL`).
1. Enter the token you obtained to register the runner.
1. Enter a description for the runner. You can change this value later in the
   GitLab user interface.
1. Enter the [tags associated with the runner](https://docs.gitlab.com/ee/ci/runners/configure_runners.html#use-tags-to-control-which-jobs-a-runner-can-run),
   separated by commas. You can change this value later in the GitLab user
   interface.
1. Enter any optional maintenance note for the runner.
1. Provide the [runner executor](../executors/index.md). For most use cases, enter
   `docker`.
1. If you entered `docker` as your executor, you are asked for the default
   image to be used for projects that do not define one in `.gitlab-ci.yml`.

## macOS

NOTE:
Install [Docker.app](https://docs.docker.com/desktop/install/mac-install/)
before registering a runner under macOS.

To register a runner under macOS:

1. Run the following command:

   ```shell
   gitlab-runner register
   ```

1. Enter your GitLab instance URL (also known as the `gitlab-ci coordinator URL`).
1. Enter the token you obtained to register the runner.
1. Enter a description for the runner. You can change this value later in the
   GitLab user interface.
1. Enter the [tags associated with the runner](https://docs.gitlab.com/ee/ci/runners/configure_runners.html#use-tags-to-control-which-jobs-a-runner-can-run),
   separated by commas. You can change this value later in the GitLab user
   interface.
1. Enter any optional maintenance note for the runner.
1. Provide the [runner executor](../executors/index.md). For most use cases, enter
   `docker`.
1. If you entered `docker` as your executor, you'll be asked for the default
   image to be used for projects that do not define one in `.gitlab-ci.yml`.

## Windows

To register a runner under Windows:

1. Run the following command:

   ```shell
   .\gitlab-runner.exe register
   ```

1. Enter your GitLab instance URL (also known as the `gitlab-ci coordinator URL`).
1. Enter the token you obtained to register the runner.
1. Enter a description for the runner. You can change this value later in the
   GitLab user interface.
1. Enter the [tags associated with the runner](https://docs.gitlab.com/ee/ci/runners/configure_runners.html#use-tags-to-control-which-jobs-a-runner-can-run),
   separated by commas. You can change this value later in the GitLab user
   interface.
1. Enter any optional maintenance note for the runner.
1. Provide the [runner executor](../executors/index.md). For most use cases, enter
   `docker`.
1. If you entered `docker` as your executor, you are asked for the default
   image to be used for projects that do not define one in `.gitlab-ci.yml`.

## FreeBSD

To register a runner under FreeBSD:

1. Run the following command:

   ```shell
   sudo -u gitlab-runner -H /usr/local/bin/gitlab-runner register
   ```

1. Enter your GitLab instance URL (also known as the `gitlab-ci coordinator URL`).
1. Enter the token you obtained to register the runner.
1. Enter a description for the runner. You can change this value later in the
   GitLab user interface.
1. Enter the [tags associated with the runner](https://docs.gitlab.com/ee/ci/runners/configure_runners.html#use-tags-to-control-which-jobs-a-runner-can-run),
   separated by commas. You can change this value later in the GitLab user
   interface.
1. Enter any optional maintenance note for the runner.
1. Provide the [runner executor](../executors/index.md). For most use cases, enter
   `docker`.
1. If you entered `docker` as your executor, you are asked for the default
   image to be used for projects that do not define one in `.gitlab-ci.yml`.

## One-line registration command

If you want to use the non-interactive mode to register a runner, you can
either use the `register` subcommands or use their equivalent environment
variables.

To display a list of all the `register` subcommands, run the following command:

```shell
gitlab-runner register -h
```

To register a runner using the most common options, you would do:

```shell
sudo gitlab-runner register \
  --non-interactive \
  --url "https://gitlab.com/" \
  --registration-token "PROJECT_REGISTRATION_TOKEN" \
  --executor "docker" \
  --docker-image alpine:latest \
  --description "docker-runner" \
  --maintenance-note "Free-form maintainer notes about this runner" \
  --tag-list "docker,aws" \
  --run-untagged="true" \
  --locked="false" \
  --access-level="not_protected"
```

If you're running the runner in a Docker container, the `register` command
is structured similar to the following:

```shell
docker run --rm -v /srv/gitlab-runner/config:/etc/gitlab-runner gitlab/gitlab-runner register \
  --non-interactive \
  --executor "docker" \
  --docker-image alpine:latest \
  --url "https://gitlab.com/" \
  --registration-token "PROJECT_REGISTRATION_TOKEN" \
  --description "docker-runner" \
  --maintenance-note "Free-form maintainer notes about this runner" \
  --tag-list "docker,aws" \
  --run-untagged="true" \
  --locked="false" \
  --access-level="not_protected"
```

The `--access-level` parameter was added in GitLab Runner 12.0. It uses a registration API parameter introduced in GitLab 11.11.
Use this parameter during registration to create a [protected runner](https://docs.gitlab.com/ee/ci/runners/configure_runners.html#prevent-runners-from-revealing-sensitive-information).
For a protected runner, use the `--access-level="ref_protected"` parameter.
For an unprotected runner, use `--access-level="not_protected"` instead or leave the value undefined.
This value can later be toggled on or off in the project's **Settings > CI/CD** menu.

The `--maintenance-note` parameter was [added](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/3268) in GitLab Runner 14.8.
You can use it to add information related to runner maintenance. The maximum allowed length is 255 characters.

## `Check registration token` error

The `check registration token` error message is displayed when the GitLab instance does not recognize
the entered registration token. This issue can occur when the instance group or project registration token
has been changed in GitLab or when the user did not correctly enter the registration token.

When this error occurs, the first step is to ask a GitLab administrator to verify that the registration token is valid.

## `[[runners]]` configuration template file

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4228) in GitLab Runner 12.2.

Some runner configuration settings can't be set with environment variables or command line options.

For example:

- Environment variables do not support slices.
- Command line option support is intentionally unavailable for the settings for the
  whole Kubernetes executor volumes tree.

This is a problem for environments that are handled by any kind of automation, such as the
[GitLab Runner official Helm chart](../install/kubernetes.md). In cases like these, the only solution was
to manually update the `config.toml` file after the runner was registered. This is less
than ideal, error-prone, and not reliable. Especially when more than one registration
for the same GitLab Runner installation is done.

This problem can be resolved with the usage of a _configuration template file_.

To use a configuration template file, pass a path to the file to `register` with either
the:

- `--template-config` command line option.
- `TEMPLATE_CONFIG_FILE` environment variable.

The configuration template file supports:

- Only a single
  [`[[runners]]`](../configuration/advanced-configuration.md#the-runners-section)
  section.
- No global options.

When `--template-config` or `TEMPLATE_CONFIG_FILE` is used, the configuration of `[[runners]]` entry
is merged into the configuration of newly created `[[runners]]` entry in the regular `config.toml`
file.

The merging is done only for options that were _empty_. That is:

- Empty strings.
- Nulls or/non existent entries.
- Zeroes.

With this:

- All configuration provided with command line options and/or environment variables during the
  `register` command call take precedence.
- The template fills the gaps and adds additional settings.

### Example

We register a Kubernetes-executor-based runner to some test project and see what the
`config.toml` file looks like:

```shell
$ sudo gitlab-runner register \
     --config /tmp/test-config.toml \
     --non-interactive \
     --url https://gitlab.com \
     --registration-token __REDACTED__ \
     --name test-runner \
     --tag-list kubernetes,test \
     --locked \
     --paused \
     --executor kubernetes \
     --kubernetes-host http://localhost:9876/

Runtime platform                                    arch=amd64 os=linux pid=1684 revision=88310882 version=11.10.0~beta.1251.g88310882

Registering runner... succeeded                     runner=__REDACTED__
Runner registered successfully. Feel free to start it, but if it's running already the config should be automatically reloaded!
```

The command above creates the following `config.toml` file:

```toml
concurrent = 1
check_interval = 0

[session_server]
  session_timeout = 1800

[[runners]]
  name = "test-runner"
  url = "https://gitlab.com"
  token = "__REDACTED__"
  executor = "kubernetes"
  [runners.cache]
    [runners.cache.s3]
    [runners.cache.gcs]
  [runners.kubernetes]
    host = "http://localhost:9876/"
    bearer_token_overwrite_allowed = false
    image = ""
    namespace = ""
    namespace_overwrite_allowed = ""
    privileged = false
    service_account_overwrite_allowed = ""
    pod_labels_overwrite_allowed = ""
    pod_annotations_overwrite_allowed = ""
    [runners.kubernetes.volumes]
```

We can see the basic configuration created from the provided command line options:

- Runner credentials (URL and token).
- The executor specified.
- The default, empty section `runners.kubernetes` with only the one option
  provided during the registration filled out.

Normally one would have to set few more options to make the Kubernetes executor
usable, but the above is enough for the purpose of our example.

Let's now assume that we have to configure an `emptyDir` volume for our Kubernetes executor. There is
no way to add this while registering with neither environment variables nor command line options.
We would have to **manually append** something like this to the end of the file:

```toml
[[runners.kubernetes.volumes.empty_dir]]
  name = "empty_dir"
  mount_path = "/path/to/empty_dir"
  medium = "Memory"
```

Because [TOML](https://github.com/toml-lang/toml) doesn't require proper indentation (it
relies on entries ordering), we could just append the required changes to the end of the
file.
​
However, this becomes tricky when more `[[runners]]` sections are being registered
within one `config.toml` file. The assumption that the new one is always at the
end is risky.

With GitLab Runner 12.2, this becomes much easier using the `--template-config` flag.

```shell
$ cat > /tmp/test-config.template.toml << EOF
[[runners]]
  [runners.kubernetes]
    [runners.kubernetes.volumes]
      [[runners.kubernetes.volumes.empty_dir]]
        name = "empty_dir"
        mount_path = "/path/to/empty_dir"
        medium = "Memory"
EOF
```

Having the file, we can now try to register the runner again, but this time adding the
`--template-config /tmp/test-config.template.toml` option. Apart from this change, the
rest of registration command is exactly the same:

```shell
$ sudo gitlab-runner register \
     --config /tmp/test-config.toml \
     --template-config /tmp/test-config.template.toml \
     --non-interactive \
     --url https://gitlab.com \
     --registration-token __REDACTED__ \
     --name test-runner \
     --tag-list kubernetes,test \
     --locked \
     --paused \
     --executor kubernetes \
     --kubernetes-host http://localhost:9876/

Runtime platform                                    arch=amd64 os=linux pid=8798 revision=88310882 version=11.10.0~beta.1251.g88310882

Registering runner... succeeded                     runner=__REDACTED__
Merging configuration from template file
Runner registered successfully. Feel free to start it, but if it's running already the config should be automatically reloaded!
```

As we can see, there is a little change in the output of the registration command.
We can see a `Merging configuration from template file` line.

Now let's see what the configuration file looks like after using the template:

```toml
concurrent = 1
check_interval = 0

[session_server]
  session_timeout = 1800

[[runners]]
  name = "test-runner"
  url = "https://gitlab.com"
  token = "__REDACTED__"
  executor = "kubernetes"
  [runners.cache]
    [runners.cache.s3]
    [runners.cache.gcs]
  [runners.kubernetes]
    host = "http://localhost:9876/"
    bearer_token_overwrite_allowed = false
    image = ""
    namespace = ""
    namespace_overwrite_allowed = ""
    privileged = false
    service_account_overwrite_allowed = ""
    pod_labels_overwrite_allowed = ""
    pod_annotations_overwrite_allowed = ""
    [runners.kubernetes.volumes]

      [[runners.kubernetes.volumes.empty_dir]]
        name = "empty_dir"
        mount_path = "/path/to/empty_dir"
        medium = "Memory"
```

We can see, that the configuration is almost the same as it was previously. The only
change is that it now has the `[[runners.kubernetes.volumes.empty_dir]]` entry with
its options at the end of the file. It's added to the `[[runners]]` entry that was
created by the registration. And because the whole file is saved with the same mechanism,
we also have proper indentation.

If the configuration template includes a settings, and the same setting is passed to the
`register` command, the one passed to the `register` command takes precedence over the one
specified inside of the configuration template.

```shell
$ cat > /tmp/test-config.template.toml << EOF
[[runners]]
  executor = "docker"
EOF

$ sudo gitlab-runner register \
     --config /tmp/test-config.toml \
     --template-config /tmp/test-config.template.toml \
     --non-interactive \
     --url https://gitlab.com \
     --registration-token __REDACTED__ \
     --name test-runner \
     --tag-list shell,test \
     --locked \
     --paused \
     --executor shell

Runtime platform                                    arch=amd64 os=linux pid=12359 revision=88310882 version=11.10.0~beta.1251.g88310882

Registering runner... succeeded                     runner=__REDACTED__
Merging configuration from template file
Runner registered successfully. Feel free to start it, but if it's running already the config should be automatically reloaded!
```

As we can see, the registration command is specifying the `shell` executor, while the template
contains the `docker` one. Let's see what is the final configuration content:

```toml
concurrent = 1
check_interval = 0

[session_server]
  session_timeout = 1800

[[runners]]
  name = "test-runner"
  url = "https://gitlab.com"
  token = "__REDACTED__"
  executor = "shell"
  [runners.cache]
    [runners.cache.s3]
    [runners.cache.gcs]
```

The configuration set with the `register` command options took priority and was
chosen to be placed in the final configuration.
