---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Instance executor (BETA)

> - Introduced in GitLab Runner 15.11.0 as an [Experiment](https://docs.gitlab.com/ee/policy/alpha-beta-support.html).
> - [Changed](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29404) to [Beta](https://docs.gitlab.com/ee/policy/alpha-beta-support.html) in GitLab Runner 16.6.

The instance executor is an autoscale-enabled executor that creates instances on-demand to accommodate
the expected volume of jobs that the runner manager processes.

You can use the instance executor when jobs need full access to the host instance, operating system, and
attached devices. The instance executor can also be configured to accommodate single and multi-tenant jobs
with various levels of isolation and security.

## Nested virtualization

The instance executor supports nested virtualization with the GitLab-developed
[nesting daemon](https://gitlab.com/gitlab-org/fleeting/nesting). The nesting daemon enables creation
and deletion of pre-configured virtual machines on host systems used for isolated and short-lived workloads, like jobs.
Nesting is only supported on Apple Silicon instances.

## Prepare the environment for autoscaling

To enable scaling for your target platform, install the AWS, GCP, or Azure fleeting plugins.
The AWS plugin is in [Beta](https://docs.gitlab.com/ee/policy/alpha-beta-support.html). The GCP and Azure plugins are [Experiments](https://docs.gitlab.com/ee/policy/alpha-beta-support.html).

For other official plugins developed by GitLab, see the [`fleeting` project](https://gitlab.com/gitlab-org/fleeting).

To prepare the environment for autoscaling:

1. Install the binary for your host platform:
   - [AWS fleeting plugin](https://gitlab.com/gitlab-org/fleeting/fleeting-plugin-aws/-/releases)
   - [GCP fleeting plugin](https://gitlab.com/gitlab-org/fleeting/fleeting-plugin-googlecompute/-/releases)
   - [Azure fleeting plugin](https://gitlab.com/gitlab-org/fleeting/fleeting-plugin-azure/-/releases)
1. Ensure plugin binaries are discoverable through the `PATH` environment variable.
1. Create a VM image for the platform you're using. The image must include:
   - Git
   - GitLab Runner
   - Dependencies required by the jobs you plan to run

## Configure the executor to autoscale

Prerequisites:

- You must be an administrator.

To configure the instance executor for autoscaling, update the following sections in the `config.toml`:

- [`[runners.autoscaler]`](../configuration/advanced-configuration.md#the-runnersautoscaler-section)
- [`[runners.instance]`](../configuration/advanced-configuration.md#the-runnersinstance-section)

## AWS autoscaling group configuration examples

### One job per instance

Prerequisites:

- An AMI with at least `git` and GitLab Runner installed.
- An AWS Autoscaling group. For the scaling policy use `none`. The runner handles the scaling.
- An IAM Policy with the [correct permissions](https://gitlab.com/gitlab-org/fleeting/fleeting-plugin-aws#recommended-iam-policy).

This configuration supports:

- A capacity of `1` for each instance.
- A use count of `1`.
- An idle scale of `5`.
- An idle time of 20 minutes.
- A maximum instance count of `10`.

When the capacity and use count are set to `1`, each job is given a secure ephemeral instance that cannot be
affected by other jobs. When the job completes, the instance it was executed on is deleted immediately.

When the capacity for each instance is `1`, and the idle scale is `5`, the runner keeps 5 whole instances
available for future demand. These instances remain for at least 20 minutes.

The runner `concurrent` field is set to 10 (maximum number of instances * capacity per instance).

```toml
concurrent = 10

[[runners]]
  name = "instance autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  shell = "sh"

  executor = "instance"

  # Autoscaler config
  [runners.autoscaler]
    plugin = "fleeting-plugin-aws"

    capacity_per_instance = 1
    max_use_count = 1
    max_instances = 10

    [runners.autoscaler.plugin_config] # plugin specific configuration (see plugin documentation)
      name             = "my-linux-asg"                # AWS Autoscaling Group name
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

### Five jobs per instance with unlimited uses

Prerequisites:

- An AMI with at least `git` and GitLab Runner installed.
- An AWS Autoscaling group with the scaling policy set to `none`. The runner handles the scaling.
- An IAM Policy with the [correct permissions](https://gitlab.com/gitlab-org/fleeting/fleeting-plugin-aws#recommended-iam-policy).

This configuration supports:

- A capacity of `5` for each instance.
- An unlimited use count.
- An idle scale of `5`.
- An idle time of 20 minutes.
- A maximum instance count of `10`.

When the capacity per instance is set to `5` and the use count is unlimited, each instance concurrently
executes 5 jobs for the lifetime of the instance.

When the idle scale is `5`, 1 idle instance is created to accommodate an idle capacity of 5
(due to the capacity for each instance) whenever the in-use capacity is lower than 5. Idle
instances remain for at least 20 minutes.

Jobs executed in these environments should be **trusted** as there is little isolation between
them and each job can affect the performance of another.

The runner `concurrent` field is set to 50 (maximum number instances * capacity per instance).

```toml
concurrent = 50

[[runners]]
  name = "instance autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  shell = "sh"

  executor = "instance"

  # Autoscaler config
  [runners.autoscaler]
    plugin = "fleeting-plugin-aws"

    capacity_per_instance = 5
    max_use_count = 0
    max_instances = 10

    [runners.autoscaler.plugin_config] # plugin specific configuration (see plugin documentation)
      name             = "my-windows-asg"              # AWS Autoscaling Group name
      profile          = "default"                     # optional, default is 'default'
      config_file      = "/home/user/.aws/config"      # optional, default is '~/.aws/config'
      credentials_file = "/home/user/.aws/credentials" # optional, default is '~/.aws/credentials'

    [runners.autoscaler.connector_config]
      username          = "Administrator"
      timeout           = "5m0s"
      use_external_addr = true

    [[runners.autoscaler.policy]]
      idle_count = 5
      idle_time = "20m0s"
```

### Two jobs per instance, unlimited uses, nested virtualization on EC2 Mac instances

Prerequisites:

- An Apple Silicon AMI with [nesting](https://gitlab.com/gitlab-org/fleeting/nesting)
  and [Tart](https://github.com/cirruslabs/tart) installed.
- The Tart VM images that the runner uses. The VM images are specified by the `image` keyword
  of the job. The VM images should have at least `git` and GitLab Runner installed.
- An AWS Autoscaling group. For the scaling policy use `none`, because runner handles the scaling.
  For information about how to set up an ASG for MacOS, see [Implementing autoscaling for EC2 Mac instances](https://aws.amazon.com/blogs/compute/implementing-autoscaling-for-ec2-mac-instances/).
- An IAM policy with the [correct permissions](https://gitlab.com/gitlab-org/fleeting/fleeting-plugin-aws#recommended-iam-policy).

This configuration supports:

- A capacity of `2` for each instance.
- An unlimited use count.
- Nested virtualization to support isolated jobs. Nested virtualization is only available
for Apple silicon instances with [nesting](https://gitlab.com/gitlab-org/fleeting/nesting) installed.
- An idle scale of `5`.
- An idle time of 20 minutes.
- A maximum instance count of `10`.

When the capacity for each instance is `2` and the use count is unlimited, each instance concurrently
executes 2 jobs for the lifetime of the instance.

When the idle scale is `2`, 1 idle instance is created to accommodate an idle capacity of 2
(due to the capacity per instance) whenever the in use capacity is lower than 2. Idle instances remain for at
least 24 hours. This time frame is due to the 24 hour minimal allocation period of AWS MacOS instance hosts.

Jobs executed in this environment do not need to be trusted because
[nesting](https://gitlab.com/gitlab-org/fleeting/nesting) is used for nested virtualization of each job. This
only works on Apple silicon instances.

The runner `concurrent` field is set to 8 (maximum number instances * capacity per instance).

```toml
concurrent = 8

[[runners]]
  name = "macos applesilicon autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  executor = "instance"

  [runners.instance]
    allowed_images = ["*"] # allow any nesting image

  [runners.autoscaler]
    capacity_per_instance = 2 # AppleSilicon can only support 2 VMs per host
    max_use_count = 0
    max_instances = 4

    plugin = "fleeting-plugin-aws"

    [[runners.autoscaler.policy]]
      idle_count = 2
      idle_time  = "24h" # AWS's MacOS instances

    [runners.autoscaler.connector_config]
      username = "ec2-user"
      key_path = "macos-key.pem"
      timeout  = "1h" # connecting to a MacOS instance can take some time, as they can be slow to provision

    [runners.autoscaler.plugin_config]
      name = "mac2metal"
      region = "us-west-2"

    [runners.autoscaler.vm_isolation]
      enabled = true
      nesting_host = "unix:///Users/ec2-user/Library/Application Support/nesting.sock"

    [runners.autoscaler.vm_isolation.connector_config]
      username = "nested-vm-username"
      password = "nested-vm-password"
      timeout  = "20m"
```

## GCP instance group configuration examples

### One job per instance using a GCP instance group

Prerequisites:

- A custom image with at least `git` and GitLab Runner installed.
- A GCP instance group where the autoscaling mode is set to `do not autoscale`. The runner handles the scaling.
- An IAM policy with the [correct permissions](https://gitlab.com/gitlab-org/fleeting/fleeting-plugin-googlecompute#recommended-iam-policy).

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
  name = "instance autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  shell = "sh"

  executor = "instance"

  # Autoscaler config
  [runners.autoscaler]
    plugin = "fleeting-plugin-googlecompute"

    capacity_per_instance = 1
    max_use_count = 1
    max_instances = 10

    [runners.autoscaler.plugin_config] # plugin specific configuration (see plugin documentation)
      name             = "my-linux-instance-group" # GCP Instance Group name
      project          = "my-gcp-project"
      zone             = "europe-west1-c"
      credentials_file = "/home/user/.config/gcloud/application_default_credentials.json" # optional, default is '~/.config/gcloud/application_default_credentials.json'

    [runners.autoscaler.connector_config]
      username          = "runner"
      use_external_addr = true

    [[runners.autoscaler.policy]]
      idle_count = 5
      idle_time = "20m0s"
```

### Five jobs per instance, unlimited uses, using GCP Instance group

Prerequisites:

- A custom image with at least `git` and GitLab Runner installed.
- An Instance group. For the "Autoscaling mode" select "do not autoscale", as Runner handles the scaling.
- An IAM Policy with the [correct permissions](https://gitlab.com/gitlab-org/fleeting/fleeting-plugin-googlecompute#recommended-iam-policy).

This configuration supports:

- A capacity per instance of 5
- An unlimited use count
- An idle scale of 5
- An idle time of 20 minutes
- A maximum instance count of 10

When the capacity is set `5` and the use count is unlimited, each instance concurrently
executes 5 jobs for the lifetime of the instance.

Jobs executed in these environments should be **trusted** as there is little isolation between them and each job
can affect the performance of another.

When the idle scale is set to `5`, 1 idle instance is created to accommodate an idle capacity of 5
(due to the capacity per instance) whenever the in-use capacity is lower than 5. Idle instances
stay for at least 20 minutes.

The runner `concurrent` field is set to 50 (maximum number instances * capacity per instance).

```toml
concurrent = 50

[[runners]]
  name = "instance autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  shell = "sh"

  executor = "instance"

  # Autoscaler config
  [runners.autoscaler]
    plugin = "fleeting-plugin-googlecompute"

    capacity_per_instance = 5
    max_use_count = 0
    max_instances = 10

    [runners.autoscaler.plugin_config] # plugin specific configuration (see plugin documentation)
      name             = "my-windows-instance-group" # GCP Instance Group name
      project          = "my-gcp-project"
      zone             = "europe-west1-c"
      credentials_file = "/home/user/.config/gcloud/application_default_credentials.json" # optional, default is '~/.config/gcloud/application_default_credentials.json'

    [runners.autoscaler.connector_config]
      username          = "Administrator"
      timeout           = "5m0s"
      use_external_addr = true

    [[runners.autoscaler.policy]]
      idle_count = 5
      idle_time = "20m0s"
```

## Azure scale set configuration examples

### One job per instance using a Azure scale set

Prerequisites:

- A custom image with at least `git` and GitLab Runner installed.
- An Azure scale set where the autoscaling mode is set to `manual`. The runner handles the scaling.

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
  name = "instance autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  shell = "sh"

  executor = "instance"

  # Autoscaler config
  [runners.autoscaler]
    plugin = "fleeting-plugin-azure"

    capacity_per_instance = 1
    max_use_count = 1
    max_instances = 10

    [runners.autoscaler.plugin_config] # plugin specific configuration (see plugin documentation)
      name                = "my-linux-scale-set" # Azure scale set name
      subscription_id     = "9b3c4602-cde2-4089-bed8-889e5a3e7102"
      resource_group_name = "my-resource-group" 

    [runners.autoscaler.connector_config]
      username               = "runner"
      password               = "my-scale-set-static-password"
      use_static_credentials = true
      timeout                = "10m"
      use_external_addr      = true

    [[runners.autoscaler.policy]]
      idle_count = 5
      idle_time  = "20m0s"
```

### Five jobs per instance, unlimited uses, using GCP Instance group

Prerequisites:

- A custom image with at least `git` and GitLab Runner installed.
- An Azure scale set where the autoscaling mode is set to `manual`. The runner handles the scaling.

This configuration supports:

- A capacity per instance of 5
- An unlimited use count
- An idle scale of 5
- An idle time of 20 minutes
- A maximum instance count of 10

When the capacity is set `5` and the use count is unlimited, each instance concurrently
executes 5 jobs for the lifetime of the instance.

Jobs executed in these environments should be **trusted** as there is little isolation between them and each job
can affect the performance of another.

When the idle scale is set to `5`, 1 idle instance is created to accommodate an idle capacity of 5
(due to the capacity per instance) whenever the in-use capacity is lower than 5. Idle instances
stay for at least 20 minutes.

The runner `concurrent` field is set to 50 (maximum number instances * capacity per instance).

```toml
concurrent = 50

[[runners]]
  name = "instance autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  shell = "sh"

  executor = "instance"

  # Autoscaler config
  [runners.autoscaler]
    plugin = "fleeting-plugin-azure"

    capacity_per_instance = 5
    max_use_count = 0
    max_instances = 10

    [runners.autoscaler.plugin_config] # plugin specific configuration (see plugin documentation)
      name                = "my-windows-scale-set" # Azure scale set name
      subscription_id     = "9b3c4602-cde2-4089-bed8-889e5a3e7102"
      resource_group_name = "my-resource-group" 

    [runners.autoscaler.connector_config]
      username               = "Administrator"
      password               = "my-scale-set-static-password"
      use_static_credentials = true
      timeout                = "10m"
      use_external_addr      = true

    [[runners.autoscaler.policy]]
      idle_count = 5
      idle_time = "20m0s"
```

## Troubleshooting

When working with the Instance executor, you might encounter the following issues:

### sh: 1: eval: Running on ip-x.x.x.x via runner-host...n: not found

This error typically occurs when the `eval` command in the preparation step fails. To resolve this error, switch to `bash` shell and enable the [feature flag](../configuration/feature-flags.md) `FF_USE_NEW_BASH_EVAL_STRATEGY`.
