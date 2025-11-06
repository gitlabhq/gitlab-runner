---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: GitLab Runnerのオートスケール
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

GitLab Runnerのオートスケールを使用すると、パブリッククラウドインスタンスでRunnerを自動的にスケールできます。オートスケーラーを使用するようにRunnerを設定すると、クラウドインフラストラクチャ上で複数のジョブを同時に実行することで、CI/CDジョブのワークロード増加に対処できます。

パブリッククラウドインスタンスのオートスケールオプションに加えて、次のコンテナオーケストレーションソリューションを使用して、Runnerフリートをホストおよびスケールできます。

- Red Hat OpenShift Kubernetesクラスター
- Kubernetesクラスター: AWS EKS、Azure、オンプレミス
- AWS FargateのAmazon Elastic Container Servicesクラスター

## Runnerマネージャーを設定する {#configure-the-runner-manager}

GitLab Runnerのオートスケール（Docker Machine AutoscalingソリューションとGitLab Runner Autoscalerの両方）を使用するようにRunnerマネージャーを設定する必要があります。

Runnerマネージャーは、オートスケール用に複数のRunnerを作成するRunnerの一種です。GitLabに対しジョブを継続的にポーリングし、パブリッククラウドインフラストラクチャと連携して、ジョブを実行するための新しいインスタンスを作成します。Runnerマネージャーは、GitLab Runnerがインストールされているホストマシン上で実行する必要があります。DockerとGitLab Runnerがサポートするディストリビューション（Ubuntu、Debian、CentOS、RHELなど）を選択します。

1. Runnerマネージャーをホストするインスタンスを作成します。これはスポットインスタンス（AWS）またはスポット仮想マシン（GCP、Azure）**であってはなりません**。
1. [インスタンス](../install/linux-repository.md)にGitLab Runnerをインストールします。
1. クラウドプロバイダーの認証情報をRunnerマネージャーのホストマシンに追加します。

{{< alert type="note" >}}

コンテナ内でRunnerマネージャーをホストできます。[GitLab.comでホストされるRunner](https://docs.gitlab.com/ci/runners/)の場合、Runnerマネージャーは仮想マシンインスタンスでホストされます。

{{< /alert >}}

### GitLab Runner Docker Machine Autoscalingの認証情報の設定例 {#example-credentials-configuration-for-gitlab-runner-docker-machine-autoscaling}

このスニペットは、ファイル`config.toml`の`runners.machine`セクションの中にあります。

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

認証情報ファイルはオプションです。AWS環境のRunnerマネージャーには[AWSアイデンティティおよびアクセス管理](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2_instance-profiles.html)（IAM）インスタンスプロファイルを使用できます。AWSでRunnerマネージャーをホストしない場合は、認証情報ファイルを使用できます。

{{< /alert >}}

## 耐障害性のあるデザインを実装する {#implement-a-fault-tolerant-design}

耐障害性のあるデザインを作成し、Runnerマネージャーホストの障害を防ぐには、同じRunnerタグを使用する少なくとも2つのRunnerマネージャーから始めます。

たとえばGitLab.comでは、[LinuxでホストされるRunner](https://docs.gitlab.com/ci/runners/hosted_runners/linux/)に対して複数のRunnerマネージャーが設定されています。各Runnerマネージャーにはタグ`saas-linux-small-amd64`があります。

組織のCI/CDワークロードの効率性とパフォーマンスのバランスを取るためにオートスケールパラメータを調整するときには、可観測性とRunnerフリートのメトリクスを使用します。

## Runnerのオートスケールexecutorを設定する {#configure-runner-autoscaling-executors}

Runnerマネージャーを設定したら、オートスケールに固有のexecutorを設定します:

- [インスタンスExecutor](../executors/instance.md)
- [Docker Autoscaling Executor](../executors/docker_autoscaler.md)
- [Docker Machine Executor](../executors/docker_machine.md)

{{< alert type="note" >}}

Instance executorとDocker Autoscaling executorを使用してください。これらのexecutorは、Docker Machineオートスケーラーに代わるテクノロジーを構成しています。

{{< /alert >}}
