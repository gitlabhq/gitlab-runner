---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: GitLab 러너 인스턴스 그룹 자동 크기 조정
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

GitLab 러너 인스턴스 그룹 자동 크기 조정은 Docker Machine 기반 자동 크기 조정 기술의 후속 제품입니다. GitLab 러너 인스턴스 그룹 자동 크기 조정 솔루션의 구성 요소는 다음과 같습니다:

- Taskscaler:  자동 크기 조정 논리를 관리하고, 기록을 유지하며, 클라우드 공급자 자동 크기 조정 그룹을 사용하는 러너 인스턴스를 위한 플릿을 생성합니다.
- [Fleeting](../fleet_scaling/fleeting.md):  클라우드 공급자 가상 머신을 위한 추상화입니다.
- 클라우드 공급자 플러그인:  대상 클라우드 플랫폼에 대한 API 호출을 처리하며 플러그인 개발 프레임워크를 사용하여 구현됩니다.

GitLab 러너 인스턴스 그룹 자동 크기 조정은 다음과 같이 작동합니다:

1. 러너 관리자는 지속적으로 GitLab 작업을 폴링합니다.
1. 응답으로 GitLab은 작업 페이로드를 러너 관리자에게 전송합니다.
1. 러너 관리자는 공용 클라우드 인프라와 상호작용하여 작업을 실행할 새 인스턴스를 생성합니다.
1. 러너 관리자는 이러한 작업을 자동 크기 조정 풀의 사용 가능한 러너에 배포합니다.

![GitLab Next 러너 자동 크기 조정 개요](img/next-runner-autoscaling-overview.png)

## 러너 관리자 구성 {#configure-the-runner-manager}

GitLab 러너 인스턴스 그룹 자동 크기 조정을 사용하려면 [러너 관리자를 구성](_index.md#configure-the-runner-manager)해야 합니다.

1. 러너 관리자를 호스팅할 인스턴스를 생성합니다. 이는 **must not** 스팟 인스턴스(AWS), 또는 스팟 가상 머신(GCP 또는 Azure)이 아니어야 합니다.
1. 인스턴스에 [GitLab 러너 설치](../install/linux-repository.md)합니다.
1. 러너 관리자 호스트 머신에 클라우드 공급자 자격증명을 추가합니다.

   > [!note]
   > 러너 관리자를 컨테이너에 호스팅할 수 있습니다. GitLab.com 및 GitLab Dedicated [호스팅 러너](https://docs.gitlab.com/ci/runners/)의 경우, 러너 관리자는 가상 머신 인스턴스에 호스팅됩니다.

### GitLab 러너 인스턴스 그룹 자동 크기 조정에 대한 예시 자격증명 구성 {#example-credentials-configuration-for-gitlab-runner-instance-group-autoscaler}

[AWS Identity and Access Management](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2_instance-profiles.html) (IAM) 인스턴스 프로필을 AWS 환경의 러너 관리자에 사용할 수 있습니다. 러너 관리자를 AWS에서 호스팅하지 않으려면 자격증명 파일을 사용할 수 있습니다.

예를 들어:

``` toml
## credentials_file

[default]
aws_access_key_id=__REDACTED__
aws_secret_access_key=__REDACTED__
```

자격증명 파일은 선택 사항입니다.

## 지원되는 공용 클라우드 인스턴스 {#supported-public-cloud-instances}

다음 자동 크기 조정 옵션이 공용 클라우드 컴퓨팅 인스턴스에 지원됩니다:

- Amazon Web Services EC2 인스턴스
- Google Compute Engine
- Microsoft Azure Virtual Machines

이러한 클라우드 인스턴스는 GitLab 러너 Docker Machine 자동 크기 조정에서도 지원됩니다.

## 지원되는 플랫폼 {#supported-platforms}

| 실행기                   | Linux                                | macOS                                | Windows                              |
|----------------------------|--------------------------------------|--------------------------------------|--------------------------------------|
| 인스턴스 실행기          | {{< icon name="check-circle" >}} 예 | {{< icon name="check-circle" >}} 예 | {{< icon name="check-circle" >}} 예 |
| Docker 자동 크기 조정 실행기 | {{< icon name="check-circle" >}} 예 | {{< icon name="dotted-circle" >}} 아니오 | {{< icon name="check-circle" >}} 예 |
