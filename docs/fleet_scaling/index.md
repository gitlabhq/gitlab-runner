---
stage: Verify
group: Runner
info: >-
  To determine the technical writer assigned to the Stage/Group associated with
  this page, see
  https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
---
# Plan and operate a fleet of instance or group runners

This guide contains best practices for scaling a fleet of runners in a shared service model.

When you host a fleet of instance runners, you need a well-planned infrastructure that takes
into consideration your:

- Computing capacity.
- Storage capacity.
- Network bandwidth and throughput.
- Type of jobs (including programming language, OS platform, and dependent libraries).

Use this guide to develop a GitLab Runner deployment strategy based on your organization's requirements.

The guide does not make specific recommendations about the type of infrastructure you should use.
However, it provides insights from the experience of operating the runner fleet on GitLab.com,
which processes millions of CI/CD jobs each month.

## Consider your workload and environment

Before you deploy runners, consider your workload and environment requirements.

- Create a list of the teams that you plan to onboard to GitLab.
- Catalog the programming languages, web frameworks, and libraries in use
  at your organization. For example, Go, C++, PHP, Java, Python, JavaScript, React, Node.js.
- Estimate the number of CI/CD jobs each team may execute per hour, per day.
- Validate if any team has build environment requirements that cannot be
  addressed by using containers.
- Validate if any team has build environment requirements that are best served
  by having runners dedicated to that team.
- Estimate the compute capacity that you may need to support the expected demand.

You might choose different infrastructure stacks to host different runner fleets.
For example, you might need to deploy some runners in the public cloud and some on-premise.

The performance of the CI/CD jobs on the runner fleet is directly related to the fleet's environment.
If you are executing a large number of resource-intensive CI/CD jobs, hosting the fleet on a shared
computing platform is not recommended.

## Runners, executors, and autoscaling capabilities

The `gitlab-runner` executable runs your CI/CD jobs. Each runner is an isolated process that
picks up requests for job executions and deals with them according to pre-defined configurations.
As an isolated process, each runner can create "sub-processes" (also called "workers") to run jobs.

### Concurrency and limit

- [Concurrency](../configuration/advanced-configuration.md#the-global-section):
  Sets the number of jobs that can run concurrently when you're using all of the configured runners on a host system.
- [Limit](../configuration/advanced-configuration.md#the-runners-section):
  Sets the number of sub-processes that a runner can create to execute jobs simultaneously.

The limit is different for autoscaling runners (like Docker Machine and Kubernetes) than it is for runners that don't autoscale.

- On runners that do not autoscale, `limit` defines the capacity of the runner on a host system.
- On autoscaling runners, `limit` is the number of runners you want to run in total.

### Basic configuration: one runner manager, one runner

For the most basic configuration, you install the GitLab Runner software on a supported compute architecture and operating system.
For example, you might have an x86-64 virtual machine (VM) running Ubuntu Linux.

After the installation is complete, you execute the runner registration command just once
and you select the `shell` executor. Then you edit the runner `config.toml` file to set concurrency to `1`.

```toml
concurrent = 1

[[runners]]
  name = "instance-level-runner-001"
  url = ""
  token = ""
  executor = "shell"
```

The GitLab CI/CD jobs that this runner can process are executed directly on the host system where you installed the runner.
It's as if you were running the CI/CD job commands yourself in a terminal. In this case, because you only executed the registration
command one time, the `config.toml` file contains only one `[[runners]]` section. Assuming you set the concurrency value to `1`,
only one runner "worker" can execute CI/CD jobs for the runner process on this system.

### Intermediate configuration: one runner manager, multiple runners

You can also register multiple runners on the same machine.
When you do this, the runner's `config.toml` file has multiple `[[runners]]` sections in it.
If all of the additional runner workers are registered to use the shell executor,
and you update the value of the global configuration option, `concurrent`, to `3`,
the upper limit of jobs that can run concurrently on this host is equal to three.

```toml
concurrent = 3

[[runners]]
  name = "instance_level_shell_001"
  url = ""
  token = ""
  executor = "shell"

[[runners]]
  name = "instance_level_shell_002"
  url = ""
  token = ""
  executor = "shell"

[[runners]]
  name = "instance_level_shell_003"
  url = ""
  token = ""
  executor = "shell"

```

You can register many runner workers on the same machine, and each one is an isolated process.
The performance of the CI/CD jobs for each worker is dependent on the compute capacity of the host system.

### Autoscaling configuration: one or more runner managers, multiple workers

When GitLab Runner is set up for autoscaling, you can configure a runner to act as a manager of other runners.
You can do this with the `docker-machine` or `kubernetes` executors. In this type of
manager-only configuration, the runner agent is itself not executing any CI/CD jobs.

#### Docker Machine executor

With the [Docker Machine executor](../executors/docker_machine.md):

- The runner manager provisions on-demand virtual machine instances with Docker.
- On these VMs, GitLab Runner executes the CI/CD jobs using a container image that you specify in your `.gitlab-ci.yml` file.
- You should test the performance of your CI/CD jobs on various machine types.
- You should consider optimizing your compute hosts based on speed or cost.

#### Kubernetes executor

With the [Kubernetes executor](../executors/kubernetes/index.md):

- The runner manager provisions pods on the target Kubernetes cluster.
- The CI/CD jobs are executed on each pod, which is comprised of multiple containers.
- The pods used for job execution typically require more compute and memory resources than the pod that hosts the runner manager.

#### Reusing a runner configuration

Each runner manager associated with the same runner authentication token is assigned a `system_id` identifier.
The `system_id` identifies the machine where the runner is being used. Runners registered with the same authentication token are grouped under a single runner entry by a unique `system_id.`

Grouping similar runners under a single configuration simplifies runner fleet operations.

Here is an example scenario where you can group similar runners under a single configuration:

A platform administrator needs to provide multiple runners with the same underlying virtual machine instance sizes (2 vCPU, 8GB RAM) using the tag `docker-builds-2vCPU-8GB`. They want at least two such runners, either for high availability or scaling.
Instead of creating two distinct runner entries in the UI, the administrator can create one runner configuration for all runners with the same compute instance size. They can reuse the authentication token for the runner configuration to register multiple runners.
Each registered runner inherits the `docker-builds-2vCPU-8GB` tag.
For all child runners of a single runner configuration, `system_id` acts as a unique identifier.

Grouped runners can be reused to run different jobs by multiple runner managers.

GitLab Runner generates the `system_id` at startup or when the configuration is saved. The `system_id` is saved to the
`.runner_system_id` file in the same directory as the
[`config.toml`](../configuration/advanced-configuration.md), and displays in job logs and the runner
administration page.

##### Generating `system_id` identifiers

To generate the `system_id`, GitLab Runner attempts to derive a unique system identifier from hardware identifiers
(for instance, `/etc/machine-id` in some Linux distributions).
If not successful, GitLab Runner uses a random identifier to generate the `system_id`.

The `system_id` has one the following prefixes:

- `r_`: GitLab Runner assigned a random identifier.
- `s_`: GitLab Runner assigned a unique system identifier from hardware identifiers.

It is important to take this into account when creating container images for example, so that the `system_id` is not
hard-coded into the image. If the `system_id` is hard-coded, you cannot distinguish between hosts
executing a given job.

##### Delete runners and runner managers

To delete runners and runner managers registered with a runner registration token (deprecated), use the `gitlab-runner unregister`
command.

To delete runners and runner managers created with a runner authentication token, use the
[UI](https://docs.gitlab.com/ee/ci/runners/runners_scope.html#delete-instance-runners) or
[API](https://docs.gitlab.com/ee/api/runners.html#delete-a-runner).
Runners created with a runner authentication token are reusable configurations that can be reused in multiple machines.
If you use the [`gitlab-runner unregister`](../commands/index.md#gitlab-runner-unregister) command, only the
runner manager is deleted, not the runner.

## Configure instance runners

Using instance runners in an autoscaling configuration (where a runner acts as a "runner manager")
is an efficient and effective way to start.

The compute capacity of the infrastructure stack where you host your VMs or pods depends on:

- The requirements you captured when you were considering your workload and environment.
- The technology stack you use to host your runner fleet.

You will probably need to adjust your computing capacity after you start
running CI/CD workloads and analyzing the performance over time.

For configurations that use instance runners with an autoscaling executor,
we recommend that you start with, at minimum, two runner managers.

The total number of runner managers you may need over time depends on:

- The compute resources of the stack that hosts the runner managers.
- The concurrency that you choose to configure for each runner manager.
- The load that is generated by the CI/CD jobs that each manager is executing hourly, daily, and monthly.

For example, on GitLab.com, we currently run seven runner managers with the Docker Machine executor.
Each CI/CD job is executed in a Google Cloud Platform (GCP) `n1-standard-1` VM. With this configuration,
we process millions of jobs per month.

## Monitoring runners

An essential step in operating a runner fleet at scale is to set up and use the [runner monitoring](../monitoring/index.md) capabilities included with GitLab.

The following table includes a summary of GitLab Runner metrics. The list does not include the Go-specific process metrics.
To view those metrics on a runner, execute the command as noted [here](../monitoring/index.md#available-metrics).

| Metric name | Description |
| ------ | ------ |
| `gitlab_runner_api_request_statuses_total` | The total number of API requests, partitioned by runner, endpoint, and status. |
| `gitlab_runner_autoscaling_machine_creation_duration_seconds` | Histogram of machine creation time.|
| `gitlab_runner_autoscaling_machine_states`  | The number of machines per state in this provider. |
| `gitlab_runner_concurrent` | The value of concurrent setting. |
| `gitlab_runner_errors_total` | The number of caught errors. This metric is a counter that tracks log lines. The metric includes the label `level`. The possible values are `warning` and `error`. If you plan to include this metric, then use `rate()` or `increase()` when observing. In other words, if you notice that the rate of warnings or errors is increasing, then this could suggest an issue that needs further investigation. |
| `gitlab_runner_jobs` | This shows how many jobs are currently being executed (with different scopes in the labels). |
| `gitlab_runner_job_duration_seconds` | Histogram of job durations. |
| `gitlab_runner_jobs_total` | This displays the total jobs executed. |
| `gitlab_runner_limit` | The current value of the limit setting. |
| `gitlab_runner_request_concurrency` | The current number of concurrent requests for a new job. |
| `gitlab_runner_request_concurrency_exceeded_total` | Count of excess requests above the configured `request_concurrency` limit. |
| `gitlab_runner_version_info` | A metric with a constant `1` value labeled by different build stats fields. |
| `process_cpu_seconds_total` | Total user and system CPU time spent in seconds. |
| `process_max_fds`  | Maximum number of open file descriptors. |
| `process_open_fds` | Number of open file descriptors. |
| `process_resident_memory_bytes`  | Resident memory size in bytes. |
| `process_start_time_seconds` | Start time of the process since unix epoch in seconds. |
| `process_virtual_memory_bytes` | Virtual memory size in bytes. |
| `process_virtual_memory_max_bytes` | Maximum amount of virtual memory available in bytes. |

### Grafana dashboard configuration tips

In this [public repository](https://gitlab.com/gitlab-com/runbooks/-/tree/master/dashboards/ci-runners) you will
find the source code for the Grafana dashboards
that we use to operate the runner fleet on GitLab.com.

We track a lot of metrics for GitLab.com. As a large provider of cloud-based CI/CD, we need many different views
into the system so we can debug issues. In most cases, self-managed runner fleets don't need to track the volume
of metrics that we track with GitLab.com.

Here are a few essential dashboards that we recommend you use to monitor your runner fleet.

**Jobs started on runners**:

- View an overview of the total jobs executed on your runner fleet for a selected time interval.
- View trends in usage. You should analyze this dashboard weekly at a minimum.

You can correlate this data with other metrics, like job duration, to determine if you need configuration changes or
capacity upgrades to continue to service your internal SLO's for CI/CD job performance.

**Job duration**:

- Analyze the performance and scaling of your runner fleet.

**Runner capacity**:

- View the number of jobs being executed divided by the value of limit or concurrent.
- Determine if there is still capacity to execute additional jobs.

### Considerations for monitoring runners on Kubernetes

When you use a Kubernetes platform to host your runner fleet, for example, OpenShift, EKS, or GKE,
you need a different approach for setting up the Grafana dashboards.

On Kubernetes, runner CI/CD job execution pods can be created and deleted frequently.
In these cases, you should plan to monitor the runner manager pod and potentially implement the following:

- Gauges: Display the aggregate of the same metric from different sources.
- Counters: Reset the counter when applying `rate` or `increase` functions.
