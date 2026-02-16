---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: Software for CI/CD jobs.
title: System requirements and supported platforms
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

## Supported operating systems

You can install GitLab Runner on:

- Linux from a [GitLab repository](linux-repository.md) or [manually](linux-manually.md)
- [FreeBSD](freebsd.md)
- [macOS](osx.md)
- [Windows](windows.md)
- [z/OS](z-os.md)

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
- riscv64
- loong64

## System requirements

The system requirements for GitLab Runner depend on the following considerations:

- Anticipated CPU load of CI/CD jobs
- Anticipated memory usage of CI/CD jobs
- Number of concurrent CI/CD jobs
- Number of projects in active development
- Number of developers expected to work in parallel

For more information about the machine types available for GitLab.com,
see [GitLab-hosted runners](https://docs.gitlab.com/ci/runners/).

## FIPS-compliant GitLab Runner

A GitLab Runner binary compliant with FIPS 140-2 is available for
Red Hat Enterprise Linux (RHEL) distributions and the AMD64 architecture.
Support for other distributions and architectures is proposed in
[issue 28814](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28814).

This binary is built with the [Red Hat Go compiler](https://developers.redhat.com/blog/2019/06/24/go-and-fips-140-2-on-red-hat-enterprise-linux)
and calls into a FIPS 140-2 validated cryptographic library.
A [UBI-8 minimal image](https://docs.redhat.com/en/documentation/red_hat_enterprise_linux/8/html-single/building_running_and_managing_containers/index#con_understanding-the-ubi-minimal-images_assembly_types-of-container-images) is used as the base for creating the GitLab Runner FIPS image.

For more information about using FIPS-compliant GitLab Runner in RHEL, see
[Switching RHEL to FIPS mode](https://docs.redhat.com/en/documentation/red_hat_enterprise_linux/8/html/security_hardening/switching-rhel-to-fips-mode_security-hardening).
