---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: GitLab 러너 명령
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

GitLab 러너에는 러너를 등록, 관리하고 빌드를 실행하는 데 사용하는 명령 세트가 포함되어 있습니다.

다음을 실행하여 명령 목록을 확인할 수 있습니다:

```shell
gitlab-runner --help
```

`--help`를 명령 뒤에 추가하여 특정 도움말 페이지를 확인할 수 있습니다:

```shell
gitlab-runner <command> --help
```

## 환경 변수 사용 {#using-environment-variables}

대부분의 명령은 환경 변수를 지원하며, 이를 통해 명령에 설정을 전달할 수 있습니다.

특정 명령에 대해 `--help`를 호출할 때 환경 변수의 이름을 확인할 수 있습니다. 예를 들어 다음은 `run` 명령의 도움말 메시지입니다:

```shell
gitlab-runner run --help
```

출력은 다음과 유사합니다:

```plaintext
NAME:
   gitlab-runner run - run multi runner service

USAGE:
   gitlab-runner run [command options] [arguments...]

OPTIONS:
   -c, --config "/Users/ayufan/.gitlab-runner/config.toml"      Config file [$CONFIG_FILE]
```

## 디버그 모드에서 실행 {#running-in-debug-mode}

정의되지 않은 동작이나 오류의 원인을 찾고 있을 때는 디버그 모드를 사용하세요.

명령을 디버그 모드에서 실행하려면 명령 앞에 `--debug`를 추가하세요:

```shell
gitlab-runner --debug <command>
```

## 슈퍼유저 권한 {#super-user-permission}

GitLab 러너 설정에 접근하는 명령은 슈퍼유저(`root`)로 실행할 때 다르게 작동합니다. 파일 위치는 명령을 실행하는 사용자에 따라 다릅니다.

`gitlab-runner` 명령을 실행할 때 실행 중인 모드를 확인할 수 있습니다:

```shell
$ gitlab-runner run

INFO[0000] Starting multi-runner from /Users/ayufan/.gitlab-runner/config.toml ...  builds=0
WARN[0000] Running in user-mode.
WARN[0000] Use sudo for system-mode:
WARN[0000] $ sudo gitlab-runner...
```

작업하려는 모드가 확실한 경우 `user-mode`를 사용하세요. 그렇지 않으면 명령 앞에 `sudo`를 추가하세요:

```shell
$ sudo gitlab-runner run

INFO[0000] Starting multi-runner from /etc/gitlab-runner/config.toml ...  builds=0
INFO[0000] Running in system-mode.
```

Windows의 경우 명령 프롬프트를 관리자로 실행해야 할 수 있습니다.

## 설정 파일 {#configuration-file}

GitLab 러너 설정은 [TOML](https://github.com/toml-lang/toml) 형식을 사용합니다.

편집할 파일을 찾을 수 있습니다:

1. \*nix 시스템에서 GitLab 러너를 슈퍼유저(`root`)로 실행할 때: `/etc/gitlab-runner/config.toml`
1. \*nix 시스템에서 GitLab 러너를 root가 아닌 사용자로 실행할 때: `~/.gitlab-runner/config.toml`
1. 다른 시스템: `./config.toml`

대부분의 명령은 사용자 지정 설정 파일을 지정하는 인수를 허용하므로 단일 머신에 여러 개의 다른 설정을 둘 수 있습니다. 사용자 지정 설정 파일을 지정하려면 `-c` 또는 `--config` 플래그를 사용하거나 `CONFIG_FILE` 환경 변수를 사용하세요.

## 신호 {#signals}

시스템 신호를 사용하여 GitLab 러너와 상호 작용할 수 있습니다. 다음 명령은 다음 신호를 지원합니다:

| 명령             | 신호              | 작업 |
|---------------------|---------------------|--------|
| `register`          | `SIGINT`            | 러너 등록을 취소하고 이미 등록된 경우 삭제합니다. |
| `run`, `run-single` | `SIGINT`, `SIGTERM` | 실행 중인 모든 빌드를 중단하고 최대한 빨리 종료합니다. 두 번 사용하여 지금 종료합니다(**forceful shutdown**). |
| `run`, `run-single` | `SIGQUIT`           | 새로운 빌드 수락을 중단합니다. 실행 중인 빌드가 완료되면 종료합니다(**graceful shutdown**). |
| `run`               | `SIGHUP`            | 설정 파일을 다시 로드하도록 강제합니다. |

예를 들어 러너의 설정 파일을 강제로 다시 로드하려면 다음을 실행합니다:

```shell
sudo kill -SIGHUP <main_runner_pid>
```

[우아한 종료](#gitlab-runner-stop-doesnt-shut-down-gracefully)의 경우:

```shell
sudo kill -SIGQUIT <main_runner_pid>
```

> [!warning]
> **not** `killall` 또는 `pkill`를 `shell` 또는 `docker` 실행기를 사용할 때 우아한 종료에 사용합니다. 이는 하위 프로세스도 종료되므로 신호 처리가 제대로 되지 않을 수 있습니다. 작업을 처리하는 주 프로세스에만 사용합니다.

일부 운영 체제는 서비스가 실패할 때 자동으로 다시 시작하도록 구성되어 있습니다(일부 플랫폼에서는 기본값). 운영 체제가 이 설정을 가지고 있다면 위의 신호로 종료된 경우 러너를 자동으로 다시 시작할 수 있습니다.

## 명령 개요 {#commands-overview}

인수 없이 `gitlab-runner`를 실행하면 다음이 표시됩니다:

```plaintext
NAME:
   gitlab-runner - a GitLab Runner

USAGE:
   gitlab-runner [global options] command [command options] [arguments...]

VERSION:
   17.10.1 (ef334dcc)

AUTHOR:
   GitLab Inc. <support@gitlab.com>

COMMANDS:
   list                  List all configured runners
   run                   run multi runner service
   register              register a new runner
   reset-token           reset a runner's token
   install               install service
   uninstall             uninstall service
   start                 start service
   stop                  stop service
   restart               restart service
   status                get status of a service
   run-single            start single runner
   unregister            unregister specific runner
   verify                verify all registered runners
   wrapper               start multi runner service wrapped with gRPC manager server
   fleeting              manage fleeting plugins
   artifacts-downloader  download and extract build artifacts (internal)
   artifacts-uploader    create and upload build artifacts (internal)
   cache-archiver        create and upload cache artifacts (internal)
   cache-extractor       download and extract cache artifacts (internal)
   cache-init            changed permissions for cache paths (internal)
   health-check          check health for a specific address
   proxy-exec            execute internal commands (internal)
   read-logs             reads job logs from a file, used by kubernetes executor (internal)
   help, h               Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --cpuprofile value           write cpu profile to file [$CPU_PROFILE]
   --debug                      debug mode [$RUNNER_DEBUG]
   --log-format value           Choose log format (options: runner, text, json) [$LOG_FORMAT]
   --log-level value, -l value  Log level (options: debug, info, warn, error, fatal, panic) [$LOG_LEVEL]
   --help, -h                   show help
   --version, -v                print the version
```

다음에서는 각 명령이 자세히 수행하는 작업을 설명합니다.

## 등록 관련 명령 {#registration-related-commands}

다음 명령을 사용하여 새 러너를 등록하거나 등록되어 있는지 나열 및 확인합니다.

- [`gitlab-runner register`](#gitlab-runner-register)
  - [대화형 등록](#interactive-registration)
  - [비대화형 등록](#non-interactive-registration)
- [`gitlab-runner list`](#gitlab-runner-list)
- [`gitlab-runner verify`](#gitlab-runner-verify)
- [`gitlab-runner unregister`](#gitlab-runner-unregister)

이 명령은 다음 인수를 지원합니다:

| 매개변수  | 기본값                                                   | 설명 |
|------------|-----------------------------------------------------------|-------------|
| `--config` | [설정 파일 섹션](#configuration-file) 참조 | 사용할 사용자 지정 설정 파일을 지정합니다 |

### `gitlab-runner register` {#gitlab-runner-register}

이 명령은 GitLab [Runners API](https://docs.gitlab.com/api/runners/)를 사용하여 GitLab에서 러너를 등록합니다.

등록된 러너는 [설정 파일](#configuration-file)에 추가됩니다. GitLab 러너의 단일 설치에서 여러 설정을 사용할 수 있습니다. `gitlab-runner register`를 실행하면 새 설정 항목이 추가됩니다. 이전 항목을 제거하지 않습니다.

러너를 등록할 수 있습니다:

- 대화형으로
- 비대화형으로

> [!note]
> 러너는 GitLab [Runners API](https://docs.gitlab.com/api/runners/)를 사용하여 직접 등록할 수 있지만 설정은 자동으로 생성되지 않습니다.

#### 대화형 등록 {#interactive-registration}

이 명령은 일반적으로 대화형 모드(**기본값**)에서 사용됩니다. 러너 등록 중에 여러 질문을 받습니다.

등록 명령을 호출할 때 인수를 추가하여 이 질문을 미리 채울 수 있습니다:

```shell
gitlab-runner register --name my-runner --url "http://gitlab.example.com" --token my-authentication-token
```

또는 `register` 명령 전에 환경 변수를 설정하여 수행할 수 있습니다:

```shell
export CI_SERVER_URL=http://gitlab.example.com
export RUNNER_NAME=my-runner
export CI_SERVER_TOKEN=my-authentication-token
gitlab-runner register
```

가능한 모든 인수 및 환경을 확인하려면 실행하세요:

```shell
gitlab-runner register --help
```

#### 비대화형 등록 {#non-interactive-registration}

비대화형/무인 모드에서 등록을 사용할 수 있습니다.

등록 명령을 호출할 때 인수를 지정할 수 있습니다:

```shell
gitlab-runner register --non-interactive <other-arguments>
```

또는 `register` 명령 전에 환경 변수를 설정하여 수행할 수 있습니다:

```shell
<other-environment-variables>
export REGISTER_NON_INTERACTIVE=true
gitlab-runner register
```

> [!note]
> 부울 매개변수는 명령줄에 `--key={true|false}`로 전달해야 합니다.

#### `[[runners]]` 설정 템플릿 파일 {#runners-configuration-template-file}

{{< history >}}

- GitLab 러너 12.2에서 [도입](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4228)되었습니다.

{{< /history >}}

[설정 템플릿 파일](../register/_index.md#register-with-a-configuration-template) 기능을 사용하여 러너 등록 중에 추가 옵션을 설정할 수 있습니다.

### `gitlab-runner list` {#gitlab-runner-list}

이 명령은 [설정 파일](#configuration-file)에 저장된 모든 러너를 나열합니다.

### `gitlab-runner verify` {#gitlab-runner-verify}

이 명령은 등록된 러너가 GitLab에 연결할 수 있는지 확인합니다. 하지만 GitLab 러너 서비스에서 러너를 사용 중인지 확인하지는 않습니다. 출력 예시입니다:

```plaintext
Verifying runner... is alive                        runner=fee9938e
Verifying runner... is alive                        runner=0db52b31
Verifying runner... is alive                        runner=826f687f
Verifying runner... is alive                        runner=32773c0f
```

GitLab에서 제거된 오래된 러너를 제거하려면 다음 명령을 실행하세요.

> [!warning]
> 이 작업은 실행 취소할 수 없습니다. 설정 파일을 업데이트하므로 실행하기 전에 `config.toml`의 백업을 유지하세요.

```shell
gitlab-runner verify --delete
```

### `gitlab-runner unregister` {#gitlab-runner-unregister}

이 명령은 GitLab [Runners API](https://docs.gitlab.com/api/runners/#delete-a-runner)를 사용하여 등록된 러너를 등록 해제합니다.

다음 중 하나를 예상합니다:

- 전체 URL 및 러너 토큰
- 러너 이름

`--all-runners` 옵션을 사용하면 모든 연결된 러너를 등록 해제합니다.

> [!note]
> 러너는 GitLab [Runners API](https://docs.gitlab.com/api/runners/#delete-a-runner)로 등록 해제할 수 있지만 사용자를 위해 설정이 수정되지 않습니다.

- 러너 등록 토큰으로 생성된 경우 `gitlab-runner unregister`은 러너 인증 토큰으로 러너를 삭제합니다.
- GitLab UI 또는 Runners API로 생성된 경우 `gitlab-runner unregister`은 러너 인증 토큰으로 러너 관리자를 삭제하지만 러너는 삭제하지 않습니다. 러너를 완전히 제거하려면 [러너 관리 페이지에서 러너 삭제](https://docs.gitlab.com/ci/runners/runners_scope/#delete-instance-runners) 를 하거나 [`DELETE /runners`](https://docs.gitlab.com/api/runners/#delete-a-runner) REST API 엔드포인트를 사용합니다.

단일 러너를 등록 해제하려면 먼저 `gitlab-runner list`을 실행하여 러너의 세부 정보를 얻으세요:

```plaintext
test-runner     Executor=shell Token=t0k3n URL=http://gitlab.example.com
```

이 정보를 사용하여 다음 명령 중 하나를 사용하여 등록을 해제합니다.

> [!warning]
> 이 작업은 실행 취소할 수 없습니다. 설정 파일을 업데이트하므로 실행하기 전에 `config.toml`의 백업을 유지하세요.

#### URL 및 토큰별 {#by-url-and-token}

```shell
gitlab-runner unregister --url "http://gitlab.example.com/" --token t0k3n
```

#### 이름별 {#by-name}

```shell
gitlab-runner unregister --name test-runner
```

> [!note]
> 지정된 이름을 가진 러너가 둘 이상인 경우 첫 번째 항목만 제거됩니다.

#### 모든 러너 {#all-runners}

```shell
gitlab-runner unregister --all-runners
```

### `gitlab-runner reset-token` {#gitlab-runner-reset-token}

이 명령은 GitLab Runners API를 사용하여 러너 토큰을 재설정합니다(중 하나는 [러너 ID](https://docs.gitlab.com/api/runners/#reset-runners-authentication-token-by-using-the-runner-id) 또는 [현재 토큰](https://docs.gitlab.com/api/runners/#reset-runners-authentication-token-by-using-the-current-token)).

러너 이름(또는 URL 및 ID), 그리고 러너 ID로 재설정할 경우 선택적 개인 액세스 토큰을 예상합니다. 개인 액세스 토큰 및 러너 ID는 토큰이 이미 만료된 경우에 사용하기 위해 제공됩니다.

`--all-runners` 옵션을 사용하면 모든 연결된 러너의 토큰을 재설정합니다.

#### 러너 현재 토큰 포함 {#with-runners-current-token}

```shell
gitlab-runner reset-token --name test-runner
```

#### 개인 액세스 토큰 및 러너 이름 포함 {#with-pat-and-runner-name}

```shell
gitlab-runner reset-token --name test-runner --pat PaT
```

#### 개인 액세스 토큰, GitLab URL 및 러너 ID 포함 {#with-pat-gitlab-url-and-runner-id}

```shell
gitlab-runner reset-token --url "https://gitlab.example.com/" --id 12345 --pat PaT
```

#### 모든 러너 {#all-runners-1}

```shell
gitlab-runners reset-token --all-runners
```

## 서비스 관련 명령 {#service-related-commands}

다음 명령을 통해 러너를 시스템 또는 사용자 서비스로 관리할 수 있습니다. 이들을 사용하여 러너 서비스를 설치, 제거, 시작 및 중지합니다.

- [`gitlab-runner install`](#gitlab-runner-install)
- [`gitlab-runner uninstall`](#gitlab-runner-uninstall)
- [`gitlab-runner start`](#gitlab-runner-start)
- [`gitlab-runner stop`](#gitlab-runner-stop)
- [`gitlab-runner restart`](#gitlab-runner-restart)
- [`gitlab-runner status`](#gitlab-runner-status)
- [다중 서비스](#multiple-services)
- [**접근이 거부되었습니다** 서비스 관련 명령 실행 시](#access-denied-when-running-the-service-related-commands)

모든 서비스 관련 명령은 다음 인수를 허용합니다:

| 매개변수        | 기본값                                           | 설명 |
|------------------|---------------------------------------------------|-------------|
| `--service`      | `gitlab-runner`                                   | 사용자 지정 서비스 이름을 지정합니다 |
| `--config`       | [설정 파일](#configuration-file) 참조 | 사용할 사용자 지정 설정 파일을 지정합니다 |
| `--user-service` | [사용자 서비스](#user-service) 참조                 | GitLab 러너를 사용자 서비스(systemd)로 실행하도록 설정합니다 |

### `gitlab-runner install` {#gitlab-runner-install}

이 명령은 GitLab 러너를 서비스로 설치합니다. 실행되는 시스템에 따라 다양한 인수 집합을 허용합니다.

**Windows**에서 실행하거나 슈퍼유저로 실행할 때 `--user` 플래그를 허용하며, 이를 통해 **shell** 실행기로 실행되는 빌드의 권한을 제거할 수 있습니다.

| 매개변수             | 기본값                                           | 설명 |
|-----------------------|---------------------------------------------------|-------------|
| `--service`           | `gitlab-runner`                                   | 사용할 서비스 이름을 지정합니다 |
| `--config`            | [설정 파일](#configuration-file) 참조 | 사용할 사용자 지정 설정 파일을 지정합니다 |
| `--syslog`            | `true` (systemd가 아닌 시스템의 경우)                  | 서비스가 시스템 로깅 서비스와 통합되어야 하는지 지정합니다 |
| `--working-directory` | 현재 디렉터리                             | **shell** 실행기로 빌드를 실행할 때 모든 데이터가 저장되는 루트 디렉터리를 지정합니다 |
| `--user`              | `root`                                            | 빌드를 실행하는 사용자를 지정합니다 |
| `--password`          | 없음                                              | 빌드를 실행하는 사용자의 비밀번호를 지정합니다 |

### `gitlab-runner uninstall` {#gitlab-runner-uninstall}

이 명령은 GitLab 러너를 서비스로 실행하는 것을 중단하고 제거합니다.

### `gitlab-runner start` {#gitlab-runner-start}

이 명령은 GitLab 러너 서비스를 시작합니다.

### `gitlab-runner stop` {#gitlab-runner-stop}

이 명령은 GitLab 러너 서비스를 중단합니다.

### `gitlab-runner restart` {#gitlab-runner-restart}

이 명령은 GitLab 러너 서비스를 중단한 다음 시작합니다.

### `gitlab-runner status` {#gitlab-runner-status}

이 명령은 GitLab 러너 서비스의 상태를 출력합니다. 서비스가 실행 중일 때 종료 코드는 0이고 서비스가 실행 중이 아닐 때는 0이 아닙니다.

### 다중 서비스 {#multiple-services}

`--service` 플래그를 지정하면 여러 개의 GitLab 러너 서비스를 여러 개의 별도 설정과 함께 설치할 수 있습니다.

### 사용자 서비스 {#user-service}

일부 init 시스템(예: `systemd`)을 사용하여 [사용자 서비스](https://wiki.archlinux.org/title/Systemd/User)로 서비스를 관리할 수 있습니다. init 시스템이 이 기능을 제공하고 `gitlab-runner` 서비스를 사용자 서비스로 관리하려면 서비스 관련 명령을 실행할 때 `--user-service` 플래그를 지정합니다.

## 실행 관련 명령 {#run-related-commands}

이 명령을 사용하면 GitLab에서 빌드를 가져오고 처리할 수 있습니다.

### `gitlab-runner run` {#gitlab-runner-run}

`gitlab-runner run` 명령은 GitLab 러너를 서비스로 시작할 때 실행되는 주 명령입니다. `config.toml`에서 정의된 모든 러너를 읽고 모두 실행하려고 합니다.

이 명령은 [신호를 수신](#signals)할 때까지 실행되고 작동합니다.

다음 매개변수를 허용합니다.

| 매개변수             | 기본값                                       | 설명 |
|-----------------------|-----------------------------------------------|-------------|
| `--config`            | [설정 파일](#configuration-file) 참조 | 사용할 사용자 지정 설정 파일을 지정합니다 |
| `--working-directory` | 현재 디렉터리                         | **shell** 실행기로 빌드를 실행할 때 모든 데이터가 저장되는 루트 디렉터리를 지정합니다 |
| `--user`              | 현재 사용자                              | 빌드를 실행하는 사용자를 지정합니다 |
| `--syslog`            | `false`                                       | 모든 로그를 SysLog(Unix) 또는 EventLog(Windows)로 보냅니다 |
| `--listen-address`    | 비어 있음                                         | Prometheus 메트릭 HTTP 서버가 수신 대기해야 하는 주소(`<host>:<port>`) |

### `gitlab-runner run-single` {#gitlab-runner-run-single}

{{< history >}}

- 설정 파일을 사용하는 기능이 GitLab 러너 17.1에서 [도입](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/37670)되었습니다.

{{< /history >}}

이 보조 명령을 사용하여 단일 GitLab 인스턴스에서 단일 빌드를 실행합니다. 다음이 가능합니다:

- GitLab URL 및 러너 토큰을 포함하여 모든 옵션을 CLI 매개변수 또는 환경 변수로 사용합니다. 예를 들어 명시적으로 지정된 모든 매개변수가 있는 단일 작업:

  ```shell
  gitlab-runner run-single -u http://gitlab.example.com -t my-runner-token --executor docker --docker-image ruby:3.3
  ```

- 설정 파일을 읽어 특정 러너의 설정을 사용합니다. 예를 들어 설정 파일이 있는 단일 작업:

  ```shell
  gitlab-runner run-single -c ~/.gitlab-runner/config.toml -r runner-name
  ```

`--help` 플래그를 사용하여 가능한 모든 설정 옵션을 볼 수 있습니다:

```shell
gitlab-runner run-single --help
```

`--max-builds` 옵션을 사용하여 러너가 종료되기 전에 실행하는 빌드 수를 제어할 수 있습니다. `0`의 기본값은 러너가 빌드 제한이 없고 작업이 계속 실행됨을 의미합니다.

`--wait-timeout` 옵션을 사용하여 러너가 종료되기 전에 작업을 기다리는 시간을 제어할 수 있습니다. `0`의 기본값은 러너가 시간 초과가 없고 작업 간에 계속 대기함을 의미합니다.

## 내부 명령 {#internal-commands}

GitLab 러너는 단일 바이너리로 배포되며 빌드 중에 사용되는 몇 가지 내부 명령을 포함합니다.

### `gitlab-runner artifacts-downloader` {#gitlab-runner-artifacts-downloader}

GitLab에서 아티팩트 아카이브를 다운로드합니다.

### `gitlab-runner artifacts-uploader` {#gitlab-runner-artifacts-uploader}

GitLab에 아티팩트 아카이브를 업로드합니다.

### `gitlab-runner cache-archiver` {#gitlab-runner-cache-archiver}

캐시 아카이브를 만들고 로컬로 저장하거나 외부 서버에 업로드합니다.

### `gitlab-runner cache-extractor` {#gitlab-runner-cache-extractor}

로컬 또는 외부에 저장된 파일에서 캐시 아카이브를 복원합니다.

## 문제 해결 {#troubleshooting}

다음은 일반적인 함정 중 일부입니다.

### **접근이 거부되었습니다** 서비스 관련 명령 실행 시 {#access-denied-when-running-the-service-related-commands}

일반적으로 [서비스 관련 명령](#service-related-commands)에는 관리자 권한이 필요합니다:

- Unix(Linux, macOS, FreeBSD) 시스템에서 `gitlab-runner` 앞에 `sudo`를 추가합니다
- Windows 시스템에서는 승격된 명령 프롬프트를 사용합니다. `Administrator` 명령 프롬프트를 실행합니다. Windows 검색 필드에 `Command Prompt`를 입력하고 마우스 오른쪽 단추를 클릭한 후 `Run as administrator`을 선택합니다. 승격된 명령 프롬프트를 실행하려고 한다는 것을 확인합니다.

## `gitlab-runner stop`이 우아하게 종료되지 않음 {#gitlab-runner-stop-doesnt-shut-down-gracefully}

GitLab 러너가 호스트에 설치되고 로컬 실행기를 실행할 때 아티팩트 다운로드 또는 업로드, 캐시 처리와 같은 작업을 위해 추가 프로세스를 시작합니다. 이들 프로세스는 `gitlab-runner` 명령으로 실행되며, 이는 `pkill -QUIT gitlab-runner` 또는 `killall QUIT gitlab-runner`을 사용하여 종료할 수 있음을 의미합니다. 이들을 종료하면 이들이 담당하는 작업이 실패합니다.

다음은 이를 방지하기 위한 두 가지 방법입니다:

- 러너를 로컬 서비스(예: `systemd`)로 등록하면서 `SIGQUIT`을 작업 신호로 사용하고 `gitlab-runner stop` 또는 `systemctl stop gitlab-runner.service`를 사용합니다. 이 동작을 활성화하기 위한 예제 설정입니다:

  ```ini
  ; /etc/systemd/system/gitlab-runner.service.d/kill.conf
  [Service]
  KillSignal=SIGQUIT
  TimeoutStopSec=infinity
  ```

  - 이 파일을 만든 후 설정 변경을 적용하려면 `systemd`을 `systemctl daemon-reload`로 다시 로드합니다.
- `kill -SIGQUIT <pid>`을 사용하여 프로세스를 수동으로 종료합니다. 주 `gitlab-runner` 프로세스의 `pid`를 찾아야 합니다. 로그를 보고 시작 시 표시되는 항목을 확인하여 이를 찾을 수 있습니다:

  ```shell
  $ gitlab-runner run
  Runtime platform                                    arch=arm64 os=linux pid=8 revision=853330f9 version=16.5.0
  ```

### 시스템 ID 상태 파일 저장: 접근 거부 {#saving-system-id-state-file-access-denied}

GitLab 러너 15.7 및 15.8은 `config.toml` 파일이 포함된 디렉터리에 대한 쓰기 권한이 없으면 시작되지 않을 수 있습니다.

GitLab 러너가 시작되면 `.runner_system_id` 파일을 `config.toml`가 포함된 디렉터리에서 검색합니다. `.runner_system_id` 파일을 찾을 수 없으면 새 파일을 만듭니다. GitLab 러너에 쓰기 권한이 없으면 시작이 실패합니다.

이 문제를 해결하려면 일시적으로 파일 쓰기 권한을 허용한 다음 `gitlab-runner run`을 실행합니다. `.runner_system_id` 파일이 생성되면 권한을 읽기 전용으로 재설정할 수 있습니다.
