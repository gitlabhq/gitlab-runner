---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: executor
---

{{< details >}}

- プラン:Free、Premium、Ultimate
- 製品:GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

GitLab Runnerはさまざまなexecutorを実装しています。これらのexecutorは、さまざまな環境でビルドを実行するために使用できます。

どのexecutorを選択すればよいかわからない場合は、[executorを選択する](#selecting-the-executor)を参照してください。

各executorでサポートされている機能の詳細については、[互換性チャート](#compatibility-chart)を参照してください。

GitLab Runnerは次のexecutorを提供します。

- [SSH](ssh.md)
- [Shell](shell.md)
- [Parallels](parallels.md)
- [VirtualBox](virtualbox.md)
- [Docker](docker.md)
- [Docker Autoscaler](docker_autoscaler.md)
- [Docker Machine（自動スケーリング）](docker_machine.md)
- [Kubernetes](../executors/kubernetes/_index.md)
- [Instance](instance.md)
- [Custom](custom.md)

これらのexecutorはロックされており、新規のexecutorの開発や受け入れは行っていません。詳細については、[新しいexecutorのコントリビュート](https://gitlab.com/gitlab-org/gitlab-runner/blob/main/CONTRIBUTING.md#contributing-new-executors)を参照してください。

## Docker以外のexecutorの前提要件

[ヘルパーイメージに依存しない](../configuration/advanced-configuration.md#helper-image)executorでは、ターゲットマシンと`PATH`にGitがインストールされている必要があります。常に[利用可能な最新バージョンのGit](https://git-scm.com/downloads)を使用してください。

ターゲットマシンに[Git LFS](https://git-lfs.com/)がインストールされている場合、GitLab Runnerは`git lfs`コマンドを使用します。GitLab Runnerがこれらのexecutorを使用するすべてのシステムで、Git LFSが最新であることを確認してください。

`git lfs install`を使用して、GitLab Runnerコマンドを実行するユーザーに対してGit LFSを初期化してください。システム全体でGit LFSを初期化するには、`git lfs install --system`を使用します。

[FF_GIT_URLS_WITHOUT_TOKENS](../configuration/feature-flags.md)を有効にする場合は、Git認証情報ヘルパーなどを使用してビルド間でGit認証情報をキャッシュすることがないようにしないでください。認証情報をキャッシュすると、[`CI_JOB_TOKEN`](https://docs.gitlab.com/ci/jobs/ci_job_token/)が同時ビルドまたは連続ビルド間で共有され、これによって認証エラーやビルドの失敗が発生する可能性があります。

## executorを選択する

executorは、プロジェクトをビルドするためのさまざまなプラットフォームと開発手法をサポートしています。次の表に、使用するexecutorを決定する際に役立つ各executorの重要な情報を示します。

| Executor                                         | SSH  |     Shell      |   VirtualBox   |   Parallels    | Docker | Docker Autoscaler |                 Instance |   Kubernetes   |          Custom          |
|:-------------------------------------------------|:----:|:--------------:|:--------------:|:--------------:|:------:|:-----------------:|-------------------------:|:--------------:|:------------------------:|
| すべてのビルドのためのクリーンなビルド環境          |  ✗   |       ✗        |       ✓        |       ✓        |   ✓    |         ✓         | 条件付き<sup>4</sup> |       ✓        | 条件付き<sup>4</sup> |
| 存在する場合は、以前のクローンを再利用する                |  ✓   |       ✓        |       ✗        |       ✗        |   ✓    |         ✓         | 条件付き<sup>4</sup> | ✓ <sup>6</sup> | 条件付き<sup>4</sup> |
| Runnerファイルシステムへのアクセスが保護されている<sup>5</sup> |  ✓   |       ✗        |       ✓        |       ✓        |   ✓    |         ✓         |                        ✗ |       ✓        |       条件付き        |
| Runnerマシンを移行する                           |  ✗   |       ✗        |    部分的     |    部分的     |   ✓    |         ✓         |                        ✓ |       ✓        |            ✓             |
| 同時ビルドのゼロ設定サポート |  ✗   | ✗ <sup>1</sup> |       ✓        |       ✓        |   ✓    |         ✓         |                        ✓ |       ✓        | 条件付き<sup>4</sup> |
| 複雑なビルド環境                   |  ✗   | ✗ <sup>2</sup> | ✓ <sup>3</sup> | ✓ <sup>3</sup> |   ✓    |         ✓         |           ✗ <sup>2</sup> |       ✓        |            ✓             |
| ビルドの問題のデバッグ                         | 簡単 |      簡単      |      難しい      |      難しい      | 普通 |      普通       |                   普通 |     普通     |          普通          |

**脚注:**

1. ビルドマシンにインストールされているサービスをビルドで使用する場合、executorを選択できますが、問題があります。
1. 依存関係を手動でインストールする必要があります。
1. たとえば、[Vagrant](https://developer.hashicorp.com/vagrant/docs/providers/virtualbox "VirtualBoxのVagrantドキュメント")を使用します。
1. プロビジョニングする環境によって異なります。完全に分離することも、ビルド間で共有することもできます。
1. Runnerのファイルシステムアクセスが保護されていない場合、ジョブはRunnerのトークンや他のジョブのキャッシュとコードなど、システム全体にアクセスできます。✓が付いているexecutorは、デフォルトではRunnerがファイルシステムにアクセスすることを許可していません。ただし、セキュリティ上の欠陥または特定の設定により、ジョブがコンテナからブレイクアウトし、Runnerをホストしているファイルシステムにアクセスする可能性があります。
1. [並行処理ごとの永続ビルドボリューム](kubernetes/_index.md#persistent-per-concurrency-build-volumes)設定が必要です。

### Shell executor

Shell executorは、GitLab Runnerの最もシンプルな設定オプションです。GitLab Runnerがインストールされているシステムでジョブをローカルに実行し、すべての依存関係を同じマシンに手動でインストールする必要があります。

このexecutorは、Linux、macOS、およびFreeBSDオペレーティングシステムではBashをサポートし、Windows環境ではPowerShellをサポートしています。

最小限の依存関係を持つビルドにとって理想的ですが、ジョブ間の分離は限定的です。

### Docker executor

Docker executorは、コンテナを介してクリーンなビルド環境を提供します。すべての依存関係がDockerイメージにパッケージ化されているため、依存関係を容易に管理できます。このexecutorを使用するには、RunnerホストにDockerがインストールされている必要があります。

このexecutorは、MySQLなどの追加の[サービス](https://docs.gitlab.com/ci/services/)をサポートしています。また、Podmanを代替コンテナランタイムとして受け入れます。

このexecutorは、一貫性のある分離されたビルド環境を保持します。

### Docker Machine Executor（非推奨）

{{< alert type="warning" >}}

この機能はGitLab 17.5で[非推奨](https://gitlab.com/gitlab-org/gitlab/-/issues/498268)になりました。20.0で削除される予定です。代わりに[GitLab Runner Autoscaler](../runner_autoscale/_index.md)を使用してください。

{{< /alert >}}

Docker Machine Executorは、自動スケーリングに対応しているDocker executorの特別なバージョンです。標準的なDocker executorと同様に動作しますが、Docker Machineによってオンデマンドで作成されたビルドホストを使用します。この機能により、このexecutorはAWS EC2などのクラウド環境で特に効果的であり、さまざまなワークロードに対して優れた分離性とスケーラビリティを提供します。

### Docker Autoscaler executor

Docker Autoscaler executorは、Runnerマネージャーが処理するジョブに対処するために、オンデマンドでインスタンスを作成する自動スケール対応のDocker executorです。[Docker executor](docker.md)をラップしているため、すべてのDocker executorのオプションと機能がサポートされています。

Docker Autoscalerは、[フリートプラグイン](https://gitlab.com/gitlab-org/fleeting/fleeting)を使用して自動スケールします。フリートとは、自動スケールされたインスタンスのグループの抽象化であり、Google Cloud、AWS、Azureなどのクラウドプロバイダーをサポートするプラグインを使用します。このexecutorは、動的なワークロードの要件がある環境に特に適しています。

### Instance executor

Instance executorは、Runnerマネージャーが処理するジョブの予期されるボリュームに対処するために、オンデマンドでインスタンスを作成する自動スケール対応のexecutorです。

このexecutorと、関連するDocker Autoscale executorは、GitLab RunnerフリートおよびTaskscalerテクノロジーと連携する新しい自動スケールexecutorです。

Instance executorも[フリートプラグイン](https://gitlab.com/gitlab-org/fleeting/fleeting)を使用して自動スケールします。

ジョブがホストインスタンス、オペレーティングシステム、および接続デバイスへのフルアクセスを必要とする場合は、Instance executorを使用できます。Instance executorは、シングルテナントジョブとマルチテナントジョブに対応するように設定することもできます。

### Kubernetes executor

ビルドに既存のKubernetesクラスターを使用する場合にKubernetes executorを使用できます。このexecutorはKubernetesクラスターAPIを呼び出して、各GitLab CI/CDジョブの新しいポッド（ビルドコンテナとサービスコンテナを含む）を作成します。このexecutorは、クラウドネイティブ環境に特に適しており、優れたスケーラビリティとリソース利用率を実現します。

### SSH executor

SSH executorは完全性を期すために追加されましたが、サポートが最も少ないexecutorの1つです。SSH executorを使用すると、GitLab Runnerは外部サーバーに接続し、そこでビルドを実行します。このexecutorを使用している組織からの成功事例がいくつかありますが、通常は他のタイプのexecutorを使用してください。

### Custom executor

Custom executorを使用すると、独自の実行環境を指定できます。GitLab Runnerがexecutor（Linuxコンテナなど）を提供しない場合、カスタムの実行可能ファイルを使用して環境をプロビジョニングよびクリーンアップできます。

## 互換性チャート

各種executorでサポートされている機能を以下に示します。

| Executor                                     | SSH            | Shell          |VirtualBox      | Parallels      | Docker  | Docker Autoscaler | Instance       | Kubernetes | Custom                                                       |
|:---------------------------------------------|:--------------:|:--------------:|:--------------:|:--------------:|:-------:|:-----------------:|:--------------:| :---------:| :-----------------------------------------------------------:|
| セキュア変数                             | ✓              | ✓              | ✓              | ✓              | ✓       | ✓                 | ✓              | ✓          | ✓                                                           |
| `.gitlab-ci.yml`: イメージ                      | ✗              | ✗              | ✓（1）          | ✓（1）          | ✓       | ✗                 | ✗              | ✓          | ✓（[`$CUSTOM_ENV_CI_JOB_IMAGE`](custom.md#stages)を使用） |
| `.gitlab-ci.yml`: サービス                   | ✗              | ✗              | ✗              | ✗              | ✓       | ✗                 | ✗              | ✓          | ✓      |
| `.gitlab-ci.yml`: キャッシュ                      | ✓              | ✓              | ✓              | ✓              | ✓       | ✓                 | ✓              | ✓          | ✓      |
| `.gitlab-ci.yml`: アーティファクト                  | ✓              | ✓              | ✓              | ✓              | ✓       | ✓                 | ✓              | ✓          | ✓      |
| ステージ間のアーティファクトの受け渡し             | ✓              | ✓              | ✓              | ✓              | ✓       | ✓                 | ✓              | ✓          | ✓      |
| GitLabコンテナレジストリのプライベートイメージを使用する | 該当なし | 該当なし | 該当なし | 該当なし | ✓       | ✓                 | 該当なし | ✓          | 該当なし |
| インタラクティブウェブターミナル                     | ✗              | ✓（UNIX）       | ✗              | ✗              | ✓       | ✗                 | ✗              | ✓          | ✗              |

1. GitLab Runner 14.2でサポートが[追加](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/1257)されました。詳細については、[ベースVMイメージの上書き](../configuration/advanced-configuration.md#overriding-the-base-vm-image)セクションを参照してください。

各種Shellでサポートされているシステムを以下に示します。

| Shell  | Bash        | PowerShell Desktop | PowerShell Core | Windows Batch（非推奨） |
|:-------:|:-----------:|:------------------:|:---------------:|:--------------------------:|
| Windows | ✗（4）       | ✓（3）              | ✓               | ✓（2）                      |
| Linux   | ✓（1）       | ✗                  | ✓               | ✗                          |
| macOS   | ✓（1）       | ✗                  | ✓               | ✗                          |
| FreeBSD | ✓（1）       | ✗                  | ✗               | ✗                          |

1. デフォルトのShell。
1. 非推奨。[`shell`](../configuration/advanced-configuration.md#the-runners-section)が指定されていない場合のデフォルトのShell。
1. 新しいRunnerの登録時のデフォルトのShell。
1. WindowsのBash Shellはサポートされていません。

各種Shellによりサポートされているインタラクティブウェブターミナルのシステムを以下に示します。

| Shell  | Bash        | PowerShell Desktop    | PowerShell Core    | Windows Batch（非推奨） |
|:-------:|:-----------:|:---------------------:|:------------------:|:--------------------------:|
| Windows | ✗           | ✗                     | ✗                  | ✗                          |
| Linux   | ✓           | ✗                     | ✗                  | ✗                          |
| macOS   | ✓           | ✗                     | ✗                  | ✗                          |
| FreeBSD | ✓           | ✗                     | ✗                  | ✗                          |

次の図は、オペレーティングシステムとプラットフォームに基づいてどのexecutorを選択すべきかを示しています。

```mermaid
graph TD
    Start[Which executor to choose?] --> BuildType{Autoscaling or No Autosclaing?}


    BuildType -->|No| BuildType2{Container or OS Shell builds?}
    BuildType-->|Yes| Platform{Platform}
    BuildType2 -->|Shell| ShellOptions{Operating System}
    BuildType2 -->|Container| ContainerOptions{Operating System}


    Platform -->|Cloud Native| Kubernetes[Kubernetes]
    Platform -->|Cloud VMs| OSType{Operating System}

    OSType -->|Windows| WinExec{Executor Type}
    OSType -->|macOS| MacExec{Executor Type}
    OSType -->|Linux| LinuxExec{Executor Type}


    WinExec --> AutoscalerWin[Fleeting: Docker Autoscaler Executor]
    WinExec --> InstanceWin[Fleeting:Instance Executor]

    MacExec --> AutoscalerMac[Fleeting: Docker Autoscaler Executor]
    MacExec --> InstanceMac[Fleeting:Instance Executor]

    LinuxExec --> AutoscalerLin[Fleeting: Docker Autoscaler Executor]
    LinuxExec --> InstanceLin[Fleeting:Instance Executor]


    ShellOptions -->|Linux| Linux_Shell[Bash;Zsh]
    ShellOptions -->|macOS| MacOS[Bash;Zsh]
    ShellOptions -->|Windows| Windows[Powershell 5.1; PowerShell 7.x]
    ShellOptions -->|Remote Machine| SSH[SSH]


    ContainerOptions -->|Linux| Linux_Shell2[Docker;Podman]
    ContainerOptions -->|macOS| macOS2[Docker]
    ContainerOptions -->|Windows| Windows2[Docker]

    %% Styling
    classDef default fill:#f9f9f9,stroke:#333,stroke-width:2px;
    classDef decision fill:#e1f3fe,stroke:#333,stroke-width:2px;
    classDef executor fill:#dcffe4,stroke:#333,stroke-width:2px;

    class Start default;
    class BuildType,BuildType2,Container,Scaling,AutoScale,NoAutoScale,ShellOptions,ContainerOptions,OSType,WinExec,MacExec,Platform,LinuxExec decision;
    class Kubernetes,Docker,Custom,Shell,Windows,SSH,DockerMachineWin,AutoscalerWin,InstanceWin,DockerMachineMac,AutoscalerMac,InstanceMac,DockerMachineLin,AutoscalerLin,InstanceLin executor;
```
