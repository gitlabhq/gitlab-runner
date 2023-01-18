---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Install GitLab Runner on Windows **(FREE)**

To install and run GitLab Runner on Windows you need:

- Git, which can be installed from the [official site](https://git-scm.com/download/win)
- A password for your user account, if you want to run it under your user
  account rather than the Built-in System Account.

## Installation

WARNING:
With GitLab Runner 10, the executable was renamed to `gitlab-runner`.

1. Create a folder somewhere in your system, ex.: `C:\GitLab-Runner`.
1. Download the binary for [64-bit](https://gitlab-runner-downloads.s3.amazonaws.com/latest/binaries/gitlab-runner-windows-amd64.exe) or [32-bit](https://gitlab-runner-downloads.s3.amazonaws.com/latest/binaries/gitlab-runner-windows-386.exe) and put it into the folder you
   created. The following assumes you have renamed the binary to `gitlab-runner.exe` (optional).
   You can download a binary for every available version as described in
   [Bleeding Edge - download any other tagged release](bleeding-edge.md#download-any-other-tagged-release).
1. Make sure to restrict the `Write` permissions on the GitLab Runner directory and executable.
   If you do not set these permissions, regular users can replace the executable with their own and run arbitrary code with elevated privileges.
1. Run an [elevated command prompt](https://learn.microsoft.com/en-us/powershell/scripting/windows-powershell/starting-windows-powershell?view=powershell-7#with-administrative-privileges-run-as-administrator):
1. [Register a runner](../register/index.md).
1. Install GitLab Runner as a service and start it. You can either run the service
   using the Built-in System Account (recommended) or using a user account.

   **Run service using Built-in System Account** (under directory created in step 1. from above, ex.: `C:\GitLab-Runner`)

   ```powershell
   cd C:\GitLab-Runner
   .\gitlab-runner.exe install
   .\gitlab-runner.exe start
   ```

   **Run service using user account** (under directory created in step 1. from above, ex.: `C:\GitLab-Runner`)

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

Voila! Runner is installed, running, and will start again after each system reboot.
Logs are stored in Windows Event Log.

## Update

1. Stop the service (you need an [elevated command prompt](https://learn.microsoft.com/en-us/powershell/scripting/windows-powershell/starting-windows-powershell?view=powershell-7#with-administrative-privileges-run-as-administrator) as before):

   ```powershell
   cd C:\GitLab-Runner
   .\gitlab-runner.exe stop
   ```

1. Download the binary for [64-bit](https://gitlab-runner-downloads.s3.amazonaws.com/latest/binaries/gitlab-runner-windows-amd64.exe) or [32-bit](https://gitlab-runner-downloads.s3.amazonaws.com/latest/binaries/gitlab-runner-windows-386.exe) and replace runner's executable.
   You can download a binary for every available version as described in
   [Bleeding Edge - download any other tagged release](bleeding-edge.md#download-any-other-tagged-release).

1. Start the service:

   ```powershell
   .\gitlab-runner.exe start
   ```

## Uninstall

From an [elevated command prompt](https://learn.microsoft.com/en-us/powershell/scripting/windows-powershell/starting-windows-powershell?view=powershell-7#with-administrative-privileges-run-as-administrator):

```powershell
cd C:\GitLab-Runner
.\gitlab-runner.exe stop
.\gitlab-runner.exe uninstall
cd ..
rmdir /s GitLab-Runner
```

## Windows version support policy

We follow the same lifecycle policy as Microsoft
[Servicing Channels](https://learn.microsoft.com/en-us/windows/deployment/update/waas-overview#servicing-channels).

This means that we support:

- [Long-Term Servicing Channel](https://learn.microsoft.com/en-us/windows/deployment/update/waas-overview#long-term-servicing-channel),
  versions for 5 years after their release date. Note that we don't
  support versions that are on extended support.
- [Semi-Annual Channel](https://learn.microsoft.com/en-us/windows/deployment/update/waas-overview#semi-annual-channel)
  versions for 18 months after their release date. We don't support
  these versions after mainstream support ends.

This is the case for both the [Windows binaries](#installation) that we
distribute, and also for the [Docker executor](../executors/docker.md#supported-windows-versions).

NOTE:
The Docker executor for Windows containers has strict version
requirements, because containers have to match the version of the host
OS. See the [list of supported Windows containers](../executors/docker.md#supported-windows-versions)
for more information.

After a Windows version no longer receives mainstream support from
Microsoft, we officially [deprecate the version](https://about.gitlab.com/handbook/product/#deprecated) and
remove it in the next major change. For example, in 12.x we started
supporting [`Windows 1803`](https://learn.microsoft.com/en-us/lifecycle/products/?alpha=1803)
because it came out on `2018-04-30`. Mainstream support ended on
`2019-11-12`, so we deprecated `Windows 1803` in 12.x and it was
[removed](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/6553) in
GitLab 13.0.

As a single source of truth we use
<https://learn.microsoft.com/en-us/lifecycle/products/> which specifies
both the release and mainstream support dates.

Below is a list of versions that are commonly used and their end of life
date:

| OS                                  | Mainstream support end of life date |
|-------------------------------------|-------------------------------------|
| Windows 10 1809/2019                | January 2024                        |
| Windows Server Datacenter 1809/2019 | January 2024                        |
| Windows Server Datacenter 1903      | December 2020                       |

### Future releases

Microsoft releases new Windows Server products in the
[Semi-Annual Channel](https://learn.microsoft.com/en-us/windows-server/get-started/servicing-channels-comparison#semi-annual-channel)
twice a year, and every 2 - 3 years a new major version of Windows Sever
is released in the
[Long-Term Servicing Channel (LTSC)](https://learn.microsoft.com/en-us/windows-server/get-started/servicing-channels-comparison#long-term-servicing-channel-ltsc).

GitLab aims to test and release new GitLab Runner helper images that
include the latest Windows Server version (Semi-Annual Channel) within 1
month of the official Microsoft release date on the Google Cloud Platform. Refer to the
[Windows Server current versions by servicing option list](https://learn.microsoft.com/en-us/windows-server/get-started/windows-server-release-info#windows-server-current-versions-by-servicing-option)
for availability dates.

## Windows troubleshooting

Make sure that you read the [FAQ](../faq/index.md) section which describes
some of the most common problems with GitLab Runner.

If you encounter an error like _The account name is invalid_ try to add `.\` before the username:

```powershell
.\gitlab-runner.exe install --user ".\ENTER-YOUR-USERNAME" --password "ENTER-YOUR-PASSWORD"
```

If you encounter a _The service did not start due to a logon failure_ error
while starting the service, please [look in the FAQ](#the-service-did-not-start-due-to-a-logon-failure-error-when-starting-service) to check how to resolve the problem.

If you don't have a Windows Password, the GitLab Runner service won't start but you can
use the Built-in System Account.

If you have issues with the Built-in System Account, please read
[Configure the Service to Start Up with the Built-in System Account](https://learn.microsoft.com/en-us/troubleshoot/windows-server/system-management-components/service-startup-permissions#resolution-3-configure-the-service-to-start-up-with-the-built-in-system-account)
on Microsoft's support website.

### Get runner logs

When you run `.\gitlab-runner.exe install` it installs `gitlab-runner`
as a Windows service. You can find the logs in the Event Viewer
with the provider name `gitlab-runner`.

If you don't have access to the GUI, in PowerShell, you can run
[`Get-WinEvent`](https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.diagnostics/get-winevent?view=powershell-7.2).

```shell
PS C:\> Get-WinEvent -ProviderName gitlab-runner

   ProviderName: gitlab-runner

TimeCreated                     Id LevelDisplayName Message
-----------                     -- ---------------- -------
2/4/2021 6:20:14 AM              1 Information      [session_server].listen_address not defined, session endpoints disabled  builds=0...
2/4/2021 6:20:14 AM              1 Information      listen_address not defined, metrics & debug endpoints disabled  builds=0...
2/4/2021 6:20:14 AM              1 Information      Configuration loaded                                builds=0...
2/4/2021 6:20:14 AM              1 Information      Starting multi-runner from C:\config.toml...        builds=0...
```

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
a *Remove-Item2* method which supports long paths. GitLab Runner will
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
the `SeServiceLogonRight` permission. In this case, you need to add this
permission for the chosen user and then try to start the service again.

1. Go to _Control Panel > System and Security > Administrative Tools_.
1. Open the _Local Security Policy_ tool.
1. Choose the _Security Settings > Local Policies > User Rights Assignment_ on the
   list on the left.
1. Open the _Log on as a service_ on the list on the right.
1. Click the _Add User or Group..._ button.
1. Add the user ("by hand" or using _Advanced..._ button) and apply the settings.

According to [Microsoft's documentation](https://learn.microsoft.com/en-us/previous-versions/windows/it-pro/windows-server-2012-R2-and-2012/dn221981(v=ws.11))
this should work for: Windows Vista, Windows Server 2008, Windows 7, Windows 8.1,
Windows Server 2008 R2, Windows Server 2012 R2, Windows Server 2012, and Windows 8.

The _Local Security Policy_ tool may be not available in some
Windows versions - for example in "Home Edition" variant of each version.

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
functions. While `exit 1` will mark a job as failed, `return 1` will **not**.

### Job marked as success and terminated midway using Kubernetes executor

Please see [Job execution](../executors/kubernetes.md#job-execution).

### Docker executor: `unsupported Windows Version`

GitLab Runner checks the version of Windows Server to verify that it's supported.

It does this by running `docker info`.

If GitLab Runner fails to start with the following error, but with no Windows
Server version specified, then the likely root cause is that the Docker
version is too old.

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

### I'm using a mapped network drive and my build cannot find the correct path

If GitLab Runner is not being run under an administrator account and instead is using a
standard user account, mapped network drives cannot be used and you'll receive an error stating
`The system cannot find the path specified.`  This is because using a service logon session
[creates some limitations](https://learn.microsoft.com/en-us/windows/win32/services/services-and-redirected-drives)
on accessing resources for security. Use the
[UNC path](https://learn.microsoft.com/en-us/dotnet/standard/io/file-path-formats#unc-paths)
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
The Docker engine must be able to write to this directory for certain actions, and without the correct permissions it will fail.

[Read more about configuring Docker Engine on Windows](https://learn.microsoft.com/en-us/virtualization/windowscontainers/manage-docker/configure-docker-daemon).
