---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: Windows 시스템에 GitLab 러너를 설치합니다.
title: Windows에 GitLab 러너 설치
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

Windows에서 GitLab 러너를 설치하고 실행하려면 다음이 필요합니다:

- [공식 사이트](https://git-scm.com/download/win)에서 설치할 수 있는 Git
- 기본 제공 시스템 계정이 아닌 사용자 계정으로 실행하려는 경우 사용자 계정의 비밀번호
- 문자 인코딩 문제를 방지하기 위해 시스템 로캘을 English (United States)로 설정합니다. 자세한 내용은 [이슈 38702](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/38702)를 참조하세요.

## 설치 {#installation}

1. 시스템의 어딘가에 폴더를 생성합니다. 예를 들어 `C:\GitLab-Runner`입니다.
1. [x86 64비트](https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-windows-amd64.exe) , [ARM 64비트](https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-windows-arm64.exe) 또는 [x86 32비트](https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-windows-386.exe) 바이너리를 다운로드하여 생성한 폴더에 넣습니다. 다음은 바이너리의 이름을 `gitlab-runner.exe`로 변경했다고 가정합니다(선택 사항). [출시 버전 다운로드 - 다른 태그가 지정된 릴리스 다운로드](bleeding-edge.md#download-any-other-tagged-release)에 설명된 대로 사용 가능한 모든 버전의 바이너리를 다운로드할 수 있습니다.
1. GitLab 러너 디렉터리 및 실행 파일에서 `Write` 권한을 제한해야 합니다. 이러한 권한을 설정하지 않으면 일반 사용자가 실행 파일을 자신의 파일로 바꾸고 상승된 권한으로 임의의 코드를 실행할 수 있습니다.
1. [관리자 권한 명령 프롬프트](https://learn.microsoft.com/en-us/powershell/scripting/windows-powershell/starting-windows-powershell?view=powershell-7.4#with-administrative-privileges-run-as-administrator)를 실행합니다:
1. [러너 등록](../register/_index.md)합니다.
1. GitLab 러너를 서비스로 설치하고 시작합니다. 기본 제공 시스템 계정(권장) 또는 사용자 계정을 사용하여 서비스를 실행할 수 있습니다.

   > [!note]
   > Windows 서비스는 대화형 데스크톱 세션을 제공하지 않습니다. GUI 또는 데스크톱 자동화 테스트를 실행하려면 [GUI 테스트 및 대화형 데스크톱 세션](#gui-tests-and-interactive-desktop-sessions)을 참조하세요.

   **Run service using Built-in System Account** (1단계에서 생성된 예제 디렉터리 아래, `C:\GitLab-Runner`)

   ```powershell
   cd C:\GitLab-Runner
   .\gitlab-runner.exe install
   .\gitlab-runner.exe start
   ```

   **Run service using user account** (1단계에서 생성된 예제 디렉터리 아래, `C:\GitLab-Runner`)

   Windows에서 서비스를 시작하는 데 필요하므로 현재 사용자 계정에 대해 올바른 비밀번호를 입력해야 합니다:

   ```powershell
   cd C:\GitLab-Runner
   .\gitlab-runner.exe install --user ENTER-YOUR-USERNAME --password ENTER-YOUR-PASSWORD
   .\gitlab-runner.exe start
   ```

   GitLab 러너 설치 중에 오류가 발생하면 [문제 해결 섹션](#windows-troubleshooting)을 참조하세요.

1. (선택 사항) `C:\GitLab-Runner\config.toml`에서 러너의 `concurrent` 값을 업데이트하여 [고급 구성 세부 사항](../configuration/advanced-configuration.md)에 설명된 대로 여러 동시 작업을 허용합니다. 또한 고급 구성 세부 사항을 사용하여 셸 실행기를 Batch 대신 Bash 또는 PowerShell을 사용하도록 업데이트할 수 있습니다.

완료되었습니다! 러너가 설치되고 실행 중이며 각 시스템 재부팅 후 다시 시작됩니다. 로그는 Windows 이벤트 로그에 저장됩니다.

## 업그레이드 {#upgrade}

1. 서비스를 중지합니다([관리자 권한 명령 프롬프트](https://learn.microsoft.com/en-us/powershell/scripting/windows-powershell/starting-windows-powershell?view=powershell-7.4#with-administrative-privileges-run-as-administrator)가 필요함):

   ```powershell
   cd C:\GitLab-Runner
   .\gitlab-runner.exe stop
   ```

1. [x86 64비트](https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-windows-amd64.exe) , [ARM 64비트](https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-windows-arm64.exe) 또는 [x86 32비트](https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-windows-386.exe) 바이너리를 다운로드하여 러너의 실행 파일을 바꿉니다. [출시 버전 다운로드 - 다른 태그가 지정된 릴리스 다운로드](bleeding-edge.md#download-any-other-tagged-release)에 설명된 대로 사용 가능한 모든 버전의 바이너리를 다운로드할 수 있습니다.

1. 서비스를 시작합니다:

   ```powershell
   .\gitlab-runner.exe start
   ```

## 제거 {#uninstall}

[관리자 권한 명령 프롬프트](https://learn.microsoft.com/en-us/powershell/scripting/windows-powershell/starting-windows-powershell?view=powershell-7.4#with-administrative-privileges-run-as-administrator)에서:

```powershell
cd C:\GitLab-Runner
.\gitlab-runner.exe stop
.\gitlab-runner.exe uninstall
cd ..
rmdir /s GitLab-Runner
```

## Windows 문제 해결 {#windows-troubleshooting}

GitLab 러너와 관련된 가장 일반적인 문제를 설명하는 [FAQ](../faq/_index.md) 섹션을 읽어야 합니다.

_계정 이름이 유효하지 않음_ 같은 오류가 발생하면 다음을 시도하세요:

```powershell
# Add \. before the username
.\gitlab-runner.exe install --user ".\ENTER-YOUR-USERNAME" --password "ENTER-YOUR-PASSWORD"
```

서비스를 시작하는 동안 `The service did not start due to a logon failure` 오류가 발생하면 [FAQ 섹션](#error-the-service-did-not-start-due-to-a-logon-failure)을 참조하여 문제를 해결하는 방법을 확인하세요.

Windows 비밀번호가 없으면 GitLab 러너 서비스를 시작할 수 없지만 기본 제공 시스템 계정을 사용할 수 있습니다.

기본 제공 시스템 계정 문제의 경우 Microsoft 지원 웹 사이트에서 [기본 제공 시스템 계정으로 시작되도록 서비스 구성](https://learn.microsoft.com/en-us/troubleshoot/windows-server/system-management-components/service-startup-permissions#resolution-3-configure-the-service-to-start-up-with-the-built-in-system-account)을 참조하세요.

### 러너 로그 가져오기 {#get-runner-logs}

`.\gitlab-runner.exe install`를 실행하면 `gitlab-runner`가 Windows 서비스로 설치됩니다. 이벤트 뷰어에서 공급자 이름 `gitlab-runner`으로 로그를 찾을 수 있습니다.

GUI에 액세스할 수 없으면 PowerShell에서 [`Get-WinEvent`](https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.diagnostics/get-winevent?view=powershell-7.4)를 실행할 수 있습니다.

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

### GUI 테스트 및 대화형 데스크톱 세션 {#gui-tests-and-interactive-desktop-sessions}

Windows GUI 테스트 도구(예: Ranorex 및 데스크톱 자동화 프레임워크)는 표시되는 데스크톱에 액세스할 수 있는 대화형 사용자 세션이 필요합니다. 이는 알려진 플랫폼 제한 사항입니다. 자세한 내용은 [이슈 1046](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/1046)을 참조하세요.

GitLab 러너가 Windows 서비스로만 실행되는 경우:

- 작업이 비대화형 세션에서 실행됩니다.
- 작업이 표시되는 데스크톱에 액세스할 수 없습니다.
- GUI 테스트가 실패하거나 중단됩니다.

GUI 또는 데스크톱 자동화 테스트를 실행하려면:

1. `shell` 실행기를 사용합니다.

   Windows의 Docker 및 Kubernetes 실행기는 대화형 데스크톱 세션을 제공하지 않습니다.

1. 대화형 세션에 대한 사용자 계정으로 Windows에 로그인합니다.
1. 서비스를 사용하는 대신 해당 세션에서 GitLab 러너를 포그라운드 프로세스로 시작합니다:

   ```powershell
   cd C:\GitLab-Runner
   .\gitlab-runner.exe run
   ```

1. GUI 테스트가 실행되는 동안 사용자 세션을 활성 상태로 유지합니다.
1. `.gitlab-ci.yml` 파일에서 태그를 사용하여 GUI 테스트 작업을 이 러너로 보냅니다:

   ```yaml
   gui_tests:
     stage: test
     tags:
       - windows-gui
     script:
       - .\run-gui-tests.ps1
   ```

자동 확장 또는 임시 Windows 러너는 대화형 데스크톱 세션을 지원하지 않으므로 GUI 테스트를 실행할 수 없습니다. 각 작업이 로그인한 사용자가 없는 새로 프로비저닝된 VM에서 실행되므로 GUI 자동화가 대상으로 할 표시되는 데스크톱이 없습니다.

### Windows에서 빌드 중에 `PathTooLongException`가 발생합니다 {#i-get-a-pathtoolongexception-during-my-builds-on-windows}

이 오류는 경로가 260자 이상인 디렉터리 구조를 생성하는 `npm` 같은 도구로 인해 발생합니다. 문제를 해결하려면 다음 해결책 중 하나를 채택하세요.

- `core.longpaths`이(가) 활성화된 Git 사용:

  Git을 사용하여 디렉터리 구조를 정리하면 문제를 방지할 수 있습니다.

  1. 명령줄에서 `git config --system core.longpaths true`을(를) 실행합니다.
  1. GitLab CI 프로젝트 설정 페이지에서 `git fetch`을(를) 사용하도록 프로젝트를 설정합니다.

- PowerShell용 NTFSSecurity 도구 사용:

  [NTFSSecurity](https://github.com/raandree/NTFSSecurity) PowerShell 모듈은 긴 경로를 지원하는 `Remove-Item2` 메서드를 제공합니다. GitLab 러너는 사용 가능한 경우를 감지하고 자동으로 사용합니다.

> GitLab 러너 16.9.1에서 도입된 회귀는 GitLab 러너 17.10.0에서 수정되었습니다. 회귀가 있는 GitLab 러너 버전을 사용하려는 경우 다음 해결책 중 하나를 사용하세요:
>
> - `pre_get_sources_script`을(를) 사용하여 Git 시스템 수준 설정을 다시 활성화합니다(`Git_CONFIG_NOSYSTEM` 설정 해제). 이 작업은 Windows에서 기본적으로 `core.longpaths`을(를) 활성화합니다.
>
>   ```yaml
>   build:
>     hooks:
>       pre_get_sources_script:
>         - $env:GIT_CONFIG_NOSYSTEM=''
>   ```
>
> - 사용자 지정 `GitLab-runner-helper` 이미지 빌드:
>
>   ```dockerfile
>   FROM registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:x86_64-v17.8.3-servercore21H2
>   ENV GIT_CONFIG_NOSYSTEM=
>   ```

### Windows 배치 스크립트 오류: `The system cannot find the batch label specified - buildscript` {#error-with-windows-batch-scripts-the-system-cannot-find-the-batch-label-specified---buildscript}

`.gitlab-ci.yml` 파일의 배치 파일 줄 앞에 `call`을(를) 붙여야 `call C:\path\to\test.bat`처럼 보이게 합니다. 예를 들어:

```yaml
before_script:
  - call C:\path\to\test.bat
```

자세한 내용은 [이슈 1025](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/1025)를 참조하세요.

### 웹 터미널에서 색상 출력을 얻으려면 어떻게 해야 합니까? {#how-can-i-get-colored-output-on-the-web-terminal}

**Short answer**:

프로그램 출력에 ANSI 색상 코드가 있는지 확인하세요. 텍스트 형식 지정 목적으로 UNIX ANSI 터미널 에뮬레이터에서 실행 중이라고 가정합니다(웹 인터페이스 출력이기 때문).

**Long Answer**:

GitLab CI용 웹 인터페이스는 UNIX ANSI 터미널을 에뮬레이션합니다(최소한 부분적으로). `gitlab-runner`은(는) 빌드의 모든 출력을 웹 인터페이스로 직접 파이프합니다. 즉, 있는 모든 ANSI 색상 코드가 적용됩니다.

이전 버전의 Windows 명령 프롬프트 터미널(Windows 10 버전 1511 이전)은 ANSI 색상 코드를 지원하지 않습니다. 대신 win32([`ANSI.SYS`](https://en.wikipedia.org/wiki/ANSI.SYS)) 호출을 사용하며, 이는 표시될 문자열에 **not**. 크로스 플랫폼 프로그램을 작성할 때 개발자는 일반적으로 기본적으로 ANSI 색상 코드를 사용합니다. 이러한 코드는 Windows 시스템에서 실행할 때 win32 호출로 변환되며, 예를 들어 [Colorama](https://pypi.org/project/colorama/)입니다.

프로그램이 위의 작업을 수행하는 경우 CI 빌드에 대해 해당 변환을 비활성화하여 ANSI 코드가 문자열에 유지되도록 해야 합니다.

자세한 내용은 [GitLab CI YAML 설명서](https://docs.gitlab.com/ci/yaml/script/#add-color-codes-to-script-output) 에서 PowerShell을 사용한 예제를 참조하고 [이슈 332](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/332)를 참조하세요.

### 오류: `The service did not start due to a logon failure` {#error-the-service-did-not-start-due-to-a-logon-failure}

Windows에 GitLab 러너 서비스를 설치하고 시작할 때 이러한 오류가 발생할 수 있습니다:

```shell
gitlab-runner install --password WINDOWS_MACHINE_PASSWORD
gitlab-runner start
FATA[0000] Failed to start GitLab Runner: The service did not start due to a logon failure.
```

이 오류는 서비스를 실행하는 데 사용되는 사용자가 `SeServiceLogonRight` 권한을 갖지 않은 경우 발생할 수 있습니다. 이 경우 선택된 사용자에 대해 이 권한을 추가한 다음 서비스를 다시 시작해야 합니다.

1. **Control Panel > System and Security > Administrative Tools**로 이동합니다.
1. **Local Security Policy** 도구를 엽니다.
1. 왼쪽의 목록에서 **Security Settings > Local Policies > User Rights Assignment**을 선택합니다.
1. 오른쪽의 목록에서 **Log on as a service**을 엽니다.
1. **Add User or Group...**를 선택합니다.
1. 사용자를 추가하고(**Advanced...**을 사용하여 수동으로 또는) 설정을 적용합니다.

[Microsoft 설명서](https://learn.microsoft.com/en-us/previous-versions/windows/it-pro/windows-server-2012-R2-and-2012/dn221981(v=ws.11))에 따르면 다음에서 작동합니다:

- Windows Vista
- Windows Server 2008
- Windows 7
- Windows 8.1
- Windows Server 2008 R2
- Windows Server 2012 R2
- Windows Server 2012
- Windows 8

로컬 보안 정책 도구는 일부 Windows 버전, 예를 들어 각 버전의 "Home Edition" 변형에서 사용할 수 없습니다.

서비스 구성에 사용되는 사용자에 대해 `SeServiceLogonRight`을(를) 추가한 후 명령 `gitlab-runner start`이(가) 오류 없이 완료되고 서비스가 제대로 시작되어야 합니다.

### 작업이 성공 또는 실패로 잘못 표시됨 {#job-marked-as-success-or-failed-incorrectly}

대부분의 Windows 프로그램은 성공을 위해 `exit code 0`을(를) 출력합니다. 그러나 일부 프로그램은 종료 코드를 반환하지 않거나 성공을 위해 다른 값을 갖습니다. 예는 Windows 도구 `robocopy`입니다. 다음 `.gitlab-ci.yml`은(는) `robocopy`에서 출력된 종료 코드로 인해 성공해야 함에도 불구하고 실패합니다:

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

위의 경우 `script:`에 종료 코드 검사를 수동으로 추가해야 합니다. 예를 들어 PowerShell 스크립트를 만들 수 있습니다:

```powershell
$exitCodes = 0,1

robocopy ./source ./dest

if ( $exitCodes.Contains($LastExitCode) ) {
    exit 0
} else {
    exit 1
}
```

`.gitlab-ci.yml` 파일을 다음과 같이 변경합니다:

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

PowerShell 함수를 사용할 때 `return`과(와) `exit` 사이의 차이에 주의하세요. `exit 1`이(가) 작업을 실패로 표시하는 동안 `return 1`은(는) 그렇지 않습니다.

### Kubernetes 실행기를 사용하여 작업이 중간에 성공하고 종료됨 {#job-marked-as-success-and-terminated-midway-using-kubernetes-executor}

자세한 내용은 [작업 실행](../executors/kubernetes/_index.md#job-execution)을 참조하세요.

### Docker 실행기: `unsupported Windows Version` {#docker-executor-unsupported-windows-version}

GitLab 러너는 Windows Server의 버전을 확인하여 지원되는지 확인합니다.

`docker info`을(를) 실행하여 이 작업을 수행합니다.

GitLab 러너가 시작에 실패하고 Windows Server 버전을 지정하지 않고 오류를 표시하면 Docker 버전이 오래되었을 수 있습니다.

```plaintext
Preparation failed: detecting base image: unsupported Windows Version: Windows Server Datacenter
```

오류는 Windows Server 버전에 대한 자세한 정보를 포함해야 하며, GitLab 러너가 지원하는 버전과 비교됩니다.

```plaintext
unsupported Windows Version: Windows Server Datacenter Version (OS Build 18363.720)
```

Docker 17.06.2는 Windows Server의 `docker info` 출력에서 다음을 반환합니다.

```plaintext
Operating System: Windows Server Datacenter
```

이 경우 수정 사항은 Windows Server 릴리스와 유사하거나 이후 연령대의 Docker 버전으로 업그레이드하는 것입니다.

### Kubernetes 실행기: `unsupported Windows Version` {#kubernetes-executor-unsupported-windows-version}

Windows의 Kubernetes 실행기는 다음 오류로 실패할 수 있습니다:

```plaintext
Using Kubernetes namespace: gitlab-runner
ERROR: Preparation failed: prepare helper image: detecting base image: unsupported Windows Version:
Will be retried in 3s ...
ERROR: Job failed (system failure): prepare helper image: detecting base image: unsupported Windows Version:
```

이를 수정하려면 GitLab 러너 구성 파일의 `[runners.kubernetes.node_selector]` 섹션에서 `node.kubernetes.io/windows-build` 노드 선택기를 추가합니다. 예를 들어:

```toml
   [runners.kubernetes.node_selector]
     "kubernetes.io/arch" = "amd64"
     "kubernetes.io/os" = "windows"
     "node.kubernetes.io/windows-build" = "10.0.17763"
```

### 매핑된 네트워크 드라이브를 사용 중이며 빌드가 올바른 경로를 찾을 수 없습니다 {#im-using-a-mapped-network-drive-and-my-build-cannot-find-the-correct-path}

GitLab 러너가 관리자 계정이 아닌 표준 사용자 계정으로 실행되면 매핑된 네트워크 드라이브에 액세스할 수 없습니다. 매핑된 네트워크 드라이브를 사용하려고 하면 `The system cannot find the path specified.` 오류가 발생합니다. 이 오류는 서비스 로그온 세션이 리소스에 액세스할 때 [보안 제한 사항](https://learn.microsoft.com/en-us/windows/win32/services/services-and-redirected-drives)을 갖기 때문에 발생합니다. 대신 드라이브의 [UNC 경로](https://learn.microsoft.com/en-us/dotnet/standard/io/file-path-formats#unc-paths)를 사용합니다.

### 빌드 컨테이너가 서비스 컨테이너에 연결할 수 없습니다 {#the-build-container-is-unable-to-connect-to-service-containers}

Windows 컨테이너에서 서비스를 사용하려면:

- [각 작업에 대한 네트워크를 만드는](../executors/docker.md#create-a-network-for-each-job) 네트워킹 모드를 사용합니다.
- `FF_NETWORK_PER_BUILD` 기능 플래그가 활성화되어 있는지 확인합니다.

### 작업이 빌드 디렉터리를 만들 수 없고 오류로 실패합니다 {#the-job-cannot-create-a-build-directory-and-fails-with-an-error}

`GitLab-Runner`과(와) `Docker-Windows` 실행기를 사용하면 작업이 다음과 같은 오류로 실패할 수 있습니다:

```shell
fatal: cannot chdir to c:/builds/gitlab/test: Permission denied`
```

이 오류가 발생하면 Docker 엔진이 실행 중인 사용자가 `C:\Program Data\Docker`에 대한 전체 권한을 갖는지 확인합니다. Docker 엔진은 특정 작업을 위해 이 디렉터리에 쓸 수 있어야 하며, 올바른 권한이 없으면 실패합니다.

[Windows에서 Docker Engine 구성에 대해 자세히 알아보기](https://learn.microsoft.com/en-us/virtualization/windowscontainers/manage-docker/configure-docker-daemon).

### 작업 로그의 Windows Subsystem for Linux(WSL) STDOUT 출력에 대한 빈 줄 {#blank-lines-for-windows-subsystem-for-linux-wsl-stdout-output-in-job-logs}

기본적으로 Windows Subsystem for Linux(WSL)의 STDOUT 출력은 UTF8로 인코딩되지 않으며 작업 로그에 빈 줄로 표시됩니다. STDOUT 출력을 표시하려면 `WSL_UTF8` 환경 변수를 설정하여 WSL에 대해 UTF8 인코딩을 강제할 수 있습니다.

```yaml
job:
  variables:
    WSL_UTF8: "1"
```

### 표시 해상도는 1024x768로 제한됩니다 {#display-resolution-is-limited-to-1024x768}

GitLab 러너를 시스템 서비스로 사용하여 Windows에서 CI/CD 작업을 실행하면 표시 해상도가 1024x768로 제한됩니다. 이 문제는 Windows Session 0 격리로 인해 발생합니다. 자세한 내용은 [Session 0 격리](https://learn.microsoft.com/en-us/previous-versions/bb756986(v=msdn.10)?redirectedfrom=MSDN)를 참조하세요.

세션 및 표시 해상도를 확인하려면 작업에서 다음 PowerShell 스크립트를 실행하세요:

```powershell
echo "Current session:"
[System.Diagnostics.Process]::GetCurrentProcess().SessionId

Add-Type -AssemblyName System.Windows.Forms
[System.Windows.Forms.Screen]::AllScreens
```

격리된 세션 0에서 실행할 때의 스크립트 출력은 다음과 같습니다:

```plaintext
Current session:
0
BitsPerPixel : 0
Bounds       : {X=0,Y=0,Width=1024,Height=768}
DeviceName   : WinDisc
Primary      : True
WorkingArea  : {X=0,Y=0,Width=1024,Height=768}
```
