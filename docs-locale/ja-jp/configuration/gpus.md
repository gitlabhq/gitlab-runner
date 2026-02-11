---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: グラフィカルプロセッシングユニット（GPU）の使用
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

{{< history >}}

- GitLab Runner 13.9で導入。

{{< /history >}}

GitLab Runnerは、グラフィカルプロセッシングユニット（GPU）の使用をサポートしています。次のセクションでは、さまざまなexecutorに対してGPUを有効にするために必要な設定について説明します。

## Shell executor {#shell-executor}

必要なRunnerの設定はありません。

## Docker executor {#docker-executor}

{{< alert type="warning" >}}

Podmanをコンテナのランタイムエンジンとして使用している場合、GPUは検出されません。詳細については、[issue 39095](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/39095)を参照してください。

{{< /alert >}}

前提条件: 

- [NVIDIAドライバー](https://docs.nvidia.com/datacenter/tesla/driver-installation-guide/index.html)をインストールします。
- [NVIDIAコンテナツールキット](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html)をインストールします。

[`runners.docker`セクション](advanced-configuration.md#the-runnersdocker-section)で、`gpus`または`service_gpus`の設定オプションを使用します:

```toml
[runners.docker]
    gpus = "all"
    service_gpus = "all"
```

## Docker Machine executor {#docker-machine-executor}

[Docker MachineのGitLabフォークのドキュメント](../executors/docker_machine.md#using-gpus-on-google-compute-engine)を参照してください。

## Kubernetes executor {#kubernetes-executor}

前提条件: 

- [ノードセレクターがGPUをサポートするノードを選択](https://kubernetes.io/docs/tasks/manage-gpus/scheduling-gpus/)していることを確認してください。
- `FF_USE_ADVANCED_POD_SPEC_CONFIGURATION`機能フラグを有効にします。

GPUサポートを有効にするには、ポッドの仕様でGPUリソースをリクエストするようにRunnerを設定します。例: 

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

ジョブの要件に基づいて、`requests`および`limits`のGPU数を調整します。

GitLab Runnerは、[Amazon Elastic Kubernetes Serviceでテスト](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4355)されており、[GPU対応のインスタンス](https://docs.aws.amazon.com/dlami/latest/devguide/gpu.html)を備えています。

## GPUが有効になっていることを検証する {#validate-that-gpus-are-enabled}

NVIDIA GPUでRunnerを使用できます。NVIDIA GPUの場合、CIジョブに対してGPUが有効になっていることを確認する方法の1つは、スクリプトの先頭で`nvidia-smi`を実行することです。例: 

```yaml
train:
  script:
    - nvidia-smi
```

GPUが有効になっている場合、`nvidia-smi`の出力には、使用可能なデバイスが表示されます。次の例では、単一のNVIDIA Tesla P4が有効になっています:

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

ハードウェアがGPUをサポートしていない場合、`nvidia-smi`が見つからないか、ドライバーと通信できないため、失敗するはずです:

```shell
modprobe: ERROR: could not insert 'nvidia': No such device
NVIDIA-SMI has failed because it couldn't communicate with the NVIDIA driver. Make sure that the latest NVIDIA driver is installed and running.
```
