---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: Prometheus metrics.
title: Monitor GitLab Runner usage
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

GitLab Runner can be monitored using [Prometheus](https://prometheus.io).

## Embedded Prometheus metrics

GitLab Runner includes native Prometheus metrics,
which you can expose using an embedded HTTP server on the `/metrics`
path. The server - if enabled - can be scraped by the Prometheus monitoring
system or accessed with any other HTTP client.

The exposed information includes:

- Runner business logic metrics (for example, the number of jobs running at the moment)
- Go-specific process metrics (for example, garbage collection stats, goroutines, and memstats)
- general process metrics (memory usage, CPU usage, file descriptor usage, etc.)
- build version information

The metrics format is documented in Prometheus'
[Exposition formats](https://prometheus.io/docs/instrumenting/exposition_formats/)
specification.

These metrics are meant as a way for operators to monitor and gain insight into
your runners. For example, you might want to know if an increase in load average
on the runner host is related to an increase in processed jobs. Or perhaps
you are running a cluster of machines, and you want to
track build trends so you can make changes to your infrastructure.

### Learning more about Prometheus

To set up Prometheus server to scrape this HTTP endpoint and
use the collected metrics, see Prometheus's
[getting started](https://prometheus.io/docs/prometheus/latest/getting_started/) guide.
For more details on how to configure Prometheus, see
the [configuration](https://prometheus.io/docs/prometheus/latest/configuration/configuration/)
section. For more details about alert configuration, see
[alerting rules](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/) and [Alertmanager](https://prometheus.io/docs/alerting/latest/alertmanager/).

## Available metrics

To find a full list of all available metrics, `curl` the metrics endpoint after it is configured and enabled. For example, for a local runner configured with listening port `9252`:

```shell
$ curl -s "http://localhost:9252/metrics" | grep -E "# HELP"

# HELP gitlab_runner_api_request_statuses_total The total number of api requests, partitioned by runner, endpoint and status.
# HELP gitlab_runner_autoscaling_machine_creation_duration_seconds Histogram of machine creation time.
# HELP gitlab_runner_autoscaling_machine_states The current number of machines per state in this provider.
# HELP gitlab_runner_concurrent The current value of concurrent setting
# HELP gitlab_runner_errors_total The number of caught errors.
# HELP gitlab_runner_limit The current value of limit setting
# HELP gitlab_runner_request_concurrency The current number of concurrent requests for a new job
# HELP gitlab_runner_request_concurrency_exceeded_total Count of excess requests above the configured request_concurrency limit
# HELP gitlab_runner_version_info A metric with a constant '1' value labeled by different build stats fields.
...
```

The list includes [Go-specific process metrics](https://github.com/prometheus/client_golang/blob/v1.19.0/prometheus/go_collector.go).
For a list of available metrics that do not include Go-specific processes, see [Monitoring runners](../fleet_scaling/_index.md#monitoring-runners).

## `pprof` HTTP endpoints

The internal state of the GitLab Runner process through metrics is valuable,
but in some cases you must examine the Running process in real time.
That's why we've introduced the `pprof` HTTP endpoints.

`pprof` endpoints are available through an embedded HTTP server on `/debug/pprof/`
path.

You can read more about using `pprof` in its [documentation](https://pkg.go.dev/net/http/pprof).

## Configuration of the metrics HTTP server

> [!note]
> The metrics server exports data about the internal state of the
> GitLab Runner process and should not be publicly available!

Configure the metrics HTTP server by using one of the following methods:

- Use the `listen_address` global configuration option in the `config.toml` file.
- Use the `--listen-address` command line option for the `run` command.
- For runners using Helm chart, in the `values.yaml`:

  1. Configure the `metrics` option:

     ```yaml
     ## Configure integrated Prometheus metrics exporter
     ##
     ## ref: https://docs.gitlab.com/runner/monitoring/#configuration-of-the-metrics-http-server
     ##
     metrics:
       enabled: true

       ## Define a name for the metrics port
       ##
       portName: metrics

       ## Provide a port number for the integrated Prometheus metrics exporter
       ##
       port: 9252

       ## Configure a prometheus-operator serviceMonitor to allow automatic detection of
       ## the scraping target. Requires enabling the service resource below.
       ##
       serviceMonitor:
         enabled: true

         ...
     ```

  1. Configure the `service` monitor to retrieve the configured `metrics`:

     ```yaml
     ## Configure a service resource to allow scraping metrics by using
     ## prometheus-operator serviceMonitor
     service:
       enabled: true

       ## Provide additional labels for the service
       ##
       labels: {}

       ## Provide additional annotations for the service
       ##
       annotations: {}

       ...
     ```

If you add the address to your `config.toml` file, to start the metrics HTTP server,
you must restart the runner process.

In both cases the option accepts a string with the format `[host]:<port>`,
where:

- `host` can be an IP address or a hostname,
- `port` is a valid TCP port or symbolic service name (like `http`). You should use port `9252` which is already [allocated in Prometheus](https://github.com/prometheus/prometheus/wiki/Default-port-allocations).

If the listen address does not contain a port, it defaults to `9252`.

Examples of addresses:

- `:9252` listens on all interfaces on port `9252`.
- `localhost:9252` listens on the loopback interface on port `9252`.
- `[2001:db8::1]:http` listens on IPv6 address `[2001:db8::1]` on the HTTP port `80`.

Remember that for listening on ports below `1024` - at least on Linux/Unix
systems - you need to have root/administrator privileges.

The HTTP server is opened on the selected `host:port`
**without any authorization**. If you bind the metrics
server to a public interface, use your firewall to limit access
or add an HTTP proxy for authorization and access control.

## Monitor Operator managed GitLab Runners

GitLab Runners managed by the GitLab Runner Operator use the same embedded Prometheus
metrics server as standalone GitLab Runner instances. The metrics server is preconfigured
with `listenAddr` set to `[::]:9252`, which listens on all IPv6 and IPv4 interfaces on port `9252`.

### Expose metrics port

To enable monitoring and metrics collection for GitLab Runners managed by the GitLab Runner Operator,
see [Monitor Operator managed GitLab Runners](#monitor-operator-managed-gitlab-runners).

#### Configure the metrics port

Add the following patch to the `podSpec` field in your runner configuration:

```yaml
apiVersion: apps.gitlab.com/v1beta2
kind: Runner
metadata:
  name: gitlab-runner
spec:
  gitlabUrl: https://gitlab.example.com
  token: gitlab-runner-secret
  buildImage: alpine
  podSpec:
    name: "metrics-config"
    patch: |
      {
        "containers": [
          {
            "name": "runner",
            "ports": [
              {
                "name": "metrics",
                "containerPort": 9252,
                "protocol": "TCP"
              }
            ]
          }
        ]
      }
    patchType: "strategic"
```

This configuration:

- `name`: Assigns a name to the custom `PodSpec` for identification.
- `patch`: Defines the JSON patch to apply to the `PodSpec`, exposes port `9252` on the runner container.
- `patchType`: Uses the `strategic` merge strategy (default) to apply the patch.
- `port`: Named as `metrics` for easy identification in Kubernetes services.

#### Configure Prometheus scraping

For environments using Prometheus Operator, create a `PodMonitor` resource to directly scrape metrics from runner pods:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
metadata:
  name: gitlab-runner-metrics
  namespace: kube-prometheus-stack
  labels:
    release: kube-prometheus-stack
spec:
  selector:
    matchLabels:
      app.kubernetes.io/component: runner
  namespaceSelector:
    matchNames:
      - gitlab-runner-system
  podMetricsEndpoints:
    - port: metrics
      interval: 10s
      path: /metrics
```

Apply the `PodMonitor` configuration:

```shell
kubectl apply -f gitlab-runner-podmonitor.yaml
```

The `PodMonitor` configuration:

- `selector`: Matches pods with the `app.kubernetes.io/component: runner` label.
- `namespaceSelector`: Limits scraping to the `gitlab-runner-system` namespace.
- `podMetricsEndpoints`: Defines the metrics port, scrape interval, and path.

#### Add runner identification to metrics

To add runner identification to all exported metrics, include relabel configuration in the `PodMonitor`:

```yaml
podMetricsEndpoints:
  - port: metrics
    interval: 10s
    path: /metrics
    relabelings:
      - sourceLabels: [__meta_kubernetes_pod_label_app_kubernetes_io_name]
        targetLabel: runner_name
```

The relabel configuration:

- Extracts the `app.kubernetes.io/name` label from each runner pod (automatically set by GitLab Runner Operator).
- Adds it as a `runner_name` label to all metrics from that pod.
- Enables filter and aggregation metrics by specific runner instances.

The following is an example metrics with runner identification:

```prometheus
gitlab_runner_concurrent{runner_name="my-gitlab-runner"} 10
gitlab_runner_jobs_running_total{runner_name="my-gitlab-runner"} 3
```

#### Direct Prometheus scrape configuration

If you're not using Prometheus Operator, you can add the relabel configuration
directly in the Prometheus scrape configuration:

```yaml
scrape_configs:
  - job_name: 'gitlab-runner-operator'
    kubernetes_sd_configs:
      - role: pod
        namespaces:
          names:
            - gitlab-runner-system
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_label_app_kubernetes_io_name]
        target_label: runner_name
    metrics_path: /metrics
    scrape_interval: 10s
```

This configuration:

- Uses Kubernetes service discovery to find pods in the `gitlab-runner-system` namespace.
- Extracts the `app.kubernetes.io/name` label and adds it as `runner_name` to metrics.

## Monitor GitLab Runner with executors other than Kubernetes

For GitLab Runner deployments with executors other than Kubernetes, you can add runner identification
through external labels in your Prometheus configuration.

### Static configuration with external labels

Configure Prometheus to scrape your GitLab Runner instances and add identifying labels:

```yaml
scrape_configs:
  - job_name: 'gitlab-runner'
    static_configs:
      - targets: ['runner1.example.com:9252']
        labels:
          runner_name: 'production-runner-1'
      - targets: ['runner2.example.com:9252']
        labels:
          runner_name: 'staging-runner-1'
    metrics_path: /metrics
    scrape_interval: 30s
```

This configuration adds runner identification to your metrics:

```prometheus
gitlab_runner_concurrent{runner_name="production-runner-1"} 10
gitlab_runner_jobs_running_total{runner_name="staging-runner-1"} 3
```

This configuration enables you to:

- Filter metrics by specific runner instances.
- Create runner-specific dashboards and alerts.
- Track performance across different runner deployments.

### Available metrics for Operator managed GitLab Runners

GitLab Runners managed by the GitLab Runner Operator expose the same metrics as standalone GitLab Runner deployments. To view all available metrics, use `kubectl` to access the metrics endpoint:

```shell
kubectl port-forward pod/<gitlab-runner-pod-name> 9252:9252
curl -s "http://localhost:9252/metrics" | grep -E "# HELP"
```

For a complete list of available metrics, see [Available metrics](#available-metrics).

### Security considerations for Operator managed GitLab Runners

When you configure the metrics collection for GitLab Runners managed by the GitLab Runner Operator:

- Use Kubernetes `NetworkPolicies` to restrict access to authorized monitoring systems.
- Consider using `mutal` TLS encryption for metric scraping in production environments.

### Troubleshooting Operator managed GitLab Runner monitoring

#### Metrics endpoint not accessible

If you cannot access the metrics endpoint:

1. Verify that the pod specification includes the metrics port configuration.
1. Ensure that the runner pod is running and healthy:

   ```shell
   kubectl get pods -l app.kubernetes.io/component=runner -n gitlab-runner-system
   kubectl describe pod <runner-pod-name> -n gitlab-runner-system
   ```

1. Test the connectivity to the metrics endpoint:

   ```shell
   kubectl port-forward pod/<runner-pod-name> 9252:9252 -n gitlab-runner-system
   curl "http://localhost:9252/metrics"
   ```

#### Missing metrics in Prometheus

If metrics are not appearing in Prometheus:

1. Verify that the `PodMonitor` is correctly configured and applied.
1. Check that the namespace and label selectors match your runner pods.
1. Review Prometheus logs for scraping errors.
1. Validate that the `PodMonitor` is discoverable by Prometheus Operator:

   ```shell
   kubectl get podmonitor gitlab-runner-metrics -n kube-prometheus-stack
   kubectl describe podmonitor gitlab-runner-metrics -n kube-prometheus-stack
   ```
