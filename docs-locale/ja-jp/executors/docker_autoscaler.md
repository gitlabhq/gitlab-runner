---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: Docker Autoscaler executor
---

{{< history >}}

- GitLab Runner 15.11.0で[実験](https://docs.gitlab.com/policy/development_stages_support/#experiment)として導入されました。
- GitLab Runner 16.6で[ベータ](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29404)に[変更](https://docs.gitlab.com/policy/development_stages_support/#beta)されました。
- GitLab Runner 17.1で[一般提供](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29221)になりました。

{{< /history >}}

Docker Autoscaler executorを使用する前に、一連の既知のイシューについて、GitLab Runner自動スケールに関する[フィードバックイシュー](https://gitlab.com/gitlab-org/gitlab/-/issues/408131)を参照してください。

Docker Autoscaler executorは、Runnerマネージャーが処理するジョブに対処するために、オンデマンドでインスタンスを作成する自動スケール対応のDocker executorです。[Docker executor](docker.md)をラップしているため、すべてのDocker executorのオプションと機能がサポートされています。

Docker Autoscalerは、[フリートプラグイン](https://gitlab.com/gitlab-org/fleeting/fleeting)を使用して自動スケールします。フリートとは、自動スケールされたインスタンスのグループの抽象化であり、Google Cloud、AWS、Azureなどのクラウドプロバイダーをサポートするプラグインを使用します。

## フリートプラグインをインストールする

ご使用のターゲットプラットフォームに対応するプラグインをインストールするには、[フリートプラグインをインストールする](../fleet_scaling/fleeting.md#install-a-fleeting-plugin)を参照してください。

## Docker Autoscalerを設定する

Docker Autoscaler executorは[Docker executor](docker.md)をラップしているため、すべてのDocker executorオプションと機能がサポートされています。

Docker Autoscalerを設定するには、`config.toml`で以下のように設定します。

- [`[runners]`](../configuration/advanced-configuration.md#the-runners-section)セクションで`executor`を`docker-autoscaler`として指定します。
- 以下のセクションで、要件に基づいてDocker Autoscalerを設定します。
  - [`[runners.docker]`](../configuration/advanced-configuration.md#the-runnersdocker-section)
  - [`[runners.autoscaler]`](../configuration/advanced-configuration.md#the-runnersautoscaler-section)

### 各Runner設定の専用自動スケールグループ

各Docker Autoscaler設定には、それぞれに専用の自動スケールリソースが必要です。

- AWSでは専用の自動スケールグループ
- GCPでは専用のインスタンスグループ
- Azureでは専用のスケールセット

これらの自動スケールリソースを以下の要素間で共有しないでください。

- 複数のRunnerマネージャー（個別のGitLab Runnerインストール）
- 同じRunnerマネージャーの`config.toml`内の複数の`[[runners]]`エントリ

Docker Autoscalerは、クラウドプロバイダーの自動スケールリソースと同期する必要があるインスタンスの状態を追跡します。複数のシステムが同じ自動スケールリソースを管理しようとすると、競合するスケーリングコマンドが発行され、予測できない動作、ジョブの失敗、および高い可能性があるコストが発生する可能性があります。

### 例:インスタンスあたり1つのジョブに対するAWS自動スケール

前提要件:

- [Docker Engine](https://docs.docker.com/engine/)がインストールされたAMI。RunnerマネージャーがAMI上のDockerソケットにアクセスできるようにするには、ユーザーが`docker`グループに所属している必要があります。
- AWS自動スケールグループ。Runnerがスケーリングを処理するため、スケーリングポリシーには「none」を使用します。インスタンスのスケールイン保護を有効にします。
- [適切な権限](https://gitlab.com/gitlab-org/fleeting/plugins/aws#recommended-iam-policy)を持つIAMポリシー

この設定では以下がサポートされています。

- インスタンスあたりのキャパシティ: 1
- 使用回数: 1
- アイドルスケール: 5
- アイドル時間: 20分
- インスタンスの最大数: 10

キャパシティと使用回数を両方とも1に設定することで、各ジョブに、他のジョブの影響を受けない安全な一時インスタンスが与えられます。ジョブが完了すると即時に、ジョブが実行されていたインスタンスが削除されます。

アイドルスケールが5の場合、Runnerは将来の需要に備えて5つのインスタンス全体を維持しようとします（インスタンスあたりのキャパシティが1であるため）。これらのインスタンスは少なくとも20分間維持されます。

Runnerの`concurrent`フィールドは10（インスタンスの最大数*インスタンスあたりのキャパシティ）に設定されます。

```toml
concurrent = 10

[[runners]]
  name = "docker autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  shell = "sh"                                        # use powershell or pwsh for Windows AMIs

  # uncomment for Windows AMIs when the Runner manager is hosted on Linux
  # environment = ["FF_USE_POWERSHELL_PATH_RESOLVER=1"]

  executor = "docker-autoscaler"

  # Docker Executor config
  [runners.docker]
    image = "busybox:latest"

  # Autoscaler config
  [runners.autoscaler]
    plugin = "aws" # in GitLab 16.11 and later, ensure you run `gitlab-runner fleeting install` to automatically install the plugin

    # in GitLab 16.10 and earlier, manually install the plugin and use:
    # plugin = "fleeting-plugin-aws"

    capacity_per_instance = 1
    max_use_count = 1
    max_instances = 10

    [runners.autoscaler.plugin_config] # plugin specific configuration (see plugin documentation)
      name             = "my-docker-asg"               # AWS Autoscaling Group name
      profile          = "default"                     # optional, default is 'default'
      config_file      = "/home/user/.aws/config"      # optional, default is '~/.aws/config'
      credentials_file = "/home/user/.aws/credentials" # optional, default is '~/.aws/credentials'

    [runners.autoscaler.connector_config]
      username          = "ec2-user"
      use_external_addr = true

    [[runners.autoscaler.policy]]
      idle_count = 5
      idle_time = "20m0s"
```

### 例:インスタンスあたり1つのジョブに対するGoogle Cloudインスタンスグループ

前提要件:

- [Docker Engine](https://docs.docker.com/engine/)がインストールされたVMイメージ（[`COS`](https://cloud.google.com/container-optimized-os/docs)など）。
- Google Cloudインスタンスグループ。**Autoscaling mode**で**Do not autoscale**を選択します。Runnerが自動スケールを処理し、Google Cloudインスタンスグループは処理しません。
- [適切な権限](https://gitlab.com/gitlab-org/fleeting/plugins/googlecloud#required-permissions)が設定されたIAMポリシー。GKEクラスターにRunnerをデプロイする場合は、KubernetesサービスアカウントとGCPサービスアカウントの間にIAMバインディングを追加できます。`credentials_file`でキーファイルを使用する代わりに、`iam.workloadIdentityUser`ロールでこのバインディングを追加し、GCPに対して認証できます。

この設定では以下がサポートされています。

- インスタンスあたりのキャパシティ: 1
- 使用回数: 1
- アイドルスケール: 5
- アイドル時間: 20分
- インスタンスの最大数: 10

キャパシティと使用回数を両方とも1に設定することで、各ジョブに、他のジョブの影響を受けない安全な一時インスタンスが与えられます。ジョブが完了すると即時に、ジョブが実行されていたインスタンスが削除されます。

アイドルスケールが5の場合、Runnerは将来の需要に備えて5つのインスタンス全体を維持しようとします（インスタンスあたりのキャパシティが1であるため）。これらのインスタンスは少なくとも20分間維持されます。

Runnerの`concurrent`フィールドは10（インスタンスの最大数*インスタンスあたりのキャパシティ）に設定されます。

```toml
concurrent = 10

[[runners]]
  name = "docker autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  shell = "sh"                                        # use powershell or pwsh for Windows Images

  # uncomment for Windows Images when the Runner manager is hosted on Linux
  # environment = ["FF_USE_POWERSHELL_PATH_RESOLVER=1"]

  executor = "docker-autoscaler"

  # Docker Executor config
  [runners.docker]
    image = "busybox:latest"

  # Autoscaler config
  [runners.autoscaler]
    plugin = "googlecloud" # for >= 16.11, ensure you run `gitlab-runner fleeting install` to automatically install the plugin

    # for versions < 17.0, manually install the plugin and use:
    # plugin = "fleeting-plugin-googlecompute"

    capacity_per_instance = 1
    max_use_count = 1
    max_instances = 10

    [runners.autoscaler.plugin_config] # plugin specific configuration (see plugin documentation)
      name             = "my-docker-instance-group" # Google Cloud Instance Group name
      project          = "my-gcp-project"
      zone             = "europe-west1"
      credentials_file = "/home/user/.config/gcloud/application_default_credentials.json" # optional, default is '~/.config/gcloud/application_default_credentials.json'

    [runners.autoscaler.connector_config]
      username          = "runner"
      use_external_addr = true

    [[runners.autoscaler.policy]]
      idle_count = 5
      idle_time = "20m0s"
```

### 例:インスタンスあたり1つのジョブに対するAzureスケールセット

前提要件:

- [Docker Engine](https://docs.docker.com/engine/)がインストールされたAzure VMイメージ。
- 自動スケールポリシーが`manual`に設定されているAzureスケールセット。Runnerがスケーリングを処理します。

この設定では以下がサポートされています。

- インスタンスあたりのキャパシティ: 1
- 使用回数: 1
- アイドルスケール: 5
- アイドル時間: 20分
- インスタンスの最大数: 10

キャパシティと使用回数が両方とも`1`に設定されている場合、各ジョブに、他のジョブの影響を受けない安全な一時インスタンスが与えられます。ジョブが完了すると、ジョブが実行されたインスタンスが直ちに削除されます。

アイドルスケールが`5`に設定されている場合、Runnerは将来の需要に備えて5つのインスタンスを維持します（インスタンスあたりのキャパシティが1であるため）。これらのインスタンスは少なくとも20分間維持されます。

Runnerの`concurrent`フィールドは10（インスタンスの最大数*インスタンスあたりのキャパシティ）に設定されます。

```toml
concurrent = 10

[[runners]]
  name = "docker autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  shell = "sh"                                        # use powershell or pwsh for Windows AMIs

  # uncomment for Windows AMIs when the Runner manager is hosted on Linux
  # environment = ["FF_USE_POWERSHELL_PATH_RESOLVER=1"]

  executor = "docker-autoscaler"

  # Docker Executor config
  [runners.docker]
    image = "busybox:latest"

  # Autoscaler config
  [runners.autoscaler]
    plugin = "azure" # for >= 16.11, ensure you run `gitlab-runner fleeting install` to automatically install the plugin

    # for versions < 17.0, manually install the plugin and use:
    # plugin = "fleeting-plugin-azure"

    capacity_per_instance = 1
    max_use_count = 1
    max_instances = 10

    [runners.autoscaler.plugin_config] # plugin specific configuration (see plugin documentation)
      name = "my-docker-scale-set"
      subscription_id = "9b3c4602-cde2-4089-bed8-889e5a3e7102"
      resource_group_name = "my-resource-group"

    [runners.autoscaler.connector_config]
      username = "azureuser"
      password = "my-scale-set-static-password"
      use_static_credentials = true
      timeout = "10m"
      use_external_addr = true

    [[runners.autoscaler.policy]]
      idle_count = 5
      idle_time = "20m0s"
```

## トラブルシューティング

### `ERROR: error during connect: ssh tunnel: EOF ()`

インスタンスが外部ソース（自動スケールグループや自動スクリプトなど）によって削除された場合、ジョブは次のエラーで失敗します。

```plaintext
ERROR: Job failed (system failure): error during connect: Post "http://internal.tunnel.invalid/v1.43/containers/xyz/wait?condition=not-running": ssh tunnel: EOF ()
```

また、GitLab Runnerのログには、ジョブに割り当てられたインスタンスIDの`instance unexpectedly removed`エラーが表示されます。

```plaintext
ERROR: instance unexpectedly removed    instance=<instance_id> max-use-count=9999 runner=XYZ slots=map[] subsystem=taskscaler used=45
```

このエラーを解決するには、クラウドプロバイダープラットフォームでインスタンスに関連するイベントを確認してください。たとえばAWSでは、イベントソース`ec2.amazonaws.com`のCloudTrailイベント履歴を確認します。
