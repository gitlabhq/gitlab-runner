---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: Docker Machine Executor autoscale configuration
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

{{< alert type="note" >}}

The Docker Machine executor was deprecated in GitLab 17.5 and is scheduled for removal in GitLab 20.0 (May 2027).
While we continue to support the Docker Machine executor till GitLab 20.0, we do not plan to add new features.
We will address only critical bugs that could prevent CI/CD job execution or affect running costs.
If you're using the Docker Machine executor on Amazon Web Services (AWS) EC2,
Microsoft Azure Compute, or Google Compute Engine (GCE), migrate to the
[GitLab Runner Autoscaler](../runner_autoscale/_index.md).

{{< /alert >}}

With the autoscale feature, you use resources in a more elastic and
dynamic way.

GitLab Runner can autoscale, so that your infrastructure contains only as
many build instances as are necessary at any time. When you configure GitLab Runner to
use only autoscale, the system hosting GitLab Runner acts as a
bastion for all the machines it creates. This machine is referred to as a "Runner Manager."

{{< alert type="note" >}}

Docker has deprecated Docker Machine, the underlying technology used to autoscale
runners on public cloud virtual machines. You can read the issue discussing the
[strategy in response to the deprecation of Docker Machine](https://gitlab.com/gitlab-org/gitlab/-/issues/341856)
for more details.

{{< /alert >}}

Docker Machine autoscaler creates one container per VM, regardless of the `limit` and `concurrent` configuration.

When this feature is enabled and configured properly, jobs are executed on
machines created _on demand_. Those machines, after the job is finished, can
wait to run the next jobs or can be removed after the configured `IdleTime`.
In case of many cloud providers, this approach reduce costs by using existing instances.

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

The autoscale mechanism is based on [Docker Machine](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/tree/main/).
All supported virtualization and cloud provider parameters are available at the
GitLab-managed fork of [Docker Machine](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/tree/main/).

## Runner configuration

This section describes the significant autoscale parameters.
For more configurations details read the
[advanced configuration](advanced-configuration.md).

### Runner global options

| Parameter    | Value   | Description |
|--------------|---------|-------------|
| `concurrent` | integer | Limits how many jobs globally can be run concurrently. This parameter sets the maximum number of jobs that can use _all_ defined runners, both local and autoscale. Together with `limit` (from [`[[runners]]` section](#runners-options)) and `IdleCount` (from [`[runners.machine]` section](advanced-configuration.md#the-runnersmachine-section)) it affects the upper limit of created machines. |

### `[[runners]]` options

| Parameter  | Value   | Description |
|------------|---------|-------------|
| `executor` | string  | To use the autoscale feature, `executor` must be set to `docker+machine`. |
| `limit`    | integer | Limits how many jobs can be handled concurrently by this specific token. `0` means don't limit. For autoscale, it's the upper limit of machines created by this provider (in conjunction with `concurrent` and `IdleCount`). |

### `[runners.machine]` options

Configuration parameters details can be found
in [GitLab Runner - Advanced Configuration - The `[runners.machine]` section](advanced-configuration.md#the-runnersmachine-section).

### `[runners.cache]` options

Configuration parameters details can be found
in [GitLab Runner - Advanced Configuration - The `[runners.cache]` section](advanced-configuration.md#the-runnerscache-section)

### Additional configuration information

There is also a special mode, when you set `IdleCount = 0`. In this mode,
machines are **always** created **on-demand** before each job (if there is no
available machine in idle state). After the job is finished, the autoscaling
algorithm works
[the same as it is described below](#autoscaling-algorithm-and-parameters).
The machine is waiting for the next jobs, and if no one is executed, after
the `IdleTime` period, the machine is removed. If there are no jobs, there
are no machines in idle state.

If the `IdleCount` is set to a value greater than `0`, then idle VMs are created in the background. The runner acquires an existing idle VM before asking for a new job.

- If the job is assigned to the runner, then that job is sent to the previously acquired VM.
- If the job is not assigned to the runner, then the lock on the idle VM is released and the VM is returned back to the pool.

## Limit the number of VMs created by the Docker Machine executor

To limit the number of virtual machines (VMs) created by the Docker Machine executor, use the `limit` parameter in the `[[runners]]` section of the `config.toml` file.

The `concurrent` parameter **does not** limit the number of VMs.

One process can be configured to manage multiple runner workers.
For more information, see [Basic configuration: one runner manager, one runner](../fleet_scaling/_index.md#basic-configuration-one-runner-manager-one-runner).

This example illustrates the values set in the `config.toml` file for one runner process:

```toml
concurrent = 100

[[runners]]
name = "first"
executor = "shell"
limit = 40
(...)

[[runners]]
name = "second"
executor = "docker+machine"
limit = 30
(...)

[[runners]]
name = "third"
executor = "ssh"
limit = 10

[[runners]]
name = "fourth"
executor = "virtualbox"
limit = 20
(...)

```

With this configuration:

- One runner process can create four different runner workers using different execution environments.
- The `concurrent` value is set to 100, so this one runner executes a maximum of 100 concurrent GitLab CI/CD jobs.
- Only the `second` runner worker is configured to use the Docker Machine executor and therefore can automatically create VMs.
- The `limit` setting of `30` means that the `second` runner worker can execute a maximum of 30 CI/CD jobs on autoscaled VMs at any point in time.
- While `concurrent` defines the global concurrency limit across multiple `[[runners]]` workers, `limit` defines the maximum concurrency for a single `[[runners]]` worker.

In this example, the runner process handles:

- Across all `[[runners]]` workers, up to 100 concurrent jobs.
- For the `first` worker, no more than 40 jobs, which are executed with the `shell` executor.
- For the `second` worker, no more than 30 jobs, which are executed with the `docker+machine` executor. Additionally, Runner maintain VMs based on the autoscaling configuration in `[runners.machine]`, but no more than 30 VMs in all states (idle, in-use, in-creation, in-removal).
- For the `third` worker, no more than 10 jobs, executed with the `ssh` executor.
- For the `fourth` worker, no more than 20 jobs, executed with the `virtualbox` executor.

In this second example, there are two `[[runners]]` workers configured to use the `docker+machine` executor. With this configuration, each runner worker manages a separate pool of VMs that are constrained by the value of the `limit` parameter.

```toml
concurrent = 100

[[runners]]
name = "first"
executor = "docker+machine"
limit = 80
(...)

[[runners]]
name = "second"
executor = "docker+machine"
limit = 50
(...)

```

In this example:

- The runner processes no more than 100 jobs (the value of `concurrent`).
- The runner process executes jobs in two `[[runners]]` workers, each of which uses the `docker+machine` executor.
- The `first` runner can create a maximum of 80 VMs. Therefore this runner can execute a maximum of 80 jobs at any point in time.
- The `second` runner can create a maximum of 50 VMs. Therefore this runner can execute a maximum of 50 jobs at any point in time.

{{< alert type="note" >}}

Though the sum of limit values is `130` (`80 + 50`), the runner process executes a maximum of 100 jobs concurrently because the global
`concurrent` setting is 100.

{{< /alert >}}

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

**What happened**:

In the example, there are two machines waiting in idle state for new jobs. After the five jobs
are queued, new machines are created. So, in total there are seven machines:
five running jobs and two in idle state waiting for the next
jobs.

GitLab Runner creates a new
idle machine for each machine used for the job execution, until `IdleCount`
is satisfied. Machines are created up to the number defined by the
`limit` parameter. When GitLab Runner detects that this `limit` has been reached,
it stops autoscaling. The new jobs must wait in the job queue until machines
start returning to idle state.

In the above example, two idle machines are always available. The `IdleTime` parameter
applies only when the number exceeds `IdleCount`. At this point, GitLab Runner reduces
the number of machines to match `IdleCount`.

**Scaling down**:

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

## How `concurrent`, `limit` and `IdleCount` generate the upper limit of running machines

A magic equation doesn't exist to tell you what to set `limit` or
`concurrent` to. Act according to your needs. Having `IdleCount` of idle
machines is a speedup feature. You don't need to wait 10 s/20 s/30 s for the
instance to be created. But as a user, you'd want all your machines (for which
you need to pay) to be running jobs, not stay in idle state. So you should
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

## The `IdleScaleFactor` strategy

The `IdleCount` parameter defines a static number of idle machines that runner should sustain.
The value you assign depends on your use case.

Start by assigning a reasonably small number of machines in the idle state. Then, have them
automatically adjust to a bigger number, depending on the current usage. To do that, use the experimental
`IdleScaleFactor` setting.

{{< alert type="warning" >}}

`IdleScaleFactor` internally is an `float64` value and requires the float format to be used,
for example: `0.0`, or `1.0` or ,`1.5` etc. If an integer format is used (for example `IdleScaleFactor = 1`),
Runner's process fails with the error:
`FATAL: Service run failed   error=toml: cannot load TOML value of type int64 into a Go float`.

{{< /alert >}}

When you use this setting, GitLab Runner tries to sustain a defined number of
machines in the idle state. However, this number is no longer static. Instead of using `IdleCount`,
GitLab Runner counts the machines in use and defines the desired idle capacity as
a factor of that number.

If there aren't any used machines, `IdleScaleFactor` evaluates to no idle machines
to maintain. If `IdleCount` is greater than `0` (and only then
the `IdleScaleFactor` is applicable), runner doesn't ask for jobs if there are no idle machines that can handle
them. Without new jobs the number of used machines would not rise, so `IdleScaleFactor` would constantly evaluate
to `0`. And this would block the Runner in unusable state.

Therefore, we've introduced the second setting: `IdleCountMin`. It defines the minimum number of idle machines
that need to be sustained no matter what `IdleScaleFactor` evaluates to. **The setting can't be set to less than
one if `IdleScaleFactor` is used. Runner automatically sets `IdleCountMin` it one**.

You can also use `IdleCountMin` to define the minimum number of idle machines that should always be available.
This allows new jobs entering the queue to start quickly. As with `IdleCount`, the value you assign
depends on your use case.

For example:

```toml
concurrent=200

[[runners]]
  limit = 200
  [runners.machine]
    IdleCount = 100
    IdleCountMin = 10
    IdleScaleFactor = 1.1
```

In this case, when Runner approaches the decision point, it checks how many machines are in use.
For example, if there are five idle machines and ten machines in use. Multiplying it by the `IdleScaleFactor`
Runner decides that it should have 11 idle machines. So 6 more are created.

If you have 90 idle machines and 100 machines in use, based on the `IdleScaleFactor`, GitLab Runner sees that
it should have `100 * 1.1 = 110` idle machines. So it again starts creating new ones. However, when it reaches
the number of `100` idle machines, it stops creating more idle machines because this is the upper limit defined by `IdleCount`.

If the 100 idle machines in use goes down to 20, the desired number of idle machines is `20 * 1.1 = 22`.
GitLab Runner starts terminating the machines. As described above, GitLab Runner removes the
machines that aren't used for `IdleTime`. Therefore, the removal of too many idle VMs are done
aggressively.

If the number of idle machines goes down to 0, the desired number of idle machines is `0 * 1.1 = 0`. This,
however, is less than the defined `IdleCountMin` setting, so Runner starts removing the idle VMs
until 10 VMs remain. After that point, scaling down stops and Runner keeps 10 machines in idle state.

## Configure autoscaling periods

Autoscaling can be configured to have different values depending on the time period.
Organizations might have regular times when spikes of jobs are being executed,
and other times with few to no jobs.
For example, most commercial companies work from Monday to
Friday in fixed hours, like 10am to 6pm. On nights and weekends
for the rest of the week, and on the weekends, no pipelines are started.

These periods can be configured with the help of `[[runners.machine.autoscaling]]` sections.
Each of them supports setting `IdleCount` and `IdleTime` based on a set of `Periods`.

### How autoscaling periods work

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

In this configuration, every weekday between 9 and 16:59 UTC, machines are over-provisioned to handle the large traffic during operating hours. On the weekend, `IdleCount` drops to 5 to account for the drop in traffic.
The rest of the time, the values are taken from the defaults in the root - `IdleCount = 10` and `IdleTime = 1800`.

{{< alert type="note" >}}

The 59th second of the last
minute in any period that you specify is not be considered part of the
period. For more information, see [issue #2170](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/2170).

{{< /alert >}}

You can specify the `Timezone` of a period, for example `"Australia/Sydney"`. If you don't,
the system setting of the host machine of every runner is used. This
default can be stated as `Timezone = "Local"` explicitly.

More information about the syntax of `[[runner.machine.autoscaling]]` sections can be found
in [GitLab Runner - Advanced Configuration - The `[runners.machine]` section](advanced-configuration.md#the-runnersmachine-section).

## Distributed runners caching

{{< alert type="note" >}}

Read how to [use a distributed cache](speed_up_job_execution.md#use-a-distributed-cache).

{{< /alert >}}

To speed up your jobs, GitLab Runner provides a [cache mechanism](https://docs.gitlab.com/ci/yaml/#cache)
where selected directories and/or files are saved and shared between subsequent
jobs.

This mechanism works fine when jobs are run on the same host. However, when you start
using the GitLab Runner autoscale feature, most of your jobs run on a
new (or almost new) host. This new host executes each job in a new Docker
container. In that case, you can't take advantage of the cache
feature.

To overcome this issue, together with the autoscale feature, the distributed
runners cache feature was introduced.

This feature uses configured object storage server to share the cache between used Docker hosts.
GitLab Runner queries the server and downloads the archive to restore the cache,
or uploads it to archive the cache.

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

To speed up jobs executed inside of Docker containers, you can use the
[Docker registry mirroring service](https://docs.docker.com/retired/#registry-now-cncf-distribution). This service provides a proxy between your
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

Read more about how to [use a proxy for containers](speed_up_job_execution.md#use-a-proxy-for-containers).

## A complete example of `config.toml`

The `config.toml` below uses the [`google` Docker Machine driver](https://github.com/docker/docs/blob/173d3c65f8e7df2a8c0323594419c18086fc3a30/machine/drivers/gce.md):

```toml
concurrent = 50   # All registered runners can run up to 50 concurrent jobs

[[runners]]
  url = "https://gitlab.com"
  token = "RUNNER_TOKEN"             # Note this is different from the registration token used by `gitlab-runner register`
  name = "autoscale-runner"
  executor = "docker+machine"        # This runner is using the 'docker+machine' executor
  limit = 10                         # This runner can execute up to 10 jobs (created machines)
  [runners.docker]
    image = "ruby:3.3"               # The default image used for jobs is 'ruby:3.3'
  [runners.machine]
    IdleCount = 5                    # There must be 5 machines in Idle state - when Off Peak time mode is off
    IdleTime = 600                   # Each machine can be in Idle state up to 600 seconds (after this it will be removed) - when Off Peak time mode is off
    MaxBuilds = 100                  # Each machine can handle up to 100 jobs in a row (after this it will be removed)
    MachineName = "auto-scale-%s"    # Each machine will have a unique name ('%s' is required)
    MachineDriver = "google" # Refer to Docker Machine docs on how to authenticate: https://docs.docker.com/machine/drivers/gce/#credentials
    MachineOptions = [
      "google-project=GOOGLE-PROJECT-ID",
      "google-zone=GOOGLE-ZONE", # e.g. 'us-west1'
      "google-machine-type=GOOGLE-MACHINE-TYPE", # e.g. 'n1-standard-8'
      "google-machine-image=ubuntu-os-cloud/global/images/family/ubuntu-1804-lts",
      "google-username=root",
      "google-use-internal-ip",
      "engine-registry-mirror=https://mirror.gcr.io"
    ]
    [[runners.machine.autoscaling]]  # Define periods with different settings
      Periods = ["* * 9-17 * * mon-fri *"] # Every workday between 9 and 17 UTC
      IdleCount = 50
      IdleCountMin = 5
      IdleScaleFactor = 1.5 # Means that current number of Idle machines will be 1.5*in-use machines,
                            # no more than 50 (the value of IdleCount) and no less than 5 (the value of IdleCountMin)
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
      ServerAddress = "s3.eu-west-1.amazonaws.com"
      AccessKey = "AMAZON_S3_ACCESS_KEY"
      SecretKey = "AMAZON_S3_SECRET_KEY"
      BucketName = "runner"
      Insecure = false
```

The `MachineOptions` parameter contains options for both the `google` driver that Docker Machine
uses to create machines on Google Compute Engine and for Docker Machine itself (`engine-registry-mirror`).
