---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: AWS Fargate에서 GitLab CI 자동 크기 조정
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

> [!warning]
> Fargate 드라이버는 커뮤니티에서 지원합니다. GitLab 지원팀이 문제 디버깅을 시도하지만 보장은 제공하지 않습니다.

GitLab [사용자 정의 실행기](../../executors/custom.md) 는 [AWS Fargate](https://gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate) 드라이버로 Amazon Elastic Container Service(ECS)에서 각 GitLab CI 작업을 실행하기 위해 자동으로 컨테이너를 시작합니다.

이 문서의 작업을 완료한 후 실행기는 GitLab에서 시작된 작업을 실행할 수 있습니다. GitLab에서 커밋할 때마다 GitLab 인스턴스는 새 작업을 사용할 수 있다는 것을 러너에 알립니다. 그러면 러너는 AWS ECS에서 구성한 작업 정의를 기반으로 대상 ECS 클러스터에서 새 작업을 시작합니다. AWS ECS 작업 정의를 구성하여 Docker 이미지를 사용할 수 있습니다. 이 방식으로 AWS Fargate에서 실행할 수 있는 빌드 유형에 완전한 유연성이 있습니다.

![GitLab Runner Fargate 드라이버 아키텍처](../img/runner_fargate_driver_ssh.png)

이 문서는 구현에 대한 초기 이해를 제공하기 위한 예시입니다. 프로덕션 사용을 위한 것이 아니며, AWS에서 추가 보안이 필요합니다.

예를 들어 두 가지 AWS 보안 그룹이 필요할 수 있습니다:

- GitLab Runner를 호스팅하는 EC2 인스턴스에서 사용되며 제한된 외부 IP 범위에서만 SSH 연결을 수락합니다(관리 액세스용).
- Fargate 작업에 적용되며 EC2 인스턴스에서만 SSH 트래픽을 허용합니다.

공개되지 않은 컨테이너 레지스트리의 경우 ECS 작업에는 [IAM 권한(AWS ECR만 해당)](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_execution_IAM_role.html) 또는 ECR이 아닌 프라이빗 레지스트리의 경우 [프라이빗 레지스트리 인증](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/private-auth.html)이 필요합니다.

CloudFormation 또는 Terraform을 사용하여 AWS 인프라의 프로비저닝 및 설정을 자동화할 수 있습니다.

CI/CD 작업은 ECS 작업에서 정의한 이미지를 사용하며, `image:` 키워드의 값이 아닙니다. `.gitlab-ci.yml` 파일에서는 ECS 작업에 사용되는 이미지를 재정의하도록 허용하지 않습니다. ECS에서는 ECS 작업에 사용되는 이미지를 재정의할 수 없습니다.

이 제한을 해결하기 위해 다음 작업을 수행할 수 있습니다:

- 러너가 사용되는 모든 프로젝트의 모든 빌드 종속성을 포함하는 ECS 작업 정의에서 이미지를 생성하고 사용합니다.
- 다양한 이미지를 사용하여 여러 ECS 작업 정의를 생성하고 `FARGATE_TASK_DEFINITION` CI/CD 변수에서 ARN을 지정합니다.
- [AWS EKS Blueprints](https://aws-ia.github.io/terraform-aws-eks-blueprints/)로 생성된 Amazon EKS 클러스터의 Kubernetes 실행기를 사용하는 것을 고려하세요. Fargate 사용자 정의 실행기 드라이버는 GitLab에서 유지 관리하지 않으며 최선의 노력으로 지원됩니다.

자세한 내용은 [1시간 내에 GitLab EKS Fargate 러너 시작하기(제로 코드)](https://about.gitlab.com/blog/eks-fargate-runner/)를 참조하세요.

> [!warning]
> Fargate는 컨테이너 호스트를 추상화하여 컨테이너 호스트 속성의 구성 가능성을 제한합니다. 이는 높은 디스크 또는 네트워크 IO가 필요한 러너 워크로드에 영향을 주며, 이러한 속성은 Fargate에서 구성 가능성이 제한되거나 없습니다. Fargate에서 GitLab Runner를 사용하기 전에 CPU, 메모리, 디스크 IO 또는 네트워크 IO의 높은 컴퓨팅 특성을 가진 러너 워크로드가 Fargate에 적합한지 확인하세요.

## 필수 요구 사항 {#prerequisites}

시작하기 전에 다음이 필요합니다:

- EC2, ECS 및 ECR 리소스를 생성하고 구성할 수 있는 권한이 있는 AWS IAM 사용자입니다.
- AWS VPC 및 서브넷입니다.
- 하나 이상의 AWS 보안 그룹입니다.

## 1단계:  AWS Fargate 작업을 위한 컨테이너 이미지 준비 {#step-1-prepare-a-container-image-for-the-aws-fargate-task}

컨테이너 이미지를 준비합니다. 이 이미지를 레지스트리에 업로드할 수 있으며, GitLab 작업이 실행될 때 컨테이너를 생성하는 데 사용할 수 있습니다.

1. 이미지에 CI 작업을 빌드하는 데 필요한 도구가 있는지 확인합니다. 예를 들어, Java 프로젝트에는 `Java JDK` 및 Maven 또는 Gradle과 같은 빌드 도구가 필요합니다. Node.js 프로젝트에는 `node` 및 `npm`이 필요합니다.
1. 이미지에 GitLab Runner가 있는지 확인합니다. 이는 아티팩트와 캐싱을 처리합니다. 사용자 정의 실행기 문서의 [Run](../../executors/custom.md#run) 스테이지 섹션을 참조하여 추가 정보를 확인하세요.
1. 컨테이너 이미지가 공개 키 인증을 통해 SSH 연결을 수락할 수 있는지 확인합니다. 러너는 이 연결을 사용하여 `.gitlab-ci.yml` 파일에 정의된 빌드 명령을 AWS Fargate의 컨테이너로 전송합니다. SSH 키는 Fargate 드라이버에서 자동으로 관리됩니다. 컨테이너는 `SSH_PUBLIC_KEY` 환경 변수에서 키를 수락할 수 있어야 합니다.

[Debian 예시](https://gitlab.com/tmaczukin-test-projects/fargate-driver-debian)를 보면 GitLab Runner 및 SSH 구성을 확인할 수 있습니다. [Node.js 예시](https://gitlab.com/aws-fargate-driver-demo/docker-nodejs-gitlab-ci-fargate)를 보세요.

## 2단계:  레지스트리로 컨테이너 이미지 푸시 {#step-2-push-the-container-image-to-a-registry}

이미지를 생성한 후 ECS 작업 정의에서 사용할 이미지를 컨테이너 레지스트리에 게시합니다.

- [Amazon ECR 리포지토리](https://docs.aws.amazon.com/AmazonECR/latest/userguide/Repositories.html) 문서를 따라 리포지토리를 생성하고 ECR로 이미지를 푸시합니다.
- AWS CLI를 사용하여 ECR로 이미지를 푸시하려면 [AWS CLI를 사용하여 Amazon ECR 시작하기](https://docs.aws.amazon.com/AmazonECR/latest/userguide/getting-started-cli.html) 문서를 따릅니다.
- [GitLab 컨테이너 레지스트리](https://docs.gitlab.com/user/packages/container_registry/) 를 사용하려면 [Debian](https://gitlab.com/tmaczukin-test-projects/fargate-driver-debian) 또는 [NodeJS](https://gitlab.com/aws-fargate-driver-demo/docker-nodejs-gitlab-ci-fargate) 예시를 사용할 수 있습니다. Debian 이미지는 `registry.gitlab.com/tmaczukin-test-projects/fargate-driver-debian:latest`에 게시됩니다. NodeJS 예시 이미지는 `registry.gitlab.com/aws-fargate-driver-demo/docker-nodejs-gitlab-ci-fargate:latest`에 게시됩니다.

## 3단계:  GitLab Runner를 위한 EC2 인스턴스 생성 {#step-3-create-an-ec2-instance-for-gitlab-runner}

이제 AWS EC2 인스턴스를 생성합니다. 다음 단계에서 GitLab Runner를 설치합니다.

1. <https://console.aws.amazon.com/ec2/v2/home#LaunchInstanceWizard>로 이동합니다.
1. 인스턴스의 경우 Ubuntu Server 18.04 LTS AMI를 선택합니다. 선택한 AWS 지역에 따라 이름이 다를 수 있습니다.
1. 인스턴스 유형의 경우 t2.micro를 선택합니다. **다음: 인스턴스 세부 정보 구성**을 선택합니다.
1. **Number of instances**의 기본값을 유지합니다.
1. **네트워크**의 경우 VPC를 선택합니다.
1. **Auto-assign Public IP**를 **사용**로 설정합니다.
1. **IAM role** 아래에서 **Create new IAM role**을 선택합니다. 이 역할은 테스트 목적으로만 사용되며 안전하지 않습니다.
   1. **역할 생성**을 선택합니다.
   1. **AWS service**를 선택하고 **Common use cases** 아래에서 **EC2**를 선택합니다. 그 다음 **다음: 권한**을 선택합니다.
   1. **AmazonECS_FullAccess** 정책의 체크박스를 선택합니다. **다음: 태그**를 선택합니다.
   1. **다음: 검토**를 선택합니다.
   1. IAM 역할의 이름을 입력합니다(예: `fargate-test-instance`). 그리고 **역할 생성**을 선택합니다.
1. 인스턴스를 생성하는 브라우저 탭으로 돌아갑니다.
1. **Create new IAM role**의 왼쪽에서 새로 고침 버튼을 선택합니다. `fargate-test-instance` 역할을 선택합니다. **다음: 스토리지 추가**를 선택합니다.
1. **다음: 태그 추가**를 선택합니다.
1. **다음: 보안 그룹 구성**을 선택합니다.
1. **Create a new security group**을 선택하고 `fargate-test`라고 지정합니다. 그리고 SSH에 대한 규칙이 정의되어 있는지 확인합니다(`Type: SSH, Protocol: TCP, Port Range: 22`). 인바운드 및 아웃바운드 규칙의 IP 범위를 지정해야 합니다.
1. **Review and Launch**을 선택합니다.
1. **Launch**을 선택합니다.
1. 선택 사항입니다. **Create a new key pair**을 선택하고 `fargate-runner-manager`라고 지정합니다. 그리고 **Download Key Pair**를 선택합니다. SSH의 프라이빗 키가 컴퓨터에 다운로드됩니다(브라우저에서 구성한 디렉터리 확인).
1. **Launch Instances**을 선택합니다.
1. **View Instances**를 선택합니다.
1. 인스턴스가 시작될 때까지 기다립니다. `IPv4 Public IP` 주소를 기록합니다.

## 4단계:  EC2 인스턴스에서 GitLab Runner 설치 및 구성 {#step-4-install-and-configure-gitlab-runner-on-the-ec2-instance}

이제 Ubuntu 인스턴스에 GitLab Runner를 설치합니다.

1. GitLab 프로젝트의 **설정 > CI/CD**로 이동하고 Runners 섹션을 확장합니다. **Set up a specific Runner manually** 아래에서 등록 토큰을 기록합니다.
1. `chmod 400 path/to/downloaded/key/file`을 실행하여 키 파일의 권한이 올바른지 확인합니다.
1. 다음을 사용하여 생성한 EC2 인스턴스로 SSH 연결합니다:

   ```shell
   ssh ubuntu@[ip_address] -i path/to/downloaded/key/file
   ```

1. 연결에 성공하면 다음 명령을 실행합니다:

   ```shell
   sudo mkdir -p /opt/gitlab-runner/{metadata,builds,cache}
   curl -s "https://packages.gitlab.com/install/repositories/runner/gitlab-runner/script.deb.sh" | sudo bash
   sudo apt install gitlab-runner
   ```

1. 1단계에서 기록한 GitLab URL 및 등록 토큰과 함께 이 명령을 실행합니다.

   ```shell
   sudo gitlab-runner register --url "https://gitlab.com/" --registration-token TOKEN_HERE --name fargate-test-runner --run-untagged --executor custom -n
   ```

1. `sudo vim /etc/gitlab-runner/config.toml`을 실행하고 다음 내용을 추가합니다:

   ```toml
   concurrent = 1
   check_interval = 0

   [session_server]
     session_timeout = 1800

   [[runners]]
     name = "fargate-test"
     url = "https://gitlab.com/"
     token = "__REDACTED__"
     executor = "custom"
     builds_dir = "/opt/gitlab-runner/builds"
     cache_dir = "/opt/gitlab-runner/cache"
     [runners.custom]
       volumes = ["/cache", "/path/to-ca-cert-dir/ca.crt:/etc/gitlab-runner/certs/ca.crt:ro"]
       config_exec = "/opt/gitlab-runner/fargate"
       config_args = ["--config", "/etc/gitlab-runner/fargate.toml", "custom", "config"]
       prepare_exec = "/opt/gitlab-runner/fargate"
       prepare_args = ["--config", "/etc/gitlab-runner/fargate.toml", "custom", "prepare"]
       run_exec = "/opt/gitlab-runner/fargate"
       run_args = ["--config", "/etc/gitlab-runner/fargate.toml", "custom", "run"]
       cleanup_exec = "/opt/gitlab-runner/fargate"
       cleanup_args = ["--config", "/etc/gitlab-runner/fargate.toml", "custom", "cleanup"]
   ```

1. GitLab Self-Managed 인스턴스(프라이빗 CA 포함)가 있는 경우 이 라인을 추가합니다:

   ```toml
          volumes = ["/cache", "/path/to-ca-cert-dir/ca.crt:/etc/gitlab-runner/certs/ca.crt:ro"]
   ```

   [인증서 신뢰에 대해 자세히 알아보기](../tls-self-signed.md#trusting-the-certificate-for-the-other-cicd-stages).

   아래 표시된 `config.toml` 파일의 섹션은 등록 명령으로 생성됩니다. 변경하지 마세요.

   ```toml
   concurrent = 1
   check_interval = 0

   [session_server]
     session_timeout = 1800

   name = "fargate-test"
   url = "https://gitlab.com/"
   token = "__REDACTED__"
   executor = "custom"
   ```

1. `sudo vim /etc/gitlab-runner/fargate.toml`을 실행하고 다음 내용을 추가합니다:

   ```toml
   LogLevel = "info"
   LogFormat = "text"

   [Fargate]
     Cluster = "test-cluster"
     Region = "us-east-2"
     Subnet = "subnet-xxxxxx"
     SecurityGroup = "sg-xxxxxxxxxxxxx"
     TaskDefinition = "test-task:1"
     EnablePublicIP = true

   [TaskMetadata]
     Directory = "/opt/gitlab-runner/metadata"

   [SSH]
     Username = "root"
     Port = 22
   ```

   - `Cluster`의 값과 `TaskDefinition`의 이름을 기록합니다. 이 예시는 `test-task`을 `:1`로 수정 번호를 표시합니다. 수정 번호를 지정하지 않으면 최신 **active** 수정 버전을 사용합니다.
   - 지역을 선택합니다. 러너 관리자 인스턴스에서 `Subnet` 값을 가져옵니다.
   - 보안 그룹 ID를 찾으려면:

     1. AWS의 인스턴스 목록에서 생성한 EC2 인스턴스를 선택합니다. 세부 정보가 표시됩니다.
     1. **Security groups** 아래에서 생성한 그룹의 이름을 선택합니다.
     1. **Security group ID**를 복사합니다.

     프로덕션 환경에서는 [AWS 가이드라인](https://docs.aws.amazon.com/vpc/latest/userguide/vpc-security-groups.html)을 따라 보안 그룹을 설정하고 사용합니다.

   - `EnablePublicIP`이 true로 설정된 경우 작업 컨테이너의 공개 IP를 수집하여 SSH 연결을 수행합니다.
   - `EnablePublicIP`이 false로 설정된 경우:
     - Fargate 드라이버는 작업 컨테이너의 프라이빗 IP를 사용합니다. `false`로 설정되었을 때 연결을 설정하려면 VPC 보안 그룹에 포트 22(SSH)에 대한 인바운드 규칙이 있어야 하며, 소스는 VPC CIDR이어야 합니다.
     - 외부 종속성을 가져오려면 프로비저닝된 AWS Fargate 컨테이너가 퍼블릭 인터넷에 액세스할 수 있어야 합니다. AWS Fargate 컨테이너에 퍼블릭 인터넷 액세스를 제공하려면 VPC의 NAT Gateway를 사용할 수 있습니다.

   - SSH 서버의 포트 번호는 선택 사항입니다. 생략되면 기본 SSH 포트(22)가 사용됩니다.
   - 섹션 설정에 대한 자세한 내용은 [Fargate 드라이버 문서](https://gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/-/tree/master/docs#configuration)를 참조하세요.

1. Fargate 드라이버를 설치합니다:

   ```shell
   sudo curl -Lo /opt/gitlab-runner/fargate "https://gitlab-runner-custom-fargate-downloads.s3.amazonaws.com/latest/fargate-linux-amd64"
   sudo chmod +x /opt/gitlab-runner/fargate
   ```

## 5단계:  ECS Fargate 클러스터 생성 {#step-5-create-an-ecs-fargate-cluster}

Amazon ECS 클러스터는 ECS 컨테이너 인스턴스의 그룹입니다.

1. [`https://console.aws.amazon.com/ecs/home#/clusters`](https://console.aws.amazon.com/ecs/home#/clusters)로 이동합니다.
1. **Create Cluster**을 선택합니다.
1. **Networking only** 유형을 선택합니다. **다음 단계**를 선택합니다.
1. `test-cluster`이라고 이름을 지정합니다(`fargate.toml`와 동일).
1. **생성**을 선택합니다.
1. **View cluster**를 선택합니다. `Cluster ARN` 값에서 지역 및 계정 ID 부분을 기록합니다.
1. **Update Cluster**를 선택합니다.
1. `Default capacity provider strategy` 다음에 **Add another provider**를 선택하고 `FARGATE`을 선택합니다. **업데이트**를 선택합니다.

AWS [문서](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/Welcome.html)를 참조하여 ECS Fargate 클러스터를 설정하고 작업하는 방법에 대한 자세한 지침을 확인합니다.

## 6단계:  ECS 작업 정의 생성 {#step-6-create-an-ecs-task-definition}

이 단계에서는 `Fargate` 유형의 작업 정의를 생성하고 CI 빌드에 사용할 수 있는 컨테이너 이미지를 참조합니다.

1. [`https://console.aws.amazon.com/ecs/home#/taskDefinitions`](https://console.aws.amazon.com/ecs/home#/taskDefinitions)로 이동합니다.
1. **Create new Task Definition**을 선택합니다.
1. **FARGATE**를 선택하고 **다음 단계**를 선택합니다.
1. `test-task`이라고 이름을 지정합니다. (참고:  이름은 `fargate.toml` 파일에서 정의된 것과 동일한 값이지만 `:1` 없음).
1. **Task memory (GB)** 및 **Task CPU (vCPU)**의 값을 선택합니다.
1. **Add container**를 선택합니다. 그 다음:
   1. `ci-coordinator`이라고 이름을 지정합니다. 그러면 Fargate 드라이버가 `SSH_PUBLIC_KEY` 환경 변수를 삽입할 수 있습니다.
   1. 이미지를 정의합니다(예: `registry.gitlab.com/tmaczukin-test-projects/fargate-driver-debian:latest`).
   1. 포트 매핑을 22/TCP로 정의합니다.
   1. **추가**를 선택합니다.
1. **생성**을 선택합니다.
1. **View task definition**를 선택합니다.

> [!warning]
> 단일 Fargate 작업은 하나 이상의 컨테이너를 시작할 수 있습니다. Fargate 드라이버는 `SSH_PUBLIC_KEY` 환경 변수를 `ci-coordinator` 이름을 가진 컨테이너에만 삽입합니다. Fargate 드라이버에서 사용하는 모든 작업 정의에 이 이름을 가진 컨테이너가 있어야 합니다. 이 이름을 가진 컨테이너는 SSH 서버가 있고 위에서 설명한 대로 모든 GitLab Runner 요구 사항이 설치된 것이어야 합니다.

AWS [문서](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/create-task-definition.html)를 참조하여 작업 정의를 설정하고 작업하는 방법에 대한 자세한 지침을 확인합니다.

AWS ECR에서 이미지를 시작하는 데 필요한 ECS 서비스 권한에 대한 자세한 내용은 [Amazon ECS 작업 실행 IAM 역할](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_execution_IAM_role.html)을 참조하세요.

GitLab 인스턴스에서 호스팅되는 것을 포함한 프라이빗 레지스트리에 대한 ECS 인증에 대한 자세한 내용은 [프라이빗 레지스트리 인증](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/private-auth.html)을 참조하세요.

이 시점에서 러너 관리자 및 Fargate 드라이버가 구성되고 AWS Fargate에서 작업 실행을 시작할 준비가 됩니다.

## 7단계:  구성 테스트 {#step-7-test-the-configuration}

구성이 이제 사용 준비가 되었습니다.

1. GitLab 프로젝트에서 `.gitlab-ci.yml` 파일을 생성합니다:

   ```yaml
   test:
     script:
       - echo "It works!"
       - for i in $(seq 1 30); do echo "."; sleep 1; done
   ```

1. 프로젝트의 **CI/CD > 파이프라인**으로 이동합니다.
1. **Run Pipeline**을 선택합니다.
1. 브랜치와 모든 변수를 업데이트하고 **Run Pipeline**을 선택합니다.

> [!note]
> `image` 및 `service` 키워드는 `.gitlab-ci.yml` 파일에서 무시됩니다. 러너는 작업 정의에서 지정한 값만 사용합니다.

## 정리 {#clean-up}

AWS Fargate를 사용하여 사용자 정의 실행기를 테스트한 후 정리하려면 다음 객체를 제거합니다:

- [3단계](#step-3-create-an-ec2-instance-for-gitlab-runner)에서 생성된 EC2 인스턴스, 키 페어, IAM 역할 및 보안 그룹.
- [5단계](#step-5-create-an-ecs-fargate-cluster)에서 생성된 ECS Fargate 클러스터.
- [6단계](#step-6-create-an-ecs-task-definition)에서 생성된 ECS 작업 정의.

## 프라이빗 AWS Fargate 작업 구성 {#configure-a-private-aws-fargate-task}

높은 수준의 보안을 보장하려면 [프라이빗 AWS Fargate 작업](https://repost.aws/knowledge-center/ecs-fargate-tasks-private-subnet)을 구성합니다. 이 구성에서 실행기는 내부 AWS IP 주소만 사용합니다. AWS에서의 아웃바운드 트래픽만 허용하므로 CI/CD 작업이 프라이빗 AWS Fargate 인스턴스에서 실행됩니다.

프라이빗 AWS Fargate 작업을 구성하려면 다음 단계를 완료하여 AWS를 구성하고 프라이빗 서브넷에서 AWS Fargate 작업을 실행합니다:

1. 기존 퍼블릭 서브넷이 VPC 주소 범위의 모든 IP 주소를 예약하지 않았는지 확인합니다. VPC 및 서브넷의 `cidr` 주소 범위를 검사합니다. 서브넷 `cidr` 주소 범위가 VPC `cidr` 주소 범위의 부분집합인 경우 2단계와 4단계를 건너뜁니다. 그렇지 않으면 VPC에 사용 가능한 주소 범위가 없으므로 VPC 및 퍼블릭 서브넷을 삭제하고 다시 생성해야 합니다:
   1. 기존 서브넷 및 VPC를 삭제합니다.
   1. 삭제한 VPC와 동일한 구성으로 [VPC를 생성](https://docs.aws.amazon.com/vpc/latest/privatelink/create-interface-endpoint.html#create-interface-endpoint)하고 `cidr` 주소를 업데이트합니다(예: `10.0.0.0/23`).
   1. 삭제한 서브넷과 동일한 구성으로 [퍼블릭 서브넷을 생성](https://docs.aws.amazon.com/vpc/latest/privatelink/interface-endpoints.html)합니다. VPC 주소 범위의 부분집합인 `cidr` 주소를 사용합니다(예: `10.0.0.0/24`).
1. 퍼블릭 서브넷과 동일한 구성으로 [프라이빗 서브넷을 생성](https://docs.aws.amazon.com/vpc/latest/userguide/create-subnet.html#create-subnets)합니다. 퍼블릭 서브넷 범위와 겹치지 않는 `cidr` 주소 범위를 사용합니다(예: `10.0.1.0/24`).
1. [NAT Gateway를 생성](https://docs.aws.amazon.com/vpc/latest/userguide/vpc-nat-gateway.html)하고 퍼블릭 서브넷 내에 배치합니다.
1. 프라이빗 서브넷 라우팅 테이블을 수정하여 대상 `0.0.0.0/0`이 NAT Gateway를 가리키도록 합니다.
1. `farget.toml` 구성을 업데이트합니다:

   ```toml
   Subnet = "private-subnet-id"
   EnablePublicIP = false
   UsePublicIP = false
   ```

1. Fargate 작업과 연결된 IAM 역할에 다음 인라인 정책을 추가합니다(Fargate 작업과 연결된 IAM 역할은 일반적으로 `ecsTaskExecutionRole`이라고 이름이 지정되며 이미 존재해야 합니다).

   ```json
   {
       "Statement": [
           {
               "Sid": "VisualEditor0",
               "Effect": "Allow",
               "Action": [
                   "secretsmanager:GetSecretValue",
                   "kms:Decrypt",
                   "ssm:GetParameters"
               ],
               "Resource": [
                   "arn:aws:secretsmanager:*:<account-id>:secret:*",
                   "arn:aws:kms:*:<account-id>:key/*"
               ]
           }
       ]
   }
   ```

1. 보안 그룹의 "인바운드 규칙"을 변경하여 보안 그룹 자체를 참조합니다. AWS 구성 대화상자에서:
   - `Type`을 `ssh`로 설정합니다.
   - `Source`을 `Custom`로 설정합니다.
   - 보안 그룹을 선택합니다.
   - 모든 호스트에서 SSH 액세스를 허용하는 기존 인바운드 규칙을 제거합니다.

> [!warning]
> 기존 인바운드 규칙을 제거하면 Amazon Elastic Compute Cloud 인스턴스에 SSH로 연결할 수 없습니다.

자세한 내용은 다음 AWS 문서를 참조하세요:

- [Amazon ECS 작업 실행 IAM 역할](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_execution_IAM_role.html)
- [Amazon ECR 인터페이스 VPC 엔드포인트(AWS PrivateLink)](https://docs.aws.amazon.com/AmazonECR/latest/userguide/vpc-endpoints.html)
- [Amazon ECS 인터페이스 VPC 엔드포인트](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/vpc-endpoints.html)
- [퍼블릭 및 프라이빗 서브넷이 있는 VPC](https://docs.aws.amazon.com/vpc/latest/userguide/vpc-example-private-subnets-nat.html)

## 문제 해결 {#troubleshooting}

### `No Container Instances were found in your cluster` 오류가 구성 테스트 중에 발생 {#no-container-instances-were-found-in-your-cluster-error-when-testing-the-configuration}

`error="starting new Fargate task: running new task on Fargate: error starting AWS Fargate Task: InvalidParameterException: No Container Instances were found in your cluster."`

AWS Fargate 드라이버에는 ECS 클러스터가 [default capacity provider strategy](#step-5-create-an-ecs-fargate-cluster)로 구성되어야 합니다.

추가 참고 자료:

- 기본 [capacity provider strategy](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/cluster-capacity-providers.html)는 각 Amazon ECS 클러스터와 연결됩니다. 다른 capacity provider strategy 또는 실행 유형을 지정하지 않으면 작업이 실행되거나 서비스가 생성될 때 클러스터는 이 전략을 사용합니다.
- [`capacityProviderStrategy`](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_RunTask.html#ECS-RunTask-request-capacityProviderStrategy)를 지정하면 `launchType` 매개변수를 생략해야 합니다. `capacityProviderStrategy` 또는 `launchType`를 지정하지 않으면 클러스터의 `defaultCapacityProviderStrategy`이 사용됩니다.

### 메타데이터 `file does not exist` 오류가 작업 실행 중에 발생 {#metadata-file-does-not-exist-error-when-running-jobs}

`Application execution failed PID=xxxxx error="obtaining information about the running task: trying to access file \"/opt/gitlab-runner/metadata/<runner_token>-xxxxx.json\": file does not exist" cleanup_std=err job=xxxxx project=xx runner=<runner_token>`

IAM 역할 정책이 올바르게 구성되었으며 `/opt/gitlab-runner/metadata/`에서 메타데이터 JSON 파일을 생성하기 위해 쓰기 작업을 수행할 수 있는지 확인합니다. 비프로덕션 환경에서 테스트하려면 AmazonECS_FullAccess 정책을 사용합니다. 조직의 보안 요구 사항에 따라 IAM 역할 정책을 검토합니다.

### `connection timed out` 작업 실행 중에 발생 {#connection-timed-out-when-running-jobs}

`Application execution failed PID=xxxx error="executing the script on the remote host: executing script on container with IP \"172.x.x.x\": connecting to server: connecting to server \"172.x.x.x:22\" as user \"root\": dial tcp 172.x.x.x:22: connect: connection timed out"`

`EnablePublicIP`이 false로 구성된 경우 VPC 보안 그룹에 SSH 연결을 허용하는 인바운드 규칙이 있는지 확인합니다. AWS Fargate 작업 컨테이너는 GitLab Runner EC2 인스턴스에서의 SSH 트래픽을 수락해야 합니다.

### `connection refused` 작업 실행 중에 발생 {#connection-refused-when-running-jobs}

`Application execution failed PID=xxxx error="executing the script on the remote host: executing script on container with IP \"10.x.x.x\": connecting to server: connecting to server \"10.x.x.x:22\" as user \"root\": dial tcp 10.x.x.x:22: connect: connection refused"`

[6단계에서의 지시사항: ECS 작업 정의 생성](#step-6-create-an-ecs-task-definition)을 기준으로 작업 컨테이너에 포트 22가 노출되어 있고 포트 매핑이 구성되어 있는지 확인합니다. 포트가 노출되고 컨테이너가 구성된 경우:

1. **Amazon ECS > Clusters > Choose your task definition > Tasks**에서 컨테이너에 대한 오류가 있는지 확인합니다.
1. `Stopped` 상태의 작업을 보고 실패한 최신 작업을 확인합니다. **logs** 탭에는 컨테이너 실패가 있는 경우 자세한 정보가 있습니다.

또는 Docker 컨테이너를 로컬에서 실행할 수 있는지 확인합니다.

### 오류: `ssh: unable to authenticate, attempted methods [none publickey], no supported methods remain` {#error-ssh-unable-to-authenticate-attempted-methods-none-publickey-no-supported-methods-remain}

AWS Fargate 드라이버의 이전 버전으로 인해 지원되지 않는 키 유형을 사용하는 경우 다음 오류가 발생합니다.

`Application execution failed PID=xxxx error="executing the script on the remote host: executing script on container with IP \"172.x.x.x\": connecting to server: connecting to server \"172.x.x.x:22\" as user \"root\": ssh: handshake failed: ssh: unable to authenticate, attempted methods [none publickey], no supported methods remain"`

이 문제를 해결하려면 GitLab Runner EC2 인스턴스에서 최신 AWS Fargate 드라이버를 설치합니다:

```shell
sudo curl -Lo /opt/gitlab-runner/fargate "https://gitlab-runner-custom-fargate-downloads.s3.amazonaws.com/latest/fargate-linux-amd64"
sudo chmod +x /opt/gitlab-runner/fargate
```
