---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: AWS EC2でRunnerのDocker Machineオートスケールを設定する
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

GitLab Runnerの最大の利点の1つは、ビルドがすぐに処理されるようにするために、VMを自動的に起動および停止できることです。これは優れた機能であり、適切に使用すれば、Runnerを常時使用していない場合に、費用対効果が高くスケーラブルなソリューションが必要な状況で非常に役立ちます。

## はじめに {#introduction}

このチュートリアルでは、AWSでGitLab Runnerを適切に設定する方法について説明します。AWSのインスタンスは、新しいDockerインスタンスをオンデマンドで起動するRunnerマネージャーとして機能します。これらのインスタンスのRunnerは自動的に作成されます。Runnerはこのガイドで説明されているパラメータを使用します。作成後の手動設定は必要ありません。

さらに[AmazonのEC2スポットインスタンス](https://aws.amazon.com/ec2/spot/)を利用することで、非常に強力なオートスケールマシンを使用しながら、GitLab Runnerインスタンスのコストを大幅に削減できます。

## 前提条件 {#prerequisites}

設定のほとんどがAWSで行われるため、Amazon Web Services（AWS）に関する知識が必要です。

Docker Machineの[`amazonec2`ドライバーのドキュメント](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/aws.md)をざっと読んで、この記事で後述するパラメータを理解しておくことをお勧めします。

GitLab Runnerはネットワーク経由でGitLabインスタンスと通信する必要があります。このことは、AWSセキュリティグループを設定する場合やDNS設定を行う場合に考慮する必要があります。

たとえば、ネットワークセキュリティを強化するために、EC2リソースを別のVPCでパブリックトラフィックから分離できます。ご使用の環境は異なる可能性があるため、状況に対して最適なものを検討してください。

### AWSセキュリティグループ {#aws-security-groups}

Docker Machineは、Dockerデーモンとの通信に必要なポート`2376`およびSSH `22`のルールと[デフォルトのセキュリティグループ](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/aws.md/#security-group)を使用しようとします。Dockerに依存する代わりに、必要なルールを使用してセキュリティグループを作成し、[下記](#the-runnersmachine-section)で説明するように、GitLab Runnerオプションでそのグループを指定できます。これにより、ネットワーク環境に基づいて、好みに合わせて事前にカスタマイズできます。[Runnerマネージャーインスタンス](#prepare-the-runner-manager-instance)からポート`2376`と`22`にアクセスできることを確認する必要があります。

### AWS認証情報 {#aws-credentials}

キャッシュのスケール（EC2）とキャッシュの更新（S3経由）の権限を持つユーザーに関連付けられている[AWSアクセスキー](https://docs.aws.amazon.com/IAM/latest/UserGuide/security-creds.html)が必要です。EC2（AmazonEC2FullAccess）およびS3の[ポリシー](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-policies-for-amazon-ec2.html)を使用して新しいユーザーを作成します。S3に必要な最小限の権限の詳細については、[`runners.cache.s3`](../advanced-configuration.md#the-runnerscaches3-section)を参照してください。セキュリティを強化するために、そのユーザーのコンソールログインを無効にできます。タブを開いたままにするか、後で[GitLab Runnerの設定](#the-runnersmachine-section)で使用するためにセキュリティ認証情報をエディタにコピーして貼り付けます。

必要な`AmazonEC2FullAccess`ポリシーと`AmazonS3FullAccess`ポリシーを使用して[EC2インスタンスプロファイル](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2_instance-profiles.html)を作成することもできます。

ジョブの実行のために新しいEC2インスタンスをプロビジョニングするには、このインスタンスプロファイルをRunnerマネージャーEC2インスタンスにアタッチします。Runnerマシンがインスタンスプロファイルを使用している場合は、Runnerマネージャーのインスタンスプロファイルに`iam:PassRole`アクションを含めます。

例: 

```json
{
    "Statement": [
        {
            "Action": "iam:PassRole",
            "Effect": "Allow",
            "Resource": "arn:aws:iam:::role/instance-profile-of-runner-machine"
        }
    ],
    "Version": "2012-10-17"
}
```

## Runnerマネージャーインスタンスを準備する {#prepare-the-runner-manager-instance}

最初に、新しいマシンを起動するRunnerマネージャーとして機能するEC2インスタンスにGitLab Runnerをインストールします。DockerとGitLab Runnerの両方がサポートするディストリビューション（Ubuntu、Debian、CentOS、RHELなど）を選択します。

Runnerマネージャーインスタンス自体はジョブを実行しないため、これは強力なマシンである必要はありません。最初の設定では、小さなインスタンスから開始できます。このマシンは常に稼働している必要があるため、専任ホストです。したがって、継続的なベースラインコストがかかるのはこのホストだけです。

前提条件をインストールします。

1. サーバーにログインします
1. [GitLabの公式リポジトリからGitLab Runnerをインストールします](../../install/linux-repository.md)
1. [Dockerをインストールします](https://docs.docker.com/engine/install/#server)
1. [GitLabフォークからDocker Machineをインストールします](https://gitlab.com/gitlab-org/ci-cd/docker-machine)（DockerではDocker Machineが非推奨になりました）

Runnerがインストールされたので、次にRunnerを登録します。

## GitLab Runnerを登録する {#registering-the-gitlab-runner}

GitLab Runnerを設定する前に、最初にGitLab Runnerを登録して、GitLabインスタンスに接続する必要があります。

1. [Runnerトークンを取得します](https://docs.gitlab.com/ci/runners/)
1. [Runnerを登録します](../../register/_index.md)
1. executorの種類を尋ねられたら、`docker+machine`と入力します

これで、最も重要な部分であるGitLab Runnerの設定に進むことができます。

{{< alert type="note" >}}

インスタンス内のすべてのユーザーが、オートスケールされたRunnerを使用できるようにする場合は、Runnerを共有Runnerとして登録します。

{{< /alert >}}

## Runnerを設定する {#configuring-the-runner}

Runnerが登録されたので、その設定ファイルを編集してAWS Machineドライバーに必要なオプションを追加する必要があります。

次に設定ファイルの各セクションについて詳しく説明します。

### グローバルセクション {#the-global-section}

グローバルセクションでは、すべてのRunnerで同時に実行できるジョブの制限（`concurrent`）を定義できます。これは、GitLab Runnerが対応するユーザーの数やビルドにかかる時間などのニーズに応じて大きく異なります。最初に`10`のような小さい値を使用し、その後、値を増減できます。

`check_interval`オプションは、RunnerがGitLabで新しいジョブを確認する頻度を秒単位で定義します。

例: 

```toml
concurrent = 10
check_interval = 0
```

[その他のオプション](../advanced-configuration.md#the-global-section)も利用できます。

### `runners`セクション {#the-runners-section}

`[[runners]]`セクションで最も重要な設定は`executor`です。これは`docker+machine`に設定する必要があります。これらの設定のほとんどは、Runnerを初めて登録するときに処理されます。

`limit`は、このRunnerが起動するマシン（実行中のマシンおよびアイドル状態のマシン）の最大数を設定します。詳細については、[`limit`、`concurrent`、`IdleCount`の間の関係](../autoscale.md#how-concurrent-limit-and-idlecount-generate-the-upper-limit-of-running-machines)をご確認ください。

例: 

```toml
[[runners]]
  name = "gitlab-aws-autoscaler"
  url = "<URL of your GitLab instance>"
  token = "<Runner's token>"
  executor = "docker+machine"
  limit = 20
```

`[[runners]]`の[その他のオプション](../advanced-configuration.md#the-runners-section)も利用できます。

### `runners.docker`セクション {#the-runnersdocker-section}

`[runners.docker]`セクションでは、[`.gitlab-ci.yml`](https://docs.gitlab.com/ci/yaml/)でDockerイメージが定義されていない場合に子Runnerが使用するデフォルトのDockerイメージを定義できます。`privileged = true`を使用すると、すべてのRunnerが[Docker in Docker](https://docs.gitlab.com/ci/docker/using_docker_build/#use-docker-in-docker)を実行できるようになります。これは、GitLab CI/CDで独自のDockerイメージをビルドする予定がある場合に役立ちます。

次に`disable_cache = true`を使用して、Docker executorの内部キャッシュメカニズムを無効にします。これは、以下のセクションで説明するように分散キャッシュモードを使用するためです。

例: 

```toml
  [runners.docker]
    image = "alpine"
    privileged = true
    disable_cache = true
```

`[runners.docker]`の[その他のオプション](../advanced-configuration.md#the-runnersdocker-section)も利用できます。

### `runners.cache`セクション {#the-runnerscache-section}

ジョブの処理をスピードアップするために、GitLab Runnerは、選択されたディレクトリやファイルを保存し、後続のジョブ間で共有するキャッシュメカニズムを提供します。このセットアップでは必須ではありませんが、GitLab Runnerが提供する分散キャッシュメカニズムを使用することをお勧めします。新しいインスタンスがオンデマンドで作成されるため、キャッシュを保存する共通の場所を確保することが重要です。

次の例ではAmazon S3を使用します。

```toml
  [runners.cache]
    Type = "s3"
    Shared = true
    [runners.cache.s3]
      ServerAddress = "s3.amazonaws.com"
      AccessKey = "<your AWS Access Key ID>"
      SecretKey = "<your AWS Secret Access Key>"
      BucketName = "<the bucket where your cache should be kept>"
      BucketLocation = "us-west-2"
```

キャッシュメカニズムを詳しく調べるための詳細情報を以下に示します。

- [`runners.cache`のリファレンス](../advanced-configuration.md#the-runnerscache-section)
- [`runners.cache.s3`のリファレンス](../advanced-configuration.md#the-runnerscaches3-section)
- [GitLab Runnerでのキャッシュサーバーのデプロイと使用](../autoscale.md#distributed-runners-caching)
- [キャッシュの仕組み](https://docs.gitlab.com/ci/yaml/#cache)

### `runners.machine`セクション {#the-runnersmachine-section}

これは設定で最も重要な部分であり、GitLab Runnerに対して新しいDocker Machineインスタンスを起動または削除する方法とタイミングを指示します。

AWS Machineオプションを中心に説明します。その他の設定については、以下の資料を参照してください。

- [基盤となるオートスケールアルゴリズムとパラメータ](../autoscale.md#autoscaling-algorithm-and-parameters) \- 組織のニーズに応じて異なります。
- [オートスケール期間](../autoscale.md#configure-autoscaling-periods) \- 組織で作業が行われない一定の期間がある場合（週末など）に役立ちます。

以下に`runners.machine`セクションの例を示します。

```toml
  [runners.machine]
    IdleCount = 1
    IdleTime = 1800
    MaxBuilds = 10
    MachineDriver = "amazonec2"
    MachineName = "gitlab-docker-machine-%s"
    MachineOptions = [
      "amazonec2-access-key=XXXX",
      "amazonec2-secret-key=XXXX",
      "amazonec2-region=eu-central-1",
      "amazonec2-vpc-id=vpc-xxxxx",
      "amazonec2-subnet-id=subnet-xxxxx",
      "amazonec2-zone=x",
      "amazonec2-use-private-address=true",
      "amazonec2-tags=runner-manager-name,gitlab-aws-autoscaler,gitlab,true,gitlab-runner-autoscale,true",
      "amazonec2-security-group=xxxxx",
      "amazonec2-instance-type=m4.2xlarge",
    ]
    [[runners.machine.autoscaling]]
      Periods = ["* * 9-17 * * mon-fri *"]
      IdleCount = 50
      IdleTime = 3600
      Timezone = "UTC"
    [[runners.machine.autoscaling]]
      Periods = ["* * * * * sat,sun *"]
      IdleCount = 5
      IdleTime = 60
      Timezone = "UTC"
```

Docker Machineドライバーは`amazonec2`に設定され、マシン名には標準のプレフィックスが付加され、その後に`%s`（必須）が続きます。これは子RunnerのIDに置き換えられます（`gitlab-docker-machine-%s`）。

ご使用のAWSインフラストラクチャに応じて、`MachineOptions`で設定できる多くのオプションがあります。最も一般的なオプションを以下に示します。

| マシンオプション                                                         | 説明 |
|------------------------------------------------------------------------|-------------|
| `amazonec2-access-key=XXXX`                                            | EC2インスタンスを作成する権限を持つユーザーのAWSアクセスキー。[AWS認証情報](#aws-credentials)を参照してください。 |
| `amazonec2-secret-key=XXXX`                                            | EC2インスタンスを作成する権限を持つユーザーのAWSシークレットキーについては、[AWS認証情報](#aws-credentials)を参照してください。 |
| `amazonec2-region=eu-central-2`                                        | インスタンスを起動するときに使用するリージョン。これを完全に省略すると、デフォルトの`us-east-1`が使用されます。 |
| `amazonec2-vpc-id=vpc-xxxxx`                                           | インスタンスを起動する[VPC ID](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/aws.md#vpc-id)。 |
| `amazonec2-subnet-id=subnet-xxxx`                                      | AWS VPCサブネットID。 |
| `amazonec2-zone=x`                                                     | 指定しない場合、[アベイラビリティゾーンは`a`になります](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/aws.md#environment-variables-and-default-values)。これは、指定されたサブネットと同じアベイラビリティゾーンに設定する必要があります。たとえば、ゾーンが`eu-west-1b`の場合は`amazonec2-zone=b`にする必要があります。 |
| `amazonec2-use-private-address=true`                                   | Docker MachineのプライベートIPアドレスを使用しますが、パブリックIPアドレスを引き続き作成します。トラフィックを内部で維持し、余分なコストを回避するのに役立ちます。 |
| `amazonec2-tags=runner-manager-name,gitlab-aws-autoscaler,gitlab,true,gitlab-runner-autoscale,true` | AWSの追加タグキー値ペア。AWSコンソールでインスタンスを識別する際に役立ちます。「Name」[タグ](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/Using_Tags.html)は、デフォルトでマシン名に設定されます。`[[runners]]`で設定されているRunnerの名前に一致するように、「runner-manager-name」に設定しました。これにより、セットアップされている特定のマネージャーにより作成されるすべてのEC2インスタンスをフィルタリングできます。 |
| `amazonec2-security-group=xxxx`                                        | AWS VPCセキュリティグループ名。セキュリティグループIDではありません。[AWSセキュリティグループ](#aws-security-groups)を参照してください。 |
| `amazonec2-instance-type=m4.2xlarge`                                   | 子Runnerが実行されるインスタンスのタイプ。 |
| `amazonec2-ssh-user=xxxx`                                              | インスタンスへのSSHアクセス権を持つユーザー。 |
| `amazonec2-iam-instance-profile=xxxx_runner_machine_inst_profile_name` | Runnerマシンに使用するIAMインスタンスプロファイル。 |
| `amazonec2-ami=xxxx_runner_machine_ami_id`                             | 特定のイメージのGitLab Runner AMI ID。 |
| `amazonec2-request-spot-instance=true`                                 | オンデマンドの価格よりも安価で利用できる予備のEC2キャパシティを使用します。 |
| `amazonec2-spot-price=xxxx_runner_machine_spot_price=x.xx`             | スポットインスタンスの入札価格（米ドル）。`--amazonec2-request-spot-instance flag`を`true`に設定する必要があります。`amazonec2-spot-price`を省略すると、Docker Machineは最高価格をデフォルト値（1時間あたり`$0.50`）に設定します。 |
| `amazonec2-security-group-readonly=true`                               | セキュリティグループを読み取り専用に設定します。 |
| `amazonec2-userdata=xxxx_runner_machine_userdata_path`                 | Runnerマシンの`userdata`パスを指定します。 |
| `amazonec2-root-size=XX`                                               | インスタンスのルートディスクサイズ（GB単位）。 |

ノート:

- `MachineOptions`の下には、[AWS Docker Machineドライバーでサポートされている](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/aws.md#options)すべてのオプションを追加できます。インフラストラクチャのセットアップでさまざまなオプションを適用することが必要となる場合があるため、Dockerのドキュメントを読んでおくことを強くお勧めします。
- `amazonec2-ami`を設定して別のAMI IDを選択しない限り、子インスタンスはデフォルトでUbuntu 16.04を使用します。[Docker Machineでサポートされているベースオペレーティングシステム](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/os-base)のみを設定します。
- マシンオプションの1つとして`amazonec2-private-address-only=true`を指定すると、EC2インスタンスにパブリックIPは割り当てられません。これは、VPCがインターネットゲートウェイ（IGW）で正しく設定されており、ルーティングが正常に機能している場合は問題ありませんが、より複雑な設定では検討が必要となります。詳しくは、[VPC接続に関するDockerドキュメント](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/aws.md#vpc-connectivity)を参照してください。

`[runners.machine]`の[その他のオプション](../advanced-configuration.md#the-runnersmachine-section)も利用できます。

### 完全な例 {#getting-it-all-together}

完全な`/etc/gitlab-runner/config.toml`の例を次に示します。

```toml
concurrent = 10
check_interval = 0

[[runners]]
  name = "gitlab-aws-autoscaler"
  url = "<URL of your GitLab instance>"
  token = "<runner's token>"
  executor = "docker+machine"
  limit = 20
  [runners.docker]
    image = "alpine"
    privileged = true
    disable_cache = true
  [runners.cache]
    Type = "s3"
    Shared = true
    [runners.cache.s3]
      ServerAddress = "s3.amazonaws.com"
      AccessKey = "<your AWS Access Key ID>"
      SecretKey = "<your AWS Secret Access Key>"
      BucketName = "<the bucket where your cache should be kept>"
      BucketLocation = "us-west-2"
  [runners.machine]
    IdleCount = 1
    IdleTime = 1800
    MaxBuilds = 100
    MachineDriver = "amazonec2"
    MachineName = "gitlab-docker-machine-%s"
    MachineOptions = [
      "amazonec2-access-key=XXXX",
      "amazonec2-secret-key=XXXX",
      "amazonec2-region=eu-central-1",
      "amazonec2-vpc-id=vpc-xxxxx",
      "amazonec2-subnet-id=subnet-xxxxx",
      "amazonec2-use-private-address=true",
      "amazonec2-tags=runner-manager-name,gitlab-aws-autoscaler,gitlab,true,gitlab-runner-autoscale,true",
      "amazonec2-security-group=XXXX",
      "amazonec2-instance-type=m4.2xlarge",
    ]
    [[runners.machine.autoscaling]]
      Periods = ["* * 9-17 * * mon-fri *"]
      IdleCount = 50
      IdleTime = 3600
      Timezone = "UTC"
    [[runners.machine.autoscaling]]
      Periods = ["* * * * * sat,sun *"]
      IdleCount = 5
      IdleTime = 60
      Timezone = "UTC"
```

## Amazon EC2スポットインスタンスによってコストを削減する {#cutting-down-costs-with-amazon-ec2-spot-instances}

Amazonでは次のように[説明](https://aws.amazon.com/ec2/spot/)されています。

>
Amazon EC2スポットインスタンスを使用すると、予備のAmazon EC2コンピューティングキャパシティに入札できます。スポットインスタンスは、オンデマンド料金と比較して割引された料金で利用できることが多いため、アプリケーションの実行コストを大幅に削減し、同じ予算でアプリケーションのコンピューティングキャパシティとスループットを向上させ、新しいタイプのクラウドコンピューティングアプリケーションを有効にすることができます。

上記で選択した[`runners.machine`](#the-runnersmachine-section)オプションに加えて、`/etc/gitlab-runner/config.toml`の`MachineOptions`セクションの下に次の内容を追加します。

```toml
    MachineOptions = [
      "amazonec2-request-spot-instance=true",
      "amazonec2-spot-price=",
    ]
```

この設定では、`amazonec2-spot-price`が空の場合、AWSはスポットインスタンスの入札価格を、そのインスタンスクラスのデフォルトのオンデマンド価格に設定します。`amazonec2-spot-price`を完全に省略すると、Docker Machineは最高価格を[デフォルト値（1時間あたり$0.50）](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/aws.md#environment-variables-and-default-values)に設定します。

スポットインスタンスのリクエストをさらにカスタマイズできます。

```toml
    MachineOptions = [
      "amazonec2-request-spot-instance=true",
      "amazonec2-spot-price=0.03",
      "amazonec2-block-duration-minutes=60"
    ]
```

この設定では、Docker Machineは1時間あたり最大スポットリクエスト価格が$0.03のスポットインスタンスを使用して作成され、スポットインスタンスの期間は60分に制限されます。前述の数値`0.03`は単なる例です。選択したリージョンに基づいて現在の価格を確認してください。

Amazon EC2スポットインスタンスの詳細については、次のリンクをご覧ください。

- <https://aws.amazon.com/ec2/spot/>
- <https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/spot-requests.html>
- <https://aws.amazon.com/ec2/spot/getting-started/>

### スポットインスタンスの注意事項 {#caveats-of-spot-instances}

スポットインスタンスは、未使用のリソースを利用してインフラストラクチャのコストを最小限に抑える優れた方法ですが、その影響に注意する必要があります。

スポットインスタンスの価格モデルが原因で、スポットインスタンスでCIジョブを実行すると、失敗率が高まる可能性があります。指定したスポット最高価格が現在のスポット価格を超えている場合、リクエストしたキャパシティは取得されません。スポット料金は1時間ごとに改定されます。既存のスポットインスタンスで設定されている最高価格が、改定されたスポットインスタンス価格よりも低い場合、そのスポットインスタンスは2分以内に終了し、スポットホスト上のすべてのジョブは失敗します。

その結果、オートスケールRunnerは新しいインスタンスをリクエストし続けても、新しいマシンを作成できません。これにより、最終的に60件のリクエストが行われ、AWSはそれ以上のリクエストを受け入れなくなります。その後、許容できるスポット価格になっても、呼び出し回数の制限を超えているため、しばらくの間ロックアウトされます。

この状況が発生した場合は、Runnerマネージャーマシンで次のコマンドを使用して、Docker Machineの状態を確認できます。

```shell
docker-machine ls -q --filter state=Error --format "{{.NAME}}"
```

{{< alert type="note" >}}

GitLab Runnerがスポット価格の変更を正常に処理することに関していくつかの問題があり、`docker-machine`がDocker Machine継続的に削除しようとするという報告があります。GitLabは、アップストリームプロジェクトで両方のケースに対するパッチを提供しました。詳細については、[イシュー#2771](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/2771)と[\#2772](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/2772)を参照してください。

{{< /alert >}}

GitLabフォークは、AWS EC2フリートとスポットインスタンスでのこれらのフリートの使用をサポートしていません。代替策として、[Continuous Kernel Integration Projectのダウンストリームフォーク](https://gitlab.com/cki-project/mirror/docker-machine)を使用できます。

## まとめ {#conclusion}

このガイドでは、AWSでオートスケールモードでGitLab Runnerをインストールおよび設定する方法を説明しました。

GitLab Runnerのオートスケール機能を使用すると、時間と費用の両方を節約できます。AWSが提供するスポットインスタンスを使用するとさらに節約できますが、その影響に注意する必要があります。入札価格が十分に高ければ、問題はありません。

このチュートリアルに（大きな）影響を与えた次のユースケースを読むことができます。

- [HumanGeo、JenkinsからGitLabへ乗り換え](https://about.gitlab.com/blog/humangeo-switches-jenkins-gitlab-ci/)
- [Substrakt Health - GitLab CI/CD Runnerをオートスケールし、EC2コストを90%削減](https://about.gitlab.com/blog/autoscale-ci-runners/)
