---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/engineering/ux/technical-writing/#assignments
---

# Runners autoscale configuration

> The autoscale feature was introduced in GitLab Runner 1.1.0.

Autoscale provides the ability to utilize resources in a more elastic and
dynamic way.

GitLab Runner can autoscale, so that your infrastructure contains only as
many build instances as are necessary at any time. If you configure GitLab Runner to
only use autoscale, the system on which GitLab Runner is installed acts as a
bastion for all the machines it creates. This machine is referred to as a "Runner Manager."

## Overview

When this feature is enabled and configured properly, jobs are executed on
machines created _on demand_. Those machines, after the job is finished, can
wait to run the next jobs or can be removed after the configured `IdleTime`.
In case of many cloud providers this helps to utilize the cost of already used
instances.

Below, you can see a real life example of the GitLab Runner autoscale feature, tested
on GitLab.com for the [GitLab Community Edition](https://gitlab.com/gitlab-org/gitlab-foss) project:

![Real life example of autoscaling](img/autoscale-example.png)

Each machine on the chart is an independent cloud instance, running jobs
inside of Docker containers.

## System requirements

Before configuring autoscale, you must:

- [Prepare your own environment](../executors/docker_machine.md#preparing-the-environment).
- Optionally use a [forked version](../executors/docker_machine.md#forked-version-of-docker-machine) of Docker machine supplied by GitLab, which has some additional fixes.

## Supported cloud providers

The autoscale mechanism is based on [Docker Machine](https://docs.docker.com/machine/overview/).
All supported virtualization/cloud provider parameters, are available at the
[Docker Machine drivers documentation](https://docs.docker.com/machine/drivers/).

## Runner configuration

This section describes the significant autoscale parameters.
For more configurations details read the
[advanced configuration](advanced-configuration.md).

### Runner global options

| Parameter    | Value   | Description |
|--------------|---------|-------------|
| `concurrent` | integer | Limits how many jobs globally can be run concurrently. This is the most upper limit of number of jobs using _all_ defined runners, local and autoscale. Together with `limit` (from [`[[runners]]` section](#runners-options)) and `IdleCount` (from [`[runners.machine]` section](advanced-configuration.md#the-runnersmachine-section)) it affects the upper limit of created machines. |

### `[[runners]]` options

| Parameter  | Value            | Description |
|------------|------------------|-------------|
| `executor` | string           | To use the autoscale feature, `executor` must be set to `docker+machine` or `docker-ssh+machine`. |
| `limit`    | integer          | Limits how many jobs can be handled concurrently by this specific token. 0 simply means don't limit. For autoscale it's the upper limit of machines created by this provider (in conjunction with `concurrent` and `IdleCount`). |

### `[runners.machine]` options

Configuration parameters details can be found
in [GitLab Runner - Advanced Configuration - The `[runners.machine]` section](advanced-configuration.md#the-runnersmachine-section).

### `[runners.cache]` options

Configuration parameters details can be found
in [GitLab Runner - Advanced Configuration - The `[runners.cache]` section](advanced-configuration.md#the-runnerscache-section)

### Additional configuration information

There is also a special mode, when you set `IdleCount = 0`. In this mode,
machines are **always** created **on-demand** before each job (if there is no
available machine in _Idle_ state). After the job is finished, the autoscaling
algorithm works
[the same as it is described below](#autoscaling-algorithm-and-parameters).
The machine is waiting for the next jobs, and if no one is executed, after
the `IdleTime` period, the machine is removed. If there are no jobs, there
are no machines in _Idle_ state.

## Autoscaling algorithm and parameters

The autoscaling algorithm is based on these parameters:

- `IdleCount`
- `IdleTime`
- `MaxGrowthRate`
- `limit`

We say that each machine that does not run a job is in _Idle_ state. When
GitLab Runner is in autoscale mode, it monitors all machines and ensures that
there is always an `IdleCount` of machines in _Idle_ state.

If there is an insufficient number of _Idle_ machines, GitLab Runner
starts provisioning new machines, subject to the `MaxGrowthRate` limit.
Requests for machines above the `MaxGrowthRate` value are put on hold
until the number of machines being created falls below `MaxGrowthRate`.

At the same time, GitLab Runner is checking the duration of the _Idle_ state of
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
(`IdleCount = 2`), and sets them in _Idle_ state. Notice that we have also set
`IdleTime` to 30 minutes (`IdleTime = 1800`).

Now, let's assume that 5 jobs are queued in GitLab CI. The first 2 jobs are
sent to the _Idle_ machines of which we have two. GitLab Runner now notices that
the number of _Idle_ is less than `IdleCount` (`0 < 2`), so it starts new
machines. These machines are provisioned sequentially, to prevent exceeding the
`MaxGrowthRate`.

The remaining 3 jobs are assigned to the first machine that is ready. As an
optimization, this can be a machine that was busy, but has now completed its job,
or it can be a newly provisioned machine. For the sake of this example, let us
assume that provisioning is fast, and the provisioning of new machines completed
before any of the earlier jobs completed.

We now have 1 _Idle_ machine, so GitLab Runner starts another 1 new machine to
satisfy `IdleCount`. Because there are no new jobs in queue, those two
machines stay in _Idle_ state and GitLab Runner is satisfied.

---

**This is what happened:**
We had 2 machines, waiting in _Idle_ state for new jobs. After the 5 jobs
where queued, new machines were created, so in total we had 7 machines. Five of
them were running jobs, and 2 were in _Idle_ state, waiting for the next
jobs.

The algorithm still works the same way; GitLab Runner creates a new
_Idle_ machine for each machine used for the job execution until `IdleCount`
is satisfied. Those machines are created up to the number defined by
`limit` parameter. If GitLab Runner notices that there is a `limit` number of
total created machines, it stops autoscaling, and new jobs must
wait in the job queue until machines start returning to _Idle_ state.

In the above example we always have two idle machines. The `IdleTime`
applies only when we are over the `IdleCount`. Then we try to reduce the number
of machines to `IdleCount`.

---

**Scaling down:**
After the job is finished, the machine is set to _Idle_ state and is waiting
for the next jobs to be executed. Let's suppose that we have no new jobs in
the queue. After the time designated by `IdleTime` passes, the _Idle_ machines
are removed. In our example, after 30 minutes, all machines are removed
(each machine after 30 minutes from when last job execution ended) and GitLab
Runner starts to keep an `IdleCount` of _Idle_ machines running, just like
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

## How `concurrent`, `limit` and `IdleCount` generate the upper limit of running machines

A magic equation doesn't exist to tell you what to set `limit` or
`concurrent` to. Act according to your needs. Having `IdleCount` of _Idle_
machines is a speedup feature. You don't need to wait 10s/20s/30s for the
instance to be created. But as a user, you'd want all your machines (for which
you need to pay) to be running jobs, not stay in _Idle_ state. So you should
have `concurrent` and `limit` set to values that run the maximum count of
machines you are willing to pay for. As for `IdleCount`, it should be set to a
value that generates a minimum amount of _not used_ machines when the job
queue is empty.

Let's assume the following example:

```toml
concurrent=20

[[runners]]
  limit = 40
  [runners.machine]
    IdleCount = 10
```

In the above scenario the total amount of machines we could have is 30. The
`limit` of total machines (building and idle) can be 40. We can have 10 idle
machines but the `concurrent` jobs are 20. So in total we can have 20
concurrent machines running jobs and 10 idle, summing up to 30.

But what happens if the `limit` is less than the total amount of machines that
could be created? The example below explains that case:

```toml
concurrent=20

[[runners]]
  limit = 25
  [runners.machine]
    IdleCount = 10
```

In this example, you can have a maximum of 20 concurrent jobs and 25 machines.
In the worst case scenario, you can't have 10 idle machines, but only 5, because the `limit` is 25.

## Autoscaling periods configuration

> Introduced in [GitLab Runner 13.0](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/5069).

Autoscaling can be configured to have different values depending on the time period.
Organizations might have regular times when spikes of jobs are being executed,
and other times with few to no jobs.
For example, most commercial companies work from Monday to
Friday in fixed hours, like 10am to 6pm. On nights and weekends
for the rest of the week, and on the weekends, no pipelines are started.

These periods can be configured with the help of `[[runners.machine.autoscaling]]` sections.
Each of them supports setting `IdleCount` and `IdleTime` based on a set of `Periods`.

**How autoscaling periods work**

In the `[runners.machine]` settings, you can add multiple `[[runners.machine.autoscaling]]` sections, each one with its own `IdleCount`, `IdleTime`, `Periods` and `Timezone` properties. A section should be defined for each configuration, proceeding in order from the most general scenario to the most specific scenario.

All sections are parsed. The last one to match the current time is active. If none match, the values from the root of `[runners.machine]` are used.

For example:

```toml
[runners.machine]
  MachineName = "auto-scale-%s"
  MachineDriver = "google"
  IdleCount = 10
  IdleTime = 1800
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

In this configuration, every weekday between 9 and 16:59 UTC, machines are overprovisioned to handle the large traffic during operating hours. On the weekend, `IdleCount` drops to 5 to account for the drop in traffic.
The rest of the time, the values are taken from the defaults in the root - `IdleCount = 10` and `IdleTime = 1800`.

NOTE:
The 59th second of the last
minute in any period that you specify is *not* be considered part of the
period. For more information, see [issue #2170](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/2170).

You can specify the `Timezone` of a period, for example `"Australia/Sydney"`. If you don't,
the system setting of the host machine of every runner is used. This
default can be stated as `Timezone = "Local"` explicitly.

More information about the syntax of `[[runner.machine.autoscaling]]` sections can be found
in [GitLab Runner - Advanced Configuration - The `[runners.machine]` section](advanced-configuration.md#the-runnersmachine-section).

## Off Peak time mode configuration (Deprecated)

> This setting is deprecated and will be removed in 14.0. Use autoscaling periods instead.
> If both settings are used, the Off Peak settings will be ignored.

Autoscale can be configured with the support for _Off Peak_ time mode periods.

**What is _Off Peak_ time mode period?**

Some organizations can select a regular time periods when no work is done.
These time periods are called _Off Peak_.

Organizations where _Off Peak_ time periods occur probably don't want
to pay for the _Idle_ machines when jobs are going to be
executed in this time. Especially when `IdleCount` is set to a big number.

**How it is working?**

Configuration of _Off Peak_ is done by four parameters: `OffPeakPeriods`,
`OffPeakTimezone`, `OffPeakIdleCount` and `OffPeakIdleTime`. The
`OffPeakPeriods` setting contains an array of cron-style patterns defining
when the _Off Peak_ time mode should be set on. For example:

```toml
[runners.machine]
  OffPeakPeriods = [
    "* * 0-8,18-23 * * mon-fri *",
    "* * * * * sat,sun *"
  ]
```

This example enables the _Off Peak_ periods described above, so on weekdays
from 12:00am through 8:59am and 6:00pm through 11:59pm, plus all of Saturday and Sunday. Machines
scheduler is checking all patterns from the array and if at least one of
them describes current time, then the _Off Peak_ time mode is enabled.

When the _Off Peak_ time mode is enabled machines scheduler use
`OffPeakIdleCount` instead of `IdleCount` setting and `OffPeakIdleTime`
instead of `IdleTime` setting. The autoscaling algorithm is not changed,
only the parameters. When machines scheduler discovers that none from
the `OffPeakPeriods` pattern is fulfilled then it switches back to
`IdleCount` and `IdleTime` settings.

## Distributed runners caching

NOTE:
Read how to [use a distributed cache](../configuration/speed_up_job_execution.md#use-a-distributed-cache).

To speed up your jobs, GitLab Runner provides a [cache mechanism](https://docs.gitlab.com/ee/ci/yaml/README.html#cache)
where selected directories and/or files are saved and shared between subsequent
jobs.

This is working fine when jobs are run on the same host, but when you start
using the GitLab Runner autoscale feature, most of your jobs run on a
new (or almost new) host, which executes each job in a new Docker
container. In that case, you can't take advantage of the cache
feature.

To overcome this issue, together with the autoscale feature, the distributed
runners cache feature was introduced.

It uses configured object storage server to share the cache between used Docker hosts.
When restoring and archiving the cache, GitLab Runner queried the server
and downloaded or uploaded the archive respectively.

To enable distributed caching, you have to define it in `config.toml` using the
[`[runners.cache]` directive](advanced-configuration.md#the-runnerscache-section):

```toml
[[runners]]
  limit = 10
  executor = "docker+machine"
  [runners.cache]
    Type = "s3"
    Path = "path/to/prefix"
    Shared = false
    [runners.cache.s3]
      ServerAddress = "s3.example.com"
      AccessKey = "access-key"
      SecretKey = "secret-key"
      BucketName = "runner"
      Insecure = false
```

In the example above, the S3 URLs follow the structure
`http(s)://<ServerAddress>/<BucketName>/<Path>/runner/<runner-id>/project/<id>/<cache-key>`.

To share the cache between two or more runners, set the `Shared` flag to true.
This flag removes the runner token from the URL (`runner/<runner-id>`) and
all configured runners share the same cache. You can also
set `Path` to separate caches between runners when cache sharing is enabled.

## Distributed container registry mirroring

To speed up jobs executed inside of Docker containers, you can use the [Docker
registry mirroring service](https://docs.docker.com/registry/). This service provides a proxy between your
Docker machines and all used registries. Images are downloaded one time by the
registry mirror. On each new host, or on an existing host where the image is
not available, the image is downloaded from the configured registry mirror.

Provided that the mirror exists in your Docker machines LAN, the image
downloading step should be much faster on each host.

To configure the Docker registry mirroring, you have to add `MachineOptions` to
the configuration in `config.toml`:

```toml
[[runners]]
  limit = 10
  executor = "docker+machine"
  [runners.machine]
    (...)
    MachineOptions = [
      (...)
      "engine-registry-mirror=http://10.11.12.13:12345"
    ]
```

Where `10.11.12.13:12345` is the IP address and port where your registry mirror
is listening for connections from the Docker service. It must be accessible for
each host created by Docker Machine.

Read more about how to [use a proxy for containers](../configuration/speed_up_job_execution.md#use-a-proxy-for-containers).

## A complete example of `config.toml`

The `config.toml` below uses the [`google` Docker Machine driver](https://docs.docker.com/machine/drivers/gce/):

```toml
concurrent = 50   # All registered runners can run up to 50 concurrent jobs

[[runners]]
  url = "https://gitlab.com"
  token = "RUNNER_TOKEN"             # Note this is different from the registration token used by `gitlab-runner register`
  name = "autoscale-runner"
  executor = "docker+machine"        # This runner is using the 'docker+machine' executor
  limit = 10                         # This runner can execute up to 10 jobs (created machines)
  [runners.docker]
    image = "ruby:2.6"               # The default image used for jobs is 'ruby:2.6'
  [runners.machine]
    IdleCount = 5                    # There must be 5 machines in Idle state - when Off Peak time mode is off
    IdleTime = 600                   # Each machine can be in Idle state up to 600 seconds (after this it will be removed) - when Off Peak time mode is off
    MaxBuilds = 100                  # Each machine can handle up to 100 jobs in a row (after this it will be removed)
    MachineName = "auto-scale-%s"    # Each machine will have a unique name ('%s' is required)
    MachineDriver = "google" # Refer to Docker Machine docs on how to authenticate: https://docs.docker.com/machine/drivers/gce/#credentials
    MachineOptions = [
      "google-project=GOOGLE-PROJECT-ID",
      "google-zone=GOOGLE-ZONE", # e.g. 'us-central-1'
      "google-machine-type=GOOGLE-MACHINE-TYPE", # e.g. 'n1-standard-8'
      "google-machine-image=ubuntu-os-cloud/global/images/family/ubuntu-1804-lts",
      "google-username=root",
      "google-use-internal-ip",
      "engine-registry-mirror=https://mirror.gcr.io"
    ]
    [[runners.machine.autoscaling]]  # Define periods with different settings
      Periods = ["* * 9-17 * * mon-fri *"] # Every workday between 9 and 17 UTC
      IdleCount = 50
      IdleTime = 3600
      Timezone = "UTC"
    [[runners.machine.autoscaling]]
      Periods = ["* * * * * sat,sun *"] # During the weekends
      IdleCount = 5
      IdleTime = 60
      Timezone = "UTC"
  [runners.cache]
    Type = "s3"
    [runners.cache.s3]
      ServerAddress = "s3-eu-west-1.amazonaws.com"
      AccessKey = "AMAZON_S3_ACCESS_KEY"
      SecretKey = "AMAZON_S3_SECRET_KEY"
      BucketName = "runner"
      Insecure = false
```

Note that the `MachineOptions` parameter contains options for the `google`
driver which is used by Docker Machine to spawn machines hosted on Google Compute Engine,
and one option for Docker Machine itself (`engine-registry-mirror`).
