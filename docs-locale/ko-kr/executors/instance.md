---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: 인스턴스 실행기
---

{{< history >}}

- GitLab Runner 15.11.0에서 [실험](https://docs.gitlab.com/policy/development_stages_support/#experiment)으로 도입되었습니다.
- [GitLab Runner 16.6에서](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29404) [베타](https://docs.gitlab.com/policy/development_stages_support/#beta) 버전으로 변경되었습니다.
- GitLab Runner 17.1에서 [일반 공급](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29221) 상태가 되었습니다.

{{< /history >}}

인스턴스 실행기는 자동 크기 조정 기능이 활성화된 실행기로, 러너 관리자가 처리하는 작업의 예상 볼륨을 수용하기 위해 필요에 따라 인스턴스를 생성합니다.

작업이 호스트 인스턴스, 운영 체제 및 연결된 장치에 대한 전체 액세스가 필요한 경우 인스턴스 실행기를 사용할 수 있습니다. 인스턴스 실행기는 다양한 수준의 격리 및 보안으로 단일 테넌트 및 다중 테넌트 작업을 수용하도록 구성할 수 있습니다.

## 중첩 가상화 {#nested-virtualization}

인스턴스 실행기는 GitLab에서 개발한 [nesting daemon](https://gitlab.com/gitlab-org/fleeting/nesting)을 통해 중첩 가상화를 지원합니다. nesting daemon은 작업과 같은 격리되고 단기적인 워크로드에 사용되는 호스트 시스템에서 미리 구성된 가상 머신의 생성 및 삭제를 가능하게 합니다. 중첩은 Apple Silicon 인스턴스에서만 지원됩니다.

## 자동 크기 조정을 위한 환경 준비 {#prepare-the-environment-for-autoscaling}

자동 크기 조정을 위한 환경을 준비하려면:

1. 러너 관리자가 설치되고 구성된 대상 플랫폼에 [fleeting 플러그인을 설치](../fleet_scaling/fleeting.md#install-a-fleeting-plugin)합니다.
1. 사용 중인 플랫폼에 대한 VM 이미지를 생성합니다. 이미지에는 다음이 포함되어야 합니다:
   - Git
   - GitLab Runner 바이너리

     > [!note]
     > 작업 아티팩트 및 캐시를 처리하려면 가상 머신에 GitLab Runner 바이너리를 설치하고 러너 실행 파일을 기본 경로에 유지합니다. VM 이미지는 GitLab Runner를 실행하기 위해 필요하지 않습니다. VM 이미지를 사용하여 시작된 인스턴스는 GitLab에서 자신을 러너로 등록해서는 안 됩니다.

   - 실행할 계획 중인 작업에 필요한 종속성

## 실행기를 자동 크기 조정하도록 구성 {#configure-the-executor-to-autoscale}

전제 조건:

- 관리자여야 합니다.

인스턴스 실행기를 자동 크기 조정하도록 구성하려면 `config.toml`의 다음 섹션을 업데이트합니다:

- [`[runners.autoscaler]`](../configuration/advanced-configuration.md#the-runnersautoscaler-section)
- [`[runners.instance]`](../configuration/advanced-configuration.md#the-runnersinstance-section)

## 선점 모드 {#preemptive-mode}

fleeting 및 taskscaler 사용:

- 설정되면 러너 관리자는 유휴 인스턴스를 사용할 수 있을 때까지 새 CI/CD 작업을 요청하지 않습니다. 이 모드에서는 CI/CD 작업이 거의 즉시 실행됩니다.
- 선점 모드가 비활성화된 경우, 러너 관리자는 유휴 인스턴스를 사용할 수 있는지 여부에 관계없이 새 CI/CD 작업을 요청합니다. 작업 수는 `max_instances` 및 `capacity_per_instance`을(를) 기반으로 합니다. 이 모드에서 CI/CD 작업의 시작 시간이 더 느립니다. 새 인스턴스를 프로비저닝하지 못할 수 있으며 따라서 CI/CD 작업이 실행되지 않을 수 있습니다.

## AWS 자동 크기 조정 그룹 구성 예제 {#aws-autoscaling-group-configuration-examples}

### 인스턴스당 1개 작업 {#one-job-per-instance}

전제 조건:

- `git` 및 GitLab Runner가 설치된 AMI.
- AWS 자동 크기 조정 그룹. 크기 조정 정책으로 `none`을(를) 사용합니다. 러너가 크기 조정을 처리합니다.
- [올바른 권한](https://gitlab.com/gitlab-org/fleeting/plugins/aws#recommended-iam-policy)이 있는 IAM 정책.

이 구성은 다음을 지원합니다:

- 각 인스턴스의 용량은 `1`입니다.
- 사용 횟수는 `1`입니다.
- 유휴 크기 조정은 `5`입니다.
- 유휴 시간은 20분입니다.
- 최대 인스턴스 개수는 `10`입니다.

용량 및 사용 횟수가 `1`으로 설정되면 각 작업에는 다른 작업의 영향을 받을 수 없는 안전한 임시 인스턴스가 제공됩니다. 작업이 완료되면 실행된 인스턴스가 즉시 삭제됩니다.

각 인스턴스의 용량이 `1`이고 유휴 크기 조정이 `5`일 때, 러너는 향후 요구를 충족하기 위해 5개의 전체 인스턴스를 사용 가능하게 유지합니다. 이러한 인스턴스는 최소 20분 동안 유지됩니다.

러너 `concurrent` 필드는 10(최대 인스턴스 수 * 인스턴스당 용량)으로 설정됩니다.

```toml
concurrent = 10

[[runners]]
  name = "instance autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  shell = "sh"

  executor = "instance"

  # Autoscaler config
  [runners.autoscaler]
    plugin = "aws" # in GitLab 16.11 and later, ensure you run `gitlab-runner fleeting install` to automatically install the plugin

    # in GitLab 16.10 and earlier, manually install the plugin and use:
    # plugin = "fleeting-plugin-aws"

    capacity_per_instance = 1
    max_use_count = 1
    max_instances = 10

    [runners.autoscaler.plugin_config] # plugin specific configuration (see plugin documentation)
      name             = "my-linux-asg"                # AWS Autoscaling Group name
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

### 인스턴스당 5개 작업, 무제한 사용 {#five-jobs-per-instance-with-unlimited-uses}

전제 조건:

- `git` 및 GitLab Runner가 설치된 AMI.
- 크기 조정 정책이 `none`로 설정된 AWS 자동 크기 조정 그룹. 러너가 크기 조정을 처리합니다.
- [올바른 권한](https://gitlab.com/gitlab-org/fleeting/plugins/aws#recommended-iam-policy)이 있는 IAM 정책.

이 구성은 다음을 지원합니다:

- 각 인스턴스의 용량은 `5`입니다.
- 무제한 사용 횟수.
- 유휴 크기 조정은 `5`입니다.
- 유휴 시간은 20분입니다.
- 최대 인스턴스 개수는 `10`입니다.

인스턴스당 용량을 `5`으로 설정하고 사용 횟수가 무제한일 때, 각 인스턴스는 인스턴스 수명 동안 5개의 작업을 동시에 실행합니다.

유휴 크기 조정이 `5`이고 인스턴스의 유휴 용량이 `5`일 때, 사용 중인 용량이 5 미만으로 떨어질 때마다 유휴 인스턴스 1개가 생성됩니다. 유휴 인스턴스는 최소 20분 동안 유지됩니다.

이러한 환경에서 실행되는 작업은 **trusted** 하며, 이들 사이의 격리가 거의 없고 각 작업이 다른 작업의 성능에 영향을 미칠 수 있습니다.

러너 `concurrent` 필드는 50(최대 인스턴스 수 * 인스턴스당 용량)으로 설정됩니다.

```toml
concurrent = 50

[[runners]]
  name = "instance autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  shell = "sh"

  executor = "instance"

  # Autoscaler config
  [runners.autoscaler]
    plugin = "aws" # in GitLab 16.11 and later, ensure you run `gitlab-runner fleeting install` to automatically install the plugin

    # in GitLab 16.10 and earlier, manually install the plugin and use:
    # plugin = "fleeting-plugin-aws"

    capacity_per_instance = 5
    max_use_count = 0
    max_instances = 10

    [runners.autoscaler.plugin_config] # plugin specific configuration (see plugin documentation)
      name             = "my-windows-asg"              # AWS Autoscaling Group name
      profile          = "default"                     # optional, default is 'default'
      config_file      = "/home/user/.aws/config"      # optional, default is '~/.aws/config'
      credentials_file = "/home/user/.aws/credentials" # optional, default is '~/.aws/credentials'

    [runners.autoscaler.connector_config]
      username          = "Administrator"
      timeout           = "5m0s"
      use_external_addr = true

    [[runners.autoscaler.policy]]
      idle_count = 5
      idle_time = "20m0s"
```

### 인스턴스당 2개 작업, 무제한 사용, EC2 Mac 인스턴스에서 중첩 가상화 {#two-jobs-per-instance-unlimited-uses-nested-virtualization-on-ec2-mac-instances}

전제 조건:

- [nesting](https://gitlab.com/gitlab-org/fleeting/nesting) 및 [Tart](https://github.com/cirruslabs/tart)가 설치된 Apple Silicon AMI.
- 러너가 사용하는 Tart VM 이미지. VM 이미지는 작업의 `image` 키워드로 지정됩니다. VM 이미지에는 최소한 `git` 및 GitLab Runner가 설치되어야 합니다.
- AWS 자동 크기 조정 그룹. 크기 조정 정책으로 `none`을(를) 사용합니다. 러너가 크기 조정을 처리하기 때문입니다. MacOS용 ASG를 설정하는 방법에 대한 자세한 내용은 [EC2 Mac 인스턴스에 대한 자동 크기 조정 구현](https://aws.amazon.com/blogs/compute/implementing-autoscaling-for-ec2-mac-instances/)을(를) 참조하세요.
- [올바른 권한](https://gitlab.com/gitlab-org/fleeting/plugins/aws#recommended-iam-policy)이 있는 IAM 정책.

이 구성은 다음을 지원합니다:

- 각 인스턴스의 용량은 `2`입니다.
- 무제한 사용 횟수.
- 격리된 작업을 지원하기 위한 중첩 가상화. 중첩 가상화는 [nesting](https://gitlab.com/gitlab-org/fleeting/nesting)이 설치된 Apple Silicon 인스턴스에서만 사용할 수 있습니다.
- 유휴 크기 조정은 `5`입니다.
- 유휴 시간은 20분입니다.
- 최대 인스턴스 개수는 `10`입니다.

각 인스턴스의 용량이 `2`이고 사용 횟수가 무제한일 때, 각 인스턴스는 인스턴스 수명 동안 2개의 작업을 동시에 실행합니다.

유휴 크기 조정이 `2`일 때, 사용 중인 용량이 `2` 미만으로 떨어질 때마다 유휴 인스턴스 1개가 생성됩니다. 유휴 인스턴스는 최소 24시간 동안 유지됩니다. 이 시간 범위는 AWS MacOS 인스턴스 호스트의 24시간 최소 할당 기간 때문입니다.

이 환경에서 실행되는 작업은 각 작업의 중첩 가상화에 [nesting](https://gitlab.com/gitlab-org/fleeting/nesting)을(를) 사용하기 때문에 신뢰할 수 있어야 하지 않습니다. 이는 Apple Silicon 인스턴스에서만 작동합니다.

러너 `concurrent` 필드는 8(최대 인스턴스 수 * 인스턴스당 용량)로 설정됩니다.

```toml
concurrent = 8

[[runners]]
  name = "macos applesilicon autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  executor = "instance"

  [runners.instance]
    allowed_images = ["*"] # allow any nesting image

  [runners.autoscaler]
    capacity_per_instance = 2 # AppleSilicon can only support 2 VMs per host
    max_use_count = 0
    max_instances = 4

    plugin = "aws" # in GitLab 16.11 and later, ensure you run `gitlab-runner fleeting install` to automatically install the plugin

    # in GitLab 16.10 and earlier, manually install the plugin and use:
    # plugin = "fleeting-plugin-aws"

    [[runners.autoscaler.policy]]
      idle_count = 2
      idle_time  = "24h" # AWS's MacOS instances

    [runners.autoscaler.connector_config]
      username = "ec2-user"
      key_path = "macos-key.pem"
      timeout  = "1h" # connecting to a MacOS instance can take some time, as they can be slow to provision

    [runners.autoscaler.plugin_config]
      name = "mac2metal"
      region = "us-west-2"

    [runners.autoscaler.vm_isolation]
      enabled = true
      nesting_host = "unix:///Users/ec2-user/Library/Application Support/nesting.sock"

    [runners.autoscaler.vm_isolation.connector_config]
      username = "nested-vm-username"
      password = "nested-vm-password"
      timeout  = "20m"
```

## Google Cloud 인스턴스 그룹 구성 예제 {#google-cloud-instance-group-configuration-examples}

### Google Cloud 인스턴스 그룹을 사용한 인스턴스당 1개 작업 {#one-job-per-instance-using-a-google-cloud-instance-group}

전제 조건:

- `git` 및 GitLab Runner가 설치된 사용자 지정 이미지.
- 자동 크기 조정 모드가 `do not autoscale`으로 설정된 Google Cloud 인스턴스 그룹. 러너가 크기 조정을 처리합니다.
- [올바른 권한](https://gitlab.com/gitlab-org/fleeting/plugins/googlecloud#required-permissions)이 있는 IAM 정책. GKE 클러스터에 러너를 배포하는 경우 Kubernetes 서비스 계정과 GCP 서비스 계정 간에 IAM 바인딩을 추가할 수 있습니다. `iam.workloadIdentityUser` 역할로 이 바인딩을 추가하여 `credentials_file` 파일을 사용하는 대신 GCP에 인증할 수 있습니다.

이 구성은 다음을 지원합니다:

- 인스턴스당 용량은 1입니다.
- 사용 횟수는 1입니다.
- 유휴 크기 조정은 5입니다.
- 유휴 시간은 20분입니다.
- 최대 인스턴스 개수는 10입니다.

용량 및 사용 횟수가 모두 `1`으로 설정되면 각 작업에는 다른 작업의 영향을 받을 수 없는 안전한 임시 인스턴스가 제공됩니다. 작업이 완료되면 실행된 인스턴스가 즉시 삭제됩니다.

유휴 크기 조정이 `5`으로 설정되면 러너는 향후 요구를 충족하기 위해 5개의 인스턴스를 사용 가능하게 유지합니다(인스턴스당 용량이 1이기 때문). 이러한 인스턴스는 최소 20분 동안 유지됩니다.

러너 `concurrent` 필드는 10(최대 인스턴스 수 * 인스턴스당 용량)으로 설정됩니다.

```toml
concurrent = 10

[[runners]]
  name = "instance autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  shell = "sh"

  executor = "instance"

  # Autoscaler config
  [runners.autoscaler]
    plugin = "googlecloud" # for >= 16.11, ensure you run `gitlab-runner fleeting install` to automatically install the plugin

    # for versions < 17.0, manually install the plugin and use:
    # plugin = "fleeting-plugin-googlecompute"

    capacity_per_instance = 1
    max_use_count = 1
    max_instances = 10

    [runners.autoscaler.plugin_config] # plugin specific configuration (see plugin documentation)
      name             = "my-linux-instance-group" # Google Cloud Instance Group name
      project          = "my-gcp-project"
      zone             = "europe-west1-c"
      credentials_file = "/home/user/.config/gcloud/application_default_credentials.json" # optional, default is '~/.config/gcloud/application_default_credentials.json'

    [runners.autoscaler.connector_config]
      username          = "runner"
      use_external_addr = true

    [[runners.autoscaler.policy]]
      idle_count = 5
      idle_time = "20m0s"
```

### 인스턴스당 5개 작업, 무제한 사용, Google Cloud 인스턴스 그룹 사용 {#five-jobs-per-instance-unlimited-uses-using-google-cloud-instance-group}

전제 조건:

- `git` 및 GitLab Runner가 설치된 사용자 지정 이미지.
- 인스턴스 그룹. "자동 크기 조정 모드"에 대해 "자동 크기 조정 안 함"을 선택합니다. 러너가 크기 조정을 처리하기 때문입니다.
- [올바른 권한](https://gitlab.com/gitlab-org/fleeting/plugins/googlecloud#required-permissions)이 있는 IAM 정책.

이 구성은 다음을 지원합니다:

- 인스턴스당 용량은 5입니다.
- 무제한 사용 횟수
- 유휴 크기 조정은 5입니다.
- 유휴 시간은 20분입니다.
- 최대 인스턴스 개수는 10입니다.

용량이 `5`으로 설정되고 사용 횟수가 무제한일 때, 각 인스턴스는 인스턴스 수명 동안 5개의 작업을 동시에 실행합니다.

이러한 환경에서 실행되는 작업은 **trusted** 하며, 이들 사이의 격리가 거의 없고 각 작업이 다른 작업의 성능에 영향을 미칠 수 있습니다.

유휴 크기 조정이 `5`일 때, 사용 중인 용량이 `5` 미만으로 떨어질 때마다 유휴 인스턴스 1개가 생성됩니다. 유휴 인스턴스는 최소 20분 동안 유지됩니다.

러너 `concurrent` 필드는 50(최대 인스턴스 수 * 인스턴스당 용량)으로 설정됩니다.

```toml
concurrent = 50

[[runners]]
  name = "instance autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  shell = "sh"

  executor = "instance"

  # Autoscaler config
  [runners.autoscaler]
    plugin = "googlecloud" # for >= 16.11, ensure you run `gitlab-runner fleeting install` to automatically install the plugin

    # for versions < 17.0, manually install the plugin and use:
    # plugin = "fleeting-plugin-googlecompute"

    capacity_per_instance = 5
    max_use_count = 0
    max_instances = 10

    [runners.autoscaler.plugin_config] # plugin specific configuration (see plugin documentation)
      name             = "my-windows-instance-group" # Google Cloud Instance Group name
      project          = "my-gcp-project"
      zone             = "europe-west1-c"
      credentials_file = "/home/user/.config/gcloud/application_default_credentials.json" # optional, default is '~/.config/gcloud/application_default_credentials.json'

    [runners.autoscaler.connector_config]
      username          = "Administrator"
      timeout           = "5m0s"
      use_external_addr = true

    [[runners.autoscaler.policy]]
      idle_count = 5
      idle_time = "20m0s"
```

## Azure 확장 집합 구성 예제 {#azure-scale-set-configuration-examples}

### Azure 확장 집합을 사용한 인스턴스당 1개 작업 {#one-job-per-instance-using-an-azure-scale-set}

전제 조건:

- `git` 및 GitLab Runner가 설치된 사용자 지정 이미지.
- 자동 크기 조정 모드가 `manual`으로 설정되고 과잉 프로비저닝이 비활성화된 Azure 확장 집합. 러너가 크기 조정을 처리합니다.

이 구성은 다음을 지원합니다:

- 인스턴스당 용량은 1입니다.
- 사용 횟수는 1입니다.
- 유휴 크기 조정은 5입니다.
- 유휴 시간은 20분입니다.
- 최대 인스턴스 개수는 10입니다.

용량 및 사용 횟수가 모두 `1`으로 설정되면 각 작업에는 다른 작업의 영향을 받을 수 없는 안전한 임시 인스턴스가 제공됩니다. 작업이 완료되면 실행된 인스턴스가 즉시 삭제됩니다.

유휴 크기 조정이 `5`으로 설정되면 러너는 향후 요구를 충족하기 위해 5개의 인스턴스를 사용 가능하게 유지합니다(인스턴스당 용량이 1이기 때문). 이러한 인스턴스는 최소 20분 동안 유지됩니다.

러너 `concurrent` 필드는 10(최대 인스턴스 수 * 인스턴스당 용량)으로 설정됩니다.

```toml
concurrent = 10

[[runners]]
  name = "instance autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  shell = "sh"

  executor = "instance"

  # Autoscaler config
  [runners.autoscaler]
    plugin = "azure" # for >= 16.11, ensure you run `gitlab-runner fleeting install` to automatically install the plugin

    # for versions < 17.0, manually install the plugin and use:
    # plugin = "fleeting-plugin-azure"

    capacity_per_instance = 1
    max_use_count = 1
    max_instances = 10

    [runners.autoscaler.plugin_config] # plugin specific configuration (see plugin documentation)
      name                = "my-linux-scale-set" # Azure scale set name
      subscription_id     = "9b3c4602-cde2-4089-bed8-889e5a3e7102"
      resource_group_name = "my-resource-group"

    [runners.autoscaler.connector_config]
      username               = "runner"
      password               = "my-scale-set-static-password"
      use_static_credentials = true
      timeout                = "10m"
      use_external_addr      = true

    [[runners.autoscaler.policy]]
      idle_count = 5
      idle_time  = "20m0s"
```

### 인스턴스당 5개 작업, 무제한 사용, Azure 확장 집합 사용 {#five-jobs-per-instance-unlimited-uses-using-an-azure-scale-set}

전제 조건:

- `git` 및 GitLab Runner가 설치된 사용자 지정 이미지.
- 자동 크기 조정 모드가 `manual`으로 설정되고 과잉 프로비저닝이 비활성화된 Azure 확장 집합. 러너가 크기 조정을 처리합니다.

이 구성은 다음을 지원합니다:

- 인스턴스당 용량은 5입니다.
- 무제한 사용 횟수
- 유휴 크기 조정은 5입니다.
- 유휴 시간은 20분입니다.
- 최대 인스턴스 개수는 10입니다.

용량이 `5`으로 설정되고 사용 횟수가 무제한일 때, 각 인스턴스는 인스턴스 수명 동안 5개의 작업을 동시에 실행합니다.

이러한 환경에서 실행되는 작업은 **trusted** 하며, 이들 사이의 격리가 거의 없고 각 작업이 다른 작업의 성능에 영향을 미칠 수 있습니다.

유휴 크기 조정이 `2`일 때, 사용 중인 용량이 `5` 미만으로 떨어질 때마다 유휴 인스턴스 1개가 생성됩니다. 유휴 인스턴스는 최소 20분 동안 유지됩니다.

러너 `concurrent` 필드는 50(최대 인스턴스 수 * 인스턴스당 용량)으로 설정됩니다.

```toml
concurrent = 50

[[runners]]
  name = "instance autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  shell = "sh"

  executor = "instance"

  # Autoscaler config
  [runners.autoscaler]
    plugin = "azure" # for >= 16.11, ensure you run `gitlab-runner fleeting install` to automatically install the plugin

    # for versions < 17.0, manually install the plugin and use:
    # plugin = "fleeting-plugin-azure"

    capacity_per_instance = 5
    max_use_count = 0
    max_instances = 10

    [runners.autoscaler.plugin_config] # plugin specific configuration (see plugin documentation)
      name                = "my-windows-scale-set" # Azure scale set name
      subscription_id     = "9b3c4602-cde2-4089-bed8-889e5a3e7102"
      resource_group_name = "my-resource-group"

    [runners.autoscaler.connector_config]
      username               = "Administrator"
      password               = "my-scale-set-static-password"
      use_static_credentials = true
      timeout                = "10m"
      use_external_addr      = true

    [[runners.autoscaler.policy]]
      idle_count = 5
      idle_time = "20m0s"
```

## 슬롯 기반 cgroup 지원 {#slot-based-cgroup-support}

인스턴스 실행기는 동시 작업 간의 향상된 리소스 격리를 위해 슬롯 기반 cgroup을 지원합니다. 활성화되면 `GITLAB_RUNNER_SLOT_CGROUP` 환경 변수가 자동으로 작업에 제공되므로 슬롯 특정 cgroup 아래에서 프로세스를 실행할 수 있습니다.

슬롯 기반 cgroup에 대한 자세한 정보(이점, 전제 조건, 구성, 설정 지침 포함)는 [슬롯 기반 cgroup 지원](../configuration/slot_based_cgroups.md)을(를) 참조하세요.

### GitLab Runner 슬롯 cgroup 환경 변수 사용 {#using-the-gitlab-runner-slot-cgroup-environment-variable}

인스턴스 실행기는 작업에 `GITLAB_RUNNER_SLOT_CGROUP` 환경 변수를 제공합니다. 이 변수를 `systemd-run` 또는 `cgexec`과(와) 같은 도구와 함께 사용하여 슬롯 특정 cgroup 아래에서 프로세스를 실행합니다.

사용 예제 및 문제 해결은 슬롯 기반 cgroup 설명서의 [인스턴스 실행기 섹션](../configuration/slot_based_cgroups.md#instance-executor)을(를) 참조하세요.

## 문제 해결 {#troubleshooting}

인스턴스 실행기로 작업할 때 다음 문제가 발생할 수 있습니다:

### `sh: 1: eval: Running on ip-x.x.x.x via runner-host...n: not found` {#sh-1-eval-running-on-ip-xxxx-via-runner-hostn-not-found}

이 오류는 일반적으로 준비 단계에서 `eval` 명령이 실패할 때 발생합니다. 이 오류를 해결하려면 `bash` 셸로 전환하고 [기능 플래그](../configuration/feature-flags.md) `FF_USE_NEW_BASH_EVAL_STRATEGY`을(를) 활성화합니다.
