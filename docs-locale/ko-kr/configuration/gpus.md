---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: 그래픽 처리 장치(GPU) 사용
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

{{< history >}}

- GitLab Runner 13.9에서 도입되었습니다.

{{< /history >}}

GitLab Runner는 그래픽 처리 장치(GPU)의 사용을 지원합니다. 다음 섹션에서는 다양한 실행기에 대해 GPU를 활성화하는 데 필요한 구성을 설명합니다.

## Shell 실행기 {#shell-executor}

러너 구성이 필요하지 않습니다.

## Docker 실행기 {#docker-executor}

> [!warning]
> Podman을 컨테이너 런타임 엔진으로 사용하는 경우 GPU가 감지되지 않습니다. 자세한 내용은 [이슈 39095](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/39095)를 참조하세요.

전제 조건:

- [NVIDIA Driver](https://docs.nvidia.com/datacenter/tesla/driver-installation-guide/index.html)를 설치합니다.
- [NVIDIA Container Toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html)을 설치합니다.

`gpus` 또는 `service_gpus` 구성 옵션을 [`runners.docker` 섹션](advanced-configuration.md#the-runnersdocker-section)에서 사용합니다:

```toml
[runners.docker]
    gpus = "all"
    service_gpus = "all"
```

## Docker Machine 실행기 {#docker-machine-executor}

[GitLab fork of Docker Machine에 대한 문서](../executors/docker_machine.md#using-gpus-on-google-compute-engine)를 참조하세요.

## Kubernetes 실행기 {#kubernetes-executor}

전제 조건:

- [노드 선택기가 GPU 지원이 있는 노드를 선택하는지](https://kubernetes.io/docs/tasks/manage-gpus/scheduling-gpus/) 확인합니다.
- `FF_USE_ADVANCED_POD_SPEC_CONFIGURATION` 기능 플래그를 활성화합니다.

GPU 지원을 활성화하려면 러너를 구성하여 pod 사양에서 GPU 리소스를 요청합니다. 예를 들어:

```toml
[[runners.kubernetes.pod_spec]]
  name = "gpu"
  patch = '''
    containers:
    - name: build
      resources:
        requests:
          nvidia.com/gpu: 1
        limits:
          nvidia.com/gpu: 1
  '''
  patch_type = "strategic" # <--- `strategic` patch_type
```

`requests`의 GPU 개수와 `limits`를 작업 요구 사항에 따라 조정합니다.

GitLab Runner는 [Amazon Elastic Kubernetes Service](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4355) 에서 [GPU 지원 인스턴스](https://docs.aws.amazon.com/dlami/latest/devguide/gpu.html)로 테스트되었습니다.

## GPU가 활성화되었는지 검증 {#validate-that-gpus-are-enabled}

NVIDIA GPU가 있는 러너를 사용할 수 있습니다. NVIDIA GPU의 경우 CI 작업에 대해 GPU가 활성화되었는지 확인하는 한 가지 방법은 스크립트의 시작 부분에서 `nvidia-smi`을 실행하는 것입니다. 예를 들어:

```yaml
train:
  script:
    - nvidia-smi
```

GPU가 활성화되면 `nvidia-smi`의 출력에 사용 가능한 디바이스가 표시됩니다. 다음 예제에서는 단일 NVIDIA Tesla P4가 활성화되었습니다:

```shell
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

하드웨어가 GPU를 지원하지 않으면 `nvidia-smi`은 누락되었거나 드라이버와 통신할 수 없기 때문에 실패해야 합니다:

```shell
modprobe: ERROR: could not insert 'nvidia': No such device
NVIDIA-SMI has failed because it couldn't communicate with the NVIDIA driver. Make sure that the latest NVIDIA driver is installed and running.
```
