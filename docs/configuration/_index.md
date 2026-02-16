---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: Config.toml, certificates, autoscaling, proxy setup.
title: Configure GitLab Runner
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

Learn how to configure GitLab Runner.

- [Advanced configuration options](advanced-configuration.md): Use
  the [`config.toml`](https://github.com/toml-lang/toml) configuration file
  to edit runner settings.
- [Use self-signed certificates](tls-self-signed.md): Configure certificates
  that verify TLS peers when connecting to the GitLab server.
- [Autoscale with Docker Machine](autoscale.md): Execute jobs on machines
  created automatically by Docker Machine.
- [Autoscale GitLab Runner on AWS EC2](runner_autoscale_aws/_index.md): Execute jobs on auto-scaled AWS EC2 instances.
- [Autoscale GitLab CI on AWS Fargate](runner_autoscale_aws_fargate/_index.md):
  Use the AWS Fargate driver with the GitLab custom executor to run jobs in AWS ECS.
- [Graphical Processing Units](gpus.md): Use GPUs to execute jobs.
- [The init system](init.md): GitLab Runner installs
  its init service files based on your operating system.
- [Supported shells](../shells/_index.md): Execute builds on different systems by
  using shell script generators.
- [Security considerations](../security/_index.md): Be aware of potential
  security implications when running your jobs with GitLab Runner.
- [Runner monitoring](../monitoring/_index.md): Monitor the behavior of your
  runners.
- [Clean up Docker cache automatically](../executors/docker.md#clear-the-docker-cache):
  If you are running low on disk space, use a cron job to clean old containers and volumes.
- [Configure GitLab Runner to run behind a proxy](proxy.md): Set
  up a Linux proxy and configure GitLab Runner. This setup works well with the Docker executor.
- [Configure GitLab Runner for Oracle Cloud Infrastructure (OCI)](oracle_cloud_performance.md): Optimize your GitLab Runner performance in OCI.
- [Handling rate limited requests](proxy.md#handling-rate-limited-requests).
- [Configure GitLab Runner Operator](configuring_runner_operator.md).
