---
stage: Verify
group: Runner
info: >-
  To determine the technical writer assigned to the Stage/Group associated with
  this page, see
  https://about.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Registering runners **(FREE ALL)**

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/3414) in GitLab Runner 15.0, a change to the registration request format prevents the GitLab Runner from communicating with GitLab 14.7 and earlier. You must use a GitLab Runner version that is appropriate for the GitLab version, or upgrade the GitLab application.

Runner registration is the process that links the runner with one or more GitLab instances. You must register the runner so that it can pick up jobs from the GitLab instance.

## Requirements

Before you register a runner:

- Install [GitLab Runner](../install/index.md) on a server separate to where GitLab
  is installed.
- For runner registration with Docker, install [GitLab Runner in a Docker container](../install/docker.md).

## Register with a runner authentication token

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29613) in GitLab 15.10.

Prerequisite:

- Obtain a runner authentication token. You can either:
  - Create a [shared](https://docs.gitlab.com/ee/ci/runners/runners_scope.html#create-a-shared-runner-with-a-runner-authentication-token),
    [group](https://docs.gitlab.com/ee/ci/runners/runners_scope.html#create-a-group-runner-with-a-runner-authentication-token), or
    [project](https://docs.gitlab.com/ee/ci/runners/runners_scope.html#create-a-project-runner-with-a-runner-authentication-token) runner.
  - Locate the runner authentication token in the `config.toml` file. Runner authentication tokens have the prefix, `glrt-`.

To register the runner with a [runner authentication token](https://docs.gitlab.com/ee/security/token_overview.html#runner-authentication-tokens):

1. Run the register command:

   ::Tabs

   :::TabTitle Linux

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

   :::TabTitle macOS

   ```shell
   gitlab-runner register
   ```

   :::TabTitle Windows

   ```shell
   .\gitlab-runner.exe register
   ```

   :::TabTitle FreeBSD

   ```shell
   sudo -u gitlab-runner -H /usr/local/bin/gitlab-runner register
   ```

   :::TabTitle Docker

   To launch a short-lived `gitlab-runner` container to register the container
   you created during installation:

   - For local system volume mounts:

     ```shell
     docker run --rm -it -v /srv/gitlab-runner/config:/etc/gitlab-runner gitlab/gitlab-runner register
     ```

     NOTE:
     If you used a configuration volume other than `/srv/gitlab-runner/config`
     during installation, update the command with the correct volume.

   - For Docker volume mounts:

     ```shell
     docker run --rm -it -v gitlab-runner-config:/etc/gitlab-runner gitlab/gitlab-runner:latest register
     ```

   ::EndTabs

1. Enter your GitLab URL:
   - For runners on GitLab self-managed, use the URL for your GitLab instance. For example,
   if your project is hosted on `gitlab.example.com/yourname/yourproject`, your GitLab instance URL is `https://gitlab.example.com`.
   - For runners on GitLab.com, the `gitlab-ci coordinator URL` is `https://gitlab.com`.
1. Enter the runner authentication token.
1. Enter a name for the runner.
1. Enter the type of [executor](../executors/index.md).

- To register multiple runners on the same host machine, each with a different configuration,
repeat the `register` command.
- To register the same configuration on multiple host machines, use the same runner authentication token
for each runner registration. For more information, see [Reusing a runner configuration](../fleet_scaling/index.md#reusing-a-runner-configuration).

You can also use the [non-interactive mode](../commands/index.md#non-interactive-registration) to use additional arguments to register the runner:

::Tabs

:::TabTitle Linux

```shell
sudo gitlab-runner register \
  --non-interactive \
  --url "https://gitlab.com/" \
  --token "$RUNNER_TOKEN" \
  --executor "docker" \
  --docker-image alpine:latest \
  --description "docker-runner"
```

:::TabTitle macOS

```shell
gitlab-runner register \
  --non-interactive \
  --url "https://gitlab.com/" \
  --token "$RUNNER_TOKEN" \
  --executor "docker" \
  --docker-image alpine:latest \
  --description "docker-runner"
```

:::TabTitle Windows

```shell
.\gitlab-runner.exe register \
  --non-interactive \
  --url "https://gitlab.com/" \
  --token "$RUNNER_TOKEN" \
  --executor "docker-windows" \
  --docker-image mcr.microsoft.com/windows/servercore:1809_amd64 \
  --description "docker-runner"
```

:::TabTitle FreeBSD

```shell
sudo -u gitlab-runner -H /usr/local/bin/gitlab-runner register
  --non-interactive \
  --url "https://gitlab.com/" \
  --token "$RUNNER_TOKEN" \
  --executor "docker" \
  --docker-image alpine:latest \
  --description "docker-runner"
```

:::TabTitle Docker

```shell
docker run --rm -v /srv/gitlab-runner/config:/etc/gitlab-runner gitlab/gitlab-runner register \
  --non-interactive \
  --url "https://gitlab.com/" \
  --token "$RUNNER_TOKEN" \
  --executor "docker" \
  --docker-image alpine:latest \
  --description "docker-runner"
```

::EndTabs

## Register with a runner registration token (deprecated)

WARNING:
The ability to pass a runner registration token, and support for certain configuration arguments was
[deprecated](https://gitlab.com/gitlab-org/gitlab/-/issues/380872) in GitLab 15.6 and will be removed
in GitLab 18.0. Runner authentication tokens should be used instead. For more information, see
[Migrating to the new runner registration workflow](https://docs.gitlab.com/ee/ci/runners/new_creation_workflow.html).

Prerequisite:

- Obtain a runner registration token
  for a [shared](https://docs.gitlab.com/ee/ci/runners/runners_scope.html#create-a-shared-runner-with-a-registration-token-deprecated),
  [group](https://docs.gitlab.com/ee/ci/runners/runners_scope.html#create-a-project-runner-with-a-registration-token-deprecated), or
  [project](https://docs.gitlab.com/ee/ci/runners/runners_scope.html#create-a-group-runner-with-a-registration-token-deprecated) runner.

After you register the runner, the configuration is saved to the `config.toml`.

To register the runner with a [runner registration token](https://docs.gitlab.com/ee/security/token_overview.html#runner-registration-tokens-deprecated):

1. Run the register command:

   ::Tabs

   :::TabTitle Linux

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

   :::TabTitle macOS

   ```shell
   gitlab-runner register
   ```

   :::TabTitle Windows

   ```shell
   .\gitlab-runner.exe register
   ```

   :::TabTitle FreeBSD

   ```shell
   sudo -u gitlab-runner -H /usr/local/bin/gitlab-runner register
   ```

   :::TabTitle Docker

   To launch a short-lived `gitlab-runner` container to register the container
   you created during installation:

   - For local system volume mounts:

     ```shell
     docker run --rm -it -v /srv/gitlab-runner/config:/etc/gitlab-runner gitlab/gitlab-runner register
     ```

     NOTE:
     If you used a configuration volume other than `/srv/gitlab-runner/config`
     during installation, update the command with the correct volume.

   - For Docker volume mounts:

     ```shell
     docker run --rm -it -v gitlab-runner-config:/etc/gitlab-runner gitlab/gitlab-runner:latest register
     ```

   ::EndTabs

1. Enter your GitLab URL:
   - For GitLab self-managed runners, use the URL for your GitLab instance. For example,
   if your project is hosted on `gitlab.example.com/yourname/yourproject`, your GitLab instance URL is `https://gitlab.example.com`.
   - For GitLab.com, the `gitlab-ci coordinator URL` is `https://gitlab.com`.
1. Enter the token you obtained to register the runner.
1. Enter a description for the runner.
1. Enter the job tags, separated by commas.
1. Enter an optional maintenance note for the runner.
1. Enter the type of [executor](../executors/index.md).

To register multiple runners on the same host machine, each with a different configuration,
repeat the `register` command.

You can also use the [non-interactive mode](../commands/index.md#non-interactive-registration) to use additional arguments to register the runner:

::Tabs

:::TabTitle Linux

```shell
sudo gitlab-runner register \
  --non-interactive \
  --url "https://gitlab.com/" \
  --registration-token "$PROJECT_REGISTRATION_TOKEN" \
  --executor "docker" \
  --docker-image alpine:latest \
  --description "docker-runner" \
  --maintenance-note "Free-form maintainer notes about this runner" \
  --tag-list "docker,aws" \
  --run-untagged="true" \
  --locked="false" \
  --access-level="not_protected"
```

:::TabTitle macOS

```shell
gitlab-runner register \
  --non-interactive \
  --url "https://gitlab.com/" \
  --registration-token "$PROJECT_REGISTRATION_TOKEN" \
  --executor "docker" \
  --docker-image alpine:latest \
  --description "docker-runner" \
  --maintenance-note "Free-form maintainer notes about this runner" \
  --tag-list "docker,aws" \
  --run-untagged="true" \
  --locked="false" \
  --access-level="not_protected"
```

:::TabTitle Windows

```shell
.\gitlab-runner.exe register \
  --non-interactive \
  --url "https://gitlab.com/" \
  --registration-token "$PROJECT_REGISTRATION_TOKEN" \
  --executor "docker-windows" \
  --docker-image mcr.microsoft.com/windows/servercore:1809_amd64 \
  --description "docker-runner" \
  --maintenance-note "Free-form maintainer notes about this runner" \
  --tag-list "docker,aws" \
  --run-untagged="true" \
  --locked="false" \
  --access-level="not_protected"
```

:::TabTitle FreeBSD

```shell
sudo -u gitlab-runner -H /usr/local/bin/gitlab-runner register
  --non-interactive \
  --url "https://gitlab.com/" \
  --registration-token "$PROJECT_REGISTRATION_TOKEN" \
  --executor "docker" \
  --docker-image alpine:latest \
  --description "docker-runner" \
  --maintenance-note "Free-form maintainer notes about this runner" \
  --tag-list "docker,aws" \
  --run-untagged="true" \
  --locked="false" \
  --access-level="not_protected"
```

:::TabTitle Docker

```shell
docker run --rm -v /srv/gitlab-runner/config:/etc/gitlab-runner gitlab/gitlab-runner register \
  --non-interactive \
  --url "https://gitlab.com/" \
  --registration-token "$PROJECT_REGISTRATION_TOKEN" \
  --executor "docker" \
  --docker-image alpine:latest \
  --description "docker-runner" \
  --maintenance-note "Free-form maintainer notes about this runner" \
  --tag-list "docker,aws" \
  --run-untagged="true" \
  --locked="false" \
  --access-level="not_protected"
```

::EndTabs

- `--access-level` creates a [protected runner](https://docs.gitlab.com/ee/ci/runners/configure_runners.html#prevent-runners-from-revealing-sensitive-information).
  - For a protected runner, use the `--access-level="ref_protected"` parameter.
  - For an unprotected runner, use `--access-level="not_protected"` or leave the value undefined.
- `--maintenance-note` adds information related to runner maintenance. The maximum length is 255 characters.

### Legacy-compatible registration process

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4157) in GitLab 16.2.

Runner registration tokens and several runner configuration arguments were [deprecated](https://gitlab.com/gitlab-org/gitlab/-/issues/379743)
in GitLab 15.6 and will be removed in GitLab 18.0. To ensure minimal disruption to your automation workflow, the `legacy-compatible registration process` triggers
if a runner authentication token is specified in the legacy parameter `--registration-token`.

The legacy-compatible registration process ignores the following command-line parameters.
These parameters can only be configured when a runner is created in the UI or with the API.

- `--locked`
- `--access-level`
- `--run-untagged`
- `--maximum-timeout`
- `--paused`
- `--tag-list`
- `--maintenance-note`

## Registering runners with Docker

After you register the runner with a Docker container:

- The configuration is written to your configuration volume. For example, `/srv/gitlab-runner/config`.
- The container uses the configuration volume to load the runner.

NOTE:
If `gitlab-runner restart` runs in a Docker container, GitLab Runner starts a new process instead of restarting the existing process.
To apply configuration changes, restart the Docker container instead.

## `[[runners]]` configuration template file

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
     --url "https://gitlab.com" \
     --registration-token __REDACTED__ \
     --name test-runner \
     --tag-list kubernetes,test \
     --locked \
     --paused \
     --executor kubernetes \
     --kubernetes-host http://localhost:9876/

Runtime platform                                    arch=amd64 os=linux pid=1684 revision=436955cb version=15.11.0

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
     --url "https://gitlab.com" \
     --registration-token __REDACTED__ \
     --name test-runner \
     --tag-list kubernetes,test \
     --locked \
     --paused \
     --executor kubernetes \
     --kubernetes-host http://localhost:9876/

Runtime platform                                    arch=amd64 os=linux pid=8798 revision=436955cb version=15.11.0

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
     --url "https://gitlab.com" \
     --registration-token __REDACTED__ \
     --name test-runner \
     --tag-list shell,test \
     --locked \
     --paused \
     --executor shell

Runtime platform                                    arch=amd64 os=linux pid=12359 revision=436955cb version=15.11.0

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

## Troubleshooting

### `Check registration token` error

The `check registration token` error message displays when the GitLab instance does not recognize
the runner registration token entered during registration. This issue can occur when either:

- The instance, group, or project runner registration token was changed in GitLab.
- An incorrect runner registration token was entered.

When this error occurs, you can ask a GitLab administrator to:

- Verify that the runner registration token is valid.
- Confirm that runner registration in the project or group is [permitted](https://docs.gitlab.com/ee/administration/settings/continuous_integration.html#restrict-runner-registration-by-all-members-in-a-group).
