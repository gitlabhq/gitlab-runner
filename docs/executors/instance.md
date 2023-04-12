---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Instance executor (Alpha)

> The instance executor autoscaler feature (alpha) was introduced in GitLab Runner 15.10.0.

The instance executor is an autoscale-enabled executor that creates instances on-demand to accommodate the expected volume of CI jobs that the runner manager will process.

You can use the instance executor when jobs need full access to the host instance, operating system, and attached devices. The instance executor can also be configured to accommodate single and multi-tenant jobs with various levels of isolation and security.

## Nested Virtualization

The instance executor supports nested virtualization, through [nesting](https://gitlab.com/gitlab-org/fleeting/nesting). Nesting is GitLab developed daemon that enables creating and deleting pre-configured Vritual Machines on a host system intended for isolated and short-lived workloads such as CI jobs. Nesting is currently only supported on Apple macOS Apple Silicon instances.

## Preparing the environment

### Step 1: Install a fleeting plugin

To get started with the Instance executor, select a `fleeting` plugin that targets the public cloud platform you want to autoscale on. An Alpha version of the [AWS fleeting plugin](https://gitlab.com/gitlab-org/fleeting/fleeting-plugin-aws) is currently available. We also plan to release and maintain GCP, and Azure fleeting plugins. However, the goal of the fleeting plugin system is to enable community members to build and contribute plugins for other cloud providers.

To install the AWS plugin, check the
[release page](https://gitlab.com/gitlab-org/fleeting/fleeting-plugin-aws/-/releases) and download the binary for your
host platform. The `fleeting` plugin binaries need to be discoverable via the `PATH` environment variable.

### Step 2: Create an Amazon Machine Image

Create an Amazon Machine Image (AMI) that includes the following:

- Any dependencies required by the CI jobs you plan to run
- Git
- GitLab Runner

## Autoscaler Configuration

The autoscaler configuration provides most of the functionality for the instance executor. Administrators can use the autoscaler configuration to specify settings like concurrency, the amount of times an instance can be use, and when to create idle capacity.

- [Instance Executor configuration](../configuration/advanced-configuration.md#the-runnersinstance-section-alpha)
- [Autoscaler configuration](../configuration/advanced-configuration.md#the-runnersautoscaler-section)

## Examples

### 1 job per instance using an AWS Autoscaling Group

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

With an idle scale of 5, the runner will try to keep 5 whole instances (because the capacity per instance is 1)
available for future demand. These instances will stay for at least 20 minutes.

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

By setting the capacity per instance to 5 and an unlimited use count, each instance will be able to concurrently
execute 5 jobs for the lifetime of the instance.

Jobs executed in these environments should be **trusted** as there is little isolation between them and each job
can affect the performance of another.

With an idle scale of 5, 1 idle instance will be created to accomodate an idle capacity of 5
(due to the capacity per instance) whenever the in use capacity is lower than 5. Idle instances
will stay for at least 20 minutes.

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

### 2 jobs per instance, unlimited uses, nested virtualization on EC2 Mac instances, using AWS Autoscaling Group

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

By setting the capacity per instance to 2 and an unlimited use count, each instance will be able to concurrently
execute 2 jobs for the lifetime of the instance.

Jobs executed in this environment do not need to be trusted because we're using
[nesting](https://gitlab.com/gitlab-org/fleeting/nesting) for nested virtualization of each job. This currently
only works on MacOS AppleSilcon instances.

With an idle scale of 2, 1 idle instance will be created to accomodate an idle capacity 2
(due to the capacity per instance) whenever the in use capacity is lower than 2. Idle instances will stay for at
least 24 hours. We set this to 24 hours because AWS's MacOS instance hosts have a 24 hour minimal allocation period.

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
