# The system services of GitLab Runner

GitLab Runner uses the [Go `service` library](https://github.com/kardianos/service)
to detect the underlying OS and eventually install the service file based on
the init system.

NOTE: **Note:**
`service` installs, un-installs, starts, stops, and runs a program as a
service (daemon). Windows XP+, Linux/(systemd | Upstart | SysV),
and macOS/Launchd are supported.

When GitLab Runner [is installed](../install/index.md), the service file is
automatically created:

- **systemd:** `/etc/systemd/system/gitlab-runner.service`
- **upstart:** `/etc/init/gitlab-runner`

## Overriding the default service files

In some cases, you might want to override the default behavior of the service.

For example, when you upgrade GitLab Runner, you should stop it gracefully
until all running jobs are finished. However, systemd, upstart, or other services
may almost immediately restart the process without even noticing.

So, when you upgrade GitLab Runner, the installation script kills and restarts
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
stop the runner and systemd uses `SIGQUIT` as the kill signal, to stop the
process. Additionally, a 2h timeout is set for the stop command, which
means that if any jobs don't terminate gracefully before this timeout, systemd
kills the process by using `SIGKILL`.

### Overriding upstart

For runners that use upstart create `/etc/init/gitlab-runner.override` with the
following content:

```shell
kill signal SIGQUIT
kill timeout 7200
```

After adding these two settings to the upstart unit configuration, you can
stop the runner and upstart does exactly the same as systemd above.
