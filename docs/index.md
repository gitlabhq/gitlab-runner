---
toc: false
last_updated: 2017-10-09
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

CAUTION: **Important:**
GitLab Runner >= 9.0 requires GitLab's API v4 endpoints, which were introduced
in GitLab CE/EE 9.0. Because of this change, GitLab Runner >= 9.0 requires
GitLab CE/EE >= 9.0 and will not work with older GitLab versions.
The old API used by GitLab Runner was deprecated in August 2017 and with this,
the v1.11.x version of GitLab Runner is deprecated as well.

In the following table you can see the compatibility chart between GitLab and
GitLab Runner.

|GitLab Runner / GitLab | 9.0.x (03.2017) | 9.1.x (04.2017) | 9.2.x (05.2017) | 9.3.x (06.2017) | 9.4.x (07.2017) | 9.5.x (08.2017) | 10.0.x (09.2017) |
|:---------------------:|:---------------:|:---------------:|:---------------:|:---------------:|:---------------:|:---------------:|:----------------:|
| v1.10.x  |  ✓               | ✓               | ✓               | ✓               | ✓               | ✓               | ✗                |
| v1.11.x  |  ✓               | ✓               | ✓               | ✓               | ✓               | ✓               | ✗                |
| v9.0.x   |  ✓               | ✓               | ✓               | ✓               | ✓               | ✓               | ✓                |
| v9.1.x   |  ✓               | ✓               | ✓               | ✓               | ✓               | ✓               | ✓                |
| v9.2.x   |  ✓               | ✓               | ✓               | ✓               | ✓               | ✓               | ✓                |
| v9.3.x   |  ✓               | ✓               | ✓               | ✓               | ✓               | ✓               | ✓                |
| v9.4.x   |  ✓               | ✓               | ✓               | ✓               | ✓               | ✓               | ✓                |
| v9.5.x   |  ✓               | ✓               | ✓               | ✓               | ✓               | ✓               | ✓                |
| v10.0.x  |  ✓               | ✓               | ✓               | ✓               | ✓               | ✓               | ✓                |

## [Install GitLab Runner](install/index.md)

GitLab Runner can be installed and used on GNU/Linux, macOS, FreeBSD and Windows.
You can install it using Docker, download the binary manually or use the
repository for rpm/deb packages that GitLab offers. Below you can find
information on the different installation methods:

- [Install using GitLab's repository for Debian/Ubuntu/CentOS/RedHat (preferred)](install/linux-repository.md)
- [Install on GNU/Linux manually (advanced)](install/linux-manually.md)
- [Install on macOS (preferred)](install/osx.md)
- [Install on Windows (preferred)](install/windows.md)
- [Install as a Docker Service](install/docker.md)
- [Install in Auto-scaling mode using Docker machine](install/autoscaling.md)
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
- [Docker Machine and Docker Machine SSH (auto-scaling)](install/autoscaling.md)
- [Parallels](executors/parallels.md)
- [VirtualBox](executors/virtualbox.md)
- [SSH](executors/ssh.md)
- [Kubernetes](executors/kubernetes.md)

## [Advanced Configuration](configuration/index.md)

- [Advanced configuration options](configuration/advanced-configuration.md) Learn how to use the [TOML][] configuration file that GitLab Runner uses.
- [Use self-signed certificates](configuration/tls-self-signed.md) Configure certificates that are used to verify TLS peer when connecting to the GitLab server.
- [Auto-scaling using Docker machine](configuration/autoscale.md) Execute jobs on machines that are created on demand using Docker machine.
- [Supported shells](shells/README.md) Learn what shell script generators are supported that allow to execute builds on different systems.
- [Security considerations](security/index.md) Be aware of potential security implications when running your jobs with GitLab Runner.
- [Runner monitoring](monitoring/README.md) Learn how to monitor Runner's behavior.
- [Cleanup the Docker images automatically](https://gitlab.com/gitlab-org/gitlab-runner-docker-cleanup) A simple Docker application that automatically garbage collects the GitLab Runner caches and images when running low on disk space.

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
