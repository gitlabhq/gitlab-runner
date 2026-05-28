---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: VirtualBox
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

> [!note]
> Parallels 실행기는 VirtualBox 실행기와 동일하게 작동합니다. 로컬 캐시는 지원되지 않습니다. [분산 캐시](../configuration/speed_up_job_execution.md)는 지원됩니다.

VirtualBox를 통해 VirtualBox의 가상화를 사용하여 매 빌드마다 깨끗한 빌드 환경을 제공할 수 있습니다. 이 실행기는 VirtualBox에서 실행할 수 있는 모든 시스템을 지원합니다. 유일한 요구 사항은 가상 머신이 SSH 서버를 노출하고 Bash 또는 PowerShell과 호환되는 셸을 제공하는 것입니다.

> [!note]
> GitLab 러너가 VirtualBox 실행기를 사용하는 모든 가상 머신에서 [공통 필수 조건](_index.md#git-requirements-for-non-docker-executors)을 충족하는지 확인하세요.

## 개요 {#overview}

프로젝트의 소스 코드는 `~/builds/<namespace>/<project-name>`로 체크아웃됩니다.

다음을 참조하세요:

- `<namespace>`는 GitLab에 프로젝트가 저장된 네임스페이스입니다
- `<project-name>`는 GitLab에 저장된 프로젝트의 이름입니다

`~/builds` 디렉토리를 재정의하려면 `[[runners]]` 섹션의 `builds_dir` 옵션을 지정하고 [`config.toml`](../configuration/advanced-configuration.md)에서 지정합니다.

[사용자 지정 빌드 디렉토리](https://docs.gitlab.com/ci/runners/configure_runners/#custom-build-directories)를 `GIT_CLONE_PATH`를 사용하여 작업별로 정의할 수도 있습니다.

## 새 기본 가상 머신 생성 {#create-a-new-base-virtual-machine}

1. [VirtualBox](https://www.virtualbox.org)를 설치합니다.
   - Windows에서 실행 중이고 VirtualBox가 기본 위치(예: `%PROGRAMFILES%\Oracle\VirtualBox`)에 설치되어 있으면 GitLab 러너가 자동으로 감지합니다. 그렇지 않으면 설치 폴더를 `gitlab-runner` 프로세스의 `PATH` 환경 변수에 추가해야 합니다.
1. VirtualBox에서 새 가상 머신을 가져오거나 생성합니다
1. 네트워크 어댑터 1을 "NAT"로 구성합니다(현재 GitLab 러너가 SSH를 통해 게스트에 연결할 수 있는 유일한 방법입니다)
1. (선택 사항) 다른 네트워크 어댑터를 "Bridged networking"으로 구성하여 게스트에서 인터넷에 액세스합니다(예: 하나)
1. 새 가상 머신에 로그인합니다
1. Windows VM인 경우 [Windows VM 체크리스트](#checklist-for-windows-vms)를 참조하세요
1. OpenSSH 서버를 설치합니다
1. 빌드에 필요한 다른 모든 종속성을 설치합니다
1. 작업 아티팩트를 다운로드하거나 업로드하려면 VM 내부에 `gitlab-runner`를 설치합니다
1. 로그아웃하고 가상 머신을 종료합니다

Vagrant와 같은 자동화 도구를 사용하여 가상 머신을 프로비저닝하는 것은 완전히 괜찮습니다.

## 새 러너 생성 {#create-a-new-runner}

1. VirtualBox를 실행 중인 호스트에 GitLab 러너를 설치합니다
1. `gitlab-runner register`로 새 러너를 등록합니다
1. `virtualbox` 실행기를 선택합니다
1. 이전에 생성한 기본 가상 머신의 이름을 입력합니다(가상 머신의 설정 **일반 > 기본 > 이름** 에서 찾을 수 있습니다)
1. SSH `user`과 `password` 또는 가상 머신의 `identity_file`로의 경로를 입력합니다

## 작동 방식 {#how-it-works}

새 빌드가 시작되면:

1. 가상 머신의 고유한 이름이 생성됩니다: `runner-<short-token>-concurrent-<id>`
1. 가상 머신이 존재하지 않으면 복제됩니다
1. SSH 서버에 액세스하기 위한 포트 포워딩 규칙이 생성됩니다
1. GitLab 러너가 가상 머신의 스냅샷을 시작하거나 복원합니다
1. GitLab 러너는 SSH 서버가 액세스 가능해질 때까지 기다립니다
1. GitLab 러너가 실행 중인 가상 머신의 스냅샷을 생성합니다(다음 빌드 속도를 높이기 위해 수행됨)
1. GitLab 러너가 가상 머신에 연결하고 빌드를 실행합니다
1. 사용 설정된 경우 작업 아티팩트 업로드는 `gitlab-runner` 바이너리를 사용하여 *내부* 가상 머신에서 수행됩니다.
1. GitLab 러너가 가상 머신을 중지하거나 종료합니다

## Windows VM 체크리스트 {#checklist-for-windows-vms}

VirtualBox를 Windows와 함께 사용하려면 Cygwin 또는 PowerShell을 설치할 수 있습니다.

### Cygwin 사용 {#use-cygwin}

- [Cygwin](https://cygwin.com/)을 설치합니다
- Cygwin에서 `sshd`과 Git을 설치합니다(*Git for Windows*는 사용하지 마세요. 많은 경로 문제가 발생할 수 있습니다!)
- Git LFS를 설치합니다
- `sshd`을 구성하고 서비스로 설정합니다([Cygwin wiki](https://cygwin.fandom.com/wiki/Sshd) 참조)
- Windows 방화벽에 포트 22의 수신 TCP 트래픽을 허용하는 규칙을 만듭니다
- GitLab 서버를 `~/.ssh/known_hosts`에 추가합니다
- Cygwin과 Windows 간의 경로를 변환하려면 [`cygpath` 유틸리티](https://cygwin.fandom.com/wiki/Cygpath_utility)를 사용합니다

### 기본 OpenSSH 및 PowerShell 사용 {#use-native-openssh-and-powershell}

- [PowerShell](https://learn.microsoft.com/en-us/powershell/scripting/install/install-powershell-on-windows?view=powershell-7.4)을 설치합니다
- [OpenSSH](https://learn.microsoft.com/en-us/windows-server/administration/openssh/openssh_install_firstuse?tabs=powershell#install-openssh-for-windows)를 설치하고 구성합니다
- [Git for Windows](https://git-scm.com/)를 설치합니다
- [`pwsh`을 기본 셸로 구성합니다](https://learn.microsoft.com/en-us/windows-server/administration/OpenSSH/openssh-server-configuration#configuring-the-default-shell-for-openssh-in-windows). 올바른 전체 경로로 예제를 업데이트합니다:

  ```powershell
  New-ItemProperty -Path "HKLM:\SOFTWARE\OpenSSH" -Name DefaultShell -Value "$PSHOME\pwsh.exe" -PropertyType String -Force
  ```

- 셸 `pwsh`을 [`config.toml`](../configuration/advanced-configuration.md)에 추가합니다
