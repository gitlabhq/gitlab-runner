---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Parallels
---

The Parallels executor uses the [Parallels Desktop](https://www.parallels.com/) virtualization software to run CI/CD jobs in virtual machines (VMs) on macOS.
Parallels Desktop can run Windows, Linux, and other operating systems alongside macOS.

The Parallels executor works similarly to the VirtualBox executor.
It creates and manages virtual machines and executes your GitLab CI/CD jobs.
Each job runs in a clean VM environment, providing isolation between builds.
For configuration information, see [VirtualBox executor](virtualbox.md).

{{< alert type="note" >}}

Parallels executors do not support local cache. [Distributed cache](../configuration/speed_up_job_execution.md) is supported.

{{< /alert >}}
