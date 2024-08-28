---
stage: Verify
group: Runner
description: Software for CI/CD jobs.
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Install GitLab Runner

DETAILS:
**Tier:** Free, Premium, Ultimate
**Offering:** GitLab.com, Self-managed

[GitLab Runner](https://gitlab.com/gitlab-org/gitlab-runner) runs the CI/CD jobs defined in GitLab.
GitLab Runner can run as a single binary and has no language-specific requirements.

For security and performance reasons, install GitLab Runner on a machine
separate from the machine that hosts your GitLab instance.

## Supported operating systems

You can install GitLab Runner on:

- Linux from a [GitLab repository](linux-repository.md) or [manually](linux-manually.md)
- [FreeBSD](freebsd.md)
- [macOS](osx.md)
- [Windows](windows.md)

[Bleeding-edge binaries](bleeding-edge.md) are also available.

To use a different operating system, ensure the operating system can compile a Go binary.

## Supported containers

You can install GitLab Runner with:

- [Docker](docker.md)
- [The GitLab Helm chart](kubernetes.md)
- [The GitLab agent for Kubernetes](kubernetes-agent.md)
- [The GitLab Operator](operator.md)

## Supported architectures

GitLab Runner is available for the following architectures:

- x86
- AMD64
- ARM64
- ARM
- s390x
- ppc64le

## System Requirements

GitLab Runner system requirements vary widely and depend on variables unique to each use-case. GitLab Runner instances can be sized individually given these variables and scaled higher or lower as necessary. These variables include:

- The anticipated:
  - CPU load of CI jobs.
  - Memory usage of CI jobs.
- The number of:
  - Concurrent CI jobs.
  - Projects in active development.
  - Developers expected to work in parallel.

For more information, see what [machine types are available for Linux (x86-64)](https://docs.gitlab.com/ee/ci/runners/hosted_runners/linux.html#machine-types-available-for-linux---x86-64) on SaaS.

## FIPS compliant GitLab Runner

In GitLab Runner 14.7 and later, a GitLab Runner binary that is FIPS 140-12 compliant is provided. This binary, built with the [Red Hat Go compiler](https://developers.redhat.com/blog/2019/06/24/go-and-fips-140-2-on-red-hat-enterprise-linux), bypasses the standard library cryptographic routines and instead calls into a FIPS 140-2 validated cryptographic library.

In GitLab Runner 15.1 and later, a [UBI-8 minimal](https://docs.redhat.com/en/documentation/red_hat_enterprise_linux/8/html-single/building_running_and_managing_containers/index#con_understanding-the-ubi-minimal-images_assembly_types-of-container-images) is used as the base for creating the GitLab Runner FIPS image.

NOTE:
Only Red Hat Enterprise Linux (RHEL) distributions are supported.

FIPS compliant GitLab Runner binaries are provided for the following architectures:

- AMD64

Docker images and RPM packages for the same architectures are also provided.

### FIPS compliant GitLab Runner in RHEL

When you use the FIPS version of GitLab Runner in RHEL, you should [enable FIPS mode](https://docs.redhat.com/en/documentation/red_hat_enterprise_linux/8/html/security_hardening/switching-rhel-to-fips-mode_security-hardening).

### FIPS compliant GitLab Runner in other systems and architectures

Refer to this [issue](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28814) to follow progress on adding other architectures and distributions.

## Autoscale

- [Install in autoscaling mode using Docker machine](../executors/docker_machine.md)
- [Install the registry and cache servers](../configuration/speed_up_job_execution.md)

## Upgrading GitLab Runner

To upgrade your version of GitLab Runner, see the instructions for each operating system:

- [Docker](docker.md#upgrade-version)
- [Debian, Ubuntu, Mint, RHEL, CentOS, or Fedora](linux-repository.md#upgrade-gitlab-runner)
- [FreeBSD](freebsd.md#upgrading-to-gitlab-runner-10)
- GNU/Linux
  - [Upgrade with the deb/rpm package](linux-manually.md#upgrade)
  - [Upgrade with the binary file](linux-manually.md#upgrade-1)
- [Kubernetes](kubernetes.md#upgrading-gitlab-runner-using-the-helm-chart)
- [macOS](osx.md#upgrade-gitlab-runner)
- [Windows](windows.md#upgrade)
