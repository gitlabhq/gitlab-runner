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

![Overview of GitLab Next Runner Autoscaling](img/next-runner-autoscaling-overview.png)

## GitLab Runner instance group autoscaler supported public cloud instances

The following autoscaling options are supported for public cloud compute instances.

|                                   | GitLab Runner instance group autoscaler | GitLab Runner Docker Machine autoscaler |
|-----------------------------------|-----------------------------------------|-----------------------------------------|
| Amazon Web Services EC2 instances | {{< icon name="check-circle" >}} Yes    | {{< icon name="check-circle" >}} Yes    |
| Google Compute Engine             | {{< icon name="check-circle" >}} Yes    | {{< icon name="check-circle" >}} Yes    |
| Microsoft Azure Virtual Machines  | {{< icon name="check-circle" >}} Yes    | {{< icon name="check-circle" >}} Yes    |

## GitLab Runner instance group autoscaler supported platforms

| Executor                   | Linux                                | macOS                                | Windows                              |
|----------------------------|--------------------------------------|--------------------------------------|--------------------------------------|
| Instance executor          | {{< icon name="check-circle" >}} Yes | {{< icon name="check-circle" >}} Yes | {{< icon name="check-circle" >}} Yes |
| Docker Autoscaler executor | {{< icon name="check-circle" >}} Yes | {{< icon name="dotted-circle" >}} No | {{< icon name="check-circle" >}} Yes |

## Configure the runner manager

You must [configure the runner manager](../runner_autoscale/_index.md#configure-the-runner-manager) to use the GitLab Runner instance group autoscaler.

1. Create an instance to host the runner manager. This **must not** be a spot instance (AWS), or spot virtual machine (GCP or Azure).
1. [Install GitLab Runner](../install/linux-repository.md) on the instance.
1. Add the cloud provider credentials to the Runner Manager host machine.

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

---

**Example:**
Let's suppose, that we have configured GitLab Runner with the following
autoscale parameters:

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

At the beginning, when no jobs are queued, GitLab Runner starts two machines
(`IdleCount = 2`), and sets them in idle state. Notice that we have also set
`IdleTime` to 30 minutes (`IdleTime = 1800`).

Now, let's assume that 5 jobs are queued in GitLab CI. The first 2 jobs are
sent to the idle machines of which we have two. GitLab Runner now notices that
the number of idle is less than `IdleCount` (`0 < 2`), so it starts new
machines. These machines are provisioned sequentially, to prevent exceeding the
`MaxGrowthRate`.

The remaining 3 jobs are assigned to the first machine that is ready. As an
optimization, this can be a machine that was busy, but has now completed its job,
or it can be a newly provisioned machine. For this example,
assume that provisioning is fast and the new machines are ready
before any earlier jobs complete.

We now have 1 idle machine, so GitLab Runner starts another 1 new machine to
satisfy `IdleCount`. Because there are no new jobs in queue, those two
machines stay in idle state and GitLab Runner is satisfied.

---

**What happened:**
We had 2 machines, waiting in idle state for new jobs. After the 5 jobs
where queued, new machines were created, so in total we had 7 machines. Five of
them were running jobs, and 2 were in idle state, waiting for the next
jobs.

The algorithm still works the same way; GitLab Runner creates a new
idle machine for each machine used for the job execution until `IdleCount`
is satisfied. Those machines are created up to the number defined by
`limit` parameter. If GitLab Runner notices that there is a `limit` number of
total created machines, it stops autoscaling. The new jobs must
wait in the job queue until machines start returning to idle state.

In the above example we always have two idle machines. The `IdleTime`
applies only when we are over the `IdleCount`. Then we try to reduce the number
of machines to `IdleCount`.

---

**Scaling down:**
After the job is finished, the machine is set to idle state and is waiting
for the next jobs to be executed. Let's suppose that we have no new jobs in
the queue. After the time designated by `IdleTime` passes, the idle machines
are removed. In this example, after 30 minutes, all machines are removed
(each machine after 30 minutes from when last job execution ended). GitLab
Runner starts to keep an `IdleCount` of idle machines running, just like
at the beginning of the example.

---

So, to sum up:

1. We start GitLab Runner
1. GitLab Runner creates 2 idle machines
1. GitLab Runner picks one job
1. GitLab Runner creates one more machine to fulfill the strong requirement of always
   having the two idle machines
1. Job finishes, we have 3 idle machines
1. When one of the three idle machines goes over `IdleTime` from the time when
   last time it picked the job it is removed
1. GitLab Runner always has at least 2 idle machines waiting for fast
   picking of the jobs

Below you can see a comparison chart of jobs statuses and machines statuses
in time:

![Autoscale state chart](img/autoscale-state-chart.png)
