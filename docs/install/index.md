---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/product/ux/technical-writing/#assignments
comments: false
---

# Install GitLab Runner **(FREE)**

GitLab Runner can be installed and used on GNU/Linux, macOS, FreeBSD, and Windows.
You can install it:

- In a container.
- By downloading a binary manually.
- By using a repository for rpm/deb packages.

GitLab Runner officially supported binaries are available for the following architectures:

- x86, AMD64, ARM64, ARM, s390x, ppc64le

Official packages are available for the following Linux distributions:

- CentOS, Debian, Ubuntu, RHEL, Fedora, Mint, Oracle, Amazon

GitLab Runner officially supports the following operating systems:

- Linux, Windows, macOS, FreeBSD

You can find information on the different installation methods below.
You can also view installation instructions in GitLab by going to your project's
**Settings > CI / CD**, expanding the **Runners** section, and clicking
**Show runner installation instructions**.

After you install GitLab Runner, you must [register individual runners](../register/index.md) with your GitLab instance. This instance can be self-managed, or you can use GitLab.com.

GitLab Runner runs the CI/CD jobs that are defined in GitLab.

## FIPS compliant GitLab Runner

As of GitLab Runner 14.7, we provide a FIPS 140-12 compliant GitLab Runner binary. This binary, built with the [Red Hat Go compiler](https://developers.redhat.com/blog/2019/06/24/go-and-fips-140-2-on-red-hat-enterprise-linux), bypasses the standard library cryptographic routines and instead calls into a FIPS 140-2 validated cryptographic library.

NOTE:
Only Red Hat Enterprise Linux (RHEL) distributions are supported.

FIPS compliant GitLab Runner binaries are provided for the following architectures:

- AMD64

Docker images and RPM packages for the same architectures are also provided.

### FIPS compliant GitLab Runner in RHEL

When you use the FIPS version of GitLab Runner in RHEL, you should [enable FIPS mode](https://access.redhat.com/documentation/en-us/red_hat_enterprise_linux/8/html/security_hardening/assembly_installing-a-rhel-8-system-with-fips-mode-enabled_security-hardening).

### FIPS compliant GitLab Runner in other systems and architectures

Refer to this [issue](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28814) to follow progress on adding other architectures and distributions.

## Repositories

- [Install using the GitLab repository for Debian/Ubuntu/CentOS/Red Hat](linux-repository.md)

## Binaries

- [Install on GNU/Linux](linux-manually.md)
- [Install on macOS](osx.md)
- [Install on Windows](windows.md)
- [Install on FreeBSD](freebsd.md)
- [Install nightly builds](bleeding-edge.md)

## Containers

- [Install as a Docker service](docker.md)
- [Install on Kubernetes](kubernetes.md)
- [Install using the agent for Kubernetes](kubernetes-agent.md)
- [Install as GitLab Runner Operator](operator.md)

## Autoscale

- [Install in autoscaling mode using Docker machine](../executors/docker_machine.md)
- [Install the registry and cache servers](../configuration/speed_up_job_execution.md)
