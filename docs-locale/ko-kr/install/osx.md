---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: "Apple Silicon 및 Intel x86-64 시스템에서 사용자 모드 서비스로 GitLab Runner를 다운로드, 설치 및 구성합니다."
title: macOS에 GitLab Runner 설치
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

Apple Silicon 또는 Intel x86-64 시스템에서 macOS에 GitLab Runner를 설치합니다. GitLab 자체는 일반적으로 컨테이너 또는 가상 머신에서 로컬로 또는 원격으로 실행됩니다.

## macOS 서비스 모드 {#macos-service-modes}

macOS에서 GitLab Runner는 사용자 모드 `LaunchAgent`로 실행되며, 시스템 수준 `LaunchDaemon`로 실행되지 않습니다. 이것이 유일한 지원되는 모드입니다.

사용자 모드에서 러너는:

- 현재 인증된 사용자로 실행되며, root로 실행되지 않습니다.
- 해당 사용자가 로그인할 때 시작되고 로그아웃할 때 중지됩니다.
- 사용자의 키체인 및 UI 세션에 액세스할 수 있으며, iOS 시뮬레이터를 실행하고 코드 서명을 수행하는 데 필요합니다.
- 구성을 `~/.gitlab-runner/config.toml`에 저장합니다.

시스템 수준 `LaunchDaemon`은 부팅 시 시작되고 root로 실행되며 사용자 세션에 액세스할 수 없습니다. GitLab Runner는 `LaunchDaemon`로 실행되는 것을 지원하지 않습니다.

재부팅 후 러너를 계속 사용할 수 있도록 macOS 머신에서 자동 로그인을 켭니다.

## GitLab Runner 설치 {#install-gitlab-runner}

macOS에서 GitLab Runner를 설치하여 Apple Silicon 또는 Intel x86-64 시스템에서 CI/CD 작업을 실행합니다.

전제 조건:

- 작업을 실행하는 사용자 계정으로 macOS 머신에 로그인해야 합니다. 이 절차를 위해 SSH 세션을 사용하지 마세요. 로컬 GUI 터미널을 사용하세요.

GitLab Runner를 설치하려면:

1. 시스템에 맞는 바이너리를 다운로드하세요:

   - Intel (x86-64):

     ```shell
     sudo curl --output /usr/local/bin/gitlab-runner \
       "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-darwin-amd64"
     ```

   - Apple Silicon:

     ```shell
     sudo curl --output /usr/local/bin/gitlab-runner \
       "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-darwin-arm64"
     ```

   특정 태그가 지정된 릴리스의 바이너리를 다운로드하려면 [다른 태그가 지정된 릴리스 다운로드](bleeding-edge.md#download-any-other-tagged-release)를 참조하세요.

1. 바이너리를 실행 가능하게 만드세요:

   ```shell
   sudo chmod +x /usr/local/bin/gitlab-runner
   ```

1. [러너 등록](../register/_index.md) 구성을 합니다. iOS 및 macOS 빌드의 경우 [셸 실행기](../executors/shell.md)를 사용하세요. 보안 세부 정보는 [셸 실행기 보안](../security/_index.md#usage-of-shell-executor)을 참조하세요.

1. GitLab Runner 서비스를 설치하고 시작합니다:

   ```shell
   cd ~
   gitlab-runner install
   gitlab-runner start
   ```

1. 시스템을 재부팅합니다.

`gitlab-runner install` 명령은 `~/Library/LaunchAgents/gitlab-runner.plist`에 `LaunchAgent` plist를 생성하고 `launchctl`에 등록합니다. 오류가 발생하면 [문제 해결](#troubleshooting)을 참조하세요.

## 구성 파일 위치 {#configuration-file-locations}

| 파일                 | 경로                                             |
|----------------------|--------------------------------------------------|
| 구성        | `~/.gitlab-runner/config.toml`                   |
| `LaunchAgent` plist  | `~/Library/LaunchAgents/gitlab-runner.plist`     |
| 표준 출력 로그  | `~/Library/Logs/gitlab-runner.out.log`           |
| 표준 오류 로그   | `~/Library/Logs/gitlab-runner.err.log`           |

구성 옵션에 대한 자세한 내용은 [고급 구성](../configuration/advanced-configuration.md)을 참조하세요.

## GitLab Runner 업그레이드 {#upgrade-gitlab-runner}

GitLab Runner를 최신 버전으로 업그레이드하려면:

1. 서비스를 중지합니다:

   ```shell
   gitlab-runner stop
   ```

1. GitLab Runner 실행 파일을 대체할 바이너리를 다운로드합니다:

   - Intel (x86-64):

     ```shell
     sudo curl -o /usr/local/bin/gitlab-runner \
       "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-darwin-amd64"
     ```

   - Apple Silicon:

     ```shell
     sudo curl -o /usr/local/bin/gitlab-runner \
       "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-darwin-arm64"
     ```

   특정 태그가 지정된 릴리스의 바이너리를 다운로드하려면 [다른 태그가 지정된 릴리스 다운로드](bleeding-edge.md#download-any-other-tagged-release)를 참조하세요.

1. 바이너리를 실행 가능하게 만드세요:

   ```shell
   sudo chmod +x /usr/local/bin/gitlab-runner
   ```

1. 서비스를 시작합니다:

   ```shell
   gitlab-runner start
   ```

## 서비스 파일 업그레이드 {#upgrade-the-service-file}

`LaunchAgent` 구성을 업그레이드하려면 서비스를 제거했다가 다시 설치합니다:

```shell
gitlab-runner uninstall
gitlab-runner install
gitlab-runner start
```

## GitLab Runner에서 `codesign` 사용 {#use-codesign-with-gitlab-runner}

GitLab Runner를 Homebrew로 설치했는데 빌드가 `codesign`을 호출하는 경우, 사용자 키체인에 액세스하려면 `<key>SessionCreate</key><true/>`을 설정해야 할 수도 있습니다.

> [!note]
> GitLab은 Homebrew 공식을 유지하지 않습니다. 공식 바이너리를 사용하여 GitLab Runner를 설치하세요.

다음 예제에서 러너는 `gitlab` 사용자로 빌드를 실행하고 해당 사용자의 서명 인증서에 액세스해야 합니다:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
  <dict>
    <key>SessionCreate</key><true/>
    <key>KeepAlive</key>
    <dict>
      <key>SuccessfulExit</key>
      <false/>
    </dict>
    <key>RunAtLoad</key><true/>
    <key>Disabled</key><false/>
    <key>Label</key>
    <string>com.gitlab.gitlab-runner</string>
    <key>UserName</key>
    <string>gitlab</string>
    <key>GroupName</key>
    <string>staff</string>
    <key>ProgramArguments</key>
    <array>
      <string>/usr/local/opt/gitlab-runner/bin/gitlab-runner</string>
      <string>run</string>
      <string>--working-directory</string>
      <string>/Users/gitlab/gitlab-runner</string>
      <string>--config</string>
      <string>/Users/gitlab/gitlab-runner/config.toml</string>
      <string>--service</string>
      <string>gitlab-runner</string>
      <string>--syslog</string>
    </array>
    <key>EnvironmentVariables</key>
    <dict>
      <key>PATH</key>
      <string>/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
    </dict>
  </dict>
</plist>
```

## 문제 해결 {#troubleshooting}

macOS에 GitLab Runner를 설치할 때 다음과 같은 문제가 발생할 수 있습니다.

일반적인 문제 해결은 [GitLab Runner 문제 해결](../faq/_index.md)을 참조하세요.

### 오류: `killed: 9` {#error-killed-9}

Apple Silicon에서 `gitlab-runner install`, `gitlab-runner start`, 또는 `gitlab-runner register` 명령을 실행할 때 이 오류가 발생할 수 있습니다.

이 오류를 해결하려면 `~/Library/LaunchAgents/gitlab-runner.plist`의 `StandardOutPath` 및 `StandardErrorPath`에 대한 디렉터리가 존재하고 쓰기 가능한지 확인합니다. 예를 들어:

```xml
<key>StandardErrorPath</key>
<string>/Users/<username>/gitlab-runner-log/gitlab-runner.err.log</string>
<key>StandardOutPath</key>
<string>/Users/<username>/gitlab-runner-log/gitlab-runner.out.log</string>
```

### 오류: `"launchctl" failed: Could not find domain for` {#error-launchctl-failed-could-not-find-domain-for}

이 오류는 로컬 GUI 터미널 대신 SSH를 통해 GitLab Runner 서비스를 관리할 때 발생합니다.

이 오류를 해결하려면 macOS 머신에서 직접 터미널 애플리케이션을 열고 그곳에서 `install` 및 `start` 명령을 실행합니다.

### 오류: `Failed to authorize rights (0x1) with status: -60007` {#error-failed-to-authorize-rights-0x1-with-status--60007}

이 오류에는 두 가지 가능한 원인이 있습니다.

사용자 계정에 개발자 도구 액세스 권한이 없습니다. 액세스 권한을 부여하려면:

```shell
DevToolsSecurity -enable
sudo security authorizationdb remove system.privilege.taskport is-developer
```

또는 `LaunchAgent` plist에 `SessionCreate`이 `true`로 설정되어 있습니다. 이 문제를 해결하려면 서비스를 다시 설치합니다:

```shell
gitlab-runner uninstall
gitlab-runner install
gitlab-runner start
```

`~/Library/LaunchAgents/gitlab-runner.plist`이 이제 `SessionCreate`이 `false`으로 설정되었는지 확인합니다.

### 오류: `Failed to connect to path port 3000: Operation timed out` {#error-failed-to-connect-to-path-port-3000-operation-timed-out}

러너가 GitLab 인스턴스에 연결할 수 없습니다. 연결을 차단할 수 있는 방화벽, 프록시, 라우팅 구성 또는 권한 문제가 있는지 확인합니다.

### 오류: `FATAL: Failed to start gitlab-runner: exit status 134` {#error-fatal-failed-to-start-gitlab-runner-exit-status-134}

이 오류는 GitLab Runner 서비스가 올바르게 설치되지 않았음을 나타냅니다.

이 오류를 해결하려면 서비스를 다시 설치합니다:

```shell
gitlab-runner uninstall
gitlab-runner install
gitlab-runner start
```

오류가 지속되면 SSH를 사용하는 대신 macOS GUI 데스크톱에 로그인하고 그곳의 터미널에서 명령을 실행합니다. `LaunchAgent`은 부팅하기 위해 그래픽 로그인 세션이 필요합니다.

AWS의 macOS 인스턴스의 경우 [AWS 설명서](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/connect-to-mac-instance.html)를 따라 GUI에 연결한 다음 해당 세션의 터미널에서 다시 시도합니다.

### 오류: `launchctl failed: Load failed: 5: Input/output error` {#error-launchctl-failed-load-failed-5-inputoutput-error}

`gitlab-runner start` 명령을 실행할 때 이 오류가 발생하면 먼저 러너가 이미 실행 중인지 확인합니다:

```shell
gitlab-runner status
```

러너가 실행 중이 아닌 경우, `~/Library/LaunchAgents/gitlab-runner.plist`의 `StandardOutPath` 및 `StandardErrorPath`에 대한 디렉터리가 존재하고 러너의 사용자 계정이 읽고 쓸 수 있는 액세스 권한을 가지고 있는지 확인합니다. 그런 다음 러너를 시작합니다:

```shell
gitlab-runner start
```

### 오류: `couldn't build CA Chain` {#error-couldnt-build-ca-chain}

이 오류는 GitLab Runner v15.5.0으로 업그레이드한 후에 발생할 수 있습니다. 전체 오류 메시지는 다음과 같습니다:

```plaintext
ERROR: Error on fetching TLS Data from API response... error  error=couldn't build CA Chain:
error while fetching certificates from TLS ConnectionState: error while fetching certificates
into the CA Chain: couldn't resolve certificates chain from the leaf certificate: error while
resolving certificates chain with verification: error while verifying last certificate from
the chain: x509: "Baltimore CyberTrust Root" certificate is not permitted for this usage
runner=x7kDEc9Q
```

이 오류를 해결하려면:

1. GitLab Runner v15.5.1 이상으로 업그레이드합니다.
1. 업그레이드할 수 없는 경우 [`[runners.feature_flags]` 구성](../configuration/feature-flags.md#enable-feature-flag-in-runner-configuration)에서 `FF_RESOLVE_FULL_TLS_CHAIN`을 `false`로 설정합니다:

   ```toml
   [[runners]]
     name = "example-runner"
     url = "https://gitlab.com/"
     token = "TOKEN"
     executor = "docker"
     [runners.feature_flags]
       FF_RESOLVE_FULL_TLS_CHAIN = false
   ```

### Homebrew Git 자격 증명 헬퍼로 인해 가져오기가 중단됨 {#homebrew-git-credential-helper-causes-fetches-to-hang}

Homebrew에서 Git을 설치한 경우 `/usr/local/etc/gitconfig`에 `credential.helper = osxkeychain` 항목을 추가했을 수 있습니다. 이는 macOS 키체인에 자격 증명을 캐시하고 `git fetch`이 중단될 수 있습니다.

시스템 전체에서 자격 증명 헬퍼를 제거하려면:

```shell
git config --system --unset credential.helper
```

GitLab Runner 사용자만 비활성화하려면:

```shell
git config --global --add credential.helper ''
```

현재 설정을 확인하려면:

```shell
git config credential.helper
```
