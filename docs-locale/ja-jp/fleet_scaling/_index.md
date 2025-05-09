---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: インスタンスRunnerまたはグループRunnerのRunnerフリートを計画および運用する
---

このガイドでは、共有サービスモデルでRunnerフリートをスケーリングするためのベストプラクティスについて説明します。

インスタンスRunnerフリートをホストする場合は、以下を考慮して十分に計画されたインフラストラクチャが必要です。

- コンピューティングキャパシティ。
- ストレージキャパシティ。
- ネットワークの帯域幅とスループット。
- ジョブの種類（プログラミング言語、OSプラットフォーム、依存関係ライブラリなど）。

このガイドを使用して、組織の要件に基づいてGitLab Runnerのデプロイ戦略を策定します。

このガイドでは、使用する必要があるインフラストラクチャの種類に関する具体的な推奨事項は示されていません。ただし、毎月数百万件のCI/CDジョブを処理するGitLab.comでRunnerフリートを運用した経験から得られたインサイトを説明しています。

## ワークロードと環境を検討する

Runnerをデプロイする前に、ワークロードと環境の要件を検討してください。

- GitLabにオンボードする予定のチームのリストを作成します。
- 組織で使用しているプログラミング言語、Webフレームワーク、およびライブラリをカタログ化します。たとえば、Go、C++、PHP、Java、Python、JavaScript、React、Node.jsなどです。
- 各チームが1日あたり、1時間ごとに実行するCI/CDジョブの数を推定します。
- いずれかのチームに、コンテナを使用しても対処できないビルド環境要件があるかどうかを検証します。
- いずれかのチームに、チーム専用のRunnerを用意することで最適に対応できるビルド環境要件があるかどうかを検証します。
- 予想される需要に対応するために必要なコンピューティングキャパシティを見積もります。

さまざまなRunnerフリートをホストするために、異なるインフラストラクチャスタックを選択できます。たとえば、パブリッククラウドにデプロイする必要があるRunnerと、オンプレミスにデプロイする必要があるRunnerがある場合があります。

RunnerフリートでのCI/CDジョブのパフォーマンスは、フリートの環境に直接関係しています。大量のリソースを消費するCI/CDジョブを多数実行している場合、共有コンピューティングプラットフォームでRunnerフリートをホストすることはお勧めできません。

## Runner、executor、および自動スケール機能

`gitlab-runner`実行可能ファイルはCI/CDジョブを実行します。各Runnerは、ジョブ実行のリクエストを取得し、事前定義された設定に従って処理する分離プロセスです。各Runnerは分離プロセスとして、ジョブを実行するための「サブプロセス」（「ワーカー」とも呼ばれる）を作成できます。

### 同時実行数と制限

- [同時実行数](../configuration/advanced-configuration.md#the-global-section):ホストシステムで設定済みのすべてのRunnerを使用している場合に、同時実行できるジョブの数を設定します。
- [制限](../configuration/advanced-configuration.md#the-runners-section):Runnerがジョブの同時実行のために作成できるサブプロセスの数を設定します。

この制限は、（Docker MachineやKubernetesのような）自動スケールRunnerと、自動スケールしないRunnerでは異なります。

- 自動スケールしないRunnerでは、`limit`はホストシステムのRunnerのキャパシティを定義します。
- 自動スケールRunnerでは、`limit`は実行するRunnerの合計数です。

### 基本設定: 1つのRunnerマネージャー、1つのRunner

最も基本的な設定では、サポートされているコンピューティングアーキテクチャとオペレーティングシステムにGitLab Runnerソフトウェアをインストールします。たとえば、Ubuntu Linuxを実行しているx86-64仮想マシン（VM）があるとします。

インストールが完了したら、Runnerの登録コマンドを1回だけ実行し、`shell` executorを選択します。次にRunnerの`config.toml`ファイルを編集して、同時実行数を`1`に設定します。

```toml
concurrent = 1

[[runners]]
  name = "instance-level-runner-001"
  url = ""
  token = ""
  executor = "shell"
```

このRunnerが処理できるGitLab CI/CDジョブは、Runnerをインストールしたホストシステム上で直接実行されます。これは、ターミナルでCI/CDジョブコマンドを自分で実行する場合と同様です。この場合、登録コマンドを1回だけ実行したため、`config.toml`ファイルには1つの`[[runners]]`セクションのみが含まれています。同時実行数の値を`1`に設定した場合、1つのRunner「ワーカー」のみがこのシステムのRunnerプロセスでCI/CD ジョブを実行できます。

### 中程度の設定: 1つのRunnerマネージャー、複数のRunner

同じマシンに複数のRunnerを登録することもできます。このように登録すると、Runnerの`config.toml`ファイルに複数の`[[runners]]`セクションが含まれます。追加のすべてのRunnerワーカーがShell executorを使用している場合に、グローバルの`concurrent`設定の値を`3`に更新すると、ホストは一度に最大3つのジョブを実行できます。

```toml
concurrent = 3

[[runners]]
  name = "instance_level_shell_001"
  url = ""
  token = ""
  executor = "shell"

[[runners]]
  name = "instance_level_shell_002"
  url = ""
  token = ""
  executor = "shell"

[[runners]]
  name = "instance_level_shell_003"
  url = ""
  token = ""
  executor = "shell"

```

同じマシンに複数のRunnerワーカーを登録でき、各ワーカーは分離プロセスになります。各ワーカーのCI/CDジョブのパフォーマンスは、ホストシステムのコンピューティングキャパシティに依存します。

### 自動スケール設定: 1つ以上のRunnerマネージャー、複数のワーカー

自動スケール用にGitLab Runnerがセットアップされている場合、1つのRunnerが他のRunnerのマネージャーとして機能するように設定できます。これは、`docker-machine` executorまたは`kubernetes` executorで行うことができます。このようなマネージャーのみの設定では、Runnerエージェント自体はCI/CDジョブを実行しません。

#### Docker Machine executor

[Docker Machine Executor](../executors/docker_machine.md)を使用する場合、次のようになります。

- Runnerマネージャーは、Dockerを使用してオンデマンドの仮想マシンインスタンスをプロビジョニングします。
- これらのVMで、GitLab Runnerは`.gitlab-ci.yml`ファイルに指定されているコンテナイメージを使用して、CI/CDジョブを実行します。
- さまざまなマシンタイプでCI/CDジョブのパフォーマンスをテストする必要があります。
- スピードまたはコストに基づいてコンピューティングホストを最適化することを検討する必要があります。

#### Kubernetes executor

[Kubernetes executor](../executors/kubernetes/_index.md)を使用する場合、次のようになります。

- Runnerマネージャーが、ターゲットのKubernetesクラスターでポッドをプロビジョニングします。
- CI/CDジョブは、複数のコンテナで構成される各ポッドで実行されます。
- ジョブの実行に使用されるポッドは通常、Runnerマネージャーをホストするポッドよりも多くのコンピューティングとメモリリソースを必要とします。

#### Runner設定を再利用する

同じRunner認証トークンに関連付けられている各Runnerマネージャーには、`system_id`識別子が割り当てられます。`system_id`は、Runnerが使用されているマシンを識別します。同じ認証トークンで登録されたRunnerは、一意の`system_id.`によって1つのRunnerエントリにグループ化されます。

類似するRunnerを1つの設定にグループ化すると、Runnerフリートのオペレーションが簡素化されます。

類似するRunnerを1つの設定にグループ化できるシナリオの例を次に示します。

プラットフォーム管理者は、タグ`docker-builds-2vCPU-8GB`を使用して、基盤となる仮想マシンインスタンスサイズ（2 vCPU、8 GB RAM）が同じである複数のRunnerを指定する必要があります。高可用性またはスケーリングのために、このようなRunnerが少なくとも2つ必要です。UIで2つの個別のRunnerエントリを作成する代わりに、管理者は同じコンピューティングインスタンスサイズを持つすべてのRunnerに対して1つのRunner 設定を作成できます。複数のRunnerを登録するために、Runner設定に認証トークンを再利用できます。登録された各Runnerは`docker-builds-2vCPU-8GB`タグを継承します。1つのRunner設定のすべての子Runnerに対して、`system_id`は固有識別子として機能します。

グループにまとめられたRunnerは、複数のRunnerマネージャーによってさまざまなジョブを実行するために再利用できます。

GitLab Runnerは、起動時、または設定の保存時に`system_id`を生成します。`system_id`は、[`config.toml`](../configuration/advanced-configuration.md)lと同じディレクトリ内の`.runner_system_id`ファイルに保存され、ジョブログとRunner管理ページに表示されます。

##### `system_id`識別子を生成する

GitLab Runnerは`system_id`を生成するために、ハードウェア識別子（一部のLinuxディストリビューションの`/etc/machine-id`など）から一意のシステム識別子を派生しようと試みます。この操作が成功しなかった場合、GitLab Runnerはランダムな識別子を使用して`system_id`を生成します。

`system_id`には、次のいずれかのプレフィックスが付いています。

- `r_`:GitLab Runnerがランダムな識別子を割り当てました。
- `s_`:GitLab Runnerがハードウェア識別子から一意のシステム識別子を割り当てました。

たとえば、`system_id`がイメージにハードコードされないように、コンテナイメージを作成する際にこの点を考慮することが重要です。`system_id`がハードコードされている場合、特定のジョブを実行しているホストを区別できません。

##### RunnerとRunnerマネージャーを削除する

Runner登録トークン（非推奨）を使用して登録されたRunnerとRunnerマネージャーを削除するには、`gitlab-runner unregister`コマンドを使用します。

Runner認証トークンを使用して作成されたRunnerとRunnerマネージャーを削除するには、[UI](https://docs.gitlab.com/ci/runners/runners_scope/#delete-instance-runners)または[API](https://docs.gitlab.com/api/runners/#delete-a-runner)を使用します。Runner認証トークンを使用して作成されたRunnerは再利用可能な設定であり、複数のマシンで再利用できます。[`gitlab-runner unregister`](../commands/_index.md#gitlab-runner-unregister)コマンドを使用すると、Runnerマネージャーのみが削除され、Runnerは削除されません。

## インスタンスRunnerを設定する

効率的かつ効果的な開始方法は、自動スケール設定（Runnerが「Runnerマネージャー」として機能する設定）でインスタンスRunnerを使用することです。

VMまたはポッドをホストするインフラストラクチャスタックのコンピューティングキャパシティは、以下の条件によって異なります。

- ワークロードと環境を検討する際に特定した要件。
- Runnerフリートをホストするために使用するテクノロジースタック。

CI/CDワークロードの実行と、経時的なパフォーマンスの分析を開始した後で、場合によってはコンピューティングキャパシティを調整する必要があります。

インスタンスRunnerと自動スケールexecutorを使用する設定では、最小限の2つのRunnerマネージャーで開始する必要があります。

時間の経過とともに必要になるRunnerマネージャーの合計数は、以下の条件によって異なります。

- Runnerマネージャーをホストするスタックのコンピューティングリソース。
- 各Runnerマネージャーに設定する同時実行数。
- 各マネージャーが毎時、毎日、毎月実行するCI/CDジョブによって生成される負荷。

たとえばGitLab.comでは、Docker Machine Executorで7つのRunnerマネージャーを実行します。各CI/CDジョブは、Google Cloud Platform（GCP）`n1-standard-1` VMで実行されます。この設定では、毎月数百万件のジョブを処理します。

## Runnerをモニタリングする

大規模なRunnerフリートを運用する上で不可欠なステップは、GitLabに含まれている[Runnerモニタリング](../monitoring/_index.md)機能をセットアップして使用することです。

次の表に、GitLab Runnerメトリクスの概要を示します。このリストには、Go固有のプロセスメトリクスは含まれていません。Runnerでこれらのメトリクスを表示するには、[利用可能なメトリクス](../monitoring/_index.md#available-metrics)に示されているようにコマンドを実行します。

| メトリクス名 | 説明 |
| ------ | ------ |
| `gitlab_runner_api_request_statuses_total` | Runner、エンドポイント、ステータスに基づいてパーティショニングされたAPIリクエストの総数。 |
| `gitlab_runner_autoscaling_machine_creation_duration_seconds` | マシン作成時間のヒストグラム。|
| `gitlab_runner_autoscaling_machine_states`  | このプロバイダーの状態別のマシンの数。 |
| `gitlab_runner_concurrent` | concurrent設定の値。 |
| `gitlab_runner_errors_total` | キャッチされたエラーの数。このメトリクスは、ログの行を追跡するカウンターです。このメトリクスには`level`というラベルが含まれています。使用可能な値は`warning`と`error`です。このメトリクスを含める場合は、監視時に`rate()`または`increase()`を使用してください。つまり、警告またはエラーの発生率が上昇していることが判明した場合には、詳しい調査が必要な問題を示唆している可能性があります。 |
| `gitlab_runner_jobs` | これにより、（ラベル内のさまざまなスコープで）実行されているジョブの数が表示されます。 |
| `gitlab_runner_job_duration_seconds` | ジョブ期間のヒストグラム。 |
| `gitlab_runner_job_queue_duration_seconds` | ジョブキュー期間を表すヒストグラム。 |
| `gitlab_runner_acceptable_job_queuing_duration_exceeded_total` | 設定されたキューイング時間のしきい値をジョブが超過する頻度をカウントします。 |
| `gitlab_runner_job_stage_duration_seconds` | 各ステージのジョブ期間を表すヒストグラム。このメトリクスは**高カーディナリティメトリクス**です。詳細については、[高カーディナリティメトリクスのセクション](#high-cardinality-metrics)を参照してください。 |
| `gitlab_runner_jobs_total` | 実行されたジョブの合計数を表示します。 |
| `gitlab_runner_limit` | 制限設定の現在の値。 |
| `gitlab_runner_request_concurrency` | 新しいジョブに対する現在の同時リクエストの数。 |
| `gitlab_runner_request_concurrency_exceeded_total` | 設定されている`request_concurrency`制限を超える過剰なリクエストの数。 |
| `gitlab_runner_version_info` | さまざまなビルド統計フィールドでラベル付けされている、定数値`1`を持つメトリクス。 |
| `process_cpu_seconds_total` | 消費されたユーザーCPU時間とシステムCPU時間の合計（秒単位）。 |
| `process_max_fds`  | オープンファイル記述子の最大数。 |
| `process_open_fds` | オープンファイル記述子の数。 |
| `process_resident_memory_bytes`  | 常駐メモリのサイズ（バイト単位）。 |
| `process_start_time_seconds` | Unixエポックからの秒数で測定された、プロセスの開始時間。 |
| `process_virtual_memory_bytes` | 仮想メモリのサイズ（バイト単位）。 |
| `process_virtual_memory_max_bytes` | 利用可能な仮想メモリの最大量（バイト単位）。 |

### Grafanaダッシュボードの設定に関するヒント

この[公開リポジトリ](https://gitlab.com/gitlab-com/runbooks/-/tree/master/dashboards/ci-runners)には、GitLab.comでRunnerフリートを運用するために使用するGrafanaダッシュボードのソースコードがあります。

GitLab.comの多数のメトリクスを追跡しています。クラウドベースのCI/CDの大規模プロバイダーとして、イシューをデバッグできるように、システムをさまざまな観点から把握する必要があります。ほとんどの場合、Self-Managed Runnerフリートは、GitLab.comで追跡している大量のメトリクスを追跡する必要はありません。

Runnerフリートのモニタリングに使用する必要がある重要なダッシュボードの一部を以下に示します。

**Jobs started on runners**:

- 選択した時間間隔にわたってRunnerフリートで実行されたジョブの合計の概要を表示します。
- 使用状況の傾向を表示します。このダッシュボードは、少なくとも毎週分析する必要があります。

このデータをジョブ期間などのメトリクスに関連付けて、CI/CDジョブのパフォーマンスSLOを満たすために、設定の変更が必要かどうか、またはキャパシティのアップグレードが必要かどうかを判断できるようにします。

**Job duration**:

- Runnerフリートのパフォーマンスとスケーリングを分析します。

**Runner capacity**:

- 実行中のジョブの数を、limitまたはconcurrentの値で割った値を表示します。
- 追加のジョブを実行できるキャパシティがまだあるかどうかを判断します。

### KubernetesでのRunnerのモニタリングに関する考慮事項

OpenShift、Amazon EKS、GKEなどのKubernetesプラットフォームでホストされているRunnerフリートの場合は、別の方法でGrafanaダッシュボードをセットアップします。

Kubernetesでは、Runner CI/CDジョブ実行ポッドを頻繁に作成および削除することがあります。このような場合は、Runnerマネージャーポッドをモニタリングし、次の機能を実装する予定を立てておく必要があります。

- ゲージ:異なるソースからの同一メトリクスの集計を表示します。
- カウンター:`rate`または`increase`関数を適用するときにカウンターをリセットします。

## 高カーディナリティメトリクス

一部のメトリクスは、高カーディナリティであるために、インジェストおよび保存の際にリソースを大量に消費する可能性があります。高カーディナリティとなるのは、多数の使用可能な値があるラベルがメトリクスに含まれており、これによって大量の一意の時系列データポイントが作成される場合です。

パフォーマンスを最適化するために、このようなメトリクスはデフォルトでは有効になっていません。[FF_EXPORT_HIGH_CARDINALITY_METRICS機能フラグ](../configuration/feature-flags.md)を使用して切り替えることができます。

### 高カーディナリティメトリクスのリスト

- `gitlab_runner_job_stage_duration_seconds`:個々のジョブステージの期間（秒単位）を測定します。このメトリクスには`stage`ラベルが含まれており、次の定義済みの値があります。

  - `resolve_secrets`
  - `prepare_executor`
  - `prepare_script`
  - `get_sources`
  - `clear_worktree`
  - `restore_cache`
  - `download_artifacts`
  - `after_script`
  - `step_script`
  - `archive_cache`
  - `archive_cache_on_failure`
  - `upload_artifacts_on_success`
  - `upload_artifacts_on_failure`
  - `cleanup_file_variables`

  さらに、このリストに`step_run`などのカスタムユーザー定義のステップが含まれる場合があります。

### 高カーディナリティメトリクスを管理する

[Prometheusのrelabel設定](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config)を使用して不要なラベル値またはメトリクス全体を削除することで、カーディナリティを制御および削減できます。

#### 特定のステージを削除する設定の例

次の設定は、`stage`ラベルに`prepare_executor`値が設定されているすべてのメトリクスを削除します。

```yaml
scrape_configs:
  - job_name: 'gitlab_runner_metrics'
    static_configs:
      - targets: ['localhost:9252']
    metric_relabel_configs:
      - source_labels: [__name__, "stage"]
        regex: "gitlab_runner_job_stage_duration_seconds;prepare_executor"
        action: drop
```

#### 関連するステージのみを保持する例

次の設定は、`step_script`ステージのメトリクスのみを保持し、他のメトリクスを完全に破棄します。

```yaml
scrape_configs:
  - job_name: 'gitlab_runner_metrics'
    static_configs:
      - targets: ['localhost:9252']
    metric_relabel_configs:
      - source_labels: [__name__, "stage"]
        regex: "gitlab_runner_job_stage_duration_seconds;step_script"
        action: keep
```
