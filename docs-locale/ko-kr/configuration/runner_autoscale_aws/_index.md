---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: AWS EC2에서 러너 Docker Machine 자동 크기 조정 구성
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

GitLab 러너의 가장 큰 장점 중 하나는 VM을 자동으로 시작하고 중지하여 빌드가 즉시 처리되도록 할 수 있다는 것입니다. 이것은 훌륭한 기능이며, 올바르게 사용하면 24/7 러너를 사용하지 않고 비용 효율적이고 확장 가능한 솔루션을 원하는 상황에서 매우 유용할 수 있습니다.

## 소개 {#introduction}

이 자습서에서는 AWS에서 GitLab 러너를 올바르게 구성하는 방법을 살펴봅니다. AWS의 인스턴스는 온디맨드로 새 Docker 인스턴스를 생성하는 러너 관리자 역할을 합니다. 이 인스턴스의 러너는 자동으로 생성됩니다. 이들은 이 가이드에서 다루는 매개변수를 사용하며 생성 후 수동 구성이 필요하지 않습니다.

또한 [Amazon의 EC2 Spot 인스턴스](https://aws.amazon.com/ec2/spot/)를 활용하여 GitLab 러너 인스턴스의 비용을 크게 줄이면서도 매우 강력한 자동 크기 조정 머신을 사용할 수 있습니다.

## 필수 요구 사항 {#prerequisites}

Amazon Web Services(AWS)에 대한 기본 이해가 필요합니다. 대부분의 구성이 여기서 진행되기 때문입니다.

Docker 머신 [`amazonec2` 드라이버 설명서](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/aws.md)를 빠르게 읽어보아 이 문서의 뒷부분에서 설정할 매개변수에 대해 숙지하시기를 권장합니다.

귀사의 GitLab 러너는 네트워크를 통해 GitLab 인스턴스와 통신해야 하며, 이는 AWS 보안 그룹을 구성하거나 DNS 구성을 설정할 때 고려해야 할 사항입니다.

예를 들어, EC2 리소스를 다른 VPC의 공용 트래픽에서 분리하여 네트워크 보안을 강화할 수 있습니다. 사용자의 환경은 다를 가능성이 높으므로 상황에 가장 적합한 방법을 고려하세요.

### AWS 보안 그룹 {#aws-security-groups}

Docker Machine은 [기본 보안 그룹](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/aws.md/#security-group)을 사용하려고 시도하며, 포트 `2376`과 SSH `22`에 대한 규칙이 있어 Docker 데몬과의 통신이 필요합니다. Docker에 의존하는 대신 필요한 규칙이 있는 보안 그룹을 생성하고 GitLab 러너 옵션에 제공할 수 있습니다. 아래에서 [보기](#the-runnersmachine-section)와 같이 진행하겠습니다. 이렇게 하면 네트워킹 환경에 따라 미리 원하는 대로 사용자 정의할 수 있습니다. 포트 `2376`과 `22`이 [러너 관리자 인스턴스](#prepare-the-runner-manager-instance)에서 액세스할 수 있는지 확인해야 합니다.

### AWS 자격 증명 {#aws-credentials}

EC2를 확장하고 캐시를 업데이트(S3를 통해)할 수 있는 권한이 있는 사용자에게 연결된 [AWS 액세스 키](https://docs.aws.amazon.com/IAM/latest/UserGuide/security-creds.html)가 필요합니다. [정책](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-policies-for-amazon-ec2.html)을 사용하여 EC2(AmazonEC2FullAccess) 및 S3에 대한 새 사용자를 생성합니다. S3에 필요한 최소 권한에 대한 자세한 내용은 [`runners.cache.s3`](../advanced-configuration.md#the-runnerscaches3-section)를 참조하세요. 더 안전하게 하려면 해당 사용자의 콘솔 로그인을 비활성화할 수 있습니다. 탭을 열린 상태로 두거나 보안 자격 증명을 편집기에 복사하여 나중에 [GitLab 러너 구성](#the-runnersmachine-section) 중에 사용합니다.

[EC2 인스턴스 프로필](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2_instance-profiles.html)을 필수 `AmazonEC2FullAccess` 및 `AmazonS3FullAccess` 정책과 함께 생성할 수도 있습니다.

작업을 실행하기 위해 새 EC2 인스턴스를 프로비저닝하려면 이 인스턴스 프로필을 러너 관리자 EC2 인스턴스에 연결합니다. 러너 머신이 인스턴스 프로필을 사용하는 경우 러너 관리자의 인스턴스 프로필에 `iam:PassRole` 작업을 포함합니다.

예시:

```json
{
    "Statement": [
        {
            "Action": "iam:PassRole",
            "Effect": "Allow",
            "Resource": "arn:aws:iam:::role/instance-profile-of-runner-machine"
        }
    ],
    "Version": "2012-10-17"
}
```

## 러너 관리자 인스턴스 준비 {#prepare-the-runner-manager-instance}

첫 번째 단계는 새 머신을 생성하는 러너 관리자 역할을 하는 EC2 인스턴스에 GitLab 러너를 설치하는 것입니다. Docker와 GitLab 러너를 모두 지원하는 배포판(Ubuntu, Debian, CentOS 또는 RHEL 등)을 선택합니다.

러너 관리자 인스턴스는 작업을 자체적으로 실행하지 않기 때문에 강력한 머신일 필요가 없습니다. 초기 구성을 위해 더 작은 인스턴스로 시작할 수 있습니다. 이 머신은 항상 실행 중이어야 하기 때문에 전용 호스트입니다. 따라서 지속적인 기본 비용이 있는 유일한 호스트입니다.

필수 조건을 설치합니다:

1. 서버에 로그인합니다
1. [공식 GitLab 리포지토리에서 GitLab 러너 설치](../../install/linux-repository.md)
1. [Docker 설치](https://docs.docker.com/engine/install/#server)
1. [GitLab fork에서 Docker Machine 설치](https://gitlab.com/gitlab-org/ci-cd/docker-machine)(Docker는 Docker Machine을 더 이상 사용하지 않습니다)

러너가 설치되었으므로 이제 등록할 차례입니다.

## GitLab 러너 등록 {#registering-the-gitlab-runner}

GitLab 러너를 구성하기 전에 먼저 등록하여 GitLab 인스턴스와 연결해야 합니다:

1. [러너 토큰 획득](https://docs.gitlab.com/ci/runners/)
1. [러너 등록](../../register/_index.md)
1. 실행기 유형을 묻는 메시지가 표시되면 `docker+machine`을 입력합니다

이제 가장 중요한 부분인 GitLab 러너 구성으로 넘어갈 수 있습니다.

> [!note]
> 인스턴스의 모든 사용자가 자동 크기 조정된 러너를 사용할 수 있도록 하려면 러너를 공유 러너로 등록합니다.

## 러너 구성 {#configuring-the-runner}

러너가 등록되었으므로 구성 파일을 편집하고 AWS 머신 드라이버에 필요한 옵션을 추가해야 합니다.

먼저 부분별로 분석해 보겠습니다.

### 전역 섹션 {#the-global-section}

전역 섹션에서 모든 러너에 걸쳐 동시에 실행할 수 있는 작업의 제한을 정의할 수 있습니다(`concurrent`). 이는 GitLab Runner가 수용할 수 있는 사용자 수, 빌드에 소요되는 시간 등과 같은 요구사항에 크게 달려 있습니다. `10`과 같은 낮은 값으로 시작할 수 있으며, 향후 그 값을 증가 또는 감소시킬 수 있습니다.

`check_interval` 옵션은 러너가 새 작업에 대해 GitLab를 확인하는 빈도를 정의합니다(초 단위).

예시:

```toml
concurrent = 10
check_interval = 0
```

[기타 옵션](../advanced-configuration.md#the-global-section)도 사용할 수 있습니다.

### `runners` 섹션 {#the-runners-section}

`[[runners]]` 섹션에서 가장 중요한 부분은 `executor`이며 `docker+machine`으로 설정해야 합니다. 이 설정의 대부분은 처음 러너를 등록할 때 처리됩니다.

`limit`은 이 러너가 생성할 머신의 최대 개수(실행 중 및 유휴)를 설정합니다. 자세한 내용은 [`limit`, `concurrent` 및 `IdleCount` 간의 관계](../autoscale.md#how-concurrent-limit-and-idlecount-generate-the-upper-limit-of-running-machines)를 확인하세요.

예시:

```toml
[[runners]]
  name = "gitlab-aws-autoscaler"
  url = "<URL of your GitLab instance>"
  token = "<Runner's token>"
  executor = "docker+machine"
  limit = 20
```

`[[runners]]` 아래의 [기타 옵션](../advanced-configuration.md#the-runners-section)도 사용할 수 있습니다.

### `runners.docker` 섹션 {#the-runnersdocker-section}

`[runners.docker]` 섹션에서 [`.gitlab-ci.yml`](https://docs.gitlab.com/ci/yaml/)에 정의되지 않은 경우 자식 러너에서 사용할 기본 Docker 이미지를 정의할 수 있습니다. `privileged = true`를 사용하면 모든 러너는 GitLab CI/CD를 통해 자신의 Docker 이미지를 빌드하려는 경우 유용한 [Docker in Docker](https://docs.gitlab.com/ci/docker/using_docker_build/#use-docker-in-docker)를 실행할 수 있습니다.

다음으로 `disable_cache = true`을 사용하여 Docker 실행기의 내부 캐시 메커니즘을 비활성화합니다. 다음 섹션에서 설명하는 분산 캐시 모드를 사용할 것이기 때문입니다.

예시:

```toml
  [runners.docker]
    image = "alpine"
    privileged = true
    disable_cache = true
```

`[runners.docker]` 아래의 [기타 옵션](../advanced-configuration.md#the-runnersdocker-section)도 사용할 수 있습니다.

### `runners.cache` 섹션 {#the-runnerscache-section}

GitLab Runner 속도를 높이기 위해 작업 속도를 높이기 위해 캐시 메커니즘을 제공합니다. 여기서 선택한 디렉터리 및/또는 파일이 저장되고 후속 작업들 간에 공유됩니다. 이 설정에 필수는 아니지만 GitLab 러너에서 제공하는 분산 캐시 메커니즘을 사용하는 것이 좋습니다. 온디맨드로 새 인스턴스가 생성되므로 캐시가 저장되는 공통 위치를 갖는 것이 필수적입니다.

다음 예에서는 Amazon S3를 사용합니다:

```toml
  [runners.cache]
    Type = "s3"
    Shared = true
    [runners.cache.s3]
      ServerAddress = "s3.amazonaws.com"
      AccessKey = "<your AWS Access Key ID>"
      SecretKey = "<your AWS Secret Access Key>"
      BucketName = "<the bucket where your cache should be kept>"
      BucketLocation = "us-west-2"
```

캐시 메커니즘을 더 자세히 탐색할 수 있는 추가 정보입니다:

- [`runners.cache`에 대한 참고](../advanced-configuration.md#the-runnerscache-section)
- [`runners.cache.s3`에 대한 참고](../advanced-configuration.md#the-runnerscaches3-section)
- [GitLab 러너를 위한 캐시 서버 배포 및 사용](../autoscale.md#distributed-runners-caching)
- [캐시 작동 방식](https://docs.gitlab.com/ci/yaml/#cache)

### `runners.machine` 섹션 {#the-runnersmachine-section}

이것이 구성에서 가장 중요한 부분이며 GitLab 러너에 새 Docker Machine 인스턴스를 생성하거나 이전 인스턴스를 제거하는 방법과 시기를 알려줍니다.

AWS 머신 옵션에 중점을 둘 것이고 나머지 설정에 대해서는 다음을 읽어보세요:

- [자동 크기 조정 알고리즘 및 기반 매개변수](../autoscale.md#autoscaling-algorithm-and-parameters) \- 조직의 요구사항에 따라 달라짐
- [자동 크기 조정 기간](../autoscale.md#configure-autoscaling-periods) \- 주말 등 조직에서 작업이 수행되지 않는 정기적인 기간이 있을 때 유용함

`runners.machine` 섹션의 예시입니다:

```toml
  [runners.machine]
    IdleCount = 1
    IdleTime = 1800
    MaxBuilds = 10
    MachineDriver = "amazonec2"
    MachineName = "gitlab-docker-machine-%s"
    MachineOptions = [
      "amazonec2-access-key=XXXX",
      "amazonec2-secret-key=XXXX",
      "amazonec2-region=eu-central-1",
      "amazonec2-vpc-id=vpc-xxxxx",
      "amazonec2-subnet-id=subnet-xxxxx",
      "amazonec2-zone=x",
      "amazonec2-use-private-address=true",
      "amazonec2-tags=runner-manager-name,gitlab-aws-autoscaler,gitlab,true,gitlab-runner-autoscale,true",
      "amazonec2-security-group=xxxxx",
      "amazonec2-instance-type=m4.2xlarge",
    ]
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

Docker Machine 드라이버는 `amazonec2`로 설정되고 머신 이름은 표준 접두사 다음에 `%s`(필수)가 붙어 있으며, 이는 자식 러너의 ID로 대체됩니다: `gitlab-docker-machine-%s`.

이제 AWS 인프라에 따라 `MachineOptions` 아래에서 설정할 수 있는 많은 옵션이 있습니다. 아래에서 가장 일반적인 것들을 볼 수 있습니다.

| 머신 옵션                                                         | 설명 |
|------------------------------------------------------------------------|-------------|
| `amazonec2-access-key=XXXX`                                            | EC2 인스턴스를 생성할 수 있는 권한이 있는 사용자의 AWS 액세스 키입니다. [AWS 자격 증명](#aws-credentials)을 참조하세요. |
| `amazonec2-secret-key=XXXX`                                            | EC2 인스턴스를 생성할 수 있는 권한이 있는 사용자의 AWS 비밀 키입니다. [AWS 자격 증명](#aws-credentials)을 참조하세요. |
| `amazonec2-region=eu-central-2`                                        | 인스턴스를 시작할 때 사용할 영역입니다. 이를 완전히 생략할 수 있으며 기본값 `us-east-1`이 사용됩니다. |
| `amazonec2-vpc-id=vpc-xxxxx`                                           | 인스턴스를 시작할 [VPC ID](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/aws.md#vpc-id)입니다. |
| `amazonec2-subnet-id=subnet-xxxx`                                      | AWS VPC 서브넷 ID입니다. |
| `amazonec2-zone=x`                                                     | 지정하지 않으면 [가용 영역은 `a`](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/aws.md#environment-variables-and-default-values)이며, 지정된 서브넷과 동일한 가용 영역으로 설정해야 합니다. 예를 들어 영역이 `eu-west-1b`인 경우 `amazonec2-zone=b`이어야 합니다 |
| `amazonec2-use-private-address=true`                                   | Docker 머신의 개인 IP 주소를 사용하지만 공용 IP 주소도 생성합니다. 트래픽을 내부적으로 유지하고 추가 비용을 피하는 데 유용합니다. |
| `amazonec2-tags=runner-manager-name,gitlab-aws-autoscaler,gitlab,true,gitlab-runner-autoscale,true` | AWS 추가 태그 키-값 쌍으로 AWS 콘솔에서 인스턴스를 식별하는 데 유용합니다. "Name" [태그](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/Using_Tags.html)는 기본적으로 머신 이름으로 설정됩니다. "runner-manager-name"을 `[[runners]]`에 설정된 러너 이름과 일치하도록 설정하여 특정 관리자 설정으로 생성된 모든 EC2 인스턴스를 필터링할 수 있습니다. |
| `amazonec2-security-group=xxxx`                                        | AWS VPC 보안 그룹 이름(보안 그룹 ID 아님)입니다. [AWS 보안 그룹](#aws-security-groups)을 참조하세요. |
| `amazonec2-instance-type=m4.2xlarge`                                   | 자식 러너가 실행될 인스턴스 유형입니다. |
| `amazonec2-ssh-user=xxxx`                                              | 인스턴스에 SSH 액세스를 할 사용자입니다. |
| `amazonec2-iam-instance-profile=xxxx_runner_machine_inst_profile_name` | 러너 머신에 사용할 IAM 인스턴스 프로필입니다. |
| `amazonec2-ami=xxxx_runner_machine_ami_id`                             | 특정 이미지의 GitLab 러너 AMI ID입니다. |
| `amazonec2-request-spot-instance=true`                                 | 온디맨드 가격보다 저렴한 여유 EC2 용량을 사용합니다. |
| `amazonec2-spot-price=xxxx_runner_machine_spot_price=x.xx`             | 스팟 인스턴스 입찰 가격(미국 달러)입니다. `--amazonec2-request-spot-instance flag`이 `true`로 설정되어야 합니다. `amazonec2-spot-price`을 생략하면 Docker Machine은 최대 가격을 `$0.50` 시간당의 기본값으로 설정합니다. |
| `amazonec2-security-group-readonly=true`                               | 보안 그룹을 읽기 전용으로 설정합니다. |
| `amazonec2-userdata=xxxx_runner_machine_userdata_path`                 | 러너 머신 `userdata` 경로를 지정합니다. |
| `amazonec2-root-size=XX`                                               | 인스턴스의 루트 디스크 크기(GB)입니다. |

참고:

- `MachineOptions` 아래에서 [AWS Docker Machine 드라이버](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/aws.md#options)가 지원하는 모든 항목을 추가할 수 있습니다. 인프라 설정이 다양한 옵션을 적용할 수 있으므로 Docker 설명서를 읽어보시기를 권장합니다.
- `amazonec2-ami`을 설정하여 다른 AMI ID를 선택하지 않으면 자식 인스턴스는 기본적으로 Ubuntu 16.04를 사용합니다. [Docker Machine에서 지원하는 기본 운영 체제](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/os-base)만 설정합니다.
- 머신 옵션 중 하나로 `amazonec2-private-address-only=true`을 지정하면 EC2 인스턴스에 공용 IP가 할당되지 않습니다. VPC가 인터넷 게이트웨이(IGW)로 올바르게 구성되고 라우팅이 정상인 경우 괜찮지만, 더 복잡한 구성이 있는 경우 고려할 사항입니다. [VPC 연결에 대한 Docker 설명서](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/aws.md#vpc-connectivity)에서 자세히 알아보세요.

`[runners.machine]` 아래의 [기타 옵션](../advanced-configuration.md#the-runnersmachine-section)도 사용할 수 있습니다.

### 모든 것을 하나로 모으기 {#getting-it-all-together}

`/etc/gitlab-runner/config.toml`의 전체 예시입니다:

```toml
concurrent = 10
check_interval = 0

[[runners]]
  name = "gitlab-aws-autoscaler"
  url = "<URL of your GitLab instance>"
  token = "<runner's token>"
  executor = "docker+machine"
  limit = 20
  [runners.docker]
    image = "alpine"
    privileged = true
    disable_cache = true
  [runners.cache]
    Type = "s3"
    Shared = true
    [runners.cache.s3]
      ServerAddress = "s3.amazonaws.com"
      AccessKey = "<your AWS Access Key ID>"
      SecretKey = "<your AWS Secret Access Key>"
      BucketName = "<the bucket where your cache should be kept>"
      BucketLocation = "us-west-2"
  [runners.machine]
    IdleCount = 1
    IdleTime = 1800
    MaxBuilds = 100
    MachineDriver = "amazonec2"
    MachineName = "gitlab-docker-machine-%s"
    MachineOptions = [
      "amazonec2-access-key=XXXX",
      "amazonec2-secret-key=XXXX",
      "amazonec2-region=eu-central-1",
      "amazonec2-vpc-id=vpc-xxxxx",
      "amazonec2-subnet-id=subnet-xxxxx",
      "amazonec2-use-private-address=true",
      "amazonec2-tags=runner-manager-name,gitlab-aws-autoscaler,gitlab,true,gitlab-runner-autoscale,true",
      "amazonec2-security-group=XXXX",
      "amazonec2-instance-type=m4.2xlarge",
    ]
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

## Amazon EC2 Spot 인스턴스로 비용 절감 {#cutting-down-costs-with-amazon-ec2-spot-instances}

[Amazon이 설명한 대로](https://aws.amazon.com/ec2/spot/):

>
Amazon EC2 Spot 인스턴스는 여유 Amazon EC2 컴퓨팅 용량에 대해 입찰할 수 있습니다. Spot 인스턴스는 온디맨드 가격에 비해 할인된 가격으로 자주 사용 가능하므로 애플리케이션 실행 비용을 크게 줄이고 같은 예산으로 애플리케이션의 컴퓨팅 용량과 처리량을 늘리며 새로운 유형의 클라우드 컴퓨팅 애플리케이션을 사용할 수 있습니다.

위에서 선택한 [`runners.machine`](#the-runnersmachine-section) 옵션 외에도 `/etc/gitlab-runner/config.toml` 내의 `MachineOptions` 섹션에 다음을 추가합니다:

```toml
    MachineOptions = [
      "amazonec2-request-spot-instance=true",
      "amazonec2-spot-price=",
    ]
```

빈 `amazonec2-spot-price`을 사용하는 이 구성에서 AWS는 Spot 인스턴스에 대한 입찰 가격을 해당 인스턴스 클래스의 기본 온디맨드 가격으로 설정합니다. `amazonec2-spot-price`을 완전히 생략하면 Docker Machine은 최대 가격을 [시간당 기본값 $0.50](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/aws.md#environment-variables-and-default-values)으로 설정합니다.

Spot 인스턴스 요청을 추가로 사용자 정의할 수 있습니다:

```toml
    MachineOptions = [
      "amazonec2-request-spot-instance=true",
      "amazonec2-spot-price=0.03",
      "amazonec2-block-duration-minutes=60"
    ]
```

이 구성을 사용하면 Docker 머신이 최대 Spot 요청 가격이 시간당 $0.03이고 Spot 인스턴스의 기간이 60분으로 제한되는 Spot 인스턴스를 사용하여 생성됩니다. 위에서 언급한 `0.03` 숫자는 단지 예시일 뿐이므로 선택한 영역을 기반으로 현재 가격을 확인하세요.

Amazon EC2 Spot 인스턴스에 대해 자세히 알아보려면 다음 링크를 방문하세요:

- <https://aws.amazon.com/ec2/spot/>
- <https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/spot-requests.html>
- <https://aws.amazon.com/ec2/spot/getting-started/>

### Spot 인스턴스의 주의 사항 {#caveats-of-spot-instances}

Spot 인스턴스는 미사용 리소스를 활용하고 인프라 비용을 최소화하는 좋은 방법이지만 그 의미를 숙지해야 합니다.

Spot 인스턴스에서 CI 작업을 실행하면 Spot 인스턴스 가격 책정 모델로 인해 실패율이 증가할 수 있습니다. 지정한 최대 Spot 가격이 현재 Spot 가격을 초과하면 요청된 용량을 얻을 수 없습니다. Spot 가격 책정은 시간 단위로 수정됩니다. 수정된 Spot 인스턴스 가격 아래의 최대 가격을 가진 기존 Spot 인스턴스는 2분 이내에 종료되며 Spot 호스트의 모든 작업이 실패합니다.

결과적으로 자동 크기 조정 러너는 새 머신을 생성하지 못하지만 새 인스턴스를 계속 요청합니다. 이는 결국 60개의 요청을 하게 되고 그 후 AWS는 더 이상 수락하지 않습니다. 그 후 Spot 가격이 허용되면 호출 금액 제한이 초과되어 잠시 잠급니다.

이 경우가 발생하면 러너 관리자 머신에서 다음 명령을 사용하여 Docker 머신의 상태를 볼 수 있습니다:

```shell
docker-machine ls -q --filter state=Error --format "{{.NAME}}"
```

> [!note]
> GitLab 러너가 Spot 가격 변화를 우아하게 처리하는 것과 관련된 몇 가지 문제가 있으며, `docker-machine`이 Docker 머신을 계속 제거하려고 시도한다는 보고가 있습니다. GitLab은 업스트림 프로젝트의 두 경우 모두에 대한 패치를 제공했습니다. 자세한 내용은 [이슈 2771](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/2771) 및 [이슈 2772](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/2772)를 참조하세요.

GitLab fork는 AWS EC2 플릿과 spot 인스턴스와의 사용을 지원하지 않습니다. 대신 [Continuous Kernel Integration Project의 다운스트림 fork](https://gitlab.com/cki-project/mirror/docker-machine)를 사용할 수 있습니다.

## 결론 {#conclusion}

이 가이드에서는 AWS에서 자동 크기 조정 모드로 GitLab 러너를 설치하고 구성하는 방법을 배웠습니다.

GitLab 러너의 자동 크기 조정 기능을 사용하면 시간과 비용을 모두 절약할 수 있습니다. AWS에서 제공하는 Spot 인스턴스를 사용하면 더욱 절약할 수 있지만 그 의미를 숙지해야 합니다. 입찰가가 충분히 높다면 문제가 없어야 합니다.

이 자습서에 (크게) 영향을 미친 다음 사용 사례를 읽을 수 있습니다:

- [HumanGeo가 Jenkins에서 GitLab으로 전환](https://about.gitlab.com/blog/humangeo-switches-jenkins-gitlab-ci/)
- [Substrakt Health - GitLab CI/CD 러너를 자동 크기 조정하고 EC2 비용을 90% 절감](https://about.gitlab.com/blog/autoscale-ci-runners/)
