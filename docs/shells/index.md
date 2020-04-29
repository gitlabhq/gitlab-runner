# Shells supported by GitLab Runner

GitLab Runner implements a few shell script generators that allow to execute
builds on different systems.

## Overview

The shell scripts contain commands to execute all steps of the build:

1. `git clone`
1. Restore the build cache
1. Build commands
1. Update the build cache
1. Generate and upload the build artifacts

The shells don't have any configuration options. The build steps are received
from the commands defined in the [`script` directive in `.gitlab-ci.yml`][script].

The currently supported shells are:

| Shell         | Status             |  Description |
| --------------| ------------------ |  ----------- |
| `bash`        | Fully Supported    | Bash (Bourne-shell) shell. All commands executed in Bash context (default for all Unix systems) |
| `sh`          | Fully Supported    | Sh (Bourne-shell) shell. All commands executed in Sh context (fallback for `bash` for all Unix systems) |
| `powershell`  | Fully Supported    | Windows PowerShell script. All commands are executed in PowerShell context. Default when registering a new Runner in version 12.0 or newer. |

If you want to select a particular shell to use other than the default, you will need to [specify the shell](../executors/shell.md#selecting-your-shell) in your `config.toml` file.

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

## PowerShell

The default shell when a new Runner is registered using GitLab Runner
12.0 or newer.

Powershell is the default shell used on Windows when
[`shell`](../configuration/advanced-configuration.md#the-runners-section) is not
specified.

PowerShell doesn't support executing the build in context of another user.

The generated PowerShell script is executed by saving its content to a file and
passing the file name to the following command:

```batch
powershell -noprofile -noninteractive -executionpolicy Bypass -command generated-windows-powershell.ps1
```

This is how an example PowerShell script looks like:

```powershell
$ErrorActionPreference = "Continue"

echo "Running on $env:computername..."

& {
  $CI="true"
  $env:CI=$CI
  $CI_COMMIT_SHA="db45ad9af9d7af5e61b829442fd893d96e31250c"
  $env:CI_COMMIT_SHA=$CI_COMMIT_SHA
  $CI_COMMIT_BEFORE_SHA="d63117656af6ff57d99e50cc270f854691f335ad"
  $env:CI_COMMIT_BEFORE_SHA=$CI_COMMIT_BEFORE_SHA
  $CI_COMMIT_REF_NAME="master"
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
  md "C:\GitLab-Runner\builds\0\project-1.tmp" -Force | out-null
  $GIT_SSL_CAINFO | Out-File "C:\GitLab-Runner\builds\0\project-1.tmp\GIT_SSL_CAINFO"
  $GIT_SSL_CAINFO="C:\GitLab-Runner\builds\0\project-1.tmp\GIT_SSL_CAINFO"
  $env:GIT_SSL_CAINFO=$GIT_SSL_CAINFO
  $CI_SERVER_TLS_CA_FILE=""
  md "C:\GitLab-Runner\builds\0\project-1.tmp" -Force | out-null
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

  echo "Checking out db45ad9a as master..."
  & "git" "checkout" "db45ad9af9d7af5e61b829442fd893d96e31250c"
  if(!$?) { Exit $LASTEXITCODE }

  if(Test-Path "..\..\..\cache\project-1\pages\master\cache.tgz" -PathType Leaf) {
    echo "Restoring cache..."
    & "gitlab-runner-windows-amd64.exe" "extract" "--file" "..\..\..\cache\project-1\pages\master\cache.tgz"
    if(!$?) { Exit $LASTEXITCODE }

  } else {
    if(Test-Path "..\..\..\cache\project-1\pages\master\cache.tgz" -PathType Leaf) {
      echo "Restoring cache..."
      & "gitlab-runner-windows-amd64.exe" "extract" "--file" "..\..\..\cache\project-1\pages\master\cache.tgz"
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
  $CI_COMMIT_REF_NAME="master"
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
  md "C:\GitLab-Runner\builds\0\project-1.tmp" -Force | out-null
  $GIT_SSL_CAINFO | Out-File "C:\GitLab-Runner\builds\0\project-1.tmp\GIT_SSL_CAINFO"
  $GIT_SSL_CAINFO="C:\GitLab-Runner\builds\0\project-1.tmp\GIT_SSL_CAINFO"
  $env:GIT_SSL_CAINFO=$GIT_SSL_CAINFO
  $CI_SERVER_TLS_CA_FILE=""
  md "C:\GitLab-Runner\builds\0\project-1.tmp" -Force | out-null
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
  $CI_COMMIT_REF_NAME="master"
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
  md "C:\GitLab-Runner\builds\0\project-1.tmp" -Force | out-null
  $GIT_SSL_CAINFO | Out-File "C:\GitLab-Runner\builds\0\project-1.tmp\GIT_SSL_CAINFO"
  $GIT_SSL_CAINFO="C:\GitLab-Runner\builds\0\project-1.tmp\GIT_SSL_CAINFO"
  $env:GIT_SSL_CAINFO=$GIT_SSL_CAINFO
  $CI_SERVER_TLS_CA_FILE=""
  md "C:\GitLab-Runner\builds\0\project-1.tmp" -Force | out-null
  $CI_SERVER_TLS_CA_FILE | Out-File "C:\GitLab-Runner\builds\0\project-1.tmp\CI_SERVER_TLS_CA_FILE"
  $CI_SERVER_TLS_CA_FILE="C:\GitLab-Runner\builds\0\project-1.tmp\CI_SERVER_TLS_CA_FILE"
  $env:CI_SERVER_TLS_CA_FILE=$CI_SERVER_TLS_CA_FILE
  cd "C:\GitLab-Runner\builds\0\project-1"
  if(!$?) { Exit $LASTEXITCODE }

  echo "Archiving cache..."
  & "gitlab-runner-windows-amd64.exe" "archive" "--file" "..\..\..\cache\project-1\pages\master\cache.tgz" "--path" "vendor"
  if(!$?) { Exit $LASTEXITCODE }

}
if(!$?) { Exit $LASTEXITCODE }
```

[script]: http://doc.gitlab.com/ce/ci/yaml/README.html#script

### Running windows batch

TIP: **Tip:**
You can execute batch scripts from PowerShell using `Start-Process
"cmd.exe" "/c C:\Path\file.bat"` for old batch scripts not ported to
PowerShell.

## Video walkthrough of a working example

The [Slicing and Dicing with PowerShell on GitLab CI](https://www.youtube.com/watch?v=UZvtAYwruFc)
video is a walkthrough of the [PowerShell Pipelines on GitLab CI](https://gitlab.com/guided-explorations/gitlab-ci-yml-powershell-tips-tricks-and-hacks/powershell-pipelines-on-gitlab-ci)
Guided Exploration project. It was tested on:

- Windows PowerShell and PowerShell Core 7 on [GitLab's Windows shared runners](https://docs.gitlab.com/ee/user/gitlab_com/#windows-shared-runners-beta).
- PowerShell Core 7 in Linux Containers with the [Docker-Machine Runner](../executors/docker_machine.md).

The example can be copied to your own group or instance for testing. More details
on what other GitLab CI patterns are demonstrated are available at the project page.
