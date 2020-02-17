# The system services of GitLab Runner

GitLab Runner uses the [Go `service` library](https://github.com/kardianos/service)
to detect the underlying OS and eventually install the service file based on
the init system.

NOTE: **Note:**
`service` will install / un-install, start / stop, and run a program as a
service (daemon). Currently supports Windows XP+, Linux/(systemd | Upstart | SysV),
and macOS/Launchd.

Once GitLab Runner [is installed](../install/index.md), the service file will
be automatically be created:

- **systemd:** `/etc/systemd/system/gitlab-runner.service`
- **upstart:** `/etc/init/gitlab-runner`

## Overriding the default service files

In some cases, you might want to override the default behavior of the service.

For example, when upgrading the Runner, you'll want it to stop gracefully
until all running jobs are finished, but systemd, upstart, or some other service
may almost immediately restart the process without even noticing.

So, when upgrading the Runner, the installation script will kill and restart
the Runner's process which would most probably be handling some new jobs at
that time.

### Overriding systemd

For Runners that use systemd create
`/etc/systemd/system/gitlab-runner.service.d/kill.conf` with the following
content:

```toml
[Service]
TimeoutStopSec=7200
KillSignal=SIGQUIT
```

After adding these two settings to the systemd unit configuration, you can
stop the Runner and systemd will use `SIGQUIT` as the kill signal, to stop the
process. Additionally, a 2h timeout is set for the stop command, which
means that if any jobs don't terminate gracefully before this timeout, systemd
will just `SIGKILL` the process.

### Overriding upstart

For Runners that use upstart create `/etc/init/gitlab-runner.override` with the
following content:

```shell
kill signal SIGQUIT
kill timeout 7200
```

After adding these two settings to the upstart unit configuration, you can
stop the Runner and upstart will do exactly the same as systemd above.
