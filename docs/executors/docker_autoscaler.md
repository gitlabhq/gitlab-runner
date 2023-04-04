---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Docker Autoscaler executor (Alpha)

> The Docker Autoscaler feature (alpha) was introduced in GitLab Runner 15.10.0.

The Docker Autoscaler executor is an autoscale-enabled Docker executor that creates instances on-demand to
accommodate the jobs that the runner manager processes.

The autoscaler uses [fleeting](https://gitlab.com/gitlab-org/fleeting/fleeting) plugins. `fleeting` is an abstraction
for a group of autoscaled instances and uses plugins that support different cloud providers (such as GCP, AWS and
Azure). This allows instances to be created on-demand to accomodate the jobs that a GitLab Runner manager processes.

## Preparing the environment

To get started with the Docker Autoscaler executor, select a `fleeting` plugin that targets the
platform you want to autoscale on.

Whilst this feature is in alpha, our focus at the moment is with the
[AWS fleeting plugin](https://gitlab.com/gitlab-org/fleeting/fleeting-plugin-aws). However, you can find our official
plugins [here](https://gitlab.com/gitlab-org/fleeting) and the goal of our plugin system is to also support community
contributed plugins.

To install the AWS plugin, check the
[release page](https://gitlab.com/gitlab-org/fleeting/fleeting-plugin-aws/-/releases) and download the binary for your
host platform. `fleeting` plugin binaries need to be discoverable through the `PATH` environment variable.

## Configuration

The Docker Autoscaler executor wraps the [Docker executor](docker.md). Therefore, all Docker Executor options and
features are supported. To enable the autoscaler, the executor `docker-autoscaler` must be used.

- [Docker Executor configuration](../configuration/advanced-configuration.md#the-runnersdocker-section)
- [Autoscaler configuration](../configuration/advanced-configuration.md#the-runnersautoscaler-section)

## Examples

### 1 job per instance using AWS Autoscaling Group

Prerequisites: 

- An AMI with [Docker Engine](https://docs.docker.com/engine/) installed.
- An AWS Autoscaling group. For the scaling policy use "none", as Runner handles the scaling.
- An IAM Policy with the [correct permissions](https://gitlab.com/gitlab-org/fleeting/fleeting-plugin-aws#recommended-iam-policy)

This configuration supports:

- A capacity per instance of 1
- A use count of 1
- An idle scale of 5
- An idle time of 20 minutes
- A maximum instance count of 10

By setting the capacity and use count to both 1, each job is given a secure ephemeral instance that cannot be
affected by other jobs. As soon the job is complete the instance it was executed on is immediately deleted.

With an idle scale of 5, the runner will try to keep 5 whole instances (because the capacity per instance is 1)
available for future demand. These instances will stay for at least 20 minutes.

The runner `concurrent` field is set to 10 (maximum number instances * capacity per instance).

```toml
concurrent = 10

[[runners]]
  name = "instance autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  shell = "sh"                                        # use powershell or pwsh for Windows AMIs

  # uncomment for Windows AMIs when the Runner manager is hosted on Linux
  # environment = ["FF_USE_POWERSHELL_PATH_RESOLVER=1"]

  executor = "docker-autoscaler"

  # Docker Executor config
  [runners.docker]
    image = "busybox:latest"

  # Autoscaler config
  [runners.autoscaler]
    plugin = "fleeting-plugin-aws"

    capacity_per_instance = 1
    max_use_count = 1
    max_instances = 10

    [runners.autoscaler.plugin_config] # plugin specific configuration (see plugin documentation)
      name             = "my-docker-asg"               # AWS Autoscaling Group name
      profile          = "default"                     # optional, default is 'default'
      config_file      = "/home/user/.aws/config"      # optional, default is '~/.aws/config'
      credentials_file = "/home/user/.aws/credentials" # optional, default is '~/.aws/credentials'

    [runners.autoscaler.connector_config]
      username          = "ec2-user"
      use_external_addr = true

    [[runners.autoscaler.policy]]
      idle_count = 5
      idle_time = "20m0s"
```
