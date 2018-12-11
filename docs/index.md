---
toc: false
comments: false
last_updated: 2018-11-07
---

# GitLab Runner

[![Build Status](https://gitlab.com/gitlab-org/gitlab-runner/badges/master/build.svg)](https://gitlab.com/gitlab-org/gitlab-runner)

GitLab Runner is the open source project that is used to run your jobs and
send the results back to GitLab. It is used in conjunction with [GitLab CI][ci],
the open-source continuous integration service included with GitLab that
coordinates the jobs.

## Requirements

GitLab Runner is written in [Go][golang] and can be run as a single binary, no
language specific requirements are needed.

It is designed to run on the GNU/Linux, macOS, and Windows operating systems.
Other operating systems will probably work as long as you can compile a Go
binary on them.

If you want to use Docker make sure that you have version `v1.5.0` at least
installed.

## Features

- Allows to run:
  - multiple jobs concurrently
  - use multiple tokens with multiple server (even per-project)
  - limit number of concurrent jobs per-token
- Jobs can be run:
  - locally
  - using Docker containers
  - using Docker containers and executing job over SSH
  - using Docker containers with autoscaling on different clouds and virtualization hypervisors
  - connecting to remote SSH server
- Is written in Go and distributed as single binary without any other requirements
- Supports Bash, Windows Batch and Windows PowerShell
- Works on GNU/Linux, OS X and Windows (pretty much anywhere you can run Docker)
- Allows to customize the job running environment
- Automatic configuration reload without restart
- Easy to use setup with support for Docker, Docker-SSH, Parallels or SSH running environments
- Enables caching of Docker containers
- Easy installation as a service for GNU/Linux, OSX and Windows
- Embedded Prometheus metrics HTTP server

## Compatibility chart

The GitLab Runner version should be in sync with the GitLab version. While older
Runners may still work with newer GitLab versions, and vice versa, in some cases,
features may be not available or work properly if there's a version difference.

Backward incompatibility is allowed only for major version updates.

## [Install GitLab Runner](install/index.md)

GitLab Runner can be installed and used on GNU/Linux, macOS, FreeBSD and Windows.
You can install it using Docker, download the binary manually or use the
repository for rpm/deb packages that GitLab offers. Below you can find
information on the different installation methods:

- [Install using GitLab's repository for Debian/Ubuntu/CentOS/RedHat (preferred)](install/linux-repository.md)
- [Install on GNU/Linux manually (advanced)](install/linux-manually.md)
- [Install on macOS](install/osx.md)
- [Install on Windows](install/windows.md)
- [Install as a Docker service](install/docker.md)
- [Install in autoscaling mode using Docker machine](executors/docker_machine.md)
- [Install on FreeBSD](install/freebsd.md)
- [Install on Kubernetes](install/kubernetes.md)
- [Install the nightly binary manually (development)](install/bleeding-edge.md)

## [Register GitLab Runner](register/index.md)

Once GitLab Runner is installed, you need to register it with GitLab.

Learn how to [register a GitLab Runner](register/index.md).

## Using GitLab Runner

- [See the commands documentation](commands/README.md)

## [Selecting the executor](executors/README.md)

GitLab Runner implements a number of executors that can be used to run your
builds in different scenarios. If you are not sure what to select, read the
[I am not sure](executors/README.md#i-am-not-sure) section.
Visit the [compatibility chart](executors/README.md#compatibility-chart) to find
out what features each executor supports and what not.

To jump into the specific documentation of each executor, visit:

- [Shell](executors/shell.md)
- [Docker](executors/docker.md)
- [Docker Machine and Docker Machine SSH (autoscaling)](executors/docker_machine.md)
- [Parallels](executors/parallels.md)
- [VirtualBox](executors/virtualbox.md)
- [SSH](executors/ssh.md)
- [Kubernetes](executors/kubernetes.md)

## [Advanced Configuration](configuration/index.md)

- [Advanced configuration options](configuration/advanced-configuration.md) Learn how to use the [TOML][] configuration file that GitLab Runner uses.
- [Use self-signed certificates](configuration/tls-self-signed.md) Configure certificates that are used to verify TLS peer when connecting to the GitLab server.
- [Autoscaling using Docker machine](configuration/autoscale.md) Execute jobs on machines that are created on demand using Docker machine.
- [Autoscaling GitLab Runner on AWS](configuration/runner_autoscale_aws/index.md)
- [The init system of GitLab Runner](configuration/init.md) Learn how the Runner installs its init service files based on your operating system.
- [Supported shells](shells/README.md) Learn what shell script generators are supported that allow to execute builds on different systems.
- [Security considerations](security/index.md) Be aware of potential security implications when running your jobs with GitLab Runner.
- [Runner monitoring](monitoring/README.md) Learn how to monitor the Runner's behavior.
- [Cleanup the Docker images automatically](https://gitlab.com/gitlab-org/gitlab-runner-docker-cleanup) A simple Docker application that automatically garbage collects the GitLab Runner caches and images when running low on disk space.
- [Configure GitLab Runner to run behind a proxy](configuration/proxy.md) Learn how to set up a Linux proxy and configure GitLab Runner. Especially useful for the Docker executor.
- [Feature Flags](configuration/feature-flags.md) Learn how to use feature flags to get access to features in beta stage or to enable breaking changes before the full deprecation and replacement is handled.

## Troubleshooting

Read the [FAQ](faq/README.md) for troubleshooting common issues.

## Release process

The description of release process of the GitLab Runner project can be found in
the [release documentation](release_process/README.md).

## Contributing

Contributions are welcome, see [`CONTRIBUTING.md`][contribute] for more details.

## Development

See the [development documentation](development/README.md) to hack on GitLab
Runner.

## Changelog

Visit [Changelog] to view recent changes.

## License

This code is distributed under the MIT license, see the [LICENSE][] file.

[ci]: https://about.gitlab.com/gitlab-ci
[Changelog]: https://gitlab.com/gitlab-org/gitlab-runner/blob/master/CHANGELOG.md
[contribute]: https://gitlab.com/gitlab-org/gitlab-runner/blob/master/CONTRIBUTING.md
[golang]: https://golang.org/
[LICENSE]: https://gitlab.com/gitlab-org/gitlab-runner/blob/master/LICENSE
[TOML]: https://github.com/toml-lang/toml
