---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: GitLab Runner Autoscaling
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

You can use GitLab Runner autoscaling to automatically scale the runner on public cloud instances.
When you configure a runner to use autoscaler, you can handle increased CI/CD job loads by
running multiple jobs simultaneously on your cloud infrastructure.

In addition to the autoscaling options for public cloud instances, you can use
the following container orchestration solutions for hosting and scaling a runner fleet.

- Red Hat OpenShift Kubernetes clusters
- Kubernetes clusters: AWS EKS, Azure, on-premise
- Amazon Elastic Container Services clusters on AWS Fargate

## GitLab Runner Autoscaler

GitLab Runner Autoscaler is the successor to the autoscaling technology based on Docker Machine. The components of the GitLab Runner Autoscaler are:

- Taskscaler: Manages the autoscaling logic, bookkeeping, and creates fleets for runner instances that use cloud provider autoscaling groups of instances.
- Fleeting: An abstraction for cloud provider virtual machines.
- Cloud provider plugin: Handles the API calls to the target cloud platform and is implemented using a plugin development framework.

![Overview of GitLab Next Runner Autoscaling](img/next-runner-autoscaling-overview.png)

### GitLab Runner Autoscaler supported public cloud instances

The following autoscaling options are supported for public cloud compute instances.

|                   | Next Runner Autoscaler                 | GitLab Runner Docker Machine Autoscaler                |
|----------------------------|------------------------|------------------------|
| Amazon Web Services EC2 instances         | {{< icon name="check-circle" >}} Yes | {{< icon name="check-circle" >}} Yes |
| Google Compute Engine | {{< icon name="check-circle" >}} Yes | {{< icon name="check-circle" >}} Yes |
|Microsoft Azure Virtual Machines|{{< icon name="check-circle" >}} Yes|{{< icon name="check-circle" >}} Yes|

### GitLab Runner Autoscaler supported platforms

| Executor                   | Linux                  | macOS                  | Windows                |
|----------------------------|------------------------|------------------------|------------------------|
| Instance executor          | {{< icon name="check-circle" >}} Yes | {{< icon name="check-circle" >}} Yes | {{< icon name="check-circle" >}} Yes |
| Docker Autoscaler executor | {{< icon name="check-circle" >}} Yes | {{< icon name="dotted-circle" >}} No | {{< icon name="check-circle" >}} Yes |

## Configure the runner manager

You must configure the runner manager to use GitLab Runner Autoscaling, both the Docker Machine Autoscaling solution and the GitLab Runner Autoscaler.

The runner manager is a type of runner that creates multiple runners for
autoscaling. It continuously polls GitLab for jobs and interacts with the
public cloud infrastructure to create a new instance to execute jobs. The
runner manager must run on a host machine that has GitLab Runner installed.
Choose a distribution that
Docker and GitLab Runner supports, like Ubuntu, Debian, CentOS, or RHEL.

1. Create an instance to host the runner manager. This **must not** be a spot instance (AWS), or spot virtual machine (GCP, Azure).
1. [Install GitLab Runner](../install/linux-repository.md) on the instance.
1. Add the cloud provider credentials to the Runner Manager host machine.

{{< alert type="note" >}}

You can host the runner manager in a container.
For [GitLab-hosted runners](https://docs.gitlab.com/ci/runners/), the runner manager is hosted on a virtual machine instance.

{{< /alert >}}

### Example credentials configuration for the GitLab Runner Autoscaler

``` toml
## credentials_file

[default]
aws_access_key_id=__REDACTED__
aws_secret_access_key=__REDACTED__
```

### Example credentials configuration for GitLab Runner Docker Machine Autoscaling

This snippet is in the runners.machine section of the `config.toml` file.

``` toml
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
      "amazonec2-security-group=xxxxx",
    ]
```

{{< alert type="note" >}}

The credentials file is optional.
You can use an [AWS Identity and Access Management](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2_instance-profiles.html)
(IAM) instance profile for the runner manager in the AWS environment.
If you do not want to host the runner manager in AWS, you can use the credentials file.

{{< /alert >}}

## Implement a fault-tolerant design

Start with at least two runner managers that use the same runner tags to create a
fault-tolerant design and prevent runner manager host failures.

For example, on GitLab.com, multiple runner managers are configured for
[hosted runners on Linux](https://docs.gitlab.com/ci/runners/hosted_runners/linux/).
Each runner manager has the tag `saas-linux-small-amd64`.

Use observability and runner fleet metrics when you adjust autoscaling parameters to balance
efficiency and performance for your organization's CI/CD workloads.

## Configure runner autoscaling executors

After you configure the runner manager, configure the executors specific to autoscaling:

- [Instance Executor](../executors/instance.md)
- [Docker Autoscaling Executor](../executors/docker_autoscaler.md)
- [Docker Machine Executor](../executors/docker_machine.md)

{{< alert type="note" >}}

You should use the Instance and Docker Autoscaling executors, as these comprise the
technology that replaces the Docker Machine autoscaler.

{{< /alert >}}
