---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: GitLab Functions를 사용하기 위해 step runner를 수동으로 설치합니다
title: step runner를 수동으로 설치합니다
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

step runner는 러너가 기본 함수 지원이 없는 실행기에서 GitLab Functions를 실행할 수 있도록 해주는 바이너리입니다. 이러한 실행기의 경우, 파이프라인에서 함수를 사용하기 전에 작업이 실행되는 호스트 또는 컨테이너에 step runner 바이너리를 설치해야 합니다.

## 수동으로 단계 러너 설치가 필요한 실행기 {#executors-that-require-manual-step-runner-installation}

러너를 수동으로 설치해야 하는지는 실행기에 따라 달라집니다. 다음 표는 러너를 수동으로 설치해야 하는 실행기를 보여줍니다:

| 실행기          | 수동 설치 필요 |
|-------------------|------------------------------|
| 셸             | 예                          |
| SSH               | 예                          |
| Kubernetes        | 예                          |
| VirtualBox        | 예                          |
| Parallels         | 예                          |
| Custom            | 예                          |
| Instance          | 예                          |
| Docker            | Windows에서만              |
| Docker Autoscaler | Windows에서만              |
| Docker Machine    | Windows에서만              |

수동 설치가 필요 없는 실행기의 경우 `gitlab-runner-helper`이 단계 러너로 작동합니다. `step-runner` 바이너리는 이러한 실행기에 있지 않으며 필수가 아닙니다.

### 변수 액세스 제한 {#variable-access-restrictions}

러너를 수동으로 설치하는 실행기에서는 단계 러너가 작업 변수 및 환경 변수에 대한 제한된 액세스 권한을 갖습니다:

| 구문               | 사용 가능한 값                                                                                                                                                                        |
|----------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `${{ vars.<name> }}` | `CI_`, `DOCKER_` 또는 `GITLAB_` 접두사가 있는 작업 변수만 해당됩니다.                                                                                                                      |
| `${{ env.<name> }}`  | `HTTPS_PROXY`, `HTTP_PROXY`, `NO_PROXY`, `http_proxy`, `https_proxy`, `no_proxy`, `all_proxy`, `LANG`, `LC_ALL`, `LC_CTYPE`, `LOGNAME`, `USER`, `PATH`, `SHELL`, `TERM`, `TMPDIR`, `TZ` |

## 단계 러너 수동으로 설치 {#install-step-runner-manually}

여러 플랫폼용 사전 컴파일된 바이너리는 [단계 러너 릴리스 페이지](https://gitlab.com/gitlab-org/step-runner/-/releases)에서 사용할 수 있습니다. 지원되는 플랫폼에는 여러 아키텍처(amd64, arm64, 386, arm, s390x, ppc64le)에 걸쳐 Windows, Linux, macOS 및 FreeBSD가 포함됩니다.

### 바이너리의 진정성 확인 {#verify-authenticity-of-the-binary}

설치하기 전에 바이너리가 변조되지 않았으며 공식 GitLab 팀에서 제공되었는지 확인합니다.

1. GPG 공개 키를 다운로드하고 가져오기:

   ```shell
   # All platforms (requires gpg installed: https://gnupg.org/download/)
   curl -o step-runner.pub.gpg "https://gitlab.com/gitlab-org/step-runner/-/package_files/257922684/download"
   gpg --import step-runner.pub.gpg
   gpg --fingerprint
   ```

   가져온 키가 다음과 일치하는지 확인하세요:

   | 키 속성 | 값                                                |
   |---------------|------------------------------------------------------|
   | 이름          | `GitLab, Inc.`                                       |
   | 이메일         | `support@gitlab.com`                                 |
   | 지문   | `0FCD 59B1 6F4A 62D0 3839  27A5 42FF CA71 62A5 35F5` |
   | 만료        | `2029-01-05`                                         |

1. [릴리스 페이지](https://gitlab.com/gitlab-org/step-runner/-/releases)에서 다음 파일을 다운로드하세요:

   - 플랫폼에 맞는 바이너리(예: `step-runner-linux-amd64` 또는 `step-runner-darwin-arm64`)
   - `step-runner-release.sha256`
   - `step-runner-release.sha256.asc`

1. GPG 서명 확인:

   ```shell
   # All platforms (requires gpg)
   gpg --verify step-runner-release.sha256.asc step-runner-release.sha256
   ```

   출력에 `Good signature` 메시지가 포함되어야 합니다.

1. 바이너리 체크섬 확인:

   ```shell
   # Linux
   sha256sum -c step-runner-release.sha256
   ```

   ```shell
   # macOS
   shasum -a 256 -c step-runner-release.sha256
   ```

   ```shell
   # Windows (PowerShell) — replace 'step-runner-windows-amd64.exe' with your binary name
   $binary = "step-runner-windows-amd64.exe"
   $expected = (Select-String -Path "step-runner-release.sha256" -Pattern $binary).Line.Split(" ")[0]
   $actual = (Get-FileHash -Algorithm SHA256 $binary).Hash.ToLower()
   if ($actual -eq $expected) { "OK" } else { "FAILED: checksum mismatch" }
   ```

   출력에는 바이너리에 대해 `OK`이 표시되어야 합니다.

### PATH에 단계 러너 추가 {#add-step-runner-to-path}

바이너리를 다운로드하고 확인한 후 작업이 실행되는 인스턴스의 `PATH`에서 사용할 수 있도록 만듭니다. 이 인스턴스는 실행기에 따라 호스트 머신 또는 컨테이너일 수 있습니다.

1. 바이너리의 이름을 `step-runner`(또는 Windows에서 `step-runner.exe`)로 변경합니다:

   ```shell
   mv step-runner-<os>-<arch> step-runner
   ```

1. Unix와 유사한 시스템에서 바이너리를 실행 가능하게 만듭니다:

   ```shell
   chmod +x step-runner
   ```

1. 바이너리를 `PATH`의 디렉터리로 이동합니다:

   ```shell
   mv step-runner /usr/local/bin/
   ```
