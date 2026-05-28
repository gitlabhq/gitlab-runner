---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: GitLab 러너 인프라 도구 키트
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated
- 상태:  실험

{{< /details >}}

[GitLab 러너 인프라 도구 키트(GRIT)](https://gitlab.com/gitlab-org/ci-cd/runner-tools/grit)는 공개 클라우드 공급자에서 많은 일반적인 러너 구성을 만들고 관리하는 데 사용할 수 있는 Terraform 모듈 라이브러리입니다.

> [!note]
> 이 기능은 [실험](https://docs.gitlab.com/policy/development_stages_support/#experiment) 단계입니다. GRIT 개발 상태에 대한 자세한 정보는 [에픽 1](https://gitlab.com/groups/gitlab-org/ci-cd/runner-tools/-/epics/1)을 참조하세요. 이 기능에 대한 피드백을 제공하려면 [이슈 84](https://gitlab.com/gitlab-org/ci-cd/runner-tools/grit/-/issues/84)에 댓글을 남기세요.

## GRIT를 사용하여 러너 만들기 {#create-a-runner-with-grit}

GRIT를 사용하여 AWS에서 자동 크기 조정 Linux Docker를 배포하려면:

1. GitLab 및 AWS에 대한 액세스를 제공하려면 다음 변수를 설정합니다:

   - `GITLAB_TOKEN`
   - `AWS_REGION`
   - `AWS_SECRET_ACCESS_KEY`
   - `AWS_ACCESS_KEY_ID`

1. 최신 [GRIT 릴리스](https://gitlab.com/gitlab-org/ci-cd/runner-tools/grit/-/releases)를 다운로드하고 `.local/grit`에 추출합니다.
1. `main.tf` Terraform 모듈을 만듭니다:

   ```hcl
   module "runner" {
     source = ".local/grit/scenarios/aws/linux/docker-autoscaler-default"

     name               = "grit-runner"
     gitlab_project_id  = "39258790" # gitlab.com/josephburnett/hello-runner
     runner_description = "Autoscaling Linux Docker runner on AWS deployed with GRIT. "
     runner_tags        = ["aws", "linux"]
     max_instances      = 5
     min_support        = "experimental"
   }
   ```

1. 모듈을 초기화하고 적용합니다:

   ```plaintext
   terraform init
   terraform apply
   ```

이 단계들은 GitLab 프로젝트에 새로운 러너를 만듭니다. 러너 관리자는 `docker-autoscaler` 실행기를 사용하여 `aws` 및 `linux`로 태그된 작업을 실행합니다. 러너는 워크로드를 기반으로 새로운 자동 크기 조정 그룹(ASG)을 통해 1개에서 5개의 VM을 프로비저닝합니다. ASG는 러너 팀이 소유한 공개 AMI를 사용합니다. 러너 관리자와 ASG 모두 새로운 VPC에서 운영됩니다. 모든 리소스는 제공된 값(`grit-runner`)을 기반으로 이름이 지정되므로 단일 AWS 프로젝트에서 이 모듈의 여러 인스턴스를 다른 이름으로 만들 수 있습니다.

## 지원 수준 및 `min_support` 매개변수 {#support-levels-and-the-min_support-parameter}

모든 GRIT 모듈에 대해 `min_support` 값을 제공해야 합니다. 이 매개변수는 운영자가 배포에 필요로 하는 최소 지원 수준을 지정합니다. GRIT 모듈은 `none`, `experimental`, `beta` 또는 `GA`의 지원 지정으로 연결됩니다. 목표는 모든 모듈이 `GA` 상태에 도달하는 것입니다.

`none`은 특수한 경우입니다. 지원 보장이 없는 모듈로, 주로 테스트 및 개발을 위한 것입니다.

`experimental`, `beta` 및 `ga` 모듈은 [GitLab의 개발 단계 정의](https://docs.gitlab.com/policy/development_stages_support/)를 준수합니다.

### 공유 책임 모델 {#shared-responsibility-model}

GRIT는 작가(모듈 개발자)와 운영자(GRIT를 배포하는 사람) 간의 공유 책임 모델에서 운영됩니다. 각 역할의 구체적인 책임과 지원 수준이 결정되는 방식에 대한 자세한 내용은 GORP 문서의 [공유 책임 섹션](https://gitlab.com/gitlab-org/ci-cd/runner-tools/grit/-/blob/main/GORP.md#shared-responsibility)을 참조하세요.

## 러너 상태 관리 {#manage-runner-state}

러너를 유지하려면:

1. 모듈을 GitLab 프로젝트에 확인합니다.
1. Terraform 상태를 GitLab Terraform `backend.tf`에 저장합니다:

   ```hcl
   terraform {
     backend "http" {}
   }
   ```

1. `.gitlab-ci.yml`을 사용하여 변경 사항을 적용합니다:

   ```yaml
   terraform-apply:
     variables:
       TF_HTTP_LOCK_ADDRESS: "https://gitlab.com/api/v4/projects/${CI_PROJECT_ID}/terraform/state/${NAME}/lock"
       TF_HTTP_UNLOCK_ADDRESS: ${TF_HTTP_LOCK_ADDRESS}
       TF_HTTP_USERNAME: ${GITLAB_USER_LOGIN}
       TF_HTTP_PASSWORD: ${GITLAB_TOKEN}
       TF_HTTP_LOCK_METHOD: POST
       TF_HTTP_UNLOCK_METHOD: DELETE
     script:
       - terraform init
       - terraform apply -auto-approve
   ```

### 러너 삭제 {#delete-a-runner}

러너 및 인프라를 제거하려면:

```plaintext
terraform destroy
```

## 지원되는 구성 {#supported-configurations}

| 공급자     | 서비스 | 아키텍처   | OS    | 실행기         | 기능 지원 |
|--------------|---------|--------|-------|-------------------|-----------------|
| AWS          | EC2     | x86-64 | Linux | Docker 자동 크기 조정 | 실험    |
| AWS          | EC2     | Arm64  | Linux | Docker 자동 크기 조정 | 실험    |
| Google Cloud | GCE     | x86-64 | Linux | Docker 자동 크기 조정 | 실험    |
| Google Cloud | GKE     | x86-64 | Linux | Kubernetes        | 실험    |

## 고급 구성 {#advanced-configuration}

### 최상위 수준 모듈 {#top-level-modules}

공급자의 최상위 수준 모듈은 높은 수준의 분리되지 않거나 선택적 러너 구성 측면을 나타냅니다. 예를 들어, `fleeting` 및 `runner`는 액세스 자격 증명 및 인스턴스 그룹 이름만 공유하기 때문에 별도의 모듈입니다. `vpc`은 일부 사용자가 자신의 VPC를 제공하기 때문에 별도의 모듈입니다. 기존 VPC를 보유한 사용자는 다른 GRIT 모듈과 연결하기 위해 일치하는 입력 구조를 만들면 됩니다.

예를 들어, 최상위 수준의 VPC 모듈을 사용하여 VPC가 필요한 모듈의 VPC를 만들 수 있습니다:

   ```hcl
   module "runner" {
      source = ".local/grit/modules/aws/runner"

      vpc = {
         id         = module.vpc.id
         subnet_ids = module.vpc.subnet_ids
      }

      # ...additional config omitted
   }

   module "vpc" {
      source   = ".local/grit/modules/aws/vpc"

      zone = "us-east-1b"

      cidr        = "10.0.0.0/16"
      subnet_cidr = "10.0.0.0/24"
   }
   ```

사용자는 자신의 VPC를 제공하고 GRIT의 VPC 모듈을 사용하지 않을 수 있습니다:

   ```hcl
   module "runner" {
      source = ".local/grit/modules/aws/runner"

      vpc = {
         id         = PREEXISTING_VPC_ID
         subnet_ids = [PREEXISTING_SUBNET_ID]
      }

      # ...additional config omitted
   }
   ```

## GRIT에 기여 {#contributing-to-grit}

GRIT는 커뮤니티 기여를 환영합니다. 기여하기 전에 다음 리소스를 검토하세요:

### 개발자 인증서 출처 및 라이센스 {#developer-certificate-of-origin-and-license}

GRIT에 대한 모든 기여는 [개발자 인증서 출처 및 라이센스](https://docs.gitlab.com/legal/developer_certificate_of_origin/)의 대상입니다. 기여함으로써 GitLab Inc.에 제출된 현재 및 향후 기여에 대해 이 약관에 동의하고 동의합니다.

### 행동 강령 {#code-of-conduct}

GRIT는 [Contributor Covenant](https://www.contributor-covenant.org)에서 개작한 GitLab 행동 강령을 따릅니다. 프로젝트는 배경 또는 정체성에 관계없이 모든 사람을 위한 괴롭힘 없는 참여 경험을 만드는 데 최선을 다하고 있습니다.

### 기여 지침 {#contribution-guidelines}

GRIT에 기여할 때 다음 지침을 따르세요:

- 전체 아키텍처 설계를 위해 [GORP 지침](https://gitlab.com/gitlab-org/ci-cd/runner-tools/grit/-/blob/main/GORP.md)을 검토하세요.
- [Terraform 사용에 대한 Google의 모범 사례](https://docs.cloud.google.com/docs/terraform/best-practices/general-style-structure)를 준수하세요.
- 복잡성과 반복을 줄이기 위해 구성 가능한 모듈 방식을 따르세요.
- 기여에 대한 적절한 Go 테스트를 포함합니다.

### 테스트 및 린팅 {#testing-and-linting}

GRIT는 품질을 보장하기 위해 여러 테스트 및 린팅 도구를 사용합니다:

- 통합 테스트:  [Terratest](https://terratest.gruntwork.io/)를 사용하여 Terraform 계획을 검증합니다.
- 종단 간 테스트:  [e2e 디렉토리](https://gitlab.com/gitlab-org/ci-cd/runner-tools/grit/-/blob/main/e2e/README.md)에서 사용할 수 있습니다.
- Terraform 린팅:  `tflint`, `terraform fmt` 및 `terraform validate`을 사용합니다.
- Go 린팅:  Go 코드(주로 테스트)에 [golangci-lint](https://golangci-lint.run/)를 사용합니다.
- 문서:  [GitLab 문서 스타일 가이드](https://docs.gitlab.com/development/documentation/styleguide/)를 따르며 `vale` 및 `markdownlint`을 사용합니다.

개발 환경 설정, 테스트 실행 및 린팅에 대한 자세한 지침은 [CONTRIBUTING.md](https://gitlab.com/gitlab-org/ci-cd/runner-tools/grit/-/blob/main/CONTRIBUTING.md)를 참조하세요.

## GRIT를 누가 사용합니까? {#who-uses-grit}

GRIT는 GitLab 생태계 내의 다양한 팀과 서비스에서 채택되었습니다:

- **[GitLab Dedicated](https://about.gitlab.com/dedicated/)**:  [GitLab Dedicated를 위한 호스팅 러너](https://docs.gitlab.com/administration/dedicated/hosted_runners/)는 GRIT를 사용하여 러너 인프라를 프로비저닝하고 관리합니다.
- **GitLab Self-Managed**:  GRIT는 많은 GitLab 자체 관리 고객들 사이에서 매우 요청되고 있습니다. 일부 조직에서는 표준화된 방식으로 러너 배포를 관리하기 위해 GRIT를 채택하기 시작했습니다.

조직에서 GRIT를 사용하고 있으며 이 섹션에 소개되고 싶다면 머지 리퀘스트를 열어주세요!
