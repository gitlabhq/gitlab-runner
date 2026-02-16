---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Troubleshooting GitLab Runner
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

This section can assist when troubleshooting GitLab Runner.

## General troubleshooting tips

### View the logs

The GitLab Runner service sends logs to syslog. To view the logs, see your distribution documentation.
If your distribution includes the `journalctl` command, you can use the command to view the logs:

```shell
journalctl --unit=gitlab-runner.service -n 100 --no-pager
docker logs gitlab-runner-container # Docker
kubectl logs gitlab-runner-pod # Kubernetes
```

### Restart the service

```shell
systemctl restart gitlab-runner.service
```

### View the Docker machines

```shell
sudo docker-machine ls
sudo su - && docker-machine ls
```

### Delete all Docker machines

```shell
docker-machine rm $(docker-machine ls -q)
```

### Apply changes to `config.toml`

```shell
systemctl restart gitlab-runner.service
docker-machine rm $(docker-machine ls -q) # Docker machine
journalctl --unit=gitlab-runner.service -f # Tail the logs to check for potential errors
```

## Confirm your GitLab and GitLab Runner versions

GitLab aims to [guarantee backward compatibility](../_index.md#gitlab-runner-versions).
However, as a first troubleshooting step, you should ensure your version
of GitLab Runner is the same as your GitLab version.

## What does `coordinator` mean?

The `coordinator` is the GitLab installation from which a job is requested.

In other words, runner is an isolated agent that request jobs from
the `coordinator` (GitLab installation through GitLab API).

## Where are logs stored when run as a service on Windows?

- If GitLab Runner is running as a service on Windows, it creates system event logs. To view them, open the Event Viewer (from the Run menu, type `eventvwr.msc` or search for "Event Viewer"). Then go to **Windows Logs > Application**. The **Source** for Runner logs is `gitlab-runner`. If you are using Windows Server Core, run this PowerShell command to get the last 20 log entries: `get-eventlog Application -Source gitlab-runner -Newest 20 | format-table -wrap -auto`.

## Enable debug logging mode

{{< alert type="warning" >}}

Debug logging can be a serious security risk. The output contains the content of
all variables and other secrets available to the job. You should disable any log aggregation
that might transmit secrets to third parties. The use of masked variables allows secrets
to be protected in job log output, but not in container logs.

{{< /alert >}}

### In the command line

From a terminal, logged in as root, run the following.

{{< alert type="warning" >}}

This should not be performed on runners with the [Shell executor](../executors/shell.md), because it redefines the `systemd` service
and runs all jobs as root. This poses security risks and changes to file ownership that makes it difficult to revert to a non privileged account.

{{< /alert >}}

```shell
gitlab-runner stop
gitlab-runner --debug run
```

### In the GitLab Runner `config.toml`

Debug logging can be enabled in the [global section of the `config.toml`](../configuration/advanced-configuration.md#the-global-section) by setting the `log_level` setting to `debug`. Add the following line at the very top of your `config.toml`, before/after the concurrent line:

```toml
log_level = "debug"
```

### In the Helm Chart

If GitLab Runner is installed in a Kubernetes cluster using the [GitLab Runner Helm Chart](../install/kubernetes.md), to enable debug logging, set the `logLevel` option in the [`values.yaml` customization](../install/kubernetes.md#configure-gitlab-runner-with-the-helm-chart):

```yaml
## Configure the GitLab Runner logging level. Available values are: debug, info, warn, error, fatal, panic
## ref: https://docs.gitlab.com/runner/configuration/advanced-configuration/#the-global-section
##
logLevel: debug
```

## Configure DNS for a Docker executor runner

When you configure GitLab Runner with the Docker executor, Docker containers might fail to access GitLab, even when the host Runner daemon has access.
This can happen when DNS is configured in the host but those configurations are not passed to the container.

**Example**:

GitLab service and GitLab Runner exist in two different networks that are bridged in two ways (for example, over the Internet and through a VPN).
The runner's routing mechanism might query DNS through the default internet service instead of the DNS service over the VPN.
This configuration would result in the following message:

```shell
Created fresh repository.
++ echo 'Created fresh repository.'
++ git -c 'http.userAgent=gitlab-runner 16.5.0 linux/amd64' fetch origin +da39a3ee5e6b4b0d3255bfef95601890afd80709:refs/pipelines/435345 +refs/heads/master:refs/remotes/origin/master --depth 50 --prune --quiet
fatal: Authentication failed for 'https://gitlab.example.com/group/example-project.git/'
```

In this case, the authentication failure is caused by a service in between the Internet and the GitLab service. This service uses separate credentials, which the runner could circumvent if they used the DNS service over the VPN.

You can tell Docker which DNS server to use by using the `dns` configuration in the `[runners.docker]` section of [the Runner's `config.toml` file](../configuration/advanced-configuration.md#the-runnersdocker-section).

```toml
dns = ["192.168.xxx.xxx","192.168.xxx.xxx"]
```

## I'm seeing `x509: certificate signed by unknown authority`

For more information, see [the self-signed certificates](../configuration/tls-self-signed.md).

## I get `Permission Denied` when accessing the `/var/run/docker.sock`

If you want to use Docker executor,
and you are connecting to Docker Engine installed on server.
You can see the `Permission Denied` error.
The most likely cause is that your system uses SELinux (enabled by default on CentOS, Fedora and RHEL).
Check your SELinux policy on your system for possible denials.

## Docker-machine error: `Unable to query docker version: Cannot connect to the docker engine endpoint.`

This error relates to machine provisioning and might be due to the following reasons:

- There is a TLS failure. When `docker-machine` is installed, some certificates might be invalid.
  To resolve this issue, remove the certificates and restart the runner:

  ```shell
  sudo su -
  rm -r /root/.docker/machine/certs/*
  service gitlab-runner restart
  ```

  After the runner restarts, it registers that the certificates are empty and recreates them.

- The hostname is longer than the supported length in the provisioned machine. For example, Ubuntu machines have
  a 64 character limit for `HOST_NAME_MAX`. The hostname is reported by `docker-machine ls`. Check the `MachineName` in the runner configuration
  and reduce the hostname length if required.

{{< alert type="note" >}}

This error might have occurred before Docker was installed in the machine.

{{< /alert >}}

## `dialing environment connection: ssh: rejected: connect failed (open failed)`

This error occurs when the Docker autoscaler cannot reach the Docker daemon on the
target system when the connection is tunneled through SSH. Ensure that you can SSH to the target system
and successfully run Docker commands, for example `docker info`.

## Adding an AWS Instance Profile to your autoscaled runners

After you create an AWS IAM Role, in your IAM console, the role has a **Role ARN** and a **Instance Profile ARNs**. You must use the **Instance Profile** name, **not** the **Role Name**.

Add the following value to your `[runners.machine]` section:
`"amazonec2-iam-instance-profile=<instance-profile-name>",`

## The Docker executor gets timeout when building Java project

This most likely happens, because of the broken `aufs` storage driver:
[Java process hangs on inside container](https://github.com/moby/moby/issues/18502).
The best solution is to change the [storage driver](https://docs.docker.com/engine/storage/drivers/select-storage-driver/)
to either OverlayFS (faster) or DeviceMapper (slower).

Check this article about [configuring and running Docker](https://docs.docker.com/engine/daemon/)
or this article about [control and configure with systemd](https://docs.docker.com/engine/daemon/proxy/#systemd-unit-file).

## I get 411 when uploading artifacts

This happens due to fact that GitLab Runner uses `Transfer-Encoding: chunked` which is broken on early version of NGINX (<https://serverfault.com/questions/164220/is-there-a-way-to-avoid-nginx-411-content-length-required-errors>).

Upgrade your NGINX to newer version. For more information see this issue: <https://gitlab.com/gitlab-org/gitlab-runner/-/issues/1031>

## I am seeing other artifact upload errors, how can I further debug this?

Artifacts are uploaded directly from the build environment to the GitLab instance,
bypassing the GitLab Runner process.
For example:

- With the Docker executor, uploads occur from the Docker container
- With the Kubernetes executor, uploads occur from the build container in the build pod

The network route from the build environment to the GitLab instance might be different from the
GitLab Runner to the GitLab instance route.

To enable artifact uploads, ensure that all components in the upload path allow
POST requests from the build environment to the GitLab instance.

By default, the artifact uploader logs the upload URL and the HTTP status code
of the upload response. This information is not enough to understand which system
caused an error or blocked artifact uploads. To troubleshoot artifact upload issues,
[enable debug logging](https://docs.gitlab.com/ci/variables/#enable-debug-logging)
for upload attempts to see upload response's headers and body.

{{< alert type="note" >}}

The response body length for artifact upload debug logging is capped at 512 bytes.
Enable logging only for debugging because sensitive data can be exposed in logs.

{{< /alert >}}

If uploads reach GitLab but fail with an error status code
(for example, produces a non-successful response status code), investigate the
GitLab instance itself. For common artifact upload issues, see
[GitLab documentation](https://docs.gitlab.com/administration/cicd/job_artifacts_troubleshooting/#job-artifact-upload-fails-with-error-500).

## `No URL provided, cache will not be download`/`uploaded`

This error occurs when the GitLab Runner helper receives an invalid URL or does not have
any pre-signed URLs to access a remote cache.
Review each [cache-related `config.toml` entry](../configuration/advanced-configuration.md#the-runnerscache-section)
and provider-specific keys and values.
An invalid URL might be constructed from any item that does not follow the URL syntax requirements.

Additionally, ensure that your helper `image` and `helper_image_flavor` match and are up-to-date.

If there is a problem with the credentials configuration, a
diagnostic error message is added to the GitLab Runner process log.

## Error: `warning: You appear to have cloned an empty repository.`

When running `git clone` using HTTP(s) (with GitLab Runner or manually for
tests) and you see the following output:

```shell
$ git clone https://git.example.com/user/repo.git

Cloning into 'repo'...
warning: You appear to have cloned an empty repository.
```

Make sure that HTTP proxy configuration in your GitLab server
installation is done properly. When using HTTP proxy with its own configuration, ensure that
requests are proxied to the
**GitLab Workhorse socket**, not the **GitLab Unicorn socket**.

Git protocol through HTTP(S) is resolved by the GitLab Workhorse, so this is the
**main entrypoint** of GitLab.

If you are using a Linux package installation, but don't want to use the bundled NGINX
server, see [using a non-bundled web-server](https://docs.gitlab.com/omnibus/settings/nginx/#use-a-non-bundled-web-server).

In the GitLab Recipes repository there are
[web-server configuration examples](https://gitlab.com/gitlab-org/gitlab-recipes/tree/master/web-server) for Apache and NGINX.

If you are using GitLab installed from source, see the above
documentation and examples. Make sure that all HTTP(S) traffic is going
through the **GitLab Workhorse**.

See [an example of a user issue](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/1105).

## Error: `zoneinfo.zip: no such file or directory` error when using `Timezone` or `OffPeakTimezone`

It's possible to configure the time zone in which `[[docker.machine.autoscaling]]` periods
are described. This feature should work on most Unix systems out of the box. However, on some
Unix systems and most non-Unix systems (like Windows, where GitLab Runner binaries are available),
the runner might crash at start with an error:

```plaintext
Failed to load config Invalid OffPeakPeriods value: open /usr/local/go/lib/time/zoneinfo.zip: no such file or directory
```

The error is caused by the `time` package in Go. Go uses the IANA Time Zone database to load
the configuration of the specified time zone. On most Unix systems, this database is already present on
one of well-known paths (`/usr/share/zoneinfo`, `/usr/share/lib/zoneinfo`, `/usr/lib/locale/TZ/`).
Go's `time` package looks for the Time Zone database in all those three paths. If it doesn't find any
of them, but the machine has a configured Go development environment, then it falls back to
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
   the file from a tagged path. In that case you should replace `latest` with the tag name (for example, `v9.1.0`)
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

   If you are starting GitLab Runner as a system service then you must update or override
   the service configuration:

   - On Unix systems, modify the settings through your service manager software.
   - On Windows, add the `ZONEINFO` variable to the list of environment variables available for the GitLab Runner user through System Settings.

## Why can't I run more than one instance of GitLab Runner?

You can, but not sharing the same `config.toml` file.

Running multiple instances of GitLab Runner using the same configuration file can cause
unexpected and hard-to-debug behavior. Only a single instance of GitLab Runner can use a specific `config.toml` file at
one time.

## Jobs experience delays before starting

If jobs from some projects experience significant delays before starting while jobs from other projects run immediately,
you might be experiencing long polling issues.

**Symptoms:**

- Jobs are queued but take an unusually long time to start execution (typically matching your GitLab instance long polling timeout).
- Some runners appear to be stuck while others process jobs normally.
- GitLab Runner logs show `CONFIGURATION: Long polling issues detected`.

**Cause:**

This issue occurs when GitLab Runner workers get stuck in long polling requests to GitLab,
which prevents other jobs from being processed promptly. These issues range from performance
bottlenecks to complete deadlocks, depending on the configuration. The issue is related to the
GitLab CI/CD long polling feature controlled by the GitLab Workhorse `apiCiLongPollingDuration`
setting (default: 50s).

**Solution:**

These issues can occur in several configuration scenarios. For comprehensive information about the causes, configuration examples, and solutions, see the [Long polling issues](../configuration/advanced-configuration.md#long-polling-issues) section in the advanced configuration documentation.

## `Job failed (system failure): preparing environment:`

This error is often due to your shell
[loading your profile](../shells/_index.md#shell-profile-loading), and one of the scripts is
causing the failure.

Example of `dotfiles` that are known to cause failure:

- `.bash_logout`
- `.condarc`
- `.rvmrc`

SELinux can also be the culprit of this error. You can confirm this by looking at the SELinux audit log:

```shell
sealert -a /var/log/audit/audit.log
```

## Runner abruptly terminates after `Cleaning up` stage

CrowdStrike Falcon Sensor has been reported to kill pods after the `Cleaning up files` stage of a job
when the "container drift detection" setting was enabled. To ensure that jobs are able to complete, you must disable this setting.

## Job fails with `remote error: tls: bad certificate (exec.go:71:0s)`

This error can occur when the system time changes significantly during a job
that creates artifacts. Due to the change in system time, SSL certificates are expired, which causes an error when the runner attempts to uploads artifacts.

To ensure SSL verification can succeed during artifact upload,
change the system time to a valid date and time at the end
of the job.
Because the creation time of the artifacts file has also changed,
they are automatically archived.

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

## Elasticsearch service startup error `max virtual memory areas vm.max_map_count [65530] is too low`

On startup of an Elasticsearch service container, you might receive an error simlar to:

- `max virtual memory areas vm.max_map_count [65530] is too low, increase to at least [262144]`

Elasticsearch has a `vm.max_map_count` requirement that has to be set on the instance on which Elasticsearch is run.
See the [Elasticsearch documentation](https://www.elastic.co/docs/deploy-manage/deploy/self-managed/install-elasticsearch-docker-prod)
for how to set this value correctly depending on the platform.

## Error: `Preparing the "docker+machine" executor ERROR: Preparation failed: exit status 1`

This error can occur when the Docker machine is not able to successfully create the executor virtual machines. To get more information
about the error, manually create the virtual machine with the same `MachineOptions` that you have defined in your `config.toml`.

For example: `docker-machine create --driver=google --google-project=GOOGLE-PROJECT-ID --google-zone=GOOGLE-ZONE ...`.

## Error: `No unique index found for name`

This error might occur when you create or update a runner and
the database does not have a unique index for the `tags` table.
In the GitLab UI, you might get a
`Response not successful: Received status code 500` error.

This issue might affect instances that have undergone multiple major upgrades
over an extended period.
To resolve this issue, consolidate any duplicate tags in the table with the
[`gitlab:db:deduplicate_tags` Rake task](https://docs.gitlab.com/administration/raketasks/maintenance/#check-the-database-for-deduplicate-cicd-tags).
For more information, see [Rake tasks](https://docs.gitlab.com/administration/raketasks/).

## Error: `Not authorized to perform sts:AssumeRoleWithWebIdentity`

If you configured an IAM role for your runner's Kubernetes ServiceAccount resource,
but runner logs show that it is not able to perform `sts:AssumeRoleWithWebIdentity`,
you might get an error that states:

```plaintext
{"error":"Not authorized to perform sts:AssumeRoleWithWebIdentity","level":"error","msg":"error while generating S3 pre-signed URL","time":"2025-10-15T18:07:20Z"}
```

This issue occurs when you include `https://` in the `StringLike` or `StringEquals`
condition of your IAM role's trusted entities configuration.

To resolve this issue, remove `https://` from the OIDC URL:

```json
"Action": "sts:AssumeRoleWithWebIdentity",
"Condition": {
  "StringLike": {
    "oidc.eks.<AWS_REGION>.amazonaws.com/id/<OIDC_ID>:sub": "system:serviceaccount:<NAMESPACE>:<SERVICE_ACCOUNT>"
  }
}
```
