---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: GitLab Runner가 지원하는 셸의 유형
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

GitLab Runner는 다양한 시스템에서 빌드를 실행할 수 있게 해주는 셸 스크립트 생성기를 구현합니다.

셸 스크립트에는 빌드의 모든 단계를 실행하는 명령이 포함됩니다:

1. `git clone`
1. 빌드 캐시 복원
1. 빌드 명령
1. 빌드 캐시 업데이트
1. 빌드 아티팩트 생성 및 업로드

셸에는 구성 옵션이 없습니다. 빌드 단계는 [`script` 지시문이 `.gitlab-ci.yml`에서 정의한 명령으로부터 수신됩니다](https://docs.gitlab.com/ci/yaml/#script).

지원되는 셸은 다음과 같습니다:

| 셸        | 상태          | 설명 |
|--------------|-----------------|-------------|
| `bash`       | 완전히 지원됨 | Bash (Bourne Again Shell). 모든 명령이 Bash 컨텍스트에서 실행됩니다(모든 Unix 시스템의 기본값). |
| `sh`         | 완전히 지원됨 | Sh (Bourne shell). 모든 명령이 Sh 컨텍스트에서 실행됩니다(모든 Unix 시스템의 `bash`에 대한 폴백). |
| `powershell` | 완전히 지원됨 | PowerShell 스크립트. 모든 명령이 PowerShell Desktop 컨텍스트에서 실행됩니다. `kubernetes` 및 `docker-windows` 실행기를 사용하여 Windows의 작업을 위한 기본 셸. |
| `pwsh`       | 완전히 지원됨 | PowerShell 스크립트. 모든 명령이 PowerShell Core 컨텍스트에서 실행됩니다. Windows에서 새 러너 등록을 위한 기본 셸이며, `shell` 실행기를 사용하는 작업의 경우입니다. |

기본값 이외의 특정 셸을 사용하려면 [셸 지정](../executors/shell.md#selecting-your-shell)을 `config.toml` 파일에서 해야 합니다.

## Sh/Bash 셸 {#shbash-shells}

Sh/Bash는 모든 Unix 기반 시스템에서 사용되는 기본 셸입니다. `.gitlab-ci.yml`에서 사용되는 bash 스크립트는 셸 스크립트를 다음 명령 중 하나로 파이핑하여 실행됩니다:

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

### 셸 프로필 로드 {#shell-profile-loading}

특정 실행기의 경우 러너는 위에 표시된 `--login` 플래그를 전달하며, 이는 셸 프로필도 로드합니다. `.bashrc`, `.bash_logout`, [또는 다른 모든 dotfile](https://tldp.org/LDP/Bash-Beginners-Guide/html/sect_03_01.html#sect_03_01_02)에 있는 모든 것은 작업에서 실행됩니다.

[작업이 `Prepare environment` 단계에서 실패](../faq/_index.md#job-failed-system-failure-preparing-environment)하면, 셸 프로필의 무언가가 실패를 야기하고 있을 가능성이 높습니다. 일반적인 실패는 콘솔을 지우려고 하는 `.bash_logout`이 있을 때입니다.

이 오류를 해결하려면 `/home/gitlab-runner/.bash_logout`을 확인하세요. 예를 들어, `.bash_logout` 파일에 다음과 같은 스크립트 섹션이 있으면 이를 주석 처리하고 파이프라인을 다시 시작하세요:

```shell
if [ "$SHLVL" = 1 ]; then
    [ -x /usr/bin/clear_console ] && /usr/bin/clear_console -q
fi
```

셸 프로필을 로드하는 실행기:

- [`shell`](../executors/shell.md)
- [`parallels`](../executors/parallels.md) (대상 가상 머신의 셸 프로필이 로드됨)
- [`virtualbox`](../executors/virtualbox.md) (대상 가상 머신의 셸 프로필이 로드됨)
- [`ssh`](../executors/ssh.md) (대상 머신의 셸 프로필이 로드됨)

## PowerShell {#powershell}

PowerShell Core는 Windows에서 새 러너 등록을 위한 기본 셸입니다. 그러나 이 등록 기본값은 `shell` 값을 `config.toml`에서 명시적으로 설정할 때만 적용됩니다. `shell`가 구성되지 않은 경우:

- `docker-windows` 및 `kubernetes` 실행기는 런타임에 PowerShell Desktop으로 기본 설정됩니다.
- `shell` 실행기는 PowerShell Core로 기본 설정됩니다.

PowerShell은 다른 사용자의 컨텍스트에서 빌드를 실행하는 것을 지원하지 않습니다.

생성된 PowerShell 스크립트는 해당 내용을 파일에 저장하고 파일 이름을 다음 명령으로 전달하여 실행됩니다:

- PowerShell Desktop Edition의 경우:

  ```batch
  powershell -NoProfile -NonInteractive -ExecutionPolicy Bypass -Command generated-windows-powershell.ps1
  ```

- PowerShell Core Edition의 경우:

  ```batch
  pwsh -NoProfile -NonInteractive -ExecutionPolicy Bypass -Command generated-windows-powershell.ps1
  ```

다음은 PowerShell 스크립트의 예입니다:

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

### Windows Batch 실행 {#running-windows-batch}

PowerShell에서 `Start-Process
"cmd.exe" "/c C:\Path\file.bat"`을 사용하여 Batch 스크립트를 실행할 수 있습니다(PowerShell로 포팅되지 않은 오래된 Batch 스크립트의 경우).

### `CMD` 셸에 접근할 수 있습니다(PowerShell이 기본값일 때) {#access-cmd-shell-when-powershell-is-the-default}

[Call `CMD` From Default PowerShell in GitLab CI](https://gitlab.com/guided-explorations/microsoft/windows/call-cmd-from-powershell) 파이프라인은 `CMD` 셸에 접근하는 방법을 보여줍니다. 이 방법은 PowerShell이 러너의 기본 셸일 때 작동합니다.

### 작동하는 PowerShell 예제의 비디오 연습 {#video-walkthrough-of-working-powershell-examples}

[Slicing and Dicing with PowerShell on GitLab CI](https://www.youtube.com/watch?v=UZvtAYwruFc) 비디오는 [PowerShell 파이프라인s on GitLab CI](https://gitlab.com/guided-explorations/microsoft/powershell/powershell-pipelines-on-gitlab-ci) Guided Exploration 파이프라인의 연습입니다. 다음에서 테스트되었습니다:

- Windows PowerShell 및 PowerShell Core 7 on [hosted 러너s on Windows for GitLab.com](https://docs.gitlab.com/ci/runners/hosted_runners/windows/).
- PowerShell Core 7 in Linux Containers with the [Docker-Machine 러너](../executors/docker_machine.md).

테스트를 위해 예제를 자신의 그룹 또는 인스턴스로 복사할 수 있습니다. 다른 GitLab CI 파이프라인 패턴이 무엇인지에 대한 자세한 내용은 파이프라인 페이지에서 확인할 수 있습니다.
