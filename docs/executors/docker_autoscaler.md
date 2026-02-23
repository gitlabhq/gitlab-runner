---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Docker Autoscaler executor
---

{{< history >}}

- Introduced in GitLab Runner 15.11.0 as an [experiment](https://docs.gitlab.com/policy/development_stages_support/#experiment).
- [Changed](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29404) to [beta](https://docs.gitlab.com/policy/development_stages_support/#beta) in GitLab Runner 16.6.
- [Generally available](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29221) in GitLab Runner 17.1.

{{< /history >}}

Before you use the Docker Autoscaler executor, see the [feedback issue](https://gitlab.com/gitlab-org/gitlab/-/issues/408131) about
GitLab Runner autoscaling for a list of known issues.

The Docker Autoscaler executor is an autoscale-enabled Docker executor that creates instances on demand to
accommodate the jobs that the runner manager processes. It wraps the [Docker executor](docker.md) so that all
Docker executor options and features are supported.

The Docker Autoscaler uses [fleeting plugins](https://gitlab.com/gitlab-org/fleeting/plugins) to autoscale.
Fleeting is an abstraction for a group of autoscaled instances, which uses plugins that support cloud providers,
like Google Cloud, AWS, and Azure.

## Install a fleeting plugin

To install a plugin for your target platform, see [Install the fleeting plugin](../fleet_scaling/fleeting.md#install-a-fleeting-plugin).
For specific configuration details, see the [respective plugin project documentation](https://gitlab.com/gitlab-org/fleeting/plugins).

## Configure Docker Autoscaler

The Docker Autoscaler executor wraps the [Docker executor](docker.md) so that all Docker executor options and
features are supported.

To configure the Docker Autoscaler, in the `config.toml`:

- In the [`[runners]`](../configuration/advanced-configuration.md#the-runners-section) section, specify
  the `executor` as `docker-autoscaler`.
- In the following sections, configure the Docker Autoscaler based on your requirements:
  - [`[runners.docker]`](../configuration/advanced-configuration.md#the-runnersdocker-section)
  - [`[runners.autoscaler]`](../configuration/advanced-configuration.md#the-runnersautoscaler-section)

### Dedicated autoscaling groups for each runner configuration

Each Docker Autoscaler configuration must have its own dedicated autoscaling resource:

- For AWS, a dedicated auto scaling group
- For GCP, a dedicated instance group
- For Azure, a dedicated scale set

Do not share these autoscaling resources across:

- Multiple runner managers (separate GitLab Runner installations)
- Multiple `[[runners]]` entries within the same runner manager's `config.toml`

The Docker Autoscaler keeps track of the instance state that must be synchronized with the cloud
provider's autoscaling resource. When multiple systems attempt to manage the same autoscaling
resource, they might issue conflicting scaling commands, resulting in unpredictable behavior, job
failures, and potentially higher costs.

### Example: AWS autoscaling for 1 job per instance

Prerequisites:

- An AMI with [Docker Engine](https://docs.docker.com/engine/) installed. To enable Runner Manager's access to the Docker socket on the AMI, the user must be part of the `docker` group.

  {{< alert type="note" >}}

  The AMI does not require GitLab Runner to be installed. The instances launched using the AMI must not register themselves as runners in GitLab.

  {{< /alert >}}

- An AWS autoscaling group. The runner directly manages all scaling behavior. For the scaling policy, use `none` and turn on instance scale-in protection. If you have configured multiple availability zones, turn off the `AZRebalance` process.
- An IAM policy with the [correct permissions](https://gitlab.com/gitlab-org/fleeting/plugins/aws#recommended-iam-policy).

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
    plugin = "aws" # in GitLab 16.11 and later, ensure you run `gitlab-runner fleeting install` to automatically install the plugin

    # in GitLab 16.10 and earlier, manually install the plugin and use:
    # plugin = "fleeting-plugin-aws"

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

- A VM image with [Docker Engine](https://docs.docker.com/engine/) installed, such as [`COS`](https://docs.cloud.google.com/container-optimized-os/docs).

  {{< alert type="note" >}}

  The VM image does not require GitLab Runner to be installed. The instances launched using the VM image must not register themselves as runners in GitLab.

  {{< /alert >}}

- A single-zone Google Cloud instance group. For **Autoscaling mode**, select **Do not autoscale**. The runner handles autoscaling, not
  the Google Cloud instance group.

  {{< alert type="note" >}}

  Multi-zone instance groups are not currently supported. An [issue](https://gitlab.com/gitlab-org/fleeting/plugins/googlecloud/-/issues/20)
  exists to support multi-zone instance groups in the future.

  {{< /alert >}}

- An IAM policy with the [correct permissions](https://gitlab.com/gitlab-org/fleeting/plugins/googlecloud#required-permissions).
  If you're deploying your runner in a GKE cluster, you can add an IAM binding
  between the Kubernetes service account and the GCP service account.
  You can add this binding with the `iam.workloadIdentityUser` role to authenticate
  to GCP instead of using a key file with `credentials_file`.

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
    plugin = "googlecloud" # for >= 16.11, ensure you run `gitlab-runner fleeting install` to automatically install the plugin

    # for versions < 17.0, manually install the plugin and use:
    # plugin = "fleeting-plugin-googlecompute"

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

- An Azure VM image with [Docker Engine](https://docs.docker.com/engine/) installed.

  {{< alert type="note" >}}

  The VM image does not require GitLab Runner to be installed. The instances launched using the VM image must not register themselves as runners in GitLab.

  {{< /alert >}}

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
    plugin = "azure" # for >= 16.11, ensure you run `gitlab-runner fleeting install` to automatically install the plugin

    # for versions < 17.0, manually install the plugin and use:
    # plugin = "fleeting-plugin-azure"

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

## Slot-based cgroup support

The Docker Autoscaler executor supports slot-based cgroups for improved resource isolation between concurrent jobs. Cgroup paths are automatically applied to Docker containers using the `--cgroup-parent` flag.

For detailed information about slot-based cgroups, including benefits, prerequisites, and setup instructions,
see [slot-based cgroup support](../configuration/slot_based_cgroups.md).

### Docker-specific configuration

In addition to the standard slot cgroup configuration, you can specify a separate cgroup template for service containers:

```toml
[[runners]]
  executor = "docker+autoscaler"
  use_slot_cgroups = true
  slot_cgroup_template = "gitlab-runner/slot-${slot}"

  [runners.docker]
    service_slot_cgroup_template = "gitlab-runner/service-slot-${slot}"
```

For all available options, see the [slot-based cgroup configuration documentation](../configuration/slot_based_cgroups.md#docker-specific-configuration).

## Troubleshooting

### `ERROR: error during connect: ssh tunnel: EOF ()`

When instances are removed by an external source (for example, an autoscaling group or automated script),
jobs fail with the following error:

```plaintext
ERROR: Job failed (system failure): error during connect: Post "http://internal.tunnel.invalid/v1.43/containers/xyz/wait?condition=not-running": ssh tunnel: EOF ()
```

And the GitLab Runner logs show an `instance unexpectedly removed` error
for the instance ID assigned to the job:

```plaintext
ERROR: instance unexpectedly removed    instance=<instance_id> max-use-count=9999 runner=XYZ slots=map[] subsystem=taskscaler used=45
```

To resolve this error, check the events related to the instance
on your cloud provider platform. For example, on AWS, check the
CloudTrail event history for the event source `ec2.amazonaws.com`.

### `ERROR: Preparation failed: unable to acquire instance: context deadline exceeded`

When you use the [AWS fleeting plugin](https://gitlab.com/gitlab-org/fleeting/plugins/aws), jobs might fail intermittently
with the following error:

```plaintext
ERROR: Preparation failed: unable to acquire instance: context deadline exceeded
```

This often shows up in the AWS CloudWatch logs because the `reserved` instance count oscillates up and down:

```plaintext
"2024-07-23T18:10:24Z","instance_count:1,max_instance_count:1000,acquired:0,unavailable_capacity:0,pending:0,reserved:0,idle_count:0,scale_factor:0,scale_factor_limit:0,capacity_per_instance:1","required scaling change",
"2024-07-23T18:10:25Z","instance_count:1,max_instance_count:1000,acquired:0,unavailable_capacity:0,pending:0,reserved:1,idle_count:0,scale_factor:0,scale_factor_limit:0,capacity_per_instance:1","required scaling change",
"2024-07-23T18:11:15Z","instance_count:1,max_instance_count:1000,acquired:0,unavailable_capacity:0,pending:0,reserved:0,idle_count:0,scale_factor:0,scale_factor_limit:0,capacity_per_instance:1","required scaling change",
"2024-07-23T18:11:16Z","instance_count:1,max_instance_count:1000,acquired:0,unavailable_capacity:0,pending:0,reserved:1,idle_count:0,scale_factor:0,scale_factor_limit:0,capacity_per_instance:1","required scaling change",
```

To resolve this error, ensure that the `AZRebalance` process is disabled for your autoscaling group in AWS.

### `Job failures when scaling from zero instances on Azure VMSS`

Microsoft Azure Virtual Machine Scale Sets have an [overprovisioning feature](https://learn.microsoft.com/en-us/azure/virtual-machine-scale-sets/virtual-machine-scale-sets-design-overview#overprovisioning), which can cause job failures. When Azure scales up, it creates extra VMs to ensure capacity and then terminates them after it meets the requested capacity. This behavior conflicts with GitLab Runner's instance tracking, which causes the autoscaler to assign jobs to instances that Azure is about to terminate.

Disable overprovisioning by setting `overprovision` to `false` in your VMSS configuration.
