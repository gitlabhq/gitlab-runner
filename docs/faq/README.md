---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/engineering/ux/technical-writing/#assignments
---

# Troubleshoot GitLab Runner

This section can assist when troubleshooting GitLab Runner.

## Confirm your GitLab and GitLab Runner versions

GitLab aims to [guarantee backward compatibility](../index.md#gitlab-runner-versions).
However, as a first troubleshooting step, you should ensure your version
of GitLab Runner is the same as your GitLab version.

## What does `coordinator` mean?

The `coordinator` is the GitLab installation from which a job is requested.

In other words, runners are isolated (virtual) machines that pick up jobs
requested by their `coordinator`.

## Where are logs stored when run as a service?

- If the GitLab Runner is run as service on Linux/macOS the daemon logs to syslog.
- If GitLab Runner is running as a service on Windows, it creates system event logs. To view them, open the Event Viewer (from the Run menu, type `eventvwr.msc` or search for "Event Viewer"). Then go to **Windows Logs > Application**. The **Source** for Runner logs is `gitlab-runner`. If you are using Windows Server Core, run this PowerShell command to get the last 20 log entries: `get-eventlog Application -Source gitlab-runner -Newest 20 | format-table -wrap -auto`.

## Run in `--debug` mode

Is it possible to run GitLab Runner in debug/verbose mode. From a terminal, run:

```shell
gitlab-runner --debug run
```

## Enable debug mode logging in `config.toml`

Debug logging can be enabled in the [global section of the `config.toml`](../configuration/advanced-configuration.md#the-global-section) by setting the `log_level` setting to `debug`.

## Enable debug mode logging for the Helm Chart

If GitLab Runner was installed in a Kubernetes cluster by using the [GitLab Runner Helm Chart](../install/kubernetes.md), you can enable debug logging by setting the `logLevel` option in the [`values.yaml` customization](../install/kubernetes.md#configuring-gitlab-runner-using-the-helm-chart):

```yaml
## Configure the GitLab Runner logging level. Available values are: debug, info, warn, error, fatal, panic
## ref: https://docs.gitlab.com/runner/configuration/advanced-configuration.html#the-global-section
##
logLevel: debug
```

## I'm seeing `x509: certificate signed by unknown authority`

Please see [the self-signed certificates](../configuration/tls-self-signed.md).

## I get `Permission Denied` when accessing the `/var/run/docker.sock`

If you want to use Docker executor,
and you are connecting to Docker Engine installed on server.
You can see the `Permission Denied` error.
The most likely cause is that your system uses SELinux (enabled by default on CentOS, Fedora and RHEL).
Check your SELinux policy on your system for possible denials.

## The Docker executor gets timeout when building Java project

This most likely happens, because of the broken AUFS storage driver:
[Java process hangs on inside container](https://github.com/moby/moby/issues/18502).
The best solution is to change the [storage driver](https://docs.docker.com/engine/userguide/storagedriver/selectadriver/)
to either OverlayFS (faster) or DeviceMapper (slower).

Check this article about [configuring and running Docker](https://docs.docker.com/engine/articles/configuring/)
or this article about [control and configure with systemd](https://docs.docker.com/engine/articles/systemd/).

## I get 411 when uploading artifacts

This happens due to fact that GitLab Runner uses `Transfer-Encoding: chunked` which is broken on early version of NGINX (<https://serverfault.com/questions/164220/is-there-a-way-to-avoid-nginx-411-content-length-required-errors>).

Upgrade your NGINX to newer version. For more information see this issue: <https://gitlab.com/gitlab-org/gitlab-runner/-/issues/1031>

## `warning: You appear to have cloned an empty repository.`

When running `git clone` using HTTP(s) (with GitLab Runner or manually for
tests) and you see the following output:

```shell
$ git clone https://git.example.com/user/repo.git

Cloning into 'repo'...
warning: You appear to have cloned an empty repository.
```

Make sure, that the configuration of the HTTP Proxy in your GitLab server
installation is done properly. Especially if you are using some HTTP Proxy with
its own configuration, make sure that GitLab requests are proxied to the
**GitLab Workhorse socket**, not to the **GitLab Unicorn socket**.

Git protocol via HTTP(S) is resolved by the GitLab Workhorse, so this is the
**main entrypoint** of GitLab.

If you are using Omnibus GitLab, but don't want to use the bundled NGINX
server, please read [using a non-bundled web-server](https://docs.gitlab.com/omnibus/settings/nginx.html#using-a-non-bundled-web-server).

In the GitLab Recipes repository there are [web-server configuration
examples](https://gitlab.com/gitlab-org/gitlab-recipes/tree/master/web-server) for Apache and NGINX.

If you are using GitLab installed from source, please also read the above
documentation and examples, and make sure that all HTTP(S) traffic is going
through the **GitLab Workhorse**.

See [an example of a user issue](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/1105).

## `zoneinfo.zip: no such file or directory` error when using `Timezone` or `OffPeakTimezone`

It's possible to configure the timezone in which `[[docker.machine.autoscaling]]` periods
are described. This feature should work on most Unix systems out of the box. However on some
Unix systems, and probably on most non-Unix systems (including Windows, for which we're providing
GitLab Runner binaries), when used, the runner will crash at start with an error similar to:

```plaintext
Failed to load config Invalid OffPeakPeriods value: open /usr/local/go/lib/time/zoneinfo.zip: no such file or directory
```

The error is caused by the `time` package in Go. Go uses the IANA Time Zone database to load
the configuration of the specified timezone. On most Unix systems, this database is already present on
one of well-known paths (`/usr/share/zoneinfo`, `/usr/share/lib/zoneinfo`, `/usr/lib/locale/TZ/`).
Go's `time` package looks for the Time Zone database in all those three paths. If it doesn't find any
of them, but the machine has a configured Go development environment, then it will fallback to
the `$GOROOT/lib/time/zoneinfo.zip` file.

If none of those paths are present (for example on a production Windows host) the above error is thrown.

In case your system has support for the IANA Time Zone database, but it's not available by default, you
can try to install it. For Linux systems it can be done for example by:

```shell
# on Debian/Ubuntu based systems
sudo apt-get install tzdata

# on RPM based systems
sudo yum install tzdata

# on Linux Alpine
sudo apk add -U tzdata
```

If your system doesn't provide this database in a _native_ way, then you can make `OffPeakTimezone`
working by following the steps below:

1. Downloading the [`zoneinfo.zip`](https://gitlab-runner-downloads.s3.amazonaws.com/latest/zoneinfo.zip). Starting with version v9.1.0 you can download
   the file from a tagged path. In that case you should replace `latest` with the tag name (e.g., `v9.1.0`)
   in the `zoneinfo.zip` download URL.

1. Store this file in a well known directory. We're suggesting to use the same directory where
   the `config.toml` file is present. So for example, if you're hosting Runner on Windows machine
   and your configuration file is stored at `C:\gitlab-runner\config.toml`, then save the `zoneinfo.zip`
   at `C:\gitlab-runner\zoneinfo.zip`.

1. Set the `ZONEINFO` environment variable containing a full path to the `zoneinfo.zip` file. If you
   are starting the Runner using the `run` command, then you can do this with:

   ```shell
   ZONEINFO=/etc/gitlab-runner/zoneinfo.zip gitlab-runner run <other options ...>
   ```

   or if using Windows:

   ```powershell
   C:\gitlab-runner> set ZONEINFO=C:\gitlab-runner\zoneinfo.zip
   C:\gitlab-runner> gitlab-runner run <other options ...>
   ```

   If you are starting GitLab Runner as a system service then you will need to update/override
   the service configuration in a way that is provided by your service manager software
   (unix systems) or by adding the `ZONEINFO` variable to the list of environment variables
   available for the GitLab Runner user through System Settings (Windows).

## Why can't I run more than one instance of GitLab Runner?

You can, but not sharing the same `config.toml` file.

Running multiple instances of GitLab Runner using the same configuration file can cause
unexpected and hard-to-debug behavior. In
[GitLab Runner 12.2](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4407),
only a single instance of GitLab Runner can use a specific `config.toml` file at
one time.

## `Job failed (system failure): preparing environment:`

This error is often due to your shell [loading your
profile](../shells/index.md#shell-profile-loading), and one of the scripts is
causing the failure.

Example of dotfiles that are known to cause failure:

- `.bash_logout`
- `.condarc`
- `.rvmrc`

SELinux can also be the culprit of this error. You can confirm this by looking at the SELinux audit log:

```shell
sealert -a /var/log/audit/audit.log
```

## Helm Chart: `ERROR .. Unauthorized`

Before uninstalling or upgrading runners deployed with Helm, pause them in GitLab and
wait for any jobs to complete.

If you remove a runner pod with `helm uninstall` or `helm upgrade`
while a job is running, `Unauthorized` errors like the following
may occur when the job completes:

```plaintext
ERROR: Error cleaning up pod: Unauthorized
ERROR: Error cleaning up secrets: Unauthorized
ERROR: Job failed (system failure): Unauthorized
```

This probably occurs because when the runner is removed, the role bindings
are removed. The runner pod continues until the job completes,
and then the runner tries to delete it.
Without the role binding, the runner pod no longer has access.

See [this issue](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/225)
for details.
