---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: The system services of GitLab Runner
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

GitLab Runner uses the [Go `service` library](https://github.com/kardianos/service)
to detect the underlying OS and eventually install the service file based on
the init system.

{{< alert type="note" >}}

The package `service` installs, un-installs, starts, stops, and runs a program as a
service (daemon). Windows XP+, Linux (systemd, Upstart, and System V),
and macOS (launchd) are supported.

{{< /alert >}}

When GitLab Runner [is installed](../install/_index.md), the service file is
automatically created:

- **systemd:** `/etc/systemd/system/gitlab-runner.service`
- **Upstart:** `/etc/init/gitlab-runner`

## Setting custom environment variables

You can run GitLab Runner with custom environment variables. For
example, you want to define `GOOGLE_APPLICATION_CREDENTIALS`
in the runner's environment. This action is different from the
[`environment` configuration setting](advanced-configuration.md#the-runners-section),
which defines the variables that are automatically added to all jobs
executed by a runner.

### Customizing systemd

For runners that use systemd, create `/etc/systemd/system/gitlab-runner.service.d/env.conf`
using one `Environment=key=value` line for each variable to export.

For example:

```toml
[Service]
Environment=GOOGLE_APPLICATION_CREDENTIALS=/etc/gitlab-runner/gce-credentials.json
```

Then reload the configuration:

```shell
systemctl daemon-reload
systemctl restart gitlab-runner.service
```

### Customizing Upstart

For runners that use Upstart, create `/etc/init/gitlab-runner.override` and export the
desired variables.

For example:

```shell
export GOOGLE_APPLICATION_CREDENTIALS="/etc/gitlab-runner/gce-credentials.json"
```

Restart the runner for this to take effect.

## Overriding default stopping behavior

In some cases, you might want to override the default behavior of the service.

For example, when you upgrade GitLab Runner, you should stop it gracefully
until all running jobs are finished. However, systemd, Upstart, or other services
might immediately restart the process without even noticing.

So, when you upgrade GitLab Runner, the installation script kills, and restarts
the runner process that was probably handling new jobs at
the time.

### Overriding systemd

For runners that use systemd, create
`/etc/systemd/system/gitlab-runner.service.d/kill.conf` with the following
content:

```toml
[Service]
TimeoutStopSec=7200
KillSignal=SIGQUIT
```

After adding these two settings to the systemd unit configuration, you can
stop the runner. After the runner stops, systemd uses `SIGQUIT` as the kill signal to stop the
process. Additionally, a two-hour timeout is set for the stop command. If any jobs don't terminate gracefully
before this timeout, systemd kills the process by using `SIGKILL`.

### Overriding Upstart

For runners that use Upstart, create `/etc/init/gitlab-runner.override` with the
following content:

```shell
kill signal SIGQUIT
kill timeout 7200
```

After adding these two settings to the Upstart unit configuration, you can
stop the runner. Upstart does the same as systemd above.
