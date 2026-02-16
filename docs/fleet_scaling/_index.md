---
stage: Verify
group: CI Functions Platform
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Plan and operate a fleet of instance or group runners
---

Apply these best practices and recommendations when scaling a fleet of runners in a shared service model.

When you host a fleet of instance runners, you need a well-planned infrastructure that takes
into consideration your:

- Computing capacity.
- Storage capacity.
- Network bandwidth and throughput.
- Type of jobs (including programming language, OS platform, and dependent libraries).

Use these recommendations to develop a GitLab Runner deployment strategy based on your organization's requirements.

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

For more information about how `concurrency` , `limit`, and `request_concurrency` interact to control job flow,
see the [KB article on GitLab Runner concurrency tuning](https://support.gitlab.com/hc/en-us/articles/21324350882076-GitLab-Runner-Concurrency-Tuning-Understanding-request-concurrency).

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
If all additional runner workers use the shell executor,
and you update global `concurrent` setting value to `3`, the host can run
maximum three jobs at once.

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

With the [Kubernetes executor](../executors/kubernetes/_index.md):

- The runner manager provisions pods on the target Kubernetes cluster.
- The CI/CD jobs are executed on each pod, which is comprised of multiple containers.
- The pods used for job execution typically require more compute and memory resources than the pod that hosts the runner manager.

#### Reusing a runner configuration

Each runner manager associated with the same runner authentication token is assigned a `system_id` identifier.
The `system_id` identifies the machine where the runner is being used. Runners registered with the same authentication token are grouped under a single runner entry by a unique `system_id.`

Grouping similar runners under a single configuration simplifies runner fleet operations.

Here is an example scenario where you can group similar runners under a single configuration:

A platform administrator needs to provide multiple runners with the same underlying virtual machine instance sizes (2 vCPU, 8 GB RAM) using the tag `docker-builds-2vCPU-8GB`. They want at least two such runners, either for high availability or scaling.
Instead of creating two distinct runner entries in the UI, administrators can create one runner configuration for all runners with the same compute instance size. They can reuse the authentication token for the runner configuration to register multiple runners.
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
[UI](https://docs.gitlab.com/ci/runners/runners_scope/#delete-instance-runners) or
[API](https://docs.gitlab.com/api/runners/#delete-a-runner).
Runners created with a runner authentication token are reusable configurations that can be reused in multiple machines.
If you use the [`gitlab-runner unregister`](../commands/_index.md#gitlab-runner-unregister) command, only the
runner manager is deleted, not the runner.

## Configure instance runners

Using instance runners in an autoscaling configuration (where a runner acts as a "runner manager")
is an efficient and effective way to start.

The compute capacity of the infrastructure stack where you host your VMs or pods depends on:

- The requirements you captured when you were considering your workload and environment.
- The technology stack you use to host your runner fleet.

You might have to adjust your computing capacity after you start
running CI/CD workloads and analyzing the performance over time.

For configurations that use instance runners with an autoscaling executor,
you must start with minimum two runner managers.

The total number of runner managers you may need over time depends on:

- The compute resources of the stack that hosts the runner managers.
- The concurrency that you choose to configure for each runner manager.
- The load that is generated by the CI/CD jobs that each manager is executing hourly, daily, and monthly.

For example, on GitLab.com, we run seven runner managers with the Docker Machine executor.
Each CI/CD job is executed in a Google Cloud Platform (GCP) `n1-standard-1` VM. With this configuration,
we process millions of jobs per month.

## Monitoring runners

An essential step in operating a runner fleet at scale is to set up and use the [runner monitoring](../monitoring/_index.md) capabilities included with GitLab.

The following table includes a summary of GitLab Runner metrics. The list does not include the Go-specific process metrics.
To view those metrics on a runner, execute the command as noted in [available metrics](../monitoring/_index.md#available-metrics).

| Metric name                                                    | Description |
|----------------------------------------------------------------|-------------|
| `gitlab_runner_api_request_statuses_total`                     | The total number of API requests, partitioned by runner, endpoint, and status. |
| `gitlab_runner_autoscaling_machine_creation_duration_seconds`  | Histogram of machine creation time. |
| `gitlab_runner_autoscaling_machine_states`                     | The number of machines per state in this provider. |
| `gitlab_runner_concurrent`                                     | The value of concurrent setting. |
| `gitlab_runner_errors_total`                                   | The number of caught errors. This metric is a counter that tracks log lines. The metric includes the label `level`. The possible values are `warning` and `error`. If you plan to include this metric, then use `rate()` or `increase()` when observing. In other words, if you notice that the rate of warnings or errors is increasing, then this could suggest an issue that needs further investigation. |
| `gitlab_runner_jobs`                                           | This shows how many jobs are being executed (with different scopes in the labels). |
| `gitlab_runner_job_duration_seconds`                           | Histogram of job durations. |
| `gitlab_runner_job_queue_duration_seconds`                     | A histogram representing job queue duration. |
| `gitlab_runner_acceptable_job_queuing_duration_exceeded_total` | Counts how often jobs exceed the configured queuing time threshold. |
| `gitlab_runner_job_stage_duration_seconds`                     | A histogram representing job duration across each stage. This metric is a **high cardinality metric**. For more information, see [high cardinality metrics section](#high-cardinality-metrics). |
| `gitlab_runner_jobs_total`                                     | This displays the total jobs executed. |
| `gitlab_runner_limit`                                          | The current value of the limit setting. |
| `gitlab_runner_request_concurrency`                            | The current number of concurrent requests for a new job. |
| `gitlab_runner_request_concurrency_exceeded_total`             | Count of excess requests above the configured `request_concurrency` limit. |
| `gitlab_runner_version_info`                                   | A metric with a constant `1` value labeled by different build stats fields. |
| `process_cpu_seconds_total`                                    | Total user and system CPU time spent in seconds. |
| `process_max_fds`                                              | Maximum number of open file descriptors. |
| `process_open_fds`                                             | Number of open file descriptors. |
| `process_resident_memory_bytes`                                | Resident memory size in bytes. |
| `process_start_time_seconds`                                   | Start time of the process, measured in seconds from the Unix epoch. |
| `process_virtual_memory_bytes`                                 | Virtual memory size in bytes. |
| `process_virtual_memory_max_bytes`                             | Maximum amount of virtual memory available in bytes. |

### Grafana dashboard configuration tips

In this [public repository](https://gitlab.com/gitlab-com/runbooks/-/tree/master/dashboards/ci-runners) you can
find the source code for the Grafana dashboards
that we use to operate the runner fleet on GitLab.com.

We track a lot of metrics for GitLab.com. As a large provider of cloud-based CI/CD, we need many different views
into the system so we can debug issues. In most cases, self-managed runner fleets don't need to track the volume
of metrics that we track with GitLab.com.

#### Dashboard generation process

Grafana accepts only JSON format, so you must convert the `jsonnet` files to JSON.

The [runbooks repository](https://gitlab.com/gitlab-com/runbooks/-/tree/master/dashboards) contains
automated scripts for GitLab infrastructure only. To generate these dashboards for your own environment:

1. Create dashboards using the `jsonnet` configuration language (`.dashboard.jsonnet` files).
1. Process `jsonnet` files with the `jsonnet` library to produce JSON output.
1. Upload the resulting JSON files to Grafana (using the API or UI).

#### Available runner dashboards

Here are a few essential dashboards that you should use to monitor your runner fleet:

Jobs started on runners:

- View an overview of the total jobs executed on your runner fleet for a selected time interval.
- View trends in usage. You should analyze this dashboard weekly at a minimum.
- Correlate this data with metrics like job duration to determine if you need configuration changes or
  capacity upgrades to meet your CI/CD job performance SLOs.

Job duration:

- Analyze the performance and scaling of your runner fleet.
- Identify performance bottlenecks and optimization opportunities.

Runner capacity:

- View the number of jobs being executed divided by the value of limit or concurrent.
- Determine if there is still capacity to execute additional jobs.
- Plan for capacity upgrades based on utilization trends.

Additional dashboards include:

- Main Dashboard (`main.dashboard.jsonnet`): Overview of runner infrastructure and HAProxy metrics.
- Business Metrics (`business-stats.dashboard.jsonnet`): Job statistics, finished job minutes, and runner saturation.
- Autoscaling Algorithm (`autoscaling-algorithm.dashboard.jsonnet`): Visualization of autoscaling behavior and machine states.
- Queuing Overview (`queuing-overview.dashboard.jsonnet`): Job queue depth and wait times.
- Request Concurrency (`request-concurrency.dashboard.jsonnet`): Concurrent request analysis.
- Deployment (`deployment.dashboard.jsonnet`): Deployment-related metrics.
- Incident Dashboards: Specialized dashboards for troubleshooting autoscaling, database, application, and runner manager issues.

Each dashboard includes descriptions and context in the source `jsonnet` files to explain what metrics are being displayed.

### Template variables

Dashboards use Grafana template variables to create reusable dashboard templates across different contexts:

- Environments: For example, `production`, `staging`, `development`.
- Stage: For example, `main`, `canary`.
- Type: For example, `ci`, `verify`. Varies by use case.
- Shard: Optional. For distributed runner deployments.

Organizations that implement these dashboards must adjust these variables to match their own environment structure.
Update these variables in the Grafana dashboard settings after import.

### Supported runners

These dashboards work with all GitLab Runner executor types:

- Kubernetes
- Shell
- VM (Docker Machine)
- Windows

The metrics collection is executor-independent and available across all runner fleet types.

### Customize dashboards

To modify dashboards for your environment:

1. Edit the `.dashboard.jsonnet` files in the `dashboards/ci-runners/` directory.
1. Use [Grafonnet library](https://grafana.github.io/grafonnet-lib/) syntax (built on `jsonnet`).
1. Test the changes using the playground:

   ```shell
   ./test-dashboard.sh dashboards/ci-runners/your-dashboard.dashboard.jsonnet
   ```

1. Regenerate and deploy using `./generate-dashboards.sh`.

For more information, see the [video guide on extending dashboards](https://www.youtube.com/watch?v=yZ2RiY_Akz0).

### Considerations for monitoring runners on Kubernetes

For runner fleets hosted on Kubernetes platforms like OpenShift, EKS, or GKE,
use a different approach to set up Grafana dashboards.

On Kubernetes, runner CI/CD job execution pods can be created and deleted frequently.
In these cases, you should plan to monitor the runner manager pod and potentially implement the following:

- Gauges: Display the aggregate of the same metric from different sources.
- Counters: Reset the counter when applying `rate` or `increase` functions.

## High cardinality metrics

Some metrics can be resource-intensive to ingest and store due to their high cardinality. High cardinality occurs when a metric includes labels that have many possible values, leading to a large number of unique time series data points.

To optimize performance, such metrics are not enabled by default and can be toggled by using the [FF_EXPORT_HIGH_CARDINALITY_METRICS feature flag](../configuration/feature-flags.md).

### List of high cardinality metrics

- `gitlab_runner_job_stage_duration_seconds`: Measures the duration of individual job stages in seconds.
  This metric includes the `stage` label, which can have the following predefined values:

  - `resolve_secrets`
  - `prepare_executor`
  - `prepare_script`
  - `get_sources`
  - `clear_worktree`
  - `restore_cache`
  - `download_artifacts`
  - `after_script`
  - `step_script`
  - `archive_cache`
  - `archive_cache_on_failure`
  - `upload_artifacts_on_success`
  - `upload_artifacts_on_failure`
  - `cleanup_file_variables`

  Additionally, this list may include custom user-defined steps such as `step_run`.

### Managing high cardinality metrics

You can control and reduce cardinality by using [Prometheus relabel configuration](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config)
to remove unnecessary label values or the entire metrics.

#### Example configuration to remove specific stages

The following configuration removes any metrics with the `prepare_executor` value in the `stage` label:

```yaml
scrape_configs:
  - job_name: 'gitlab_runner_metrics'
    static_configs:
      - targets: ['localhost:9252']
    metric_relabel_configs:
      - source_labels: [__name__, "stage"]
        regex: "gitlab_runner_job_stage_duration_seconds;prepare_executor"
        action: drop
```

#### Example to keep only relevant stages

The following configuration keeps only the metrics for the `step_script` stage and discards other metrics entirely:

```yaml
scrape_configs:
  - job_name: 'gitlab_runner_metrics'
    static_configs:
      - targets: ['localhost:9252']
    metric_relabel_configs:
      - source_labels: [__name__, "stage"]
        regex: "gitlab_runner_job_stage_duration_seconds;step_script"
        action: keep
```
