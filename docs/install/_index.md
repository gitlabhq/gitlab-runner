---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: Software for CI/CD jobs.
title: Install GitLab Runner
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

[GitLab Runner](https://gitlab.com/gitlab-org/gitlab-runner) runs the CI/CD jobs defined in GitLab.
GitLab Runner can run as a single binary and has no language-specific requirements.

For security and performance reasons, install GitLab Runner on a machine
separate from the machine that hosts your GitLab instance.

Before you install, review the [system requirements and supported platforms](requirements.md).

## Operating systems

{{< cards >}}

- [Linux](linux-repository.md)
- [Linux manual install](linux-manually.md)
- [FreeBSD](freebsd.md)
- [macOS](osx.md)
- [Windows](windows.md)
- [z/OS](z-os.md)

{{< /cards >}}

## Containers

{{< cards >}}

- [Docker](docker.md)
- [Helm chart](kubernetes.md)
- [GitLab agent](kubernetes-agent.md)
- [Operator](operator.md)

{{< /cards >}}

## Other installation options

{{< cards >}}

- [Bleeding edge releases](bleeding-edge.md)

{{< /cards >}}
