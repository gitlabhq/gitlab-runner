---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Docker Machine 실행기 자동 크기 조정 구성
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

> [!note]
> Docker Machine 실행기는 GitLab 17.5에서 더 이상 지원되지 않으며 GitLab 20.0(2027년 5월)에서 제거될 예정입니다. GitLab 20.0까지 Docker Machine 실행기를 계속 지원하지만 새 기능을 추가할 계획은 없습니다. CI/CD 작업 실행을 방지하거나 실행 비용에 영향을 미칠 수 있는 중요한 버그만 해결합니다. Amazon Web Services(AWS) EC2, Microsoft Azure Compute 또는 Google Compute Engine(GCE)에서 Docker Machine 실행기를 사용 중이라면 [GitLab Runner Autoscaler](../runner_autoscale/_index.md)로 마이그레이션하세요.

자동 크기 조정 기능을 사용하면 더욱 탄력적이고 동적인 방식으로 리소스를 사용할 수 있습니다.

GitLab Runner는 자동 크기 조정할 수 있으므로 인프라에 항상 필요한 만큼의 빌드 인스턴스만 포함됩니다. GitLab Runner를 자동 크기 조정만 사용하도록 구성하면 GitLab Runner를 호스팅하는 시스템이 생성하는 모든 머신의 배스천 역할을 합니다. 이 머신을 "Runner Manager"라고 합니다.

> [!note]
> Docker는 공용 클라우드 가상 머신에서 러너를 자동 크기 조정하는 데 사용되는 기본 기술인 Docker Machine을 더 이상 지원하지 않습니다. [Docker Machine 지원 중단에 대한 대응 전략](https://gitlab.com/gitlab-org/gitlab/-/issues/341856)을 논의하는 이슈를 읽을 수 있습니다.

Docker Machine 자동 크기 조정기는 `limit` 및 `concurrent` 구성과 관계없이 VM당 하나의 컨테이너를 생성합니다.

이 기능을 올바르게 활성화하고 구성하면 _필요에 따라_ 생성된 머신에서 작업을 실행합니다. 이 머신들은 작업이 완료된 후 다음 작업을 실행하기 위해 대기할 수 있거나 구성된 `IdleTime` 후에 제거될 수 있습니다. 많은 클라우드 제공자의 경우 이 방식은 기존 인스턴스를 사용하여 비용을 절감합니다.

아래에서 [GitLab Community Edition](https://gitlab.com/gitlab-org/gitlab-foss) 프로젝트에 대해 GitLab.com에서 테스트한 GitLab Runner 자동 크기 조정 기능의 실제 예시를 확인할 수 있습니다:

![자동 크기 조정의 실제 예시](img/autoscale-example.png)

차트의 각 머신은 Docker 컨테이너 내에서 작업을 실행하는 독립적인 클라우드 인스턴스입니다.

## 시스템 요구 사항 {#system-requirements}

자동 크기 조정을 구성하기 전에 다음을 수행해야 합니다:

- [자신의 환경을 준비하세요](../executors/docker_machine.md#preparing-the-environment).
- 필요에 따라 GitLab에서 제공하는 [포크 버전](../executors/docker_machine.md#forked-version-of-docker-machine)의 Docker 머신을 사용합니다. 이는 몇 가지 추가 수정 사항을 포함합니다.

## 지원되는 클라우드 제공자 {#supported-cloud-providers}

자동 크기 조정 메커니즘은 [Docker Machine](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/tree/main/)을 기반으로 합니다. 지원되는 모든 가상화 및 클라우드 제공자 매개변수는 GitLab에서 관리하는 [Docker Machine](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/tree/main/) 포크에서 사용할 수 있습니다.

## 러너 구성 {#runner-configuration}

이 섹션에서는 중요한 자동 크기 조정 매개변수에 대해 설명합니다. 더 많은 구성 세부 사항은 [고급 구성](advanced-configuration.md)을 읽으세요.

### 러너 전역 옵션 {#runner-global-options}

| 매개변수    | 값   | 설명 |
|--------------|---------|-------------|
| `concurrent` | 정수 | 전역적으로 동시에 실행할 수 있는 작업 수를 제한합니다. 이 매개변수는 로컬 및 자동 스케일링을 포함한 정의된 _모든_ 러너가 사용할 수 있는 최대 작업 수를 설정합니다. `limit`([`[[runners]]` 섹션](#runners-options)에서) 및 `IdleCount`([`[runners.machine]` 섹션](advanced-configuration.md#the-runnersmachine-section)에서)와 함께 생성된 머신의 상한선에 영향을 미칩니다. |

### `[[runners]]` 옵션 {#runners-options}

| 매개변수  | 값   | 설명 |
|------------|---------|-------------|
| `executor` | 문자열  | 자동 크기 조정 기능을 사용하려면 `executor`을(를) `docker+machine`로 설정해야 합니다. |
| `limit`    | 정수 | 이 특정 토큰으로 동시에 처리할 수 있는 작업 수를 제한합니다. `0`은(는) 제한 없음을 의미합니다. 자동 크기 조정의 경우 이 제공자가 생성한 머신의 상한선입니다(`concurrent` 및 `IdleCount`과 함께). |

### `[runners.machine]` 옵션 {#runnersmachine-options}

구성 매개변수 세부 사항은 [`[runners.machine]` 섹션 - GitLab Runner 고급 구성](advanced-configuration.md#the-runnersmachine-section)에서 찾을 수 있습니다.

### `[runners.cache]` 옵션 {#runnerscache-options}

구성 매개변수 세부 사항은 [`[runners.cache]` 섹션 - GitLab Runner 고급 구성](advanced-configuration.md#the-runnerscache-section)에서 찾을 수 있습니다

### 추가 구성 정보 {#additional-configuration-information}

`IdleCount = 0`을(를) 설정할 때의 특수 모드도 있습니다. 이 모드에서는 머신이 **항상** 각 작업 전에 **on-demand** 생성됩니다(유휴 상태인 사용 가능한 머신이 없는 경우). 작업이 완료된 후 자동 크기 조정 알고리즘은 [아래에서 설명한 것과 동일하게 작동합니다](#autoscaling-algorithm-and-parameters). 머신은 다음 작업을 기다리고 있으며 실행되지 않으면 `IdleTime` 기간 후 머신이 제거됩니다. 작업이 없으면 유휴 상태인 머신이 없습니다.

`IdleCount`이(가) `0`보다 큰 값으로 설정되면 유휴 VM이 백그라운드에서 생성됩니다. 러너는 새 작업을 요청하기 전에 기존 유휴 VM을 획득합니다.

- 작업이 러너에 할당되면 그 작업은 이전에 획득한 VM으로 전송됩니다.
- 작업이 러너에 할당되지 않으면 유휴 VM에 대한 잠금이 해제되고 VM이 풀로 반환됩니다.

## Docker Machine 실행기로 생성된 VM 수 제한 {#limit-the-number-of-vms-created-by-the-docker-machine-executor}

Docker Machine 실행기로 생성된 가상 머신(VM) 수를 제한하려면 `config.toml` 파일의 `[[runners]]` 섹션에서 `limit` 매개변수를 사용하세요.

`concurrent` 매개변수**does not** VM 수를 제한하지 않습니다.

하나의 프로세스를 여러 러너 워커를 관리하도록 구성할 수 있습니다. 자세한 내용은 [기본 구성: 하나의 러너 매니저, 하나의 러너](../fleet_scaling/_index.md#basic-configuration-one-runner-manager-one-runner)를 참조하세요.

이 예시는 하나의 러너 프로세스에 대해 `config.toml` 파일에 설정된 값을 보여줍니다:

```toml
concurrent = 100

[[runners]]
name = "first"
executor = "shell"
limit = 40
(...)

[[runners]]
name = "second"
executor = "docker+machine"
limit = 30
(...)

[[runners]]
name = "third"
executor = "ssh"
limit = 10

[[runners]]
name = "fourth"
executor = "virtualbox"
limit = 20
(...)

```

이 구성을 사용하면:

- 하나의 러너 프로세스는 서로 다른 실행 환경을 사용하는 4개의 다른 러너 워커를 생성할 수 있습니다.
- `concurrent` 값이 100으로 설정되므로 이 하나의 러너는 최대 100개의 동시 GitLab CI/CD 작업을 실행합니다.
- `second` 러너 워커만 Docker Machine 실행기를 사용하도록 구성되어 있으므로 VM을 자동으로 생성할 수 있습니다.
- `limit` 설정이 `30`이면 `second` 러너 워커는 언제든지 자동 크기 조정된 VM에서 최대 30개의 CI/CD 작업을 실행할 수 있습니다.
- `concurrent`은 여러 `[[runners]]` 워커 전체의 전역 동시성 제한을 정의하는 반면 `limit`은 단일 `[[runners]]` 워커의 최대 동시성을 정의합니다.

이 예시에서 러너 프로세스는 다음을 처리합니다:

- 모든 `[[runners]]` 워커에서 최대 100개의 동시 작업입니다.
- `first` 워커의 경우 `shell` 실행기로 실행되는 40개 이하의 작업입니다.
- `second` 워커의 경우 `docker+machine` 실행기로 실행되는 30개 이하의 작업입니다. 또한 GitLab Runner는 `[runners.machine]`의 자동 크기 조정 구성을 기반으로 VM을 유지 관리하지만 모든 상태(유휴, 사용 중, 생성 중, 제거 중)에서 최대 30개의 VM입니다.
- `third` 워커의 경우 `ssh` 실행기로 실행되는 10개 이하의 작업입니다.
- `fourth` 워커의 경우 `virtualbox` 실행기로 실행되는 20개 이하의 작업입니다.

이 두 번째 예시에서 `[[runners]]` 워커 2개가 `docker+machine` 실행기를 사용하도록 구성되어 있습니다. 이 구성을 사용하면 각 러너 워커는 `limit` 매개변수의 값으로 제한되는 VM의 별도 풀을 관리합니다.

```toml
concurrent = 100

[[runners]]
name = "first"
executor = "docker+machine"
limit = 80
(...)

[[runners]]
name = "second"
executor = "docker+machine"
limit = 50
(...)

```

이 예시에서:

- 러너 프로세스는 최대 100개의 작업(`concurrent`의 값)을 처리합니다.
- 러너 프로세스는 2개의 `[[runners]]` 워커에서 작업을 실행하며, 각각 `docker+machine` 실행기를 사용합니다.
- `first` 러너는 최대 80개의 VM을 생성할 수 있습니다. 따라서 이 러너는 언제든지 최대 80개의 작업을 실행할 수 있습니다.
- `second` 러너는 최대 50개의 VM을 생성할 수 있습니다. 따라서 이 러너는 언제든지 최대 50개의 작업을 실행할 수 있습니다.

> [!note]
> 제한 값의 합계는 `130`(`80 + 50`)이지만 전역 `concurrent` 설정이 100이므로 러너 프로세스는 최대 100개의 작업을 동시에 실행합니다.

## 자동 크기 조정 알고리즘 및 매개변수 {#autoscaling-algorithm-and-parameters}

자동 크기 조정 알고리즘은 다음 매개변수를 기반으로 합니다:

- `IdleCount`
- `IdleCountMin`
- `IdleScaleFactor`
- `IdleTime`
- `MaxGrowthRate`
- `limit`

작업을 실행 중이지 않은 머신은 유휴 상태로 간주됩니다. GitLab Runner가 자동 크기 조정 모드에 있으면 모든 머신을 모니터링하고 항상 `IdleCount`의 유휴 머신이 있는지 확인합니다.

유휴 머신 수가 부족하면 GitLab Runner는 `MaxGrowthRate` 제한을 따르는 새 머신 프로비저닝을 시작합니다. `MaxGrowthRate` 값 이상의 머신 요청은 생성 중인 머신 수가 `MaxGrowthRate` 아래로 떨어질 때까지 보류됩니다.

동시에 GitLab Runner는 각 머신의 유휴 상태 지속 시간을 확인 중입니다. 시간이 `IdleTime` 값을 초과하면 머신이 자동으로 제거됩니다.

### 예제 구성 {#example-configuration}

다음 자동 크기 조정 매개변수로 구성된 GitLab Runner를 고려하세요:

```toml
[[runners]]
  limit = 10
  # (...)
  executor = "docker+machine"
  [runners.machine]
    MaxGrowthRate = 1
    IdleCount = 2
    IdleTime = 1800
    # (...)
```

처음에 작업이 대기 중이지 않으면 GitLab Runner는 2개의 머신(`IdleCount = 2`)을 시작하고 유휴 상태로 설정합니다. 또한 `IdleTime`은(는) 30분(`IdleTime = 1800`)으로 설정됩니다.

이제 GitLab CI/CD에서 5개의 작업이 대기 중이라고 가정하세요. 처음 2개의 작업은 우리가 가지고 있는 2개의 유휴 머신으로 전송됩니다. GitLab Runner는 유휴 수가 `IdleCount` 미만인 것을 알아차렸으므로 새 머신을 시작합니다(`0 < 2`). 이 머신들은 `MaxGrowthRate`을(를) 초과하지 않기 위해 순차적으로 프로비저닝됩니다.

나머지 3개의 작업은 준비된 첫 번째 머신에 할당됩니다. 최적화로, 이는 바쁜 상태였지만 이제 작업을 완료한 머신이거나 새로 프로비저닝된 머신일 수 있습니다. 이 예시에서는 프로비저닝이 빠르고 새 머신이 이전 작업이 완료되기 전에 준비된다고 가정합니다.

이제 1개의 유휴 머신이 있으므로 GitLab Runner는 `IdleCount`을 만족시키기 위해 하나의 새로운 머신을 시작합니다. 큐에 새 작업이 없으므로 이 2개의 머신은 유휴 상태로 유지되고 GitLab Runner는 만족합니다.

**What happened**:

이 예시에서 새 작업을 기다리는 2개의 머신이 유휴 상태입니다. 5개의 작업이 대기 중인 후 새 머신이 생성됩니다. 따라서 총 7개의 머신이 있습니다: 5개는 작업을 실행하고 2개는 다음 작업을 기다리는 유휴 상태입니다.

GitLab Runner는 `IdleCount`이 만족될 때까지 작업 실행에 사용된 각 머신에 대해 새로운 유휴 머신을 생성합니다. 머신은 `limit` 매개변수로 정의된 수까지 생성됩니다. GitLab Runner가 이 `limit`이 도달되었음을 감지하면 자동 크기 조정을 중지합니다. 새 작업은 머신이 유휴 상태로 돌아올 때까지 작업 큐에서 대기해야 합니다.

위의 예시에서는 항상 2개의 유휴 머신이 사용 가능합니다. `IdleTime` 매개변수는 수가 `IdleCount`을(를) 초과할 때만 적용됩니다. 이 지점에서 GitLab Runner는 머신 수를 `IdleCount`과 일치하도록 줄입니다.

**Scaling down**:

작업이 완료된 후 머신은 유휴 상태로 설정되고 새 작업 실행을 기다립니다. 큐에 새 작업이 나타나지 않으면 `IdleTime`로 지정된 시간 후에 유휴 머신이 제거됩니다. 이 예시에서는 모든 머신이 30분의 비활성 시간 후에 제거됩니다(각 머신의 마지막 작업 실행이 끝난 시간 기준). GitLab Runner는 `IdleCount`의 유휴 머신을 유지 관리하고 있으며, 이 예시의 처음과 같습니다.

자동 크기 조정 알고리즘은 다음과 같이 작동합니다:

1. GitLab Runner가 시작됩니다.
1. GitLab Runner는 2개의 유휴 머신을 생성합니다.
1. GitLab Runner는 1개의 작업을 선택합니다.
1. GitLab Runner는 2개의 유휴 머신을 유지하기 위해 하나의 추가 머신을 생성합니다.
1. 선택한 작업이 완료되어 3개의 유휴 머신이 결과입니다.
1. 3개의 유휴 머신 중 하나가 마지막 작업을 선택한 후의 시간부터 `IdleTime`을(를) 초과하면 제거됩니다.
1. GitLab Runner는 빠른 작업 처리를 위해 항상 최소 2개의 유휴 머신을 유지합니다.

다음 차트는 시간에 따른 머신 및 빌드(작업)의 상태를 보여줍니다:

![자동 크기 조정 상태 차트](img/autoscale-state-chart.png)

## `concurrent`, `limit` 및 `IdleCount`이(가) 실행 중인 머신의 상한선을 생성하는 방식 {#how-concurrent-limit-and-idlecount-generate-the-upper-limit-of-running-machines}

`limit` 또는 `concurrent`을(를) 설정할 수 있는지 알려주는 마법의 방정식은 없습니다. 필요에 따라 행동하세요. `IdleCount`의 유휴 머신을 보유하는 것은 속도 향상 기능입니다. 인스턴스가 생성될 때까지 10초/20초/30초를 기다릴 필요가 없습니다. 하지만 사용자는 모든 머신(비용을 지불해야 하는)이 작업을 실행하고 유휴 상태로 유지되지 않기를 원할 것입니다. 따라서 `concurrent` 및 `limit`을(를) 지불할 의향이 있는 최대 머신 수를 실행하는 값으로 설정해야 합니다. `IdleCount`의 경우 작업 큐가 비어 있을 때 _사용되지 않는_ 머신의 최소 수를 생성하는 값으로 설정해야 합니다.

다음 예시를 가정해 봅시다:

```toml
concurrent=20

[[runners]]
  limit = 40
  [runners.machine]
    IdleCount = 10
```

위의 시나리오에서 가질 수 있는 총 머신 수는 30입니다. 총 머신(빌드 및 유휴)의 `limit`은 40이 될 수 있습니다. 10개의 유휴 머신을 가질 수 있지만 `concurrent` 작업은 20개입니다. 따라서 총 20개의 동시 머신이 작업을 실행하고 10개가 유휴이며 총 30개입니다.

`limit`이(가) 생성될 수 있는 총 머신 수보다 작으면 어떻게 됩니까? 아래 예시에서 해당 경우를 설명합니다:

```toml
concurrent=20

[[runners]]
  limit = 25
  [runners.machine]
    IdleCount = 10
```

이 예시에서는 최대 20개의 동시 작업 및 25개의 머신을 가질 수 있습니다. 최악의 경우 10개의 유휴 머신이 아니라 5개만 가질 수 있습니다. `limit`이 25이기 때문입니다.

## `IdleScaleFactor` 전략 {#the-idlescalefactor-strategy}

`IdleCount` 매개변수는 러너가 유지해야 하는 유휴 머신의 정적 수를 정의합니다. 할당하는 값은 사용 사례에 따라 다릅니다.

유휴 상태에서 적절히 적은 수의 머신을 할당하여 시작합니다. 그런 다음 현재 사용량에 따라 더 큰 수로 자동 조정되도록 합니다. 그렇게 하려면 실험용 `IdleScaleFactor` 설정을 사용하세요.

> [!warning]
> `IdleScaleFactor`은(는) 내부적으로 `float64` 값이며 부동 소수점 형식을 사용해야 합니다(예: `0.0`, `1.0` 또는 `1.5`). 정수 형식이 사용되면(예: `IdleScaleFactor = 1`) 러너 프로세스가 오류로 실패합니다: `FATAL: Service run failed   error=toml: cannot load TOML value of type int64 into a Go float`.

이 설정을 사용하면 GitLab Runner는 유휴 상태에서 정의된 수의 머신을 유지하려고 시도합니다. 하지만 이 수는 더 이상 정적입니다. `IdleCount` 대신 GitLab Runner는 사용 중인 머신을 세고 원하는 유휴 용량을 그 수의 계수로 정의합니다.

사용 중인 머신이 없으면 `IdleScaleFactor`은 유지할 유휴 머신이 없는 것으로 평가됩니다. `IdleCount`이 `0`보다 크면(그리고 이때만 `IdleScaleFactor`이 적용 가능함) 러너는 처리할 수 있는 유휴 머신이 없으면 작업을 요청하지 않습니다. 새 작업이 없으면 사용 중인 머신 수가 증가하지 않으므로 `IdleScaleFactor`은(는) 지속적으로 `0`으로 평가됩니다. 이것은 러너를 사용 불가능한 상태로 차단할 수 있습니다.

따라서 우리는 두 번째 설정 `IdleCountMin`을(를) 도입했습니다. 이것은 `IdleScaleFactor`이(가) 평가되는 값에 관계없이 유지해야 하는 유휴 머신의 최소 수를 정의합니다. **설정은 `IdleScaleFactor`을(를) 사용하는 경우 1보다 작게 설정할 수 없습니다. GitLab Runner는 자동으로 `IdleCountMin`을(를) 1로 설정합니다**.

`IdleCountMin`을(를) 사용하여 항상 사용 가능해야 하는 유휴 머신의 최소 수를 정의할 수도 있습니다. 이를 통해 큐에 들어가는 새 작업을 빠르게 시작할 수 있습니다. `IdleCount`에서와 마찬가지로 할당하는 값은 사용 사례에 따라 다릅니다.

예를 들어:

```toml
concurrent=200

[[runners]]
  limit = 200
  [runners.machine]
    IdleCount = 100
    IdleCountMin = 10
    IdleScaleFactor = 1.1
```

이 경우 러너가 결정 지점에 도달하면 사용 중인 머신 수를 확인합니다. 예를 들어 5개의 유휴 머신과 10개의 사용 중인 머신이 있는 경우입니다. `IdleScaleFactor`에 곱하면 러너는 11개의 유휴 머신을 가져야 한다고 결정합니다. 따라서 6개가 더 생성됩니다.

90개의 유휴 머신과 100개의 사용 중인 머신이 있는 경우 `IdleScaleFactor`을(를) 기반으로 GitLab Runner는 `100 * 1.1 = 110`의 유휴 머신을 가져야 한다고 봅니다. 따라서 다시 새로운 것 생성을 시작합니다. 하지만 `100`개의 유휴 머신 수에 도달하면 더 이상 유휴 머신을 생성하지 않습니다. 이것이 `IdleCount`으로 정의된 상한선이기 때문입니다.

사용 중인 100개의 유휴 머신이 20으로 내려가면 원하는 유휴 머신 수는 `20 * 1.1 = 22`입니다. GitLab Runner는 머신 제거를 시작합니다. 위에서 설명한 대로 GitLab Runner는 `IdleTime`에 대해 사용되지 않는 머신을 제거합니다. 따라서 너무 많은 유휴 VM을 제거하는 것은 적극적으로 수행됩니다.

유휴 머신 수가 0으로 내려가면 원하는 유휴 머신 수는 `0 * 1.1 = 0`입니다. 하지만 이것은 정의된 `IdleCountMin` 설정보다 적으므로 러너는 10개의 VM이 남을 때까지 유휴 VM을 제거하기 시작합니다. 그 이후로 크기 조정 축소는 중지되고 러너는 10개의 머신을 유휴 상태로 유지합니다.

## 자동 크기 조정 기간 구성 {#configure-autoscaling-periods}

자동 크기 조정은 기간에 따라 다른 값을 갖도록 구성할 수 있습니다. 조직은 작업 스파이크가 실행되는 정기적인 시간과 작업이 거의 없거나 전혀 없는 다른 시간이 있을 수 있습니다. 예를 들어 대부분의 상용 회사는 월요일부터 금요일까지 오전 10시부터 오후 6시와 같은 정해진 시간에 근무합니다. 밤 시간과 주중 나머지 시간, 그리고 주말에는 파이프라인이 시작되지 않습니다.

이러한 기간은 `[[runners.machine.autoscaling]]` 섹션의 도움으로 구성할 수 있습니다. 각각은 `Periods` 세트를 기반으로 `IdleCount` 및 `IdleTime`을(를) 설정하는 것을 지원합니다.

### 자동 크기 조정 기간의 작동 방식 {#how-autoscaling-periods-work}

`[runners.machine]` 설정에서 `[[runners.machine.autoscaling]]` 섹션 여러 개를 추가할 수 있으며, 각각 자신의 `IdleCount`, `IdleTime`, `Periods` 및 `Timezone` 속성이 있습니다. 각 구성에 대해 가장 일반적인 시나리오에서 가장 구체적인 시나리오 순으로 진행하여 섹션을 정의해야 합니다.

모든 섹션이 파싱됩니다. 현재 시간과 일치하는 마지막 항목이 활성화됩니다. 일치하는 항목이 없으면 `[runners.machine]`의 루트 값이 사용됩니다.

예를 들어:

```toml
[runners.machine]
  MachineName = "auto-scale-%s"
  MachineDriver = "google"
  IdleCount = 10
  IdleTime = 1800
  [[runners.machine.autoscaling]]
    Periods = ["* * 9-17 * * mon-fri *"]
    IdleCount = 50
    IdleTime = 3600
    Timezone = "UTC"
  [[runners.machine.autoscaling]]
    Periods = ["* * * * * sat,sun *"]
    IdleCount = 5
    IdleTime = 60
    Timezone = "UTC"
```

이 구성에서 매주 평일 9시부터 16:59 UTC까지 머신은 업무 시간 동안 많은 트래픽을 처리하기 위해 과다 프로비저닝됩니다. 주말에 `IdleCount`은 트래픽 감소를 고려하여 5로 떨어집니다. 나머지 시간은 기본값의 값들이 사용됩니다 - `IdleCount = 10` 및 `IdleTime = 1800`.

> [!note]
> 지정하는 모든 기간의 마지막 분의 59초는 기간의 일부로 간주되지 않습니다. 자세한 내용은 [이슈 #2170](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/2170)을 참조하세요.

기간의 `Timezone`을(를) 지정할 수 있습니다(예: `"Australia/Sydney"`). 지정하지 않으면 모든 러너의 호스트 머신의 시스템 설정이 사용됩니다. 이 기본값을 `Timezone = "Local"`로 명시적으로 나타낼 수 있습니다.

`[[runner.machine.autoscaling]]` 섹션의 문법에 대한 자세한 정보는 [`[runners.machine]` 섹션 - GitLab Runner 고급 구성](advanced-configuration.md#the-runnersmachine-section)에서 찾을 수 있습니다.

## 분산 러너 캐싱 {#distributed-runners-caching}

> [!note]
> [분산 캐시를 사용하는 방법](speed_up_job_execution.md#use-a-distributed-cache)을 읽으세요.

작업을 빠르게 하기 위해 GitLab Runner는 선택된 디렉터리 및/또는 파일을 저장하고 후속 작업 간에 공유하는 [캐시 메커니즘](https://docs.gitlab.com/ci/yaml/#cache)을 제공합니다.

이 메커니즘은 작업이 동일한 호스트에서 실행될 때 잘 작동합니다. 하지만 GitLab Runner 자동 크기 조정 기능을 사용하기 시작하면 대부분의 작업이 새로운(또는 거의 새로운) 호스트에서 실행됩니다. 이 새 호스트는 새로운 Docker 컨테이너에서 각 작업을 실행합니다. 이 경우 캐시 기능을 활용할 수 없습니다.

이 문제를 극복하기 위해 자동 크기 조정 기능과 함께 분산 러너 캐시 기능이 도입되었습니다.

이 기능은 구성된 객체 저장소 서버를 사용하여 사용된 Docker 호스트 간에 캐시를 공유합니다. GitLab Runner는 서버를 쿼리하고 캐시를 복원하기 위해 아카이브를 다운로드하거나 캐시를 아카이브하기 위해 업로드합니다.

분산 캐싱을 활성화하려면 `config.toml`에서 [`[runners.cache]` 지시문](advanced-configuration.md#the-runnerscache-section)을(를) 사용하여 정의해야 합니다:

```toml
[[runners]]
  limit = 10
  executor = "docker+machine"
  [runners.cache]
    Type = "s3"
    Path = "path/to/prefix"
    Shared = false
    [runners.cache.s3]
      ServerAddress = "s3.example.com"
      AccessKey = "access-key"
      SecretKey = "secret-key"
      BucketName = "runner"
      Insecure = false
```

위의 예시에서 S3 URL은 `http(s)://<ServerAddress>/<BucketName>/<Path>/runner/<runner-id>/project/<id>/<cache-key>` 구조를 따릅니다.

두 개 이상의 러너 간에 캐시를 공유하려면 `Shared` 플래그를 true로 설정하세요. 이 플래그는 URL에서 러너 토큰을 제거합니다(`runner/<runner-id>`) 그리고 구성된 모든 러너가 동일한 캐시를 공유합니다. 캐시 공유가 활성화된 경우 `Path`을(를) 설정하여 러너 간 캐시를 분리할 수도 있습니다.

## 분산 컨테이너 레지스트리 미러링 {#distributed-container-registry-mirroring}

Docker 컨테이너 내에서 실행되는 작업을 빠르게 하려면 [Docker 레지스트리 미러링 서비스](https://docs.docker.com/retired/#registry-now-cncf-distribution)를 사용할 수 있습니다. 이 서비스는 Docker 머신과 사용되는 모든 레지스트리 간의 프록시를 제공합니다. 이미지는 레지스트리 미러로 한 번 다운로드됩니다. 새로운 호스트 또는 이미지를 사용할 수 없는 기존 호스트에서 이미지는 구성된 레지스트리 미러에서 다운로드됩니다.

미러가 Docker 머신 LAN에 있다면 각 호스트에서 이미지 다운로드 단계가 훨씬 빨라야 합니다.

Docker 레지스트리 미러링을 구성하려면 `config.toml`의 구성에 `MachineOptions`을(를) 추가해야 합니다:

```toml
[[runners]]
  limit = 10
  executor = "docker+machine"
  [runners.machine]
    (...)
    MachineOptions = [
      (...)
      "engine-registry-mirror=http://10.11.12.13:12345"
    ]
```

`10.11.12.13:12345`은(는) Docker 서비스의 연결을 수신 대기 중인 레지스트리 미러의 IP 주소 및 포트입니다. Docker Machine에서 생성한 각 호스트에서 접근 가능해야 합니다.

[컨테이너용 프록시 사용](speed_up_job_execution.md#use-a-proxy-for-containers)에 대해 자세히 알아보세요.

## `config.toml`의 완전한 예시 {#a-complete-example-of-configtoml}

아래 `config.toml`은(는) [`google` Docker Machine 드라이버](https://github.com/docker/docs/blob/173d3c65f8e7df2a8c0323594419c18086fc3a30/machine/drivers/gce.md)를 사용합니다:

```toml
concurrent = 50   # All registered runners can run up to 50 concurrent jobs

[[runners]]
  url = "https://gitlab.com"
  token = "RUNNER_TOKEN"             # Note this is different from the registration token used by `gitlab-runner register`
  name = "autoscale-runner"
  executor = "docker+machine"        # This runner is using the 'docker+machine' executor
  limit = 10                         # This runner can execute up to 10 jobs (created machines)
  [runners.docker]
    image = "ruby:3.3"               # The default image used for jobs is 'ruby:3.3'
  [runners.machine]
    IdleCount = 5                    # There must be 5 machines in Idle state - when Off Peak time mode is off
    IdleTime = 600                   # Each machine can be in Idle state up to 600 seconds (after this it will be removed) - when Off Peak time mode is off
    MaxBuilds = 100                  # Each machine can handle up to 100 jobs in a row (after this it will be removed)
    MachineName = "auto-scale-%s"    # Each machine will have a unique name ('%s' is required)
    MachineDriver = "google" # Refer to Docker Machine docs on how to authenticate: https://docs.docker.com/machine/drivers/gce/#credentials
    MachineOptions = [
      "google-project=GOOGLE-PROJECT-ID",
      "google-zone=GOOGLE-ZONE", # e.g. 'us-west1'
      "google-machine-type=GOOGLE-MACHINE-TYPE", # e.g. 'n1-standard-8'
      "google-machine-image=ubuntu-os-cloud/global/images/family/ubuntu-1804-lts",
      "google-username=root",
      "google-use-internal-ip",
      "engine-registry-mirror=https://mirror.gcr.io"
    ]
    [[runners.machine.autoscaling]]  # Define periods with different settings
      Periods = ["* * 9-17 * * mon-fri *"] # Every workday between 9 and 17 UTC
      IdleCount = 50
      IdleCountMin = 5
      IdleScaleFactor = 1.5 # Means that current number of Idle machines will be 1.5*in-use machines,
                            # no more than 50 (the value of IdleCount) and no less than 5 (the value of IdleCountMin)
      IdleTime = 3600
      Timezone = "UTC"
    [[runners.machine.autoscaling]]
      Periods = ["* * * * * sat,sun *"] # During the weekends
      IdleCount = 5
      IdleTime = 60
      Timezone = "UTC"
  [runners.cache]
    Type = "s3"
    [runners.cache.s3]
      ServerAddress = "s3.eu-west-1.amazonaws.com"
      AccessKey = "AMAZON_S3_ACCESS_KEY"
      SecretKey = "AMAZON_S3_SECRET_KEY"
      BucketName = "runner"
      Insecure = false
```

`MachineOptions` 매개변수에는 Docker Machine이 Google Compute Engine에서 머신을 생성하는 데 사용하는 `google` 드라이버의 옵션과 Docker Machine 자체(`engine-registry-mirror`)의 옵션이 포함되어 있습니다.
