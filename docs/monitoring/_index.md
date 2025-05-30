---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
description: Prometheus metrics.
title: Monitor GitLab Runner usage
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

GitLab Runner can be monitored using [Prometheus](https://prometheus.io).

## Embedded Prometheus metrics

GitLab Runner is instrumented with native Prometheus
metrics, which can be exposed by using an embedded HTTP server on the `/metrics`
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

{{< alert type="note" >}}

The metrics server exports data about the internal state of the
GitLab Runner process and should not be publicly available!

{{< /alert >}}

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

       ## Configure a prometheus-operator serviceMonitor to allow autodetection of
       ## the scraping target. Requires enabling the service resource below.
       ##
       serviceMonitor:
         enabled: true

         ...
     ```

  1. Configure the `service` monitor to retrieve the configured `metrics`:

     ```yaml
     ## Configure a service resource to allow scraping metrics by uisng
     ## prometheus-operator serviceMonitor
     service:
       enabled: true

       ## Provide additonal labels for the service
       ##
       labels: {}

       ## Provide additonal annotations for the service
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
- `localhost:9252`listens on the loopback interface on port `9252`.
- `[2001:db8::1]:http` listens on IPv6 address `[2001:db8::1]` on the HTTP port `80`.

Remember that for listening on ports below `1024` - at least on Linux/Unix
systems - you need to have root/administrator rights.

The HTTP server is opened on the selected `host:port`
**without any authorization**. If you bind the metrics
server to a public interface, use your firewall limit access
or add an HTTP proxy for authorization and access control.
