---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Docker Autoscaler executor (Experiment)

> Introduced in GitLab Runner 15.11.0. This feature is an [Experiment](https://docs.gitlab.com/ee/policy/alpha-beta-support.html)

The Docker Autoscaler executor is an autoscale-enabled Docker executor that creates instances on-demand to
accommodate the jobs that the runner manager processes. It wraps the [Docker executor](docker.md) so that all
Docker executor options and features are supported.

The Docker Autoscaler uses [fleeting](https://gitlab.com/gitlab-org/fleeting/fleeting) plugins to autoscale. `fleeting`
is an abstraction for a group of autoscaled instances. It uses plugins that support different cloud providers,
like Google Cloud Platform (GCP), AWS, and Azure.

## Install a fleeting plugin

To enable autoscaling for your target platform, install a fleeting plugin. You can install
either the AWS or GCP fleeting plugin. Both plugins are [Experiments](https://docs.gitlab.com/ee/policy/alpha-beta-support.html).

For other official plugins developed by GitLab, see the [`fleeting` project](https://gitlab.com/gitlab-org/fleeting).

To install the plugin:

1. Install the binary for your host platform:
   - [AWS fleeting plugin](https://gitlab.com/gitlab-org/fleeting/fleeting-plugin-aws/-/releases)
   - [GCP fleeting plugin](https://gitlab.com/gitlab-org/fleeting/fleeting-plugin-googlecompute/-/releases)
1. Ensure plugin binaries are discoverable through the `PATH` environment variable.

## Configure Docker Autoscaler

The Docker Autoscaler executor wraps the [Docker executor](docker.md) so that all Docker executor options and
features are supported.

To configure the Docker Autoscaler, in the `config.toml`:

- In the [`[runners]`](../configuration/advanced-configuration.md#the-runners-section) section, specify
  the `executor` as `docker-autoscaler`.
- In the following sections, configure the Docker Autoscaler based on your requirements:
  - [`[runners.docker]`](../configuration/advanced-configuration.md#the-runnersdocker-section)
  - [`[runners.autoscaler]`](../configuration/advanced-configuration.md#the-runnersautoscaler-section)

### Example: AWS autoscaling for 1 job per instance

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

With an idle scale of 5, the runner tries to keep 5 whole instances (because the capacity per instance is 1)
available for future demand. These instances stay for at least 20 minutes.

The runner `concurrent` field is set to 10 (maximum number instances * capacity per instance).

```toml
concurrent = 10

[[runners]]
  name = "docker autoscaler example"
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

### Example: GCP instance group for 1 job per instance

Prerequisites:

- A VM image with [Docker Engine](https://docs.docker.com/engine/) installed, such as [COS](https://cloud.google.com/container-optimized-os/docs).
- An Instance group. For the "Autoscaling mode" select "do not autoscale", as Runner handles the scaling.
- An IAM Policy with the [correct permissions](https://gitlab.com/gitlab-org/fleeting/fleeting-plugin-googlecompute#recommended-iam-policy).

This configuration supports:

- A capacity per instance of 1
- A use count of 1
- An idle scale of 5
- An idle time of 20 minutes
- A maximum instance count of 10

By setting the capacity and use count to both 1, each job is given a secure ephemeral instance that cannot be
affected by other jobs. As soon the job is complete the instance it was executed on is immediately deleted.

With an idle scale of 5, the runner tries to keep 5 whole instances (because the capacity per instance is 1)
available for future demand. These instances stay for at least 20 minutes.

The runner `concurrent` field is set to 10 (maximum number instances * capacity per instance).

```toml
concurrent = 10

[[runners]]
  name = "docker autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  shell = "sh"                                        # use powershell or pwsh for Windows Images

  # uncomment for Windows Images when the Runner manager is hosted on Linux
  # environment = ["FF_USE_POWERSHELL_PATH_RESOLVER=1"]

  executor = "docker-autoscaler"

  # Docker Executor config
  [runners.docker]
    image = "busybox:latest"

  # Autoscaler config
  [runners.autoscaler]
    plugin = "fleeting-plugin-googlecompute"

    capacity_per_instance = 1
    max_use_count = 1
    max_instances = 10

    [runners.autoscaler.plugin_config] # plugin specific configuration (see plugin documentation)
      name             = "my-docker-instance-group" # GCP Instance Group name
      project          = "my-gcp-project"
      zone             = "europe-west1"
      credentials_file = "/home/user/.config/gcloud/application_default_credentials.json" # optional, default is '~/.config/gcloud/application_default_credentials.json'

    [runners.autoscaler.connector_config]
      username          = "runner"
      use_external_addr = true

    [[runners.autoscaler.policy]]
      idle_count = 5
      idle_time = "20m0s"
```
