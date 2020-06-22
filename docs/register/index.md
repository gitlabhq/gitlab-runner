# Registering Runners

Registering a Runner is the process that binds the Runner with a GitLab instance.

## Requirements

Before registering a Runner, you need to first:

- [Install it](../install/index.md) on a server separate than where GitLab
  is installed on
- [Obtain a token](https://docs.gitlab.com/ee/ci/runners/) for a shared or
  specific Runner via GitLab's interface

## GNU/Linux

To register a Runner under GNU/Linux:

1. Run the following command:

   ```shell
   sudo gitlab-runner register
   ```

1. Enter your GitLab instance URL:

   ```plaintext
   Please enter the gitlab-ci coordinator URL (e.g. https://gitlab.com )
   https://gitlab.com
   ```

1. Enter the token you obtained to register the Runner:

   ```plaintext
   Please enter the gitlab-ci token for this runner
   xxx
   ```

1. Enter a description for the Runner, you can change this later in GitLab's
   UI:

   ```plaintext
   Please enter the gitlab-ci description for this runner
   [hostname] my-runner
   ```

1. Enter the [tags associated with the Runner](https://docs.gitlab.com/ee/ci/runners/#using-tags), you can change this later in GitLab's UI:

   ```plaintext
   Please enter the gitlab-ci tags for this runner (comma separated):
   my-tag,another-tag
   ```

1. Enter the [Runner executor](../executors/README.md):

   ```plaintext
   Please enter the executor: ssh, docker+machine, docker-ssh+machine, kubernetes, docker, parallels, virtualbox, docker-ssh, shell:
   docker
   ```

1. If you chose Docker as your executor, you'll be asked for the default
   image to be used for projects that do not define one in `.gitlab-ci.yml`:

   ```plaintext
   Please enter the Docker image (eg. ruby:2.6):
   alpine:latest
   ```

## macOS

To register a Runner under macOS:

1. Run the following command:

   ```shell
   gitlab-runner register
   ```

1. Enter your GitLab instance URL:

   ```plaintext
   Please enter the gitlab-ci coordinator URL (e.g. https://gitlab.com )
   https://gitlab.com
   ```

1. Enter the token you obtained to register the Runner:

   ```plaintext
   Please enter the gitlab-ci token for this runner
   xxx
   ```

1. Enter a description for the Runner, you can change this later in GitLab's
   UI:

   ```plaintext
   Please enter the gitlab-ci description for this runner
   [hostname] my-runner
   ```

1. Enter the [tags associated with the Runner](https://docs.gitlab.com/ee/ci/runners/#using-tags), you can change this later in GitLab's UI:

   ```plaintext
   Please enter the gitlab-ci tags for this runner (comma separated):
   my-tag,another-tag
   ```

1. Enter the [Runner executor](../executors/README.md):

   ```plaintext
   Please enter the executor: ssh, docker+machine, docker-ssh+machine, kubernetes, docker, parallels, virtualbox, docker-ssh, shell:
   docker
   ```

1. If you chose Docker as your executor, you'll be asked for the default
   image to be used for projects that do not define one in `.gitlab-ci.yml`:

   ```plaintext
   Please enter the Docker image (eg. ruby:2.6):
   alpine:latest
   ```

    **Note** _[be sure Docker.app is installed on your mac](https://docs.docker.com/docker-for-mac/install/)_

## Windows

To register a Runner under Windows:

1. Run the following command:

   ```shell
   ./gitlab-runner.exe register
   ```

1. Enter your GitLab instance URL:

   ```plaintext
   Please enter the gitlab-ci coordinator URL (e.g. https://gitlab.com )
   https://gitlab.com
   ```

1. Enter the token you obtained to register the Runner:

   ```plaintext
   Please enter the gitlab-ci token for this runner
   xxx
   ```

1. Enter a description for the Runner, you can change this later in GitLab's
   UI:

   ```plaintext
   Please enter the gitlab-ci description for this runner
   [hostname] my-runner
   ```

1. Enter the [tags associated with the Runner](https://docs.gitlab.com/ee/ci/runners/#using-tags), you can change this later in GitLab's UI:

   ```plaintext
   Please enter the gitlab-ci tags for this runner (comma separated):
   my-tag,another-tag
   ```

1. Enter the [Runner executor](../executors/README.md):

   ```plaintext
   Please enter the executor: ssh, docker+machine, docker-ssh+machine, kubernetes, docker, parallels, virtualbox, docker-ssh, shell:
   docker
   ```

1. If you chose Docker as your executor, you'll be asked for the default
   image to be used for projects that do not define one in `.gitlab-ci.yml`:

   ```plaintext
   Please enter the Docker image (eg. ruby:2.6):
   alpine:latest
   ```

If you'd like to register multiple Runners on the same machine with different
configurations repeat the `./gitlab-runner.exe register` command.

## FreeBSD

To register a Runner under FreeBSD:

1. Run the following command:

   ```shell
   sudo -u gitlab-runner -H /usr/local/bin/gitlab-runner register
   ```

1. Enter your GitLab instance URL:

   ```plaintext
   Please enter the gitlab-ci coordinator URL (e.g. https://gitlab.com )
   https://gitlab.com
   ```

1. Enter the token you obtained to register the Runner:

   ```plaintext
   Please enter the gitlab-ci token for this runner
   xxx
   ```

1. Enter a description for the Runner, you can change this later in GitLab's
   UI:

   ```plaintext
   Please enter the gitlab-ci description for this runner
   [hostname] my-runner
   ```

1. Enter the [tags associated with the Runner](https://docs.gitlab.com/ee/ci/runners/#using-tags), you can change this later in GitLab's UI:

   ```plaintext
   Please enter the gitlab-ci tags for this runner (comma separated):
   my-tag,another-tag
   ```

1. Enter the [Runner executor](../executors/README.md):

   ```plaintext
   Please enter the executor: ssh, docker+machine, docker-ssh+machine, kubernetes, docker, parallels, virtualbox, docker-ssh, shell:
   docker
   ```

1. If you chose Docker as your executor, you'll be asked for the default
   image to be used for projects that do not define one in `.gitlab-ci.yml`:

   ```plaintext
   Please enter the Docker image (eg. ruby:2.6):
   alpine:latest
   ```

## Docker

These instructions are meant to be followed after [Run GitLab Runner in a container](../install/docker.md).
In this section, you will launch an ephemeral `gitlab-runner` container to
register the container that you created during install. After you finish
registration, the resulting configuration will be written to your chosen configuration
volume (e.g. `/srv/gitlab-runner/config`), and will be automatically loaded by
the Runner using that configuration volume.

To register a Runner using a Docker container:

1. Run the register command:

   For local system volume mounts:

   ```shell
   docker run --rm -it -v /srv/gitlab-runner/config:/etc/gitlab-runner gitlab/gitlab-runner register
   ```

   NOTE: **Note:**
   If you used a configuration volume other than `/srv/gitlab-runner/config` during
   install, then you should update the command with the correct volume.

   For Docker volume mounts:

   ```shell
   docker run --rm -it -v gitlab-runner-config:/etc/gitlab-runner gitlab/gitlab-runner:latest register
   ```

1. Enter your GitLab instance URL:

   ```plaintext
   Please enter the gitlab-ci coordinator URL (e.g. https://gitlab.com )
   https://gitlab.com
   ```

1. Enter the token you obtained to register the Runner:

   ```plaintext
   Please enter the gitlab-ci token for this runner
   xxx
   ```

1. Enter a description for the Runner, you can change this later in GitLab's
   UI:

   ```plaintext
   Please enter the gitlab-ci description for this runner
   [hostname] my-runner
   ```

1. Enter the [tags associated with the Runner](https://docs.gitlab.com/ee/ci/runners/#using-tags), you can change this later in GitLab's UI:

   ```plaintext
   Please enter the gitlab-ci tags for this runner (comma separated):
   my-tag,another-tag
   ```

1. Enter the [Runner executor](../executors/README.md):

   ```plaintext
   Please enter the executor: ssh, docker+machine, docker-ssh+machine, kubernetes, docker, parallels, virtualbox, docker-ssh, shell:
   docker
   ```

1. If you chose Docker as your executor, you'll be asked for the default
   image to be used for projects that do not define one in `.gitlab-ci.yml`:

   ```plaintext
   Please enter the Docker image (eg. ruby:2.6):
   alpine:latest
   ```

## One-line registration command

If you want to use the non-interactive mode to register a Runner, you can
either use the `register` subcommands or use their equivalent environment
variables.

To see a list of all the `register` subcommands, use:

```shell
gitlab-runner register -h
```

To register a Runner using the most common options, you would do:

```shell
sudo gitlab-runner register \
  --non-interactive \
  --url "https://gitlab.com/" \
  --registration-token "PROJECT_REGISTRATION_TOKEN" \
  --executor "docker" \
  --docker-image alpine:latest \
  --description "docker-runner" \
  --tag-list "docker,aws" \
  --run-untagged="true" \
  --locked="false" \
  --access-level="not_protected"
```

If you're running the Runner in a Docker container, the `register` command would
look like:

```shell
docker run --rm -v /srv/gitlab-runner/config:/etc/gitlab-runner gitlab/gitlab-runner register \
  --non-interactive \
  --executor "docker" \
  --docker-image alpine:latest \
  --url "https://gitlab.com/" \
  --registration-token "PROJECT_REGISTRATION_TOKEN" \
  --description "docker-runner" \
  --tag-list "docker,aws" \
  --run-untagged="true" \
  --locked="false" \
  --access-level="not_protected"
```

The `--access-level` parameter was added in GitLab Runner 12.0. It uses a registration API parameter introduced in GitLab 11.11.
Use this parameter during registration to create a [protected Runner](https://docs.gitlab.com/ee/ci/runners/#prevent-runners-from-revealing-sensitive-information).
For a protected Runner, use the `--access-level="ref_protected"` parameter.
For an unprotected Runner, use `--access-level="not_protected"` instead or leave the value undefined.
This value can later be toggled on or off in the project's **Settings > CI/CD** menu.

## `[[runners]]` configuration template file

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4228) in GitLab Runner 12.2.

Some Runner configuration settings can't be set with environment variables or command line options.

For example:

- Environment variables do not support slices.
- Command line option support is internationally unavailable for the settings for the
  whole Kubernetes executor volumes tree.

This is a problem for environments that are handled by any kind of automation, such as the
[GitLab Runner official Helm chart](../install/kubernetes.md). In cases like these, the only solution was
to manually update the `config.toml` file after the Runner was registered. This is less
than ideal, error-prone, and not reliable. Especially when more than one registration
for the same Runner installation is done.

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

We register a Kubernetes-executor-based Runner to some test project and see what the
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

The command above will create the following `config.toml` file:

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
    pod_annotations_overwrite_allowed = ""
    [runners.kubernetes.volumes]
```

We can see the basic configuration created from the provided command line options:

- Runner credentials (URL and token).
- The executor specified.
- The default, empty section `runners.kubernetes` with only the one option
  provided during the registration filled out.

Normally one would need to set few more options to make the Kubernetes executor
usable, but the above is enough for the purpose of our example.

Let's now assume that we need to configure an `emptyDir` volume for our Kubernetes executor. There is
no way to add this while registering with neither environment variables nor command line options.
We would need to **manually append** something like this to the end of the file:

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
within one `config.toml` file. The assumption that the new one will be always at the
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

Having the file, we can now try to register the Runner again, but this time adding the
`--template-config /tmp/test-config.template.toml` option. Apart from this change, the
rest of registration command will be exactly the same:

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
