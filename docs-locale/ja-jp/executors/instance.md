---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: インスタンスexecutor
---

{{< history >}}

- GitLab Runner 15.11.0で[実験的機能](https://docs.gitlab.com/policy/development_stages_support/#experiment)として導入されました。
- GitLab Runner 16.6で[ベータ](https://docs.gitlab.com/policy/development_stages_support/#beta)に[変更](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29404)されました。
- GitLab Runner 17.1で[一般提供](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29221)になりました。

{{< /history >}}

インスタンスexecutorは、Runnerマネージャーが処理するジョブの予期されるボリュームに対応するために、オンデマンドでインスタンスを作成するオートスケール対応のexecutorです。

ジョブがホストインスタンス、オペレーティングシステム、および接続デバイスへのフルアクセスを必要とする場合は、インスタンスexecutorを使用できます。インスタンスエグゼキューターは、さまざまなレベルの分離とセキュリティを備えたシングルテナントおよびマルチテナントジョブに対応するように構成することもできます。

## ネストされた仮想化 {#nested-virtualization}

インスタンスエグゼキューターは、GitLabが開発した[ネスティングデーモン](https://gitlab.com/gitlab-org/fleeting/nesting)を使用したネストされた仮想化をサポートしています。ネスティングデーモンを使用すると、ジョブのように、分離された短期間のワークロードに使用されるホストシステム上で、事前構成された仮想マシンの作成と削除ができます。ネストは、Apple Siliconインスタンスでのみサポートされています。

## オートスケールの環境を準備します {#prepare-the-environment-for-autoscaling}

オートスケールの環境を準備するには、次のようにします:

1. Runnerマネージャーがインストールおよび構成されているターゲットプラットフォーム用の[Fleetingプラグインをインストール](../fleet_scaling/fleeting.md#install-a-fleeting-plugin)します。
1. 使用しているプラットフォームのVMイメージを作成します。イメージには以下を含める必要があります:
   - Git
   - GitLab Runnerバイナリ

    {{< alert type="note" >}}

    ジョブのアーティファクトとキャッシュを処理するには、仮想マシンにGitLab Runnerバイナリをインストールし、Runner実行可能ファイルをデフォルトのパスに保持します。VMイメージでは、GitLab Runnerをインストールする必要はありません。VMイメージを使用して起動されたインスタンスを、GitLabにRunnerとして登録しないようにしてください。

    {{< /alert >}}

   - 実行する予定のジョブに必要な依存関係

## オートスケールするようにエグゼキューターを構成します {#configure-the-executor-to-autoscale}

前提要件:

- 管理者である必要があります。

オートスケールを行うようにインスタンスエグゼキューターを構成するには、`config.toml`の次のセクションを更新します:

- [`[runners.autoscaler]`](../configuration/advanced-configuration.md#the-runnersautoscaler-section)
- [`[runners.instance]`](../configuration/advanced-configuration.md#the-runnersinstance-section)

## プリエンプティブモード {#preemptive-mode}

FleetingとTaskscalerを使用する場合:

- オンにすると、Runnerマネージャーは、アイドル状態のインスタンスが使用可能になるまで、新しいCI/CDジョブをリクエストしません。このモードでは、CI/CDジョブはほぼすぐに実行されます。
- プリエンプティブモードがオフになっている場合、Runnerマネージャーは、アイドル状態のインスタンスがそれらのジョブを実行できるかどうかに関係なく、新しいCI/CDジョブをリクエストします。ジョブの数は、`max_instances`と`capacity_per_instance`に基づいています。このモードでは、CI/CDジョブの開始時間が遅くなります。新しいインスタンスをプロビジョニングできない場合があり、CI/CDジョブが実行されない可能性があります。

## AWSオートスケールグループ構成の例 {#aws-autoscaling-group-configuration-examples}

### インスタンスごとのジョブ数1 {#one-job-per-instance}

前提要件:

- 少なくとも`git`とGitLab RunnerがインストールされたAMI。
- AWS Auto Scalingグループ。スケールポリシーには`none`を使用します。Runnerがスケーリングを処理します。
- [適切な権限](https://gitlab.com/gitlab-org/fleeting/plugins/aws#recommended-iam-policy)が設定されたIAMポリシー。

この設定では以下がサポートされています:

- 各インスタンスの`1`の容量。
- 使用回数: `1`。
- アイドルスケール: `5`。
- アイドル時間: 20分。
- インスタンスの最大数: `10`。

キャパシティと使用回数が両方とも`1`に設定されている場合、各ジョブに、他のジョブの影響を受けない安全な一時的なインスタンスが与えられます。ジョブが完了すると、ジョブが実行されたインスタンスが直ちに削除されます。

各インスタンスの容量が`1`で、アイドルスケールが`5`の場合、Runnerは将来の需要に備えて5つのインスタンス全体を保持します。これらのインスタンスは、少なくとも20分間は残ります。

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

### 無制限の用途でインスタンスあたり5つのジョブ {#five-jobs-per-instance-with-unlimited-uses}

前提要件:

- 少なくとも`git`とGitLab RunnerがインストールされたAMI。
- スケールポリシーが`none`に設定されたAWSオートスケールグループ。Runnerがスケーリングを処理します。
- [適切な権限](https://gitlab.com/gitlab-org/fleeting/plugins/aws#recommended-iam-policy)が設定されたIAMポリシー。

この設定では以下がサポートされています:

- 各インスタンスの`5`の容量。
- 無制限の使用回数。
- アイドルスケール: `5`。
- アイドル時間: 20分。
- インスタンスの最大数: `10`。

インスタンスあたりの容量を`5`に設定し、使用回数を無制限にすると、各インスタンスはインスタンスのライフタイム全体で5つのジョブを同時に実行します。

アイドルスケールが`5`で、インスタンスのアイドル容量が`5`の場合、使用中の容量が5を下回ると、アイドルインスタンスが1つ作成されます。アイドルインスタンスは、少なくとも20分間は残ります。

これらの環境で実行されるジョブは、**信頼**されている必要があります。それらの間にはほとんど分離がなく、各ジョブが別のジョブのパフォーマンスに影響を与える可能性があるためです。

Runnerの`concurrent`フィールドは50（インスタンスの最大数*インスタンスあたりのキャパシティ）に設定されます。

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

### インスタンスあたり2つのジョブ、無制限の使用、EC2 Macインスタンスでのネストされた仮想化 {#two-jobs-per-instance-unlimited-uses-nested-virtualization-on-ec2-mac-instances}

前提要件:

- [ネスティング](https://gitlab.com/gitlab-org/fleeting/nesting)と[Tart](https://github.com/cirruslabs/tart)がインストールされたApple Silicon AMI。
- Runnerが使用するTart VMイメージ。VMイメージは、ジョブの`image`キーワードで指定されます。VMイメージには、少なくとも`git`とGitLab Runnerがインストールされている必要があります。
- AWS Auto Scalingグループ。Runnerがスケールを処理するため、スケーリングポリシーには`none`を使用します。MacOSのASGを設定する方法については、[EC2 Macインスタンスのオートスケールの実装](https://aws.amazon.com/blogs/compute/implementing-autoscaling-for-ec2-mac-instances/)を参照してください。
- [適切な権限](https://gitlab.com/gitlab-org/fleeting/plugins/aws#recommended-iam-policy)が設定されたIAMポリシー。

この設定では以下がサポートされています:

- 各インスタンスの`2`の容量。
- 無制限の使用回数。
- 分離されたジョブをサポートするためのネストされた仮想化。ネストされた仮想化は、[ネスティング](https://gitlab.com/gitlab-org/fleeting/nesting)がインストールされたAppleシリコンインスタンスでのみ使用できます。
- アイドルスケール: `5`。
- アイドル時間: 20分。
- インスタンスの最大数: `10`。

各インスタンスの容量が`2`で、使用回数が無制限の場合、各インスタンスはインスタンスのライフタイムの間、2つのジョブを同時に実行します。

アイドルスケールが`2`の場合、使用中の容量が`2`を下回ると、アイドルインスタンスが1つ作成されます。アイドルインスタンスは、少なくとも24時間は残ります。この時間枠は、AWS MacOSインスタンスホストの24時間の最小割り当て期間によるものです。

この環境で実行されるジョブは、[ネスティング](https://gitlab.com/gitlab-org/fleeting/nesting)が各ジョブのネストされた仮想化に使用されるため、信頼する必要はありません。これは、Apple Siliconインスタンスでのみ機能します。

Runnerの`concurrent`フィールドは8（インスタンスの最大数*インスタンスあたりのキャパシティ）に設定されます。

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

## Google Cloudインスタンスグループ構成の例 {#google-cloud-instance-group-configuration-examples}

### Google Cloudインスタンスグループを使用したインスタンスあたりのジョブ数1 {#one-job-per-instance-using-a-google-cloud-instance-group}

前提要件:

- 少なくとも`git`とGitLab Runnerがインストールされたカスタムイメージ。
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

### Google Cloudインスタンスグループを使用した、インスタンスあたり5つのジョブ、無制限の使用 {#five-jobs-per-instance-unlimited-uses-using-google-cloud-instance-group}

前提要件:

- 少なくとも`git`とGitLab Runnerがインストールされたカスタムイメージ。
- インスタンスグループ。Runnerがスケールを処理するため、「オートスケールモード」では「オートスケールしない」を選択します。
- [適切な権限](https://gitlab.com/gitlab-org/fleeting/plugins/googlecloud#required-permissions)が設定されたIAMポリシー。

この設定では以下がサポートされています:

- インスタンスあたりのキャパシティ: 5。
- 無制限の使用回数
- アイドルスケール: 5
- アイドル時間: 20分
- インスタンスの最大数: 10

容量が`5`に設定され、使用回数が無制限の場合、各インスタンスはインスタンスのライフタイムの間、5つのジョブを同時に実行します。

これらの環境で実行されるジョブは、**信頼**されている必要があります。それらの間にはほとんど分離がなく、各ジョブが別のジョブのパフォーマンスに影響を与える可能性があるためです。

アイドルスケールが`5`の場合、使用中の容量が`5`を下回ると、アイドルインスタンスが1つ作成されます。これらのインスタンスは少なくとも20分間維持されます。

Runnerの`concurrent`フィールドは50（インスタンスの最大数*インスタンスあたりのキャパシティ）に設定されます。

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

## Azureスケールセット構成の例 {#azure-scale-set-configuration-examples}

### Azureスケールセットを使用したインスタンスごとのジョブ数1 {#one-job-per-instance-using-an-azure-scale-set}

前提要件:

- 少なくとも`git`とGitLab Runnerがインストールされたカスタムイメージ。
- オートスケールモードが`manual`に設定され、オーバープロビジョニングがオフになっているAzureスケールセット。Runnerがスケーリングを処理します。

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

### Azureスケールセットを使用した、インスタンスあたり5つのジョブ、無制限の使用 {#five-jobs-per-instance-unlimited-uses-using-an-azure-scale-set}

前提要件:

- 少なくとも`git`とGitLab Runnerがインストールされたカスタムイメージ。
- オートスケールモードが`manual`に設定され、オーバープロビジョニングがオフになっているAzureスケールセット。Runnerがスケーリングを処理します。

この設定では以下がサポートされています:

- インスタンスあたりのキャパシティ: 5。
- 無制限の使用回数
- アイドルスケール: 5
- アイドル時間: 20分
- インスタンスの最大数: 10

容量が`5`に設定され、使用回数が無制限の場合、各インスタンスはインスタンスのライフタイムの間、5つのジョブを同時に実行します。

これらの環境で実行されるジョブは、**信頼**されている必要があります。それらの間にはほとんど分離がなく、各ジョブが別のジョブのパフォーマンスに影響を与える可能性があるためです。

アイドルスケールが`2`の場合、使用中の容量が`5`を下回ると、アイドルインスタンスが1つ作成されます。これらのインスタンスは少なくとも20分間維持されます。

Runnerの`concurrent`フィールドは50（インスタンスの最大数*インスタンスあたりのキャパシティ）に設定されます。

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

## トラブルシューティング {#troubleshooting}

インスタンスexecutorを使用するときに次の問題が発生する可能性があります:

### `sh: 1: eval: Running on ip-x.x.x.x via runner-host...n: not found` {#sh-1-eval-running-on-ip-xxxx-via-runner-hostn-not-found}

このエラーは通常、準備ステップの`eval`コマンドが失敗した場合に発生します。このエラーを解決するには、`bash`シェルに切り替え、[機能フラグ](../configuration/feature-flags.md) `FF_USE_NEW_BASH_EVAL_STRATEGY`を有効にします。
