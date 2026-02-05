---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: Types of shells supported by GitLab Runner
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

GitLab Runner implements shell script generators that allow executing
builds on different systems.

The shell scripts contain commands to execute all steps of the build:

1. `git clone`
1. Restore the build cache
1. Build commands
1. Update the build cache
1. Generate and upload the build artifacts

The shells don't have any configuration options. The build steps are received
from the commands defined in the [`script` directive in `.gitlab-ci.yml`](https://docs.gitlab.com/ci/yaml/#script).

The supported shells are:

| Shell        | Status          | Description |
|--------------|-----------------|-------------|
| `bash`       | Fully Supported | Bash (Bourne Again Shell). All commands executed in Bash context (default for all Unix systems). |
| `sh`         | Fully Supported | Sh (Bourne shell). All commands executed in Sh context (fallback for `bash` for all Unix systems). |
| `powershell` | Fully Supported | PowerShell script. All commands are executed in PowerShell Desktop context. |
| `pwsh`       | Fully Supported | PowerShell script. All commands are executed in PowerShell Core context. This is the default when registering a new runner on Windows. |

If you want to select a particular shell to use other than the default, you must [specify the shell](../executors/shell.md#selecting-your-shell) in your `config.toml` file.

## Sh/Bash shells

Sh/Bash is the default shell used on all Unix based systems. The bash script used
in `.gitlab-ci.yml` is executed by piping the shell script to one of the
following commands:

```shell
# This command is used if the build should be executed in context
# of another user (the shell executor)
cat generated-bash-script | su --shell /bin/bash --login user

# This command is used if the build should be executed using
# the current user, but in a login environment
cat generated-bash-script | /bin/bash --login

# This command is used if the build should be executed in
# a Docker environment
cat generated-bash-script | /bin/bash
```

### Shell profile loading

For certain executors, the runner passes the `--login` flag as shown above,
which also loads the shell profile. Anything that you have in your `.bashrc`,
`.bash_logout`,
[or any other dotfile](https://tldp.org/LDP/Bash-Beginners-Guide/html/sect_03_01.html#sect_03_01_02),
is executed in your job.

If a [job fails on the `Prepare environment`](../faq/_index.md#job-failed-system-failure-preparing-environment) stage, it
is likely that something in the shell profile is causing the failure. A common
failure is when there is a `.bash_logout` that tries to clear the console.

To troubleshoot this error, check `/home/gitlab-runner/.bash_logout`. For example, if the `.bash_logout` file has a script section like the following, comment it out and restart the pipeline:

```shell
if [ "$SHLVL" = 1 ]; then
    [ -x /usr/bin/clear_console ] && /usr/bin/clear_console -q
fi
```

Executors that load shell profiles:

- [`shell`](../executors/shell.md)
- [`parallels`](../executors/parallels.md) (The shell profile of the target virtual machine is loaded)
- [`virtualbox`](../executors/virtualbox.md) (The shell profile of the target virtual machine is loaded)
- [`ssh`](../executors/ssh.md) (The shell profile of the target machine is loaded)

## PowerShell

PowerShell Desktop Edition is the default shell when a new runner is registered on Windows using GitLab Runner
12.0-13.12. In 14.0 and later, the default is PowerShell Core Edition.

PowerShell doesn't support executing the build in context of another user.

The generated PowerShell script is executed by saving its content to a file and
passing the filename to the following command:

- For PowerShell Desktop Edition:

  ```batch
  powershell -NoProfile -NonInteractive -ExecutionPolicy Bypass -Command generated-windows-powershell.ps1
  ```

- For PowerShell Core Edition:

  ```batch
  pwsh -NoProfile -NonInteractive -ExecutionPolicy Bypass -Command generated-windows-powershell.ps1
  ```

The following is an example PowerShell script:

```powershell
$ErrorActionPreference = "Continue" # This will be set to 'Stop' when targetting PowerShell Core

echo "Running on $([Environment]::MachineName)..."

& {
  $CI="true"
  $env:CI=$CI
  $CI_COMMIT_SHA="db45ad9af9d7af5e61b829442fd893d96e31250c"
  $env:CI_COMMIT_SHA=$CI_COMMIT_SHA
  $CI_COMMIT_BEFORE_SHA="d63117656af6ff57d99e50cc270f854691f335ad"
  $env:CI_COMMIT_BEFORE_SHA=$CI_COMMIT_BEFORE_SHA
  $CI_COMMIT_REF_NAME="main"
  $env:CI_COMMIT_REF_NAME=$CI_COMMIT_REF_NAME
  $CI_JOB_ID="1"
  $env:CI_JOB_ID=$CI_JOB_ID
  $CI_REPOSITORY_URL="Z:\Gitlab\tests\test"
  $env:CI_REPOSITORY_URL=$CI_REPOSITORY_URL
  $CI_PROJECT_ID="1"
  $env:CI_PROJECT_ID=$CI_PROJECT_ID
  $CI_PROJECT_DIR="Z:\Gitlab\tests\test\builds\0\project-1"
  $env:CI_PROJECT_DIR=$CI_PROJECT_DIR
  $CI_SERVER="yes"
  $env:CI_SERVER=$CI_SERVER
  $CI_SERVER_NAME="GitLab CI"
  $env:CI_SERVER_NAME=$CI_SERVER_NAME
  $CI_SERVER_VERSION=""
  $env:CI_SERVER_VERSION=$CI_SERVER_VERSION
  $CI_SERVER_REVISION=""
  $env:CI_SERVER_REVISION=$CI_SERVER_REVISION
  $GITLAB_CI="true"
  $env:GITLAB_CI=$GITLAB_CI
  $GIT_SSL_CAINFO=""
  New-Item -ItemType directory -Force -Path "C:\GitLab-Runner\builds\0\project-1.tmp" | out-null
  $GIT_SSL_CAINFO | Out-File "C:\GitLab-Runner\builds\0\project-1.tmp\GIT_SSL_CAINFO"
  $GIT_SSL_CAINFO="C:\GitLab-Runner\builds\0\project-1.tmp\GIT_SSL_CAINFO"
  $env:GIT_SSL_CAINFO=$GIT_SSL_CAINFO
  $CI_SERVER_TLS_CA_FILE=""
  New-Item -ItemType directory -Force -Path "C:\GitLab-Runner\builds\0\project-1.tmp" | out-null
  $CI_SERVER_TLS_CA_FILE | Out-File "C:\GitLab-Runner\builds\0\project-1.tmp\CI_SERVER_TLS_CA_FILE"
  $CI_SERVER_TLS_CA_FILE="C:\GitLab-Runner\builds\0\project-1.tmp\CI_SERVER_TLS_CA_FILE"
  $env:CI_SERVER_TLS_CA_FILE=$CI_SERVER_TLS_CA_FILE
  echo "Cloning repository..."
  if( (Get-Command -Name Remove-Item2 -Module NTFSSecurity -ErrorAction SilentlyContinue) -and (Test-Path "C:\GitLab-Runner\builds\0\project-1" -PathType Container) ) {
    Remove-Item2 -Force -Recurse "C:\GitLab-Runner\builds\0\project-1"
  } elseif(Test-Path "C:\GitLab-Runner\builds\0\project-1") {
    Remove-Item -Force -Recurse "C:\GitLab-Runner\builds\0\project-1"
  }

  & "git" "clone" "https://gitlab.com/group/project.git" "Z:\Gitlab\tests\test\builds\0\project-1"
  if(!$?) { Exit $LASTEXITCODE }

  cd "C:\GitLab-Runner\builds\0\project-1"
  if(!$?) { Exit $LASTEXITCODE }

  echo "Checking out db45ad9a as main..."
  & "git" "checkout" "db45ad9af9d7af5e61b829442fd893d96e31250c"
  if(!$?) { Exit $LASTEXITCODE }

  if(Test-Path "..\..\..\cache\project-1\pages\main\cache.tgz" -PathType Leaf) {
    echo "Restoring cache..."
    & "gitlab-runner-windows-amd64.exe" "extract" "--file" "..\..\..\cache\project-1\pages\main\cache.tgz"
    if(!$?) { Exit $LASTEXITCODE }

  } else {
    if(Test-Path "..\..\..\cache\project-1\pages\main\cache.tgz" -PathType Leaf) {
      echo "Restoring cache..."
      & "gitlab-runner-windows-amd64.exe" "extract" "--file" "..\..\..\cache\project-1\pages\main\cache.tgz"
      if(!$?) { Exit $LASTEXITCODE }

    }
  }
}
if(!$?) { Exit $LASTEXITCODE }

& {
  $CI="true"
  $env:CI=$CI
  $CI_COMMIT_SHA="db45ad9af9d7af5e61b829442fd893d96e31250c"
  $env:CI_COMMIT_SHA=$CI_COMMIT_SHA
  $CI_COMMIT_BEFORE_SHA="d63117656af6ff57d99e50cc270f854691f335ad"
  $env:CI_COMMIT_BEFORE_SHA=$CI_COMMIT_BEFORE_SHA
  $CI_COMMIT_REF_NAME="main"
  $env:CI_COMMIT_REF_NAME=$CI_COMMIT_REF_NAME
  $CI_JOB_ID="1"
  $env:CI_JOB_ID=$CI_JOB_ID
  $CI_REPOSITORY_URL="Z:\Gitlab\tests\test"
  $env:CI_REPOSITORY_URL=$CI_REPOSITORY_URL
  $CI_PROJECT_ID="1"
  $env:CI_PROJECT_ID=$CI_PROJECT_ID
  $CI_PROJECT_DIR="Z:\Gitlab\tests\test\builds\0\project-1"
  $env:CI_PROJECT_DIR=$CI_PROJECT_DIR
  $CI_SERVER="yes"
  $env:CI_SERVER=$CI_SERVER
  $CI_SERVER_NAME="GitLab CI"
  $env:CI_SERVER_NAME=$CI_SERVER_NAME
  $CI_SERVER_VERSION=""
  $env:CI_SERVER_VERSION=$CI_SERVER_VERSION
  $CI_SERVER_REVISION=""
  $env:CI_SERVER_REVISION=$CI_SERVER_REVISION
  $GITLAB_CI="true"
  $env:GITLAB_CI=$GITLAB_CI
  $GIT_SSL_CAINFO=""
  New-Item -ItemType directory -Force -Path "C:\GitLab-Runner\builds\0\project-1.tmp" | out-null
  $GIT_SSL_CAINFO | Out-File "C:\GitLab-Runner\builds\0\project-1.tmp\GIT_SSL_CAINFO"
  $GIT_SSL_CAINFO="C:\GitLab-Runner\builds\0\project-1.tmp\GIT_SSL_CAINFO"
  $env:GIT_SSL_CAINFO=$GIT_SSL_CAINFO
  $CI_SERVER_TLS_CA_FILE=""
  New-Item -ItemType directory -Force -Path "C:\GitLab-Runner\builds\0\project-1.tmp" | out-null
  $CI_SERVER_TLS_CA_FILE | Out-File "C:\GitLab-Runner\builds\0\project-1.tmp\CI_SERVER_TLS_CA_FILE"
  $CI_SERVER_TLS_CA_FILE="C:\GitLab-Runner\builds\0\project-1.tmp\CI_SERVER_TLS_CA_FILE"
  $env:CI_SERVER_TLS_CA_FILE=$CI_SERVER_TLS_CA_FILE
  cd "C:\GitLab-Runner\builds\0\project-1"
  if(!$?) { Exit $LASTEXITCODE }

  echo "`$ echo true"
  echo true
}
if(!$?) { Exit $LASTEXITCODE }

& {
  $CI="true"
  $env:CI=$CI
  $CI_COMMIT_SHA="db45ad9af9d7af5e61b829442fd893d96e31250c"
  $env:CI_COMMIT_SHA=$CI_COMMIT_SHA
  $CI_COMMIT_BEFORE_SHA="d63117656af6ff57d99e50cc270f854691f335ad"
  $env:CI_COMMIT_BEFORE_SHA=$CI_COMMIT_BEFORE_SHA
  $CI_COMMIT_REF_NAME="main"
  $env:CI_COMMIT_REF_NAME=$CI_COMMIT_REF_NAME
  $CI_JOB_ID="1"
  $env:CI_JOB_ID=$CI_JOB_ID
  $CI_REPOSITORY_URL="Z:\Gitlab\tests\test"
  $env:CI_REPOSITORY_URL=$CI_REPOSITORY_URL
  $CI_PROJECT_ID="1"
  $env:CI_PROJECT_ID=$CI_PROJECT_ID
  $CI_PROJECT_DIR="Z:\Gitlab\tests\test\builds\0\project-1"
  $env:CI_PROJECT_DIR=$CI_PROJECT_DIR
  $CI_SERVER="yes"
  $env:CI_SERVER=$CI_SERVER
  $CI_SERVER_NAME="GitLab CI"
  $env:CI_SERVER_NAME=$CI_SERVER_NAME
  $CI_SERVER_VERSION=""
  $env:CI_SERVER_VERSION=$CI_SERVER_VERSION
  $CI_SERVER_REVISION=""
  $env:CI_SERVER_REVISION=$CI_SERVER_REVISION
  $GITLAB_CI="true"
  $env:GITLAB_CI=$GITLAB_CI
  $GIT_SSL_CAINFO=""
  New-Item -ItemType directory -Force -Path "C:\GitLab-Runner\builds\0\project-1.tmp" | out-null
  $GIT_SSL_CAINFO | Out-File "C:\GitLab-Runner\builds\0\project-1.tmp\GIT_SSL_CAINFO"
  $GIT_SSL_CAINFO="C:\GitLab-Runner\builds\0\project-1.tmp\GIT_SSL_CAINFO"
  $env:GIT_SSL_CAINFO=$GIT_SSL_CAINFO
  $CI_SERVER_TLS_CA_FILE=""
  New-Item -ItemType directory -Force -Path "C:\GitLab-Runner\builds\0\project-1.tmp" | out-null
  $CI_SERVER_TLS_CA_FILE | Out-File "C:\GitLab-Runner\builds\0\project-1.tmp\CI_SERVER_TLS_CA_FILE"
  $CI_SERVER_TLS_CA_FILE="C:\GitLab-Runner\builds\0\project-1.tmp\CI_SERVER_TLS_CA_FILE"
  $env:CI_SERVER_TLS_CA_FILE=$CI_SERVER_TLS_CA_FILE
  cd "C:\GitLab-Runner\builds\0\project-1"
  if(!$?) { Exit $LASTEXITCODE }

  echo "Archiving cache..."
  & "gitlab-runner-windows-amd64.exe" "archive" "--file" "..\..\..\cache\project-1\pages\main\cache.tgz" "--path" "vendor"
  if(!$?) { Exit $LASTEXITCODE }

}
if(!$?) { Exit $LASTEXITCODE }
```

### Running Windows Batch

You can execute Batch scripts from PowerShell using `Start-Process
"cmd.exe" "/c C:\Path\file.bat"` for old Batch scripts not ported to
PowerShell.

### Access `CMD` shell when PowerShell is the default

The [Call `CMD` From Default PowerShell in GitLab CI](https://gitlab.com/guided-explorations/microsoft/windows/call-cmd-from-powershell) project
demonstrates how to gain access to the `CMD` shell. This approach works when PowerShell is the default shell on a runner.

### Video walkthrough of working PowerShell examples

The [Slicing and Dicing with PowerShell on GitLab CI](https://www.youtube.com/watch?v=UZvtAYwruFc)
video is a walkthrough of the [PowerShell Pipelines on GitLab CI](https://gitlab.com/guided-explorations/microsoft/powershell/powershell-pipelines-on-gitlab-ci)
Guided Exploration project. It was tested on:

- Windows PowerShell and PowerShell Core 7 on [hosted runners on Windows for GitLab.com](https://docs.gitlab.com/ci/runners/hosted_runners/windows/).
- PowerShell Core 7 in Linux Containers with the [Docker-Machine runner](../executors/docker_machine.md).

The example can be copied to your own group or instance for testing. More details
on what other GitLab CI patterns are demonstrated are available at the project page.
