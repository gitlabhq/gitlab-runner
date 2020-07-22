---
comments: false
---

# Configuring GitLab Runner

Below you can find some specific documentation on configuring GitLab Runner, the
shells supported, the security implications using the various executors, as
well as information how to set up Prometheus metrics:

- [Advanced configuration options](advanced-configuration.md) Learn how to use the [TOML](https://github.com/toml-lang/toml) configuration file that GitLab Runner uses.
- [Use self-signed certificates](tls-self-signed.md) Configure certificates that are used to verify TLS peer when connecting to the GitLab server.
- [Autoscaling using Docker machine](autoscale.md) Execute jobs on machines that are created on demand using Docker machine.
- [Autoscaling GitLab Runner on AWS EC2](runner_autoscale_aws/index.md)
- [Autoscaling GitLab CI on AWS Fargate](runner_autoscale_aws_fargate/index.md) Learn how to configure the Runner and the AWS Fargate driver with the
  Runner Custom executor.
- [The init system of GitLab Runner](init.md) Learn how the Runner installs its init service files based on your operating system.
- [Supported shells](../shells/index.md) Learn what shell script generators are supported that allow to execute builds on different systems.
- [Security considerations](../security/index.md) Be aware of potential security implications when running your jobs with GitLab Runner.
- [Runner monitoring](../monitoring/README.md) Learn how to monitor the Runner's behavior.
- [Cleanup the Docker images automatically](https://gitlab.com/gitlab-org/gitlab-runner-docker-cleanup) A simple Docker application that automatically garbage collects the GitLab Runner caches and images when running low on disk space.
- [Configure GitLab Runner to run behind a proxy](proxy.md) Learn how to set up a Linux proxy and configure GitLab Runner. Especially useful for the Docker executor.
- [Best practice for using GitLab Runner](../best_practice/index.md).
- [Handling rate limited requests](proxy.md#handling-rate-limited-requests)
