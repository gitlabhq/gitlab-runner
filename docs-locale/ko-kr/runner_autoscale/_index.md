---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: GitLab 러너 자동 크기 조정
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

러너 자동 크기 조정을 사용하여 공용 클라우드 인스턴스에서 러너를 자동으로 확장할 수 있습니다. 러너를 자동 크기 조정기를 사용하도록 구성하면 클라우드 인프라에서 여러 작업을 동시에 실행하여 증가된 CI/CD 작업 부하를 처리할 수 있습니다.

공용 클라우드 인스턴스의 자동 크기 조정 옵션 외에도 러너 플릿을 호스팅하고 확장하기 위해 다음 컨테이너 오케스트레이션 솔루션을 사용할 수 있습니다.

- Red Hat OpenShift Kubernetes 클러스터
- Kubernetes 클러스터:  AWS EKS, Azure, 온프레미스
- AWS Fargate의 Amazon Elastic Container Services 클러스터

## 러너 관리자 구성 {#configure-the-runner-manager}

GitLab 러너 자동 크기 조정을 사용하려면 러너 관리자를 구성해야 하며, Docker 머신 자동 크기 조정 솔루션과 GitLab 러너 오토스케일러 모두 구성해야 합니다.

러너 관리자는 자동 크기 조정을 위해 여러 러너를 생성하는 러너의 한 유형입니다. 지속적으로 GitLab에 작업이 있는지 폴링하고 공개 클라우드 인프라와 상호작용하여 작업을 실행할 새 인스턴스를 생성합니다. 러너 관리자는 GitLab 러너가 설치된 호스트 머신에서 실행해야 합니다. Docker와 GitLab 러너가 지원하는 배포판(예: Ubuntu, Debian, CentOS 또는 RHEL)을 선택하세요.

1. 러너 관리자를 호스팅할 인스턴스를 생성합니다. 이는 인스턴스(AWS) 또는 스팟 가상 머신(GCP, Azure)이어서는 **안 됩니다.**
1. 인스턴스에 [GitLab 러너를 설치](../install/linux-repository.md)합니다.
1. 클라우드 제공자 자격 증명을 러너 관리자 호스트 머신에 추가합니다.

> [!note]
> 러너 관리자를 컨테이너에서 호스팅할 수 있습니다. [GitLab이 호스팅하는 러너](https://docs.gitlab.com/ci/runners/)의 경우 러너 관리자가 가상 머신 인스턴스에서 호스팅됩니다.

### GitLab 러너 Docker 머신 자동 크기 조정을 위한 자격 증명 구성 예제 {#example-credentials-configuration-for-gitlab-runner-docker-machine-autoscaling}

이 스니펫은 `runners.machine` 섹션의 `config.toml` 파일에 있습니다.

``` toml
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
      "amazonec2-security-group=xxxxx",
    ]
```

> [!note]
> 자격 증명 파일은 선택 사항입니다. AWS 환경의 러너 관리자에 대해 [AWS Identity and Access Management](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2_instance-profiles.html)(IAM) 인스턴스 프로필을 사용할 수 있습니다. AWS에서 러너 관리자를 호스팅하고 싶지 않으면 자격 증명 파일을 사용할 수 있습니다.

## 장애 허용 설계 구현 {#implement-a-fault-tolerant-design}

장애 허용 설계를 만들고 러너 관리자 호스트 장애를 방지하려면 같은 러너 태그를 사용하는 최소 두 개의 러너 관리자로 시작하세요.

예를 들어 GitLab.com에서는 여러 러너 관리자가 [Linux의 호스팅 러너](https://docs.gitlab.com/ci/runners/hosted_runners/linux/)를 위해 구성됩니다. 각 러너 관리자는 `saas-linux-small-amd64` 태그를 갖고 있습니다.

조직의 CI/CD 작업 부하에 대한 효율성과 성능의 균형을 맞추기 위해 자동 크기 조정 매개변수를 조정할 때 관찰성과 러너 플릿 메트릭을 사용하세요.

## 러너 자동 크기 조정 실행기 구성 {#configure-runner-autoscaling-executors}

러너 관리자를 구성한 후 자동 크기 조정에 특정한 실행기를 구성하세요:

- [Instance Executor](../executors/instance.md)
- [Docker Autoscaling Executor](../executors/docker_autoscaler.md)
- [Docker Machine Executor](../executors/docker_machine.md)

> [!note]
> Docker 머신 오토스케일러를 대체하는 기술을 포함하므로 Instance와 Docker 자동 크기 조정 실행기를 사용해야 합니다.
