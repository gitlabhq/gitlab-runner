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

GitLab Runner runs the CI/CD jobs that are defined in GitLab.

You can install GitLab Runner on your infrastructure,
inside a Docker container, or deployed to a Kubernetes cluster.

GitLab Runner is open-source and written in [Go](https://go.dev). It can run
as a single binary and has no language-specific requirements.

After you install GitLab Runner, you must [create and register runners](../register/index.md)
with your GitLab instance. The GitLab instance can be self-managed or you can use GitLab.com.
You can also follow the tutorial,
[Create, register, and run your own project runner](https://docs.gitlab.com/ee/tutorials/create_register_first_runner/).

GitLab Runner can be installed and used on GNU/Linux, macOS, FreeBSD, and Windows.
You can install it:

- In a container.
- By downloading a binary manually.
- By using a repository for rpm/deb packages.

GitLab Runner officially supported binaries are available for the following architectures:

- x86, AMD64, ARM64, ARM, s390x, ppc64le

Official packages are available for the following Linux distributions:

- CentOS, Debian, Ubuntu, RHEL, Fedora, Mint, Oracle, Amazon

GitLab Runner officially supports the following operating systems. If you prefer to use a
different operating system that is not officially supported, it must be able to compile a
Go binary.

- Linux, Windows, macOS, FreeBSD

NOTE:
For security and performance reasons, you should install GitLab Runner on a machine that
is separate to the machine that hosts your GitLab instance.

## System Requirements

GitLab Runner system requirements vary widely and depend on variables unique to each use-case. GitLab Runner instances can be sized individually given these variables and scaled higher or lower as necessary. These variables include:

- The anticipated:
  - CPU load of CI jobs.
  - Memory usage of CI jobs.
- The number of:
  - Concurrent CI jobs.
  - Projects in active development.
  - Developers expected to work in parallel.

For more information, see what [machine types are available for Linux (x86-64)](https://docs.gitlab.com/ee/ci/runners/saas/linux_saas_runner.html#machine-types-available-for-linux-x86-64) on SaaS.

## FIPS compliant GitLab Runner

In GitLab Runner 14.7 and later, a GitLab Runner binary that is FIPS 140-12 compliant is provided. This binary, built with the [Red Hat Go compiler](https://developers.redhat.com/blog/2019/06/24/go-and-fips-140-2-on-red-hat-enterprise-linux), bypasses the standard library cryptographic routines and instead calls into a FIPS 140-2 validated cryptographic library.

In GitLab Runner 15.1 and later, a [UBI-8 minimal](https://access.redhat.com/documentation/en-us/red_hat_enterprise_linux/8/html-single/building_running_and_managing_containers/index#con_understanding-the-ubi-minimal-images_assembly_types-of-container-images) is used as the base for creating the GitLab Runner FIPS image.

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
