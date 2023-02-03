---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# GitLab Runner monitoring **(FREE)**

GitLab Runner can be monitored using [Prometheus](https://prometheus.io).

## Embedded Prometheus metrics

> The embedded HTTP Statistics Server with Prometheus metrics was introduced in GitLab Runner 1.8.0.

GitLab Runner is instrumented with native Prometheus
metrics, which can be exposed via an embedded HTTP server on the `/metrics`
path. The server - if enabled - can be scraped by the Prometheus monitoring
system or accessed with any other HTTP client.

The exposed information includes:

- Runner business logic metrics (e.g., the number of currently running jobs)
- Go-specific process metrics (garbage collection stats, goroutines, memstats, etc.)
- general process metrics (memory usage, CPU usage, file descriptor usage, etc.)
- build version information

The metrics format is documented in Prometheus'
[Exposition formats](https://prometheus.io/docs/instrumenting/exposition_formats/)
specification.

These metrics are meant as a way for operators to monitor and gain insight into
your runners. For example, you might be interested to know if an increase in load average
on the runner host is related to an increase in processed jobs. Or perhaps
you are running a cluster of machines, and you want to
track build trends so you can make changes to your infrastructure.

### Learning more about Prometheus

To learn how to set up a Prometheus server to scrape this HTTP endpoint and
make use of the collected metrics, see Prometheus's
[Getting started](https://prometheus.io/docs/prometheus/latest/getting_started/) guide. Also
see the [Configuration](https://prometheus.io/docs/prometheus/latest/configuration/configuration/)
section for more details on how to configure Prometheus, as well as the section
on [Alerting rules](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/) and setting up
an [Alertmanager](https://prometheus.io/docs/alerting/latest/alertmanager/) to
dispatch alert notifications.

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

For a complete list of available metrics see [Monitoring runners](../fleet_scaling/index.md#monitoring-runners).

## `pprof` HTTP endpoints

> `pprof` integration was introduced in GitLab Runner 1.9.0.

While having metrics about the internal state of the GitLab Runner process is useful,
we've found that in some cases it would be good to check what is happening
inside of the Running process in real time. That's why we've introduced
the `pprof` HTTP endpoints.

`pprof` endpoints will be available via an embedded HTTP server on `/debug/pprof/`
path.

You can read more about using `pprof` in its [documentation](https://pkg.go.dev/net/http/pprof).

## Configuration of the metrics HTTP server

NOTE:
The metrics server exports data about the internal state of the
GitLab Runner process and should not be publicly available!

The metrics HTTP server can be configured in two ways:

- with a `listen_address` global configuration option in `config.toml` file,
- with a `--listen-address` command line option for the `run` command.

If you add the address to your `config.toml` file, to start the metrics HTTP server,
you must restart the runner process.

In both cases the option accepts a string with the format `[host]:<port>`,
where:

- `host` can be an IP address or a hostname,
- `port` is a valid TCP port or symbolic service name (like `http`). We recommend using port `9252` which is already [allocated in Prometheus](https://github.com/prometheus/prometheus/wiki/Default-port-allocations).

If the listen address does not contain a port, it will default to `9252`.

Examples of addresses:

- `:9252` - will listen on all IPs of all interfaces on port `9252`
- `localhost:9252` - will only listen on the loopback interface on port `9252`
- `[2001:db8::1]:http` - will listen on IPv6 address `[2001:db8::1]` on the HTTP port `80`

Remember that for listening on ports below `1024` - at least on Linux/Unix
systems - you need to have root/administrator rights.

The HTTP server is opened on the selected `host:port`
**without any authorization**. If you plan to bind the metrics server
to a public interface then you should consider to use your firewall to
limit access to this server or add an HTTP proxy which will add the
authorization and access control layer.
