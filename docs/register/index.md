---
stage: Verify
group: Runner
info: >-
  To determine the technical writer assigned to the Stage/Group associated with
  this page, see
  https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Registering runners

DETAILS:
**Tier:** Free, Premium, Ultimate
**Offering:** GitLab.com, Self-managed

> - [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/3414) in GitLab Runner 15.0, a change to the registration request format prevents the GitLab Runner from communicating with earlier versions of GitLab. You must use a GitLab Runner version that is appropriate for the GitLab version, or upgrade the GitLab application.

Runner registration is the process that links the runner with one or more GitLab instances. You must register the runner so that it can pick up jobs from the GitLab instance.

## Requirements

Before you register a runner:

- Install [GitLab Runner](../install/index.md) on a server separate to where GitLab
  is installed.
- For runner registration with Docker, install [GitLab Runner in a Docker container](../install/docker.md).

## Register with a runner authentication token

> - [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29613) in GitLab 15.10.

Prerequisites:

- Obtain a runner authentication token. You can either:
  - Create an [instance](https://docs.gitlab.com/ee/ci/runners/runners_scope.html#create-an-instance-runner-with-a-runner-authentication-token),
    [group](https://docs.gitlab.com/ee/ci/runners/runners_scope.html#create-a-group-runner-with-a-runner-authentication-token), or
    [project](https://docs.gitlab.com/ee/ci/runners/runners_scope.html#create-a-project-runner-with-a-runner-authentication-token) runner.
  - Locate the runner authentication token in the `config.toml` file. Runner authentication tokens have the prefix, `glrt-`.

After you register the runner, the configuration is saved to the `config.toml`.

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

   To register with a container, you can either:

   - Use a short-lived `gitlab-runner` container with the correct config volume mount:

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

   - Use the executable inside an active runner container:

        ```shell
        docker exec -it gitlab-runner gitlab-runner register
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

Prerequisites:

- Runner registration tokens must be [enabled](https://docs.gitlab.com/ee/administration/settings/continuous_integration.html#enable-runner-registrations-tokens) in the Admin Area.
- Obtain a runner registration token
  for an [instance](https://docs.gitlab.com/ee/ci/runners/runners_scope.html#create-an-instance-runner-with-a-registration-token-deprecated),
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

> - [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4157) in GitLab 16.2.

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

## Register with a configuration template

You can use a configuration template to register a runner with settings that are not supported by the `register` command.

Prerequisites:

- The volume for the location of the template file must be mounted on the GitLab Runner container.
- A runner authentication or registration token:
  - Obtain a runner authentication token (recommended). You can either:
    - Create an [instance](https://docs.gitlab.com/ee/ci/runners/runners_scope.html#create-an-instance-runner-with-a-runner-authentication-token),
    [group](https://docs.gitlab.com/ee/ci/runners/runners_scope.html#create-a-group-runner-with-a-runner-authentication-token), or
    [project](https://docs.gitlab.com/ee/ci/runners/runners_scope.html#create-a-project-runner-with-a-runner-authentication-token) runner.
    - Locate the runner authentication token in the `config.toml` file. Runner authentication tokens have the prefix, `glrt-`.
  - Obtain a runner registration token (deprecated) for an [instance](https://docs.gitlab.com/ee/ci/runners/runners_scope.html#create-an-instance-runner-with-a-registration-token-deprecated),
  [group](https://docs.gitlab.com/ee/ci/runners/runners_scope.html#create-a-project-runner-with-a-registration-token-deprecated), or
  [project](https://docs.gitlab.com/ee/ci/runners/runners_scope.html#create-a-group-runner-with-a-registration-token-deprecated) runner.

The configuration template can be used for automated environments that do not support some arguments
in the `register` command due to:

- Size limits on environment variables based on the environment.
- Command-line options that are not available for executor volumes for Kubernetes.

WARNING:
The configuration template supports only a single [`[[runners]]`](../configuration/advanced-configuration.md#the-runners-section)
section and does not support global options.

To register a runner:

1. Create a configuration template file with the `.toml` format and add your specifications. For example:

   ```toml
   [[runners]]
     [runners.kubernetes]
     [runners.kubernetes.volumes]
       [[runners.kubernetes.volumes.empty_dir]]
         name = "empty_dir"
         mount_path = "/path/to/empty_dir"
         medium = "Memory"
   ```

1. Add the path to the file. You can use either:
   - The [non-interactive mode](../commands/index.md#non-interactive-registration) in the command line:

     ```shell
     $ sudo gitlab-runner register \
     --template-config /tmp/test-config.template.toml \
     --non-interactive \
     --url "https://gitlab.com" \
     --token <TOKEN> \ "# --registration-token if using the deprecated runner registration token"
     --name test-runner \
     --executor kubernetes
     --host = "http://localhost:9876/"
     ```

   - The environment variable in the `.gitlab.yaml` file:

     ```yaml
     variables:
       TEMPLATE_CONFIG_FILE = <file_path>
     ```

    If you update the environment variable, you do not need to
    add the file path in the `register` command each time you register.

After you register the runner, the settings in the configuration template
are merged with the `[[runners]]` entry created in the `config.toml`:

```toml
concurrent = 1
check_interval = 0

[session_server]
  session_timeout = 1800

[[runners]]
  name = "test-runner"
  url = "https://gitlab.com"
  token = glrt-<TOKEN>
  executor = "kubernetes"
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

Template settings are merged only for options that are:

- Empty strings
- Null or non-existent entries
- Zeroes

Command-line arguments or environment variables take precedence over
settings in the configuration template. For example, if the template
specifies a `docker` executor, but the command line specifies `shell`,
the configured executor is `shell`.

## Register a runner for GitLab Community Edition integration tests

To test GitLab Community Edition integrations, use a configuration template to register a runner
with a confined Docker executor.

1. Create a [project runner](https://docs.gitlab.com/ee/ci/runners/runners_scope.html#create-a-project-runner-with-a-runner-authentication-token).
1. Create a template with the `[[runners.docker.services]]` section:

   ```shell
   $ cat > /tmp/test-config.template.toml << EOF
   [[runners]]
   [runners.docker]
   [[runners.docker.services]]
   name = "mysql:latest"
   [[runners.docker.services]]
   name = "redis:latest"

   EOF
   ```

1. Register the runner:

   ::Tabs

   :::TabTitle Linux

   ```shell
   sudo gitlab-runner register \
     --non-interactive \
     --url "https://gitlab.com" \
     --token "$RUNNER_AUTHENTICATION_TOKEN" \
     --template-config /tmp/test-config.template.toml \
     --description "gitlab-ce-ruby-2.7" \
     --executor "docker" \
     --docker-image ruby:2.7
   ```

   :::TabTitle macOS

   ```shell
   gitlab-runner register \
     --non-interactive \
     --url "https://gitlab.com" \
     --token "$RUNNER_AUTHENTICATION_TOKEN" \
     --template-config /tmp/test-config.template.toml \
     --description "gitlab-ce-ruby-2.7" \
     --executor "docker" \
     --docker-image ruby:2.7
   ```

   :::TabTitle Windows

   ```shell
   .\gitlab-runner.exe register \
     --non-interactive \
     --url "https://gitlab.com" \
     --token "$RUNNER_AUTHENTICATION_TOKEN" \
     --template-config /tmp/test-config.template.toml \
     --description "gitlab-ce-ruby-2.7" \
     --executor "docker" \
     --docker-image ruby:2.7
   ```

   :::TabTitle FreeBSD

   ```shell
   sudo -u gitlab-runner -H /usr/local/bin/gitlab-runner register
     --non-interactive \
     --url "https://gitlab.com" \
     --token "$RUNNER_AUTHENTICATION_TOKEN" \
     --template-config /tmp/test-config.template.toml \
     --description "gitlab-ce-ruby-2.7" \
     --executor "docker" \
     --docker-image ruby:2.7
   ```

   :::TabTitle Docker

   ```shell
   docker run --rm -v /srv/gitlab-runner/config:/etc/gitlab-runner gitlab/gitlab-runner register \
     --non-interactive \
     --url "https://gitlab.com" \
     --token "$RUNNER_AUTHENTICATION_TOKEN" \
     --template-config /tmp/test-config.template.toml \
     --description "gitlab-ce-ruby-2.7" \
     --executor "docker" \
     --docker-image ruby:2.7
   ```

   ::EndTabs

For more configuration options, see [Advanced configuration](../configuration/advanced-configuration.md).

## Registering runners with Docker

After you register the runner with a Docker container:

- The configuration is written to your configuration volume. For example, `/srv/gitlab-runner/config`.
- The container uses the configuration volume to load the runner.

NOTE:
If `gitlab-runner restart` runs in a Docker container, GitLab Runner starts a new process instead of restarting the existing process.
To apply configuration changes, restart the Docker container instead.

## Troubleshooting

### `Check registration token` error

The `check registration token` error message displays when the GitLab instance does not recognize
the runner registration token entered during registration. This issue can occur when either:

- The instance, group, or project runner registration token was changed in GitLab.
- An incorrect runner registration token was entered.

When this error occurs, you can ask a GitLab administrator to:

- Verify that the runner registration token is valid.
- Confirm that runner registration in the project or group is [permitted](https://docs.gitlab.com/ee/administration/settings/continuous_integration.html#restrict-runner-registration-by-all-members-in-a-group).
