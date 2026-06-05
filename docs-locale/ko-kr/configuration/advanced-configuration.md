---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: 고급 구성
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

GitLab Runner와 개별 등록된 러너의 동작을 변경하려면 `config.toml` 파일을 수정합니다.

`config.toml` 파일은 다음 위치에서 찾을 수 있습니다:

- GitLab Runner를 루트로 실행하는 \*nix 시스템에서 `/etc/gitlab-runner/`를 사용합니다. 이 디렉터리는 서비스 구성의 경로이기도 합니다.
- GitLab Runner를 루트가 아닌 사용자로 실행하는 \*nix 시스템에서 `~/.gitlab-runner/`를 사용합니다.
- 다른 시스템에서 `./`를 사용합니다.

GitLab Runner는 대부분의 옵션을 변경할 때 다시 시작할 필요가 없습니다. 여기에는 `[[runners]]` 섹션의 매개변수와 `listen_address`를 제외한 전역 섹션의 대부분 매개변수가 포함됩니다. 러너가 이미 등록된 경우 다시 등록할 필요가 없습니다.

GitLab Runner는 3초마다 구성 수정 사항을 확인하고 필요하면 다시 로드합니다. GitLab Runner는 `SIGHUP` 신호에 대한 응답으로 구성을 다시 로드합니다.

## 구성 유효성 검사 {#configuration-validation}

{{< history >}}

- [GitLab Runner 15.10에서 도입됨](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/3924)

{{< /history >}}

구성 유효성 검사는 `config.toml` 파일의 구조를 확인하는 프로세스입니다. 구성 유효성 검사기의 출력은 `info` 수준 메시지만 제공합니다.

구성 유효성 검사 프로세스는 정보 제공 목적으로만 사용됩니다. 러너 구성의 잠재적 문제를 식별하기 위해 출력을 사용할 수 있습니다. 구성 유효성 검사는 모든 가능한 문제를 포착하지 못할 수 있으며, 메시지가 없다고 해서 `config.toml` 파일이 완벽하다는 보장은 없습니다.

## 전역 섹션 {#the-global-section}

이 설정은 전역입니다. 모든 러너에 적용됩니다.

| 설정              | 설명 |
|----------------------|-------------|
| `concurrent`         | 등록된 모든 러너에서 동시에 실행할 수 있는 작업 수를 제한합니다. 각 `[[runners]]` 섹션은 자체 제한을 정의할 수 있지만, 이 값은 모든 해당 값의 최대값을 설정합니다. 예를 들어 `10` 값은 최대 10개의 작업을 동시에 실행할 수 있음을 의미합니다. `0`는 금지됩니다. 이 값을 사용하면 러너 프로세스가 심각한 오류와 함께 종료됩니다. [Docker Machine 실행기](autoscale.md#limit-the-number-of-vms-created-by-the-docker-machine-executor) , [Instance 실행기](../executors/instance.md) , [Docker Autoscaler 실행기](../executors/docker_autoscaler.md#example-aws-autoscaling-for-1-job-per-instance) , 및 [`runners.custom_build_dir` 구성](#the-runnerscustom_build_dir-section)에서 이 설정이 어떻게 작동하는지 확인합니다. |
| `log_level`          | 로그 수준을 정의합니다. 옵션은 `debug`, `info`, `warn`, `error`, `fatal`, 및 `panic`입니다. 이 설정은 `--debug`, `-l`, 또는 `--log-level` 명령줄 인수로 설정된 수준보다 우선순위가 낮습니다. |
| `log_format`         | 로그 형식을 지정합니다. 옵션은 `runner`, `text`, 및 `json`입니다. 이 설정은 `--log-format` 명령줄 인수로 설정된 형식보다 우선순위가 낮습니다. 기본값은 `runner`이며, 색상 지정을 위한 ANSI 이스케이프 코드를 포함합니다. |
| `check_interval`     | 러너가 새로운 작업을 확인할 때까지의 간격(초)을 정의합니다. 기본값은 `3`입니다. `0` 이하로 설정하면 기본값이 사용됩니다. |
| `sentry_dsn`         | Sentry에 대한 모든 시스템 수준 오류의 추적을 활성화합니다. |
| `connection_max_age` | GitLab 서버로의 TLS �ープ얼라이브 연결이 다시 연결되기 전에 열려 있어야 하는 최대 기간입니다. 기본값은 `15m`입니다(15분). `0` 이하로 설정하면 연결이 최대한 오래 유지됩니다. |
| `listen_address`     | Prometheus 메트릭 HTTP 서버가 수신 대기해야 할 주소(`<host>:<port>`)를 정의합니다. |
| `shutdown_timeout`   | [강제 종료 작업](../commands/_index.md#signals)이 시간 초과되어 프로세스를 종료할 때까지의 초 단위 시간입니다. 기본값은 `30`입니다. `0` 이하로 설정하면 기본값이 사용됩니다. |

### 구성 경고 {#configuration-warnings}

#### 장시간 폴링 문제 {#long-polling-issues}

GitLab Runner는 GitLab Workhorse를 통해 GitLab 장시간 폴링이 켜져 있을 때 여러 구성 시나리오에서 장시간 폴링 문제를 경험할 수 있습니다. 이는 구성에 따라 성능 병목 현상부터 심각한 처리 지연까지 다양합니다. GitLab Runner 워커는 장시간 폴링 요청에서 장시간 동안 갇힐 수 있습니다(GitLab Workhorse 구성 `-apiCiLongPollingDuration`과 일치하며, 기본값은 50초). 이로 인해 다른 작업이 적시에 처리되지 않습니다.

이 문제는 GitLab Workhorse `-apiCiLongPollingDuration` 설정으로 제어되는 GitLab CI/CD 장시간 폴링 기능과 관련이 있습니다. 켜져 있으면 작업 요청은 작업을 사용할 수 있을 때까지 기다리는 동안 구성된 기간까지 차단될 수 있습니다.

기본 GitLab Workhorse 장시간 폴링 구성 값은 50초입니다(최근 GitLab 버전에서는 기본적으로 켜져 있음).

다음은 몇 가지 구성 예제입니다:

- Omnibus: `gitlab_workhorse['api_ci_long_polling_duration'] = "50s"` in `/etc/gitlab/gitlab.rb`
- Helm 차트:  `gitlab.webservice.workhorse.extraArgs` 설정을 사용합니다.
- CLI: `gitlab-workhorse -apiCiLongPollingDuration 50s`

자세한 정보는 다음을 참조하세요:

- [러너를 위한 장시간 폴링](https://docs.gitlab.com/ci/runners/long_polling/)
- [Workhorse 구성](https://docs.gitlab.com/development/workhorse/configuration/)

증상:

- 일부 프로젝트의 작업은 시작 전에 지연이 발생합니다(기간은 GitLab 인스턴스 장시간 폴링 시간 초과와 일치합니다).
- 다른 프로젝트의 작업은 즉시 실행됩니다.
- 러너 로그의 경고 메시지: `CONFIGURATION: Long polling issues detected`

일반적인 문제 시나리오:

- 워커 부족 병목:  `concurrent` 설정이 러너 수보다 적습니다(심각한 병목).
- 요청 병목:  `request_concurrency=1`인 러너는 장시간 폴링 중에 작업 지연을 유발합니다.
- 빌드 제한 병목:  낮은 `limit` 설정(≤2)과 `request_concurrency=1`가 결합된 러너.

GitLab Runner는 문제 시나리오를 자동으로 감지하고 경고 메시지에서 맞춤형 솔루션을 제공합니다. 일반적인 솔루션은 다음과 같습니다:

- `concurrent` 설정을 증가시켜 러너 수를 초과하도록 합니다.
- 높은 볼륨 러너의 `request_concurrency` 값을 1보다 높은 값으로 설정합니다(기본값은 1). [러너 모니터링](../monitoring/_index.md)을 활성화하여 시스템 상태를 파악하고 설정에 최적의 값을 찾습니다. `FF_USE_ADAPTIVE_REQUEST_CONCURRENCY` 기능 플래그를 사용하여 워크로드에 따라 `request_concurrency`를 자동으로 조정하는 것을 고려합니다. 적응형 동시성에 대한 자세한 내용은 [기능 플래그 설명서](feature-flags.md)를 참조하세요.
- `limit` 설정을 예상 작업 볼륨과 균형을 맞춥니다.

##### 문제가 있는 구성 예제 {#example-problematic-configurations}

시나리오 1:  워커 부족 병목: 

```toml
concurrent = 2  # Only 2 concurrent workers

[[runners]]
  name = "runner-1"
[[runners]]
  name = "runner-2"
[[runners]]
  name = "runner-3"  # 3 runners, only 2 workers - severe bottleneck
```

시나리오 2:  요청 병목: 

```toml
concurrent = 4  # 4 workers available

[[runners]]
  name = "high-volume-runner"
  request_concurrency = 1  # Default: only 1 request at a time
  limit = 10               # Can handle 10 jobs, but only 1 request slot
```

시나리오 3:  빌드 제한 병목: 

```toml
concurrent = 4

[[runners]]
  name = "limited-runner"
  limit = 2                # Only 2 builds allowed
  request_concurrency = 1  # Only 1 request at a time
  # Creates severe bottleneck: builds at capacity + request slot blocked by long polling
```

##### 수정된 구성 예제 {#example-corrected-configuration}

```toml
concurrent = 4  # Adequate worker capacity

[[runners]]
  name = "high-volume-runner"
  request_concurrency = 3  # Allow multiple simultaneous requests
  limit = 10

[[runners]]
  name = "balanced-runner"
  request_concurrency = 2
  limit = 5
```

다음은 구성 예제입니다:

```toml

# Example `config.toml` file

concurrent = 100 # A global setting for job concurrency that applies to all runner sections defined in this `config.toml` file
log_level = "warning"
log_format = "text"
check_interval = 3 # Value in seconds

[[runners]]
  name = "first"
  url = "Your Gitlab instance URL (for example, `https://gitlab.com`)"
  executor = "shell"
  (...)

[[runners]]
  name = "second"
  url = "Your Gitlab instance URL (for example, `https://gitlab.com`)"
  executor = "docker"
  (...)

[[runners]]
  name = "third"
  url = "Your Gitlab instance URL (for example, `https://gitlab.com`)"
  executor = "docker-autoscaler"
  (...)

```

### `log_format` 예제(잘림) {#log_format-examples-truncated}

#### `runner` {#runner}

```shell
Runtime platform                                    arch=amd64 os=darwin pid=37300 revision=HEAD version=development version
Starting multi-runner from /etc/gitlab-runner/config.toml...  builds=0
WARNING: Running in user-mode.
WARNING: Use sudo for system-mode:
WARNING: $ sudo gitlab-runner...

Configuration loaded                                builds=0
listen_address not defined, metrics & debug endpoints disabled  builds=0
[session_server].listen_address not defined, session endpoints disabled  builds=0
```

#### `text` {#text}

```shell
INFO[0000] Runtime platform                              arch=amd64 os=darwin pid=37773 revision=HEAD version="development version"
INFO[0000] Starting multi-runner from /etc/gitlab-runner/config.toml...  builds=0
WARN[0000] Running in user-mode.
WARN[0000] Use sudo for system-mode:
WARN[0000] $ sudo gitlab-runner...
INFO[0000]
INFO[0000] Configuration loaded                          builds=0
INFO[0000] listen_address not defined, metrics & debug endpoints disabled  builds=0
INFO[0000] [session_server].listen_address not defined, session endpoints disabled  builds=0
```

#### `json` {#json}

```shell
{"arch":"amd64","level":"info","msg":"Runtime platform","os":"darwin","pid":38229,"revision":"HEAD","time":"2025-06-05T15:57:35+02:00","version":"development version"}
{"builds":0,"level":"info","msg":"Starting multi-runner from /etc/gitlab-runner/config.toml...","time":"2025-06-05T15:57:35+02:00"}
{"level":"warning","msg":"Running in user-mode.","time":"2025-06-05T15:57:35+02:00"}
{"level":"warning","msg":"Use sudo for system-mode:","time":"2025-06-05T15:57:35+02:00"}
{"level":"warning","msg":"$ sudo gitlab-runner...","time":"2025-06-05T15:57:35+02:00"}
{"level":"info","msg":"","time":"2025-06-05T15:57:35+02:00"}
{"builds":0,"level":"info","msg":"Configuration loaded","time":"2025-06-05T15:57:35+02:00"}
{"builds":0,"level":"info","msg":"listen_address not defined, metrics \u0026 debug endpoints disabled","time":"2025-06-05T15:57:35+02:00"}
{"builds":0,"level":"info","msg":"[session_server].listen_address not defined, session endpoints disabled","time":"2025-06-05T15:57:35+02:00"}
```

### `check_interval`이 작동하는 방식 {#how-check_interval-works}

`config.toml`에 2개 이상의 `[[runners]]` 섹션이 있으면, GitLab Runner는 GitLab Runner가 구성된 GitLab 인스턴스에 작업 요청을 지속적으로 예약하는 루프를 포함합니다.

다음 예제는 10초의 `check_interval`과 2개의 `[[runners]]` 섹션(`runner-1` 및 `runner-2`)을 가지고 있습니다. GitLab Runner는 10초마다 요청을 보내고 5초 동안 절전 모드로 들어갑니다:

1. `check_interval` 값을 가져옵니다(`10s`).
1. 러너 목록을 가져옵니다(`runner-1`, `runner-2`).
1. 절전 모드 간격을 계산합니다(`10s / 2 = 5s`).
1. 무한 루프를 시작합니다:
   1. `runner-1`의 작업을 요청합니다.
   1. `5s` 동안 절전 모드로 들어갑니다.
   1. `runner-2`의 작업을 요청합니다.
   1. `5s` 동안 절전 모드로 들어갑니다.

기본적으로 러너가 작업을 수신하면, 작업이 없을 때까지 또는 실행 중인 작업 수가 `concurrent` 또는 `limit`에 도달할 때까지 더 많은 작업에 대해 즉시 다시 폴링합니다. 이 동작을 변경하려면 `strict_check_interval`를 `true`로 설정합니다. 활성화되면 러너는 확인 간격을 엄격히 준수하고 작업 수신 여부와 관계없이 매 `check_interval` 초마다 하나의 요청을 보냅니다(이 예제에서는 5초). 이 설정을 켜서 러너 플릿 간에 작업 배포를 개선하고 한 러너가 대부분의 작업을 처리하는 동안 다른 러너가 유휴 상태로 유지되는 것을 방지합니다. 그러나 작업이 큐에서 더 오래 기다릴 수 있습니다.

다음은 `check_interval` 구성 예제입니다:

```toml
# Example `config.toml` file

concurrent = 100 # A global setting for job concurrency that applies to all runner sections defined in this `config.toml` file.
log_level = "warning"
log_format = "json"
check_interval = 10 # Value in seconds

[[runners]]
  name = "runner-1"
  url = "Your Gitlab instance URL (for example, `https://gitlab.com`)"
  executor = "shell"
  (...)

[[runners]]
  name = "runner-2"
  url = "Your Gitlab instance URL (for example, `https://gitlab.com`)"
  executor = "docker"
  (...)
```

이 예제에서는 러너 프로세스의 작업 요청이 5초마다 이루어집니다. `runner-1`과 `runner-2`가 동일한 GitLab 인스턴스에 연결되어 있으면, 이 GitLab 인스턴스도 이 러너로부터 5초마다 새로운 요청을 받습니다.

`runner-1`의 첫 번째 및 두 번째 요청 사이에 2개의 절전 모드 기간이 발생합니다. 각 기간은 5초가 소요되므로 `runner-1`에 대한 후속 요청 사이에 약 10초입니다. `runner-2`에도 동일하게 적용됩니다.

더 많은 러너를 정의하면 절전 모드 간격이 더 작아집니다. 그러나 러너에 대한 요청은 다른 모든 러너에 대한 요청과 해당 절전 모드 기간 이후에 반복됩니다.

## `[machine]` 섹션 {#the-machine-section}

{{< history >}}

- GitLab Runner 18.10에서 도입됨.

{{< /history >}}

`[machine]` 섹션은 `docker+machine` 실행기 공급자의 전역 설정을 구성합니다. 이 설정은 `docker+machine` 실행기를 사용하는 모든 러너에 적용됩니다.

### `[machine.shutdown_drain]` 섹션 {#the-machineshutdown_drain-section}

러너 프로세스가 종료될 때, 풀의 유휴 머신은 일반적으로 실행 상태로 유지됩니다. 외부에서 이를 정리해야 합니다(예: `systemd` post-stop 후크를 통해). `shutdown_drain` 섹션은 종료 시 유휴 머신을 자동으로 제거하도록 러너를 구성합니다.

| 매개변수       | 유형     | 설명 |
|-----------------|----------|-------------|
| `enabled`       | 부울  | 종료 시 유휴 머신의 자동 제거를 활성화합니다. 기본값: `false`. |
| `concurrency`   | 정수  | 동시에 제거할 머신 수입니다. 기본값: `3`. |
| `max_retries`   | 정수  | 머신당 최대 재시도 시도 횟수입니다. 기본값: `3`. |
| `retry_backoff` | 기간 | 재시도 간의 기본 백오프 기간입니다(시도 번호로 곱함). 기본값: `5s`. |

> [!note]
> 드레인 작업은 전역 [`shutdown_timeout`](#the-global-section) 설정을 사용합니다. 기본 시간 초과는 30초로, 일반적으로 머신 드레인에는 너무 짧습니다. 종료 드레인을 켤 때 `shutdown_timeout`를 증가시켜 모든 머신이 제거될 때까지 충분한 시간을 허용합니다. 최소 5분이 권장되지만, 더 큰 풀은 더 긴 시간 초과를 필요로 할 수 있습니다. 러너는 시간 초과가 너무 짧으면 경고를 기록합니다.

예제:

```toml
concurrent = 10
check_interval = 0
shutdown_timeout = 600  # 10 minutes - required for draining machines

[machine]
  [machine.shutdown_drain]
    enabled = true
    concurrency = 5
    max_retries = 3
    retry_backoff = "5s"

[[runners]]
  name = "my-runner"
  url = "https://gitlab.example.com/"
  token = "xxx"
  executor = "docker+machine"

  [runners.machine]
    IdleCount = 5
    IdleTime = 600
    MachineName = "auto-scale-%s"
    MachineDriver = "google"
    MachineOptions = ["google-project=my-project", "google-zone=us-central1-a"]
```

## `[session_server]` 섹션 {#the-session_server-section}

작업과 상호 작용하려면 `[session_server]` 섹션을 루트 수준에서 `[[runners]]` 섹션 외부에 지정합니다. 이 섹션을 모든 러너에 대해 한 번 구성하고, 각 개별 러너에 대해서는 구성하지 마세요.

```toml
# Example `config.toml` file with session server configured

concurrent = 100 # A global setting for job concurrency that applies to all runner sections defined in this `config.toml` file
log_level = "warning"
log_format = "runner"
check_interval = 3 # Value in seconds

[session_server]
  listen_address = "[::]:8093" # Listen on all available interfaces on port `8093`
  advertise_address = "runner-host-name.tld:8093"
  session_timeout = 1800
```

`[session_server]` 섹션을 구성할 때:

- `listen_address`과 `advertise_address`의 경우 `host:port` 형식을 사용합니다. 여기서 `host`는 IP 주소(`127.0.0.1:8093`) 또는 도메인(`my-runner.example.com:8093`)입니다. 러너는 이 정보를 사용하여 보안 연결을 위한 TLS 인증서를 만듭니다.
- GitLab이 `listen_address` 또는 `advertise_address`에 정의된 IP 주소 및 포트에 연결할 수 있는지 확인합니다.
- `advertise_address`가 공개 IP 주소인지 확인하세요. 단, 애플리케이션 설정 [`allow_local_requests_from_web_hooks_and_services`](https://docs.gitlab.com/api/settings/#available-settings)를 활성화한 경우는 예외입니다.

| 설정             | 설명 |
|---------------------|-------------|
| `listen_address`    | 세션 서버의 내부 URL입니다. |
| `advertise_address` | 세션 서버에 액세스하는 URL입니다. GitLab Runner가 이를 GitLab에 노출합니다. 정의되지 않으면 `listen_address`이 사용됩니다. |
| `session_timeout`   | 작업이 완료된 후 세션이 활성 상태로 유지될 수 있는 초 단위 시간입니다. 시간 초과는 작업이 완료되는 것을 차단합니다. 기본값은 `1800`입니다(30분). |

세션 서버 및 터미널 지원을 비활성화하려면 `[session_server]` 섹션을 삭제합니다.

> [!note]
> 러너 인스턴스가 이미 실행 중인 경우, `gitlab-runner restart`를 실행해야 `[session_server]` 섹션의 변경 사항이 적용될 수 있습니다.

GitLab Runner Docker 이미지를 사용 중인 경우, [`docker run` 명령](../install/docker.md)에 `-p 8093:8093`를 추가하여 포트 `8093`을 노출해야 합니다.

## `[[runners]]` 섹션 {#the-runners-section}

각 `[[runners]]` 섹션은 하나의 러너를 정의합니다.

| 설정                               | 설명                                                                                                                                                                                                                                                                                                                                                                                                 |
| ------------------------------------- |-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `name`                                | 러너의 설명입니다. 정보 제공용입니다.                                                                                                                                                                                                                                                                                                                                                               |
| `url`                                 | GitLab 인스턴스 URL입니다. 환경 변수 확장을 지원합니다(예: `$GITLAB_URL` 또는 `${GITLAB_URL}`).                                                                                                                                                                                                                                                                                               |
| `token`                               | 러너 등록 중에 얻은 러너 인증 토큰입니다. [등록 토큰과는 다릅니다](https://docs.gitlab.com/api/runners/#registration-and-authentication-tokens). 환경 변수 확장을 지원합니다(예: `$RUNNER_TOKEN` 또는 `${RUNNER_TOKEN}`).                                                                                                        |
| `tls-ca-file`                         | HTTPS를 사용할 때 피어를 검증하기 위한 인증서를 포함하는 파일입니다. [자체 서명된 인증서 또는 사용자 지정 인증 기관 설명서](tls-self-signed.md)를 참조하세요.                                                                                                                                                                                                                             |
| `tls-cert-file`                       | HTTPS를 사용할 때 피어와 인증하기 위한 인증서를 포함하는 파일입니다.                                                                                                                                                                                                                                                                                                                         |
| `tls-key-file`                        | HTTPS를 사용할 때 피어와 인증하기 위한 개인 키를 포함하는 파일입니다.                                                                                                                                                                                                                                                                                                                         |
| `limit`                               | 이 등록된 러너가 동시에 처리할 수 있는 작업 수를 제한합니다. `0`(기본값)은 제한이 없음을 의미합니다. [Docker Machine](autoscale.md#limit-the-number-of-vms-created-by-the-docker-machine-executor) , [Instance](../executors/instance.md) , 및 [Docker Autoscaler](../executors/docker_autoscaler.md#example-aws-autoscaling-for-1-job-per-instance) 실행기에서 이 설정이 어떻게 작동하는지 확인합니다. |
| `executor`                            | 호스트 운영 체제의 환경 또는 명령 프로세서로, 러너가 CI/CD 작업을 실행하는 데 사용합니다. 자세한 내용은 [실행기](../executors/_index.md)를 참조하세요.                                                                                                                                                                                                                                   |
| `shell`                               | 스크립트를 생성할 셸의 이름입니다. 기본값은 [플랫폼에 따라 다릅니다](../shells/_index.md).                                                                                                                                                                                                                                                                                                           |
| `builds_dir`                          | 선택한 실행기의 컨텍스트에서 빌드가 저장되는 디렉터리의 절대 경로입니다. 예를 들어, 로컬, Docker, 또는 SSH입니다.                                                                                                                                                                                                                                                                         |
| `cache_dir`                           | 선택한 실행기의 컨텍스트에서 빌드 캐시가 저장되는 디렉터리의 절대 경로입니다. 예를 들어, 로컬, Docker, 또는 SSH입니다. `docker` 실행기를 사용하는 경우, 이 디렉터리는 `volumes` 매개변수에 포함되어야 합니다.                                                                                                                                                                         |
| `environment`                         | 환경 변수를 추가하거나 덮어씁니다.                                                                                                                                                                                                                                                                                                                                                                  |
| `request_concurrency`                 | GitLab의 새로운 작업에 대한 동시 요청 수를 제한합니다. 기본값은 `1`입니다. `concurrency`, `limit`, 및 `request_concurrency`가 작업 흐름을 제어하는 방법에 대한 자세한 내용은 [GitLab Runner 동시성 조정에 대한 KB 문서](https://support.gitlab.com/hc/en-us/articles/21324350882076-GitLab-Runner-Concurrency-Tuning-Understanding-request-concurrency)를 참조하세요.                     |
| `strict_check_interval`               | 정상 작동 중에 러너가 작업을 폴링하고 작업을 수신하면, 작업 수가 `concurrent` 또는 `limit`와 일치하거나 작업이 없을 때까지 작업에 대해 즉시 다시 폴링합니다. `strict_check_interval`을 켜면 러너는 `check_interval` 빠른 다시 폴링 루프를 비활성화하고 `check_interval`를 엄격히 준수합니다. 기본값은 `false`입니다.             |
| `output_limit`                        | 최대 빌드 로그 크기(킬로바이트)입니다. 기본값은 `4096`입니다(4MB).                                                                                                                                                                                                                                                                                                                                              |
| `pre_get_sources_script`              | Git 리포지토리를 업데이트하고 서브모듈을 업데이트하기 전에 러너에서 실행할 명령입니다. 먼저 Git 클라이언트 구성을 조정하는 데 사용합니다. 여러 명령을 삽입하려면 (삼중 인용) 다중 행 문자열 또는 `\n` 문자를 사용합니다.                                                                                                                                                 |
| `post_get_sources_script`             | Git 리포지토리를 업데이트하고 서브모듈을 업데이트한 후에 러너에서 실행할 명령입니다. 여러 명령을 삽입하려면 (삼중 인용) 다중 행 문자열 또는 `\n` 문자를 사용합니다.                                                                                                                                                                                                                    |
| `pre_build_script`                    | 작업을 실행하기 전에 러너에서 실행할 명령입니다. `before_script`, `script`, 및 `post_build_script`과 동일한 셸 컨텍스트에서 실행합니다. `pre_build_script`가 실패하면 해당 컨텍스트의 나머지 명령을 건너뛰지만 `after_script`는 계속 실행됩니다. 여러 명령을 삽입하려면 (삼중 인용) 다중 행 문자열 또는 `\n` 문자를 사용합니다.                                               |
| `post_build_script`                   | 작업을 실행한 후에 러너에서 실행할 명령입니다. `pre_build_script`, `before_script`, 및 `script`과 동일한 셸 컨텍스트에서 실행합니다. 이들 중 하나라도 실패하면 `post_build_script`를 건너뜁니다. `after_script`는 별도의 셸 컨텍스트에서 실행되고 `post_build_script`의 영향을 받지 않습니다. 여러 명령을 삽입하려면 (삼중 인용) 다중 행 문자열 또는 `\n` 문자를 사용합니다.               |
| `clone_url`                           | GitLab 인스턴스의 URL을 덮어씁니다. 러너가 GitLab URL에 연결할 수 없는 경우에만 사용됩니다.                                                                                                                                                                                                                                                                                                         |
| `debug_trace_disabled`                | [디버그 추적](https://docs.gitlab.com/ci/variables/#enable-debug-logging)을 비활성화합니다. `true`로 설정하면 `CI_DEBUG_TRACE`이 `true`로 설정되어 있어도 디버그 로그(추적)가 비활성화된 상태로 유지됩니다.                                                                                                                                                                                                                 |
| `clean_git_config`                    | Git 구성을 정리합니다. 자세한 내용은 [Git 구성 정리](#cleaning-git-configuration)를 참조하세요.                                                                                                                                                                                                                                                                                          |
| `referees`                            | GitLab에 작업 아티팩트로 결과를 전달하는 추가 작업 모니터링 워커입니다.                                                                                                                                                                                                                                                                                                                            |
| `unhealthy_requests_limit`            | 그 후 러너 워커가 비활성화되는 새로운 작업 요청에 대한 `unhealthy` 응답 수입니다.                                                                                                                                                                                                                                                                                                            |
| `unhealthy_interval`                  | 러너 워커가 건강하지 않은 요청 제한을 초과한 후 비활성화되는 기간입니다. `3600 s`, `1 h 30 min`, 및 유사한 구문을 지원합니다.                                                                                                                                                                                                                                                      |
| `job_status_final_update_retry_limit` | GitLab Runner가 최종 작업 상태를 GitLab 인스턴스로 푸시하기 위해 재시도할 수 있는 최대 횟수입니다.                                                                                                                                                                                                                                                                                                    |
| `prepare_timeout`                     | `prepare` 스테이지에 허용되는 최대 기간입니다(실행기 초기화 및 셸 환경 설정). `30s` 또는 `1h30m`과 같은 기간 문자열을 허용합니다. 설정되지 않았거나, 0이거나, 작업 시간 초과보다 크면 작업 시간 초과로 기본 설정됩니다. 자세한 내용은 [준비 스테이지 시간 초과](#prepare-stage-timeout)를 참조하세요.                                                                                        |

예제:

```toml
[[runners]]
  name = "example-runner"
  url = "http://gitlab.example.com/"
  token = "TOKEN"
  limit = 0
  executor = "docker"
  builds_dir = ""
  shell = ""
  environment = ["ENV=value", "LC_ALL=en_US.UTF-8"]
  clone_url = "http://gitlab.example.local"
```

### 민감한 값에 환경 변수 사용 {#use-environment-variables-for-sensitive-values}

`token` 및 `url` 필드의 환경 변수를 사용하여 구성 파일에 민감한 값을 직접 저장하지 않을 수 있습니다. `$VAR` 및 `${VAR}` 구문이 모두 지원됩니다.

```toml
[[runners]]
  name = "runner-1"
  url = "$GITLAB_URL"
  token = "${RUNNER_TOKEN_1}"
  executor = "docker"

[[runners]]
  name = "runner-2"
  url = "$GITLAB_URL"
  token = "${RUNNER_TOKEN_2}"
  executor = "docker"
```

다음에 유용합니다:

- 토큰이 비밀에서 마운트되는 Kubernetes 배포
- 토큰이 환경 변수로 전달되는 Docker 배포
- 버전 관리 구성 파일의 비밀 방지

### 레거시 `/ci` URL 접미사 {#legacy-ci-url-suffix}

{{< history >}}

- [GitLab Runner 1.0.0에서 더 이상 사용되지 않음](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/289).
- GitLab Runner 18.7.0에 경고가 추가됨.

{{< /history >}}

GitLab Runner 1.0.0 이전 버전에서 러너 URL은 `/ci` 접미사로 구성되었습니다(예: `url = "https://gitlab.example.com/ci"`). 이 접미사는 더 이상 필요하지 않으며 구성에서 제거해야 합니다.

`config.toml`에 `/ci` 접미사가 포함된 URL이 있으면, GitLab Runner가 구성을 처리할 때 자동으로 제거합니다. 그러나 잠재적인 문제를 방지하기 위해 구성 파일을 업데이트하여 접미사를 제거해야 합니다.

#### 알려진 문제 {#known-issues}

- Git 서브모듈 인증 실패:  `GIT_SUBMODULE_FORCE_HTTPS=true`이 설정되면 서브모듈이 `fatal: could not read Username for 'https://gitlab.example.com': terminal prompts disabled`과 같은 인증 오류로 복제하지 못할 수 있습니다. 이 문제는 `/ci` 접미사가 Git URL 다시 쓰기 규칙을 방해하기 때문에 발생합니다. 자세한 내용은 [이슈 581678](https://gitlab.com/gitlab-org/gitlab/-/work_items/581678#note_2934077238)을 참조하세요.

**Problematic configuration**:

```toml
[[runners]]
  name = "legacy-runner"
  url = "https://gitlab.example.com/ci"  # Remove the /ci suffix
  token = "TOKEN"
  executor = "docker"
```

**Corrected configuration**:

```toml
[[runners]]
  name = "legacy-runner"
  url = "https://gitlab.example.com"  # /ci suffix removed
  token = "TOKEN"
  executor = "docker"
```

GitLab Runner가 `/ci` 접미사가 포함된 URL로 시작하면 경고 메시지를 기록합니다:

```plaintext
WARNING: The runner URL contains a legacy '/ci' suffix. This suffix is deprecated and should be
removed from the configuration. Git submodules may fail to clone with authentication errors if this
suffix is present. Please update the 'url' field in your config.toml to remove the '/ci' suffix.
See https://docs.gitlab.com/runner/configuration/advanced-configuration/#legacy-ci-url-suffix for more information.
```

이 경고를 해결하려면 `config.toml` 파일을 편집하고 `url` 필드에서 `/ci` 접미사를 제거합니다.

### `clone_url`이 작동하는 방식 {#how-clone_url-works}

GitLab 인스턴스를 러너가 사용할 수 없는 URL에서 사용할 수 있을 때 `clone_url`을 구성할 수 있습니다.

예를 들어, 방화벽이 러너가 URL에 도달하는 것을 방지할 수 있습니다. 러너가 `192.168.1.23`의 노드에 도달할 수 있으면 `clone_url`을 `http://192.168.1.23`으로 설정합니다.

`clone_url`이 설정되면 러너는 `http://gitlab-ci-token:s3cr3tt0k3n@192.168.1.23/namespace/project.git` 형식의 클론 URL을 구성합니다.

> [!note]
> `clone_url`은 Git LFS 엔드포인트나 아티팩트 업로드 또는 다운로드에 영향을 주지 않습니다.

#### Git LFS 엔드포인트 수정 {#modify-git-lfs-endpoints}

[Git LFS](https://docs.gitlab.com/topics/git/lfs/) 엔드포인트를 수정하려면 다음 파일 중 하나에서 `pre_get_sources_script`을 설정합니다:

- `config.toml`:

  ```toml
  pre_get_sources_script = "mkdir -p $RUNNER_TEMP_PROJECT_DIR/git-template; git config -f $RUNNER_TEMP_PROJECT_DIR/git-template/config lfs.url https://<alternative-endpoint>"
  ```

- `.gitlab-ci.yml`:

  ```yaml
  default:
    hooks:
      pre_get_sources_script:
        - mkdir -p $RUNNER_TEMP_PROJECT_DIR/git-template
        - git config -f $RUNNER_TEMP_PROJECT_DIR/git-template/config lfs.url https://localhost
  ```

### `unhealthy_requests_limit`과 `unhealthy_interval`이 작동하는 방식 {#how-unhealthy_requests_limit-and-unhealthy_interval-works}

GitLab 인스턴스를 오래 사용할 수 없을 때(예: 버전 업그레이드 중) 러너는 유휴 상태가 됩니다. 러너는 GitLab 인스턴스를 다시 사용할 수 있게 된 후 30~60분 동안 작업 처리를 재개하지 않습니다.

러너가 유휴 상태인 기간을 증가 또는 감소시키려면 `unhealthy_interval` 설정을 변경합니다.

러너의 GitLab 서버에 대한 연결 시도 수를 변경하고 건강하지 않은 절전 모드를 받기 전에 유휴 상태가 되려면 `unhealthy_requests_limit` 설정을 변경합니다. 자세한 내용은 [`check_interval`이 작동하는 방식](advanced-configuration.md#how-check_interval-works)을 참조하세요.

### 준비 스테이지 시간 초과 {#prepare-stage-timeout}

{{< history >}}

- [GitLab Runner 19.0.0에서 도입됨](https://gitlab.com/gitlab-org/gitlab-runner/-/work_items/26583).

{{< /history >}}

`prepare_timeout` 설정은 러너가 작업 스크립트를 실행하기 전에 실행 환경 준비에 소비하는 시간을 제한합니다. 준비 스테이지는 2가지 단계를 포함합니다:

1. **Executor initialization**(`prepare_executor`):  러너는 Docker 컨테이너 시작, Kubernetes Pod 예약 또는 SSH를 통한 연결과 같은 실행 환경을 설정합니다.
1. **Shell environment setup**(`prepare_script`):  러너는 후속 작업 스테이지에 필요한 셸 환경(경로, 작업 디렉터리, 셸 함수 등)을 초기화하는 스크립트를 생성하고 실행합니다.

준비 스테이지가 `prepare_timeout`을 초과하면 작업이 즉시 실패합니다. 후속 스테이지(`get_sources`, `restore_cache`, `script` 등)는 `prepare_timeout`로 제한되지 않습니다. 대신 전체 작업 시간 초과를 사용합니다.

**Default behavior**:  `prepare_timeout`이 설정되지 않았거나, `0`이거나, 작업 시간 초과를 초과하면 러너는 준비 스테이지에 작업 시간 초과를 사용합니다.

#### `prepare_timeout`을 설정하는 시기 {#when-to-set-prepare_timeout}

느린 또는 반응하지 않는 환경 초기화가 작업 작업이 시작되기 전에 전체 작업 시간 초과를 소비할 수 있을 때 `prepare_timeout`을 설정합니다. 일반적인 시나리오는 다음과 같습니다:

- **Docker image pulls**:  컨테이너 레지스트리가 느리거나 도달할 수 없으면 이미지 끌어오기가 전체 작업 시간 초과까지 멈출 수 있습니다. 바쁜 러너에서 정지된 끌어오기는 모든 사용 가능한 작업 슬롯을 채우고 새 작업의 시작을 방지합니다. `prepare_timeout`은 이러한 작업을 신속하게 실패시켜 러너 용량을 확보합니다.
- **Custom or HPC executors**:  실행기가 외부 리소스 스케줄러를 기다려 용량을 할당해야 할 때(HPC 작업 큐와 같은) 시작 시간이 예측할 수 없고 잠재적으로 매우 길 수 있습니다. `prepare_timeout` 없이 정지된 작업은 전체 작업 시간 초과 동안 러너 슬롯을 차지합니다.

#### 구성 예제 {#example-configuration}

```toml
[[runners]]
  name = "my-runner"
  url = "https://gitlab.example.com/"
  token = "TOKEN"
  executor = "docker"
  prepare_timeout = "5m"
```

## 실행기 {#the-executors}

다음 실행기를 사용할 수 있습니다.

| 실행기            | 필요한 구성                                                  | 작업이 실행되는 위치 |
|---------------------|-------------------------------------------------------------------------|----------------|
| `shell`             |                                                                         | 로컬 셸입니다. 기본 실행기입니다. |
| `docker`            | `[runners.docker]` 및 [Docker Engine](https://docs.docker.com/engine/) | Docker 컨테이너입니다. |
| `docker-windows`    | `[runners.docker]` 및 [Docker Engine](https://docs.docker.com/engine/) | Windows Docker 컨테이너입니다. |
| `ssh`               | `[runners.ssh]`                                                         | SSH, 원격으로. |
| `parallels`         | `[runners.parallels]` 및 `[runners.ssh]`                               | Parallels VM이지만 SSH로 연결합니다. |
| `virtualbox`        | `[runners.virtualbox]` 및 `[runners.ssh]`                              | VirtualBox VM이지만 SSH로 연결합니다. |
| `docker+machine`    | `[runners.docker]` 및 `[runners.machine]`                              | `docker` 같지만 [자동 확장 Docker 머신](autoscale.md)을 사용합니다. |
| `kubernetes`        | `[runners.kubernetes]`                                                  | Kubernetes Pod입니다. |
| `docker-autoscaler` | `[docker-autoscaler]` 및 `[runners.autoscaler]`                        | `docker` 같지만 자동 확장 인스턴스를 사용하여 컨테이너에서 CI/CD 작업을 실행합니다. |
| `instance`          | `[docker-autoscaler]` 및 `[runners.autoscaler]`                        | `shell` 같지만 자동 확장 인스턴스를 사용하여 호스트 인스턴스에서 CI/CD 작업을 직접 실행합니다. |

## 셸 {#the-shells}

셸 실행기를 사용하도록 구성되었을 때 CI/CD 작업이 호스트 머신에서 로컬로 실행됩니다. 지원되는 운영 체제 셸은 다음과 같습니다:

| 셸        | 설명 |
|--------------|-------------|
| `bash`       | Bash(Bourne-shell) 스크립트를 생성합니다. 모든 명령이 Bash 컨텍스트에서 실행됩니다. 모든 Unix 시스템의 기본값입니다. |
| `sh`         | Sh(Bourne-shell) 스크립트를 생성합니다. 모든 명령이 Sh 컨텍스트에서 실행됩니다. 모든 Unix 시스템에서 `bash`의 폴백입니다. |
| `powershell` | PowerShell 스크립트를 생성합니다. 모든 명령이 PowerShell Desktop 컨텍스트에서 실행됩니다. `kubernetes` 및 `docker-windows` 실행기를 사용하는 Windows의 작업의 기본 셸입니다. |
| `pwsh`       | PowerShell 스크립트를 생성합니다. 모든 명령이 PowerShell Core 컨텍스트에서 실행됩니다. Windows의 새로운 러너 등록의 기본 셸이며 `shell` 실행기를 사용하는 작업입니다. |

`shell` 옵션이 `bash` 또는 `sh`으로 설정되면 작업 스크립트를 셸 이스케이프하기 위해 Bash의 [ANSI-C 인용](https://www.gnu.org/software/bash/manual/html_node/ANSI_002dC-Quoting.html)이 사용됩니다.

### POSIX 호환 셸 사용 {#use-a-posix-compliant-shell}

GitLab Runner 14.9 이상에서, [기능 플래그를 활성화](feature-flags.md)하여 `FF_POSIXLY_CORRECT_ESCAPES`(예: `dash`)를 사용하는 POSIX 호환 셸을 사용하세요. 활성화되면 POSIX 호환 셸 이스케이프 메커니즘인 ["이중 따옴표"](https://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html#tag_18_02)가 사용됩니다.

## `[runners.docker]` 섹션 {#the-runnersdocker-section}

다음 설정은 Docker 컨테이너 매개변수를 정의합니다. 이 설정은 러너가 Docker 실행기를 사용하도록 구성되었을 때 적용됩니다.

[Docker-in-Docker](https://docs.gitlab.com/ci/docker/using_docker_build/#use-docker-in-docker) 서비스 또는 작업 내부에 구성된 컨테이너 런타임은 이 매개변수를 상속하지 않습니다.

| 매개변수                          | 예제                                          | 설명 |
|------------------------------------|--------------------------------------------------|-------------|
| `allowed_images`                   | `["ruby:*", "python:*", "php:*"]`                | `.gitlab-ci.yml` 파일에 지정할 수 있는 이미지의 와일드카드 목록입니다. 없으면 모든 이미지가 허용됩니다(`["*/*:*"]`와 동일). [Docker](../executors/docker.md#restrict-docker-images-and-services) 또는 [Kubernetes](../executors/kubernetes/_index.md#restrict-docker-images-and-services) 실행기와 함께 사용합니다. |
| `allowed_privileged_images`        |                                                  | `allowed_images`의 와일드카드 부분집합으로 `privileged`가 활성화되면 권한 있는 모드에서 실행합니다. 없으면 모든 이미지가 허용됩니다(`["*/*:*"]`와 동일). [Docker](../executors/docker.md#restrict-docker-images-and-services) 실행기와 함께 사용합니다. |
| `allowed_pull_policies`            |                                                  | `.gitlab-ci.yml` 파일 또는 `config.toml` 파일에 지정할 수 있는 끌어오기 정책의 목록입니다. 지정하지 않으면 `pull-policy`에 지정된 끌어오기 정책만 허용됩니다. [Docker](../executors/docker.md#allow-docker-pull-policies) 실행기와 함께 사용합니다. |
| `allowed_services`                 | `["postgres:9", "redis:*", "mysql:*"]`           | `.gitlab-ci.yml` 파일에 지정할 수 있는 서비스의 와일드카드 목록입니다. 없으면 모든 이미지가 허용됩니다(`["*/*:*"]`와 동일). [Docker](../executors/docker.md#restrict-docker-images-and-services) 또는 [Kubernetes](../executors/kubernetes/_index.md#restrict-docker-images-and-services) 실행기와 함께 사용합니다. |
| `allowed_privileged_services`      |                                                  | `allowed_services`의 와일드카드 부분집합으로 `privileged` 또는 `services_privileged`가 활성화되면 권한 있는 모드에서 실행할 수 있습니다. 없으면 모든 이미지가 허용됩니다(`["*/*:*"]`와 동일). [Docker](../executors/docker.md#restrict-docker-images-and-services) 실행기와 함께 사용합니다. |
| `cache_dir`                        |                                                  | Docker 캐시를 저장해야 하는 디렉터리입니다. 이 경로는 절대 경로이거나 현재 작업 디렉터리에 상대적일 수 있습니다. 자세한 내용은 `disable_cache`를 참조하세요. |
| `cap_add`                          | `["NET_ADMIN"]`                                  | 컨테이너에 추가 Linux 기능을 추가합니다. |
| `cap_drop`                         | `["DAC_OVERRIDE"]`                               | 컨테이너에서 추가 Linux 기능을 제거합니다. |
| `cpuset_cpus`                      | `"0,1"`                                          | 제어 그룹의 `CpusetCpus`입니다. 문자열입니다. |
| `cpuset_mems`                      | `"0,1"`                                          | 제어 그룹의 `CpusetMems`입니다. 문자열입니다. |
| `cpu_shares`                       |                                                  | 상대 CPU 사용량을 설정하는 데 사용되는 CPU 공유 수입니다. 기본값은 `1024`입니다. |
| `cpus`                             | `"2"`                                            | CPU 수입니다(Docker 1.13 이상에서 사용 가능). 문자열입니다. |
| `devices`                          | `["/dev/net/tun"]`                               | 컨테이너와 추가 호스트 장치를 공유합니다. |
| `device_cgroup_rules`              |                                                  | 사용자 정의 장치 `cgroup` 규칙입니다(Docker 1.28 이상에서 사용 가능). |
| `disable_cache`                    |                                                  | Docker 실행기에는 2가지 수준의 캐싱이 있습니다. 다른 실행기와 같은 전역 캐싱과 Docker 볼륨 기반 로컬 캐싱입니다. 이 구성 플래그는 로컬 캐싱에만 작동하며 자동으로 생성된(호스트 디렉터리에 매핑되지 않은) 캐시 볼륨의 사용을 비활성화합니다. 즉, 빌드의 임시 파일을 보유하는 컨테이너 생성만 방지하고, 러너가 [분산 캐시 모드](autoscale.md#distributed-runners-caching)로 구성된 경우 캐시를 비활성화하지 않습니다. |
| `disable_entrypoint_overwrite`     |                                                  | 이미지 진입점 덮어쓰기를 비활성화합니다. |
| `dns`                              | `["8.8.8.8"]`                                    | 컨테이너가 사용할 DNS 서버 목록입니다. |
| `dns_search`                       |                                                  | DNS 검색 도메인 목록입니다. |
| `extra_hosts`                      | `["other-host:127.0.0.1"]`                       | 컨테이너 환경에 정의되어야 하는 호스트입니다. |
| `gpus`                             |                                                  | Docker 컨테이너의 GPU 장치입니다. `docker` CLI과 동일한 형식을 사용합니다. [Docker 설명서](https://docs.docker.com/engine/containers/resource_constraints/#gpu)에서 자세한 내용을 확인하세요. [GPU를 활성화하도록 구성](gpus.md#docker-executor)해야 합니다. |
| `group_add`                        | `["docker"]`                                     | 컨테이너 프로세스가 실행할 추가 그룹을 추가합니다. |
| `helper_image`                     |                                                  | (고급) [기본 도우미 이미지](#helper-image)로 리포지토리를 복제하고 아티팩트를 업로드하는 데 사용합니다. |
| `helper_image_flavor`              |                                                  | 도우미 이미지 플레이버를 설정합니다(`alpine`, `alpine3.21`, `alpine-latest`, `ubi-fips` 또는 `ubuntu`). 기본값은 `alpine`입니다. `alpine` 플레이버는 `alpine-latest`과 동일한 버전을 사용합니다. |
| `helper_image_autoset_arch_and_os` |                                                  | 기본 OS를 사용하여 도우미 이미지 아키텍처 및 OS를 설정합니다. |
| `host`                             |                                                  | 사용자 지정 Docker 엔드포인트입니다. 기본값은 `DOCKER_HOST` 환경 또는 `unix:///var/run/docker.sock`입니다. |
| `hostname`                         |                                                  | Docker 컨테이너의 사용자 지정 호스트명입니다. |
| `image`                            | `"ruby:3.3"`                                     | 작업을 실행할 이미지입니다. |
| `links`                            | `["mysql_container:mysql"]`                      | 작업을 실행하는 컨테이너와 연결되어야 하는 컨테이너입니다. |
| `log_options`                      | `{"env": "GITLAB_CI_JOB_ID,GITLAB_CI_JOB_NAME", "labels": "com.gitlab.gitlab-runner.type"}` | `json-file` 로그 드라이버를 사용하는 Docker 컨테이너의 로그 드라이버 옵션입니다. `env` 및 `labels` 옵션만 허용됩니다. 자세한 내용은 [Docker 로그 옵션](#docker-log-options)을 참조하세요. |
| `memory`                           | `"128m"`                                         | 메모리 제한입니다. 문자열입니다. |
| `memory_swap`                      | `"256m"`                                         | 전체 메모리 제한입니다. 문자열입니다. |
| `memory_reservation`               | `"64m"`                                          | 메모리 소프트 제한입니다. 문자열입니다. |
| `network_mode`                     |                                                  | 컨테이너를 사용자 정의 네트워크에 추가합니다. |
| `mac_address`                      | `92:d0:c6:0a:29:33`                              | 컨테이너 MAC 주소 |
| `oom_kill_disable`                 |                                                  | 메모리 부족(`OOM`) 오류가 발생하면 컨테이너의 프로세스를 종료하지 마세요. |
| `oom_score_adjust`                 |                                                  | `OOM` 점수 조정입니다. 양수는 프로세스를 더 빨리 종료함을 의미합니다. |
| `privileged`                       | `false`                                          | 컨테이너를 권한 있는 모드에서 실행합니다. 안전하지 않습니다. |
| `services_privileged`              |                                                  | 서비스를 권한 있는 모드에서 실행하도록 허용합니다. 설정되지 않으면(기본값) `privileged` 값이 대신 사용됩니다. [Docker](../executors/docker.md#allow-docker-pull-policies) 실행기와 함께 사용합니다. 안전하지 않습니다. |
| `pull_policy`                      |                                                  | 이미지 끌어오기 정책: `never`, `if-not-present` 또는 `always`(기본값)입니다. [끌어오기 정책 설명서](../executors/docker.md#configure-how-runners-pull-images)에서 자세한 내용을 확인하세요. [여러 끌어오기 정책](../executors/docker.md#set-multiple-pull-policies) 을 추가하거나, [실패한 끌어오기를 재시도](../executors/docker.md#retry-a-failed-pull) 하거나, [끌어오기 정책을 제한](../executors/docker.md#allow-docker-pull-policies)할 수도 있습니다. |
| `runtime`                          |                                                  | Docker 컨테이너의 런타임입니다. |
| `isolation`                        |                                                  | 컨테이너 격리 기술입니다(`default`, `hyperv` 및 `process`). Windows만 해당합니다. |
| `security_opt`                     |                                                  | 보안 옵션입니다(`docker run`의 --security-opt). `:` 구분 키/값 목록을 사용합니다. `systempaths` 지정은 지원되지 않습니다. 자세한 내용은 [이슈 36810](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/36810)을 참조하세요. |
| `shm_size`                         | `300000`                                         | 이미지의 공유 메모리 크기입니다(바이트). |
| `sysctls`                          |                                                  | `sysctl` 옵션입니다. |
| `tls_cert_path`                    | macOS `/Users/<username>/.boot2docker/certs`에서. | `ca.pem`, `cert.pem` 또는 `key.pem`이 저장되고 Docker에 보안 TLS 연결을 만드는 데 사용되는 디렉터리입니다. `boot2docker`와 함께 이 설정을 사용합니다. |
| `tls_verify`                       |                                                  | Docker 데몬으로의 연결 TLS 검증을 활성화 또는 비활성화합니다. 기본적으로 비활성화됨. 기본적으로 GitLab Runner는 SSH를 통해 Docker Unix 소켓에 연결합니다. Unix 소켓은 RTLS를 지원하지 않으며 암호화 및 인증을 제공하기 위해 SSH를 통해 HTTP로 통신합니다. `tls_verify`을 활성화하는 것은 일반적으로 필요하지 않으며 추가 구성이 필요합니다. `tls_verify`을 활성화하려면 데몬이 포트에서 수신 대기해야 하고(기본 Unix 소켓이 아님) GitLab Runner Docker 호스트가 데몬이 수신 대기하는 주소를 사용해야 합니다. |
| `user`                             |                                                  | 컨테이너에서 지정된 사용자로 모든 명령을 실행합니다. |
| `userns_mode`                      |                                                  | 사용자 네임스페이스 다시 매핑 옵션이 활성화되었을 때 컨테이너 및 Docker 서비스의 사용자 네임스페이스 모드입니다. Docker 1.10 이상에서 사용 가능합니다. 자세한 내용은 [Docker 설명서](https://docs.docker.com/engine/security/userns-remap/#disable-namespace-remapping-for-a-container)를 참조하세요. |
| `ulimit`                           |                                                  | 컨테이너에 전달되는 Ulimit 값입니다. Docker `--ulimit` 플래그와 동일한 구문을 사용합니다. |
| `volume_keep`                      |                                                  | `true`일 때, 작업 후 러너가 컨테이너를 정리할 때 Docker 볼륨이 삭제되지 않습니다. 볼륨이 디스크에 누적됩니다. 운영자는 주기적 정리를 담당합니다(예: cron 작업에서 `docker volume prune`). 볼륨 제거가 Docker 데몬을 차단하는 높은 동시성 환경에서 이 설정을 사용합니다. 기본값은 `false`입니다. |
| `volumes`                          | `["/data", "/home/project/cache"]`               | 마운트되어야 하는 추가 볼륨입니다. Docker `-v` 플래그와 동일한 구문입니다. |
| `volumes_from`                     | `["storage_container:ro"]`                       | `<container name>[:<access_level>]` 형태의 다른 컨테이너에서 상속할 볼륨 목록입니다. 액세스 수준의 기본값은 읽기-쓰기이지만 `ro`(읽기 전용) 또는 `rw`(읽기-쓰기)로 수동으로 설정할 수 있습니다. |
| `volume_driver`                    |                                                  | 컨테이너에 사용할 볼륨 드라이버입니다. |
| `wait_for_services_timeout`        | `30`                                             | Docker 서비스를 기다릴 시간입니다. `-1`로 설정하여 비활성화합니다. 기본값은 `30`입니다. |
| `container_labels`                 |                                                  | 러너가 생성하는 각 컨테이너에 추가할 레이블 집합입니다. 레이블 값에 확장을 위한 환경 변수가 포함될 수 있습니다. |
| `services_limit`                   |                                                  | 작업당 최대 허용 서비스를 설정합니다. `-1`(기본값)은 제한이 없음을 의미합니다. |
| `service_cpuset_cpus`              |                                                  | 서비스에 사용할 `cgroups CpusetCpus` 문자열 값입니다. |
| `service_cpu_shares`               |                                                  | 서비스의 상대 CPU 사용량을 설정하는 데 사용되는 CPU 공유 수입니다(기본값: [`1024`](https://docs.docker.com/engine/containers/resource_constraints/#cpu)). |
| `service_cpus`                     |                                                  | 서비스의 CPU 수 문자열 값입니다. Docker 1.13 이상에서 사용 가능합니다. |
| `service_gpus`                     |                                                  | Docker 컨테이너의 GPU 장치입니다. `docker` CLI과 동일한 형식을 사용합니다. [Docker 설명서](https://docs.docker.com/engine/containers/resource_constraints/#gpu)에서 자세한 내용을 확인하세요. [GPU를 활성화하도록 구성](gpus.md#docker-executor)해야 합니다. |
| `service_memory`                   |                                                  | 서비스의 메모리 제한 문자열 값입니다. |
| `service_memory_swap`              |                                                  | 서비스의 전체 메모리 제한 문자열 값입니다. |
| `service_memory_reservation`       |                                                  | 서비스의 메모리 소프트 제한 문자열 값입니다. |

### `[[runners.docker.services]]` 섹션 {#the-runnersdockerservices-section}

작업과 함께 실행할 추가 [서비스](https://docs.gitlab.com/ci/services/)를 지정합니다. 사용 가능한 이미지 목록은 [Docker 레지스트리](https://hub.docker.com)를 참조하세요. 각 서비스는 별도의 컨테이너에서 실행되고 작업에 연결됩니다.

| 매개변수     | 예제                            | 설명 |
|---------------|------------------------------------|-------------|
| `name`        | `"registry.example.com/svc1"`      | 서비스로 실행될 이미지의 이름입니다. |
| `alias`       | `"svc1"`                           | 서비스에 액세스하는 데 사용할 수 있는 추가 [별칭 이름](https://docs.gitlab.com/ci/services/#available-settings-for-services)입니다. |
| `entrypoint`  | `["entrypoint.sh"]`                | 컨테이너의 진입점으로 실행할 명령 또는 스크립트입니다. 구문은 [Dockerfile ENTRYPOINT](https://docs.docker.com/reference/dockerfile/#entrypoint) 지시문과 유사하며, 각 셸 토큰은 배열의 별도 문자열입니다. [GitLab Runner 13.6에서 도입됨](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27173). |
| `command`     | `["executable","param1","param2"]` | 컨테이너의 명령으로 사용할 명령 또는 스크립트입니다. 구문은 [Dockerfile CMD](https://docs.docker.com/reference/dockerfile/#cmd) 지시문과 유사하며, 각 셸 토큰은 배열의 별도 문자열입니다. [GitLab Runner 13.6에서 도입됨](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27173). |
| `environment` | `["ENV1=value1", "ENV2=value2"]`   | 서비스 컨테이너의 환경 변수를 추가하거나 덮어씁니다. |

예제:

```toml
[runners.docker]
  host = ""
  hostname = ""
  tls_cert_path = "/Users/ayufan/.boot2docker/certs"
  image = "ruby:3.3"
  memory = "128m"
  memory_swap = "256m"
  memory_reservation = "64m"
  oom_kill_disable = false
  cpuset_cpus = "0,1"
  cpuset_mems = "0,1"
  cpus = "2"
  dns = ["8.8.8.8"]
  dns_search = [""]
  service_memory = "128m"
  service_memory_swap = "256m"
  service_memory_reservation = "64m"
  service_cpuset_cpus = "0,1"
  service_cpus = "2"
  services_limit = 5
  privileged = false
  group_add = ["docker"]
  cap_add = ["NET_ADMIN"]
  cap_drop = ["DAC_OVERRIDE"]
  devices = ["/dev/net/tun"]
  disable_cache = false
  wait_for_services_timeout = 30
  cache_dir = ""
  volumes = ["/data", "/home/project/cache"]
  extra_hosts = ["other-host:127.0.0.1"]
  shm_size = 300000
  volumes_from = ["storage_container:ro"]
  links = ["mysql_container:mysql"]
  allowed_images = ["ruby:*", "python:*", "php:*"]
  allowed_services = ["postgres:9", "redis:*", "mysql:*"]
  log_options = { env = "GITLAB_CI_JOB_ID,GITLAB_CI_JOB_NAME", labels = "com.gitlab.gitlab-runner.type" }
  [runners.docker.ulimit]
    "rtprio" = "99"
  [[runners.docker.services]]
    name = "registry.example.com/svc1"
    alias = "svc1"
    entrypoint = ["entrypoint.sh"]
    command = ["executable","param1","param2"]
    environment = ["ENV1=value1", "ENV2=value2"]
  [[runners.docker.services]]
    name = "redis:2.8"
    alias = "cache"
  [[runners.docker.services]]
    name = "postgres:9"
    alias = "postgres-db"
  [runners.docker.sysctls]
    "net.ipv4.ip_forward" = "1"
```

### `[runners.docker]` 섹션의 볼륨 {#volumes-in-the-runnersdocker-section}

볼륨에 대한 자세한 내용은 [Docker 설명서](https://docs.docker.com/engine/storage/volumes/)를 참조하세요.

다음 예제는 `[runners.docker]` 섹션에서 볼륨을 지정하는 방법을 보여줍니다.

#### 예제 1:  데이터 볼륨 추가 {#example-1-add-a-data-volume}

데이터 볼륨은 하나 이상의 컨테이너에서 Union File System을 우회하는 특별히 지정된 디렉터리입니다. 데이터 볼륨은 컨테이너의 수명 주기와 독립적으로 데이터를 유지하도록 설계되었습니다.

```toml
[runners.docker]
  host = ""
  hostname = ""
  tls_cert_path = "/Users/ayufan/.boot2docker/certs"
  image = "ruby:3.3"
  privileged = false
  disable_cache = true
  volumes = ["/path/to/volume/in/container"]
```

이 예제는 `/path/to/volume/in/container`에서 컨테이너의 새 볼륨을 만듭니다.

#### 예제 2:  호스트 디렉터리를 데이터 볼륨으로 마운트 {#example-2-mount-a-host-directory-as-a-data-volume}

컨테이너 외부에 디렉터리를 저장하려면 Docker 데몬의 호스트에서 컨테이너로 디렉터리를 마운트할 수 있습니다:

```toml
[runners.docker]
  host = ""
  hostname = ""
  tls_cert_path = "/Users/ayufan/.boot2docker/certs"
  image = "ruby:3.3"
  privileged = false
  disable_cache = true
  volumes = ["/path/to/bind/from/host:/path/to/bind/in/container:rw"]
```

이 예제는 CI/CD 호스트의 `/path/to/bind/from/host`을 `/path/to/bind/in/container`의 컨테이너에서 사용합니다.

GitLab Runner 11.11 이상 [호스트 디렉터리를 마운트](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/1261) 하며 정의된 [서비스](https://docs.gitlab.com/ci/services/)도 마찬가지입니다.

### Docker 로그 옵션 {#docker-log-options}

`log_options` 매개변수는 `json-file` 로그 드라이버의 Docker 컨테이너 로그 옵션을 구성합니다. 보안 및 호환성상의 이유로 `env` 및 `labels` 옵션만 지원됩니다.

#### 지원되는 로그 옵션 {#supported-log-options}

- `env`: 로그 항목에 포함할 환경 변수 이름의 쉼표로 구분된 목록
- `labels`: 로그 항목에 포함할 컨테이너 레이블 이름의 쉼표로 구분된 목록

#### 구성 예제 {#configuration-examples}

다음은 몇 가지 구성 예제입니다:

```toml
[[runners]]
  [runners.docker]
    # Include specific environment variables in logs
    log_options = { env = "GITLAB_CI_JOB_ID,GITLAB_CI_JOB_NAME,CI_PIPELINE_ID" }
```

```toml
[[runners]]
  [runners.docker]
    # Include container labels in logs
    log_options = { labels = "com.gitlab.gitlab-runner.type" }
```

```toml
[[runners]]
  [runners.docker]
    # Include both environment variables and labels
    log_options = { env = "GITLAB_CI_JOB_ID,GITLAB_CI_JOB_NAME", labels = "com.gitlab.gitlab-runner.type" }
```

#### 유효성 검사 및 오류 처리 {#validation-and-error-handling}

GitLab Runner는 실행기 준비 중에 로그 옵션을 검증합니다. `max-size`, `max-file`, 또는 `compress`와 같은 지원되지 않는 옵션을 지정하면 작업이 구성 오류로 즉시 실패합니다.

로그 옵션은 주 작업 컨테이너와 CI/CD 구성에 정의된 모든 서비스 컨테이너에 적용됩니다.

Docker 로깅에 대한 자세한 정보는 [Docker `json-file` 로그 드라이버 설명서](https://docs.docker.com/config/containers/logging/json-file/)를 참조하세요.

### 개인 컨테이너 레지스트리 사용 {#use-a-private-container-registry}

프라이빗 레지스트리를 작업의 이미지 소스로 사용하려면 [CI/CD 변수](https://docs.gitlab.com/ci/variables/) `DOCKER_AUTH_CONFIG`를 통해 권한을 구성합니다. 다음 중 하나에서 변수를 설정할 수 있습니다:

- [`file` 유형](https://docs.gitlab.com/ci/variables/#use-file-type-cicd-variables)으로 프로젝트의 CI/CD 설정
- `config.toml` 파일

`if-not-present` 풀 정책으로 개인 레지스트리를 사용하면 [보안상 영향](../security/_index.md#usage-of-private-docker-images-with-if-not-present-pull-policy)을 야기할 수 있습니다. 풀 정책 작동 방식에 대한 자세한 정보는 [러너가 이미지를 가져오는 방식 구성](../executors/docker.md#configure-how-runners-pull-images)을 참조하세요.

개인 컨테이너 레지스트리 사용에 대한 자세한 정보는 다음을 참조하세요:

- [개인 컨테이너 레지스트리에서 이미지 액세스](https://docs.gitlab.com/ci/docker/using_docker_images/#access-an-image-from-a-private-container-registry)
- [`.gitlab-ci.yml` 키워드 참조](https://docs.gitlab.com/ci/yaml/#image)

러너가 수행하는 단계는 다음과 같이 요약할 수 있습니다:

1. 이미지 이름에서 레지스트리 이름을 찾습니다.
1. 값이 비어 있지 않으면 실행기는 이 레지스트리의 인증 구성을 검색합니다.
1. 마지막으로 지정된 레지스트리에 해당하는 인증이 발견되면 후속 풀에서 사용합니다.

#### GitLab 통합 레지스트리 지원 {#support-for-gitlab-integrated-registry}

GitLab은 작업의 데이터와 함께 통합 레지스트리에 대한 자격 증명을 보냅니다. 이 자격 증명은 자동으로 레지스트리의 인증 매개변수 목록에 추가됩니다.

이 단계 후 레지스트리에 대한 인증은 `DOCKER_AUTH_CONFIG` 변수로 추가된 구성과 유사하게 진행됩니다.

작업에서는 GitLab 통합 레지스트리의 모든 이미지를 사용할 수 있습니다(이미지가 개인 또는 보호되어 있는 경우도 포함). 작업이 액세스할 수 있는 이미지에 대한 정보는 [CI/CD 작업 토큰 설명서](https://docs.gitlab.com/ci/jobs/ci_job_token/)를 읽으세요.

#### Docker 인증 해결 우선순위 {#precedence-of-docker-authorization-resolving}

앞서 설명한 대로 GitLab Runner는 다양한 방식으로 전송되는 자격 증명을 사용하여 레지스트리에 대해 Docker를 인증할 수 있습니다. 적절한 레지스트리를 찾기 위해 다음 우선순위를 고려합니다:

1. `DOCKER_AUTH_CONFIG`로 구성된 자격 증명.
1. GitLab Runner 호스트의 `~/.docker/config.json` 또는 `~/.dockercfg` 파일로 로컬로 구성된 자격 증명(예: 호스트에서 `docker login`를 실행하여).
1. 작업의 페이로드와 함께 기본으로 전송되는 자격 증명(예: 이전에 설명한 통합 레지스트리의 자격 증명).

레지스트리에 대해 처음 발견된 자격 증명이 사용됩니다. 예를 들어 `DOCKER_AUTH_CONFIG` 변수를 사용하여 통합 레지스트리에 대한 자격 증명을 추가하면 기본 자격 증명이 재정의됩니다.

## `[runners.parallels]` 섹션 {#the-runnersparallels-section}

다음 매개변수는 Parallels용입니다.

| 매개변수           | 설명 |
|---------------------|-------------|
| `base_name`         | 복제되는 Parallels VM의 이름입니다. |
| `template_name`     | Parallels VM 링크된 템플릿의 사용자 정의 이름입니다. 선택적입니다. |
| `disable_snapshots` | 비활성화하면 작업이 완료될 때 VM이 제거됩니다. |
| `allowed_images`    | 허용된 `image`/`base_name` 값의 목록(정규식으로 표현됨). [기본 VM 이미지 재정의](#overriding-the-base-vm-image) 섹션을 참조하세요. |

예제:

```toml
[runners.parallels]
  base_name = "my-parallels-image"
  template_name = ""
  disable_snapshots = false
```

## `[runners.virtualbox]` 섹션 {#the-runnersvirtualbox-section}

다음 매개변수는 VirtualBox용입니다. 이 실행기는 `vboxmanage` 실행 파일을 사용하여 VirtualBox 머신을 제어하므로 Windows 호스트에서 `PATH` 환경 변수를 조정해야 합니다: `PATH=%PATH%;C:\Program Files\Oracle\VirtualBox`.

| 매개변수           | 설명 |
|---------------------|-------------|
| `base_name`         | 복제되는 VirtualBox VM의 이름입니다. |
| `base_snapshot`     | VM의 특정 스냅샷의 이름 또는 UUID로 연결된 복제본을 만듭니다. 이 값이 비어 있거나 생략하면 현재 스냅샷이 사용됩니다. 현재 스냅샷이 없으면 생성됩니다. `disable_snapshots`이(가) true인 경우를 제외하고는 기본 VM의 전체 복제본이 만들어집니다. |
| `base_folder`       | 새로운 VM을 저장할 폴더입니다. 이 값이 비어 있거나 생략하면 기본 VM 폴더가 사용됩니다. |
| `disable_snapshots` | 비활성화하면 작업이 완료될 때 VM이 제거됩니다. |
| `allowed_images`    | 허용된 `image`/`base_name` 값의 목록(정규식으로 표현됨). [기본 VM 이미지 재정의](#overriding-the-base-vm-image) 섹션을 참조하세요. |
| `start_type`        | VM을 시작할 때의 그래픽 프론트엔드 유형입니다. |

예제:

```toml
[runners.virtualbox]
  base_name = "my-virtualbox-image"
  base_snapshot = "my-image-snapshot"
  disable_snapshots = false
  start_type = "headless"
```

`start_type` 매개변수는 가상 이미지를 시작할 때 사용되는 그래픽 프론트 엔드를 결정합니다. 유효한 값은 `headless`(기본값), `gui` 또는 `separate`이며 호스트 및 게스트 조합에서 지원됩니다.

## 기본 VM 이미지 재정의 {#overriding-the-base-vm-image}

Parallels 및 VirtualBox 실행기 모두에서 `base_name`로 지정된 기본 VM 이름을 재정의할 수 있습니다. 이 작업을 수행하려면 `.gitlab-ci.yml` 파일에서 [image](https://docs.gitlab.com/ci/yaml/#image) 매개변수를 사용합니다.

이전 버전과의 호환성을 위해 기본값으로 이 값을 재정의할 수 없습니다. `base_name`로 지정된 이미지만 허용됩니다.

`.gitlab-ci.yml` [image](https://docs.gitlab.com/ci/yaml/#image) 매개변수를 사용하여 사용자가 VM 이미지를 선택하도록 허용하려면:

```toml
[runners.virtualbox]
  ...
  allowed_images = [".*"]
```

예제에서는 기존의 모든 VM 이미지를 사용할 수 있습니다.

`allowed_images` 매개변수는 정규식의 목록입니다. 구성은 필요한 만큼 정확할 수 있습니다. 예를 들어 특정 VM 이미지만 허용하려는 경우 다음과 같은 정규식을 사용할 수 있습니다:

```toml
[runners.virtualbox]
  ...
  allowed_images = ["^allowed_vm[1-2]$"]
```

이 예제에서는 `allowed_vm1` 및 `allowed_vm2`만 허용됩니다. 다른 모든 시도는 오류가 발생합니다.

## `[runners.ssh]` 섹션 {#the-runnersssh-section}

다음 매개변수는 SSH 연결을 정의합니다.

| 매개변수                          | 설명 |
|------------------------------------|-------------|
| `host`                             | 연결할 위치입니다. |
| `port`                             | 포트입니다. 기본값은 `22`입니다. |
| `user`                             | 사용자 이름입니다.   |
| `password`                         | 암호입니다.   |
| `identity_file`                    | SSH 개인 키 파일 경로(`id_rsa`, `id_dsa` 또는 `id_edcsa`). 파일은 암호화되지 않은 상태로 저장되어야 합니다. |
| `disable_strict_host_key_checking` | 이 값은 러너가 엄격한 호스트 키 확인을 사용해야 하는지 여부를 결정합니다. 기본값은 `true`입니다. GitLab 15.0에서 기본값 또는 지정하지 않은 경우의 값은 `false`입니다. |

예제:

```toml
[runners.ssh]
  host = "my-production-server"
  port = "22"
  user = "root"
  password = "production-server-password"
  identity_file = ""
```

## `[runners.machine]` 섹션 {#the-runnersmachine-section}

다음 매개변수는 Docker Machine 기반 자동 크기 조정 기능을 정의합니다. 자세한 정보는 [Docker Machine Executor 자동 크기 조정 구성](autoscale.md)을 참조하세요.

| 매개변수                         | 설명 |
|-----------------------------------|-------------|
| `MaxGrowthRate`                   | 러너에 병렬로 추가할 수 있는 최대 머신 수입니다. 기본값은 `0`(제한 없음)입니다. |
| `IdleCount`                       | _유휴_ 상태로 생성되고 대기 중인 머신의 수입니다. |
| `IdleScaleFactor`                 | 사용 중인 머신 수의 배수인 _유휴_ 머신의 수입니다. 부동 소수점 수 형식이어야 합니다. [자동 크기 조정 설명서](autoscale.md#the-idlescalefactor-strategy)를 참조하세요. 기본값은 `0.0`입니다. |
| `IdleCountMin`                    | `IdleScaleFactor`가 사용 중일 때 _유휴_ 상태로 생성되고 대기 중인 최소 머신 수입니다. 기본값은 1입니다. |
| `IdleTime`                        | 머신이 제거되기 전에 _유휴_ 상태로 있는 시간(초)입니다. |
| `[[runners.machine.autoscaling]]` | 각각 자동 크기 조정 구성의 재정의를 포함하는 여러 섹션입니다. 현재 시간과 일치하는 마지막 섹션이 선택됩니다. |
| `OffPeakPeriods`                  | 더 이상 사용되지 않음:  스케줄러가 OffPeak 모드에 있을 때의 시간 기간입니다. 크론 스타일 패턴의 배열([아래](#periods-syntax) 설명). |
| `OffPeakTimezone`                 | 더 이상 사용되지 않음:  OffPeakPeriods에 지정된 시간의 시간대입니다. `Europe/Berlin`과 같은 시간대 문자열입니다. 생략하거나 비어 있으면 호스트의 로케일 시스템 설정으로 기본값이 설정됩니다. GitLab Runner는 `ZONEINFO` 환경 변수로 지정된 디렉토리 또는 압축되지 않은 zip 파일에서 시간대 데이터베이스를 찾으려고 시도하고, Unix 시스템의 알려진 설치 위치를 확인한 다음 최종적으로 `$GOROOT/lib/time/zoneinfo.zip`를 확인합니다. |
| `OffPeakIdleCount`                | 더 이상 사용되지 않음:  `IdleCount`과 같지만 _Off Peak_ 시간대용입니다. |
| `OffPeakIdleTime`                 | 더 이상 사용되지 않음:  `IdleTime`과 같지만 _Off Peak_ 시간대용입니다. |
| `MaxBuilds`                       | 머신이 제거되기 전의 최대 작업 수(빌드)입니다. |
| `MachineName`                     | 머신의 이름입니다. **must** `%s`를 포함해야 하며 고유한 머신 식별자로 바뀝니다. |
| `MachineDriver`                   | Docker Machine `driver`. [Docker Machine 구성의 클라우드 공급자 섹션](autoscale.md#supported-cloud-providers)에서 세부 정보를 보세요. |
| `MachineOptions`                  | MachineDriver에 대한 Docker Machine 옵션입니다. 자세한 정보는 [지원되는 클라우드 공급자](autoscale.md#supported-cloud-providers)를 참조하세요. AWS의 모든 옵션에 대한 자세한 정보는 Docker Machine 리포지토리의 [AWS](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/aws.md) 및 [GCP](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/gce.md) 프로젝트를 참조하세요. |

### `[[runners.machine.autoscaling]]` 섹션 {#the-runnersmachineautoscaling-sections}

다음 매개변수는 [Instance](../executors/instance.md) 또는 [Docker Autoscaler](../executors/docker_autoscaler.md#example-aws-autoscaling-for-1-job-per-instance) 실행기를 사용할 때 사용 가능한 구성을 정의합니다.

| 매개변수         | 설명 |
|-------------------|-------------|
| `Periods`         | 이 스케줄이 활성 상태인 시간 기간입니다. 크론 스타일 패턴의 배열([아래](#periods-syntax) 설명). |
| `IdleCount`       | _유휴_ 상태로 생성되고 대기 중인 머신의 수입니다. |
| `IdleScaleFactor` | (실험) 사용 중인 머신 수의 배수인 _유휴_ 머신의 수입니다. 부동 소수점 수 형식이어야 합니다. [자동 크기 조정 설명서](autoscale.md#the-idlescalefactor-strategy)를 참조하세요. 기본값은 `0.0`입니다. |
| `IdleCountMin`    | `IdleScaleFactor`가 사용 중일 때 _유휴_ 상태로 생성되고 대기 중인 최소 머신 수입니다. 기본값은 1입니다. |
| `IdleTime`        | 머신이 제거되기 전에 _유휴_ 상태로 있는 시간(초)입니다. |
| `Timezone`        | `Periods`에 지정된 시간의 시간대입니다. `Europe/Berlin`과 같은 시간대 문자열입니다. 생략하거나 비어 있으면 호스트의 로케일 시스템 설정으로 기본값이 설정됩니다. GitLab Runner는 `ZONEINFO` 환경 변수로 지정된 디렉토리 또는 압축되지 않은 zip 파일에서 시간대 데이터베이스를 찾으려고 시도하고, Unix 시스템의 알려진 설치 위치를 확인한 다음 최종적으로 `$GOROOT/lib/time/zoneinfo.zip`를 확인합니다. |

예제:

```toml
[runners.machine]
  IdleCount = 5
  IdleTime = 600
  MaxBuilds = 100
  MachineName = "auto-scale-%s"
  MachineDriver = "google" # Refer to Docker Machine docs on how to authenticate: https://docs.docker.com/machine/drivers/gce/#credentials
  MachineOptions = [
      # Additional machine options can be added using the Google Compute Engine driver.
      # If you experience problems with an unreachable host (ex. "Waiting for SSH"),
      # you should remove optional parameters to help with debugging.
      # https://docs.docker.com/machine/drivers/gce/
      "google-project=GOOGLE-PROJECT-ID",
      "google-zone=GOOGLE-ZONE", # e.g. 'us-central1-a', full list in https://cloud.google.com/compute/docs/regions-zones/
  ]
  [[runners.machine.autoscaling]]
    Periods = ["* * 9-17 * * mon-fri *"]
    IdleCount = 50
    IdleCountMin = 5
    IdleScaleFactor = 1.5 # Means that current number of Idle machines will be 1.5*in-use machines,
                          # no more than 50 (the value of IdleCount) and no less than 5 (the value of IdleCountMin)
    IdleTime = 3600
    Timezone = "UTC"
  [[runners.machine.autoscaling]]
    Periods = ["* * * * * sat,sun *"]
    IdleCount = 5
    IdleTime = 60
    Timezone = "UTC"
```

### 기간 구문 {#periods-syntax}

`Periods` 설정에는 크론 스타일 형식으로 표현된 시간 기간의 문자열 패턴 배열이 포함됩니다. 라인에는 다음 필드가 포함됩니다:

```plaintext
[second] [minute] [hour] [day of month] [month] [day of week] [year]
```

표준 cron 구성 파일과 마찬가지로 필드에는 단일 값, 범위, 목록 및 별표가 포함될 수 있습니다. [구문의 자세한 설명](https://github.com/gorhill/cronexpr#implementation)을 보세요.

## `[runners.instance]` 섹션 {#the-runnersinstance-section}

| 매개변수        | 유형   | 설명 |
|------------------|--------|-------------|
| `allowed_images` | 문자열 | VM 격리가 활성화되면 `allowed_images`는 작업이 지정할 수 있는 이미지를 제어합니다. |

## `[runners.autoscaler]` 섹션 {#the-runnersautoscaler-section}

{{< history >}}

- GitLab Runner v15.10.0에서 도입되었습니다.

{{< /history >}}

다음 매개변수는 자동 스케일러 기능을 구성합니다. 이 매개변수는 [Instance](../executors/instance.md) 및 [Docker Autoscaler](../executors/docker_autoscaler.md) 실행기와만 함께 사용할 수 있습니다.

| 매개변수                        | 설명 |
|----------------------------------|-------------|
| `capacity_per_instance`          | 단일 인스턴스에서 동시에 실행할 수 있는 작업의 수입니다. |
| `max_use_count`                  | 제거가 예약되기 전에 인스턴스를 사용할 수 있는 최대 횟수입니다. |
| `max_instances`                  | 허용되는 최대 인스턴스 수입니다(인스턴스 상태(대기 중, 실행 중, 삭제 중)와 관계없이). 기본값: `0`(무제한). |
| `plugin`                         | 사용할 [fleeting](https://gitlab.com/gitlab-org/fleeting/fleeting) 플러그인입니다. 플러그인을 설치하고 참조하는 방법에 대한 자세한 정보는 [fleeting 플러그인 설치](../fleet_scaling/fleeting.md#install-a-fleeting-plugin)를 참조하세요. |
| `delete_instances_on_shutdown`   | GitLab Runner가 종료될 때 프로비저닝된 모든 인스턴스가 삭제되는지 여부를 지정합니다. 기본값: `false`. [GitLab Runner 15.11](https://gitlab.com/gitlab-org/fleeting/taskscaler/-/merge_requests/24)에서 도입되었습니다. |
| `instance_ready_command`         | 자동 스케일러에서 프로비저닝한 각 인스턴스에서 이 명령을 실행하여 사용할 준비가 되었는지 확인합니다. 실패하면 인스턴스가 제거됩니다. [GitLab Runner 16.11](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/37473)에서 도입되었습니다. |
| `instance_acquire_timeout`       | 러너가 인스턴스를 획득하기 위해 대기하는 최대 기간(시간 초과 이전)입니다. 기본값:  `15m`(15분). 환경에 맞게 이 값을 조정할 수 있습니다. [GitLab Runner 18.1](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5563)에서 도입되었습니다. |
| `update_interval`                | 인스턴스 업데이트를 위해 fleeting 플러그인을 확인하는 간격입니다. 기본값:  `1m`(1분). [GitLab Runner 16.11](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4722)에서 도입되었습니다. |
| `update_interval_when_expecting` | 상태 변경이 예상될 때 인스턴스 업데이트를 위해 fleeting 플러그인을 확인하는 간격입니다. 예를 들어 인스턴스가 인스턴스를 프로비저닝하고 러너가 `pending`에서 `running`로 전환되기를 기다리고 있을 때입니다. 기본값:  `2s`(2초). [GitLab Runner 16.11](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4722)에서 도입되었습니다. |
| `deletion_retry_interval` | 이전 삭제 시도가 효과가 없었을 때 fleeting 플러그인이 삭제를 재시도하기 전에 대기하는 간격입니다. 기본값:  `1m`(1분). [GitLab Runner 18.4](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5777)에서 도입되었습니다. |
| `shutdown_deletion_interval`| 종료 중 인스턴스를 제거하고 상태를 확인하는 사이에 fleeting 플러그인이 사용하는 간격입니다. 기본값:  `10s`(10초). [GitLab Runner 18.4](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5777)에서 도입되었습니다. |
| `shutdown_deletion_retries` | 종료 전 인스턴스 삭제가 완료되도록 fleeting 플러그인이 수행하는 최대 시도 횟수입니다. 기본값: `3`. [GitLab Runner 18.4](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5777)에서 도입되었습니다. |
| `failure_threshold` | fleeting 플러그인이 인스턴스를 교체하기 전의 최대 연속 상태 확인 실패 횟수입니다. 하트비트 기능도 참조하세요. 기본값: `3`. [GitLab Runner 18.4](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5777)에서 도입되었습니다. |
| `log_internal_ip`                | CI/CD 출력이 VM의 내부 IP 주소를 기록하는지 여부를 지정합니다. 기본값: `false`. [GitLab Runner 18.1](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5519)에서 도입되었습니다. |
| `log_external_ip`                | CI/CD 출력이 VM의 외부 IP 주소를 기록하는지 여부를 지정합니다. 기본값: `false`. [GitLab Runner 18.1](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5519)에서 도입되었습니다. |

`instance_ready_command`이(가) 유휴 스케일 규칙으로 자주 실패하면 인스턴스가 러너가 작업을 승인하는 것보다 빠르게 제거되고 생성될 수 있습니다. 스케일 제한을 지원하기 위해 [GitLab 17.0](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/37497)에서 지수 백오프가 추가되었습니다.

> [!note]
> 자동 스케일러 구성 옵션은 구성 변경과 함께 다시 로드되지 않습니다. 그러나 GitLab 17.5.0 이상에서는 `[[runners.autoscaler.policy]]` 항목이 구성 변경 시 다시 로드됩니다.

## `[runners.autoscaler.plugin_config]` 섹션 {#the-runnersautoscalerplugin_config-section}

이 해시 테이블은 JSON으로 다시 인코딩되어 구성된 플러그인에 직접 전달됩니다.

[fleeting](https://gitlab.com/gitlab-org/fleeting/fleeting) 플러그인은 일반적으로 지원되는 구성에 대한 동반 설명서를 제공합니다.

## `[runners.autoscaler.scale_throttle]` 섹션 {#the-runnersautoscalerscale_throttle-section}

{{< history >}}

- GitLab Runner v17.0.0에서 도입되었습니다.

{{< /history >}}

| 매개변수 | 설명 |
|-----------|-------------|
| `limit`   | 프로비저닝할 수 있는 초당 새 인스턴스의 속도 제한입니다. `-1`은 무한합니다. 기본값(`0`)은 제한을 `100`로 설정합니다. |
| `burst`   | 새 인스턴스의 버스트 제한입니다. `max_instances`이(가) 설정되지 않은 경우 `max_instances` 또는 `limit`로 기본값이 설정됩니다. `limit`이(가) 무한이면 `burst`은 무시됩니다. |

### `limit` 및 `burst` 간의 관계 {#relationship-between-limit-and-burst}

스케일 제한은 토큰 할당량 시스템을 사용하여 인스턴스를 생성합니다. 이 시스템은 두 가지 값으로 정의됩니다:

- `burst`: 할당량의 최대 크기입니다.
- `limit`: 할당량이 초당 새로 고쳐지는 속도입니다.

한 번에 생성할 수 있는 인스턴스의 수는 남은 할당량에 따라 달라집니다. 충분한 할당량이 있으면 해당 양까지 인스턴스를 생성할 수 있습니다. 할당량이 소진되면 초당 `limit` 인스턴스를 생성할 수 있습니다. 인스턴스 생성이 중지되면 할당량이 초당 `limit` 씩 증가하여 `burst` 값에 도달합니다.

예를 들어 `limit`이 `1`이고 `burst`이 `60`이면:

- 60개의 인스턴스를 즉시 생성할 수 있지만 제한이 있습니다.
- 60초를 기다리면 다시 60개의 인스턴스를 즉시 생성할 수 있습니다.
- 기다리지 않으면 초당 1개의 인스턴스를 생성할 수 있습니다.

## `[runners.autoscaler.connector_config]` 섹션 {#the-runnersautoscalerconnector_config-section}

[fleeting](https://gitlab.com/gitlab-org/fleeting/fleeting) 플러그인은 일반적으로 지원되는 연결 옵션에 대한 동반 설명서를 제공합니다.

플러그인이 커넥터 구성을 자동으로 업데이트합니다. `[runners.autoscaler.connector_config]`를 사용하여 커넥터 구성의 자동 업데이트를 재정의하거나 플러그인이 결정할 수 없는 빈 값을 채울 수 있습니다.

| 매개변수                | 설명 |
|--------------------------|-------------|
| `os`                     | 인스턴스의 운영 체제입니다. |
| `arch`                   | 인스턴스의 아키텍처입니다. |
| `protocol`               | `ssh`, `winrm` 또는 `winrm+https`. Windows가 감지되면 `winrm`이(가) 기본값으로 사용됩니다. |
| `protocol_port`          | 지정된 프로토콜을 기반으로 연결을 설정하는 데 사용되는 포트입니다. `ssh:22`, `winrm+http:5985`, `winrm+https:5986`로 기본값이 설정됩니다. |
| `username`               | 연결하는 데 사용되는 사용자 이름입니다. |
| `password`               | 연결하는 데 사용되는 암호입니다. |
| `key_path`               | 연결하거나 자격 증명을 동적으로 프로비저닝하는 데 사용되는 TLS 키입니다. |
| `use_static_credentials` | 자동 자격 증명 프로비저닝을 비활성화합니다. 기본값: `false`. |
| `keepalive`              | 연결 킵얼라이브 지속 시간입니다. |
| `timeout`                | 연결 시간 초과 기간입니다. |
| `use_external_addr`      | 플러그인에서 제공하는 외부 주소를 사용할지 여부입니다. 플러그인이 내부 주소만 반환하면 이 설정과 관계없이 사용됩니다. 기본값: `false`. |

## `[runners.autoscaler.state_storage]` 섹션 {#the-runnersautoscalerstate_storage-section}

{{< details >}}

- 상태:  베타

{{< /details >}}

{{< history >}}

- GitLab Runner 17.5.0에서 도입되었습니다.

{{< /history >}}

상태 저장소가 비활성화된 상태(기본값)에서 GitLab Runner가 시작되면 기존 fleeting 인스턴스는 안전을 위해 즉시 제거됩니다. 예를 들어 `max_use_count`이 `1`로 설정된 경우 사용 상태를 모르면 이미 사용된 인스턴스에 작업을 부주의로 할당할 수 있습니다.

상태 저장소 기능을 활성화하면 인스턴스의 상태가 로컬 디스크에 유지될 수 있습니다. 이 경우 GitLab Runner가 시작될 때 인스턴스가 존재하면 삭제되지 않습니다. 캐시된 연결 세부 정보, 사용 수 및 기타 구성이 복원됩니다.

상태 저장소 기능을 활성화할 때 다음 정보를 고려하세요:

- 인스턴스의 인증 세부 정보(사용자 이름, 암호, 키)는 디스크에 남아 있습니다.
- 인스턴스가 작업 실행 중에 복원되면 GitLab Runner는 기본적으로 이를 제거합니다. 이 동작은 GitLab Runner가 작업을 재개할 수 없으므로 안전을 보장합니다. 인스턴스를 유지하려면 `keep_instance_with_acquisitions`을(를) `true`로 설정합니다.

  `keep_instance_with_acquisitions`을(를) `true`로 설정하면 인스턴스의 진행 중인 작업에 대해 걱정하지 않을 때 도움이 됩니다. `instance_ready_command` 구성 옵션을 사용하여 인스턴스를 유지하기 위해 환경을 정리할 수도 있습니다. 이에 따라 모든 실행 중인 명령을 중지하거나 Docker 컨테이너를 강제로 제거해야 할 수 있습니다.

| 매개변수                         | 설명 |
|-----------------------------------|-------------|
| `enabled`                         | 상태 저장소가 활성화되는지 여부입니다. 기본값: `false`. |
| `dir`                             | 상태 저장소 디렉토리입니다. 각 러너 구성 항목에는 여기에 하위 디렉토리가 있습니다. 기본값: GitLab Runner 구성 파일 디렉토리의 `.taskscaler`. |
| `keep_instance_with_acquisitions` | 활성 작업이 있는 인스턴스가 제거되는지 여부입니다. 기본값: `false`. |

## `[[runners.autoscaler.policy]]` 섹션 {#the-runnersautoscalerpolicy-sections}

**노트** - `idle_count`은 이 컨텍스트에서 레거시 자동 크기 조정 방법에서와 같이 자동 크기 조정된 머신 수가 아닌 작업 수를 의미합니다.

| 매개변수            | 설명 |
|----------------------|-------------|
| `periods`            | 이 정책이 활성화된 기간을 나타내는 unix-cron 형식의 문자열 배열입니다. 기본값: `* * * * *` |
| `timezone`           | unix-cron 기간을 평가할 때 사용되는 시간대입니다. 기본값:  시스템의 로컬 시간대입니다. |
| `idle_count`         | 즉시 작업에 사용할 수 있도록 하려는 목표 유휴 용량입니다. |
| `idle_time`          | 인스턴스가 종료되기 전에 유휴 상태로 있을 수 있는 시간입니다. |
| `scale_factor`       | `idle_count` 위에 즉시 작업에 사용할 수 있도록 하려는 목표 유휴 용량(현재 사용 중인 용량의 배수)입니다. 기본값은 `0.0`입니다. |
| `scale_factor_limit` | `scale_factor` 계산이 생성할 수 있는 최대 용량입니다. |
| `preemptive_mode`    | 선제적 모드가 켜져 있으면 인스턴스를 사용할 수 있음이 확인된 경우에만 작업이 요청됩니다. 이 작업을 통해 프로비저닝 지연 없이 작업이 거의 즉시 시작될 수 있습니다. 선제적 모드가 꺼져 있으면 먼저 작업이 요청된 후 시스템이 필요한 용량을 찾거나 프로비저닝하려고 시도합니다. |

유휴 인스턴스를 제거할지 여부를 결정하기 위해 작업 스케일러는 `idle_time`을(를) 인스턴스의 유휴 기간과 비교합니다. 각 인스턴스의 유휴 기간은 다음 시간부터 계산됩니다:

- 마지막으로 작업을 완료한 시간(인스턴스를 이전에 사용한 경우).
- 프로비저닝됨(사용한 적이 없는 경우).

이 확인은 스케일링 이벤트 중에 발생합니다. 구성된 `idle_time`을(를) 초과하는 인스턴스는 필요한 `idle_count` 작업 용량을 유지해야 하지 않는 한 제거됩니다.

`scale_factor`이 설정되면 `idle_count`는 최소 `idle` 용량이 되고 `scaler_factor_limit`는 최대 `idle` 용량이 됩니다.

여러 정책을 정의할 수 있습니다. 마지막으로 일치하는 정책이 사용됩니다.

다음 예에서는 유휴 수 `1`이 월요일부터 금요일까지 오전 8시부터 오후 3시 59분(15:59)까지 사용됩니다. 그 외에는 유휴 수가 0입니다.

```toml
[[runners.autoscaler.policy]]
  idle_count        = 0
  idle_time         = "0s"
  periods           = ["* * * * *"]

[[runners.autoscaler.policy]]
  idle_count        = 1
  idle_time         = "30m0s"
  periods           = ["* 8-15 * * mon-fri"]
```

### 기간 구문 {#periods-syntax-1}

`periods` 설정에는 정책이 활성화된 기간을 나타내는 unix-cron 형식의 문자열 배열이 포함됩니다. cron 형식은 5개의 필드로 구성됩니다:

```plaintext
 ┌────────── minute (0 - 59)
 │ ┌──────── hour (0 - 23)
 │ │ ┌────── day of month (1 - 31)
 │ │ │ ┌──── month (1 - 12)
 │ │ │ │ ┌── day of week (1 - 7 or MON-SUN, 0 is an alias for Sunday)
 * * * * *
```

- `-`은(는) 두 숫자 사이의 범위를 지정하는 데 사용할 수 있습니다.
- `*`은(는) 해당 필드의 전체 유효 값 범위를 나타내는 데 사용할 수 있습니다.
- `/` 뒤에 숫자가 오거나 범위 뒤에 올 수 있으며 범위를 통해 해당 숫자를 건너뜁니다. 예를 들어 시간 필드에 대해 0-12/2는 오전 00:00과 00:12 사이의 시간마다 2시간마다 기간을 활성화합니다.
- `,`은(는) 필드의 유효한 숫자 또는 범위 목록을 분리하는 데 사용할 수 있습니다. 예를 들어, `1,2,6-9`.

이 크론 작업이 시간 범위를 나타낸다는 점을 명심하세요. 예를 들어:

| 기간               | 영향 |
|----------------------|--------|
| `1 * * * * *`        | 시간당 1분 동안 활성화된 규칙(효과가 있을 가능성이 낮음) |
| `* 0-12 * * *`       | 매일 시작 시 12시간 동안 활성화된 규칙 |
| `0-30 13,16 * * SUN` | 매주 일요일에 오후 1시에 30분, 오후 4시에 30분 동안 활성화된 규칙입니다. |

## `[runners.autoscaler.vm_isolation]` 섹션 {#the-runnersautoscalervm_isolation-section}

VM 격리는 [`nesting`](../executors/instance.md#nested-virtualization)을 사용하며 macOS에서만 지원됩니다.

| 매개변수        | 설명 |
|------------------|-------------|
| `enabled`        | VM 격리가 활성화되는지 여부를 지정합니다. 기본값: `false`. |
| `nesting_host`   | `nesting` 데몬 호스트입니다. |
| `nesting_config` | `nesting` 구성으로 JSON으로 직렬화되어 `nesting` 데몬으로 전송됩니다. |
| `image`          | 작업 이미지가 지정되지 않은 경우 nesting 데몬이 사용하는 기본 이미지입니다. |

## `[runners.autoscaler.vm_isolation.connector_config]` 섹션 {#the-runnersautoscalervm_isolationconnector_config-section}

`[runners.autoscaler.vm_isolation.connector_config]` 섹션에 대한 매개변수는 [`[runners.autoscaler.connector_config]`](#the-runnersautoscalerconnector_config-section) 섹션과 동일하지만 자동 확장 인스턴스가 아닌 `nesting` 프로비저닝된 가상 머신에 연결하는 데 사용됩니다.

## `[runners.custom]` 섹션 {#the-runnerscustom-section}

다음 매개변수는 [custom 실행기](../executors/custom.md) 구성을 정의합니다.

| 매개변수               | 유형         | 설명 |
|-------------------------|--------------|-------------|
| `config_exec`           | 문자열       | 사용자가 작업이 시작되기 전에 일부 구성 설정을 재정의할 수 있도록 하는 실행 파일의 경로입니다. 이 값은 [`[[runners]]`](#the-runners-section) 섹션에서 설정된 값을 재정의합니다. [custom 실행기 설명서](../executors/custom.md#config)에 전체 목록이 있습니다. |
| `config_args`           | 문자열 배열 | `config_exec` 실행 파일로 전달되는 첫 번째 인수 집합입니다. |
| `config_exec_timeout`   | 정수      | `config_exec`이 실행을 완료할 때까지의 시간 초과(초)입니다. 기본값은 3600초(1시간)입니다. |
| `prepare_exec`          | 문자열       | 환경을 준비하기 위한 실행 파일의 경로입니다. |
| `prepare_args`          | 문자열 배열 | `prepare_exec` 실행 파일로 전달되는 첫 번째 인수 집합입니다. |
| `prepare_exec_timeout`  | 정수      | `prepare_exec`이 실행을 완료할 때까지의 시간 초과(초)입니다. 기본값은 3600초(1시간)입니다. |
| `run_exec`              | 문자열       | **필수**. 환경에서 스크립트를 실행하기 위한 실행 파일의 경로입니다. 예를 들어 복제 및 빌드 스크립트입니다. |
| `run_args`              | 문자열 배열 | `run_exec` 실행 파일로 전달되는 첫 번째 인수 집합입니다. |
| `cleanup_exec`          | 문자열       | 환경을 정리하기 위한 실행 파일의 경로입니다. |
| `cleanup_args`          | 문자열 배열 | `cleanup_exec` 실행 파일로 전달되는 첫 번째 인수 집합입니다. |
| `cleanup_exec_timeout`  | 정수      | `cleanup_exec`이 실행을 완료할 때까지의 시간 초과(초)입니다. 기본값은 3600초(1시간)입니다. |
| `graceful_kill_timeout` | 정수      | 종료된 경우 `prepare_exec` 및 `cleanup_exec`를 기다릴 시간(초)(예: 작업 취소 중). 이 시간 초과 후에는 프로세스가 종료됩니다. 기본값은 600초(10분)입니다. |
| `force_kill_timeout`    | 정수      | 종료 신호가 스크립트로 전송된 후 대기 시간(초)입니다. 기본값은 600초(10분)입니다. |

## `[runners.cache]` 섹션 {#the-runnerscache-section}

다음 매개변수는 분산 캐시 기능을 정의합니다. [러너 자동 크기 조정 설명서](autoscale.md#distributed-runners-caching)에서 세부 정보를 보세요.

| 매개변수                | 유형    | 설명 |
|--------------------------|---------|-------------|
| `Type`                   | 문자열  | 다음 중 하나: `s3`, `gcs`, `azure`. |
| `Path`                   | 문자열  | 캐시 URL에 앞에 붙일 경로의 이름입니다. |
| `Shared`                 | 부울 | 러너 간 캐시 공유를 활성화합니다. 기본값은 `false`입니다. |
| `MaxUploadedArchiveSize` | int64   | 클라우드 저장소에 업로드되는 캐시 아카이브의 제한(바이트 단위)입니다. 악의적인 행위자는 이 제한을 우회할 수 있으므로 GCS 어댑터는 서명된 URL에서 X-Goog-Content-Length-Range 헤더를 통해 적용합니다. 클라우드 저장소 공급자에서도 제한을 설정해야 합니다. |

다음 환경 변수를 사용하여 캐시 압축을 구성할 수 있습니다:

| 변수                   | 설명                           | 기본값   | 값                                          |
|----------------------------|---------------------------------------|-----------|-------------------------------------------------|
| `CACHE_COMPRESSION_FORMAT` | 캐시 아카이브의 압축 형식 | `zip`     | `zip`, `tarzstd`                                |
| `CACHE_COMPRESSION_LEVEL`  | 캐시 아카이브의 압축 수준  | `default` | `fastest`, `fast`, `default`, `slow`, `slowest` |

`tarzstd` 형식은 Zstandard 압축으로 TAR을 사용하며 `zip`보다 더 나은 압축 비율을 제공합니다. 압축 수준은 `fastest`(최소 압축으로 최대 속도)에서 `slowest`(최대 압축으로 최소 파일 크기)까지입니다. `default` 수준은 압축 비율과 속도 간의 균형 잡힌 절충안을 제공합니다.

예제:

```yaml
job:
  variables:
    CACHE_COMPRESSION_FORMAT: tarzstd
    CACHE_COMPRESSION_LEVEL: fast
```

### 병렬 캐시 객체 저장소 전송 {#parallel-cache-object-storage-transfers}

기본적으로 캐시 다운로드는 단일 HTTP GET 또는 GoCloud 읽기 스트림을 사용하고 GoCloud 경로를 사용하는 캐시 업로드(예: `RoleARN`이 있는 S3)는 한 번에 하나의 동시 멀티파트 부분을 사용합니다.

`FF_USE_PARALLEL_CACHE_TRANSFER` [기능 플래그](feature-flags.md)로 빠른 링크에 대한 더 높은 처리량을 활성화할 수 있습니다. 활성화되면:

- **다운로드**는 백엔드에서 범위를 지원하고 캐시 객체가 하나의 청크보다 클 때 여러 동시 범위 GET(서명된 URL, HEAD가 실패하는 GET 전용 서명된 URL(예: S3)을 위해 HEAD 대신 작은 초기 범위 요청이 사용됨) 또는 동시 GoCloud 범위 읽기를 사용할 수 있습니다.
- **업로드**는 GoCloud 경로에서 동시 부분으로 멀티파트 업로드를 사용합니다.

기능 플래그가 꺼져 있으면 아래 변수와 관계없이 동작이 변하지 않습니다. 이러한 작업 환경 변수(`cache-extractor` 및 `cache-archiver` 도우미에서 읽음)로 병렬 처리를 조정할 수 있습니다:

| 변수                     | 설명                                                                 | 기본값 |
|------------------------------|-----------------------------------------------------------------------------|---------|
| `CACHE_CHUNK_SIZE`           | 병렬 범위 다운로드 및 GoCloud 업로드의 멀티파트 부분 크기에 대한 청크 크기(바이트 단위) | `16777216`(16MiB) |
| `CACHE_CONCURRENCY`          | 동시 범위 다운로드 또는 동시 업로드 부분(GoCloud) 수입니다. 순차 다운로드의 경우 `0` 또는 `1`를 사용합니다. | `16` |
| `CACHE_TRANSFER_BUFFER_SIZE` | 아카이브 파일에서 또는 아카이브 파일로 스트리밍할 때의 버퍼 크기(바이트 단위)           | `4194304`(4MiB) |

예제:

```yaml
job:
  variables:
    FF_USE_PARALLEL_CACHE_TRANSFER: "true"
    CACHE_CONCURRENCY: "8"
    CACHE_CHUNK_SIZE: "16777216"
```

### 병렬 아티팩트 다운로드(직접 다운로드) {#parallel-artifact-downloads-direct-download}

기본적으로 [`direct_download`](https://docs.gitlab.com/ci/jobs/job_artifacts/#download-artifacts-from-a-job)이 객체 저장소에 리디렉션을 반환하면 러너는 단일 HTTP GET 스트림으로 아티팩트를 다운로드합니다.

`FF_USE_PARALLEL_ARTIFACT_TRANSFER` [기능 플래그](feature-flags.md)를 활성화하여 객체 저장소 백엔드에서 `206 Partial Content`과(와) `Content-Range` 합계를 지원할 때 병렬 HTTP 범위 GET을 허용합니다. 청크 크기 및 동시성은 러너에서 고정됩니다(`CACHE_*` 변수 아님). 이 플래그는 `FF_USE_PARALLEL_CACHE_TRANSFER`과 독립적입니다.

예제:

```yaml
job:
  variables:
    FF_USE_PARALLEL_ARTIFACT_TRANSFER: "true"
```

캐시 메커니즘은 서명된 URL을 사용하여 캐시를 업로드하고 다운로드합니다. URL은 GitLab Runner의 자체 인스턴스에서 서명됩니다. 작업의 스크립트(캐시 업로드/다운로드 스크립트 포함)가 로컬 또는 외부 머신에서 실행되는지 여부는 중요하지 않습니다. 예를 들어 `shell` 또는 `docker` 실행기는 GitLab Runner 프로세스가 실행 중인 동일한 머신에서 스크립트를 실행합니다. 동시에 `virtualbox` 또는 `docker+machine`는 별도의 VM에 연결하여 스크립트를 실행합니다. 이 프로세스는 보안상의 이유로 수행됩니다: 캐시 어댑터의 자격 증명 유출 가능성을 최소화합니다.

[S3 캐시 어댑터](#the-runnerscaches3-section)가 IAM 인스턴스 프로필을 사용하도록 구성된 경우 어댑터는 GitLab Runner 머신에 연결된 프로필을 사용합니다. 마찬가지로 [GCS 캐시 어댑터](#the-runnerscachegcs-section)의 경우 `CredentialsFile`를 사용하도록 구성되면:  파일은 GitLab Runner 머신에 있어야 합니다.

이 표는 `config.toml`, CLI 옵션 및 `register`에 대한 환경 변수를 나열합니다. 이 환경 변수를 정의할 때 값은 새 GitLab Runner를 등록한 후 `config.toml`에 저장됩니다.

S3 자격 증명을 `config.toml`에서 생략하고 환경에서 정적 자격 증명을 로드하려는 경우 `AWS_ACCESS_KEY_ID` 및 `AWS_SECRET_ACCESS_KEY`를 정의할 수 있습니다. 자세한 정보는 [AWS SDK 기본 자격 증명 체인 섹션](#aws-sdk-default-credential-chain)을 참조하세요.

| 설정                        | TOML 필드                                        | `register`의 CLI 옵션                  | `register`의 환경 변수 |
|--------------------------------|---------------------------------------------------|--------------------------------------------|-------------------------------------|
| `Type`                         | `[runners.cache] -> Type`                         | `--cache-type`                             | `$CACHE_TYPE`                       |
| `Path`                         | `[runners.cache] -> Path`                         | `--cache-path`                             | `$CACHE_PATH`                       |
| `Shared`                       | `[runners.cache] -> Shared`                       | `--cache-shared`                           | `$CACHE_SHARED`                     |
| `S3.ServerAddress`             | `[runners.cache.s3] -> ServerAddress`             | `--cache-s3-server-address`                | `$CACHE_S3_SERVER_ADDRESS`          |
| `S3.AccessKey`                 | `[runners.cache.s3] -> AccessKey`                 | `--cache-s3-access-key`                    | `$CACHE_S3_ACCESS_KEY`              |
| `S3.SecretKey`                 | `[runners.cache.s3] -> SecretKey`                 | `--cache-s3-secret-key`                    | `$CACHE_S3_SECRET_KEY`              |
| `S3.SessionToken`              | `[runners.cache.s3] -> SessionToken`              | `--cache-s3-session-token`                 | `$CACHE_S3_SESSION_TOKEN`           |
| `S3.BucketName`                | `[runners.cache.s3] -> BucketName`                | `--cache-s3-bucket-name`                   | `$CACHE_S3_BUCKET_NAME`             |
| `S3.BucketLocation`            | `[runners.cache.s3] -> BucketLocation`            | `--cache-s3-bucket-location`               | `$CACHE_S3_BUCKET_LOCATION`         |
| `S3.Insecure`                  | `[runners.cache.s3] -> Insecure`                  | `--cache-s3-insecure`                      | `$CACHE_S3_INSECURE`                |
| `S3.AuthenticationType`        | `[runners.cache.s3] -> AuthenticationType`        | `--cache-s3-authentication_type`           | `$CACHE_S3_AUTHENTICATION_TYPE`     |
| `S3.ServerSideEncryption`      | `[runners.cache.s3] -> ServerSideEncryption`      | `--cache-s3-server-side-encryption`        | `$CACHE_S3_SERVER_SIDE_ENCRYPTION`  |
| `S3.ServerSideEncryptionKeyID` | `[runners.cache.s3] -> ServerSideEncryptionKeyID` | `--cache-s3-server-side-encryption-key-id` | `$CACHE_S3_SERVER_SIDE_ENCRYPTION_KEY_ID` |
| `S3.DualStack`                 | `[runners.cache.s3] -> DualStack`                 | `--cache-s3-dual-stack`                    | `$CACHE_S3_DUAL_STACK`              |
| `S3.Accelerate`                | `[runners.cache.s3] -> Accelerate`                | `--cache-s3-accelerate`                    | `$CACHE_S3_ACCELERATE`              |
| `S3.PathStyle`                 | `[runners.cache.s3] -> PathStyle`                 | `--cache-s3-path-style`                    | `$CACHE_S3_PATH_STYLE`              |
| `S3.RoleARN`                   | `[runners.cache.s3] -> RoleARN`                   | `--cache-s3-role-arn`                      | `$CACHE_S3_ROLE_ARN`                |
| `S3.UploadRoleARN`             | `[runners.cache.s3] -> UploadRoleARN`             | `--cache-s3-upload-role-arn`               | `$CACHE_S3_UPLOAD_ROLE_ARN`         |
| `S3.AssumeRoleMaxConcurrency`  | `[runners.cache.s3] -> AssumeRoleMaxConcurrency`  | `--cache-s3-assume-role-max-concurrency`   | `$CACHE_S3_ASSUME_ROLE_MAX_CONCURRENCY` |
| `GCS.AccessID`                 | `[runners.cache.gcs] -> AccessID`                 | `--cache-gcs-access-id`                    | `$CACHE_GCS_ACCESS_ID`              |
| `GCS.PrivateKey`               | `[runners.cache.gcs] -> PrivateKey`               | `--cache-gcs-private-key`                  | `$CACHE_GCS_PRIVATE_KEY`            |
| `GCS.CredentialsFile`          | `[runners.cache.gcs] -> CredentialsFile`          | `--cache-gcs-credentials-file`             | `$GOOGLE_APPLICATION_CREDENTIALS`   |
| `GCS.BucketName`               | `[runners.cache.gcs] -> BucketName`               | `--cache-gcs-bucket-name`                  | `$CACHE_GCS_BUCKET_NAME`            |
| `Azure.AccountName`            | `[runners.cache.azure] -> AccountName`            | `--cache-azure-account-name`               | `$CACHE_AZURE_ACCOUNT_NAME`         |
| `Azure.AccountKey`             | `[runners.cache.azure] -> AccountKey`             | `--cache-azure-account-key`                | `$CACHE_AZURE_ACCOUNT_KEY`          |
| `Azure.ContainerName`          | `[runners.cache.azure] -> ContainerName`          | `--cache-azure-container-name`             | `$CACHE_AZURE_CONTAINER_NAME`       |
| `Azure.StorageDomain`          | `[runners.cache.azure] -> StorageDomain`          | `--cache-azure-storage-domain`             | `$CACHE_AZURE_STORAGE_DOMAIN`       |

### 캐시 키 처리 {#cache-key-handling}

{{< history >}}

- GitLab Runner 18.4.0에서 [도입](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5751)되었습니다.
- 분산 캐시의 객체 경로는 GitLab Runner 19.0에서 `FF_HASH_CACHE_KEYS`이 활성화될 때 샤드 접두사를 포함하도록 [변경](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6628)되었습니다.

{{< /history >}}

GitLab Runner 18.4.0 이상에서는 `FF_HASH_CACHE_KEYS` [기능 플래그](feature-flags.md)를 사용하여 캐시 키를 해시할 수 있습니다.

`FF_HASH_CACHE_KEYS`이 꺼져 있으면(기본값) GitLab Runner는 캐시 키를 새니타이징합니다. 로컬 캐시 파일과 저장소 버킷의 객체 모두에 대해 경로를 만드는 데 사용합니다. 새니타이징이 캐시 키를 변경하면 GitLab Runner가 이 변경을 로깅합니다. GitLab Runner가 캐시 키를 새니타이징할 수 없으면 이것도 로깅하고 이 특정 캐시를 사용하지 않습니다.

이 기능 플래그를 설정하면 GitLab Runner는 캐시 키(SHA-256)를 해시합니다. 로컬 캐시 아티팩트 및 원격 저장소 버킷의 객체 경로를 만드는 데 사용합니다. GitLab Runner는 캐시 키를 새니타이징하지 않습니다. 특정 캐시 아티팩트를 만든 캐시 키를 이해하는 데 도움이 되도록 GitLab Runner는 메타데이터를 연결합니다:

- 로컬 캐시 아티팩트의 경우 GitLab Runner는 `metadata.json` 파일을 캐시 아티팩트 `cache.zip` 옆에 배치하며 다음 콘텐츠가 있습니다:

  ```json
  {"cachekey": "the human readable cache key"}
  ```

- 분산 캐시의 캐시 아티팩트의 경우 GitLab Runner는 메타데이터를 저장소 객체 blob에 직접 연결하며 키 `cachekey`을 사용합니다. 클라우드 공급자의 메커니즘을 사용하여 쿼리할 수 있습니다. 예를 들어 AWS S3에 대한 [사용자 정의 객체 메타데이터](https://docs.aws.amazon.com/AmazonS3/latest/userguide/UsingMetadata.html#UserMetadata)를 참조하세요.

#### `FF_HASH_CACHE_KEYS`을 사용한 분산 캐시 객체 경로 {#distributed-cache-object-path-with-ff_hash_cache_keys}

GitLab Runner 19.0 이상에서 `FF_HASH_CACHE_KEYS`이 활성화되면 GitLab Runner는 SHA-256 해시의 처음 두 16진수 문자를 분산 캐시 객체 경로에 샤드 접두사로 삽입합니다:

```plaintext
[path/][runner/<token>/]project/<project_id>/<shard>/<hash>/cache.zip
```

예를 들어:

```plaintext
runner/abc123/project/42/d0/d03a852ba491ba611e907b1ef60ad5c4516a05b8f3aae6abb77f42bc60325aed/cache.zip
```

이는 프로젝트별로 256개의 서로 다른 객체 접두사에 캐시 객체를 분산하며 많은 병렬 작업이 높은 요청 속도로 캐시에 액세스할 때 [Amazon S3 503(느린 다운) 응답](https://docs.aws.amazon.com/AmazonS3/latest/userguide/optimizing-performance.html)을 방지합니다.

> [!warning]
> `FF_HASH_CACHE_KEYS`을 사용하는 경우 GitLab Runner 19.0으로 업그레이드는 주요 변경 사항입니다. 이미 `FF_HASH_CACHE_KEYS`이 활성화되어 있고 GitLab Runner 19.0 이상으로 업그레이드하면 샤드 접두사가 분산 저장소의 모든 캐시 아티팩트에 대한 객체 경로를 변경합니다. 이전 경로(`.../<hash>/cache.zip`)에 저장된 기존 객체는 더 이상 접근할 수 없습니다. 업그레이드 후 첫 번째 작업 실행 시 캐시 누락 및 캐시 아티팩트 재빌드를 예상합니다.

#### 캐시 키 처리 동작 요약 {#cache-key-handling-behavior-summary}

`FF_HASH_CACHE_KEYS`을(를) 변경하면 GitLab Runner는 캐시 키를 해싱하는 것이 캐시 아티팩트의 이름과 위치를 변경하므로 기존 캐시 아티팩트를 무시합니다. 이 변경은 양방향으로 적용되며 `FF_HASH_CACHE_KEYS=true`에서 `FF_HASH_CACHE_KEYS=false`로(그 반대도) 적용됩니다.

분산 캐시를 공유하지만 `FF_HASH_CACHE_KEYS`에 대해 다른 설정을 사용하는 여러 러너를 실행하는 경우 캐시 아티팩트를 공유하지 않습니다.

따라서 가장 좋은 방법은 다음과 같습니다:

- 분산 캐시를 공유하는 러너 간에 `FF_HASH_CACHE_KEYS`을 동기화 상태로 유지합니다.

- `FF_HASH_CACHE_KEYS`을 변경한 후 캐시 누락, 캐시 아티팩트 재빌드 및 더 긴 첫 번째 작업 실행을 예상합니다.

- GitLab Runner가 기본 및 대체 캐시 위치를 모두 확인하는 동안 전환 기간 중 추가 네트워크 요청을 예상합니다.

> [!warning]
> `FF_HASH_CACHE_KEYS`을 설정하지만 이전 버전의 도우미 바이너리를 실행하는 경우(예: 도우미 이미지를 이전 버전으로 고정한 경우) 캐시 키 해싱 및 캐시 업로드 또는 다운로드가 여전히 작동합니다. 그러나 GitLab Runner는 캐시 아티팩트의 메타데이터를 유지하지 않습니다.

### `[runners.cache.s3]` 섹션 {#the-runnerscaches3-section}

다음 매개변수는 캐시의 S3 저장소를 정의합니다.

| 매개변수                   | 유형    | 설명 |
|-----------------------------|---------|-------------|
| `ServerAddress`             | 문자열  | S3 호환 서버의 `host:port`. AWS가 아닌 서버를 사용하는 경우 올바른 주소를 확인하려면 저장소 제품 설명서를 참조하세요. DigitalOcean의 경우 주소는 `spacename.region.digitaloceanspaces.com` 형식이어야 합니다. |
| `AccessKey`                 | 문자열  | S3 인스턴스에 대해 지정된 액세스 키입니다. |
| `SecretKey`                 | 문자열  | S3 인스턴스에 대해 지정된 비밀 키입니다. |
| `SessionToken`              | 문자열  | 임시 자격 증명을 사용할 때 S3 인스턴스에 대해 지정된 세션 토큰입니다. |
| `BucketName`                | 문자열  | 캐시가 저장되는 저장소 버킷의 이름입니다. |
| `BucketLocation`            | 문자열  | S3 영역의 이름입니다. |
| `Insecure`                  | 부울 | S3 서비스를 `HTTP`으로 사용할 수 있는 경우 `true`로 설정합니다. 기본값은 `false`입니다. |
| `AuthenticationType`        | 문자열  | `iam` 또는 `access-key`로 설정합니다. `ServerAddress`, `AccessKey` 및 `SecretKey`이 모두 제공된 경우 기본값은 `access-key`입니다. `ServerAddress`, `AccessKey` 또는 `SecretKey`이 누락된 경우 `iam`로 기본값이 설정됩니다. |
| `ServerSideEncryption`      | 문자열  | S3과 함께 사용할 서버 측 암호화 유형입니다. GitLab 15.3 이상에서 사용 가능한 유형은 `S3` 또는 `KMS`입니다. GitLab 17.5 이상에서는 [`DSSE-KMS`](https://docs.aws.amazon.com/AmazonS3/latest/userguide/UsingDSSEncryption.html)이 지원됩니다. |
| `ServerSideEncryptionKeyID` | 문자열  | KMS를 사용할 때 암호화에 사용되는 KMS 키의 별칭, ID 또는 ARN입니다. 별칭을 사용하는 경우 `alias/`을(를) 접두사로 붙입니다. 교차 계정 시나리오에 ARN 형식을 사용합니다. GitLab 15.3 이상에서 사용 가능합니다. |
| `DualStack`                 | 부울 | IPv4 및 IPv6 엔드포인트를 활성화합니다. 기본값은 `true`입니다. AWS S3 Express를 사용 중인 경우 이 설정을 비활성화하세요. `ServerAddress`을 설정한 경우 GitLab이 이 설정을 무시합니다. GitLab 17.5 이상에서 사용 가능합니다. |
| `Accelerate`                | 부울 | AWS S3 Transfer Acceleration을 활성화합니다. `ServerAddress`이 Accelerated 엔드포인트로 구성된 경우 GitLab이 자동으로 이것을 `true`으로 설정합니다. GitLab 17.5 이상에서 사용 가능합니다. |
| `PathStyle`                 | 부울 | 경로 스타일 액세스를 활성화합니다. 기본적으로 GitLab이 `ServerAddress` 값을 기반으로 이 설정을 자동으로 감지합니다. GitLab 17.5 이상에서 사용 가능합니다. |
| `UploadRoleARN`             | 문자열  | 더 이상 지원되지 않습니다. 대신 `RoleARN`을 사용하세요. `AssumeRole`로 사용할 수 있으며 시간 제한이 있는 `PutObject` S3 요청을 생성하는 AWS 역할 ARN을 지정합니다. S3 다중 부분 업로드를 활성화합니다. GitLab 17.5 이상에서 사용 가능합니다. |
| `RoleARN`                   | 문자열  | `AssumeRole`로 사용할 수 있으며 시간 제한이 있는 `GetObject` 및 `PutObject` S3 요청을 생성하는 AWS 역할 ARN을 지정합니다. S3 다중 부분 전송을 활성화합니다. GitLab 17.8 이상에서 사용 가능합니다. |
| `AssumeRoleMaxConcurrency`  | 정수 | `RoleARN`이 설정된 경우 AWS STS에 대한 최대 동시 `AssumeRole` 요청 수입니다. 기본값은 `5`입니다. `-1`로 설정하여 제한을 제거하세요. |

예제:

```toml
[runners.cache]
  Type = "s3"
  Path = "path/to/prefix"
  Shared = false
  [runners.cache.s3]
    ServerAddress = "s3.amazonaws.com"
    AccessKey = "AWS_S3_ACCESS_KEY"
    SecretKey = "AWS_S3_SECRET_KEY"
    BucketName = "runners-cache"
    BucketLocation = "eu-west-1"
    Insecure = false
    ServerSideEncryption = "KMS"
    ServerSideEncryptionKeyID = "alias/my-key"
```

## 인증 {#authentication}

GitLab Runner는 구성을 기반으로 S3에 대해 다른 인증 방법을 사용합니다.

### 정적 자격증명 {#static-credentials}

러너가 다음의 경우 정적 액세스 키 인증을 사용합니다:

- `ServerAddress`, `AccessKey`, `SecretKey` 매개변수가 지정되었지만 `AuthenticationType`이 제공되지 않은 경우입니다.
- `AuthenticationType = "access-key"`이 명시적으로 설정된 경우입니다.

### AWS SDK 기본 자격증명 체인 {#aws-sdk-default-credential-chain}

다음의 경우 러너가 [AWS SDK 기본 자격증명 체인](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html#specifying-credentials)을 사용합니다:

- `ServerAddress`, `AccessKey`, `SecretKey` 중 하나라도 생략되었으며 `AuthenticationType`이 제공되지 않은 경우입니다.
- `AuthenticationType = "iam"`이 명시적으로 설정된 경우입니다.

자격증명 체인은 다음 순서로 인증을 시도합니다:

1. 환경 변수 (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`)
1. 공유 자격증명 파일 (`~/.aws/credentials`)
1. IAM 인스턴스 프로필 (EC2 인스턴스의 경우)
1. SDK에서 지원하는 기타 AWS 자격증명 소스

`RoleARN`이 지정되지 않은 경우 기본 자격증명 체인은 러너 관리자에 의해 실행되며, 이는 빌드가 실행되는 머신과 반드시 같은 머신에 있지 않습니다. 예를 들어 [자동 확장](autoscale.md) 구성에서 작업은 다른 머신에서 실행됩니다. 마찬가지로 Kubernetes 실행기를 사용할 경우 빌드 포드도 러너 관리자와 다른 노드에서 실행될 수 있습니다. 이 동작으로 인해 러너 관리자에만 버킷 수준 액세스를 부여할 수 있습니다.

`RoleARN`이 지정된 경우 자격증명은 헬퍼 이미지의 실행 컨텍스트 내에서 해결됩니다. 자세한 내용은 [RoleARN](#enable-multipart-transfers-with-rolearn)을 참조하세요.

GitLab Runner를 설치하기 위해 Helm 차트를 사용하고 `rbac.create`이 `values.yaml` 파일에서 `true`로 설정된 경우 서비스 계정이 생성됩니다. 서비스 계정의 주석은 `rbac.serviceAccountAnnotations` 섹션에서 검색됩니다.

Amazon EKS의 러너의 경우 서비스 계정에 할당할 IAM 역할을 지정할 수 있습니다. 필요한 특정 주석은 `eks.amazonaws.com/role-arn: arn:aws:iam::<ACCOUNT_ID>:role/<IAM_ROLE_NAME>`입니다.

이 역할의 IAM 정책은 지정된 버킷에 대해 다음 작업을 수행할 수 있는 권한이 있어야 합니다:

- `s3:PutObject`
- `s3:GetObjectVersion`
- `s3:GetObject`
- `s3:DeleteObject`
- `s3:ListBucket`

`ServerSideEncryption`을 `KMS` 유형으로 사용하는 경우 이 역할은 지정된 AWS KMS 키에 대해 다음 작업을 수행할 수 있는 권한도 있어야 합니다:

- `kms:Encrypt`
- `kms:Decrypt`
- `kms:ReEncrypt*`
- `kms:GenerateDataKey*`
- `kms:DescribeKey`

`ServerSideEncryption`의 `SSE-C` 유형은 지원되지 않습니다. `SSE-C`은 다운로드 요청을 위해 미리 서명된 URL 외에도 사용자가 제공한 키가 포함된 헤더를 제공해야 합니다. 이는 키 자료를 작업에 전달하는 것을 의미하며, 여기서 키를 안전하게 유지할 수 없습니다. 이로 인해 암호 해독 키가 유출될 가능성이 있습니다. 이 문제에 대한 논의는 [이 병합 요청](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/3295)에 있습니다.

> [!note]
> AWS S3 캐시에 업로드할 수 있는 단일 파일의 최대 크기는 5GB입니다. 이 동작에 대한 잠재적 해결 방법에 대한 논의는 [이 이슈](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/26921)에 있습니다.

#### 러너 캐시를 위해 S3 버킷에서 KMS 키 암호화 사용 {#use-kms-key-encryption-in-s3-bucket-for-runner-cache}

`GenerateDataKey` API는 KMS 대칭 키를 사용하여 클라이언트 측 암호화를 위한 데이터 키를 생성합니다 (<https://docs.aws.amazon.com/kms/latest/APIReference/API_GenerateDataKey.html>). KMS 키 구성은 다음과 같아야 합니다:

| 속성 | 설명 |
|-----------|-------------|
| 키 유형  | 대칭   |
| 출처    | `AWS_KMS`   |
| 키 사양  | `SYMMETRIC_DEFAULT` |
| 키 사용 | 암호화 및 암호 해독 |

`rbac.serviceAccountName`에 정의된 ServiceAccount에 할당된 역할의 IAM 정책은 KMS 키에 대해 다음 작업을 수행할 수 있는 권한이 있어야 합니다:

- `kms:GetPublicKey`
- `kms:Decrypt`
- `kms:Encrypt`
- `kms:DescribeKey`
- `kms:GenerateDataKey`

#### `RoleARN`로 다중 부분 전송 활성화 {#enable-multipart-transfers-with-rolearn}

캐시에 대한 액세스를 제한하기 위해 러너 관리자는 작업이 캐시에서 다운로드하고 캐시에 업로드하기 위한 시간 제한이 있는 [미리 서명된 URL](https://docs.aws.amazon.com/AmazonS3/latest/userguide/using-presigned-url.html)을 생성합니다. 그러나 AWS S3는 [단일 PUT 요청을 5GB로 제한](https://docs.aws.amazon.com/AmazonS3/latest/userguide/upload-objects.html)합니다. 5GB보다 큰 파일의 경우 다중 부분 업로드 API를 사용해야 합니다.

다중 부분 전송은 AWS S3에서만 지원되며 다른 S3 공급자에서는 지원되지 않습니다. 러너 관리자는 다양한 프로젝트에 대한 작업을 처리하기 때문에 버킷 전체 권한이 있는 S3 자격증명을 전달할 수 없습니다. 대신 러너 관리자는 시간 제한이 있는 미리 서명된 URL과 범위가 좁은 자격증명을 사용하여 특정 개체에 대한 액세스를 제한합니다.

AWS에서 S3 다중 부분 전송을 사용하려면 `RoleARN`에서 `arn:aws:iam:::<ACCOUNT ID>:<YOUR ROLE NAME>` 형식으로 IAM 역할을 지정하세요. 이 역할은 버킷의 특정 블로브에 쓰기 위해 범위가 좁은 시간 제한이 있는 AWS 자격증명을 생성합니다. 원본 S3 자격증명이 지정된 `RoleARN`에 대해 `AssumeRole`에 액세스할 수 있도록 하세요.

`RoleARN`에 지정된 IAM 역할은 다음 권한이 있어야 합니다:

- `BucketName`에 지정된 버킷에 대한 `s3:GetObject` 액세스입니다.
- `BucketName`에 지정된 버킷에 대한 `s3:PutObject` 액세스입니다.
- `BucketName`에 지정된 버킷에 대한 `s3:ListBucket` 액세스입니다.
- KMS 또는 DSSE-KMS를 사용한 서버 측 암호화가 활성화된 경우 `kms:Decrypt` 및 `kms:GenerateDataKey`입니다.

예를 들어 `my-instance-role`이라는 IAM 역할이 ARN `arn:aws:iam::1234567890123:role/my-instance-role`을 가진 EC2 인스턴스에 연결되어 있다고 가정합니다.

`BucketName`에 대해서만 `s3:PutObject` 권한이 있는 새로운 역할 `arn:aws:iam::1234567890123:role/my-upload-role`을 생성할 수 있습니다. `my-instance-role`의 AWS 설정에서 `Trust relationships`은 다음과 같을 수 있습니다:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": {
                "AWS": "arn:aws:iam::1234567890123:role/my-upload-role"
            },
            "Action": "sts:AssumeRole"
        }
    ]
}
```

`my-instance-role`을 `RoleARN`로 재사용하고 새로운 역할을 만드는 것을 피할 수 있습니다. `my-instance-role`이 `AssumeRole` 권한을 가지고 있는지 확인하세요. 예를 들어 EC2 인스턴스와 관련된 IAM 프로필은 다음 `Trust relationships`을 가질 수 있습니다:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": {
                "Service": "ec2.amazonaws.com",
                "AWS": "arn:aws:iam::1234567890123:role/my-instance-role"
            },
            "Action": "sts:AssumeRole"
        }
    ]
}
```

AWS 명령줄 인터페이스를 사용하여 인스턴스가 `AssumeRole` 권한을 가지고 있는지 확인할 수 있습니다. 예를 들어:

```shell
aws sts assume-role --role-arn arn:aws:iam::1234567890123:role/my-upload-role --role-session-name gitlab-runner-test1
```

##### `RoleARN`로 업로드가 작동하는 방식 {#how-uploads-work-with-rolearn}

`RoleARN`이 있으면 러너가 캐시에 업로드할 때마다:

1. 러너 관리자는 `AuthenticationType`, `AccessKey`, `SecretKey`을 통해 지정된 원본 S3 자격증명을 검색합니다.
1. S3 자격증명으로 러너 관리자는 Amazon Security Token Service(STS)에 `RoleARN`과 함께 [`AssumeRole`](https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRole.html)를 요청합니다. 정책 요청은 다음과 같습니다:

   ```json
   {
       "Version": "2012-10-17",
       "Statement": [
           {
               "Effect": "Allow",
               "Action": ["s3:PutObject"],
               "Resource": "arn:aws:s3:::<YOUR-BUCKET-NAME>/<CACHE-FILENAME>"
           }
       ]
   }
   ```

1. 요청이 성공하면 러너 관리자는 제한된 세션으로 임시 AWS 자격증명을 얻습니다.
1. 러너 관리자는 이러한 자격증명과 URL을 `s3://<bucket name>/<filename>` 형식으로 캐시 아카이버에 전달하며, 이는 파일을 업로드합니다.

##### AssumeRole Prometheus 메트릭 {#assumerole-prometheus-metrics}

`RoleARN`이 설정된 경우 GitLab Runner는 STS 요청 동작을 모니터링하기 위해 다음 Prometheus 메트릭을 노출합니다:

| 메트릭 | 유형 | 설명 |
|--------|------|-------------|
| `gitlab_runner_cache_s3_assume_role_requests_in_flight` | 계기 | 진행 중인 AWS STS에 대한 `AssumeRole` 요청의 수입니다. |
| `gitlab_runner_cache_s3_assume_role_wait_seconds` | 히스토그램 | `AssumeRole` 요청을 발급하기 전에 동시성 슬롯을 얻기 위해 대기합니다. |
| `gitlab_runner_cache_s3_assume_role_duration_seconds` | 히스토그램 | AWS STS에 대한 `AssumeRole` API 호출의 기간입니다. |
| `gitlab_runner_cache_s3_assume_role_cache_hits_total` | 카운터 | `AssumeRole` 자격증명 캐시 히트 수 (STS 호출 방지)입니다. |
| `gitlab_runner_cache_s3_assume_role_cache_misses_total` | 카운터 | `AssumeRole` 자격증명 캐시 미스 수 (STS 호출 수행)입니다. |
| `gitlab_runner_cache_s3_assume_role_cached_credentials` | 계기 | 메모리 내 LRU 캐시에 보관된 `AssumeRole` 자격증명 수입니다. |
| `gitlab_runner_cache_s3_assume_role_failures_total` | 카운터 | 실패한 `AssumeRole` 요청의 수입니다. |

#### Kubernetes ServiceAccount 리소스에 대해 IAM 역할 활성화 {#enable-iam-roles-for-kubernetes-serviceaccount-resources}

서비스 계정에 IAM 역할을 사용하려면 IAM OIDC 공급자가 [클러스터에 대해 존재해야 합니다](https://docs.aws.amazon.com/eks/latest/userguide/enable-iam-roles-for-service-accounts.html). IAM OIDC 공급자가 클러스터와 연결된 후 러너의 서비스 계정과 연결할 IAM 역할을 생성할 수 있습니다.

1. **Create Role** 창에서 **Select type of trusted entity** 아래에서 **Web Identity**를 선택하세요.
1. 역할의 **Trusted Relationships tab**에서:

   - **Trusted entities** 섹션은 다음 형식이어야 합니다: `arn:aws:iam::<ACCOUNT_ID>:oidc-provider/oidc.eks.<AWS_REGION>.amazonaws.com/id/<OIDC_ID>`. **OIDC ID**는 EKS 클러스터의 **구성** 탭에서 찾을 수 있습니다.

   - **Condition** 섹션은 `rbac.serviceAccountName`에 정의된 GitLab Runner 서비스 계정 또는 `rbac.create`이 `true`로 설정된 경우 생성된 기본 서비스 계정을 포함해야 합니다:

     | 조건      | 키                                                    | 값 |
     |----------------|--------------------------------------------------------|-------|
     | `StringEquals` | `oidc.eks.<AWS_REGION>.amazonaws.com/id/<OIDC_ID>:sub` | `system:serviceaccount:<GITLAB_RUNNER_NAMESPACE>:<GITLAB_RUNNER_SERVICE_ACCOUNT>` |

#### S3 Express One Zone 버킷 사용 {#use-s3-express-one-zone-buckets}

{{< history >}}

- GitLab Runner 17.5.0에서 도입되었습니다.

{{< /history >}}

> [!note]
> [S3 Express One Zone 디렉터리 버킷은 `RoleARN`과 작동하지 않습니다](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/38484#note_2313111840) 러너 관리자가 특정 개체에 대한 액세스를 제한할 수 없기 때문입니다.

1. [Amazon 튜토리얼](https://docs.aws.amazon.com/AmazonS3/latest/userguide/s3-express-getting-started.html)을 따라 S3 Express One Zone 버킷을 설정하세요.
1. `config.toml`을 `BucketName` 및 `BucketLocation`과 함께 구성하세요.
1. `DualStack`을 `false`로 설정하세요. S3 Express는 이중 스택 엔드포인트를 지원하지 않습니다.

`config.toml` 예시:

```toml
[runners.cache]
  Type = "s3"
  [runners.cache.s3]
    BucketName = "example-express--usw2-az1--x-s3"
    BucketLocation = "us-west-2"
    DualStack = false
```

### `[runners.cache.gcs]` 섹션 {#the-runnerscachegcs-section}

다음 매개변수는 Google Cloud Storage에 대한 기본 지원을 정의합니다. 이러한 값에 대한 자세한 내용은 [Google Cloud Storage(GCS) 인증 설명서](https://docs.cloud.google.com/storage/docs/authentication#service_accounts)를 참조하세요.

| 매개변수         | 유형   | 설명 |
|-------------------|--------|-------------|
| `CredentialsFile` | 문자열 | Google JSON 키 파일의 경로입니다. `service_account` 유형만 지원됩니다. 구성된 경우 이 값은 `config.toml`에서 직접 구성된 `AccessID` 및 `PrivateKey`보다 우선합니다. |
| `AccessID`        | 문자열 | 스토리지에 액세스하는 데 사용되는 GCP 서비스 계정의 ID입니다. |
| `PrivateKey`      | 문자열 | GCS 요청에 서명하는 데 사용되는 개인 키입니다. |
| `BucketName`      | 문자열 | 캐시가 저장되는 저장소 버킷의 이름입니다. |
| `UniverseDomain`  | 문자열 | GCS 요청을 위한 Universe 도메인 (선택 사항)입니다. 공개 Google Cloud의 경우 `googleapis.com`을 사용하세요. Google Cloud Dedicated 또는 기타 사용자 정의 Universe 도메인의 경우 적절한 도메인을 지정하세요 (예: `custom.universe.com`). 도메인을 지정하지 않으면 기본값은 `googleapis.com`입니다. |

예시:

**`config.toml` 파일에서 직접 구성된 자격증명**:

```toml
[runners.cache]
  Type = "gcs"
  Path = "path/to/prefix"
  Shared = false
  [runners.cache.gcs]
    AccessID = "cache-access-account@test-project-123456.iam.gserviceaccount.com"
    PrivateKey = "-----BEGIN PRIVATE KEY-----\nXXXXXX\n-----END PRIVATE KEY-----\n"
    BucketName = "runners-cache"
    UniverseDomain = "googleapis.com"  # Optional
```

**Credentials in JSON file downloaded from GCP**:

```toml
[runners.cache]
  Type = "gcs"
  Path = "path/to/prefix"
  Shared = false
  [runners.cache.gcs]
    CredentialsFile = "/etc/gitlab-runner/service-account.json"
    BucketName = "runners-cache"
    UniverseDomain = "googleapis.com"  # Optional
```

**Application Default Credentials (ADC) from the metadata server in GCP**:

Google Cloud ADC에서 GitLab Runner를 사용할 때 일반적으로 기본 서비스 계정을 사용합니다. 그러면 인스턴스에 자격증명을 제공할 필요가 없습니다:

```toml
[runners.cache]
  Type = "gcs"
  Path = "path/to/prefix"
  Shared = false
  [runners.cache.gcs]
    BucketName = "runners-cache"
    UniverseDomain = "googleapis.com"  # Optional
```

ADC를 사용하는 경우 사용하는 서비스 계정이 `iam.serviceAccounts.signBlob` 권한을 가지고 있는지 확인하세요. 일반적으로 이는 서비스 계정에 [Service Account Token Creator 역할](https://docs.cloud.google.com/iam/docs/service-account-permissions#token-creator-role)을 부여함으로써 수행됩니다.

#### GKE를 위한 Workload Identity Federation {#workload-identity-federation-for-gke}

GKE용 Workload Identity Federation은 ADC (Application Default Credentials)를 지원합니다. Workload Identity를 작동시키는 데 문제가 있으면:

- 러너 포드 로그 (빌드 로그가 아님)에서 `ERROR: generating signed URL` 메시지를 확인하세요. 이 오류는 다음과 같은 권한 문제를 나타낼 수 있습니다:

  ```plaintext
  IAM returned 403 Forbidden: Permission 'iam.serviceAccounts.getAccessToken' denied on resource (or it may not exist).
  ```

- 러너 포드 내에서 다음 `curl` 명령을 시도하세요:

  ```shell
  curl -H "Metadata-Flavor: Google" http://169.254.169.254/computeMetadata/v1/instance/service-accounts/default/email
  ```

   이 명령은 올바른 Kubernetes 서비스 계정을 반환해야 합니다. 다음으로 액세스 토큰을 얻으려고 시도하세요:

  ```shell
  curl -H "Metadata-Flavor: Google" http://169.254.169.254/computeMetadata/v1/instance/service-accounts/default/token?scopes=https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fcloud-platform
  ```

   명령이 성공하면 결과는 액세스 토큰이 있는 JSON 페이로드를 반환합니다. 실패하면 서비스 계정 권한을 확인하세요.

### `[runners.cache.azure]` 섹션 {#the-runnerscacheazure-section}

다음 매개변수는 Azure Blob Storage에 대한 기본 지원을 정의합니다. 자세히 알아보려면 [Azure Blob Storage 설명서](https://learn.microsoft.com/en-us/azure/storage/blobs/storage-blobs-introduction)를 참조하세요. S3 및 GCS는 개체 컬렉션의 경우 `bucket` 단어를 사용하는 반면 Azure는 Blob 컬렉션을 나타내기 위해 `container` 단어를 사용합니다.

| 매개변수       | 유형   | 설명 |
|-----------------|--------|-------------|
| `AccountName`   | 문자열 | 스토리지에 액세스하는 데 사용되는 Azure Blob Storage 계정의 이름입니다. |
| `AccountKey`    | 문자열 | 컨테이너에 액세스하는 데 사용되는 저장소 계정 액세스 키입니다. 구성에서 `AccountKey`을 생략하려면 [Azure Workload 또는 관리형 ID](#azure-workload-and-managed-identities)를 사용하세요. |
| `ContainerName` | 문자열 | 캐시 데이터를 저장할 [스토리지 컨테이너](https://learn.microsoft.com/en-us/azure/storage/blobs/storage-blobs-introduction#containers)의 이름입니다. |
| `StorageDomain` | 문자열 | [Azure 저장소 엔드포인트를 서비스하는 데 사용되는](https://learn.microsoft.com/en-us/azure/china/resources-developer-guide#check-endpoints-in-azure) 도메인 이름 (선택 사항)입니다. 기본값은 `blob.core.windows.net`입니다. |

예제:

```toml
[runners.cache]
  Type = "azure"
  Path = "path/to/prefix"
  Shared = false
  [runners.cache.azure]
    AccountName = "<AZURE STORAGE ACCOUNT NAME>"
    AccountKey = "<AZURE STORAGE ACCOUNT KEY>"
    ContainerName = "runners-cache"
    StorageDomain = "blob.core.windows.net"
```

#### Azure Workload 및 관리형 ID {#azure-workload-and-managed-identities}

{{< history >}}

- GitLab Runner v17.5.0에 [도입되었습니다](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27303).

{{< /history >}}

Azure Workload 또는 관리형 ID를 사용하려면 구성에서 `AccountKey`을 생략하세요. `AccountKey`이 비어 있으면 러너는 다음을 시도합니다:

1. [`DefaultAzureCredential`](https://github.com/Azure/azure-sdk-for-go/blob/main/sdk/azidentity/README.md#defaultazurecredential)를 사용하여 임시 자격증명을 얻으세요.
1. [사용자 위임 키](https://learn.microsoft.com/en-us/rest/api/storageservices/get-user-delegation-key)를 얻으세요.
1. 해당 키로 SAS 토큰을 생성하여 저장소 계정 Blob에 액세스합니다.

인스턴스가 `Storage Blob Data Contributor` 역할을 할당받았는지 확인하세요. 인스턴스가 위의 작업을 수행할 액세스 권한이 없으면 GitLab Runner는 `AuthorizationPermissionMismatch` 오류를 보고합니다.

Azure Workload 식별을 사용하려면 ID와 관련된 `service_account`과 `runner.kubernetes` 섹션의 포드 레이블 `azure.workload.identity/use`을 추가하세요. 예를 들어 `service_account`이 `gitlab-runner`인 경우:

```toml
  [runners.kubernetes]
    service_account = "gitlab-runner"
    [runners.kubernetes.pod_labels]
      "azure.workload.identity/use" = "true"
```

`service_account`이 `azure.workload.identity/client-id` 주석을 포함하도록 하세요:

```yaml
serviceAccount:
  annotations:
    azure.workload.identity/client-id: <YOUR CLIENT ID HERE>
```

GitLab 17.7 이상에서는 이 구성으로 Workload Identity를 설정할 수 있습니다.

그러나 GitLab Runner 17.5 및 17.6의 경우 다음으로 러너 관리자를 구성해야 합니다:

- `azure.workload.identity/use` 포드 레이블
- Workload Identity와 함께 사용할 서비스 계정

예를 들어 GitLab Runner Helm 차트 사용:

```yaml
serviceAccount:
  name: "gitlab-runner"
podLabels:
  azure.workload.identity/use: "true"
```

자격증명은 다른 출처에서 검색되기 때문에 레이블이 필요합니다. 캐시 다운로드의 경우 자격증명은 러너 관리자에서 검색됩니다. 캐시 업로드의 경우 자격증명은 [헬퍼 이미지](#helper-image)를 실행하는 포드에서 검색됩니다.

자세한 내용은 [이슈 38330](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/38330)을 참조하세요.

## `[runners.kubernetes]` 섹션 {#the-runnerskubernetes-section}

다음 표는 Kubernetes 실행기에 사용 가능한 구성 매개변수를 나열합니다. 더 많은 매개변수는 [Kubernetes 실행기 설명서](../executors/kubernetes/_index.md)를 참조하세요.

| 매개변수                    | 유형    | 설명 |
|------------------------------|---------|-------------|
| `host`                       | 문자열  | 선택적입니다. Kubernetes 호스트 URL입니다. 지정하지 않으면 러너가 자동 검색을 시도합니다. |
| `cert_file`                  | 문자열  | 선택적입니다. Kubernetes 인증 인증서입니다. |
| `key_file`                   | 문자열  | 선택적입니다. Kubernetes 인증 개인 키입니다. |
| `ca_file`                    | 문자열  | 선택적입니다. Kubernetes 인증 ca 인증서입니다. |
| `image`                      | 문자열  | 지정된 작업이 없을 때 사용할 기본 컨테이너 이미지입니다. |
| `allowed_images`             | 배열   | `.gitlab-ci.yml`에서 허용되는 컨테이너 이미지의 와일드카드 목록입니다. 없으면 모든 이미지가 허용됩니다 (`["*/*:*"]`과 동일). [Docker](../executors/docker.md#restrict-docker-images-and-services) 또는 [Kubernetes](../executors/kubernetes/_index.md#restrict-docker-images-and-services) 실행기와 함께 사용합니다. |
| `allowed_services`           | 배열   | `.gitlab-ci.yml`에서 허용되는 서비스의 와일드카드 목록입니다. 없으면 모든 이미지가 허용됩니다 (`["*/*:*"]`과 동일). [Docker](../executors/docker.md#restrict-docker-images-and-services) 또는 [Kubernetes](../executors/kubernetes/_index.md#restrict-docker-images-and-services) 실행기와 함께 사용합니다. |
| `namespace`                  | 문자열  | Kubernetes 작업을 실행할 네임스페이스입니다. |
| `privileged`                 | 부울 | 권한 있는 플래그가 활성화된 상태에서 모든 컨테이너를 실행합니다. |
| `allow_privilege_escalation` | 부울 | 선택적입니다. `allowPrivilegeEscalation` 플래그가 활성화된 상태에서 모든 컨테이너를 실행합니다. |
| `node_selector`              | 테이블   | `string=string`의 `key=value` 쌍으로 된 `table`입니다. 모든 `key=value` 쌍과 일치하는 Kubernetes 노드에 대한 포드 생성을 제한합니다. |
| `image_pull_secrets`         | 배열   | 개인 레지스트리에서 컨테이너 이미지를 풀하기 위해 인증하는 데 사용되는 Kubernetes `docker-registry` 시크릿 이름이 포함된 항목의 배열입니다. |
| `logs_base_dir`              | 문자열  | 빌드 로그를 저장하기 위해 생성된 경로 앞에 추가할 기본 디렉터리입니다. GitLab Runner 17.2에 [도입되었습니다](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/37760). |
| `scripts_base_dir`           | 문자열  | 빌드 스크립트를 저장하기 위해 생성된 경로 앞에 추가할 기본 디렉터리입니다. GitLab Runner 17.2에 [도입되었습니다](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/37760). |
| `service_account`            | 문자열  | 작업/실행기 포드가 Kubernetes API와 통신하는 데 사용하는 기본 서비스 계정입니다. |

예제:

```toml
[runners.kubernetes]
  host = "https://45.67.34.123:4892"
  cert_file = "/etc/ssl/kubernetes/api.crt"
  key_file = "/etc/ssl/kubernetes/api.key"
  ca_file = "/etc/ssl/kubernetes/ca.crt"
  image = "golang:1.8"
  privileged = true
  allow_privilege_escalation = true
  image_pull_secrets = ["docker-registry-credentials", "optional-additional-credentials"]
  allowed_images = ["ruby:*", "python:*", "php:*"]
  allowed_services = ["postgres:9.4", "postgres:latest"]
  logs_base_dir = "/tmp"
  scripts_base_dir = "/tmp"
  [runners.kubernetes.node_selector]
    gitlab = "true"
```

## 헬퍼 이미지 {#helper-image}

`docker`, `docker+machine` 또는 `kubernetes` 실행기를 사용할 때 GitLab Runner는 Git, 아티팩트 및 캐시 작업을 처리하기 위해 특정 컨테이너를 사용합니다. 이 컨테이너는 `helper image`이라는 이미지에서 생성됩니다.

헬퍼 이미지는 amd64, arm, arm64, s390x, ppc64le 및 riscv64 아키텍처에 사용 가능합니다. 이 파일에는 `gitlab-runner-helper` 바이너리가 포함되어 있으며, 이는 GitLab Runner 바이너리의 특수 컴파일입니다. 사용 가능한 명령의 하위 집합, Git, Git LFS 및 SSL 인증서 저장소만 포함합니다.

헬퍼 이미지는 여러 플레이버를 가지고 있습니다: `alpine`, `alpine3.21`, `alpine-latest`, `ubi-fips` 및 `ubuntu`. `alpine` 이미지는 작은 크기로 인해 기본값입니다. `helper_image_flavor = "ubuntu"`을 사용하면 헬퍼 이미지의 `ubuntu` 플레이버가 선택됩니다.

GitLab Runner 16.1부터 17.1까지 `alpine` 플레이버는 `alpine3.18`의 별칭입니다. GitLab Runner 17.2부터 17.6까지 `alpine3.19`의 별칭입니다. GitLab Runner 17.7 이상에서는 `alpine3.21`의 별칭입니다. GitLab Runner 18.4 이상에서는 `alpine-latest`의 별칭입니다.

`alpine-latest` 플레이버는 `alpine:latest`을 기본 이미지로 사용하며 새로운 업스트림 버전이 출시될 때 자연스럽게 버전이 증가합니다.

GitLab Runner가 `DEB` 또는 `RPM` 패키지에서 설치될 때 지원되는 아키텍처의 이미지가 호스트에 설치됩니다. Docker Engine이 지정된 이미지 버전을 찾을 수 없으면 러너가 작업을 실행하기 전에 자동으로 다운로드합니다. `docker` 및 `docker+machine` 실행기 모두 이렇게 작동합니다.

`alpine` 플레이버의 경우 기본 `alpine` 플레이버 이미지만 패키지에 포함됩니다. 다른 모든 플레이버는 레지스트리에서 다운로드됩니다.

`kubernetes` 실행기 및 GitLab Runner의 수동 설치는 다르게 작동합니다.

- 수동 설치의 경우 `gitlab-runner-helper` 바이너리가 포함되지 않습니다.
- `kubernetes` 실행기의 경우 Kubernetes API는 `gitlab-runner-helper` 이미지를 로컬 아카이브에서 로드할 수 없습니다.

두 경우 모두 GitLab Runner가 [헬퍼 이미지를 다운로드합니다](#helper-image-registry). GitLab Runner 개정 및 아키텍처는 다운로드할 태그를 정의합니다.

### Arm의 Kubernetes에 대한 헬퍼 이미지 구성 {#helper-image-configuration-for-kubernetes-on-arm}

기본적으로 올바른 [아키텍처용 헬퍼 이미지](../executors/kubernetes/_index.md#operating-system-architecture-and-windows-kernel-version)가 선택됩니다. `helper_image` 경로를 사용자 지정해야 하는 경우 `arm64` Kubernetes 클러스터에서 `arm64` 도우미 이미지를 사용하려면 [구성 파일](../executors/kubernetes/_index.md#configuration-settings)에서 다음 값을 설정합니다:

```toml
[runners.kubernetes]
  helper_image = "my.registry.local/gitlab/gitlab-runner-helper:arm64-v${CI_RUNNER_VERSION}"
```

### Alpine Linux의 이전 버전을 사용하는 러너 이미지 {#runner-images-that-use-an-old-version-of-alpine-linux}

{{< history >}}

- GitLab Runner 14.5에 [도입되었습니다](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/3122).

{{< /history >}}

이미지는 여러 버전의 Alpine Linux로 빌드됩니다. Alpine의 최신 버전을 사용할 수 있지만 동시에 이전 버전도 사용할 수 있습니다.

헬퍼 이미지의 경우 `helper_image_flavor`을 변경하거나 [헬퍼 이미지](#helper-image) 섹션을 읽으세요.

GitLab Runner 이미지의 경우 동일한 논리를 따르십시오. 여기서 `alpine`, `alpine3.19`, `alpine3.21` 또는 `alpine-latest`은 버전 앞의 이미지에서 접두사로 사용됩니다:

```shell
docker pull gitlab/gitlab-runner:alpine3.19-v16.1.0
```

### Alpine `pwsh` 이미지 {#alpine-pwsh-images}

GitLab Runner 16.1 이상의 모든 `alpine` 헬퍼 이미지는 `pwsh` 변형을 가지고 있습니다. 유일한 예외는 `alpine-latest`입니다. GitLab Runner 헬퍼 이미지가 기반하는 [`powershell` Docker 이미지](https://learn.microsoft.com/en-us/powershell/scripting/install/powershell-in-docker?view=powershell-7.4)는 `alpine:latest`을 지원하지 않습니다.

예제:

```shell
docker pull registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:alpine3.21-x86_64-v17.7.0-pwsh
```

### 헬퍼 이미지 레지스트리 {#helper-image-registry}

GitLab 15.0 이전에는 Docker Hub의 이미지를 사용하도록 헬퍼 이미지를 구성합니다.

GitLab 15.1 이상에서는 헬퍼 이미지가 GitLab.com의 GitLab 컨테이너 레지스트리에서 `registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:x86_64-v${CI_RUNNER_VERSION}`로 당겨집니다. GitLab 자체 관리 인스턴스도 기본적으로 GitLab.com의 GitLab Container Registry에서 헬퍼 이미지를 가져옵니다. GitLab.com의 GitLab Container Registry 상태를 확인하려면 [GitLab 시스템 상태](https://status.gitlab.com/)를 참조하세요.

### 헬퍼 이미지 재정의 {#override-the-helper-image}

경우에 따라 다음과 같은 이유로 헬퍼 이미지를 재정의해야 할 수 있습니다:

1. **Speed up jobs execution**:  인터넷 연결이 느린 환경에서는 동일한 이미지를 여러 번 다운로드하면 작업 실행 시간이 증가할 수 있습니다. `registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:XYZ`의 정확한 복사본이 저장되는 로컬 레지스트리에서 헬퍼 이미지를 다운로드하면 속도가 향상될 수 있습니다.
1. **Security concerns**:  이전에 확인되지 않은 외부 종속성을 다운로드하지 않을 수 있습니다. 검토되어 로컬 레지스트리에 저장된 종속성만 사용하는 비즈니스 규칙이 있을 수 있습니다.
1. **Build environments without internet access**:  [오프라인 환경에 설치된 Kubernetes 클러스터](../install/operator.md#install-gitlab-runner-operator-on-kubernetes-clusters-in-offline-environments)가 있으면 로컬 이미지 레지스트리 또는 패키지 저장소를 사용하여 CI/CD 작업에 사용되는 이미지를 가져올 수 있습니다.
1. **Additional software**:  `git+ssh` 대신 `openssh`을 사용하여 서브모듈에 액세스할 수 있도록 헬퍼 이미지에 추가 소프트웨어를 설치할 수 있습니다 (`git+http` 사용).

이러한 경우 `helper_image` 구성 필드를 사용하여 사용자 정의 이미지를 구성할 수 있으며, 이는 `docker`, `docker+machine` 및 `kubernetes` 실행기에서 사용 가능합니다:

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    helper_image = "my.registry.local/gitlab/gitlab-runner-helper:tag"
```

헬퍼 이미지의 버전은 GitLab Runner의 버전과 엄격하게 연결되어 있다고 간주해야 합니다. 이러한 이미지를 제공하는 주요 이유 중 하나는 GitLab Runner가 `gitlab-runner-helper` 바이너리를 사용하고 있기 때문입니다. 이 바이너리는 GitLab Runner 소스의 일부에서 컴파일됩니다. 이 바이너리는 두 바이너리에서 동일하기로 예상되는 내부 API를 사용합니다.

기본적으로 GitLab Runner는 `registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:XYZ` 이미지를 참조하며, 여기서 `XYZ`는 GitLab Runner 아키텍처 및 Git 개정 기반입니다. [버전 변수](https://gitlab.com/gitlab-org/gitlab-runner/blob/main/common/version.go#L60-61) 중 하나를 사용하여 이미지 버전을 정의할 수 있습니다:

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    helper_image = "my.registry.local/gitlab/gitlab-runner-helper:x86_64-v${CI_RUNNER_VERSION}"
```

이 구성으로 GitLab Runner는 실행기에게 컴파일 데이터를 기반으로 하는 버전 `x86_64-v${CI_RUNNER_VERSION}`의 이미지를 사용하도록 지시합니다. GitLab Runner를 새 버전으로 업데이트한 후 GitLab Runner는 올바른 이미지를 다운로드하려고 시도합니다. 이미지를 GitLab Runner로 업그레이드하기 전에 레지스트리에 업로드해야 하며, 그렇지 않으면 작업이 "이미지 없음" 오류로 실패하기 시작합니다.

헬퍼 이미지는 `$CI_RUNNER_REVISION`에 추가로 `$CI_RUNNER_VERSION`로 태그됩니다. 두 태그 모두 유효하며 동일한 이미지를 가리킵니다.

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    helper_image = "my.registry.local/gitlab/gitlab-runner-helper:x86_64-v${CI_RUNNER_VERSION}"
```

#### PowerShell Core를 사용할 때 {#when-using-powershell-core}

Linux용 헬퍼 이미지의 추가 버전 (PowerShell Core 포함)은 `registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:XYZ-pwsh` 태그로 게시됩니다.

## `[runners.custom_build_dir]` 섹션 {#the-runnerscustom_build_dir-section}

{{< history >}}

- GitLab Runner 11.10에 [도입되었습니다](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/1267).

{{< /history >}}

이 섹션은 [사용자 정의 빌드 디렉터리](https://docs.gitlab.com/ci/runners/configure_runners/#custom-build-directories) 매개변수를 정의합니다.

명시적으로 구성되지 않은 경우 이 기능은 기본적으로 `kubernetes`, `docker`, `docker+machine`, `docker autoscaler` 및 `instance` 실행기에 대해 활성화됩니다. 다른 모든 실행기의 경우 기본적으로 비활성화됩니다.

이 기능을 사용하려면 `GIT_CLONE_PATH`이 `runners.builds_dir`에 정의된 경로에 있어야 합니다. `builds_dir`을 사용하려면 `$CI_BUILDS_DIR` 변수를 사용하세요.

기본적으로 이 기능은 `docker` 및 `kubernetes` 실행기에만 활성화되어 있으므로 리소스를 분리할 수 있습니다. 이 기능은 모든 실행기에 대해 명시적으로 활성화할 수 있지만 `builds_dir`을 공유하고 `concurrent > 1`인 실행기와 함께 사용할 때는 주의하세요.

| 매개변수 | 유형    | 설명 |
|-----------|---------|-------------|
| `enabled` | 부울 | 사용자가 작업에 대한 사용자 정의 빌드 디렉터리를 정의할 수 있습니다. |

예제:

```toml
[runners.custom_build_dir]
  enabled = true
```

### 기본 빌드 디렉터리 {#default-build-directory}

GitLab Runner는 리포지토리를 _빌드 디렉터리_라고 하는 기본 경로 아래에 있는 경로에 복제합니다. 이 기본 디렉터리의 기본 위치는 실행기에 따라 다릅니다. 다음의 경우:

- [Kubernetes](../executors/kubernetes/_index.md) , [Docker](../executors/docker.md) 및 [Docker Machine](../executors/docker_machine.md) 실행기의 경우 컨테이너 내부 `/builds`입니다.
- [Instance](../executors/instance.md)의 경우 SSH 또는 WinRM 연결을 처리하도록 구성된 사용자의 홈 디렉터리의 `~/builds`입니다.
- [Docker Autoscaler](../executors/docker_autoscaler.md)의 경우 컨테이너 내부 `/builds`입니다.
- [Shell](../executors/shell.md) 실행기의 경우 `$PWD/builds`입니다.
- [SSH](../executors/ssh.md) , [VirtualBox](../executors/virtualbox.md) 및 [Parallels](../executors/parallels.md) 실행기의 경우 SSH 연결을 처리하도록 구성된 사용자의 홈 디렉터리의 `~/builds`입니다.
- [Custom](../executors/custom.md) 실행기의 경우 기본값이 제공되지 않으며 명시적으로 구성해야 하며 그렇지 않으면 작업이 실패합니다.

사용된 _빌드 디렉터리_는 [`builds_dir`](#the-runners-section) 설정으로 사용자가 명시적으로 정의할 수 있습니다.

> [!note]
> [사용자 정의 디렉터리에 복제하려면 `GIT_CLONE_PATH`](https://docs.gitlab.com/ci/runners/configure_runners/#custom-build-directories)를 지정할 수도 있으며, 아래 지침이 적용되지 않습니다.

GitLab Runner는 _빌드 디렉터리_를 모든 실행 작업에 사용하지만 특정 패턴 `{builds_dir}/$RUNNER_TOKEN_KEY/$CONCURRENT_PROJECT_ID/$NAMESPACE/$PROJECT_NAME`을 사용하여 중첩합니다. 예: `/builds/2mn-ncv-/0/user/playground`.

GitLab Runner는 _빌드 디렉터리_ 내에 항목을 저장하는 것을 중단하지 않습니다. 예를 들어 `/builds/tools` 내에 CI 실행 중에 사용할 수 있는 도구를 저장할 수 있습니다. 우리는 **HIGHLY** 이를 권장하지 않습니다. _빌드 디렉터리_ 내에 아무것도 저장하면 안 됩니다. GitLab Runner는 완전히 제어해야 하며 그러한 경우 안정성을 제공하지 않습니다. CI에 필요한 종속성이 있으면 다른 곳에 설치해야 합니다.

## Git 구성 정리 {#cleaning-git-configuration}

{{< history >}}

- GitLab Runner 17.10에 [도입되었습니다](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5438).

{{< /history >}}

모든 빌드의 시작과 끝에서 GitLab Runner는 리포지토리 및 하위모듈에서 다음 파일을 제거합니다:

- Git 잠금 파일 (`{index,shallow,HEAD,config}.lock`)
- 체크아웃 후 훅 (`hooks/post-checkout`)

`clean_git_config`을 활성화하면 리포지토리, 해당 하위모듈 및 Git 템플릿 디렉터리에서 다음 추가 파일 또는 디렉터리가 제거됩니다:

- `.git/config` 파일
- `.git/hooks` 디렉터리

이 정리는 사용자 정의, 임시 또는 잠재적으로 악의적인 Git 구성이 작업 간에 캐시되는 것을 방지합니다.

GitLab Runner 17.10 이전에는 정리 동작이 다음과 같습니다:

- Git 잠금 파일 및 체크아웃 후 훅 정리는 작업의 시작 시점에만 발생했으며 끝에서는 발생하지 않았습니다.
- 다른 Git 구성 (`clean_git_config`로 제어됨)은 `FF_ENABLE_JOB_CLEANUP`이 설정되지 않으면 제거되지 않았습니다. 이 플래그를 설정하면 주 리포지토리의 `.git/config`만 삭제되었지만 하위모듈 구성은 삭제되지 않았습니다.

`clean_git_config` 설정의 기본값은 `true`입니다. 그러나 다음의 경우 `false`으로 기본 설정됩니다:

- [Shell 실행기](../executors/shell.md)가 사용됩니다.
- [Git 전략](https://docs.gitlab.com/ci/runners/configure_runners/#git-strategy)이 `none`로 설정됩니다.

명시적 `clean_git_config` 구성은 기본 설정보다 우선합니다.

## `[runners.referees]` 섹션 {#the-runnersreferees-section}

GitLab Runner 추천인을 사용하여 추가 작업 모니터링 데이터를 GitLab에 전달합니다. Referees는 러너 관리자의 워커로 작업과 관련된 추가 데이터를 쿼리하고 수집합니다. 결과는 작업 아티팩트로 GitLab에 업로드됩니다.

### Metrics Runner 추천 사용 {#use-the-metrics-runner-referee}

작업을 실행하는 머신 또는 컨테이너가 [Prometheus](https://prometheus.io) 메트릭을 노출하는 경우 GitLab Runner는 작업 기간 전체 동안 Prometheus 서버를 쿼리할 수 있습니다. 메트릭을 받은 후 나중에 분석에 사용할 수 있는 작업 아티팩트로 업로드됩니다.

[`docker-machine` 실행기](../executors/docker_machine.md)만 추천인을 지원합니다.

### GitLab Runner용 Metrics Runner Referee 구성 {#configure-the-metrics-runner-referee-for-gitlab-runner}

`[runner.referees]` 및 `[runner.referees.metrics]`을 `[[runner]]` 섹션의 `config.toml` 파일에 정의하고 다음 필드를 추가하세요:

| 설정              | 설명 |
|----------------------|-------------|
| `prometheus_address` | GitLab Runner 인스턴스에서 메트릭을 수집하는 서버입니다. 작업이 완료될 때 러너 관리자가 액세스할 수 있어야 합니다. |
| `query_interval`     | 작업과 관련된 Prometheus 인스턴스가 시계열 데이터에 대해 쿼리되는 빈도 (초 단위 간격으로 정의됨)입니다. |
| `queries`            | 각 간격에 대해 실행되는 [PromQL](https://prometheus.io/docs/prometheus/latest/querying/basics/) 쿼리의 배열입니다. |

`node_exporter` 메트릭에 대한 완전한 구성 예시입니다:

```toml
[[runners]]
  [runners.referees]
    [runners.referees.metrics]
      prometheus_address = "http://localhost:9090"
      query_interval = 10
      metric_queries = [
        "arp_entries:rate(node_arp_entries{{selector}}[{interval}])",
        "context_switches:rate(node_context_switches_total{{selector}}[{interval}])",
        "cpu_seconds:rate(node_cpu_seconds_total{{selector}}[{interval}])",
        "disk_read_bytes:rate(node_disk_read_bytes_total{{selector}}[{interval}])",
        "disk_written_bytes:rate(node_disk_written_bytes_total{{selector}}[{interval}])",
        "memory_bytes:rate(node_memory_MemTotal_bytes{{selector}}[{interval}])",
        "memory_swap_bytes:rate(node_memory_SwapTotal_bytes{{selector}}[{interval}])",
        "network_tcp_active_opens:rate(node_netstat_Tcp_ActiveOpens{{selector}}[{interval}])",
        "network_tcp_passive_opens:rate(node_netstat_Tcp_PassiveOpens{{selector}}[{interval}])",
        "network_receive_bytes:rate(node_network_receive_bytes_total{{selector}}[{interval}])",
        "network_receive_drops:rate(node_network_receive_drop_total{{selector}}[{interval}])",
        "network_receive_errors:rate(node_network_receive_errs_total{{selector}}[{interval}])",
        "network_receive_packets:rate(node_network_receive_packets_total{{selector}}[{interval}])",
        "network_transmit_bytes:rate(node_network_transmit_bytes_total{{selector}}[{interval}])",
        "network_transmit_drops:rate(node_network_transmit_drop_total{{selector}}[{interval}])",
        "network_transmit_errors:rate(node_network_transmit_errs_total{{selector}}[{interval}])",
        "network_transmit_packets:rate(node_network_transmit_packets_total{{selector}}[{interval}])"
      ]
```

메트릭 쿼리는 `canonical_name:query_string` 형식입니다. 쿼리 문자열은 실행 중에 대체되는 두 변수를 지원합니다:

| 설정      | 설명 |
|--------------|-------------|
| `{selector}` | Prometheus에서 생성된 메트릭을 선택하는 `label_name=label_value` 쌍으로 대체됩니다. (특정 GitLab Runner 인스턴스) |
| `{interval}` | 이 Referee를 위해 `[runners.referees.metrics]` 구성에서 `query_interval` 매개변수로 대체됩니다. |

예를 들어 `docker-machine` 실행기를 사용하는 공유 GitLab Runner 환경은 `{selector}`이 `node=shared-runner-123`과 비슷한 것을 가질 것입니다.
