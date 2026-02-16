---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: Install GitLab Runner on Windows systems.
title: Install GitLab Runner on Windows
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

To install and run GitLab Runner on Windows you need:

- Git, which can be installed from the [official site](https://git-scm.com/download/win)
- A password for your user account, if you want to run it under your user
  account rather than the Built-in System Account.
- The system locale set to English (United States) to avoid character encoding issues.
  For more information, see [issue 38702](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/38702).

## Installation

1. Create a folder somewhere in your system, for example, `C:\GitLab-Runner`.
1. Download the binary for [64-bit](https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-windows-amd64.exe) or [32-bit](https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-windows-386.exe) and put it into the folder you
   created. The following assumes you have renamed the binary to `gitlab-runner.exe` (optional).
   You can download a binary for every available version as described in
   [Bleeding Edge - download any other tagged release](bleeding-edge.md#download-any-other-tagged-release).
1. Make sure to restrict the `Write` permissions on the GitLab Runner directory and executable.
   If you do not set these permissions, regular users can replace the executable with their own and run arbitrary code with elevated privileges.
1. Run an [elevated command prompt](https://learn.microsoft.com/en-us/powershell/scripting/windows-powershell/starting-windows-powershell?view=powershell-7.4#with-administrative-privileges-run-as-administrator):
1. [Register a runner](../register/_index.md).
1. Install GitLab Runner as a service and start it. You can either run the service
   using the Built-in System Account (recommended) or using a user account.

   **Run service using Built-in System Account** (under the example directory created in step 1, `C:\GitLab-Runner`)

   ```powershell
   cd C:\GitLab-Runner
   .\gitlab-runner.exe install
   .\gitlab-runner.exe start
   ```

   **Run service using user account** (under the example directory created in step 1, `C:\GitLab-Runner`)

   You have to enter a valid password for the current user account, because
   it's required to start the service by Windows:

   ```powershell
   cd C:\GitLab-Runner
   .\gitlab-runner.exe install --user ENTER-YOUR-USERNAME --password ENTER-YOUR-PASSWORD
   .\gitlab-runner.exe start
   ```

   See the [troubleshooting section](#windows-troubleshooting) if you encounter any
   errors during the GitLab Runner installation.

1. (Optional) Update the runner's `concurrent` value in `C:\GitLab-Runner\config.toml`
   to allow multiple concurrent jobs as detailed in [advanced configuration details](../configuration/advanced-configuration.md).
   Additionally, you can use the advanced configuration details to update your
   shell executor to use Bash or PowerShell rather than Batch.

Voila! Runner is installed, running, and starts again after each system reboot.
Logs are stored in Windows Event Log.

## Upgrade

1. Stop the service (you need an [elevated command prompt](https://learn.microsoft.com/en-us/powershell/scripting/windows-powershell/starting-windows-powershell?view=powershell-7.4#with-administrative-privileges-run-as-administrator) as before):

   ```powershell
   cd C:\GitLab-Runner
   .\gitlab-runner.exe stop
   ```

1. Download the binary for [64-bit](https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-windows-amd64.exe) or [32-bit](https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-windows-386.exe) and replace runner's executable.
   You can download a binary for every available version as described in
   [Bleeding Edge - download any other tagged release](bleeding-edge.md#download-any-other-tagged-release).

1. Start the service:

   ```powershell
   .\gitlab-runner.exe start
   ```

## Uninstall

From an [elevated command prompt](https://learn.microsoft.com/en-us/powershell/scripting/windows-powershell/starting-windows-powershell?view=powershell-7.4#with-administrative-privileges-run-as-administrator):

```powershell
cd C:\GitLab-Runner
.\gitlab-runner.exe stop
.\gitlab-runner.exe uninstall
cd ..
rmdir /s GitLab-Runner
```

## Windows troubleshooting

Make sure that you read the [FAQ](../faq/_index.md) section which describes
some of the most common problems with GitLab Runner.

If you encounter an error like _The account name is invalid_, try:

```powershell
# Add \. before the username
.\gitlab-runner.exe install --user ".\ENTER-YOUR-USERNAME" --password "ENTER-YOUR-PASSWORD"
```

If you encounter a `The service did not start due to a logon failure` error
while starting the service, see the [FAQ section](#error-the-service-did-not-start-due-to-a-logon-failure) to check how to resolve the problem.

If you don't have a Windows Password, you cannot start the GitLab Runner service but you can
use the Built-in System Account.

For Built-in System Account issues, see
[Configure the Service to Start Up with the Built-in System Account](https://learn.microsoft.com/en-us/troubleshoot/windows-server/system-management-components/service-startup-permissions#resolution-3-configure-the-service-to-start-up-with-the-built-in-system-account)
on the Microsoft support website.

### Get runner logs

When you run `.\gitlab-runner.exe install` it installs `gitlab-runner`
as a Windows service. You can find the logs in the Event Viewer
with the provider name `gitlab-runner`.

If you don't have access to the GUI, in PowerShell, you can run
[`Get-WinEvent`](https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.diagnostics/get-winevent?view=powershell-7.4).

```shell
PS C:\> Get-WinEvent -ProviderName gitlab-runner

   ProviderName: gitlab-runner

TimeCreated                     Id LevelDisplayName Message
-----------                     -- ---------------- -------
2/4/2025 6:20:14 AM              1 Information      [session_server].listen_address not defined, session endpoints disabled  builds=0...
2/4/2025 6:20:14 AM              1 Information      listen_address not defined, metrics & debug endpoints disabled  builds=0...
2/4/2025 6:20:14 AM              1 Information      Configuration loaded                                builds=0...
2/4/2025 6:20:14 AM              1 Information      Starting multi-runner from C:\config.toml...        builds=0...
```

### I get a `PathTooLongException` during my builds on Windows

This error is caused by tools like `npm` which sometimes generate directory structures
with paths more than 260 characters in length. To solve the problem, adopt one of the
following solutions.

- Use Git with `core.longpaths` enabled:

  You can avoid the problem by using Git to clean your directory structure.

  1. Run `git config --system core.longpaths true` from the command line.
  1. Set your project to use `git fetch` from the GitLab CI project settings page.

- Use NTFSSecurity tools for PowerShell:

  The [NTFSSecurity](https://github.com/raandree/NTFSSecurity) PowerShell module provides
  a `Remove-Item2` method which supports long paths. GitLab Runner
  detects it if it is available and automatically make use of it.

> A regression introduced in GitLab Runner 16.9.1 is fixed in GitLab Runner 17.10.0.
> If you intend to use the GitLab Runner versions with regressions, use one of the following workarounds:
>
> - Use `pre_get_sources_script` to re-enable Git system-level settings (by unsetting `Git_CONFIG_NOSYSTEM`).
>   This action enables `core.longpaths` by default on Windows.
>
>   ```yaml
>   build:
>     hooks:
>       pre_get_sources_script:
>         - $env:GIT_CONFIG_NOSYSTEM=''
>   ```
>
> - Build a custom `GitLab-runner-helper` image:
>
>   ```dockerfile
>   FROM registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:x86_64-v17.8.3-servercore21H2
>   ENV GIT_CONFIG_NOSYSTEM=
>   ```

### Error with Windows batch scripts: `The system cannot find the batch label specified - buildscript`

You need to prepend `call` to your Batch file line in `.gitlab-ci.yml` so that it looks like `call C:\path\to\test.bat`.
For example:

```yaml
before_script:
  - call C:\path\to\test.bat
```

For more information, see [issue 1025](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/1025).

### How can I get colored output on the web terminal?

**Short answer**:

Make sure that you have the ANSI color codes in your program's output. For the purposes of text formatting, assume that you're
running in a UNIX ANSI terminal emulator (because it is the web interface output).

**Long Answer**:

The web interface for GitLab CI emulates a UNIX ANSI terminal (at least partially). The `gitlab-runner` pipes any output from the build
directly to the web interface. That means that any ANSI color codes that are present are honored.

Older versions of Windows' command prompt terminal (before Windows 10, version 1511) do not support
ANSI color codes. They use win32 ([`ANSI.SYS`](https://en.wikipedia.org/wiki/ANSI.SYS)) calls instead which are **not** present in
the string to be displayed. When writing cross-platform programs, developers typically use ANSI color codes by default. These codes are converted
to win32 calls when running on a Windows system, for example, [Colorama](https://pypi.org/project/colorama/).

If your program is doing the above, you must disable that conversion for the CI builds so that the ANSI codes remain in the string.

For more information, see [GitLab CI YAML documentation](https://docs.gitlab.com/ci/yaml/#coloring-script-output)
for an example using PowerShell and [issue 332](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/332).

### Error: `The service did not start due to a logon failure`

When installing and starting the GitLab Runner service on Windows you can
meet with such error:

```shell
gitlab-runner install --password WINDOWS_MACHINE_PASSWORD
gitlab-runner start
FATA[0000] Failed to start GitLab Runner: The service did not start due to a logon failure.
```

This error can occur when the user used to execute the service doesn't have
the `SeServiceLogonRight` permission. In this case, you need to add this
permission for the chosen user and then try to start the service again.

1. Go to **Control Panel > System and Security > Administrative Tools**.
1. Open the **Local Security Policy** tool.
1. Select **Security Settings > Local Policies > User Rights Assignment** on the
   list on the left.
1. Open the **Log on as a service** on the list on the right.
1. Select **Add User or Group...**.
1. Add the user ("by hand" or using **Advanced...**) and apply the settings.

According to [Microsoft documentation](https://learn.microsoft.com/en-us/previous-versions/windows/it-pro/windows-server-2012-R2-and-2012/dn221981(v=ws.11)),
this should work for:

- Windows Vista
- Windows Server 2008
- Windows 7
- Windows 8.1
- Windows Server 2008 R2
- Windows Server 2012 R2
- Windows Server 2012
- Windows 8

The Local Security Policy tool may be not available in some
Windows versions, for example in "Home Edition" variant of each version.

After adding the `SeServiceLogonRight` for the user used in service configuration,
the command `gitlab-runner start` should finish without failures
and the service should be started properly.

### Job marked as success or failed incorrectly

Most Windows programs output `exit code 0` for success. However, some programs don't
return an exit code or have a different value for success. An example is the Windows
tool `robocopy`. The following `.gitlab-ci.yml` fails, even though it should be successful,
due to the exit code output by `robocopy`:

```yaml
test:
  stage: test
  script:
    - New-Item -type Directory -Path ./source
    - New-Item -type Directory -Path ./dest
    - Write-Output "Hello World!" > ./source/file.txt
    - robocopy ./source ./dest
  tags:
    - windows
```

In the case above, you need to manually add an exit code check to the `script:`. For example,
you can create a PowerShell script:

```powershell
$exitCodes = 0,1

robocopy ./source ./dest

if ( $exitCodes.Contains($LastExitCode) ) {
    exit 0
} else {
    exit 1
}
```

And change the `.gitlab-ci.yml` file to:

```yaml
test:
  stage: test
  script:
    - New-Item -type Directory -Path ./source
    - New-Item -type Directory -Path ./dest
    - Write-Output "Hello World!" > ./source/file.txt
    - ./robocopyCommand.ps1
  tags:
    - windows
```

Also, be careful of the difference between `return` and `exit` when using PowerShell
functions. While `exit 1` marks a job as failed, `return 1` does not.

### Job marked as success and terminated midway using Kubernetes executor

For more information, see [Job execution](../executors/kubernetes/_index.md#job-execution).

### Docker executor: `unsupported Windows Version`

GitLab Runner checks the version of Windows Server to verify that it's supported.

It does this by running `docker info`.

If GitLab Runner fails to start and displays an error without
specifying a Windows Server version, then the Docker
version might be outdated.

```plaintext
Preparation failed: detecting base image: unsupported Windows Version: Windows Server Datacenter
```

The error should contain detailed information about the Windows Server
version, which is then compared with the versions that GitLab Runner supports.

```plaintext
unsupported Windows Version: Windows Server Datacenter Version (OS Build 18363.720)
```

Docker 17.06.2 on Windows Server returns the following in the output
of `docker info`.

```plaintext
Operating System: Windows Server Datacenter
```

The fix in this case is to upgrade the Docker version of similar age, or later,
than the Windows Server release.

### Kubernetes executor: `unsupported Windows Version`

Kubernetes executor on Windows might fail with the following error:

```plaintext
Using Kubernetes namespace: gitlab-runner
ERROR: Preparation failed: prepare helper image: detecting base image: unsupported Windows Version:
Will be retried in 3s ...
ERROR: Job failed (system failure): prepare helper image: detecting base image: unsupported Windows Version:
```

To fix it, add `node.kubernetes.io/windows-build` node selector in the section `[runners.kubernetes.node_selector]`
of your GitLab Runner configuration file, For example:

```toml
   [runners.kubernetes.node_selector]
     "kubernetes.io/arch" = "amd64"
     "kubernetes.io/os" = "windows"
     "node.kubernetes.io/windows-build" = "10.0.17763"
```

### I'm using a mapped network drive and my build cannot find the correct path

When GitLab Runner runs under a standard user account instead of an administrator
account, it cannot access mapped network drives.
When you try to use mapped network drives, you get the
`The system cannot find the path specified.` error.
This error occurs because service logon sessions have
[security limitations](https://learn.microsoft.com/en-us/windows/win32/services/services-and-redirected-drives)
when accessing resources. Use the [UNC path](https://learn.microsoft.com/en-us/dotnet/standard/io/file-path-formats#unc-paths)
of your drive instead.

### The build container is unable to connect to service containers

To use services with Windows containers:

- Use the networking mode that [creates a network for each job](../executors/docker.md#create-a-network-for-each-job).
- Ensure that the `FF_NETWORK_PER_BUILD` feature flag is enabled.

### The job cannot create a build directory and fails with an error

When you use the `GitLab-Runner` with the `Docker-Windows` executor, a job might fail with an error like:

```shell
fatal: cannot chdir to c:/builds/gitlab/test: Permission denied`
```

When this error occurs, ensure the user the Docker engine is running as has full permissions to `C:\Program Data\Docker`.
The Docker engine must be able to write to this directory for certain actions, and without the correct permissions it fails.

[Read more about configuring Docker Engine on Windows](https://learn.microsoft.com/en-us/virtualization/windowscontainers/manage-docker/configure-docker-daemon).

### Blank lines for Windows Subsystem for Linux (WSL) STDOUT output in job logs

By default the STDOUT output for the Windows Subsystem for Linux (WSL) is not UTF8 encoded and displays as blank lines in the job logs. To display the STDOUT output, you can force UTF8 encoding for WSL by setting the `WSL_UTF8` environment variable.

```yaml
job:
  variables:
    WSL_UTF8: "1"
```
