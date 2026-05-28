---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Docker Machine을 사용하여 자동 크기 조정을 위해 GitLab 러너 설치 및 등록
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

> [!warning]
> Docker Machine 실행기는 GitLab 17.5에서 더 이상 사용되지 않으며 GitLab 20.0(2027년 5월)에서 제거될 예정입니다. GitLab 20.0까지 Docker Machine 실행기를 계속 지원하지만 새로운 기능을 추가할 계획은 없습니다. CI/CD 작업 실행을 방해하거나 실행 비용에 영향을 미칠 수 있는 중요한 버그만 해결합니다. Amazon Web Services (AWS) EC2, Microsoft Azure Compute 또는 Google Compute Engine (GCE)에서 Docker Machine 실행기를 사용 중인 경우 [GitLab Runner Autoscaler](../runner_autoscale/_index.md)로 마이그레이션해야 합니다.

Docker Machine 실행기는 자동 크기 조정 지원이 있는 Docker 실행기의 특수 버전입니다. 일반적인 Docker 실행기처럼 작동하지만 빌드 호스트는 Docker Machine에 의해 온디맨드로 생성됩니다. 이는 AWS EC2와 같은 클라우드 환경에서 효과적이며, 변수 워크로드에 대해 좋은 격리 및 확장성을 제공합니다.

자동 크기 조정 아키텍처에 대한 개요를 보려면 [자동 크기 조정에 대한 포괄적인 문서](../configuration/autoscale.md)를 확인하세요.

## Docker Machine의 포크 버전 {#forked-version-of-docker-machine}

Docker는 [Docker Machine을 더 이상 사용하지 않습니다](https://gitlab.com/gitlab-org/gitlab/-/issues/341856). 그러나 GitLab은 Docker Machine 실행기를 사용하는 러너 사용자를 위해 [Docker Machine 포크](https://gitlab.com/gitlab-org/ci-cd/docker-machine)를 유지합니다. 이 포크는 다음 버그에 대한 추가 패치와 함께 `main` 브랜치의 최신 `docker-machine`를 기반으로 합니다:

- [DigitalOcean 드라이버를 RateLimit 인식 가능하게 만들기](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/merge_requests/2)
- [Google 드라이버 작업 확인에 백오프 추가](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/merge_requests/7)
- [머신 생성을 위해 `--google-min-cpu-platform` 옵션 추가](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/merge_requests/9)
- [Google 드라이버용 캐시된 IP 사용](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/merge_requests/15)
- [AWS 드라이버용 캐시된 IP 사용](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/merge_requests/14)
- [Google Compute Engine에서 GPU 사용 지원 추가](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/merge_requests/48)
- [IMDSv2를 사용하여 AWS 인스턴스 실행 지원](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/merge_requests/49)

[Docker Machine 포크](https://gitlab.com/gitlab-org/ci-cd/docker-machine)의 목적은 실행 비용에 영향을 미치는 중요한 문제 및 버그만 수정하는 것입니다. 새로운 기능을 추가할 계획은 없습니다.

## 환경 준비 {#preparing-the-environment}

자동 크기 조정 기능을 사용하려면 Docker와 러너를 같은 머신에 설치해야 합니다:

1. Docker가 새로운 머신을 생성할 수 있는 배스천 서버 역할을 할 수 있는 새로운 Linux 기반 머신에 로그인합니다.
1. [러너 설치](../install/_index.md)합니다.
1. [Docker Machine 포크](https://gitlab.com/gitlab-org/ci-cd/docker-machine)에서 Docker Machine을 설치합니다.
1. 선택 사항이지만 권장하는 사항으로, [프록시 컨테이너 레지스트리 및 캐시 서버](../configuration/speed_up_job_execution.md)를 준비하여 자동 크기 조정되는 러너와 함께 사용합니다.

## GitLab 러너 구성 {#configuring-gitlab-runner}

1. `docker-machine`을 `gitlab-runner`와 함께 사용하는 핵심 개념을 숙지합니다:
   - [GitLab 러너 자동 크기 조정](../configuration/autoscale.md) 읽기
   - [GitLab 러너 MachineOptions](../configuration/advanced-configuration.md#the-runnersmachine-section) 읽기
1. **first time**으로 Docker Machine을 사용할 때는 `docker-machine create ...` 명령을 사용자의 [Docker Machine 드라이버](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/tree/main/drivers)로 수동으로 실행하는 것이 가장 좋습니다. `[runners.machine]` 섹션 아래에서 구성할 의도가 있는 옵션으로 이 명령을 실행하고 [MachineOptions](../configuration/advanced-configuration.md#the-runnersmachine-section)에서 실행합니다. 이 방식은 Docker Machine 환경을 적절히 설정하고 지정된 옵션을 검증합니다. 이후에 `docker-machine rm [machine_name]`으로 머신을 삭제하고 러너를 시작할 수 있습니다.

   > [!note]
   > `docker-machine create`에 대한 여러 동시 요청이 **at first usage** 완료되는 것은 좋지 않습니다. `docker+machine` 실행기를 사용하면 러너는 몇 개의 동시 `docker-machine create` 명령을 실행할 수 있습니다. Docker Machine이 이 환경에 새로운 경우 각 프로세스는 Docker API 인증을 위해 SSH 키와 SSL 인증서를 생성하려고 합니다. 이 작업으로 인해 동시 프로세스가 서로 방해합니다. 이는 작동하지 않는 환경으로 끝날 수 있습니다. 따라서 Docker Machine으로 GitLab 러너를 처음 설정할 때 테스트 머신을 수동으로 생성하는 것이 중요합니다.

   1. [러너 등록](../register/_index.md)하고 물었을 때 `docker+machine` 실행기를 선택합니다.
   1. [`config.toml`](../commands/_index.md#configuration-file)를 편집하고 Docker Machine을 사용하도록 러너를 구성합니다. [GitLab 러너 자동 크기 조정](../configuration/autoscale.md)에 대한 자세한 정보를 다루는 전용 페이지를 방문합니다.
   1. 이제 프로젝트에서 새로운 파이프라인을 시작해 볼 수 있습니다. 몇 초 후에 `docker-machine ls`을 실행하면 새로운 머신이 생성되는 것을 볼 수 있습니다.

## GitLab 러너 업그레이드 {#upgrading-gitlab-runner}

1. 운영 체제가 GitLab 러너를 자동으로 다시 시작하도록 구성되어 있는지 확인합니다(예: 서비스 파일을 확인하여):
   - **if yes** 서비스 관리자가 [`SIGQUIT` 사용하도록 구성](../configuration/init.md)되었는지 확인하고 서비스의 도구를 사용하여 프로세스를 중지합니다:

     ```shell
     # For systemd
     sudo systemctl stop gitlab-runner

     # For upstart
     sudo service gitlab-runner stop
     ```

   - **if no** 프로세스를 수동으로 중지할 수 있습니다:

     ```shell
     sudo killall -SIGQUIT gitlab-runner
     ```

   [`SIGQUIT` 신호](../commands/_index.md#signals)를 보내면 프로세스가 정상적으로 중지됩니다. 프로세스는 새로운 작업을 수락하지 않고 현재 작업이 완료되는 즉시 종료됩니다.

1. GitLab 러너가 종료될 때까지 기다립니다. `gitlab-runner status`로 상태를 확인하거나 최대 30분 동안 정상 종료를 기다릴 수 있습니다:

   ```shell
   for i in `seq 1 180`; do # 1800 seconds = 30 minutes
       gitlab-runner status || break
       sleep 10
   done
   ```

1. 이제 작업을 중단하지 않고 안전하게 새로운 버전의 GitLab 러너를 설치할 수 있습니다.

## Docker Machine의 포크 버전 사용 {#using-the-forked-version-of-docker-machine}

### 설치 {#install}

1. [적절한 `docker-machine` 바이너리](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/releases)를 다운로드합니다. 바이너리를 `PATH`에 액세스 가능한 위치에 복사하고 실행 가능하게 만듭니다. 예를 들어 `v0.16.2-gitlab.46`를 다운로드 및 설치하려면:

   ```shell
   curl -O "https://gitlab-docker-machine-downloads.s3.amazonaws.com/v0.16.2-gitlab.46/docker-machine-Linux-x86_64"
   cp docker-machine-Linux-x86_64 /usr/local/bin/docker-machine
   chmod +x /usr/local/bin/docker-machine
   ```

### Google Compute Engine에서 GPU 사용 {#using-gpus-on-google-compute-engine}

> [!note]
> GPU는 [모든 실행기에서 지원](../configuration/gpus.md)됩니다. GPU 지원만을 위해 Docker Machine을 사용할 필요는 없습니다. Docker Machine 실행기는 GPU 노드를 확장 및 축소합니다. 이 목적으로 [Kubernetes 실행기](kubernetes/_index.md)를 사용할 수도 있습니다.

Docker Machine [포크](#forked-version-of-docker-machine) 를 사용하여 [GPU(그래픽 처리 장치)가 있는 Google Compute Engine 인스턴스](https://docs.cloud.google.com/compute/docs/gpus)를 생성할 수 있습니다.

#### Docker Machine GPU 옵션 {#docker-machine-gpu-options}

GPU가 있는 인스턴스를 생성하려면 다음 Docker Machine 옵션을 사용합니다:

| 옵션                        | 예제                        | 설명 |
|-------------------------------|--------------------------------|-------------|
| `--google-accelerator`        | `type=nvidia-tesla-p4,count=1` | 인스턴스에 연결할 GPU 가속기의 유형과 수를 지정합니다(`type=TYPE,count=N` 형식) |
| `--google-maintenance-policy` | `TERMINATE`                    | `TERMINATE`를 항상 사용합니다. [Google Cloud는 GPU 인스턴스의 라이브 마이그레이션을 허용하지 않습니다](https://docs.cloud.google.com/compute/docs/instances/live-migration-process). |
| `--google-machine-image`      | `https://www.googleapis.com/compute/v1/projects/deeplearning-platform-release/global/images/family/tf2-ent-2-3-cu110` | GPU 지원 운영 체제의 URL입니다. [사용 가능한 이미지 목록](https://docs.cloud.google.com/deep-learning-vm/docs/images)을 참조하세요. |
| `--google-metadata`           | `install-nvidia-driver=True`   | 이 플래그는 NVIDIA GPU 드라이버를 설치하도록 이미지에 지시합니다. |

이 인수는 [`gcloud compute`의 명령줄 인수](https://docs.cloud.google.com/compute/docs/gcloud-compute)에 매핑됩니다. [연결된 GPU가 있는 VM 생성에 대한 Google 문서](https://docs.cloud.google.com/compute/docs/gpus/create-vm-with-gpus)를 참조하여 자세한 내용을 확인하세요.

#### Docker Machine 옵션 검증 {#verifying-docker-machine-options}

Google Compute Engine으로 GPU를 생성할 수 있는지 시스템을 준비하고 테스트하려면:

1. [Google Compute Engine 드라이버 자격 증명 설정](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/gce.md#credentials)을 Docker Machine용으로 수행합니다. VM에 기본 서비스 계정이 없는 경우 환경 변수를 러너로 내보내야 할 수도 있습니다. 이를 수행하는 방식은 러너 시작 방식에 따라 다릅니다. 예를 들어 다음을 사용합니다:

   - `systemd` 또는 `upstart`:  [사용자 정의 환경 변수 설정 문서](../configuration/init.md#setting-custom-environment-variables)를 참조하세요.
   - Helm Chart를 사용하는 Kubernetes:  [`values.yaml` 항목](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/blob/5e7c5c0d6e1159647d65f04ff2cc1f45bb2d5efc/values.yaml#L431-438)을 업데이트합니다.
   - Docker:  `-e` 옵션(예: `docker run -e GOOGLE_APPLICATION_CREDENTIALS=/path/to/credentials.json gitlab/gitlab-runner`)을 사용합니다.

1. `docker-machine`이 원하는 옵션으로 가상 머신을 생성할 수 있는지 확인합니다. 예를 들어 단일 NVIDIA Tesla P4 가속기가 있는 `n1-standard-1` 머신을 생성하려면 `test-gpu`을 이름으로 바꾸고 실행합니다:

   ```shell
   docker-machine create --driver google --google-project your-google-project \
     --google-disk-size 50 \
     --google-machine-type n1-standard-1 \
     --google-accelerator type=nvidia-tesla-p4,count=1 \
     --google-maintenance-policy TERMINATE \
     --google-machine-image https://www.googleapis.com/compute/v1/projects/deeplearning-platform-release/global/images/family/tf2-ent-2-3-cu110 \
     --google-metadata "install-nvidia-driver=True" test-gpu
   ```

1. GPU가 활성 상태인지 확인하려면 머신에 SSH로 접속하고 `nvidia-smi`을 실행합니다:

   ```shell
   $ docker-machine ssh test-gpu sudo nvidia-smi
   +-----------------------------------------------------------------------------+
   | NVIDIA-SMI 450.51.06    Driver Version: 450.51.06    CUDA Version: 11.0     |
   |-------------------------------+----------------------+----------------------+
   | GPU  Name        Persistence-M| Bus-Id        Disp.A | Volatile Uncorr. ECC |
   | Fan  Temp  Perf  Pwr:Usage/Cap|         Memory-Usage | GPU-Util  Compute M. |
   |                               |                      |               MIG M. |
   |===============================+======================+======================|
   |   0  Tesla P4            Off  | 00000000:00:04.0 Off |                    0 |
   | N/A   43C    P0    22W /  75W |      0MiB /  7611MiB |      3%      Default |
   |                               |                      |                  N/A |
   +-------------------------------+----------------------+----------------------+

   +-----------------------------------------------------------------------------+
   | Processes:                                                                  |
   |  GPU   GI   CI        PID   Type   Process name                  GPU Memory |
   |        ID   ID                                                   Usage      |
   |=============================================================================|
   |  No running processes found                                                 |
   +-----------------------------------------------------------------------------+
   ```

1. 비용을 절약하기 위해 이 테스트 인스턴스를 제거합니다:

   ```shell
   docker-machine rm test-gpu
   ```

#### GitLab 러너 구성 {#configuring-gitlab-runner-1}

1. 이 옵션을 검증한 후 [`runners.docker` 구성](../configuration/advanced-configuration.md#the-runnersdocker-section)에서 모든 사용 가능한 GPU를 사용하도록 Docker 실행기를 구성합니다. 그런 다음 Docker Machine 옵션을 [GitLab 러너 `runners.machine` 구성의 `MachineOptions` 설정](../configuration/advanced-configuration.md#the-runnersmachine-section)에 추가합니다. 예를 들어:

   ```toml
   [runners.docker]
     gpus = "all"
   [runners.machine]
     MachineOptions = [
       "google-project=your-google-project",
       "google-disk-size=50",
       "google-disk-type=pd-ssd",
       "google-machine-type=n1-standard-1",
       "google-accelerator=count=1,type=nvidia-tesla-p4",
       "google-maintenance-policy=TERMINATE",
       "google-machine-image=https://www.googleapis.com/compute/v1/projects/deeplearning-platform-release/global/images/family/tf2-ent-2-3-cu110",
       "google-metadata=install-nvidia-driver=True"
     ]
   ```

## 문제 해결 {#troubleshooting}

Docker Machine 실행기로 작업할 때 다음 문제가 발생할 수 있습니다.

### 오류:  머신 생성 오류 {#error-error-creating-machine}

Docker Machine을 설치할 때 `ERROR: Error creating machine: Error running provisioning: error installing docker`라는 오류가 발생할 수 있습니다.

Docker Machine은 이 스크립트를 사용하여 새로 프로비저닝된 가상 머신에 Docker를 설치하려고 합니다:

```shell
if ! type docker; then curl -sSL "https://get.docker.com" | sh -; fi
```

`docker` 명령이 성공하면 Docker Machine은 Docker가 설치되었다고 가정하고 계속합니다.

성공하지 못하면 Docker Machine은 `https://get.docker.com`의 스크립트를 다운로드하여 실행하려고 합니다. 설치가 실패하면 운영 체제가 더 이상 Docker에서 지원되지 않을 가능성이 있습니다.

이 문제를 해결하려면 `MACHINE_DEBUG=true`을 GitLab 러너가 설치된 환경에서 설정하여 Docker Machine에서 디버깅을 활성화할 수 있습니다.

### 오류:  Docker 데몬에 연결할 수 없음 {#error-cannot-connect-to-the-docker-daemon}

준비 단계 중에 작업이 오류 메시지와 함께 실패할 수 있습니다:

```plaintext
Preparing environment
ERROR: Job failed (system failure): prepare environment: Cannot connect to the Docker daemon at tcp://10.200.142.223:2376. Is the docker daemon running? (docker.go:650:120s). Check https://docs.gitlab.com/runner/shells/#shell-profile-loading for more information
```

이 오류는 Docker Machine 실행기에서 생성된 VM에서 Docker 데몬이 예상된 시간에 시작되지 못할 때 발생합니다. 이 문제를 해결하려면 [`[runners.docker]`](../configuration/advanced-configuration.md#the-runnersdocker-section) 섹션에서 `wait_for_services_timeout` 값을 늘립니다.
