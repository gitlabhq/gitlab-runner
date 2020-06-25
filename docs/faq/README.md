# Troubleshoot GitLab Runner

This section can assist when troubleshooting GitLab Runner.

## General troubleshooting

The following relate to general Runner troubleshooting.

### What does `coordinator` mean?

The `coordinator` is the GitLab installation from which a job is requested.

In other words, runners are isolated (virtual) machines that pick up jobs
requested by their `coordinator`.

### Where are logs stored when run as a service?

- If the GitLab Runner is run as service on Linux/macOS the daemon logs to syslog.
- If the GitLab Runner is run as service on Windows it logs to System's Event Log.

### Run in `--debug` mode

Is it possible to run GitLab Runner in debug/verbose mode. From a terminal, run:

```shell
gitlab-runner --debug run
```

### I'm seeing `x509: certificate signed by unknown authority`

Please see [the self-signed certificates](../configuration/tls-self-signed.md).

### I get `Permission Denied` when accessing the `/var/run/docker.sock`

If you want to use Docker executor,
and you are connecting to Docker Engine installed on server.
You can see the `Permission Denied` error.
The most likely cause is that your system uses SELinux (enabled by default on CentOS, Fedora and RHEL).
Check your SELinux policy on your system for possible denials.

### The Docker executor gets timeout when building Java project

This most likely happens, because of the broken AUFS storage driver:
[Java process hangs on inside container](https://github.com/moby/moby/issues/18502).
The best solution is to change the [storage driver](https://docs.docker.com/engine/userguide/storagedriver/selectadriver/)
to either OverlayFS (faster) or DeviceMapper (slower).

Check this article about [configuring and running Docker](https://docs.docker.com/engine/articles/configuring/)
or this article about [control and configure with systemd](https://docs.docker.com/engine/articles/systemd/).

### I get 411 when uploading artifacts

This happens due to fact that runner uses `Transfer-Encoding: chunked` which is broken on early version of NGINX (<https://serverfault.com/questions/164220/is-there-a-way-to-avoid-nginx-411-content-length-required-errors>).

Upgrade your NGINX to newer version. For more information see this issue: <https://gitlab.com/gitlab-org/gitlab-runner/-/issues/1031>

### `warning: You appear to have cloned an empty repository.`

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

### `zoneinfo.zip: no such file or directory` error when using `Timezone` or `OffPeakTimezone`

It's possible to configure the timezone in which `[[docker.machine.autoscaling]]` periods
are described. This feature should work on most Unix systems out of the box. However on some
Unix systems, and probably on most non-Unix systems (including Windows, for which we're providing
Runner's binaries), when used, the Runner will crash at start with an error similar to:

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

   If you are starting the Runner as a system service then you will need to update/override
   the service configuration in a way that is provided by your service manager software
   (unix systems) or by adding the `ZONEINFO` variable to the list of environment variables
   available for Runner's user through System Settings (Windows).

### Why can't I run more than one instance of Runner?

You can, but not sharing the same `config.toml` file.

Running multiple instances of Runner using the same configuration file can cause
unexpected and hard-to-debug behavior. In
[GitLab Runner 12.2](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4407),
only a single instance of Runner can use a specific `config.toml` file at
one time.

### `Job failed (system failure): preparing environment:`

This error is often due to your shell [loading your
profile](../shells/index.md#shell-profile-loading), and one of the scripts is
causing the failure.

Example of dotfiles that are known to cause failure:

- `.bash_logout`
- `.condarc`
- `.rvmrc`

## Windows troubleshooting

The following relate to Runner troubleshooting on Windows.

### I get a PathTooLongException during my builds on Windows

This is caused by tools like `npm` which will sometimes generate directory structures
with paths more than 260 characters in length. There are two possible fixes you can
adopt to solve the problem.

#### a) Use Git with core.longpaths enabled

You can avoid the problem by using Git to clean your directory structure, first run
`git config --system core.longpaths true` from the command line and then set your
project to use `git fetch` from the GitLab CI project settings page.

#### b) Use NTFSSecurity tools for PowerShell

The [NTFSSecurity](https://github.com/raandree/NTFSSecurity) PowerShell module provides
a *Remove-Item2* method which supports long paths. The GitLab CI Multi Runner will
detect it if it is available and automatically make use of it.

### I can't run Windows BASH scripts; I'm getting `The system cannot find the batch label specified - buildscript`

You need to prepend `call` to your Batch file line in `.gitlab-ci.yml` so that it looks like `call C:\path\to\test.bat`. Here
is a more complete example:

```yaml
before_script:
  - call C:\path\to\test.bat
```

Additional info can be found under issue [#1025](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/1025).

### How can I get colored output on the web terminal?

**Short answer:**

Make sure that you have the ANSI color codes in your program's output. For the purposes of text formatting, assume that you're
running in a UNIX ANSI terminal emulator (because that's what the webUI's output is).

**Long Answer:**

The web interface for GitLab CI emulates a UNIX ANSI terminal (at least partially). The `gitlab-runner` pipes any output from the build
directly to the web interface. That means that any ANSI color codes that are present will be honored.

Older versions of Windows' CMD terminal (before Win10 version 1511) do not support
ANSI color codes - they use win32 ([`ANSI.SYS`](https://en.wikipedia.org/wiki/ANSI.SYS)) calls instead which are **not** present in
the string to be displayed. When writing cross-platform programs, a developer will typically use ANSI color codes by default and convert
them to win32 calls when running on a Windows system (example: [Colorama](https://pypi.org/project/colorama/)).

If your program is doing the above, then you need to disable that conversion for the CI builds so that the ANSI codes remain in the string.

See [GitLab CI YAML docs](https://docs.gitlab.com/ee/ci/yaml/#coloring-script-output)
for an example using PowerShell and issue [#332](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/332)
for more information.

### `The service did not start due to a logon failure` error when starting service

When installing and starting the GitLab Runner service on Windows you can
meet with such error:

```shell
gitlab-runner install --password WINDOWS_MACHINE_PASSWORD
gitlab-runner start
FATA[0000] Failed to start GitLab Runner: The service did not start due to a logon failure.
```

This error can occur when the user used to execute the service doesn't have
the `SeServiceLogonRight` permission. In such case you need to add this
permission for the chosen user and then try to start the service again.

You can add `SeServiceLogonRight` in two ways:

1. Manually using Administrative Tools:
   - Go to _Control Panel > System and Security > Administrative Tools_,
   - open the _Local Security Policy_ tool,
   - chose the _Security Settings > Local Policies > User Rights Assignment_ on the
     list on the left,
   - open the _Log on as a service_ on the list on the right,
   - click on the _Add User or Group..._ button,
   - add the user ("by hand" or using _Advanced..._ button) and apply the settings.

     > **Notice:** According to [Microsoft's documentation](https://docs.microsoft.com/en-us/previous-versions/windows/it-pro/windows-server-2012-R2-and-2012/dn221981(v=ws.11))
     > this should work for: Windows Vista, Windows Server 2008, Windows 7, Windows 8.1,
     > Windows Server 2008 R2, Windows Server 2012 R2, Windows Server 2012, Windows 8
     >
     > **Notice:** The _Local Security Policy_ tool may be not available in some
     > Windows versions - for example in "Home Edition" variant of each version.

1. From command line, using the `Ntrights.exe` tool:
   - Download tools from [Microsoft's download site](https://www.microsoft.com/en-us/download/details.aspx?id=17657),
   - execute `ntrights.exe ntrights +r SeServiceLogonRight -u USER_NAME_HERE` (remember,
     that you should provide a full path for `ntrights.exe` executable **or** add that
     path to system's `PATH` environment variable).

     > **Notice:** The tool was created in 2003 and was initially designed to use
     > with Windows XP and Windows Server 2003. On [Microsoft sites](https://docs.microsoft.com/en-us/previous-versions/windows/it-pro/windows-server-2008-R2-and-2008/dd548356(v=ws.10))
     > you can find an example of usage `Ntrights.exe` that applies to Windows 7 and Windows Server 2008 R2.
     > This solution is not tested and because of the age of the software **it may not work
     > on newest Windows versions**.

After adding the `SeServiceLogonRight` for the user used in service configuration,
the command `gitlab-runner start` should finish without failures
and the service should be started properly.

### Job marked as success and terminated midway using Kubernetes executor

Please see [Job execution](../executors/kubernetes.md#job-execution).

## macOS troubleshooting

The following relate to Runner troubleshooting on macOS.

### `"launchctl" failed: exit status 112, Could not find domain for`

This message may occur when you try to install GitLab Runner on macOS. Make sure
that you manage GitLab Runner service from the GUI Terminal application, not
the SSH connection.

### `Failed to authorize rights (0x1) with status: -60007.`

If your Runner is stuck on the above message when using macOS, there are two
causes to why this happens:

1. Make sure that your user can perform UI interactions:

   ```shell
   DevToolsSecurity -enable
   sudo security authorizationdb remove system.privilege.taskport is-developer
   ```

   The first command enables access to developer tools for your user.
   The second command allows the user who is member of the developer group to
   do UI interactions, e.g., run the iOS simulator.

1. Make sure that your Runner service doesn't use `SessionCreate = true`.
   Previously, when running GitLab Runner as a service, we were creating
   `LaunchAgents` with `SessionCreate`. At that point (**Mavericks**), this was
   the only solution to make Code Signing work. That changed recently with
   **OS X El Capitan** which introduced a lot of new security features that
   altered this behavior.
   Since GitLab Runner 1.1, when creating a `LaunchAgent`, we don't set
   `SessionCreate`. However, in order to upgrade, you need to manually
   reinstall the `LaunchAgent` script:

   ```shell
   gitlab-runner uninstall
   gitlab-runner install
   gitlab-runner start
   ```

   Then you can verify that `~/Library/LaunchAgents/gitlab-runner.plist` has
   `SessionCreate` set to `false`.

### `fatal: unable to access 'https://path:3000/user/repo.git/': Failed to connect to path port 3000: Operation timed out` error in the job

If one of the jobs fails with this error, make sure the Runner can connect to your GitLab instance. The connection could be blocked by things like:

- firewalls
- proxies
- permissions
- routing configurations
