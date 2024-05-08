---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Docker Autoscaler executor

DETAILS:
**Status:** Beta

> - Introduced in GitLab Runner 15.11.0 as an [Experiment](https://docs.gitlab.com/ee/policy/experiment-beta-support.html#experiment).
> - [Changed](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29404) to [Beta](https://docs.gitlab.com/ee/policy/experiment-beta-support.html#beta) in GitLab Runner 16.6.

This feature is in [Beta](https://docs.gitlab.com/ee/policy/experiment-beta-support.html#beta) and is not recommended for production use. 
For production environments, you should use the [Docker Machine executor](../fleet_scaling/index.md#monitoring-runners).

Before you use the Docker Autoscaler executor, see the [feedback issue](https://gitlab.com/gitlab-org/gitlab/-/issues/408131) about
GitLab Runner autoscaling for a list of known issues.
The Docker Autoscaler executor is an autoscale-enabled Docker executor that creates instances on-demand to
accommodate the jobs that the runner manager processes. It wraps the [Docker executor](docker.md) so that all
Docker executor options and features are supported.

The Docker Autoscaler uses [fleeting plugins](https://gitlab.com/gitlab-org/fleeting/fleeting) to autoscale.
_Fleeting_ is an abstraction for a group of autoscaled instances, which uses plugins that support cloud providers,
like Google Cloud, AWS, and Azure.

The Docker Autoscaler executor is still in Beta so it is [**not** recommended for production use](https://docs.gitlab.com/ee/policy/experiment-beta-support.html#beta). For Production environments, please consider following the [Docker Machine executor fleet scaling guide instead](../fleet_scaling/index.html#monitoring-runners).

Before using the Docker Autoscaler executor, consider reading through the [GitLab Runner Autoscaling - Feedback issue for the new runner autoscaling solution issue](https://gitlab.com/gitlab-org/gitlab/-/issues/408131) and understanding limitations.

## Install a fleeting plugin

To enable autoscaling for your target platform, install a fleeting plugin. You can install
the AWS, Google Cloud, or Azure fleeting plugin. The AWS, Google Cloud and Azure plugins are currently in [Beta](https://docs.gitlab.com/ee/policy/experiment-beta-support.html#beta). See [epic 2502](https://gitlab.com/groups/gitlab-org/-/epics/2502) for a timeline.

For other official plugins developed by GitLab, see the [fleeting project](https://gitlab.com/gitlab-org/fleeting).

To install the plugin:

1. Install the binary for your host platform:
   - [AWS fleeting plugin](https://gitlab.com/gitlab-org/fleeting/fleeting-plugin-aws/-/releases)
   - [Google Cloud fleeting plugin](https://gitlab.com/gitlab-org/fleeting/fleeting-plugin-googlecompute/-/releases)
   - [Azure fleeting plugin](https://gitlab.com/gitlab-org/fleeting/fleeting-plugin-azure/-/releases)
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

- An AMI with [Docker Engine](https://docs.docker.com/engine/) installed. To enable Runner Manager's access to the Docker socket on the AMI, the user must be part of the `docker` group.
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

### Example: Google Cloud instance group for 1 job per instance

Prerequisites:

- A VM image with [Docker Engine](https://docs.docker.com/engine/) installed, such as [COS](https://cloud.google.com/container-optimized-os/docs).
- A Google Cloud instance group. For **Autoscaling mode**, select **Do not autoscale**. The runner handles autoscaling, not
the Google Cloud instance group.
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
      name             = "my-docker-instance-group" # Google Cloud Instance Group name
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

### Example: Azure scale set for 1 job per instance

Prerequisites:

- An Azure VM Image with [Docker Engine](https://docs.docker.com/engine/) installed.
- An Azure scale set where the autoscaling policy is set to `manual`. The runner handles the scaling.

This configuration supports:

- A capacity per instance of 1
- A use count of 1
- An idle scale of 5
- An idle time of 20 minutes
- A maximum instance count of 10

When the capacity and use count are both set to `1`, each job is given a secure ephemeral instance that cannot be
affected by other jobs. When the job completes, the instance it was executed on is immediately deleted.

When the idle scale is set to `5`, the runner keeps 5 instances available for future demand (because the capacity per instance is 1).
These instances stay for at least 20 minutes.

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
    plugin = "fleeting-plugin-azure"

    capacity_per_instance = 1
    max_use_count = 1
    max_instances = 10

    [runners.autoscaler.plugin_config] # plugin specific configuration (see plugin documentation)
      name = "my-docker-scale-set"
      subscription_id = "9b3c4602-cde2-4089-bed8-889e5a3e7102"
      resource_group_name = "my-resource-group"

    [runners.autoscaler.connector_config]
      username = "azureuser"
      password = "my-scale-set-static-password"
      use_static_credentials = true
      timeout = "10m"
      use_external_addr = true

    [[runners.autoscaler.policy]]
      idle_count = 5
      idle_time = "20m0s"
```
