---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Docker Autoscaler 실행기
---

{{< history >}}

- GitLab Runner 15.11.0에서 [실험](https://docs.gitlab.com/policy/development_stages_support/#experiment) 기능으로 도입되었습니다.
- [GitLab Runner 16.6에서](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29404) [베타](https://docs.gitlab.com/policy/development_stages_support/#beta) 버전으로 변경되었습니다.
- [GitLab Runner 17.1에서](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29221) 정식 릴리스되었습니다.

{{< /history >}}

Docker Autoscaler 실행기를 사용하기 전에, 알려진 이슈 목록을 확인하려면 GitLab Runner 자동 크기 조정에 대한 [피드백 이슈](https://gitlab.com/gitlab-org/gitlab/-/issues/408131)를 참고하세요.

Docker Autoscaler 실행기는 자동 크기 조정 기능이 활성화된 Docker 실행기로, 러너 관리자가 처리하는 작업을 수용하기 위해 필요에 따라 인스턴스를 생성합니다. [Docker 실행기](docker.md)를 래핑하므로 모든 Docker 실행기 옵션 및 기능이 지원됩니다.

Docker Autoscaler는 [fleeting 플러그인](https://gitlab.com/gitlab-org/fleeting/plugins)을 사용하여 자동 크기 조정을 수행합니다. Fleeting은 자동 크기 조정되는 인스턴스 그룹의 추상화로, Google Cloud, AWS, Azure와 같은 클라우드 공급자를 지원하는 플러그인을 사용합니다.

## Fleeting 플러그인 설치 {#install-a-fleeting-plugin}

대상 플랫폼에 플러그인을 설치하려면 [Fleeting 플러그인 설치](../fleet_scaling/fleeting.md#install-a-fleeting-plugin)를 참고하세요. 특정 구성 세부 사항은 [해당 플러그인 프로젝트 설명서](https://gitlab.com/gitlab-org/fleeting/plugins)를 참고하세요.

## Docker Autoscaler 구성 {#configure-docker-autoscaler}

Docker Autoscaler 실행기는 [Docker 실행기](docker.md)를 래핑하므로 모든 Docker 실행기 옵션 및 기능이 지원됩니다.

Docker Autoscaler를 구성하려면 `config.toml`에서 다음을 수행하세요:

- [`[runners]`](../configuration/advanced-configuration.md#the-runners-section) 섹션에서 `executor`을(를) `docker-autoscaler`로 지정합니다.
- 다음 섹션에서 요구 사항에 따라 Docker Autoscaler를 구성합니다:
  - [`[runners.docker]`](../configuration/advanced-configuration.md#the-runnersdocker-section)
  - [`[runners.autoscaler]`](../configuration/advanced-configuration.md#the-runnersautoscaler-section)

### 각 러너 구성에 대한 전용 자동 크기 조정 그룹 {#dedicated-autoscaling-groups-for-each-runner-configuration}

각 Docker Autoscaler 구성에는 자체 전용 자동 크기 조정 리소스가 있어야 합니다:

- AWS의 경우 전용 자동 크기 조정 그룹
- GCP의 경우 전용 인스턴스 그룹
- Azure의 경우 전용 스케일 세트

이러한 자동 크기 조정 리소스를 다음과 같이 공유하지 마세요:

- 여러 러너 관리자 (별도 GitLab 러너 설치)
- 같은 러너 관리자의 `config.toml` 내에서 여러 개의 `[[runners]]` 항목

Docker Autoscaler는 클라우드 공급자의 자동 크기 조정 리소스와 동기화되어야 하는 인스턴스 상태를 추적합니다. 여러 시스템이 동일한 자동 크기 조정 리소스를 관리하려고 하면 충돌하는 크기 조정 명령을 발생시킬 수 있으며, 이는 예측 불가능한 동작, 작업 실패, 잠재적으로 더 높은 비용을 초래할 수 있습니다.

### 예시:  인스턴스당 1개 작업에 대한 AWS 자동 크기 조정 {#example-aws-autoscaling-for-1-job-per-instance}

전제 조건:

- [Docker Engine](https://docs.docker.com/engine/)이(가) 설치된 AMI. 러너 관리자가 AMI의 Docker 소켓에 액세스할 수 있도록 하려면 사용자가 `docker` 그룹의 일부여야 합니다.

  > [!note]
  > AMI에는 GitLab 러너를 설치할 필요가 없습니다. AMI를 사용하여 시작한 인스턴스는 GitLab에서 자신을 러너로 등록해서는 안 됩니다.

- AWS 자동 크기 조정 그룹. 러너는 모든 크기 조정 동작을 직접 관리합니다. 크기 조정 정책의 경우 `none`을(를) 사용하고 인스턴스 축소 보호를 켭니다. 여러 가용 영역을 구성한 경우 `AZRebalance` 프로세스를 끕니다.
- [올바른 권한](https://gitlab.com/gitlab-org/fleeting/plugins/aws#recommended-iam-policy)이 있는 IAM 정책.

이 구성은 다음을 지원합니다:

- 인스턴스당 용량 1
- 사용 횟수 1
- 유휴 규모 5
- 유휴 시간 20분
- 최대 인스턴스 수 10

용량과 사용 횟수를 모두 1로 설정하면 각 작업에 다른 작업의 영향을 받을 수 없는 보안 임시 인스턴스가 제공됩니다. 작업이 완료되면 실행된 인스턴스는 즉시 삭제됩니다.

유휴 규모 5를 사용하면 러너는 5개의 전체 인스턴스 (인스턴스당 용량이 1이므로) 를 유지하려고 합니다 향후 수요에 사용 가능합니다. 이러한 인스턴스는 최소 20분 동안 유지됩니다.

러너 `concurrent` 필드는 10으로 설정됩니다 (최대 인스턴스 수 * 인스턴스당 용량).

```toml
concurrent = 10

[[runners]]
  name = "docker autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  shell = "sh"                                        # use powershell or pwsh for Windows AMIs

  # uncomment for Windows AMIs when the Runner manager is hosted on Linux
  # environment = ["FF_USE_POWERSHELL_PATH_RESOLVER=1"]

  executor = "docker-autoscaler"

  # Docker Executor config
  [runners.docker]
    image = "busybox:latest"

  # Autoscaler config
  [runners.autoscaler]
    plugin = "aws" # in GitLab 16.11 and later, ensure you run `gitlab-runner fleeting install` to automatically install the plugin

    # in GitLab 16.10 and earlier, manually install the plugin and use:
    # plugin = "fleeting-plugin-aws"

    capacity_per_instance = 1
    max_use_count = 1
    max_instances = 10

    [runners.autoscaler.plugin_config] # plugin specific configuration (see plugin documentation)
      name             = "my-docker-asg"               # AWS Autoscaling Group name
      profile          = "default"                     # optional, default is 'default'
      config_file      = "/home/user/.aws/config"      # optional, default is '~/.aws/config'
      credentials_file = "/home/user/.aws/credentials" # optional, default is '~/.aws/credentials'

    [runners.autoscaler.connector_config]
      username          = "ec2-user"
      use_external_addr = true

    [[runners.autoscaler.policy]]
      idle_count = 5
      idle_time = "20m0s"
```

### 예시:  인스턴스당 1개 작업에 대한 Google Cloud 인스턴스 그룹 {#example-google-cloud-instance-group-for-1-job-per-instance}

전제 조건:

- [Docker Engine](https://docs.docker.com/engine/) 이(가) 설치된 VM 이미지 (예: [`COS`](https://docs.cloud.google.com/container-optimized-os/docs)).

  > [!note]
  > VM 이미지에는 GitLab 러너를 설치할 필요가 없습니다. VM 이미지를 사용하여 시작한 인스턴스는 GitLab에서 자신을 러너로 등록해서는 안 됩니다.

- 단일 영역 Google Cloud 인스턴스 그룹. **Autoscaling mode**의 경우 **Do not autoscale**를 선택합니다. 러너는 자동 크기 조정을 처리합니다 (Google Cloud 인스턴스 그룹이 아님).

  > [!note]
  > 다중 영역 인스턴스 그룹은 현재 지원되지 않습니다. [이슈](https://gitlab.com/gitlab-org/fleeting/plugins/googlecloud/-/issues/20)는 향후 다중 영역 인스턴스 그룹을 지원하기 위해 존재합니다.

- [올바른 권한](https://gitlab.com/gitlab-org/fleeting/plugins/googlecloud#required-permissions)이 있는 IAM 정책. GKE 클러스터에서 러너를 배포하는 경우 Kubernetes 서비스 계정과 GCP 서비스 계정 사이에 IAM 바인딩을 추가할 수 있습니다. `iam.workloadIdentityUser` 역할로 이 바인딩을 추가하여 `credentials_file`이(가) 있는 키 파일을 사용하는 대신 GCP에 인증할 수 있습니다.

이 구성은 다음을 지원합니다:

- 인스턴스당 용량 1
- 사용 횟수 1
- 유휴 규모 5
- 유휴 시간 20분
- 최대 인스턴스 수 10

용량과 사용 횟수를 모두 1로 설정하면 각 작업에 다른 작업의 영향을 받을 수 없는 보안 임시 인스턴스가 제공됩니다. 작업이 완료되면 실행된 인스턴스는 즉시 삭제됩니다.

유휴 규모 5를 사용하면 러너는 5개의 전체 인스턴스 (인스턴스당 용량이 1이므로) 를 유지하려고 합니다 향후 수요에 사용 가능합니다. 이러한 인스턴스는 최소 20분 동안 유지됩니다.

러너 `concurrent` 필드는 10으로 설정됩니다 (최대 인스턴스 수 * 인스턴스당 용량).

```toml
concurrent = 10

[[runners]]
  name = "docker autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  shell = "sh"                                        # use powershell or pwsh for Windows Images

  # uncomment for Windows Images when the Runner manager is hosted on Linux
  # environment = ["FF_USE_POWERSHELL_PATH_RESOLVER=1"]

  executor = "docker-autoscaler"

  # Docker Executor config
  [runners.docker]
    image = "busybox:latest"

  # Autoscaler config
  [runners.autoscaler]
    plugin = "googlecloud" # for >= 16.11, ensure you run `gitlab-runner fleeting install` to automatically install the plugin

    # for versions < 17.0, manually install the plugin and use:
    # plugin = "fleeting-plugin-googlecompute"

    capacity_per_instance = 1
    max_use_count = 1
    max_instances = 10

    [runners.autoscaler.plugin_config] # plugin specific configuration (see plugin documentation)
      name             = "my-docker-instance-group" # Google Cloud Instance Group name
      project          = "my-gcp-project"
      zone             = "europe-west1"
      credentials_file = "/home/user/.config/gcloud/application_default_credentials.json" # optional, default is '~/.config/gcloud/application_default_credentials.json'

    [runners.autoscaler.connector_config]
      username          = "runner"
      use_external_addr = true

    [[runners.autoscaler.policy]]
      idle_count = 5
      idle_time = "20m0s"
```

### 예시:  인스턴스당 1개 작업에 대한 Azure 스케일 세트 {#example-azure-scale-set-for-1-job-per-instance}

전제 조건:

- [Docker Engine](https://docs.docker.com/engine/)이(가) 설치된 Azure VM 이미지.

  > [!note]
  > VM 이미지에는 GitLab 러너를 설치할 필요가 없습니다. VM 이미지를 사용하여 시작한 인스턴스는 GitLab에서 자신을 러너로 등록해서는 안 됩니다.

- 자동 크기 조정 정책이 `manual`로 설정된 Azure 스케일 세트. 러너가 크기 조정을 처리합니다.

이 구성은 다음을 지원합니다:

- 인스턴스당 용량 1
- 사용 횟수 1
- 유휴 규모 5
- 유휴 시간 20분
- 최대 인스턴스 수 10

용량과 사용 횟수가 모두 `1`로 설정되면 각 작업에 다른 작업의 영향을 받을 수 없는 보안 임시 인스턴스가 제공됩니다. 작업이 완료되면 실행된 인스턴스는 즉시 삭제됩니다.

유휴 규모가 `5`로 설정되면 러너는 향후 수요에 사용 가능한 5개 인스턴스를 유지합니다 (인스턴스당 용량이 1이므로). 이러한 인스턴스는 최소 20분 동안 유지됩니다.

러너 `concurrent` 필드는 10으로 설정됩니다 (최대 인스턴스 수 * 인스턴스당 용량).

```toml
concurrent = 10

[[runners]]
  name = "docker autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  shell = "sh"                                        # use powershell or pwsh for Windows AMIs

  # uncomment for Windows AMIs when the Runner manager is hosted on Linux
  # environment = ["FF_USE_POWERSHELL_PATH_RESOLVER=1"]

  executor = "docker-autoscaler"

  # Docker Executor config
  [runners.docker]
    image = "busybox:latest"

  # Autoscaler config
  [runners.autoscaler]
    plugin = "azure" # for >= 16.11, ensure you run `gitlab-runner fleeting install` to automatically install the plugin

    # for versions < 17.0, manually install the plugin and use:
    # plugin = "fleeting-plugin-azure"

    capacity_per_instance = 1
    max_use_count = 1
    max_instances = 10

    [runners.autoscaler.plugin_config] # plugin specific configuration (see plugin documentation)
      name = "my-docker-scale-set"
      subscription_id = "9b3c4602-cde2-4089-bed8-889e5a3e7102"
      resource_group_name = "my-resource-group"

    [runners.autoscaler.connector_config]
      username = "azureuser"
      password = "my-scale-set-static-password"
      use_static_credentials = true
      timeout = "10m"
      use_external_addr = true

    [[runners.autoscaler.policy]]
      idle_count = 5
      idle_time = "20m0s"
```

## 슬롯 기반 cgroup 지원 {#slot-based-cgroup-support}

Docker Autoscaler 실행기는 동시 작업 간의 리소스 격리를 개선하기 위해 슬롯 기반 cgroup을 지원합니다. Cgroup 경로는 `--cgroup-parent` 플래그를 사용하여 Docker 컨테이너에 자동으로 적용됩니다.

슬롯 기반 cgroup에 대한 자세한 정보 (이점, 전제 조건 및 설정 지침 포함) 는 [슬롯 기반 cgroup 지원](../configuration/slot_based_cgroups.md)을 참고하세요.

### Docker 특정 구성 {#docker-specific-configuration}

표준 슬롯 cgroup 구성 외에도 서비스 컨테이너에 대해 별도의 cgroup 템플릿을 지정할 수 있습니다:

```toml
[[runners]]
  executor = "docker+autoscaler"
  use_slot_cgroups = true
  slot_cgroup_template = "gitlab-runner/slot-${slot}"

  [runners.docker]
    service_slot_cgroup_template = "gitlab-runner/service-slot-${slot}"
```

사용 가능한 모든 옵션은 [슬롯 기반 cgroup 구성 설명서](../configuration/slot_based_cgroups.md#docker-specific-configuration)를 참고하세요.

## 문제 해결 {#troubleshooting}

### `ERROR: error during connect: ssh tunnel: EOF ()` {#error-error-during-connect-ssh-tunnel-eof-}

외부 원본 (예: 자동 크기 조정 그룹 또는 자동화된 스크립트) 에서 인스턴스가 제거되면 작업이 다음 오류로 실패합니다:

```plaintext
ERROR: Job failed (system failure): error during connect: Post "http://internal.tunnel.invalid/v1.43/containers/xyz/wait?condition=not-running": ssh tunnel: EOF ()
```

GitLab 러너 로그는 작업에 할당된 인스턴스 ID에 대한 `instance unexpectedly removed` 오류를 표시합니다:

```plaintext
ERROR: instance unexpectedly removed    instance=<instance_id> max-use-count=9999 runner=XYZ slots=map[] subsystem=taskscaler used=45
```

이 오류를 해결하려면 클라우드 공급자 플랫폼에서 인스턴스와 관련된 이벤트를 확인합니다. 예를 들어 AWS에서 이벤트 원본 `ec2.amazonaws.com`에 대한 CloudTrail 이벤트 기록을 확인합니다.

### `ERROR: Preparation failed: unable to acquire instance: context deadline exceeded` {#error-preparation-failed-unable-to-acquire-instance-context-deadline-exceeded}

[AWS fleeting 플러그인](https://gitlab.com/gitlab-org/fleeting/plugins/aws)을(를) 사용할 때 작업이 다음 오류로 간헐적으로 실패할 수 있습니다:

```plaintext
ERROR: Preparation failed: unable to acquire instance: context deadline exceeded
```

AWS CloudWatch 로그에 `reserved` 인스턴스 수가 위아래로 진동하기 때문에 자주 나타납니다:

```plaintext
"2024-07-23T18:10:24Z","instance_count:1,max_instance_count:1000,acquired:0,unavailable_capacity:0,pending:0,reserved:0,idle_count:0,scale_factor:0,scale_factor_limit:0,capacity_per_instance:1","required scaling change",
"2024-07-23T18:10:25Z","instance_count:1,max_instance_count:1000,acquired:0,unavailable_capacity:0,pending:0,reserved:1,idle_count:0,scale_factor:0,scale_factor_limit:0,capacity_per_instance:1","required scaling change",
"2024-07-23T18:11:15Z","instance_count:1,max_instance_count:1000,acquired:0,unavailable_capacity:0,pending:0,reserved:0,idle_count:0,scale_factor:0,scale_factor_limit:0,capacity_per_instance:1","required scaling change",
"2024-07-23T18:11:16Z","instance_count:1,max_instance_count:1000,acquired:0,unavailable_capacity:0,pending:0,reserved:1,idle_count:0,scale_factor:0,scale_factor_limit:0,capacity_per_instance:1","required scaling change",
```

이 오류를 해결하려면 AWS의 자동 크기 조정 그룹에 대해 `AZRebalance` 프로세스가 비활성화되어 있는지 확인합니다.

### `Job failures when scaling from zero instances on Azure VMSS` {#job-failures-when-scaling-from-zero-instances-on-azure-vmss}

Microsoft Azure Virtual Machine Scale Set에는 [과도한 프로비저닝 기능](https://learn.microsoft.com/en-us/azure/virtual-machine-scale-sets/virtual-machine-scale-sets-design-overview#overprovisioning)이 있어 작업 실패를 유발할 수 있습니다. Azure가 확대할 때 추가 VM을 생성하여 용량을 확보한 다음 요청된 용량을 충족한 후 이를 종료합니다. 이 동작은 GitLab 러너의 인스턴스 추적과 충돌하여 자동 스케일러가 Azure가 종료하려는 인스턴스에 작업을 할당하도록 합니다.

`overprovision`을(를) VMSS 구성에서 `false`로 설정하여 과도한 프로비저닝을 비활성화합니다.
