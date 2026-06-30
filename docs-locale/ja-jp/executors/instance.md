---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: インスタンスexecutor
---

{{< history >}}

- GitLab Runner 15.11.0で[実験的機能](https://docs.gitlab.com/policy/development_stages_support/#experiment)として導入されました。
- GitLab Runner 16.6で[ベータ](https://docs.gitlab.com/policy/development_stages_support/#beta)に[変更](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29404)されました。
- GitLab Runner 17.1で[一般提供](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29221)になりました。

{{< /history >}}

インスタンスexecutorは、オートスケールが有効なexecutorで、Runnerマネージャーが処理するジョブの予想される量に対応するために、オンデマンドでインスタンスを作成します。

ジョブがホストインスタンス、オペレーティングシステム、および接続されたデバイスへのフルアクセスを必要とする場合、インスタンスexecutorを使用できます。インスタンスexecutorは、さまざまなレベルの分離とセキュリティを備えたシングルテナントおよびマルチテナントのジョブに対応するように設定することもできます。

## ネストされた仮想化 {#nested-virtualization}

インスタンスexecutorは、GitLabが開発した[ネストされたデーモン](https://gitlab.com/gitlab-org/fleeting/nesting)によるネストされた仮想化をサポートします。ネストされたデーモンは、ジョブのような分離された短期間のワークロードに使用されるホストシステム上で、事前設定された仮想マシンの作成と削除を可能にします。ネストはApple Siliconインスタンスでのみサポートされています。

## オートスケールのために環境を準備する {#prepare-the-environment-for-autoscaling}

オートスケールのために環境を準備するには:

1. [Fleetingプラグインをインストール](../fleet_scaling/fleeting.md#install-a-fleeting-plugin)して、Runnerマネージャーがインストールされ、設定されているターゲットプラットフォームで利用できるようにします。
1. 使用しているプラットフォーム用のVMイメージを作成します。イメージには以下を含める必要があります:
   - Git
   - Runnerバイナリ

     > [!note]
     > ジョブアーティファクトキャッシュを処理するには、Runnerバイナリを仮想マシンにインストールし、Runner実行可能ファイルをデフォルトのパスに保持します。VMイメージはRunnerの実行を必要としません。VMイメージを使用して起動されたインスタンスを、GitLabにRunnerとして登録しないようにしてください。

   - 実行する予定のジョブに必要な依存関係

## executorをオートスケールするように設定する {#configure-the-executor-to-autoscale}

前提条件: 

- 管理者である必要があります。

インスタンスexecutorをオートスケールのために設定するには、`config.toml`で次のセクションを更新します:

- [`[runners.autoscaler]`](../configuration/advanced-configuration.md#the-runnersautoscaler-section)
- [`[runners.instance]`](../configuration/advanced-configuration.md#the-runnersinstance-section)

## プリエンプティブモード {#preemptive-mode}

fleetingとtaskscalerを使用すると:

- オンにすると、Runnerマネージャーはアイドルインスタンスが利用可能になるまで新しいCI/CDジョブをリクエストしません。このモードでは、CI/CDジョブはほぼ即座に実行されます。
- プリエンプティブモードがオフの場合、Runnerマネージャーは、アイドルインスタンスがこれらのジョブを実行できるかどうかに関わらず、新しいCI/CDジョブをリクエストします。ジョブの数は`max_instances`と`capacity_per_instance`に基づいています。このモードでは、CI/CDジョブの開始時間が遅くなります。新しいインスタンスをプロビジョニングすることができず、CI/CDジョブが実行されない可能性があります。

## AWSオートスケールグループの設定例 {#aws-autoscaling-group-configuration-examples}

### 1インスタンスあたり1ジョブ {#one-job-per-instance}

前提条件: 

- 少なくとも`git`とRunnerがインストールされたAMI。
- AWS Auto Scalingグループ。スケールポリシーには`none`を使用します。Runnerがスケーリングを処理します。
- IAMポリシーと[適切な権限](https://gitlab.com/gitlab-org/fleeting/plugins/aws#recommended-iam-policy)。

この設定では以下がサポートされています:

- 各インスタンスの容量は`1`です。
- 使用回数は`1`です。
- アイドルスケールは`5`です。
- アイドル時間は20分です。
- 最大インスタンス数は`10`です。

容量と使用回数を`1`に設定すると、各ジョブには他のジョブの影響を受けない安全な一時的なインスタンスが与えられます。ジョブが完了すると、実行されたインスタンスはすぐに削除されます。

各インスタンスの容量が`1`で、アイドルスケールが`5`の場合、Runnerは将来の需要に備えて5つのインスタンス全体を保持します。これらのインスタンスは少なくとも20分間維持されます。

Runnerの`concurrent`フィールドは10に設定されています（最大インスタンス数 * 1インスタンスあたりの容量）。

```toml
concurrent = 10

[[runners]]
  name = "instance autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  shell = "sh"

  executor = "instance"

  # Autoscaler config
  [runners.autoscaler]
    plugin = "aws" # in GitLab 16.11 and later, ensure you run `gitlab-runner fleeting install` to automatically install the plugin

    # in GitLab 16.10 and earlier, manually install the plugin and use:
    # plugin = "fleeting-plugin-aws"

    capacity_per_instance = 1
    max_use_count = 1
    max_instances = 10

    [runners.autoscaler.plugin_config] # plugin specific configuration (see plugin documentation)
      name             = "my-linux-asg"                # AWS Autoscaling Group name
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

### 無制限の使用回数で1インスタンスあたり5つのジョブ {#five-jobs-per-instance-with-unlimited-uses}

前提条件: 

- 少なくとも`git`とRunnerがインストールされたAMI。
- AWSオートスケールグループのスケールポリシーは`none`に設定されています。Runnerがスケーリングを処理します。
- IAMポリシーと[適切な権限](https://gitlab.com/gitlab-org/fleeting/plugins/aws#recommended-iam-policy)。

この設定では以下がサポートされています:

- 各インスタンスの容量は`5`です。
- 無制限の使用回数。
- アイドルスケールは`5`です。
- アイドル時間は20分です。
- 最大インスタンス数は`10`です。

1インスタンスあたりの容量を`5`に設定し、使用回数を無制限にすると、各インスタンスはインスタンスのライフタイム全体で5つのジョブを同時に実行します。

アイドルスケールが`5`で、インスタンスのアイドル容量が`5`の場合、使用中の容量が5を下回るたびに1つのアイドルインスタンスが作成されます。アイドルインスタンスは少なくとも20分間維持されます。

これらの環境で実行されるジョブは、それらの間にほとんど分離がなく、各ジョブが他のジョブのパフォーマンスに影響を与える可能性があるため、**信頼できるもの**である必要があります。

Runnerの`concurrent`フィールドは50に設定されています（最大インスタンス数 * 1インスタンスあたりの容量）。

```toml
concurrent = 50

[[runners]]
  name = "instance autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  shell = "sh"

  executor = "instance"

  # Autoscaler config
  [runners.autoscaler]
    plugin = "aws" # in GitLab 16.11 and later, ensure you run `gitlab-runner fleeting install` to automatically install the plugin

    # in GitLab 16.10 and earlier, manually install the plugin and use:
    # plugin = "fleeting-plugin-aws"

    capacity_per_instance = 5
    max_use_count = 0
    max_instances = 10

    [runners.autoscaler.plugin_config] # plugin specific configuration (see plugin documentation)
      name             = "my-windows-asg"              # AWS Autoscaling Group name
      profile          = "default"                     # optional, default is 'default'
      config_file      = "/home/user/.aws/config"      # optional, default is '~/.aws/config'
      credentials_file = "/home/user/.aws/credentials" # optional, default is '~/.aws/credentials'

    [runners.autoscaler.connector_config]
      username          = "Administrator"
      timeout           = "5m0s"
      use_external_addr = true

    [[runners.autoscaler.policy]]
      idle_count = 5
      idle_time = "20m0s"
```

### 1インスタンスあたり2ジョブ、無制限の使用、EC2 Macインスタンスでのネストされた仮想化 {#two-jobs-per-instance-unlimited-uses-nested-virtualization-on-ec2-mac-instances}

前提条件: 

- [ネスト](https://gitlab.com/gitlab-org/fleeting/nesting)と[Tart](https://github.com/openai/tart)がインストールされたApple Silicon AMI。
- Runnerが使用するTart VMイメージ。VMイメージは、ジョブの`image`キーワードで指定されます。VMイメージには、少なくとも`git`とRunnerがインストールされている必要があります。
- AWS Auto Scalingグループ。スケールポリシーには`none`を使用します。これはRunnerがスケールを処理するためです。MacOS用のASGを設定する方法については、[EC2 Macインスタンス向けオートスケールの実装](https://aws.amazon.com/blogs/compute/implementing-autoscaling-for-ec2-mac-instances/)を参照してください。
- [適切な権限](https://gitlab.com/gitlab-org/fleeting/plugins/aws#recommended-iam-policy)が設定されたIAMポリシー。

この設定では以下がサポートされています:

- 各インスタンスの容量は`2`です。
- 無制限の使用回数。
- 分離されたジョブをサポートするためのネストされた仮想化。ネストされた仮想化は、[ネスト](https://gitlab.com/gitlab-org/fleeting/nesting)がインストールされたApple Siliconインスタンスでのみ利用可能です。
- アイドルスケールは`5`です。
- アイドル時間は20分です。
- 最大インスタンス数は`10`です。

各インスタンスの容量が`2`で、使用回数が無制限の場合、各インスタンスはインスタンスのライフタイム全体で2つのジョブを同時に実行します。

アイドルスケールが`2`の場合、使用中の容量が`2`を下回るたびに1つのアイドルインスタンスが作成されます。アイドルインスタンスは少なくとも24時間維持されます。この期間は、AWS MacOSインスタンスホストの24時間最小割り当て期間によるものです。

この環境で実行されるジョブは、各ジョブのネストされた仮想化に[ネスト](https://gitlab.com/gitlab-org/fleeting/nesting)が使用されているため、信頼する必要はありません。これはApple siliconインスタンスでのみ機能します。

Runnerの`concurrent`フィールドは8に設定されています（最大インスタンス数 * 1インスタンスあたりの容量）。

```toml
concurrent = 8

[[runners]]
  name = "macos applesilicon autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  executor = "instance"

  [runners.instance]
    allowed_images = ["*"] # allow any nesting image

  [runners.autoscaler]
    capacity_per_instance = 2 # AppleSilicon can only support 2 VMs per host
    max_use_count = 0
    max_instances = 4

    plugin = "aws" # in GitLab 16.11 and later, ensure you run `gitlab-runner fleeting install` to automatically install the plugin

    # in GitLab 16.10 and earlier, manually install the plugin and use:
    # plugin = "fleeting-plugin-aws"

    [[runners.autoscaler.policy]]
      idle_count = 2
      idle_time  = "24h" # AWS's MacOS instances

    [runners.autoscaler.connector_config]
      username = "ec2-user"
      key_path = "macos-key.pem"
      timeout  = "1h" # connecting to a MacOS instance can take some time, as they can be slow to provision

    [runners.autoscaler.plugin_config]
      name = "mac2metal"
      region = "us-west-2"

    [runners.autoscaler.vm_isolation]
      enabled = true
      nesting_host = "unix:///Users/ec2-user/Library/Application Support/nesting.sock"

    [runners.autoscaler.vm_isolation.connector_config]
      username = "nested-vm-username"
      password = "nested-vm-password"
      timeout  = "20m"
```

## Google Cloudインスタンスグループの設定例 {#google-cloud-instance-group-configuration-examples}

### Google Cloudインスタンスグループを使用した1インスタンスあたり1ジョブ {#one-job-per-instance-using-a-google-cloud-instance-group}

前提条件: 

- 少なくとも`git`とRunnerがインストールされたカスタムイメージ。
- オートスケールモードが`do not autoscale`に設定されているGoogle Cloudインスタンスグループ。Runnerがスケーリングを処理します。
- [適切な権限](https://gitlab.com/gitlab-org/fleeting/plugins/googlecloud#required-permissions)が設定されたIAMポリシー。GKEクラスターにRunnerをデプロイする場合は、KubernetesサービスアカウントとGCPサービスアカウントの間にIAMバインディングを追加できます。`credentials_file`でキーファイルを使用する代わりに、`iam.workloadIdentityUser`ロールでこのバインディングを追加し、GCPに対して認証できます。

この設定では以下がサポートされています:

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
  name = "instance autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  shell = "sh"

  executor = "instance"

  # Autoscaler config
  [runners.autoscaler]
    plugin = "googlecloud" # for >= 16.11, ensure you run `gitlab-runner fleeting install` to automatically install the plugin

    # for versions < 17.0, manually install the plugin and use:
    # plugin = "fleeting-plugin-googlecompute"

    capacity_per_instance = 1
    max_use_count = 1
    max_instances = 10

    [runners.autoscaler.plugin_config] # plugin specific configuration (see plugin documentation)
      name             = "my-linux-instance-group" # Google Cloud Instance Group name
      project          = "my-gcp-project"
      zone             = "europe-west1-c"
      credentials_file = "/home/user/.config/gcloud/application_default_credentials.json" # optional, default is '~/.config/gcloud/application_default_credentials.json'

    [runners.autoscaler.connector_config]
      username          = "runner"
      use_external_addr = true

    [[runners.autoscaler.policy]]
      idle_count = 5
      idle_time = "20m0s"
```

### Google Cloudインスタンスグループを使用した、1インスタンスあたり5ジョブ、無制限の使用 {#five-jobs-per-instance-unlimited-uses-using-google-cloud-instance-group}

前提条件: 

- 少なくとも`git`とRunnerがインストールされたカスタムイメージ。
- インスタンスグループ。「オートスケールモード」には「オートスケールしない」を選択してください。これはRunnerがスケールを処理するためです。
- IAMポリシーと[適切な権限](https://gitlab.com/gitlab-org/fleeting/plugins/googlecloud#required-permissions)。

この設定では以下がサポートされています:

- 1インスタンスあたりの容量は5です。
- 無制限の使用回数
- アイドルスケール: 5
- アイドル時間: 20分
- インスタンスの最大数: 10

容量を`5`に設定し、使用回数を無制限にすると、各インスタンスはインスタンスのライフタイム全体で5つのジョブを同時に実行します。

これらの環境で実行されるジョブは、それらの間にほとんど分離がなく、各ジョブが他のジョブのパフォーマンスに影響を与える可能性があるため、**信頼できるもの**である必要があります。

アイドルスケールが`5`の場合、使用中の容量が`5`を下回るたびに1つのアイドルインスタンスが作成されます。アイドルインスタンスは少なくとも20分間維持されます。

Runnerの`concurrent`フィールドは50に設定されています（最大インスタンス数 * 1インスタンスあたりの容量）。

```toml
concurrent = 50

[[runners]]
  name = "instance autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  shell = "sh"

  executor = "instance"

  # Autoscaler config
  [runners.autoscaler]
    plugin = "googlecloud" # for >= 16.11, ensure you run `gitlab-runner fleeting install` to automatically install the plugin

    # for versions < 17.0, manually install the plugin and use:
    # plugin = "fleeting-plugin-googlecompute"

    capacity_per_instance = 5
    max_use_count = 0
    max_instances = 10

    [runners.autoscaler.plugin_config] # plugin specific configuration (see plugin documentation)
      name             = "my-windows-instance-group" # Google Cloud Instance Group name
      project          = "my-gcp-project"
      zone             = "europe-west1-c"
      credentials_file = "/home/user/.config/gcloud/application_default_credentials.json" # optional, default is '~/.config/gcloud/application_default_credentials.json'

    [runners.autoscaler.connector_config]
      username          = "Administrator"
      timeout           = "5m0s"
      use_external_addr = true

    [[runners.autoscaler.policy]]
      idle_count = 5
      idle_time = "20m0s"
```

## Azureスケールセットの設定例 {#azure-scale-set-configuration-examples}

### Azureスケールセットを使用した1インスタンスあたり1ジョブ {#one-job-per-instance-using-an-azure-scale-set}

前提条件: 

- 少なくとも`git`とRunnerがインストールされたカスタムイメージ。
- オートスケールモードが`manual`に設定されており、オーバープロビジョニングがオフになっているAzureスケールセット。Runnerがスケーリングを処理します。

この設定では以下がサポートされています:

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
  name = "instance autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  shell = "sh"

  executor = "instance"

  # Autoscaler config
  [runners.autoscaler]
    plugin = "azure" # for >= 16.11, ensure you run `gitlab-runner fleeting install` to automatically install the plugin

    # for versions < 17.0, manually install the plugin and use:
    # plugin = "fleeting-plugin-azure"

    capacity_per_instance = 1
    max_use_count = 1
    max_instances = 10

    [runners.autoscaler.plugin_config] # plugin specific configuration (see plugin documentation)
      name                = "my-linux-scale-set" # Azure scale set name
      subscription_id     = "9b3c4602-cde2-4089-bed8-889e5a3e7102"
      resource_group_name = "my-resource-group"

    [runners.autoscaler.connector_config]
      username               = "runner"
      password               = "my-scale-set-static-password"
      use_static_credentials = true
      timeout                = "10m"
      use_external_addr      = true

    [[runners.autoscaler.policy]]
      idle_count = 5
      idle_time  = "20m0s"
```

### Azureスケールセットを使用した、1インスタンスあたり5ジョブ、無制限の使用 {#five-jobs-per-instance-unlimited-uses-using-an-azure-scale-set}

前提条件: 

- 少なくとも`git`とRunnerがインストールされたカスタムイメージ。
- オートスケールモードが`manual`に設定されており、オーバープロビジョニングがオフになっているAzureスケールセット。Runnerがスケーリングを処理します。

この設定では以下がサポートされています:

- 1インスタンスあたりの容量は5です。
- 無制限の使用回数
- アイドルスケール: 5
- アイドル時間: 20分
- インスタンスの最大数: 10

容量を`5`に設定し、使用回数を無制限にすると、各インスタンスはインスタンスのライフタイム全体で5つのジョブを同時に実行します。

これらの環境で実行されるジョブは、それらの間にほとんど分離がなく、各ジョブが他のジョブのパフォーマンスに影響を与える可能性があるため、**信頼できるもの**である必要があります。

アイドルスケールが`2`の場合、使用中の容量が`5`を下回るたびに1つのアイドルインスタンスが作成されます。アイドルインスタンスは少なくとも20分間維持されます。

Runnerの`concurrent`フィールドは50に設定されています（最大インスタンス数 * 1インスタンスあたりの容量）。

```toml
concurrent = 50

[[runners]]
  name = "instance autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  shell = "sh"

  executor = "instance"

  # Autoscaler config
  [runners.autoscaler]
    plugin = "azure" # for >= 16.11, ensure you run `gitlab-runner fleeting install` to automatically install the plugin

    # for versions < 17.0, manually install the plugin and use:
    # plugin = "fleeting-plugin-azure"

    capacity_per_instance = 5
    max_use_count = 0
    max_instances = 10

    [runners.autoscaler.plugin_config] # plugin specific configuration (see plugin documentation)
      name                = "my-windows-scale-set" # Azure scale set name
      subscription_id     = "9b3c4602-cde2-4089-bed8-889e5a3e7102"
      resource_group_name = "my-resource-group"

    [runners.autoscaler.connector_config]
      username               = "Administrator"
      password               = "my-scale-set-static-password"
      use_static_credentials = true
      timeout                = "10m"
      use_external_addr      = true

    [[runners.autoscaler.policy]]
      idle_count = 5
      idle_time = "20m0s"
```

## スロットベースのcgroupのサポート {#slot-based-cgroup-support}

インスタンスexecutorは、同時実行ジョブ間のリソース分離を改善するために、スロットベースのcgroupをサポートしています。有効にすると、`GITLAB_RUNNER_SLOT_CGROUP`環境変数がジョブに自動的に提供され、スロット固有のcgroupでプロセスを実行できるようになります。

スロットベースのcgroupに関する詳細情報（利点、前提条件、設定、セットアップ手順を含む）については、[スロットベースのcgroupサポート](../configuration/slot_based_cgroups.md)を参照してください。

### Runnerスロットcgroup環境変数の使用 {#using-the-gitlab-runner-slot-cgroup-environment-variable}

インスタンスexecutorは、`GITLAB_RUNNER_SLOT_CGROUP`環境変数をジョブに提供します。この変数を`systemd-run`や`cgexec`のようなツールと組み合わせて使用し、スロット固有のcgroupでプロセスを実行します。

使用例とトラブルシューティングについては、スロットベースcgroupドキュメントの[インスタンスexecutorセクション](../configuration/slot_based_cgroups.md#instance-executor)を参照してください。

## トラブルシューティング {#troubleshooting}

インスタンスexecutorを使用する際、次の問題が発生する可能性があります:

### `sh: 1: eval: Running on ip-x.x.x.x via runner-host...n: not found` {#sh-1-eval-running-on-ip-xxxx-via-runner-hostn-not-found}

このエラーは通常、準備ステップの`eval`コマンドが失敗したときに発生します。このエラーを解決するには、`bash` Shellに切り替えて、[機能フラグ](../configuration/feature-flags.md) `FF_USE_NEW_BASH_EVAL_STRATEGY`を有効にします。
