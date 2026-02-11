---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: Docker MachineでのオートスケールのためにGitLab Runnerをインストールして登録する
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

{{< alert type="note" >}}

Docker Machine ExecutorはGitLab 17.5で非推奨となりました。GitLab 20.0（2027年5月）で削除される予定です。GitLab 20.0まではDocker Machine Executorのサポートが継続されますが、新機能を追加する予定はありません。CI/CDジョブの実行を妨げる可能性のある重大なバグ、または実行コストに影響を与えるバグのみに対処します。Amazon Web Services（AWS）EC2、Microsoft Azure Compute、またはGoogle Compute Engine（GCE）でDocker Machine Executorを使用している場合は、[GitLab Runner Autoscaler](../runner_autoscale/_index.md)に移行してください。

{{< /alert >}}

オートスケールアーキテクチャの概要については、[オートスケールに関する包括的なドキュメント](../configuration/autoscale.md)をご覧ください。

## Docker Machineのフォークバージョン {#forked-version-of-docker-machine}

Dockerでは[Docker Machineが非推奨になりました](https://gitlab.com/gitlab-org/gitlab/-/issues/341856)。ただしGitLabでは、Docker Machine executorを利用しているGitLab Runnerユーザーのために[Docker Machineフォーク](https://gitlab.com/gitlab-org/ci-cd/docker-machine)を維持しています。このフォークは、`docker-machine`の最新の`main`ブランチをベースにしており、次のバグに対する追加パッチがいくつか含まれています。

- [DigitalOceanドライバーをRateLimit対応にする](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/merge_requests/2)
- [Googleドライバーオペレーションチェックにバックオフを追加する](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/merge_requests/7)
- [マシン作成のための`--google-min-cpu-platform`オプションを追加する](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/merge_requests/9)
- [キャッシュされているIPをGoogleドライバーに使用する](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/merge_requests/15)
- [キャッシュされているIPをAWSドライバーに使用する](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/merge_requests/14)
- [Google Compute EngineでGPUを使用するためのサポートを追加する](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/merge_requests/48)
- [IMDSv2でAWSインスタンスを実行するためのサポートを追加する](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/merge_requests/49)

[Docker Machineフォーク](https://gitlab.com/gitlab-org/ci-cd/docker-machine)の目的は、実行コストに影響を与える重大な問題とバグのみを修正することです。新しい機能を追加する予定はありません。

## 環境を準備する {#preparing-the-environment}

オートスケール機能を使用するには、DockerとGitLab Runnerが同じマシンにインストールされている必要があります。

1. 踏み台サーバーとして機能できる新しいLinuxベースのマシンにサインインします。この踏み台サーバーでDockerが新しいマシンを作成します。
1. [GitLab Runnerをインストールします](../install/_index.md)。
1. [Docker Machineフォーク](https://gitlab.com/gitlab-org/ci-cd/docker-machine)からDocker Machineをインストールします。
1. オプションですが、オートスケールされたRunnerで使用する[プロキシコンテナレジストリとキャッシュサーバー](../configuration/speed_up_job_execution.md)を準備することを推奨します。

## GitLab Runnerを設定する {#configuring-gitlab-runner}

1. `docker-machine`と`gitlab-runner`を使用するという基本的な概念を理解します。
   - [GitLab Runnerのオートスケール](../configuration/autoscale.md)を読みます
   - [GitLab Runner MachineOptions](../configuration/advanced-configuration.md#the-runnersmachine-section)を読みます
1. Docker Machineを**初めて**使用する場合は、[Docker Machineドライバー](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/tree/main/drivers)を指定した`docker-machine create ...`コマンドを手動で実行する方法が最良の方法です。`[runners.machine]`セクションの[MachineOptions](../configuration/advanced-configuration.md#the-runnersmachine-section)で設定するオプションを使用して、このコマンドを実行します。この手法ではDocker Machine環境が適切に設定され、指定されたオプションが検証されます。その後に`docker-machine rm [machine_name]`でマシンを破棄し、Runnerを起動できます。

   {{< alert type="note" >}}

   **最初の使用時**に実行される`docker-machine create`に対する複数の同時リクエストは、適切ではありません。`docker+machine` executorが使用されている場合、Runnerはいくつかの同時`docker-machine create`コマンドを起動することがあります。Docker Machineがこの環境に初めて導入される場合、各プロセスはDocker API認証のためのSSHキーとSSL証明書の作成を試行します。この動作が原因で、同時実行プロセスが互いに干渉します。これにより、動作しない環境になる可能性があります。そのため、Docker MachineでGitLab Runnerを初めてセットアップするときには、テストマシンを手動で作成することが重要です。

   1. [Runnerを登録](../register/_index.md)し、要求されたら`docker+machine` executorを選択します。
   1. [`config.toml`](../commands/_index.md#configuration-file)を編集し、Docker Machineを使用するようにRunnerを設定します。[GitLab Runner](../configuration/autoscale.md)オートスケールに関する詳細情報を記載した専用ページを参照してください。
   1. これで、プロジェクトでパイプラインを新規作成して開始できます。数秒後に`docker-machine ls`を実行すると、新しいマシンが作成されていることがわかります。

   {{< /alert >}}

## GitLab Runnerをアップグレードする {#upgrading-gitlab-runner}

1. ご使用のオペレーティングシステムがGitLab Runnerを自動的に再起動するように設定されているかどうかを確認します（たとえば、そのサービスファイルを確認します）。
   - **設定されている**場合は、サービスマネージャーが[`SIGQUIT`を使用するように設定されている](../configuration/init.md)ことを確認し、サービスツールを使用してプロセスを停止します。

     ```shell
     # For systemd
     sudo systemctl stop gitlab-runner

     # For upstart
     sudo service gitlab-runner stop
     ```

   - **設定されていない**場合は、プロセスを手動で停止できます。

     ```shell
     sudo killall -SIGQUIT gitlab-runner
     ```

   {{< alert type="note" >}}

   [`SIGQUIT`シグナル](../commands/_index.md#signals)を送信すると、プロセスが正常に停止します。プロセスは新しいジョブの受け入れを停止し、現在のジョブが完了すると直ちに終了します。

   {{< /alert >}}

1. GitLab Runnerが終了するまで待ちます。`gitlab-runner status`でその状態を確認するか、正常なシャットダウンが行われるまで最大30分間待つことができます。

   ```shell
   for i in `seq 1 180`; do # 1800 seconds = 30 minutes
       gitlab-runner status || break
       sleep 10
   done
   ```

1. これで、ジョブを中断することなく、新しいバージョンのGitLab Runnerを安全にインストールできます。

## Docker Machineのフォークバージョンを使用する {#using-the-forked-version-of-docker-machine}

### インストール {#install}

1. [適切な`docker-machine`バイナリ](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/releases)をダウンロードします。`PATH`がアクセスできる場所にバイナリをコピーし、実行可能にします。たとえば、`v0.16.2-gitlab.43`をダウンロードしてインストールするには、次のようにします。

   ```shell
   curl -O "https://gitlab-docker-machine-downloads.s3.amazonaws.com/v0.16.2-gitlab.43/docker-machine-Linux-x86_64"
   cp docker-machine-Linux-x86_64 /usr/local/bin/docker-machine
   chmod +x /usr/local/bin/docker-machine
   ```

### Google Compute EngineでGPUを使用する {#using-gpus-on-google-compute-engine}

{{< alert type="note" >}}

GPUは[すべてのexecutorでサポートされています](../configuration/gpus.md)。GPUサポートのためだけにDocker Machineを使用する必要はありません。Docker Machine ExecutorはGPUノードをスケールアップおよびスケールダウンします。この目的で[Kubernetes executor](kubernetes/_index.md)を使用することもできます。

{{< /alert >}}

Docker Machine[フォーク](#forked-version-of-docker-machine)を使用して、[GPU（グラフィックスプロセッシングユニット）を使用するGoogle Compute Engineインスタンス](https://docs.cloud.google.com/compute/docs/gpus)を作成できます。

#### Docker Machine GPUオプション {#docker-machine-gpu-options}

GPUを使用するインスタンスを作成するには、次のDocker Machineオプションを使用します。

| オプション                        | 例                        | 説明 |
|-------------------------------|--------------------------------|-------------|
| `--google-accelerator`        | `type=nvidia-tesla-p4,count=1` | インスタンスにアタッチするGPUアクセラレータのタイプと数を指定します（`type=TYPE,count=N`形式）。 |
| `--google-maintenance-policy` | `TERMINATE`                    | [Google CloudではGPUインスタンスのライブ移行が許可されていない](https://docs.cloud.google.com/compute/docs/instances/live-migration-process)ため、常に`TERMINATE`を使用してください。 |
| `--google-machine-image`      | `https://www.googleapis.com/compute/v1/projects/deeplearning-platform-release/global/images/family/tf2-ent-2-3-cu110` | GPU対応オペレーティングシステムのURL。[使用可能なイメージのリスト](https://docs.cloud.google.com/deep-learning-vm/docs/images)を参照してください。 |
| `--google-metadata`           | `install-nvidia-driver=True`   | このフラグは、NVIDIA GPUドライバーをインストールするようにイメージに指示します。 |

これらの引数は、[`gcloud compute`のコマンドライン引数](https://docs.cloud.google.com/compute/docs/gcloud-compute)にマップされます。詳細については、[GPUがアタッチされたVMの作成に関するGoogleドキュメント](https://docs.cloud.google.com/compute/docs/gpus/create-vm-with-gpus)を参照してください。

#### Docker Machineオプションを検証する {#verifying-docker-machine-options}

システムを準備し、Google Compute EngineでGPUを作成できることをテストするには、次の手順に従います:

1. Docker Machineの[Google Compute Engineドライバー認証情報をセットアップ](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/gce.md#credentials)します。場合によっては、VMにデフォルトのサービスアカウントがないときに環境変数をRunnerにエクスポートする必要があります。その方法は、Runnerの起動方法によって異なります。たとえば、次のいずれかを使用します。

   - `systemd`または`upstart`: [カスタム環境変数の設定に関するドキュメント](../configuration/init.md#setting-custom-environment-variables)を参照してください。
   - Helmチャートを使用したKubernetes: [`values.yaml`エントリ](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/blob/5e7c5c0d6e1159647d65f04ff2cc1f45bb2d5efc/values.yaml#L431-438)を更新します。
   - Docker: `-e`オプションを使用します（`docker run -e GOOGLE_APPLICATION_CREDENTIALS=/path/to/credentials.json gitlab/gitlab-runner`など）。

1. 必要なオプションを指定した`docker-machine`が仮想マシンを作成できることを確認します。たとえば、1つのNVIDIA Tesla P4アクセラレータを備えた`n1-standard-1`マシンを作成するには、`test-gpu`を名前で置き換えて、次のように実行します。

   ```shell
   docker-machine create --driver google --google-project your-google-project \
     --google-disk-size 50 \
     --google-machine-type n1-standard-1 \
     --google-accelerator type=nvidia-tesla-p4,count=1 \
     --google-maintenance-policy TERMINATE \
     --google-machine-image https://www.googleapis.com/compute/v1/projects/deeplearning-platform-release/global/images/family/tf2-ent-2-3-cu110 \
     --google-metadata "install-nvidia-driver=True" test-gpu
   ```

1. GPUがアクティブであることを確認するには、マシンにSSHで接続し、`nvidia-smi`を実行します。

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

1. 費用を節約するために、このテストインスタンスを削除します。

   ```shell
   docker-machine rm test-gpu
   ```

#### GitLab Runnerを設定する {#configuring-gitlab-runner-1}

1. これらのオプションを検証したら、[`runners.docker`設定](../configuration/advanced-configuration.md#the-runnersdocker-section)で使用可能なすべてのGPUを使用するようにDocker executorを設定します。次に、[GitLab Runner `runners.machine`設定の`MachineOptions`設定](../configuration/advanced-configuration.md#the-runnersmachine-section)にDocker Machineオプションを追加します。例: 

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

## トラブルシューティング {#troubleshooting}

Docker Machine executorを使用するときに次の問題が発生する可能性があります。

### エラー: マシンの作成エラー {#error-error-creating-machine}

Docker Machineをインストールするときに、`ERROR: Error creating machine: Error running provisioning: error installing docker`というエラーが発生することがあります。

Docker Machineは次のスクリプトを使用して、新しくプロビジョニングされた仮想マシンへのDockerのインストールを試行します。

```shell
if ! type docker; then curl -sSL "https://get.docker.com" | sh -; fi
```

`docker`コマンドが成功した場合、Docker MachineはDockerがインストールされたとみなして続行します。

成功しなかった場合、Docker Machineは`https://get.docker.com`でスクリプトをダウンロードして実行しようとします。インストールが失敗する場合は、オペレーティングシステムがDockerでサポートされなくなった可能性があります。

この問題を解決するには、GitLab Runnerがインストールされている環境で`MACHINE_DEBUG=true`を設定して、Docker Machineでデバッグを有効にできます。

### エラー: Dockerデーモンに接続できない {#error-cannot-connect-to-the-docker-daemon}

ジョブは、準備段階で次のエラーメッセージで失敗することがあります。

```plaintext
Preparing environment
ERROR: Job failed (system failure): prepare environment: Cannot connect to the Docker daemon at tcp://10.200.142.223:2376. Is the docker daemon running? (docker.go:650:120s). Check https://docs.gitlab.com/runner/shells/#shell-profile-loading for more information
```

このエラーは、Docker Machine executorによって作成されたVMで、Dockerデーモンが予期されている時間内に起動できなかった場合に発生します。この問題を修正するには、[`[runners.docker]`](../configuration/advanced-configuration.md#the-runnersdocker-section)セクションの`wait_for_services_timeout`の値を大きくします。
