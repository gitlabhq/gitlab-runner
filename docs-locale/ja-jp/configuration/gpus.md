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

- GitLab Runner 13.9で導入されました。

{{< /history >}}

GitLab Runnerは、グラフィカルプロセッシングユニット（GPU）の使用をサポートしています。次のセクションでは、さまざまなexecutorでGPUを有効にするために必要な設定について説明します。

## Shell executor {#shell-executor}

Runnerの設定は必要ありません。

## Docker executor {#docker-executor}

前提要件:

- [NVIDIAドライバー](https://docs.nvidia.com/datacenter/tesla/driver-installation-guide/index.html)をインストールします。
- [NVIDIAコンテナToolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html)をインストールします。

`gpus`または`service_gpus`設定オプションを[`runners.docker`セクション](advanced-configuration.md#the-runnersdocker-section)で使用します:

```toml
[runners.docker]
    gpus = "all"
    service_gpus = "all"
```

## Docker Machine executor {#docker-machine-executor}

[Docker MachineのGitLabフォークのドキュメント](../executors/docker_machine.md#using-gpus-on-google-compute-engine)を参照してください。

## Kubernetes executor {#kubernetes-executor}

Runnerの設定は不要です。[ノードセレクターがGPUをサポートするノードを選択している](https://kubernetes.io/docs/tasks/manage-gpus/scheduling-gpus/)ことを確認してください。

GitLab Runnerは、[Amazon Elastic Kubernetes Serviceでテスト](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4355)され、[GPU対応インスタンス](https://docs.aws.amazon.com/dlami/latest/devguide/gpu.html)でテストされています。

## GPUが有効になっていることを検証する {#validate-that-gpus-are-enabled}

NVIDIA GPUでRunnerを使用できます。NVIDIA GPUの場合、CIジョブでGPUが有効になっていることを確認する方法の1つは、スクリプトの先頭で`nvidia-smi`を実行することです。次に例を示します: 

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
