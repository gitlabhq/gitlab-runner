---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/engineering/ux/technical-writing/#assignments
comments: false
---

# Install GitLab Runner

GitLab Runner can be installed and used on GNU/Linux, macOS, FreeBSD, and Windows.
You can install it:

- In a container.
- By downloading a binary manually.
- By using a repository for rpm/deb packages.

GitLab Runner officially supported binaries are available for the following architectures:

- x86, AMD64, ARM64, ARM, s390x

Official packages are available for the following Linux distributions:

- CentOS, Debian, Ubuntu, RHEL, Fedora, Mint

GitLab Runner officially supports the following operating systems:

- Linux, Windows, macOS, FreeBSD

You can find information on the different installation methods below.
You can also view installation instructions in GitLab by going to your project's
**Settings > CI / CD**, expanding the **Runners** section, and clicking
**Show runner installation instructions**.

## Repositories

- [Install using the GitLab repository for Debian/Ubuntu/CentOS/RedHat](linux-repository.md)

## Binaries

- [Install on GNU/Linux](linux-manually.md)
- [Install on macOS](osx.md)
- [Install on Windows](windows.md)
- [Install on FreeBSD](freebsd.md)
- [Install nightly builds](bleeding-edge.md)

## Containers

- [Install as a Docker service](docker.md)
- [Install on Kubernetes](kubernetes.md)
- [Install using the Kubernetes Agent](kubernetes-agent.md)
- [Install on OpenShift](openshift.md)

## Autoscale

- [Install in autoscaling mode using Docker machine](../executors/docker_machine.md)
- [Install the registry and cache servers](registry_and_cache_servers.md)
