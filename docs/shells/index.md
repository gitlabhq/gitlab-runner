---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/product/ux/technical-writing/#assignments
comments: false
---

# Types of shells supported by GitLab Runner **(FREE)**

GitLab Runner implements shell script generators that allow executing
builds on different systems.

The shell scripts contain commands to execute all steps of the build:

1. `git clone`
1. Restore the build cache
1. Build commands
1. Update the build cache
1. Generate and upload the build artifacts

The shells don't have any configuration options. The build steps are received
from the commands defined in the [`script` directive in `.gitlab-ci.yml`](https://docs.gitlab.com/ee/ci/yaml/index.html#script).

The supported shells are:

| Shell         | Status             |  Description |
| --------------| ------------------ |  ----------- |
| `bash`        | Fully Supported    | Bash (Bourne Again Shell). All commands executed in Bash context (default for all Unix systems) |
| `sh`          | Fully Supported    | Sh (Bourne shell). All commands executed in Sh context (fallback for `bash` for all Unix systems) |
| `powershell`  | Fully Supported    | PowerShell script. All commands are executed in PowerShell Desktop context. In GitLab Runner 12.0-13.12, this is the default when registering a new runner. |
| `pwsh`        | Fully Supported    | PowerShell script. All commands are executed in PowerShell Core context. In GitLab Runner 14.0 and later, this is the default when registering a new runner. |
| `cmd`         | Deprecated         | Windows Batch script. All commands are executed in Batch context. Deprecated in favor of PowerShell Core. Default when no [`shell`](../configuration/advanced-configuration.md#the-runners-section) is specified. [Learn how to gain access to the CMD shell when PowerShell is the default shell](#access-cmd-shell-when-powershell-is-the-default). |

If you want to select a particular shell to use other than the default, you must [specify the shell](../executors/shell.md#selecting-your-shell) in your `config.toml` file.

## Sh/Bash shells

This is the default shell used on all Unix based systems. The bash script used
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

If a [job fails on the `Prepare environment`](../faq/index.md#job-failed-system-failure-preparing-environment) stage, it
is likely that something in the shell profile is causing the failure. A common
failure is when you have a `.bash_logout` that tries to clear the console.

Executors that load shell profiles:

- [`shell`](../executors/shell.md)
- [`parallels`](../executors/parallels.md)  (The shell profile of the *target* virtual machine is loaded)
- [`virtualbox`](../executors/virtualbox.md)  (The shell profile of the *target* virtual machine is loaded)
- [`ssh`](../executors/ssh.md) (The shell profile of the *target* machine is loaded)
- [`docker-ssh`](../executors/docker.md#docker-vs-docker-ssh-and-dockermachine-vs-docker-sshmachine) (The shell profile of the *target* machine is loaded)
- [`docker-ssh+machine`](../executors/docker.md#docker-vs-docker-ssh-and-dockermachine-vs-docker-sshmachine) (The shell profile of the *target* machine is loaded)

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

This is what an example PowerShell script looks like:

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

## Windows Batch

NOTE:
In GitLab 11.11, we announced the [deprecation](https://about.gitlab.com/handbook/product/#deprecated)
of the Windows Batch executor, `cmd` shell, for the GitLab Runner in favor
of [PowerShell](#powershell). The `cmd` shell remains included in future versions
of GitLab Runner however, any new feature for Windows is to be tested and supported
only for use with PowerShell. Only critical bugs and regressions to the `cmd` shell
will be investigated and fixed.

You can execute Batch scripts from PowerShell using `Start-Process
"cmd.exe" "/c C:\Path\file.bat"` for old Batch scripts not ported to
PowerShell.

Windows Batch is the default shell used on Windows when
[`shell`](../configuration/advanced-configuration.md#the-runners-section) is not
specified.

It doesn't support executing the build in context of another user.

The generated Batch script is executed by saving its content to file and
passing the filename to the following command:

```batch
cmd /Q /C generated-windows-batch.cmd
```

This is what an example Batch script looks like:

```batch
@echo off
setlocal enableextensions
setlocal enableDelayedExpansion
set nl=^


echo Running on %COMPUTERNAME%...

call :prescript
IF !errorlevel! NEQ 0 exit /b !errorlevel!

call :buildscript
IF !errorlevel! NEQ 0 exit /b !errorlevel!

call :postscript
IF !errorlevel! NEQ 0 exit /b !errorlevel!

goto :EOF
:prescript
SET CI=true
SET CI_COMMIT_SHA=db45ad9af9d7af5e61b829442fd893d96e31250c
SET CI_COMMIT_BEFORE_SHA=d63117656af6ff57d99e50cc270f854691f335ad
SET CI_COMMIT_REF_NAME=main
SET CI_JOB_ID=1
SET CI_REPOSITORY_URL=http://gitlab.example.com/group/project.git
SET CI_PROJECT_ID=1
SET CI_PROJECT_DIR=Z:\Gitlab\tests\test\builds\0\project-1
SET CI_SERVER=yes
SET CI_SERVER_NAME=GitLab CI
SET CI_SERVER_VERSION=
SET CI_SERVER_REVISION=
SET GITLAB_CI=true
md "C:\\GitLab-Runner\\builds\\0\\project-1.tmp" 2>NUL 1>NUL
echo multiline!nl!tls!nl!chain > C:\GitLab-Runner\builds\0\project-1.tmp\GIT_SSL_CAINFO
SET GIT_SSL_CAINFO=C:\GitLab-Runner\builds\0\project-1.tmp\GIT_SSL_CAINFO
md "C:\\GitLab-Runner\\builds\\0\\project-1.tmp" 2>NUL 1>NUL
echo multiline!nl!tls!nl!chain > C:\GitLab-Runner\builds\0\project-1.tmp\CI_SERVER_TLS_CA_FILE
SET CI_SERVER_TLS_CA_FILE=C:\GitLab-Runner\builds\0\project-1.tmp\CI_SERVER_TLS_CA_FILE
echo Cloning repository...
rd /s /q "C:\GitLab-Runner\builds\0\project-1" 2>NUL 1>NUL
"git" "clone" "http://gitlab.example.com/group/project.git" "Z:\Gitlab\tests\test\builds\0\project-1"
IF !errorlevel! NEQ 0 exit /b !errorlevel!

cd /D "C:\GitLab-Runner\builds\0\project-1"
IF !errorlevel! NEQ 0 exit /b !errorlevel!

echo Checking out db45ad9a as main...
"git" "checkout" "db45ad9af9d7af5e61b829442fd893d96e31250c"
IF !errorlevel! NEQ 0 exit /b !errorlevel!

IF EXIST "..\..\..\cache\project-1\pages\main\cache.tgz" (
  echo Restoring cache...
  "gitlab-runner-windows-amd64.exe" "extract" "--file" "..\..\..\cache\project-1\pages\main\cache.tgz"
  IF !errorlevel! NEQ 0 exit /b !errorlevel!

) ELSE (
  IF EXIST "..\..\..\cache\project-1\pages\main\cache.tgz" (
    echo Restoring cache...
    "gitlab-runner-windows-amd64.exe" "extract" "--file" "..\..\..\cache\project-1\pages\main\cache.tgz"
    IF !errorlevel! NEQ 0 exit /b !errorlevel!

  )
)
goto :EOF

:buildscript
SET CI=true
SET CI_COMMIT_SHA=db45ad9af9d7af5e61b829442fd893d96e31250c
SET CI_COMMIT_BEFORE_SHA=d63117656af6ff57d99e50cc270f854691f335ad
SET CI_COMMIT_REF_NAME=main
SET CI_JOB_ID=1
SET CI_REPOSITORY_URL=Z:\Gitlab\tests\test
SET CI_PROJECT_ID=1
SET CI_PROJECT_DIR=Z:\Gitlab\tests\test\builds\0\project-1
SET CI_SERVER=yes
SET CI_SERVER_NAME=GitLab CI
SET CI_SERVER_VERSION=
SET CI_SERVER_REVISION=
SET GITLAB_CI=true
md "C:\\GitLab-Runner\\builds\\0\\project-1.tmp" 2>NUL 1>NUL
echo multiline!nl!tls!nl!chain > C:\GitLab-Runner\builds\0\project-1.tmp\GIT_SSL_CAINFO
SET GIT_SSL_CAINFO=C:\GitLab-Runner\builds\0\project-1.tmp\GIT_SSL_CAINFO
md "C:\\GitLab-Runner\\builds\\0\\project-1.tmp" 2>NUL 1>NUL
echo multiline!nl!tls!nl!chain > C:\GitLab-Runner\builds\0\project-1.tmp\CI_SERVER_TLS_CA_FILE
SET CI_SERVER_TLS_CA_FILE=C:\GitLab-Runner\builds\0\project-1.tmp\CI_SERVER_TLS_CA_FILE
cd /D "C:\GitLab-Runner\builds\0\project-1"
IF !errorlevel! NEQ 0 exit /b !errorlevel!

echo $ echo true
echo true
goto :EOF

:postscript
SET CI=true
SET CI_COMMIT_SHA=db45ad9af9d7af5e61b829442fd893d96e31250c
SET CI_COMMIT_BEFORE_SHA=d63117656af6ff57d99e50cc270f854691f335ad
SET CI_COMMIT_REF_NAME=main
SET CI_JOB_ID=1
SET CI_REPOSITORY_URL=Z:\Gitlab\tests\test
SET CI_PROJECT_ID=1
SET CI_PROJECT_DIR=Z:\Gitlab\tests\test\builds\0\project-1
SET CI_SERVER=yes
SET CI_SERVER_NAME=GitLab CI
SET CI_SERVER_VERSION=
SET CI_SERVER_REVISION=
SET GITLAB_CI=true
md "C:\\GitLab-Runner\\builds\\0\\project-1.tmp" 2>NUL 1>NUL
echo multiline!nl!tls!nl!chain > C:\GitLab-Runner\builds\0\project-1.tmp\GIT_SSL_CAINFO
SET GIT_SSL_CAINFO=C:\GitLab-Runner\builds\0\project-1.tmp\GIT_SSL_CAINFO
md "C:\\GitLab-Runner\\builds\\0\\project-1.tmp" 2>NUL 1>NUL
echo multiline!nl!tls!nl!chain > C:\GitLab-Runner\builds\0\project-1.tmp\CI_SERVER_TLS_CA_FILE
SET CI_SERVER_TLS_CA_FILE=C:\GitLab-Runner\builds\0\project-1.tmp\CI_SERVER_TLS_CA_FILE
cd /D "C:\GitLab-Runner\builds\0\project-1"
IF !errorlevel! NEQ 0 exit /b !errorlevel!

echo Archiving cache...
"gitlab-runner-windows-amd64.exe" "archive" "--file" "..\..\..\cache\project-1\pages\main\cache.tgz" "--path" "vendor"
IF !errorlevel! NEQ 0 exit /b !errorlevel!

goto :EOF
```

### Access CMD shell when PowerShell is the default

The project: [Call CMD From Default PowerShell in GitLab CI](https://gitlab.com/guided-explorations/microsoft/windows/call-cmd-from-powershell) demonstrates how to gain access to the CMD shell when PowerShell is the default shell on a runner.

### Video walkthrough of working PowerShell examples

The [Slicing and Dicing with PowerShell on GitLab CI](https://www.youtube.com/watch?v=UZvtAYwruFc)
video is a walkthrough of the [PowerShell Pipelines on GitLab CI](https://gitlab.com/guided-explorations/gitlab-ci-yml-powershell-tips-tricks-and-hacks/powershell-pipelines-on-gitlab-ci)
Guided Exploration project. It was tested on:

- Windows PowerShell and PowerShell Core 7 on GitLab [Windows shared runners](https://docs.gitlab.com/ee/ci/runners/saas/windows_saas_runner.html).
- PowerShell Core 7 in Linux Containers with the [Docker-Machine runner](../executors/docker_machine.md).

The example can be copied to your own group or instance for testing. More details
on what other GitLab CI patterns are demonstrated are available at the project page.
