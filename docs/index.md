---
comments: false
last_updated: 2019-01-17
---

# GitLab Runner Docs

GitLab Runner is the open source project that is used to run your jobs and
send the results back to GitLab. It is used in conjunction with [GitLab CI/CD](https://about.gitlab.com/stages-devops-lifecycle/continuous-integration/),
the open-source continuous integration service included with GitLab that
coordinates the jobs.

## Requirements

GitLab Runner is written in [Go](https://golang.org) and can be run as a single binary, no
language specific requirements are needed.

It is designed to run on the GNU/Linux, macOS, and Windows operating systems.
Other operating systems will probably work as long as you can compile a Go
binary on them.

If you want to [use Docker](executors/docker.md), install the latest version.
GitLab Runner requires a minimum of Docker `v1.13.0`.

## Features

- Allows:
  - Running multiple jobs concurrently.
  - Using multiple tokens with multiple servers (even per-project).
  - Limiting number of concurrent jobs per-token.
- Jobs can be run:
  - Locally.
  - Using Docker containers.
  - Using Docker containers and executing job over SSH.
  - Using Docker containers with autoscaling on different clouds and virtualization hypervisors.
  - Connecting to remote SSH server.
- Is written in Go and distributed as single binary without any other requirements.
- Supports Bash and Windows PowerShell.
- Works on GNU/Linux, macOS, and Windows (pretty much anywhere you can run Docker).
- Allows customization of the job running environment.
- Automatic configuration reload without restart.
- Easy to use setup with support for Docker, Docker-SSH, Parallels, or SSH running environments.
- Enables caching of Docker containers.
- Easy installation as a service for GNU/Linux, macOS, and Windows.
- Embedded Prometheus metrics HTTP server.
- Referee workers to monitor and pass Prometheus metrics and other job-specific data to GitLab.

## Compatibility with GitLab versions

The GitLab Runner version should be in sync with the GitLab version. While older
Runners may still work with newer GitLab versions, and vice versa, in some cases,
features may be not available or work properly if there's a version difference.

Backward compatibility is guaranteed between minor version updates,
but be aware that minor version updates of GitLab can introduce new features
which will require the Runner to be on the same minor version.

## Install GitLab Runner

GitLab Runner can be [installed](install/index.md) and used on GNU/Linux, macOS, FreeBSD, and Windows.
You can install it using Docker, download the binary manually or use the
repository for rpm/deb packages that GitLab offers. Below you can find
information on the different installation methods:

- [Install using GitLab's repository for Debian/Ubuntu/CentOS/RedHat (preferred)](install/linux-repository.md).
- [Install on GNU/Linux manually (advanced)](install/linux-manually.md).
- [Install on macOS](install/osx.md).
- [Install on Windows](install/windows.md).
- [Install as a Docker service](install/docker.md).
- [Install in autoscaling mode using Docker machine](executors/docker_machine.md).
- [Install on FreeBSD](install/freebsd.md).
- [Install on Kubernetes](install/kubernetes.md).
- [Install the nightly binary manually (development)](install/bleeding-edge.md).

## Register GitLab Runner

Once GitLab Runner is installed, you need to register it with GitLab.

Learn how to [register a GitLab Runner](register/index.md).

## Using GitLab Runner

- See the [commands documentation](commands/README.md).
- See [best practice documentation](best_practice/index.md).

## Selecting the executor

GitLab Runner implements a number of [executors](executors/README.md) that can be used to run your
builds in different scenarios. If you are not sure what to select, read the
[I am not sure](executors/README.md#i-am-not-sure) section.
Visit the [compatibility chart](executors/README.md#compatibility-chart) to find
out what features each executor supports and what not.

To jump into the specific documentation of each executor, see:

- [Shell](executors/shell.md).
- [Docker](executors/docker.md).
- [Docker Machine and Docker Machine SSH (autoscaling)](executors/docker_machine.md).
- [Parallels](executors/parallels.md).
- [VirtualBox](executors/virtualbox.md).
- [SSH](executors/ssh.md).
- [Kubernetes](executors/kubernetes.md).

No development of new executors is planned and we are not accepting
contributions for new ones. Please check
[CONTRIBUTION.md](https://gitlab.com/gitlab-org/gitlab-runner/blob/master/CONTRIBUTING.md#contributing-a-new-executors)
for details.

## Configuring GitLab Runner

See information on [configuring GitLab Runner](configuration/index.md), and:

- [Advanced configuration options](configuration/advanced-configuration.md): Learn how to use the [TOML](https://github.com/toml-lang/toml) configuration file that GitLab Runner uses.
- [Use self-signed certificates](configuration/tls-self-signed.md): Configure certificates that are used to verify TLS peer when connecting to the GitLab server.
- [Autoscaling using Docker machine](configuration/autoscale.md): Execute jobs on machines that are created on demand using Docker machine.
- [Autoscaling GitLab Runner on AWS](configuration/runner_autoscale_aws/index.md)
- [The init system of GitLab Runner](configuration/init.md): Learn how the Runner installs its init service files based on your operating system.
- [Supported shells](shells/index.md): Learn what shell script generators are supported that allow to execute builds on different systems.
- [Security considerations](security/index.md): Be aware of potential security implications when running your jobs with GitLab Runner.
- [Runner monitoring](monitoring/README.md): Learn how to monitor the Runner's behavior.
- [Cleanup the Docker images automatically](https://gitlab.com/gitlab-org/gitlab-runner-docker-cleanup): A simple Docker application that automatically garbage collects the GitLab Runner caches and images when running low on disk space.
- [Configure GitLab Runner to run behind a proxy](configuration/proxy.md): Learn how to set up a Linux proxy and configure GitLab Runner. Especially useful for the Docker executor.
- [Feature Flags](configuration/feature-flags.md): Learn how to use feature flags to get access to features in beta stage or to enable breaking changes before the full deprecation and replacement is handled.
- [Configure Session Server](configuration/advanced-configuration.md#the-session_server-section): Learn how to configure a session server for interacting with jobs the Runner is responsible for.

## Troubleshooting

Read the [FAQ](faq/README.md) for troubleshooting common issues.

## Release process

The description of release process of the GitLab Runner project can be
found in
[PROCESS.md](https://gitlab.com/gitlab-org/gitlab-runner/blob/master/PROCESS.md)

## Contributing

Contributions are welcome, see [`CONTRIBUTING.md`](https://gitlab.com/gitlab-org/gitlab-runner/blob/master/CONTRIBUTING.md) for more details.

## Development

See the [development documentation](development/README.md) to hack on GitLab
Runner.

If you're a reviewer of GitLab Runner project, then please take a moment to read the
[Reviewing GitLab Runner](development/reviewing-gitlab-runner.md) document.

## Changelog

See the [CHANGELOG](https://gitlab.com/gitlab-org/gitlab-runner/blob/master/CHANGELOG.md) to view recent changes.

## License

This code is distributed under the MIT license, see the [LICENSE](https://gitlab.com/gitlab-org/gitlab-runner/blob/master/LICENSE) file.
