---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Autoscaling GitLab Runner on AWS EC2 **(FREE)**

One of the biggest advantages of GitLab Runner is its ability to automatically
spin up and down VMs to make sure your builds get processed immediately. It's a
great feature, and if used correctly, it can be extremely useful in situations
where you don't use your runners 24/7 and want to have a cost-effective and
scalable solution.

## Introduction

In this tutorial, we'll explore how to properly configure GitLab Runner in
AWS. The instance in AWS will serve as a Runner Manager that spawns new Docker instances on
demand. The runners on these instances are automatically created. They use the parameters
covered in this guide and do not require manual configuration after creation.

In addition, we'll make use of [Amazon's EC2 Spot instances](https://aws.amazon.com/ec2/spot/) which will
greatly reduce the costs of the GitLab Runner instances while still using quite
powerful autoscaling machines.

## Prerequisites

A familiarity with Amazon Web Services (AWS) is required as this is where most
of the configuration will take place.

We suggest a quick read through Docker machine
[`amazonec2` driver documentation](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/aws.md)
to familiarize yourself with the parameters we will set later in this article.

Your GitLab Runner is going to need to talk to your GitLab instance over the network,
and that is something you need think about when configuring any AWS security
groups or when setting up your DNS configuration.

For example, you can keep the EC2 resources segmented away from public traffic
in a different VPC to better strengthen your network security. Your environment
is likely different, so consider what works best for your situation.

### AWS security groups

Docker Machine will attempt to use a
[default security group](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/aws.md/#security-group)
with rules for port `2376` and SSH `22`, which is required for communication with the Docker
daemon. Instead of relying on Docker, you can create a security group with the
rules you need and provide that in the GitLab Runner options as we will
[see below](#the-runnersmachine-section). This way, you can customize it to your
liking ahead of time based on your networking environment.
You have to make sure that ports `2376` and `22` are accessible by the [Runner Manager instance](#prepare-the-runner-manager-instance).

### AWS credentials

You'll need an [AWS Access Key](https://docs.aws.amazon.com/general/latest/gr/managing-aws-access-keys.html)
tied to a user with permission to scale (EC2) and update the cache (via S3).
Create a new user with [policies](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-policies-for-amazon-ec2.html)
for EC2 (AmazonEC2FullAccess) and S3 (AmazonS3FullAccess). To be more secure,
you can disable console login for that user. Keep the tab open or copy paste the
security credentials in an editor as we'll use them later during the
[GitLab Runner configuration](#the-runnersmachine-section).

You can also create an [EC2 instance profile](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2_instance-profiles.html) with the required `AmazonEC2FullAccess` and `AmazonS3FullAccess` policies. Attach this instance profile to the Runner Manager EC2 instance to allow the provisioning of new EC2 instances for the jobs' execution.

## Prepare the Runner Manager instance

The first step is to install GitLab Runner in an EC2 instance that will serve
as the Runner Manager that spawns new machines. Choose a distribution that both
Docker and GitLab Runner support, like Ubuntu, Debian, CentOS, or RHEL.

This doesn't have to be a powerful machine because a Runner Manager instance doesn't run jobs itself.
For your initial configuration, you can start with a smaller instance. This machine is a dedicated host
because we need it always up and running. Therefore, it is the only host with an ongoing baseline cost.

Install the prerequisites:

1. Log in to your server
1. [Install GitLab Runner from the official GitLab repository](../../install/linux-repository.md)
1. [Install Docker](https://docs.docker.com/engine/install/#server)
1. [Install Docker Machine from the GitLab fork](https://gitlab.com/gitlab-org/ci-cd/docker-machine) (Docker has deprecated Docker Machine)

Now that the Runner is installed, it's time to register it.

## Registering the GitLab Runner

Before configuring the GitLab Runner, you need to first register it, so that
it connects with your GitLab instance:

1. [Obtain a runner token](https://docs.gitlab.com/ee/ci/runners/)
1. [Register the runner](../../register/index.md#linux)
1. When asked the executor type, enter `docker+machine`

You can now move on to the most important part, configuring the GitLab Runner.

NOTE:
If you want every user in your instance to be able to use the autoscaled runners,
register the runner as a shared one.

## Configuring the runner

Now that the runner is registered, you need to edit its configuration file and
add the required options for the AWS machine driver.

Let's first break it down to pieces.

### The global section

In the global section, you can define the limit of the jobs that can be run
concurrently across all runners (`concurrent`). This heavily depends on your
needs, like how many users GitLab Runner will accommodate, how much time your
builds take, etc. You can start with something low like `10`, and increase or
decrease its value going forward.

The `check_interval` option defines how often the runner should check GitLab
for new jobs, in seconds.

Example:

```toml
concurrent = 10
check_interval = 0
```

[Other options](../advanced-configuration.md#the-global-section)
are also available.

### The `runners` section

From the `[[runners]]` section, the most important part is the `executor` which
must be set to `docker+machine`. Most of those settings are taken care of when
you register the runner for the first time.

`limit` sets the maximum number of machines (running and idle) that this runner
will spawn. For more information, check the
[relationship between `limit`, `concurrent` and `IdleCount`](../autoscale.md#how-concurrent-limit-and-idlecount-generate-the-upper-limit-of-running-machines).

Example:

```toml
[[runners]]
  name = "gitlab-aws-autoscaler"
  url = "<URL of your GitLab instance>"
  token = "<Runner's token>"
  executor = "docker+machine"
  limit = 20
```

[Other options](../advanced-configuration.md#the-runners-section)
under `[[runners]]` are also available.

### The `runners.docker` section

In the `[runners.docker]` section you can define the default Docker image to
be used by the child runners if it's not defined in [`.gitlab-ci.yml`](https://docs.gitlab.com/ee/ci/yaml/).
By using `privileged = true`, all runners will be able to run
[Docker in Docker](https://docs.gitlab.com/ee/ci/docker/using_docker_build.html#use-docker-in-docker-executor)
which is useful if you plan to build your own Docker images via GitLab CI/CD.

Next, we use `disable_cache = true` to disable the Docker executor's inner
cache mechanism since we will use the distributed cache mode as described
in the following section.

Example:

```toml
  [runners.docker]
    image = "alpine"
    privileged = true
    disable_cache = true
```

[Other options](../advanced-configuration.md#the-runnersdocker-section)
under `[runners.docker]` are also available.

### The `runners.cache` section

To speed up your jobs, GitLab Runner provides a cache mechanism where selected
directories and/or files are saved and shared between subsequent jobs.
While not required for this setup, it is recommended to use the distributed cache
mechanism that GitLab Runner provides. Since new instances will be created on
demand, it is essential to have a common place where the cache is stored.

In the following example, we use Amazon S3:

```toml
  [runners.cache]
    Type = "s3"
    Shared = true
    [runners.cache.s3]
      ServerAddress = "s3.amazonaws.com"
      AccessKey = "<your AWS Access Key ID>"
      SecretKey = "<your AWS Secret Access Key>"
      BucketName = "<the bucket where your cache should be kept>"
      BucketLocation = "us-east-1"
```

Here's some more information to further explore the cache mechanism:

- [Reference for `runners.cache`](../advanced-configuration.md#the-runnerscache-section)
- [Reference for `runners.cache.s3`](../advanced-configuration.md#the-runnerscaches3-section)
- [Deploying and using a cache server for GitLab Runner](../autoscale.md#distributed-runners-caching)
- [How cache works](https://docs.gitlab.com/ee/ci/yaml/#cache)

### The `runners.machine` section

This is the most important part of the configuration and it's the one that
tells GitLab Runner how and when to spawn new or remove old Docker Machine
instances.

We will focus on the AWS machine options, for the rest of the settings read
about the:

- [Autoscaling algorithm and the parameters it's based on](../autoscale.md#autoscaling-algorithm-and-parameters) - depends on the needs of your organization
- [Autoscaling periods](../autoscale.md#autoscaling-periods-configuration) - useful when there are regular time periods in your organization when no work is done, for example weekends

Here's an example of the `runners.machine` section:

```toml
  [runners.machine]
    IdleCount = 1
    IdleTime = 1800
    MaxBuilds = 10
    MachineDriver = "amazonec2"
    MachineName = "gitlab-docker-machine-%s"
    MachineOptions = [
      "amazonec2-access-key=XXXX",
      "amazonec2-secret-key=XXXX",
      "amazonec2-region=us-central-1",
      "amazonec2-vpc-id=vpc-xxxxx",
      "amazonec2-subnet-id=subnet-xxxxx",
      "amazonec2-zone=x",
      "amazonec2-use-private-address=true",
      "amazonec2-tags=runner-manager-name,gitlab-aws-autoscaler,gitlab,true,gitlab-runner-autoscale,true",
      "amazonec2-security-group=xxxxx",
      "amazonec2-instance-type=m4.2xlarge",
    ]
    [[runners.machine.autoscaling]]
      Periods = ["* * 9-17 * * mon-fri *"]
      IdleCount = 50
      IdleTime = 3600
      Timezone = "UTC"
    [[runners.machine.autoscaling]]
      Periods = ["* * * * * sat,sun *"]
      IdleCount = 5
      IdleTime = 60
      Timezone = "UTC"
```

The Docker Machine driver is set to `amazonec2` and the machine name has a
standard prefix followed by `%s` (required) that is replaced by the ID of the
child runner: `gitlab-docker-machine-%s`.

Now, depending on your AWS infrastructure, there are many options you can set up
under `MachineOptions`. Below you can see the most common ones.

| Machine option | Description |
| -------------- | ----------- |
| `amazonec2-access-key=XXXX` | The AWS access key of the user that has permissions to create EC2 instances, see [AWS credentials](#aws-credentials). |
| `amazonec2-secret-key=XXXX` | The AWS secret key of the user that has permissions to create EC2 instances, see [AWS credentials](#aws-credentials). |
| `amazonec2-region=eu-central-1` | The region to use when launching the instance. You can omit this entirely and the default `us-east-1` will be used. |
| `amazonec2-vpc-id=vpc-xxxxx` | Your [VPC ID](https://github.com/docker/docs/blob/173d3c65f8e7df2a8c0323594419c18086fc3a30/machine/drivers/aws.md#vpc-id) to launch the instance in. |
| `amazonec2-subnet-id=subnet-xxxx` | The AWS VPC subnet ID. |
| `amazonec2-zone=x` | If not specified, the [availability zone is `a`](https://github.com/docker/docs/blob/173d3c65f8e7df2a8c0323594419c18086fc3a30/machine/drivers/aws.md#environment-variables-and-default-values), it needs to be set to the same availability zone as the specified subnet, for example when the zone is `eu-west-1b` it has to be `amazonec2-zone=b` |
| `amazonec2-use-private-address=true` | Use the private IP address of Docker Machines, but still create a public IP address. Useful to keep the traffic internal and avoid extra costs.|
| `amazonec2-tags=runner-manager-name,gitlab-aws-autoscaler,gitlab,true,gitlab-runner-autoscale,true` | AWS extra tag key-value pairs, useful to identify the instances on the AWS console. The "Name" [tag](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/Using_Tags.html) is set to the machine name by default. We set the "runner-manager-name" to match the runner name set in `[[runners]]`, so that we can filter all the EC2 instances created by a specific manager setup. |
| `amazonec2-security-group=xxxx` | AWS VPC security group name, not the security group ID. See [AWS security groups](#aws-security-groups). |
| `amazonec2-instance-type=m4.2xlarge` | The instance type that the child runners will run on. |
| `amazonec2-ssh-user=xxxx` | The user that will have SSH access to the instance. |
| `amazonec2-iam-instance-profile=xxxx_runner_machine_inst_profile_name` | The IAM instance profile to use for the runner machine. |
| `amazonec2-ami=xxxx_runner_machine_ami_id` | The GitLab Runner AMI ID for a specific image. |
| `amazonec2-request-spot-instance=true` | Use spare EC2 capacity that is available for less than the on-demand price. |
| `amazonec2-spot-price=xxxx_runner_machine_spot_price=x.xx` | Spot instance bid price (in US dollars). Requires the `--amazonec2-request-spot-instance flag` set to `true`. If you omit the `amazonec2-spot-price`, Docker Machine sets the maximum price to a default value of `$0.50` per hour. |
| `amazonec2-security-group-readonly=true` | Set the security group to read-only.|
| `amazonec2-userdata=xxxx_runner_machine_userdata_path` | Specify the runner machine `userdata` path. |
| `amazonec2-root-size=XX` | The root disk size of the instance (in GB). |

Notes:

- Under `MachineOptions` you can add anything that the
  AWS Docker Machine driver [supports](https://github.com/docker/docs/blob/173d3c65f8e7df2a8c0323594419c18086fc3a30/machine/drivers/aws.md#options). You are highly
  encouraged to read Docker's docs as your infrastructure setup may warrant
  different options to be applied.
- The child instances will use by default Ubuntu 16.04 unless you choose a
  different AMI ID by setting `amazonec2-ami`. Set only
  [supported base operating systems for Docker Machine](https://github.com/docker/docs/blob/173d3c65f8e7df2a8c0323594419c18086fc3a30/machine/drivers/os-base.md).
- If you specify `amazonec2-private-address-only=true` as one of the machine
  options, your EC2 instance won't get assigned a public IP. This is ok if your
  VPC is configured correctly with an Internet Gateway (IGW) and routing is fine,
  but it’s something to consider if you've got a more complex configuration. Read
  more about [VPC connectivity](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/aws.md#vpc-connectivity).

[Other options](../advanced-configuration.md#the-runnersmachine-section)
under `[runners.machine]` are also available.

### Getting it all together

Here's the full example of `/etc/gitlab-runner/config.toml`:

```toml
concurrent = 10
check_interval = 0

[[runners]]
  name = "gitlab-aws-autoscaler"
  url = "<URL of your GitLab instance>"
  token = "<runner's token>"
  executor = "docker+machine"
  limit = 20
  [runners.docker]
    image = "alpine"
    privileged = true
    disable_cache = true
  [runners.cache]
    Type = "s3"
    Shared = true
    [runners.cache.s3]
      ServerAddress = "s3.amazonaws.com"
      AccessKey = "<your AWS Access Key ID>"
      SecretKey = "<your AWS Secret Access Key>"
      BucketName = "<the bucket where your cache should be kept>"
      BucketLocation = "us-east-1"
  [runners.machine]
    IdleCount = 1
    IdleTime = 1800
    MaxBuilds = 100
    MachineDriver = "amazonec2"
    MachineName = "gitlab-docker-machine-%s"
    MachineOptions = [
      "amazonec2-access-key=XXXX",
      "amazonec2-secret-key=XXXX",
      "amazonec2-region=us-central-1",
      "amazonec2-vpc-id=vpc-xxxxx",
      "amazonec2-subnet-id=subnet-xxxxx",
      "amazonec2-use-private-address=true",
      "amazonec2-tags=runner-manager-name,gitlab-aws-autoscaler,gitlab,true,gitlab-runner-autoscale,true",
      "amazonec2-security-group=XXXX",
      "amazonec2-instance-type=m4.2xlarge",
    ]
    [[runners.machine.autoscaling]]
      Periods = ["* * 9-17 * * mon-fri *"]
      IdleCount = 50
      IdleTime = 3600
      Timezone = "UTC"
    [[runners.machine.autoscaling]]
      Periods = ["* * * * * sat,sun *"]
      IdleCount = 5
      IdleTime = 60
      Timezone = "UTC"
```

## Cutting down costs with Amazon EC2 Spot instances

As [described by](https://aws.amazon.com/ec2/spot/) Amazon:

>
Amazon EC2 Spot instances allow you to bid on spare Amazon EC2 computing capacity.
Since Spot instances are often available at a discount compared to On-Demand
pricing, you can significantly reduce the cost of running your applications,
grow your application’s compute capacity and throughput for the same budget,
and enable new types of cloud computing applications.

In addition to the [`runners.machine`](#the-runnersmachine-section) options
you picked above, in `/etc/gitlab-runner/config.toml` under the `MachineOptions`
section, add the following:

```toml
    MachineOptions = [
      "amazonec2-request-spot-instance=true",
      "amazonec2-spot-price=",
    ]
```

In this configuration with an empty `amazonec2-spot-price`, AWS sets your
bidding price for a Spot instance to the default On-Demand price of that
instance class. If you omit the `amazonec2-spot-price` completely, Docker
Machine will set the maximum price to a
[default value of $0.50 per hour](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/aws.md#environment-variables-and-default-values).

You may further customize your Spot instance request:

```toml
    MachineOptions = [
      "amazonec2-request-spot-instance=true",
      "amazonec2-spot-price=0.03",
      "amazonec2-block-duration-minutes=60"
    ]
```

With this configuration, Docker Machines are created using Spot instances with a
maximum Spot request price of $0.03 per hour and the duration of the Spot instance
is capped at 60 minutes. The `0.03` number mentioned above is just an example, so
be sure to check on the current pricing based on the region you picked.

To learn more about Amazon EC2 Spot instances, visit the following links:

- <https://aws.amazon.com/ec2/spot/>
- <https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/spot-requests.html>
- <https://aws.amazon.com/ec2/spot/getting-started/>

### Caveats of Spot instances

While Spot instances is a great way to use unused resources and minimize the
costs of your infrastructure, you must be aware of the implications.

Running CI jobs on Spot instances may increase the failure rates because of the
Spot instances pricing model. If the maximum Spot price you specify exceeds the
current Spot price you will not get the capacity requested. Spot pricing is
revised on an hourly basis. Any existing Spot instances that have a maximum price
below the revised Spot instance price will be terminated within two minutes and
all jobs on Spot hosts will fail.

As a consequence, the auto-scale Runner would fail to create new machines while
it will continue to request new instances. This eventually will make 60 requests
and then AWS won't accept any more. Then once the Spot price is acceptable, you
are locked out for a bit because the call amount limit is exceeded.

If you encounter that case, you can use the following command in the Runner Manager
machine to see the Docker Machines state:

```shell
docker-machine ls -q --filter state=Error --format "{{.NAME}}"
```

NOTE:
There are some issues regarding making GitLab Runner gracefully handle Spot
price changes, and there are reports of `docker-machine` attempting to
continually remove a Docker Machine. GitLab has provided patches for both cases
in the upstream project. For more information, see issues
[#2771](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/2771) and
[#2772](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/2772).

## Conclusion

In this guide we learned how to install and configure a GitLab Runner in
autoscale mode on AWS.

Using the autoscale feature of GitLab Runner can save you both time and money.
Using the Spot instances that AWS provides can save you even more, but you must
be aware of the implications. As long as your bid is high enough, there shouldn't
be an issue.

You can read the following use cases from which this tutorial was (heavily)
influenced:

- [HumanGeo switched from Jenkins to GitLab](https://about.gitlab.com/blog/2017/11/14/humangeo-switches-jenkins-gitlab-ci/)
- [Substrakt Health - Autoscale GitLab CI/CD runners and save 90% on EC2 costs](https://about.gitlab.com/blog/2017/11/23/autoscale-ci-runners/)
