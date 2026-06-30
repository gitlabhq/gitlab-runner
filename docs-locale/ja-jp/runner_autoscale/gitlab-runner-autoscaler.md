---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: GitLab Runnerインスタンスグループオートスケーラー
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

GitLab Runnerインスタンスグループオートスケーラーは、Docker Machineをベースとしたオートスケール技術の後継です。GitLab Runnerインスタンスグループオートスケールソリューションのコンポーネントは以下のとおりです:

- taskscaler: 自動スケールロジック、ブックキーピングを管理し、クラウドプロバイダーのインスタンスの自動スケールグループを使用するRunnerインスタンスのフリートを作成します。
- [Fleeting](../fleet_scaling/fleeting.md): クラウドプロバイダー仮想マシンの抽象化。
- クラウドプロバイダープラグイン: ターゲットクラウドプラットフォームへのAPIコールを処理します。プラグイン開発フレームワークを使用して実装されます。

GitLab Runnerにおけるインスタンスグループオートスケールは次のとおりに動作します:

1. RunnerマネージャーはGitLabジョブを継続的にポーリングします。
1. その応答として、GitLabはジョブペイロードをRunnerマネージャーに送信します。
1. Runnerマネージャーは、公開クラウドプロバイダーインフラストラクチャと連携し、ジョブを実行するための新しいインスタンスを作成します。
1. Runnerマネージャーはこれらのジョブを、オートスケールプール内の利用可能なRunnerに分散します。

![GitLab Next Runner Autoscalingの概要](img/next-runner-autoscaling-overview.png)

## Runnerマネージャーを設定する {#configure-the-runner-manager}

GitLab Runnerインスタンスグループオートスケーラーを使用するには、[Runnerマネージャーを設定](_index.md#configure-the-runner-manager)する必要があります。

1. Runnerマネージャーをホストするインスタンスを作成します。これはスポットインスタンス (AWS) やスポット仮想マシン (GCPまたはAzure) で**あってはなりません**。
1. [インスタンス](../install/linux-repository.md)にGitLab Runnerをインストールします。
1. クラウドプロバイダーの認証情報をRunnerマネージャーホストマシンに追加します。

   > [!note]
   > Runnerマネージャーをコンテナでホストできます。GitLab.comおよびGitLab Dedicatedの[ホストされたRunner](https://docs.gitlab.com/ci/runners/)の場合、Runnerマネージャーは仮想マシンインスタンスでホストされます。

### GitLab Runnerインスタンスグループオートスケーラーの認証情報設定例 {#example-credentials-configuration-for-gitlab-runner-instance-group-autoscaler}

AWS環境のRunnerマネージャーには[AWSアイデンティティおよびアクセス管理](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2_instance-profiles.html)（IAM）インスタンスプロファイルを使用できます。RunnerマネージャーをAWSでホストしたくない場合は、認証情報ファイルを使用できます。

例: 

``` toml
## credentials_file

[default]
aws_access_key_id=__REDACTED__
aws_secret_access_key=__REDACTED__
```

認証情報ファイルはオプションです。

## サポートされている公開クラウドプロバイダーインスタンス {#supported-public-cloud-instances}

公開クラウドプロバイダーコンピューティングインスタンスでは、次のオートスケールオプションがサポートされています:

- Amazon Web Services EC2インスタンス
- Google Compute Engine
- Microsoft Azure Virtual Machines

これらのクラウドプロバイダーインスタンスは、GitLab RunnerのDocker Machineオートスケーラーでもサポートされています。

## サポートされているプラットフォーム {#supported-platforms}

| executor                   | Linux                                | macOS                                | Windows                              |
|----------------------------|--------------------------------------|--------------------------------------|--------------------------------------|
| インスタンスexecutor          | {{< icon name="check-circle" >}} はい | {{< icon name="check-circle" >}} はい | {{< icon name="check-circle" >}} はい |
| Docker Autoscaler executor | {{< icon name="check-circle" >}} はい | {{< icon name="dotted-circle" >}}非対応 | {{< icon name="check-circle" >}} はい |
