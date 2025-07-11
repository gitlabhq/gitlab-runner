---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: GitLab Runnerの自動スケール
---

{{< details >}}

- プラン:Free、Premium、Ultimate
- 製品:GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

GitLab Runnerの自動スケールを使用すると、パブリッククラウドインスタンスでRunnerを自動的にスケールできます。autoscalerを使用するようにRunnerを設定すると、クラウドインフラストラクチャ上で複数のジョブを同時に実行することで、CI/CDジョブ負荷の増加に対処できます。

パブリッククラウドインスタンスの自動スケールオプションに加えて、次のコンテナオーケストレーションソリューションを使用して、Runnerフリートをホストおよびスケールできます。

- Red Hat OpenShift Kubernetesクラスター
- Kubernetesクラスター:AWS EKS、Azure、オンプレミス
- AWS FargateのAmazon Elastic Container Servicesクラスター

## GitLab Runner Autoscaler

GitLab Runner Autoscalerは、Docker Machineをベースとした自動スケーリングテクノロジーの後継機能です。GitLab Runner Autoscalerのコンポーネントは次のとおりです。

- taskscaler:自動スケールロジック、ブックキーピングを管理し、クラウドプロバイダーのインスタンスの自動スケールグループを使用するRunnerインスタンスのフリートを作成します。
- フリート:クラウドプロバイダー仮想マシンの抽象化。
- クラウドプロバイダープラグイン:ターゲットクラウドプラットフォームへのAPIコールを処理します。プラグイン開発フレームワークを使用して実装されます。

![GitLab Next Runner Autoscalingの概要](img/next-runner-autoscaling-overview.png)

### GitLab Runner Autoscalerでサポートされているパブリッククラウドインスタンス

パブリッククラウドコンピューティングインスタンスでは、次の自動スケールオプションがサポートされています。

|                   | Next Runner Autoscaler                 | GitLab Runner Docker Machine Autoscaler                |
|----------------------------|------------------------|------------------------|
| Amazon Web Services EC2インスタンス         | {{< icon name="check-circle" >}} はい | {{< icon name="check-circle" >}} はい |
| Google Compute Engine | {{< icon name="check-circle" >}} はい | {{< icon name="check-circle" >}} はい |
|Microsoft Azure Virtual Machines|{{< icon name="check-circle" >}} はい|{{< icon name="check-circle" >}} はい|

### GitLab Runner Autoscalerでサポートされているプラットフォーム

| executor                   | Linux                  | macOS                  | Windows                |
|----------------------------|------------------------|------------------------|------------------------|
| Instance executor          | {{< icon name="check-circle" >}} はい | {{< icon name="check-circle" >}} はい | {{< icon name="check-circle" >}} はい |
| Docker Autoscaler executor | {{< icon name="check-circle" >}} はい | {{< icon name="dotted-circle" >}} いいえ | {{< icon name="check-circle" >}} はい |

## Runnerマネージャーを設定する

GitLab Runnerの自動スケール（Docker Machine AutoscalingソリューションとGitLab Runner Autoscalerの両方）を使用するようにRunnerマネージャーを設定する必要があります。

Runnerマネージャーは、自動スケール用に複数のRunnerを作成するRunnerの一種です。GitLabに対しジョブを継続的にポーリングし、パブリッククラウドインフラストラクチャと連携して、ジョブを実行するための新しいインスタンスを作成します。Runnerマネージャーは、GitLab Runnerがインストールされているホストマシン上で実行する必要があります。DockerとGitLab Runnerがサポートするディストリビューション（Ubuntu、Debian、CentOS、RHELなど）を選択します。

1. Runnerマネージャーをホストするインスタンスを作成します。これはスポットインスタンス（AWS）またはスポット仮想マシン（GCP、Azure）**であってはなりません**。
1. インスタンスに[GitLab Runnerをインストールします](../install/linux-repository.md)。
1. クラウドプロバイダーの認証情報をRunnerマネージャーのホストマシンに追加します。

{{< alert type="note" >}}

コンテナ内でRunnerマネージャーをホストできます。[GitLabでホストされるRunner](https://docs.gitlab.com/ci/runners/)の場合、Runnerマネージャーは仮想マシンインスタンスでホストされます。

{{< /alert >}}

### GitLab Runner Autoscalerの認証情報の設定例

``` toml
## credentials_file

[default]
aws_access_key_id=__REDACTED__
aws_secret_access_key=__REDACTED__
```

### GitLab Runner Docker Machine Autoscalingの認証情報の設定例

このスニペットは、`config.toml`ファイルのrunners.machineセクションにあります。

``` toml
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
      "amazonec2-security-group=xxxxx",
    ]
```

{{< alert type="note" >}}

認証情報ファイルはオプションです。AWS環境のRunnerマネージャーには[AWS Identity and Access Management](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2_instance-profiles.html)（IAM）インスタンスプロファイルを使用できます。AWSでRunnerマネージャーをホストしない場合は、認証情報ファイルを使用できます。

{{< /alert >}}

## 耐障害性のある設計を実装する

耐障害性のある設計を作成し、Runnerマネージャーホストの障害を防ぐには、同じRunnerタグを使用する少なくとも2つのRunnerマネージャーから始めます。

たとえばGitLab.comでは、[LinuxでホストされるRunner](https://docs.gitlab.com/ci/runners/hosted_runners/linux/)に対して複数のRunnerマネージャーが設定されています。各Runnerマネージャーにはタグ`saas-linux-small-amd64`があります。

組織のCI/CDワークロードの効率とパフォーマンスのバランスを取るために自動スケールパラメーターを調整するときには、可観測性とRunnerフリートのメトリクスを使用します。

## Runnerの自動スケールexecutorを設定する

Runnerマネージャーを設定したら、自動スケールに固有のexecutorを設定します。

- [Instance Executor](../executors/instance.md)
- [Docker Autoscaler Executor](../executors/docker_autoscaler.md)
- [Docker Machine Executor](../executors/docker_machine.md)

{{< alert type="note" >}}

Instance executorとDocker Autoscaler executorを使用してください。これらのexecutorは、Docker Machine autoscalerに代わるテクノロジーを構成しています。

{{< /alert >}}
