---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: GitLab Runner instance group autoscaler
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

GitLab Runner instance group autoscaler is the successor to the autoscaling technology based on Docker Machine. The components of the GitLab Runner instance group autoscaling solution are:

- Taskscaler: Manages the autoscaling logic, bookkeeping, and creates fleets for runner instances that use cloud provider autoscaling groups of instances.
- [Fleeting](../fleet_scaling/fleeting.md): An abstraction for cloud provider virtual machines.
- Cloud provider plugin: Handles the API calls to the target cloud platform and is implemented using a plugin development framework.

Instance group autoscaling in GitLab Runner works as follows:

1. The runner manager continuously polls GitLab jobs.
1. In response, GitLab sends job payloads to the runner manager.
1. The runner manager interacts with the public cloud infrastructure to create a new instance to execute jobs.
1. The runner manager distributes these jobs to the available runners in the autoscaling pool.

![Overview of GitLab Next Runner Autoscaling](img/next-runner-autoscaling-overview.png)

## Configure the runner manager

You must [configure the runner manager](../runner_autoscale/_index.md#configure-the-runner-manager) to use the GitLab Runner instance group autoscaler.

1. Create an instance to host the runner manager. This **must not** be a spot instance (AWS), or spot virtual machine (GCP or Azure).
1. [Install GitLab Runner](../install/linux-repository.md) on the instance.
1. Add the cloud provider credentials to the runner manager host machine.

   {{< alert type="note" >}}

   You can host the runner manager in a container.
   For GitLab.com and GitLab Dedicated [hosted runners](https://docs.gitlab.com/ci/runners/), the runner manager is hosted on a virtual machine instance.

   {{< /alert >}}

### Example credentials configuration for GitLab Runner instance group autoscaler

You can use an [AWS Identity and Access Management](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2_instance-profiles.html)
(IAM) instance profile for the runner manager in the AWS environment.
If you do not want to host the runner manager in AWS, you can use a credentials file.

For example:

``` toml
## credentials_file

[default]
aws_access_key_id=__REDACTED__
aws_secret_access_key=__REDACTED__
```

The credentials file is optional.

## Supported public cloud instances

The following autoscaling options are supported for public cloud compute instances:

- Amazon Web Services EC2 instances
- Google Compute Engine
- Microsoft Azure Virtual Machines

These cloud instances are supported by the GitLab Runner Docker Machine autoscaler as well.

## Supported platforms

| Executor                   | Linux                                | macOS                                | Windows                              |
|----------------------------|--------------------------------------|--------------------------------------|--------------------------------------|
| Instance executor          | {{< icon name="check-circle" >}} Yes | {{< icon name="check-circle" >}} Yes | {{< icon name="check-circle" >}} Yes |
| Docker Autoscaler executor | {{< icon name="check-circle" >}} Yes | {{< icon name="dotted-circle" >}} No | {{< icon name="check-circle" >}} Yes |

## Autoscaling algorithm and parameters

The autoscaling algorithm is based on these parameters:

- `IdleCount`
- `IdleCountMin`
- `IdleScaleFactor`
- `IdleTime`
- `MaxGrowthRate`
- `limit`

Any machine not running a job is considered to be idle. When GitLab Runner is in autoscale mode,
it monitors all machines and ensures that there is always an `IdleCount` of idle machines.

If there is an insufficient number of idle machines, GitLab Runner
starts provisioning new machines, subject to the `MaxGrowthRate` limit.
Requests for machines above the `MaxGrowthRate` value are put on hold
until the number of machines being created falls below `MaxGrowthRate`.

At the same time, GitLab Runner is checking the duration of the idle state of
each machine. If the time exceeds the `IdleTime` value, the machine is
automatically removed.

### Example configuration

Consider a GitLab Runner configured with the following autoscale parameters:

```toml
[[runners]]
  limit = 10
  # (...)
  executor = "docker+machine"
  [runners.machine]
    MaxGrowthRate = 1
    IdleCount = 2
    IdleTime = 1800
    # (...)
```

In the beginning, when no jobs are queued, GitLab Runner starts two machines
(`IdleCount = 2`), and sets them in idle state. Also, the `IdleTime` is set
to 30 minutes (`IdleTime = 1800`).

Now, assume that five jobs are queued in GitLab CI/CD. The first two jobs are
sent to the idle machines of which we have two. GitLab Runner starts new machines as it now notices that
the number of idle is less than `IdleCount` (`0 < 2`). These machines are provisioned sequentially,
to prevent exceeding the `MaxGrowthRate`.

The remaining three jobs are assigned to the first machine that is ready. As an
optimization, this can be a machine that was busy, but has now completed its job,
or it can be a newly provisioned machine. For this example,
assume that provisioning is fast and the new machines are ready
before any earlier jobs complete.

We now have one idle machine, so GitLab Runner starts one new machine to
satisfy `IdleCount`. Because there are no new jobs in queue, those two
machines stay in idle state and GitLab Runner is satisfied.

**What happened:**

In the example, there are two machines waiting in idle state for new jobs. After the five jobs
are queued, new machines are created. So, in total there are seven machines:
five running jobs and two in idle state waiting for the next
jobs.

The autoscaling algorithm works the same way. GitLab Runner creates a new
idle machine for each machine used for the job execution, until `IdleCount`
is satisfied. Machines are created up to the number defined by the
`limit` parameter. When GitLab Runner detects that this `limit` has been reached,
it stops autoscaling. The new jobs must wait in the job queue until machines
start returning to idle state.

In the above example, two idle machines are always available. The `IdleTime` parameter
applies only when the number exceeds `IdleCount`. At this point, GitLab Runner reduces
the number of machines to match `IdleCount`.

**Scaling down:**

After the job finishes, the machine is set to idle state and waits
for new jobs to be executed. If no new jobs appear in the queue,
idle machines are removed after the time specified by `IdleTime`.
In this example, all machines are removed after 30 minutes of inactivity
(measured from when each machine's last job execution ended). GitLab
Runner maintains an `IdleCount` of idle machines running, just like
at the beginning of the example.

The autoscaling algorithm works as follows:

1. GitLab Runner starts.
1. GitLab Runner creates two idle machines.
1. GitLab Runner picks one job.
1. GitLab Runner creates one more machine to maintain two idle machines.
1. The picked job finishes, resulting in three idle machines.
1. When one of the three idle machines exceeds `IdleTime` from the time after it picked the last job, it is removed.
1. GitLab Runner always maintains at least two idle machines for quick job processing.

The following chart illustrates the states of machines and builds (jobs)
in time:

![Autoscale state chart](img/autoscale-state-chart.png)
