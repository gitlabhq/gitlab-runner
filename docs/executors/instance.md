---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Instance executor (Experiment)

> Introduced in GitLab Runner 15.10.0. This feature is an [Experiment](https://docs.gitlab.com/ee/policy/alpha-beta-support.html).

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

To enable scaling for your target platform, install a fleeting plugin. You can install either the AWS or GCP fleeting plugins.
Both plugins are [Experiments](https://docs.gitlab.com/ee/policy/alpha-beta-support.html).

For other official plugins developed by GitLab, see the [`fleeting` project](https://gitlab.com/gitlab-org/fleeting).

To prepare the environment for autoscaling:

1. Install the binary for your host platform:
   - [AWS fleeting plugin](https://gitlab.com/gitlab-org/fleeting/fleeting-plugin-aws/-/releases)
   - [GCP fleeting plugin](https://gitlab.com/gitlab-org/fleeting/fleeting-plugin-googlecompute/-/releases)
1. Ensure plugin binaries are discoverable through the `PATH` environment variable.
1. Create an Amazon Machine Image (AMI) or GCP custom image. The image must include:
   - Git
   - GitLab Runner
   - Dependencies required by the jobs you plan to run

## Configure the executor to autoscale

Prerequisites:

- You must be an administrator.

To configure the instance executor for autoscaling, update the following sections in the `config.toml`:

- [`[runners.autoscaler]`](../configuration/advanced-configuration.md#the-runnersautoscaler-section)
- [`[runners.instance]`](../configuration/advanced-configuration.md#the-runnersinstance-section-alpha)

## Examples

::Tabs

:::TabTitle AWS

### 1 job per instance using an AWS Autoscaling group

Prerequisites:

- An AMI with at least `git` and GitLab Runner installed.
- An AWS Autoscaling group. For the scaling policy use "none", because runner handles the scaling.
- An IAM Policy with the [correct permissions](https://gitlab.com/gitlab-org/fleeting/fleeting-plugin-aws#recommended-iam-policy).

This configuration supports:

- A capacity per instance of 1
- A use count of 1
- An idle scale of 5
- An idle time of 20 minutes
- A maximum instance count of 10

By setting the capacity and use count to both 1, each job is given a secure ephemeral instance that cannot be
affected by other jobs. As soon the job is complete the instance it was executed on is immediately deleted.

With an idle scale of 5, the runner keeps 5 whole instances
available for future demand (because the capacity per instance is 1). These instances stay for at least 20 minutes.

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

### 5 jobs per instance, unlimited uses, using AWS Autoscaling Group

Prerequisites:

- An AMI with at least `git` and GitLab Runner installed.
- An AWS Autoscaling group. For the scaling policy use "none", because runner handles the scaling.
- An IAM Policy with the [correct permissions](https://gitlab.com/gitlab-org/fleeting/fleeting-plugin-aws#recommended-iam-policy).

This configuration supports:

- A capacity per instance of 5
- An unlimited use count
- An idle scale of 5
- An idle time of 20 minutes
- A maximum instance count of 10

By setting the capacity per instance to 5 and an unlimited use count, each instance concurrently
executes 5 jobs for the lifetime of the instance.

Jobs executed in these environments should be **trusted** as there is little isolation between them and each job
can affect the performance of another.

With an idle scale of 5, 1 idle instance is created to accommodate an idle capacity of 5
(due to the capacity per instance) whenever the in use capacity is lower than 5. Idle instances
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

### 2 jobs per instance, unlimited uses, nested virtualization on EC2 Mac instances, using AWS Autoscaling group

Prerequisites:

- An MacOS AppleSilicon AMI with [nesting](https://gitlab.com/gitlab-org/fleeting/nesting) and [Tart](https://github.com/cirruslabs/tart) installed.
- The Tart VM images that runner needs to use. The VM images are specified by the `image` keyword of the job. The VM images should have at least `git` and GitLab Runner installed.
- An AWS Autoscaling group. For the scaling policy use "none", because runner handles the scaling. To set up an ASG for MacOS, see [this Amazon guide](https://aws.amazon.com/blogs/compute/implementing-autoscaling-for-ec2-mac-instances/).
- An IAM Policy with the [correct permissions](https://gitlab.com/gitlab-org/fleeting/fleeting-plugin-aws#recommended-iam-policy)

This configuration supports:

- A capacity per instance of 2
- An unlimited use count
- Nested virtualization to support isolated jobs (currently only available for **MacOS** AppleSilicon instances with [nesting](https://gitlab.com/gitlab-org/fleeting/nesting) installed)
- An idle scale of 5
- An idle time of 20 minutes
- A maximum instance count of 10

By setting the capacity per instance to 2 and an unlimited use count, each instance concurrently
executes 2 jobs for the lifetime of the instance.

Jobs executed in this environment do not need to be trusted because we're using
[nesting](https://gitlab.com/gitlab-org/fleeting/nesting) for nested virtualization of each job. This
only works on MacOS AppleSilcon instances.

With an idle scale of 2, 1 idle instance is created to accommodate an idle capacity 2
(due to the capacity per instance) whenever the in use capacity is lower than 2. Idle instances stay for at
least 24 hours. We set this to 24 hours because the AWS MacOS instance hosts have a 24 hour minimal allocation period.

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

:::TabTitle GCP

### 1 job per instance using an GCP Instance group

Prerequisites:

- A custom image with at least `git` and GitLab Runner installed.
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

With an idle scale of 5, the runner keeps 5 whole instances
available for future demand (because the capacity per instance is 1). These instances stay for at least 20 minutes.

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

### 5 jobs per instance, unlimited uses, using GCP Instance group

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

By setting the capacity per instance to 5 and an unlimited use count, each instance concurrently
executes 5 jobs for the lifetime of the instance.

Jobs executed in these environments should be **trusted** as there is little isolation between them and each job
can affect the performance of another.

With an idle scale of 5, 1 idle instance is created to accommodate an idle capacity of 5
(due to the capacity per instance) whenever the in use capacity is lower than 5. Idle instances
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

::EndTabs
