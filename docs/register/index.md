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

    ```sh
    sudo gitlab-runner register
    ```

1. Enter your GitLab instance URL:

    ```text
    Please enter the gitlab-ci coordinator URL (e.g. https://gitlab.com )
    https://gitlab.com
    ```

1. Enter the token you obtained to register the Runner:

    ```text
    Please enter the gitlab-ci token for this runner
    xxx
    ```

1. Enter a description for the Runner, you can change this later in GitLab's
   UI:

    ```text
    Please enter the gitlab-ci description for this runner
    [hostame] my-runner
    ```

1. Enter the [tags associated with the Runner][tags], you can change this later in GitLab's UI:

    ```text
    Please enter the gitlab-ci tags for this runner (comma separated):
    my-tag,another-tag
    ```

1. Enter the [Runner executor](../executors/README.md):

    ```text
    Please enter the executor: ssh, docker+machine, docker-ssh+machine, kubernetes, docker, parallels, virtualbox, docker-ssh, shell:
    docker
    ```

1. If you chose Docker as your executor, you'll be asked for the default
   image to be used for projects that do not define one in `.gitlab-ci.yml`:

    ```text
    Please enter the Docker image (eg. ruby:2.1):
    alpine:latest
    ```

## macOS

To register a Runner under macOS:

1. Run the following command:

    ```sh
    gitlab-runner register
    ```

1. Enter your GitLab instance URL:

    ```text
    Please enter the gitlab-ci coordinator URL (e.g. https://gitlab.com )
    https://gitlab.com
    ```

1. Enter the token you obtained to register the Runner:

    ```text
    Please enter the gitlab-ci token for this runner
    xxx
    ```

1. Enter a description for the Runner, you can change this later in GitLab's
   UI:

    ```text
    Please enter the gitlab-ci description for this runner
    [hostame] my-runner
    ```

1. Enter the [tags associated with the Runner][tags], you can change this later in GitLab's UI:

    ```text
    Please enter the gitlab-ci tags for this runner (comma separated):
    my-tag,another-tag
    ```

1. Enter the [Runner executor](../executors/README.md):

    ```text
    Please enter the executor: ssh, docker+machine, docker-ssh+machine, kubernetes, docker, parallels, virtualbox, docker-ssh, shell:
    docker
    ```

1. If you chose Docker as your executor, you'll be asked for the default
   image to be used for projects that do not define one in `.gitlab-ci.yml`:

    ```text
    Please enter the Docker image (eg. ruby:2.1):
    alpine:latest
    ```
    **Note** _[be sure Docker.app is installed on your mac](https://docs.docker.com/docker-for-mac/install/)_

## Windows

To register a Runner under Windows:

1. Run the following command:

    ```sh
    ./gitlab-runner.exe register
    ```

1. Enter your GitLab instance URL:

    ```text
    Please enter the gitlab-ci coordinator URL (e.g. https://gitlab.com )
    https://gitlab.com
    ```

1. Enter the token you obtained to register the Runner:

    ```text
    Please enter the gitlab-ci token for this runner
    xxx
    ```

1. Enter a description for the Runner, you can change this later in GitLab's
   UI:

    ```text
    Please enter the gitlab-ci description for this runner
    [hostame] my-runner
    ```

1. Enter the [tags associated with the Runner][tags], you can change this later in GitLab's UI:

    ```text
    Please enter the gitlab-ci tags for this runner (comma separated):
    my-tag,another-tag
    ```

1. Enter the [Runner executor](../executors/README.md):

    ```text
    Please enter the executor: ssh, docker+machine, docker-ssh+machine, kubernetes, docker, parallels, virtualbox, docker-ssh, shell:
    docker
    ```

1. If you chose Docker as your executor, you'll be asked for the default
   image to be used for projects that do not define one in `.gitlab-ci.yml`:

    ```text
    Please enter the Docker image (eg. ruby:2.1):
    alpine:latest
    ```

If you'd like to register multiple Runners on the same machine with different
configurations repeat the `./gitlab-runner.exe register` command.

## FreeBSD

To register a Runner under FreeBSD:

1. Run the following command:

    ```sh
    sudo -u gitlab-runner -H /usr/local/bin/gitlab-runner register
    ```

1. Enter your GitLab instance URL:

    ```text
    Please enter the gitlab-ci coordinator URL (e.g. https://gitlab.com )
    https://gitlab.com
    ```

1. Enter the token you obtained to register the Runner:

    ```text
    Please enter the gitlab-ci token for this runner
    xxx
    ```

1. Enter a description for the Runner, you can change this later in GitLab's
   UI:

    ```text
    Please enter the gitlab-ci description for this runner
    [hostame] my-runner
    ```

1. Enter the [tags associated with the Runner][tags], you can change this later in GitLab's UI:

    ```text
    Please enter the gitlab-ci tags for this runner (comma separated):
    my-tag,another-tag
    ```

1. Enter the [Runner executor](../executors/README.md):

    ```text
    Please enter the executor: ssh, docker+machine, docker-ssh+machine, kubernetes, docker, parallels, virtualbox, docker-ssh, shell:
    docker
    ```

1. If you chose Docker as your executor, you'll be asked for the default
   image to be used for projects that do not define one in `.gitlab-ci.yml`:

    ```text
    Please enter the Docker image (eg. ruby:2.1):
    alpine:latest
    ```

## Docker

To register a Runner using a Docker container:

1. Run the following command which will mount the Runner's config directory
   under `/srv/gitlab-runner/config`:

    ```sh
    docker run --rm -t -i -v /srv/gitlab-runner/config:/etc/gitlab-runner gitlab/gitlab-runner register
    ```

    NOTE: **Note:**
    This will only register the runner, and then exit and destroy the container.

1. Enter your GitLab instance URL:

    ```text
    Please enter the gitlab-ci coordinator URL (e.g. https://gitlab.com )
    https://gitlab.com
    ```

1. Enter the token you obtained to register the Runner:

    ```text
    Please enter the gitlab-ci token for this runner
    xxx
    ```

1. Enter a description for the Runner, you can change this later in GitLab's
   UI:

    ```text
    Please enter the gitlab-ci description for this runner
    [hostame] my-runner
    ```

1. Enter the [tags associated with the Runner][tags], you can change this later in GitLab's UI:

    ```text
    Please enter the gitlab-ci tags for this runner (comma separated):
    my-tag,another-tag
    ```

1. Enter the [Runner executor](../executors/README.md):

    ```text
    Please enter the executor: ssh, docker+machine, docker-ssh+machine, kubernetes, docker, parallels, virtualbox, docker-ssh, shell:
    docker
    ```

1. If you chose Docker as your executor, you'll be asked for the default
   image to be used for projects that do not define one in `.gitlab-ci.yml`:

    ```text
    Please enter the Docker image (eg. ruby:2.1):
    alpine:latest
    ```

## One-line registration command

If you want to use the non-interactive mode to register a Runner, you can
either use the `register` subcommands or use their equivalent environment
variables.

To see a list of all the `register` subcommands, use:

```sh
gitlab-runner register -h
```

To register a Runner using the most common options, you would do:

```sh
sudo gitlab-runner register \
  --non-interactive \
  --url "https://gitlab.com/" \
  --registration-token "PROJECT_REGISTRATION_TOKEN" \
  --executor "docker" \
  --docker-image alpine:3 \
  --description "docker-runner" \
  --tag-list "docker,aws" \
  --run-untagged \
  --locked="false" \
```

If you're running the Runner in a Docker container, the `register` command would
look like:

```sh
docker run --rm -t -i -v /path/to/config:/etc/gitlab-runner gitlab/gitlab-runner register \
  --non-interactive \
  --executor "docker" \
  --docker-image alpine:3 \
  --url "https://gitlab.com/" \
  --registration-token "PROJECT_REGISTRATION_TOKEN" \
  --description "docker-runner" \
  --tag-list "docker,aws" \
  --run-untagged \
  --locked="false"
```

[tags]: https://docs.gitlab.com/ee/ci/runners/#using-tags
