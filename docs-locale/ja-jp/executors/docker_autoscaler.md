---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: Docker Autoscaler executor
---

{{< history >}}

- GitLab Runner 15.11.0で[実験的機能](https://docs.gitlab.com/policy/development_stages_support/#experiment)として導入されました。
- GitLab Runner 16.6で[ベータ](https://docs.gitlab.com/policy/development_stages_support/#beta)に[変更](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29404)されました。
- GitLab Runner 17.1で[一般提供](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29221)になりました。

{{< /history >}}

Docker Autoscaler executorを使用する前に、一連の既知のイシューについて、GitLab Runnerオートスケールに関する[フィードバックイシュー](https://gitlab.com/gitlab-org/gitlab/-/issues/408131)を参照してください。

Docker Autoscaler executorは、Runnerマネージャーが処理するジョブに対処するために、オンデマンドでインスタンスを作成するオートスケール対応のDocker executorです。[Docker executor](docker.md)をラップしているため、すべてのDocker executorのオプションと機能がサポートされています。

Docker Autoscalerは、[フリートプラグイン](https://gitlab.com/gitlab-org/fleeting/plugins)を使用してオートスケールします。フリートとは、オートスケールされたインスタンスのグループの抽象化であり、Google Cloud、AWS、Azureなどのクラウドプロバイダーをサポートするプラグインを使用します。

## フリートプラグインをインストールする {#install-a-fleeting-plugin}

ご使用のターゲットプラットフォームに対応するプラグインをインストールするには、[フリートプラグインをインストールする](../fleet_scaling/fleeting.md#install-a-fleeting-plugin)を参照してください。具体的な設定について詳しくは、[それぞれのプラグインプロジェクトのドキュメント](https://gitlab.com/gitlab-org/fleeting/plugins)を参照してください。

## Docker Autoscalerを設定する {#configure-docker-autoscaler}

Docker Autoscaler executorは[Docker executor](docker.md)をラップしているため、すべてのDocker executorオプションと機能がサポートされています。

Docker Autoscalerを設定するには、`config.toml`で以下のように設定します。

- [`[runners]`](../configuration/advanced-configuration.md#the-runners-section)セクションで`executor`を`docker-autoscaler`として指定します。
- 以下のセクションで、要件に基づいてDocker Autoscalerを設定します。
  - [`[runners.docker]`](../configuration/advanced-configuration.md#the-runnersdocker-section)
  - [`[runners.autoscaler]`](../configuration/advanced-configuration.md#the-runnersautoscaler-section)

### 各Runner設定の専用オートスケールグループ {#dedicated-autoscaling-groups-for-each-runner-configuration}

各Docker Autoscaler設定には、それぞれに専用のオートスケールリソースが必要です。

- AWSでは専用のオートスケールグループ
- GCPでは専用のインスタンスグループ
- Azureでは専用のスケールセット

これらのオートスケールリソースを以下の要素間で共有しないでください。

- 複数のRunnerマネージャー（個別のGitLab Runnerインストール）
- 同じRunnerマネージャーの`config.toml`内の複数の`[[runners]]`エントリ

Docker Autoscalerは、クラウドプロバイダーのオートスケールリソースと同期する必要があるインスタンスの状態を追跡します。複数のシステムが同じオートスケールリソースを管理しようとすると、競合するスケーリングコマンドが発行され、予測できない動作、ジョブの失敗、および高い可能性があるコストが発生する可能性があります。

### 例: インスタンスあたり1つのジョブに対するAWSオートスケール {#example-aws-autoscaling-for-1-job-per-instance}

前提条件: 

- [Docker Engine](https://docs.docker.com/engine/)がインストールされたAMI。RunnerマネージャーがAMI上のDockerソケットにアクセスできるようにするには、ユーザーが`docker`グループに所属している必要があります。

  {{< alert type="note" >}}

  AMIでは、GitLab Runnerをインストールする必要はありません。AMIを使用して起動されたインスタンスを、GitLabにRunnerとして登録しないようにしてください。

  {{< /alert >}}

- AWSオートスケールグループ。Runnerはすべてのスケール動作を直接管理します。スケーリングポリシーには、`none`を使用し、インスタンススケールイン保護をオンにします。複数のアベイラビリティーゾーンを設定している場合は、`AZRebalance`プロセスをオフにします。
- [適切な権限](https://gitlab.com/gitlab-org/fleeting/plugins/aws#recommended-iam-policy)が設定されたIAMポリシー。

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

### 例: インスタンスあたり1つのジョブに対するGoogle Cloudインスタンスグループ {#example-google-cloud-instance-group-for-1-job-per-instance}

前提条件: 

- [Docker Engine](https://docs.docker.com/engine/)がインストールされたVMイメージ（[`COS`](https://docs.cloud.google.com/container-optimized-os/docs)など）。

  {{< alert type="note" >}}

  VMイメージでは、GitLab Runnerをインストールする必要はありません。VMイメージを使用して起動されたインスタンスを、GitLabにRunnerとして登録しないようにしてください。

  {{< /alert >}}

- シングルゾーンGoogle Cloudインスタンスグループ。**Autoscaling mode**で**Do not autoscale**を選択します。Runnerがオートスケールを処理し、Google Cloudインスタンスグループは処理しません。

  {{< alert type="note" >}}

  現在のところ、マルチゾーンインスタンスグループはサポートされていません。将来マルチゾーンインスタンスグループをサポートするための[イシュー](https://gitlab.com/gitlab-org/fleeting/plugins/googlecloud/-/issues/20)が存在しています。

  {{< /alert >}}

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

### 例: インスタンスあたり1つのジョブに対するAzureスケールセット {#example-azure-scale-set-for-1-job-per-instance}

前提条件: 

- [Docker Engine](https://docs.docker.com/engine/)がインストールされているAzure VMイメージ。

  {{< alert type="note" >}}

  VMイメージでは、GitLab Runnerをインストールする必要はありません。VMイメージを使用して起動されたインスタンスを、GitLabにRunnerとして登録しないようにしてください。

  {{< /alert >}}

- オートスケールポリシーが`manual`に設定されているAzureスケールセット。Runnerがスケーリングを処理します。

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

## スロットベースのcgroupサポート {#slot-based-cgroup-support}

Docker Autoscaler executorは、同時実行ジョブ間のリソース分離を改善するために、スロットベースのcgroupをサポートしています。Cgroupパスは、`--cgroup-parent`フラグを使用して、Dockerコンテナに自動的に適用されます。

利点、前提条件、設定手順など、スロットベースのcgroupの詳細については、[slot-based cgroup support](../configuration/slot_based_cgroups.md)を参照してください。

### Docker固有の設定 {#docker-specific-configuration}

標準のスロットcgroup設定に加えて、サービコンテナ用に個別のcgroupテンプレートを指定できます:

```toml
[[runners]]
  executor = "docker+autoscaler"
  use_slot_cgroups = true
  slot_cgroup_template = "gitlab-runner/slot-${slot}"

  [runners.docker]
    service_slot_cgroup_template = "gitlab-runner/service-slot-${slot}"
```

利用可能なすべてのオプションについては、[slot-based cgroup configuration documentation](../configuration/slot_based_cgroups.md#docker-specific-configuration)を参照してください。

## トラブルシューティング {#troubleshooting}

### `ERROR: error during connect: ssh tunnel: EOF ()` {#error-error-during-connect-ssh-tunnel-eof-}

インスタンスが外部ソース（オートスケールグループや自動スクリプトなど）によって削除された場合、ジョブは次のエラーで失敗します。

```plaintext
ERROR: Job failed (system failure): error during connect: Post "http://internal.tunnel.invalid/v1.43/containers/xyz/wait?condition=not-running": ssh tunnel: EOF ()
```

また、GitLab Runnerのログには、ジョブに割り当てられたインスタンスIDの`instance unexpectedly removed`エラーが表示されます。

```plaintext
ERROR: instance unexpectedly removed    instance=<instance_id> max-use-count=9999 runner=XYZ slots=map[] subsystem=taskscaler used=45
```

このエラーを解決するには、クラウドプロバイダープラットフォームでインスタンスに関連するイベントを確認してください。たとえばAWSでは、イベントソース`ec2.amazonaws.com`のCloudTrailイベント履歴を確認します。

### `ERROR: Preparation failed: unable to acquire instance: context deadline exceeded` {#error-preparation-failed-unable-to-acquire-instance-context-deadline-exceeded}

[AWSフリートプラグイン](https://gitlab.com/gitlab-org/fleeting/plugins/aws)を使用している場合、ジョブが失敗して次のエラーになることが断続的に発生する可能性があります。

```plaintext
ERROR: Preparation failed: unable to acquire instance: context deadline exceeded
```

`reserved`のインスタンス数が変動するため、多くの場合、これはAWS CloudWatchのログの中に示されます。

```plaintext
"2024-07-23T18:10:24Z","instance_count:1,max_instance_count:1000,acquired:0,unavailable_capacity:0,pending:0,reserved:0,idle_count:0,scale_factor:0,scale_factor_limit:0,capacity_per_instance:1","required scaling change",
"2024-07-23T18:10:25Z","instance_count:1,max_instance_count:1000,acquired:0,unavailable_capacity:0,pending:0,reserved:1,idle_count:0,scale_factor:0,scale_factor_limit:0,capacity_per_instance:1","required scaling change",
"2024-07-23T18:11:15Z","instance_count:1,max_instance_count:1000,acquired:0,unavailable_capacity:0,pending:0,reserved:0,idle_count:0,scale_factor:0,scale_factor_limit:0,capacity_per_instance:1","required scaling change",
"2024-07-23T18:11:16Z","instance_count:1,max_instance_count:1000,acquired:0,unavailable_capacity:0,pending:0,reserved:1,idle_count:0,scale_factor:0,scale_factor_limit:0,capacity_per_instance:1","required scaling change",
```

このエラーを解決するには、AWSでオートスケールグループに対して`AZRebalance`プロセスが無効になっていることを確認してください。
