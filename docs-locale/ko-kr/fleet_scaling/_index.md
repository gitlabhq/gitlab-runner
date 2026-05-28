---
stage: Verify
group: CI Functions Platform
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: 러너 플릿을 계획하고 운영하기
---

공유 서비스 모델에서 러너 플릿의 규모를 조정할 때 다음의 베스트 프랙티스와 권장사항을 적용하세요.

인스턴스 러너 플릿을 호스팅할 때 다음을 고려한 잘 계획된 인프라가 필요합니다:

- 컴퓨팅 용량
- 스토리지 용량
- 네트워크 대역폭 및 처리량
- 작업 유형(프로그래밍 언어, OS 플랫폼 및 종속 라이브러리 포함)

조직의 요구사항에 따라 GitLab Runner 배포 전략을 개발하려면 이 권장사항을 사용하세요.

## 워크로드 및 환경 고려 {#consider-your-workload-and-environment}

러너를 배포하기 전에 워크로드 및 환경 요구사항을 고려하세요.

- GitLab에 온보딩할 계획인 팀의 목록을 작성하세요.
- 조직에서 사용 중인 프로그래밍 언어, 웹 프레임워크 및 라이브러리를 카탈로그화하세요. 예: Go, C++, PHP, Java, Python, JavaScript, React, Node.js
- 각 팀이 시간당, 일당 실행할 수 있는 CI/CD 작업의 수를 추정하세요.
- 컨테이너를 사용하여 해결할 수 없는 빌드 환경 요구사항이 있는 팀이 있는지 확인하세요.
- 해당 팀에 전용 러너를 두는 것이 가장 좋은 빌드 환경 요구사항이 있는 팀이 있는지 확인하세요.
- 예상 수요를 지원하기 위해 필요할 수 있는 컴퓨팅 용량을 추정하세요.

다양한 러너 플릿을 호스팅하기 위해 다양한 인프라 스택을 선택할 수 있습니다. 예를 들어 일부 러너는 퍼블릭 클라우드에, 일부는 온프레미스에 배포해야 할 수 있습니다.

러너 플릿의 CI/CD 작업의 성능은 직접 플릿의 환경과 관련이 있습니다. 많은 수의 리소스 집약적인 CI/CD 작업을 실행하는 경우 공유 컴퓨팅 플랫폼에 플릿을 호스팅하는 것은 권장하지 않습니다.

## 러너, 실행기 및 자동 크기 조정 기능 {#runners-executors-and-autoscaling-capabilities}

`gitlab-runner` 실행 파일이 CI/CD 작업을 실행합니다. 각 러너는 작업 실행 요청을 수집하고 사전 정의된 설정에 따라 처리하는 격리된 프로세스입니다. 격리된 프로세스인 각 러너는 작업을 실행하기 위해 "하위 프로세스"(또는 "워커"라고도 함)를 생성할 수 있습니다.

### 동시성 및 제한 {#concurrency-and-limit}

- [동시성](../configuration/advanced-configuration.md#the-global-section):  호스트 시스템의 모든 구성된 러너를 사용할 때 동시에 실행할 수 있는 작업의 수를 설정합니다.
- [제한](../configuration/advanced-configuration.md#the-runners-section):  러너가 작업을 동시에 실행하기 위해 생성할 수 있는 하위 프로세스의 수를 설정합니다.

제한은 자동 크기 조정 러너(예: Docker Machine 및 Kubernetes)의 경우 자동 크기 조정이 아닌 러너의 경우와 다릅니다.

- 자동 크기 조정이 아닌 러너의 경우 `limit`는 호스트 시스템의 러너 용량을 정의합니다.
- 자동 크기 조정 러너의 경우 `limit`는 총 실행할 러너의 수입니다.

`concurrency`, `limit`, `request_concurrency`이 작업 흐름을 제어하는 방법에 대한 자세한 정보는 [GitLab Runner 동시성 튜닝에 대한 KB 문서](https://support.gitlab.com/hc/en-us/articles/21324350882076-GitLab-Runner-Concurrency-Tuning-Understanding-request-concurrency)를 참조하세요.

### 기본 구성: 한 개의 러너 관리자, 한 개의 러너 {#basic-configuration-one-runner-manager-one-runner}

가장 기본적인 구성의 경우 지원되는 컴퓨팅 아키텍처 및 운영 체제에 GitLab Runner 소프트웨어를 설치합니다. 예를 들어 Ubuntu Linux를 실행하는 x86-64 가상 머신(VM)이 있을 수 있습니다.

설치가 완료된 후 러너 등록 명령을 한 번만 실행하고 `shell` 실행기를 선택합니다. 그런 다음 러너 `config.toml` 파일을 편집하여 동시성을 `1`로 설정합니다.

```toml
concurrent = 1

[[runners]]
  name = "instance-level-runner-001"
  url = ""
  token = ""
  executor = "shell"
```

이 러너가 처리할 수 있는 GitLab CI/CD 작업은 러너를 설치한 호스트 시스템에서 직접 실행됩니다. 마치 터미널에서 직접 CI/CD 작업 명령을 실행하는 것과 같습니다. 이 경우 등록 명령을 한 번만 실행했으므로 `config.toml` 파일에는 `[[runners]]` 섹션만 하나 포함됩니다. 동시성 값을 `1`로 설정한다고 가정하면 이 시스템의 러너 프로세스에 대해 하나의 러너 "워커"만 CI/CD 작업을 실행할 수 있습니다.

### 중급 구성: 한 개의 러너 관리자, 여러 개의 러너 {#intermediate-configuration-one-runner-manager-multiple-runners}

동일한 머신에 여러 러너를 등록할 수도 있습니다. 이렇게 하면 러너의 `config.toml` 파일에 여러 `[[runners]]` 섹션이 포함됩니다. 모든 추가 러너 워커가 셸 실행기를 사용하고 전역 `concurrent` 설정 값을 `3`로 업데이트하면 호스트에서 최대 3개의 작업을 한 번에 실행할 수 있습니다.

```toml
concurrent = 3

[[runners]]
  name = "instance_level_shell_001"
  url = ""
  token = ""
  executor = "shell"

[[runners]]
  name = "instance_level_shell_002"
  url = ""
  token = ""
  executor = "shell"

[[runners]]
  name = "instance_level_shell_003"
  url = ""
  token = ""
  executor = "shell"

```

동일한 머신에 많은 러너 워커를 등록할 수 있으며 각각은 격리된 프로세스입니다. 각 워커에 대한 CI/CD 작업의 성능은 호스트 시스템의 컴퓨팅 용량에 따라 다릅니다.

### 자동 크기 조정 구성: 한 개 이상의 러너 관리자, 여러 워커 {#autoscaling-configuration-one-or-more-runner-managers-multiple-workers}

GitLab Runner가 자동 크기 조정을 위해 설정되면 러너를 다른 러너의 관리자 역할을 하도록 구성할 수 있습니다. `docker-machine` 또는 `kubernetes` 실행기로 이 작업을 수행할 수 있습니다. 이 유형의 관리자 전용 구성에서 러너 에이전트는 자체적으로 어떤 CI/CD 작업도 실행하지 않습니다.

#### Docker Machine 실행기 {#docker-machine-executor}

[Docker Machine 실행기](../executors/docker_machine.md)를 사용하면:

- 러너 관리자가 Docker를 사용하여 온디맨드 가상 머신 인스턴스를 프로비저닝합니다.
- 이 VM에서 GitLab Runner는 `.gitlab-ci.yml` 파일에서 지정한 컨테이너 이미지를 사용하여 CI/CD 작업을 실행합니다.
- 다양한 머신 유형에서 CI/CD 작업의 성능을 테스트해야 합니다.
- 속도 또는 비용을 기준으로 컴퓨팅 호스트를 최적화하는 것을 고려해야 합니다.

#### Kubernetes 실행기 {#kubernetes-executor}

[Kubernetes 실행기](../executors/kubernetes/_index.md)를 사용하면:

- 러너 관리자가 대상 Kubernetes 클러스터에 Pod을 프로비저닝합니다.
- CI/CD 작업은 여러 컨테이너로 구성된 각 Pod에서 실행됩니다.
- 작업 실행에 사용되는 Pod은 일반적으로 러너 관리자를 호스팅하는 Pod보다 더 많은 컴퓨팅 및 메모리 리소스가 필요합니다.

#### 러너 구성 재사용 {#reusing-a-runner-configuration}

동일한 러너 인증 토큰과 관련된 각 러너 관리자에게 `system_id` 식별자가 할당됩니다. `system_id`는 러너를 사용하는 머신을 식별합니다. 동일한 인증 토큰으로 등록된 러너는 고유한 `system_id.`로 단일 러너 항목 아래에 그룹화됩니다.

유사한 러너를 단일 구성 아래에 그룹화하면 러너 플릿 운영이 간소화됩니다.

유사한 러너를 단일 구성 아래에 그룹화할 수 있는 예시 시나리오가 있습니다:

플랫폼 관리자는 `docker-builds-2vCPU-8GB` 태그를 사용하여 동일한 기본 가상 머신 인스턴스 크기(2 vCPU, 8GB RAM)를 가진 여러 러너를 제공해야 합니다. 고가용성 또는 확장을 위해 최소 2개의 이러한 러너가 필요합니다. UI에서 2개의 개별 러너 항목을 생성하는 대신 관리자는 동일한 컴퓨팅 인스턴스 크기를 가진 모든 러너에 대해 하나의 러너 구성을 생성할 수 있습니다. 러너 구성의 인증 토큰을 재사용하여 여러 러너를 등록할 수 있습니다. 등록된 각 러너는 `docker-builds-2vCPU-8GB` 태그를 상속합니다. 단일 러너 구성의 모든 하위 러너의 경우 `system_id`는 고유한 식별자 역할을 합니다.

그룹화된 러너는 여러 러너 관리자에 의해 다양한 작업을 실행하도록 재사용될 수 있습니다.

GitLab Runner는 `system_id`을 시작할 때 또는 구성을 저장할 때 생성합니다. `system_id`는 [`config.toml`](../configuration/advanced-configuration.md)와 동일한 디렉터리의 `.runner_system_id` 파일에 저장되며 작업 로그 및 러너 관리 페이지에 표시됩니다.

##### `system_id` 식별자 생성 {#generating-system_id-identifiers}

`system_id`를 생성하기 위해 GitLab Runner는 하드웨어 식별자(예: 일부 Linux 배포판의 `/etc/machine-id`)에서 고유한 시스템 식별자를 파생시키려고 시도합니다. 성공하지 못한 경우 GitLab Runner는 임의 식별자를 사용하여 `system_id`를 생성합니다.

`system_id`은 다음 접두사 중 하나를 가집니다:

- `r_`: GitLab Runner가 임의 식별자를 할당했습니다.
- `s_`: GitLab Runner가 하드웨어 식별자에서 고유한 시스템 식별자를 할당했습니다.

예를 들어 컨테이너 이미지를 생성할 때 `system_id`이 이미지에 하드코딩되지 않도록 이를 고려하는 것이 중요합니다. `system_id`이 하드코딩된 경우 주어진 작업을 실행하는 호스트를 구분할 수 없습니다.

##### 러너 및 러너 관리자 삭제 {#delete-runners-and-runner-managers}

러너 등록 토큰(더 이상 사용되지 않음)으로 등록된 러너 및 러너 관리자를 삭제하려면 `gitlab-runner unregister` 명령을 사용하세요.

러너 인증 토큰으로 생성된 러너 및 러너 관리자를 삭제하려면 [UI](https://docs.gitlab.com/ci/runners/runners_scope/#delete-instance-runners) 또는 [API](https://docs.gitlab.com/api/runners/#delete-a-runner)를 사용하세요. 러너 인증 토큰으로 생성된 러너는 여러 머신에서 재사용할 수 있는 재사용 가능한 구성입니다. [`gitlab-runner unregister`](../commands/_index.md#gitlab-runner-unregister) 명령을 사용하면 러너 관리자만 삭제되고 러너는 삭제되지 않습니다.

## 인스턴스 러너 구성 {#configure-instance-runners}

인스턴스 러너를 자동 크기 조정 구성에서 사용하는 것(러너이 "러너 관리자" 역할을 하는 경우)은 시작하기 위한 효율적이고 효과적인 방법입니다.

VM 또는 Pod을 호스팅하는 인프라 스택의 컴퓨팅 용량은 다음에 따라 달라집니다:

- 워크로드 및 환경을 고려할 때 캡처한 요구사항
- 러너 플릿을 호스팅하기 위해 사용하는 기술 스택입니다.

CI/CD 워크로드를 실행하기 시작하고 시간에 따라 성능을 분석한 후 컴퓨팅 용량을 조정해야 할 수 있습니다.

자동 크기 조정 실행기와 함께 인스턴스 러너를 사용하는 구성의 경우 최소 2개의 러너 관리자로 시작해야 합니다.

시간이 지남에 따라 필요할 수 있는 러너 관리자의 총 개수는 다음에 따라 달라집니다:

- 러너 관리자를 호스팅하는 스택의 컴퓨팅 리소스
- 각 러너 관리자에 대해 구성하도록 선택한 동시성
- 각 관리자가 매시간, 매일, 매월 실행하는 CI/CD 작업에서 생성하는 로드입니다.

예를 들어 GitLab.com에서는 Docker Machine 실행기를 사용하여 7개의 러너 관리자를 실행합니다. 각 CI/CD 작업은 Google Cloud Platform(GCP) `n1-standard-1` VM에서 실행됩니다. 이 구성으로 월별로 수백만 개의 작업을 처리합니다.

## 러너 모니터링 {#monitoring-runners}

규모에 맞게 러너 플릿을 운영하는 필수 단계는 GitLab에 포함된 [러너 플릿 모니터링](../monitoring/_index.md) 기능을 설정하고 사용하는 것입니다.

다음 표는 GitLab Runner 메트릭의 요약을 포함합니다. 이 목록은 Go 관련 프로세스 메트릭을 포함하지 않습니다. 러너에서 이러한 메트릭을 보려면 [사용 가능한 메트릭](../monitoring/_index.md#available-metrics)에 표시된 명령을 실행하세요.

| 메트릭 이름                                                    | 설명 |
|----------------------------------------------------------------|-------------|
| `gitlab_runner_api_request_statuses_total`                     | , 엔드포인트 및 상태별로 분할된 API 요청의 총 개수입니다. |
| `gitlab_runner_autoscaling_machine_creation_duration_seconds`  | 머신 생성 시간의 히스토그램입니다. |
| `gitlab_runner_autoscaling_machine_states`                     | 이 제공자의 상태별 머신 수입니다. |
| `gitlab_runner_concurrent`                                     | 동시성 설정의 값입니다. |
| `gitlab_runner_errors_total`                                   | 캡처된 오류의 수입니다. 이 메트릭은 로그 라인을 추적하는 카운터입니다. 메트릭에는 `level` 레이블이 포함됩니다. 가능한 값은 `warning` 및 `error`입니다. 이 메트릭을 포함할 계획이면 관찰할 때 `rate()` 또는 `increase()`를 사용하세요. 즉, 경고 또는 오류의 속도가 증가하는 것을 발견하면 이는 추가 조사가 필요한 문제를 시사할 수 있습니다. |
| `gitlab_runner_jobs`                                           | 이것은 실행 중인 작업의 수를 보여줍니다(레이블에 다양한 범위가 있음). |
| `gitlab_runner_job_duration_seconds`                           | 작업 기간의 히스토그램입니다. |
| `gitlab_runner_job_queue_duration_seconds`                     | 작업 큐 기간을 나타내는 히스토그램입니다. |
| `gitlab_runner_acceptable_job_queuing_duration_exceeded_total` | 작업이 구성된 큐잉 시간 임계값을 초과하는 빈도를 계산합니다. |
| `gitlab_runner_job_stage_duration_seconds`                     | 각 스테이지별 작업 기간을 나타내는 히스토그램입니다. 이 메트릭은 **high cardinality metric**입니다. 자세한 정보는 [높은 카디널리티 메트릭 섹션](#high-cardinality-metrics)을 참조하세요. |
| `gitlab_runner_jobs_total`                                     | 이것은 실행된 총 작업을 표시합니다. |
| `gitlab_runner_job_execution_mode_total`                       | 이것은 모드(`steps` 또는 `traditional`) 및 실행기별로 실행된 총 작업을 표시합니다. |
| `gitlab_runner_limit`                                          | 제한 설정의 현재 값입니다. |
| `gitlab_runner_request_concurrency`                            | 새 작업에 대한 현재 동시 요청 수입니다. |
| `gitlab_runner_request_concurrency_exceeded_total`             | 구성된 `request_concurrency` 제한 이상의 초과 요청 수입니다. |
| `gitlab_runner_version_info`                                   | 다양한 빌드 통계 필드로 레이블이 지정된 상수 `1` 값을 가진 메트릭입니다. |
| `process_cpu_seconds_total`                                    | 초 단위로 소비된 총 사용자 및 시스템 CPU 시간입니다. |
| `process_max_fds`                                              | 열린 파일 설명자의 최대 수입니다. |
| `process_open_fds`                                             | 열린 파일 설명자의 수입니다. |
| `process_resident_memory_bytes`                                | 상주 메모리 크기(바이트)입니다. |
| `process_start_time_seconds`                                   | Unix epoch로부터 초 단위로 측정된 프로세스의 시작 시간입니다. |
| `process_virtual_memory_bytes`                                 | 가상 메모리 크기(바이트)입니다. |
| `process_virtual_memory_max_bytes`                             | 사용 가능한 가상 메모리의 최대 양(바이트)입니다. |

### Grafana 대시보드 구성 팁 {#grafana-dashboard-configuration-tips}

이 [공개 저장소](https://gitlab.com/gitlab-com/runbooks/-/tree/master/dashboards/ci-runners)에서 GitLab.com의 러너 플릿을 운영하는 데 사용하는 Grafana 대시보드의 소스 코드를 찾을 수 있습니다.

GitLab.com에 대해 많은 메트릭을 추적합니다. 클라우드 기반 CI/CD의 대규모 제공자이므로 문제를 디버깅할 수 있도록 시스템의 다양한 보기가 필요합니다. 대부분의 경우 자체 관리 러너 플릿은 GitLab.com으로 추적하는 메트릭의 양을 추적할 필요가 없습니다.

#### 대시보드 생성 프로세스 {#dashboard-generation-process}

Grafana는 JSON 형식만 허용하므로 `jsonnet` 파일을 JSON으로 변환해야 합니다.

[런북 저장소](https://gitlab.com/gitlab-com/runbooks/-/tree/master/dashboards)는 GitLab 인프라 전용 자동화된 스크립트를 포함합니다. 사용자 환경에 대해 이 대시보드를 생성하려면:

1. `jsonnet` 구성 언어를 사용하여 대시보드를 생성합니다(`.dashboard.jsonnet` 파일).
1. `jsonnet` 파일을 `jsonnet` 라이브러리로 처리하여 JSON 출력을 생성합니다.
1. 생성된 JSON 파일을 Grafana에 업로드합니다(API 또는 UI 사용).

#### 사용 가능한 러너 대시보드 {#available-runner-dashboards}

러너 플릿을 모니터링하기 위해 사용해야 할 필수 대시보드가 몇 개 있습니다:

러너에서 시작된 작업:

- 선택한 시간 간격 동안 러너 플릿에서 실행된 총 작업의 개요를 봅니다.
- 사용 추세를 봅니다. 최소한 주 단위로 이 대시보드를 분석해야 합니다.
- 이 데이터를 작업 기간과 같은 메트릭과 연관지어 CI/CD 작업 성능 SLO를 충족하기 위해 구성 변경이나 용량 업그레이드가 필요한지 판단합니다.

작업 기간:

- 러너 플릿의 성능 및 확장을 분석합니다.
- 성능 병목 현상 및 최적화 기회를 식별합니다.

러너 용량:

- 제한 또는 동시성의 값으로 나눈 실행되는 작업의 수를 봅니다.
- 추가 작업을 실행할 여유 용량이 있는지 판단합니다.
- 사용률 추세에 따라 용량 업그레이드를 계획합니다.

추가 대시보드는 다음을 포함합니다:

- 메인 대시보드(`main.dashboard.jsonnet`):  러너 인프라 및 HAProxy 메트릭의 개요입니다.
- 비즈니스 메트릭(`business-stats.dashboard.jsonnet`):  작업 통계, 완료된 작업 분, 러너 포화도입니다.
- 자동 크기 조정 알고리즘(`autoscaling-algorithm.dashboard.jsonnet`):  자동 크기 조정 동작 및 머신 상태의 시각화입니다.
- 큐잉 개요(`queuing-overview.dashboard.jsonnet`):  작업 큐 깊이 및 대기 시간입니다.
- 요청 동시성(`request-concurrency.dashboard.jsonnet`):  동시 요청 분석입니다.
- 배포(`deployment.dashboard.jsonnet`):  배포 관련 메트릭입니다.
- 인시던트 대시보드:  자동 크기 조정, 데이터베이스, 애플리케이션 및 러너 관리자 문제 해결을 위한 특화된 대시보드입니다.

각 대시보드는 표시되는 메트릭을 설명하기 위해 원본 `jsonnet` 파일에 설명 및 컨텍스트를 포함합니다.

### 템플릿 변수 {#template-variables}

대시보드는 Grafana 템플릿 변수를 사용하여 다양한 컨텍스트에서 재사용 가능한 대시보드 템플릿을 생성합니다:

- 환경:  예: `production`, `staging`, `development`.
- 스테이지:  예: `main`, `canary`.
- 유형:  예: `ci`, `verify`. 사용 사례에 따라 다릅니다.
- 샤드:  선택사항입니다. 분산 러너 배포의 경우입니다.

이 대시보드를 구현하는 조직은 이 변수를 자신의 환경 구조와 일치하도록 조정해야 합니다. 가져온 후 Grafana 대시보드 설정에서 이 변수를 업데이트하세요.

### 지원되는 러너 {#supported-runners}

이 대시보드는 모든 GitLab Runner 실행기 유형과 함께 작동합니다:

- Kubernetes
- 셸
- VM(Docker Machine)
- Windows

메트릭 수집은 실행기 독립적이며 모든 러너 플릿 유형에서 사용할 수 있습니다.

### 대시보드 사용자 정의 {#customize-dashboards}

환경의 대시보드를 수정하려면:

1. `.dashboard.jsonnet` 파일을 `dashboards/ci-runners/` 디렉터리에서 편집합니다.
1. [Grafonnet 라이브러리](https://grafana.github.io/grafonnet-lib/) 구문을 사용합니다(`jsonnet`에 기반).
1. 플레이그라운드를 사용하여 변경 사항을 테스트합니다:

   ```shell
   ./test-dashboard.sh dashboards/ci-runners/your-dashboard.dashboard.jsonnet
   ```

1. `./generate-dashboards.sh`를 사용하여 재생성 및 배포합니다.

자세한 정보는 [대시보드 확장에 대한 비디오 가이드](https://www.youtube.com/watch?v=yZ2RiY_Akz0)를 참조하세요.

### Kubernetes의 러너 모니터링에 대한 고려사항 {#considerations-for-monitoring-runners-on-kubernetes}

OpenShift, EKS 또는 GKE와 같은 Kubernetes 플랫폼에서 호스팅된 러너 플릿의 경우 Grafana 대시보드를 설정하기 위해 다른 방식을 사용하세요.

Kubernetes에서 러너 CI/CD 작업 실행 Pod은 자주 생성되고 삭제될 수 있습니다. 이 경우 러너 관리자 Pod을 모니터링할 계획을 세우고 다음을 구현하는 것을 고려해야 합니다:

- 게이지:  다양한 소스에서 동일한 메트릭의 합계를 표시합니다.
- 카운터:  `rate` 또는 `increase` 함수를 적용할 때 카운터를 재설정합니다.

## 높은 카디널리티 메트릭 {#high-cardinality-metrics}

일부 메트릭은 높은 카디널리티로 인해 수집 및 저장에 리소스 집약적일 수 있습니다. 높은 카디널리티는 메트릭에 많은 가능한 값을 가진 레이블이 포함되어 많은 수의 고유 시계열 데이터 포인트가 발생할 때 발생합니다.

성능을 최적화하기 위해 이러한 메트릭은 기본적으로 활성화되지 않으며 [FF_EXPORT_HIGH_CARDINALITY_METRICS 기능 플래그](../configuration/feature-flags.md)를 사용하여 토글할 수 있습니다.

### 높은 카디널리티 메트릭 목록 {#list-of-high-cardinality-metrics}

- `gitlab_runner_job_stage_duration_seconds`: 개별 작업 스테이지의 기간(초)을 측정합니다. 이 메트릭은 `stage` 레이블을 포함하며 다음의 사전 정의된 값을 가질 수 있습니다:

  - `resolve_secrets`
  - `prepare_executor`
  - `prepare_script`
  - `get_sources`
  - `clear_worktree`
  - `restore_cache`
  - `download_artifacts`
  - `after_script`
  - `step_script`
  - `archive_cache`
  - `archive_cache_on_failure`
  - `upload_artifacts_on_success`
  - `upload_artifacts_on_failure`
  - `cleanup_file_variables`

  또한 이 목록은 `step_run`와 같은 사용자 지정 사용자 정의 스테이지를 포함할 수 있습니다.

### 높은 카디널리티 메트릭 관리 {#managing-high-cardinality-metrics}

[Prometheus 재레이블 구성](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config)을 사용하여 불필요한 레이블 값 또는 전체 메트릭을 제거하여 카디널리티를 제어하고 감소시킬 수 있습니다.

#### 특정 스테이지를 제거하기 위한 예시 구성 {#example-configuration-to-remove-specific-stages}

다음 구성은 `stage` 레이블의 `prepare_executor` 값을 가진 모든 메트릭을 제거합니다:

```yaml
scrape_configs:
  - job_name: 'gitlab_runner_metrics'
    static_configs:
      - targets: ['localhost:9252']
    metric_relabel_configs:
      - source_labels: [__name__, "stage"]
        regex: "gitlab_runner_job_stage_duration_seconds;prepare_executor"
        action: drop
```

#### 관련 스테이지만 유지하는 예시 {#example-to-keep-only-relevant-stages}

다음 구성은 `step_script` 스테이지의 메트릭만 유지하고 다른 메트릭을 모두 버립니다:

```yaml
scrape_configs:
  - job_name: 'gitlab_runner_metrics'
    static_configs:
      - targets: ['localhost:9252']
    metric_relabel_configs:
      - source_labels: [__name__, "stage"]
        regex: "gitlab_runner_job_stage_duration_seconds;step_script"
        action: keep
```
