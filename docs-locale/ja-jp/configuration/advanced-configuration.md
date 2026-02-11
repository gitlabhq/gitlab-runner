---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: 高度な設定
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

GitLab Runnerと個別に登録されたRunnerの動作を変更するには、`config.toml`ファイルを修正します。

`config.toml`ファイルは次の場所にあります。

- GitLab Runnerがrootとして実行される場合、\*nixシステムでは`/etc/gitlab-runner/`にあります。このディレクトリは、サービス設定のパスでもあります。
- GitLab Runnerが非rootユーザーとして実行される場合、\*nixシステムでは`~/.gitlab-runner/`にあります。
- その他のシステムの`./`。

ほとんどのオプションでは、オプションを変更した場合にGitLab Runnerを再起動する必要はありません。これには、`[[runners]]`セクションのパラメータと`listen_address`を除くグローバルセクションのほとんどのパラメータが含まれます。Runnerがすでに登録されている場合は、再度登録する必要はありません。

GitLab Runnerは、設定の変更を3秒ごとに確認し、必要に応じて再読み込みします。またGitLab Runnerは、`SIGHUP`シグナルに応答して設定を再読み込みします。

## 設定検証 {#configuration-validation}

{{< history >}}

- GitLab Runner 15.10で[導入](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/3924)されました。

{{< /history >}}

設定検証は、`config.toml`ファイルの構造をチェックするプロセスです。設定バリデーターからの出力は、`info`レベルのメッセージのみを示します。

設定検証プロセスは、情報提供のみを目的としています。この出力から、Runner設定に関する潜在的な問題を特定できます。設定検証では、起こり得るすべての問題を検出できるとは限りません。また、メッセージがないからといって、`config.toml`ファイルに欠陥がないことが保証されるわけではありません。

## グローバルセクション {#the-global-section}

これらの設定はグローバルなものです。すべてのRunnerに適用されます。

| 設定              | 説明 |
|----------------------|-------------|
| `concurrent`         | 登録されているすべてのRunnerで同時に実行できるジョブ数を制限します。各`[[runners]]`セクションで独自の制限を定義できますが、この値はそれらのすべての値を合計した最大値を設定します。たとえば、値が`10`の場合、同時に実行できるジョブは最大10個までとなります。`0`は禁止されています。この値を使用すると、Runnerプロセスは重大なエラーで終了します。[Docker Machine executor](autoscale.md#limit-the-number-of-vms-created-by-the-docker-machine-executor)、[インスタンスexecutor](../executors/instance.md)、[Docker Autoscaler executor](../executors/docker_autoscaler.md#example-aws-autoscaling-for-1-job-per-instance)、[`runners.custom_build_dir`設定](#the-runnerscustom_build_dir-section)でこの設定がどのように機能するかをご確認ください。 |
| `log_level`          | ログレベルを定義します。オプションには、`debug`、`info`、`warn`、`error`、`fatal`、`panic`があります。この設定は、コマンドライン引数の`--debug`、`-l`、または`--log-level`で設定されるレベルよりも優先度が低くなります。 |
| `log_format`         | ログ形式を指定します。オプションには、`runner`、`text`、`json`があります。この設定は、コマンドライン引数の`--log-format`で設定される形式よりも優先度が低くなります。デフォルト値は`runner`で、色分けのためのANSIエスケープコードが含まれています。 |
| `check_interval`     | Runnerが新しいジョブを確認する間隔を秒単位で定義します。デフォルト値は`3`です。`0`以下に設定すると、デフォルト値が使用されます。 |
| `sentry_dsn`         | Sentryへのすべてのシステムレベルのエラーの追跡を有効にします。 |
| `connection_max_age` | GitLabサーバーへのTLSキープアライブ接続を再接続するまでの最大時間を指定します。デフォルト値は`15m`（15分）です。`0`以下に設定すると、接続は可能な限り持続します。 |
| `listen_address`     | Prometheusメトリクス用HTTPサーバーがリッスンするアドレス（`<host>:<port>`）を定義します。 |
| `shutdown_timeout`   | [強制シャットダウン操作](../commands/_index.md#signals)がタイムアウトになりプロセスが終了するまでの秒数を示します。デフォルト値は`30`です。`0`以下に設定すると、デフォルト値が使用されます。 |

### 設定の警告 {#configuration-warnings}

#### ロングポーリングのイシュー {#long-polling-issues}

GitLab Runnerは、GitLabのロングポーリングがGitLab Workhorseを介してオンになっている場合、いくつかの設定シナリオでロングポーリングのイシューが発生する可能性があります。これらは、設定に応じて、パフォーマンスのボトルネックから重大な処理遅延まで多岐にわたります。GitLab Runnerのワーカーは、長時間（GitLab Workhorseの設定である`-apiCiLongPollingDuration`（デフォルトは50秒）と一致）ロングポーリングリクエストで停止し、他のジョブが迅速に処理されるのを妨げる可能性があります。

このイシューは、GitLab Workhorseの`-apiCiLongPollingDuration`設定によって制御されるGitLab CI/CDのロングポーリング機能に関連しています。オンにすると、ジョブリクエストは、ジョブが利用可能になるのを待機している間、設定された時間までブロックされる可能性があります。

デフォルトのGitLab Workhorseのロングポーリングの設定値は50秒です（最近のGitLabバージョンではデフォルトでオンになっています）。

次に、設定例をいくつか示します:

- Omnibus：`gitlab_workhorse['api_ci_long_polling_duration'] = "50s"` in `/etc/gitlab/gitlab.rb`
- Helmチャート: `gitlab.webservice.workhorse.extraArgs`設定を使用
- CLI：`gitlab-workhorse -apiCiLongPollingDuration 50s`

詳細については、以下を参照してください: 

- [Runnerのロングポーリング](https://docs.gitlab.com/ci/runners/long_polling/)
- [Workhorse](https://docs.gitlab.com/development/workhorse/configuration/)の設定

**Symptoms:**

- 一部のプロジェクトからのジョブは、開始前に遅延が発生します（時間は、GitLabインスタンスのロングポーリングのタイムアウトと一致します）。
- 他のプロジェクトからのジョブはすぐに実行されます
- Runnerログの警告メッセージ：`CONFIGURATION: Long polling issues detected`

**Common problematic scenarios:**

- ワーカーのスターベーションボトルネック: `concurrent`設定がRunnerの数よりも少ない（重大なボトルネック）
- リクエストのボトルネック: `request_concurrency=1`のRunnerは、ロングポーリング中にジョブの遅延を引き起こします
- ビルド制限のボトルネック: `limit`設定（≤2）が低いRunnerと`request_concurrency=1`の組み合わせ

**Solution options:**

GitLab Runnerは、問題のあるシナリオを自動的に検出し、警告メッセージで調整されたソリューションを提供します。一般的な解決策は次のとおりです:

- Runnerの数を超えるように`concurrent`設定を増やします。
- 高ボリュームのRunnerの`request_concurrency`値を1より大きい値に設定します（デフォルトは1）。システムのステートを理解し、設定に最適な値を見つけるために、[Runnerのモニタリング](../monitoring/_index.md)をオンにすることを検討してください。ワークロードに基づいて`request_concurrency`を自動的に調整するには、`FF_USE_ADAPTIVE_REQUEST_CONCURRENCY`機能フラグを使用することを検討してください。適応的な並行処理については、[機能フラグ](feature-flags.md)のドキュメントを参照してください。
- `limit`設定と予想されるジョブボリュームのバランスを取ります。

**Example problematic configurations:**

**シナリオ1: ワーカーのスターベーションボトルネック**

```toml
concurrent = 2  # Only 2 concurrent workers

[[runners]]
  name = "runner-1"
[[runners]]
  name = "runner-2"
[[runners]]
  name = "runner-3"  # 3 runners, only 2 workers - severe bottleneck
```

**シナリオ2: リクエストのボトルネック**

```toml
concurrent = 4  # 4 workers available

[[runners]]
  name = "high-volume-runner"
  request_concurrency = 1  # Default: only 1 request at a time
  limit = 10               # Can handle 10 jobs, but only 1 request slot
```

**シナリオ3: ビルド制限のボトルネック**

```toml
concurrent = 4

[[runners]]
  name = "limited-runner"
  limit = 2                # Only 2 builds allowed
  request_concurrency = 1  # Only 1 request at a time
  # Creates severe bottleneck: builds at capacity + request slot blocked by long polling
```

**Example corrected configuration:**

```toml
concurrent = 4  # Adequate worker capacity

[[runners]]
  name = "high-volume-runner"
  request_concurrency = 3  # Allow multiple simultaneous requests
  limit = 10

[[runners]]
  name = "balanced-runner"
  request_concurrency = 2
  limit = 5
```

設定例

```toml

# Example `config.toml` file

concurrent = 100 # A global setting for job concurrency that applies to all runner sections defined in this `config.toml` file
log_level = "warning"
log_format = "text"
check_interval = 3 # Value in seconds

[[runners]]
  name = "first"
  url = "Your Gitlab instance URL (for example, `https://gitlab.com`)"
  executor = "shell"
  (...)

[[runners]]
  name = "second"
  url = "Your Gitlab instance URL (for example, `https://gitlab.com`)"
  executor = "docker"
  (...)

[[runners]]
  name = "third"
  url = "Your Gitlab instance URL (for example, `https://gitlab.com`)"
  executor = "docker-autoscaler"
  (...)

```

### `log_format`の例（一部） {#log_format-examples-truncated}

#### `runner` {#runner}

```shell
Runtime platform                                    arch=amd64 os=darwin pid=37300 revision=HEAD version=development version
Starting multi-runner from /etc/gitlab-runner/config.toml...  builds=0
WARNING: Running in user-mode.
WARNING: Use sudo for system-mode:
WARNING: $ sudo gitlab-runner...

Configuration loaded                                builds=0
listen_address not defined, metrics & debug endpoints disabled  builds=0
[session_server].listen_address not defined, session endpoints disabled  builds=0
```

#### `text` {#text}

```shell
INFO[0000] Runtime platform                              arch=amd64 os=darwin pid=37773 revision=HEAD version="development version"
INFO[0000] Starting multi-runner from /etc/gitlab-runner/config.toml...  builds=0
WARN[0000] Running in user-mode.
WARN[0000] Use sudo for system-mode:
WARN[0000] $ sudo gitlab-runner...
INFO[0000]
INFO[0000] Configuration loaded                          builds=0
INFO[0000] listen_address not defined, metrics & debug endpoints disabled  builds=0
INFO[0000] [session_server].listen_address not defined, session endpoints disabled  builds=0
```

#### `json` {#json}

```shell
{"arch":"amd64","level":"info","msg":"Runtime platform","os":"darwin","pid":38229,"revision":"HEAD","time":"2025-06-05T15:57:35+02:00","version":"development version"}
{"builds":0,"level":"info","msg":"Starting multi-runner from /etc/gitlab-runner/config.toml...","time":"2025-06-05T15:57:35+02:00"}
{"level":"warning","msg":"Running in user-mode.","time":"2025-06-05T15:57:35+02:00"}
{"level":"warning","msg":"Use sudo for system-mode:","time":"2025-06-05T15:57:35+02:00"}
{"level":"warning","msg":"$ sudo gitlab-runner...","time":"2025-06-05T15:57:35+02:00"}
{"level":"info","msg":"","time":"2025-06-05T15:57:35+02:00"}
{"builds":0,"level":"info","msg":"Configuration loaded","time":"2025-06-05T15:57:35+02:00"}
{"builds":0,"level":"info","msg":"listen_address not defined, metrics \u0026 debug endpoints disabled","time":"2025-06-05T15:57:35+02:00"}
{"builds":0,"level":"info","msg":"[session_server].listen_address not defined, session endpoints disabled","time":"2025-06-05T15:57:35+02:00"}
```

### `check_interval`の仕組み {#how-check_interval-works}

`config.toml`に複数の`[[runners]]`セクションが含まれている場合、GitLab Runnerは設定されているGitlabインスタンスに対して、ジョブリクエストを継続的にスケジュールするループ処理を行います。

次の例では、`check_interval`が10秒で、2つの`[[runners]]`セクション（`runner-1`と`runner-2`）があります。GitLab Runnerは10秒ごとにリクエストを送信し、5秒間スリープします。

1. `check_interval`の値（`10s`）を取得します。
1. Runnerのリスト（`runner-1`、`runner-2`）を取得します。
1. スリープ間隔（`10s / 2 = 5s`）を計算します。
1. 無限ループを開始します。
   1. `runner-1`のジョブをリクエストします。
   1. `5s`（5秒間）スリープします。
   1. `runner-2`のジョブをリクエストします。
   1. `5s`（5秒間）スリープします。
   1. 繰り返します。

`check_interval`設定例

```toml
# Example `config.toml` file

concurrent = 100 # A global setting for job concurrency that applies to all runner sections defined in this `config.toml` file.
log_level = "warning"
log_format = "json"
check_interval = 10 # Value in seconds

[[runners]]
  name = "runner-1"
  url = "Your Gitlab instance URL (for example, `https://gitlab.com`)"
  executor = "shell"
  (...)

[[runners]]
  name = "runner-2"
  url = "Your Gitlab instance URL (for example, `https://gitlab.com`)"
  executor = "docker"
  (...)
```

この例では、Runnerのプロセスからのジョブリクエストが5秒ごとに行われます。`runner-1`と`runner-2`が同じGitlabインスタンスに接続されている場合、このGitlabインスタンスも5秒ごとにこのRunnerから新しいリクエストを受信します。

`runner-1`の最初のリクエストから次のリクエストまでの間に、合計で2回のスリープ期間が発生します。各期間の長さは5秒であるため、`runner-1`のリクエストの間隔は約10秒です。`runner-2`にも同じことが当てはまります。

定義するRunnerが多いと、スリープ間隔は短くなります。ただし、Runnerに対するリクエストが繰り返されるのは、他のすべてのRunnerに対するリクエストとそれぞれのスリープ期間が実行された後になります。

## `[session_server]`セクション {#the-session_server-section}

ジョブを操作するには、`[[runners]]`セクションの外側のルートレベルで`[session_server]`セクションを指定します。このセクションは、個々のRunnerごとではなく、すべてのRunnerに対して1回だけ設定を行います。

```toml
# Example `config.toml` file with session server configured

concurrent = 100 # A global setting for job concurrency that applies to all runner sections defined in this `config.toml` file
log_level = "warning"
log_format = "runner"
check_interval = 3 # Value in seconds

[session_server]
  listen_address = "[::]:8093" # Listen on all available interfaces on port `8093`
  advertise_address = "runner-host-name.tld:8093"
  session_timeout = 1800
```

`[session_server]`セクションを設定する場合

- `listen_address`と`advertise_address`には、`host:port`という形式を使用します。ここで、`host`はIPアドレス（`127.0.0.1:8093`）またはドメイン（`my-runner.example.com:8093`）です。Runnerはこの情報を使用して、セキュアな接続のためのTLS証明書を作成します。
- `listen_address`または`advertise_address`で定義されているIPアドレスとポートにGitLabが接続できることを確認します。
- アプリケーション設定[`allow_local_requests_from_web_hooks_and_services`](https://docs.gitlab.com/api/settings/#available-settings)を有効にしていない場合は、`advertise_address`がパブリックIPアドレスであることを確認してください。

| 設定             | 説明 |
|---------------------|-------------|
| `listen_address`    | セッションサーバーの内部URL。 |
| `advertise_address` | セッションサーバーにアクセスするためのURL。GitLab RunnerはこのURLをGitlabに公開します。定義されていない場合は、`listen_address`が使用されます。 |
| `session_timeout`   | ジョブの完了後、セッションがアクティブな状態を維持できる秒数。タイムアウトによってジョブの終了がブロックされます。デフォルトは`1800`（30分）です。 |

セッションサーバーとターミナルサポートを無効にするには、`[session_server]`セクションを削除します。

{{< alert type="note" >}}

Runnerインスタンスがすでに実行中の場合は、`[session_server]`セクションの変更を有効にするために`gitlab-runner restart`を実行する必要があることがあります。

{{< /alert >}}

GitLab Runner Dockerイメージを使用している場合は、[`docker run`コマンド](../install/docker.md)に`-p 8093:8093`を追加して、ポート`8093`を公開する必要があります。

## `[[runners]]`セクション {#the-runners-section}

各`[[runners]]`セクションは1つのRunnerを定義します。

| 設定                               | 説明                                                                                                                                                                                                                                                                                                                                                                                                 |
|---------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `name`                                | Runnerの説明（情報提供のみを目的としています）                                                                                                                                                                                                                                                                                                                                                               |
| `url`                                 | GitLabインスタンスのURL。                                                                                                                                                                                                                                                                                                                                                                                        |
| `token`                               | Runner認証トークン。Runnerの登録中に取得されます。[登録トークンとは異なります](https://docs.gitlab.com/api/runners/#registration-and-authentication-tokens)。                                                                                                                                                                                                     |
| `tls-ca-file`                         | HTTPSを使用する場合に、ピアを検証するための証明書を含むファイル。[自己署名証明書またはカスタム認証局のドキュメント](tls-self-signed.md)を参照してください。                                                                                                                                                                                                                             |
| `tls-cert-file`                       | HTTPSを使用する場合に、ピアとの認証に使用する証明書を含むファイル。                                                                                                                                                                                                                                                                                                                         |
| `tls-key-file`                        | HTTPSを使用する場合に、ピアとの認証に使用する秘密キーを含むファイル。                                                                                                                                                                                                                                                                                                                         |
| `limit`                               | この登録済みRunnerが同時に処理できるジョブ数の制限を設定します。`0`（デフォルト）は、制限なしを意味します。この設定が[Docker Machine](autoscale.md#limit-the-number-of-vms-created-by-the-docker-machine-executor)、[Instance](../executors/instance.md)、[Docker Autoscaler](../executors/docker_autoscaler.md#example-aws-autoscaling-for-1-job-per-instance)の各executorでどのように機能するかについては、関連ドキュメントを参照してください。 |
| `executor`                            | RunnerがCI/CDジョブを実行するために使用するホストのオペレーティングシステムの環境またはコマンドプロセッサ。詳細については、[executor](../executors/_index.md)を参照してください。                                                                                                                                                                                                                                   |
| `shell`                               | スクリプトを生成するShellの名前。デフォルト値は[プラットフォームに応じて異なります](../shells/_index.md)。                                                                                                                                                                                                                                                                                                           |
| `builds_dir`                          | 選択したexecutorのコンテキストでビルドが保存されるディレクトリの絶対パス。たとえば、ローカル、Docker、またはSSH環境で使用します。                                                                                                                                                                                                                                                                         |
| `cache_dir`                           | 選択したexecutorのコンテキストでビルドキャッシュが保存されるディレクトリの絶対パス。たとえば、ローカル、Docker、またはSSH環境で使用します。`docker` executorが使用されている場合、このディレクトリを`volumes`パラメータに含める必要があります。                                                                                                                                                                         |
| `environment`                         | 環境変数を追加または上書きします。                                                                                                                                                                                                                                                                                                                                                                  |
| `request_concurrency`                 | GitLabからの新しいジョブに対する同時リクエスト数を制限します。デフォルトは`1`です。ジョブフローを制御するために`concurrency`、`limit`、および`request_concurrency`がどのように相互作用するかについて詳しくは、[GitLab Runnerの並行処理チューニングに関するKB記事](https://support.gitlab.com/hc/en-us/articles/21324350882076-GitLab-Runner-Concurrency-Tuning-Understanding-request-concurrency)をご覧ください。                      |
| `output_limit`                        | ビルドログの最大サイズ（KB単位）。デフォルトは`4096`（4 MB）です。                                                                                                                                                                                                                                                                                                                                              |
| `pre_get_sources_script`              | Gitリポジトリの更新とサブモジュールの更新の前にRunnerで実行されるコマンド。たとえば、最初にGitクライアントの設定を調整するために使用します。複数のコマンドを挿入するには、（三重引用符で囲まれた）複数行の文字列または`\n`文字を使用します。                                                                                                                                                 |
| `post_get_sources_script`             | Gitリポジトリの更新とサブモジュールの更新の後にRunnerで実行されるコマンド。複数のコマンドを挿入するには、（三重引用符で囲まれた）複数行の文字列または`\n`文字を使用します。                                                                                                                                                                                                                    |
| `pre_build_script`                    | ジョブの実行前にRunnerで実行されるコマンド。複数のコマンドを挿入するには、（三重引用符で囲まれた）複数行の文字列または`\n`文字を使用します。                                                                                                                                                                                                                                                     |
| `post_build_script`                   | ジョブの実行直後、`after_script`の実行前にRunnerで実行されるコマンド。複数のコマンドを挿入するには、（三重引用符で囲まれた）複数行の文字列または`\n`文字を使用します。                                                                                                                                                                                                            |
| `clone_url`                           | GitLabインスタンスのURLを上書きします。RunnerがGitlab URLに接続できない場合にのみ使用されます。                                                                                                                                                                                                                                                                                                         |
| `debug_trace_disabled`                | [デバッグトレーシング](https://docs.gitlab.com/ci/variables/#enable-debug-logging)を無効にします。`true`に設定すると、`CI_DEBUG_TRACE`が`true`に設定されていても、デバッグログ（トレース）は無効のままになります。                                                                                                                                                                                                                 |
| `clean_git_config`                    | Git設定をクリーンアップします。詳しくは、[Git設定をクリーンアップする](#cleaning-git-configuration)を参照してください。                                                                                                                                                                                                                                                                                          |
| `referees`                            | 結果をジョブアーティファクトとしてGitLabに渡す追加のジョブモニタリングワーカー。                                                                                                                                                                                                                                                                                                                            |
| `unhealthy_requests_limit`            | 新規ジョブリクエストの`unhealthy`応答の数。この数を超えると、Runnerワーカーは無効になります。                                                                                                                                                                                                                                                                                                            |
| `unhealthy_interval`                  | 異常なリクエストの制限を超えた後に、Runnerワーカーが無効になる期間。`3600 s`、`1 h 30 min`などの構文をサポートしています。                                                                                                                                                                                                                                                      |
| `job_status_final_update_retry_limit` | GitLab Runnerが最終ジョブ状態をGitLabインスタンスにプッシュする操作を再試行できる最大回数。                                                                                                                                                                                                                                                                                                    |

例: 

```toml
[[runners]]
  name = "example-runner"
  url = "http://gitlab.example.com/"
  token = "TOKEN"
  limit = 0
  executor = "docker"
  builds_dir = ""
  shell = ""
  environment = ["ENV=value", "LC_ALL=en_US.UTF-8"]
  clone_url = "http://gitlab.example.local"
```

### 従来の`/ci` URLサフィックス {#legacy-ci-url-suffix}

{{< history >}}

- [GitLab Runner 1.0.0](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/289)で非推奨になりました。
- 警告がGitLab Runner 18.7.0で追加されました。

{{< /history >}}

1.0.0より前のバージョンのGitLab Runnerでは、RunnerのURLは`/ci`サフィックスで設定されていました（例：`url = "https://gitlab.example.com/ci"`）。このサフィックスは不要になったため、設定から削除する必要があります。

`config.toml`に`/ci`サフィックスを含むURLが含まれている場合、GitLab Runnerは設定を処理するときに自動的にそれを削除します。ただし、イシューの可能性を回避するために、設定ファイルを更新してサフィックスを削除する必要があります。

#### 既知の問題 {#known-issues}

- Gitサブモジュールの認証の失敗: `GIT_SUBMODULE_FORCE_HTTPS=true`が設定されている場合、サブモジュールは`fatal: could not read Username for 'https://gitlab.example.com': terminal prompts disabled`のような認証エラーでクローンに失敗する可能性があります。このイシューは、`/ci`サフィックスがGit URLの書き換えルールを妨げるために発生します。詳しくは、[issue 581678](https://gitlab.com/gitlab-org/gitlab/-/work_items/581678#note_2934077238)をご覧ください。

**Problematic configuration**:

```toml
[[runners]]
  name = "legacy-runner"
  url = "https://gitlab.example.com/ci"  # Remove the /ci suffix
  token = "TOKEN"
  executor = "docker"
```

**Corrected configuration**:

```toml
[[runners]]
  name = "legacy-runner"
  url = "https://gitlab.example.com"  # /ci suffix removed
  token = "TOKEN"
  executor = "docker"
```

GitLab Runnerが`/ci`サフィックスを含むURLで起動すると、警告メッセージをログに記録します:

```plaintext
WARNING: The runner URL contains a legacy '/ci' suffix. This suffix is deprecated and should be
removed from the configuration. Git submodules may fail to clone with authentication errors if this
suffix is present. Please update the 'url' field in your config.toml to remove the '/ci' suffix.
See https://docs.gitlab.com/runner/configuration/advanced-configuration.html#legacy-ci-url-suffix for more information.
```

この警告を解決するには、`config.toml`ファイルを編集し、`url`フィールドから`/ci`サフィックスを削除します。

### `clone_url`の仕組み {#how-clone_url-works}

Runnerが使用できないURLでGitLabインスタンスが利用可能な場合は、`clone_url`を設定できます。

たとえば、ファイアウォールが原因でRunnerがURLにアクセスできない場合があります。Runnerが`192.168.1.23`上のノードに接続できる場合は、`clone_url`を`http://192.168.1.23`に設定します。

`clone_url`が設定されると、Runnerは`http://gitlab-ci-token:s3cr3tt0k3n@192.168.1.23/namespace/project.git`の形式でクローンURLを作成します。

{{< alert type="note" >}}

`clone_url`は、Git LFSエンドポイントまたはアーティファクトのアップロードとダウンロードには影響しません。

{{< /alert >}}

#### Git LFSエンドポイントを変更する {#modify-git-lfs-endpoints}

[Git LFS](https://docs.gitlab.com/topics/git/lfs/)エンドポイントを変更するには、次のいずれかのファイルで`pre_get_sources_script`を設定します。

- `config.toml`: 

  ```toml
  pre_get_sources_script = "mkdir -p $RUNNER_TEMP_PROJECT_DIR/git-template; git config -f $RUNNER_TEMP_PROJECT_DIR/git-template/config lfs.url https://<alternative-endpoint>"
  ```

- `.gitlab-ci.yml`: 

  ```yaml
  default:
    hooks:
      pre_get_sources_script:
        - mkdir -p $RUNNER_TEMP_PROJECT_DIR/git-template
        - git config -f $RUNNER_TEMP_PROJECT_DIR/git-template/config lfs.url https://localhost
  ```

### `unhealthy_requests_limit`と`unhealthy_interval`の仕組み {#how-unhealthy_requests_limit-and-unhealthy_interval-works}

GitLabインスタンスが長期間使用できない場合（バージョンのアップグレード中など）、そのRunnerはアイドル状態になります。GitLabインスタンスが再び使用可能になっても、Runnerは後の30～60分間は、ジョブ処理を再開しません。

Runnerがアイドル状態になる期間を増減するには、`unhealthy_interval`設定を変更します。

RunnerのGitLabサーバーへの接続試行回数を変更し、アイドル状態になる前に異常なスリープを受信するには、`unhealthy_requests_limit`設定を変更します。詳細については、[`check_interval`の仕組み](advanced-configuration.md#how-check_interval-works)を参照してください。

## executor {#the-executors}

次のexecutorを使用できます。

| executor            | 必要な設定                                                  | ジョブの実行場所 |
|---------------------|-------------------------------------------------------------------------|----------------|
| `shell`             |                                                                         | ローカルShell。デフォルトのexecutor。 |
| `docker`            | `[runners.docker]`と[Docker Engine](https://docs.docker.com/engine/) | Dockerコンテナ。 |
| `docker-windows`    | `[runners.docker]`と[Docker Engine](https://docs.docker.com/engine/) | Windows Dockerコンテナ。 |
| `ssh`               | `[runners.ssh]`                                                         | SSH、リモート。 |
| `parallels`         | `[runners.parallels]`と`[runners.ssh]`                               | Parallels VM、SSHで接続。 |
| `virtualbox`        | `[runners.virtualbox]`と`[runners.ssh]`                              | VirtualBox VM、SSHで接続。 |
| `docker+machine`    | `[runners.docker]`と`[runners.machine]`                              | `docker`と同じ。ただし、[オートスケールDocker Machine](autoscale.md)を使用。 |
| `kubernetes`        | `[runners.kubernetes]`                                                  | Kubernetesポッド。 |
| `docker-autoscaler` | `[docker-autoscaler]`と`[runners.autoscaler]`                        | `docker`と同じ。ただし、オートスケールインスタンスを使用してCI/CDジョブをコンテナ内で実行。 |
| `instance`          | `[docker-autoscaler]`と`[runners.autoscaler]`                        | `shell`と同じ。ただし、オートスケールインスタンスを使用してCI/CDジョブをホストインスタンス上で直接実行。 |

## Shell {#the-shells}

Shell executorを使用するように設定されている場合、CI/CDジョブはホストマシンでローカルに実行されます。サポートされているオペレーティングシステムのShellは次のとおりです。

| Shell        | 説明 |
|--------------|-------------|
| `bash`       | Bash（Bourne-shell）スクリプトを生成します。すべてのコマンドはBashコンテキストで実行されます。すべてのUnixシステムのデフォルトです。 |
| `sh`         | Sh（Bourne-shell）スクリプトを生成します。すべてのコマンドはShコンテキストで実行されます。すべてのUnixシステムで`bash`のフォールバックとして使用されます。 |
| `powershell` | PowerShellスクリプトを生成します。すべてのコマンドはPowerShell Desktopのコンテキストで実行されます。 |
| `pwsh`       | PowerShellスクリプトを生成します。すべてのコマンドはPowerShell Coreのコンテキストで実行されます。これは、WindowsのデフォルトShellです。 |

`shell`オプションが`bash`または`sh`に設定されている場合、Bashの[ANSI-C引用符の処理方法](https://www.gnu.org/software/bash/manual/html_node/ANSI_002dC-Quoting.html)を使用して、ジョブスクリプトがShellエスケープされます。

### POSIX準拠のShellを使用する {#use-a-posix-compliant-shell}

GitLab Runner 14.9以降では、`dash`などのPOSIX準拠のShellを使用するには、`FF_POSIXLY_CORRECT_ESCAPES`[機能フラグを有効にします](feature-flags.md)。有効にすると、POSIX準拠のShellエスケープメカニズムである[二重引用符](https://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html#tag_18_02)が使用されます。

## `[runners.docker]`セクション {#the-runnersdocker-section}

次の設定は、Dockerコンテナのパラメータを定義します。これらの設定は、Docker executorを使用するようにRunnerが設定されている場合に適用されます。

サービスとしての[Docker-in-Docker](https://docs.gitlab.com/ci/docker/using_docker_build/#use-docker-in-docker)、またはジョブ内で設定されているコンテナランタイムは、これらのパラメータを継承しません。

| パラメータ                          | 例                                          | 説明 |
|------------------------------------|--------------------------------------------------|-------------|
| `allowed_images`                   | `["ruby:*", "python:*", "php:*"]`                | `.gitlab-ci.yml`ファイルで指定できるイメージのワイルドカードリスト。この設定がない場合は、すべてのイメージが許可されます（`["*/*:*"]`と同等）。[Docker](../executors/docker.md#restrict-docker-images-and-services) executorまたは[Kubernetes](../executors/kubernetes/_index.md#restrict-docker-images-and-services) executorで使用します。 |
| `allowed_privileged_images`        |                                                  | `privileged`が有効になっている場合に、特権モードで実行される`allowed_images`のワイルドカードサブセット。この設定がない場合は、すべてのイメージが許可されます（`["*/*:*"]`と同等）。[Docker](../executors/docker.md#restrict-docker-images-and-services) executorで使用します。 |
| `allowed_pull_policies`            |                                                  | `.gitlab-ci.yml`ファイルまたは`config.toml`ファイルで指定できるプルポリシーのリスト。指定されていない場合、`pull-policy`で指定されたプルポリシーのみが許可されます。[Docker](../executors/docker.md#allow-docker-pull-policies) executorで使用します。 |
| `allowed_services`                 | `["postgres:9", "redis:*", "mysql:*"]`           | `.gitlab-ci.yml`ファイルで指定できるサービスのワイルドカードリスト。この設定がない場合は、すべてのイメージが許可されます（`["*/*:*"]`と同等）。[Docker](../executors/docker.md#restrict-docker-images-and-services) executorまたは[Kubernetes](../executors/kubernetes/_index.md#restrict-docker-images-and-services) executorで使用します。 |
| `allowed_privileged_services`      |                                                  | `privileged`または`services_privileged`が有効になっている場合に、特権モードで実行できる`allowed_services`のワイルドカードサブセット。この設定がない場合は、すべてのイメージが許可されます（`["*/*:*"]`と同等）。[Docker](../executors/docker.md#restrict-docker-images-and-services) executorで使用します。 |
| `cache_dir`                        |                                                  | Dockerキャッシュを保存するディレクトリ。絶対パス、または現在の作業ディレクトリを基準にした相対パスを指定できます。詳細については、`disable_cache`を参照してください。 |
| `cap_add`                          | `["NET_ADMIN"]`                                  | コンテナにLinux機能を追加します。 |
| `cap_drop`                         | `["DAC_OVERRIDE"]`                               | コンテナから追加のLinux機能を削除します。 |
| `cpuset_cpus`                      | `"0,1"`                                          | コントロールグループの`CpusetCpus`。文字列。 |
| `cpuset_mems`                      | `"0,1"`                                          | コントロールグループの`CpusetMems`。文字列。 |
| `cpu_shares`                       |                                                  | 相対CPU使用率を設定するために使用されるCPU共有の数。デフォルトは`1024`です。 |
| `cpus`                             | `"2"`                                            | CPUの数（Docker 1.13以降で利用可能）。文字列。 |
| `devices`                          | `["/dev/net/tun"]`                               | 追加のホストデバイスをコンテナと共有します。 |
| `device_cgroup_rules`              |                                                  | カスタムデバイスの`cgroup`ルール（Docker 1.28以降で利用可能）。 |
| `disable_cache`                    |                                                  | Docker executorには、グローバルキャッシュ（他のexecutorと同様）とDockerボリュームに基づくローカルキャッシュという2つのレベルのキャッシュがあります。この設定フラグは、自動的に作成された（ホストディレクトリにマップされていない）キャッシュボリュームの使用を無効にするローカルキャッシュでのみ機能します。つまり、ビルドの一時ファイルを保持するコンテナの作成を防ぐだけであり、Runnerが[分散キャッシュモード](autoscale.md#distributed-runners-caching)で設定されている場合は、キャッシュを無効にしません。 |
| `disable_entrypoint_overwrite`     |                                                  | イメージエントリポイントの上書きを無効にします。 |
| `dns`                              | `["8.8.8.8"]`                                    | コンテナが使用するDNSサーバーのリスト。 |
| `dns_search`                       |                                                  | DNS検索ドメインのリスト。 |
| `extra_hosts`                      | `["other-host:127.0.0.1"]`                       | コンテナ環境で定義する必要があるホスト。 |
| `gpus`                             |                                                  | Dockerコンテナ用のGPUデバイス。`docker` CLIと同じ形式を使用します。詳細については、[Dockerのドキュメント](https://docs.docker.com/engine/containers/resource_constraints/#gpu)を参照してください。[GPUを有効にするための設定](gpus.md#docker-executor)が必要です。 |
| `group_add`                        | `["docker"]`                                     | コンテナプロセスを実行するためのグループをさらに追加します。 |
| `helper_image`                     |                                                  | （高度）リポジトリのクローンやアーティファクトのアップロードに使用される[デフォルトのヘルパーイメージ](#helper-image)。 |
| `helper_image_flavor`              |                                                  | ヘルパーイメージフレーバー（`alpine`、`alpine3.21`、`alpine-latest`、`ubi-fips`、または`ubuntu`）を設定します。`alpine`がデフォルトです。`alpine`フレーバーは`alpine-latest`と同じバージョンを使用します。 |
| `helper_image_autoset_arch_and_os` |                                                  | 基盤となるOSを使用して、ヘルパーイメージのアーキテクチャとOSを設定します。 |
| `host`                             |                                                  | カスタムDockerエンドポイント。デフォルトは`DOCKER_HOST`環境変数または`unix:///var/run/docker.sock`です。 |
| `hostname`                         |                                                  | Dockerコンテナのカスタムホスト名。 |
| `image`                            | `"ruby:3.3"`                                     | ジョブを実行するイメージ。 |
| `links`                            | `["mysql_container:mysql"]`                      | ジョブを実行するコンテナにリンクする必要があるコンテナ。 |
| `memory`                           | `"128m"`                                         | メモリ制限。文字列。 |
| `memory_swap`                      | `"256m"`                                         | 合計メモリ制限。文字列。 |
| `memory_reservation`               | `"64m"`                                          | メモリのソフト制限。文字列。 |
| `network_mode`                     |                                                  | コンテナをカスタムネットワークに追加します。 |
| `mac_address`                      | `92:d0:c6:0a:29:33`                              | コンテナのMACアドレス。 |
| `oom_kill_disable`                 |                                                  | メモリ不足（`OOM`）エラーが発生した場合に、コンテナ内のプロセスを終了しません。 |
| `oom_score_adjust`                 |                                                  | `OOM`スコアの調整。正の値は、プロセスを早期に終了することを意味します。 |
| `privileged`                       | `false`                                          | コンテナを特権モードで実行します。安全ではありません。 |
| `services_privileged`              |                                                  | サービスを特権モードで実行できるようにします。設定されていない場合（デフォルト）、代わりに`privileged`の値が使用されます。[Docker](../executors/docker.md#allow-docker-pull-policies) executorで使用します。安全ではありません。 |
| `pull_policy`                      |                                                  | イメージプルポリシー（`never`、`if-not-present`、または`always`（デフォルト））。詳細については、[プルポリシーのドキュメント](../executors/docker.md#configure-how-runners-pull-images)を参照してください。[複数のプルポリシー](../executors/docker.md#set-multiple-pull-policies)の追加、[失敗したプルの再試行](../executors/docker.md#retry-a-failed-pull)、[プルポリシーの制限](../executors/docker.md#allow-docker-pull-policies)も可能です。 |
| `runtime`                          |                                                  | Dockerコンテナのランタイム。 |
| `isolation`                        |                                                  | コンテナ分離テクノロジー（`default`、`hyperv`、および`process`）。Windowsのみ。 |
| `security_opt`                     |                                                  | セキュリティオプション（`docker run`の--security-opt）。`:`で区切られたキー/値のリストを取得します。`systempaths`仕様はサポートされていません。詳細については、[issue 36810](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/36810)をご覧ください。 |
| `shm_size`                         | `300000`                                         | イメージの共有メモリサイズ（バイト単位）。 |
| `sysctls`                          |                                                  | `sysctl`のオプション。 |
| `tls_cert_path`                    | macOSの場合: `/Users/<username>/.boot2docker/certs` | `ca.pem`、`cert.pem`、または`key.pem`が保存され、Dockerへの安全なTLS接続を確立するために使用されるディレクトリ。この設定は`boot2docker`で使用します。 |
| `tls_verify`                       |                                                  | Dockerデーモンへの接続のTLS検証を有効または無効にします。デフォルトでは無効になっています。デフォルトでは、GitLab RunnerはSSH経由でDocker Unixソケットに接続します。UnixソケットはRTLSをサポートしておらず、暗号化と認証を提供するためにSSHを使用してHTTP経由で通信します。通常、`tls_verify`を有効にする必要はありません。有効にする場合には、追加の設定が必要です。`tls_verify`を有効にするには、デーモンが（デフォルトのUnixソケットではなく）ポートでリッスンする必要があり、GitLab Runner Dockerホストはデーモンがリッスンしているアドレスを使用する必要があります。 |
| `user`                             |                                                  | コンテナ内のすべてのコマンドを、指定されたユーザーとして実行します。 |
| `userns_mode`                      |                                                  | ユーザーネームスペースの再マッピングオプションが有効になっている場合の、コンテナおよびDockerサービス用のユーザーネームスペースモード。Docker 1.10以降で利用可能です。詳細については、[Dockerのドキュメント](https://docs.docker.com/engine/security/userns-remap/#disable-namespace-remapping-for-a-container)を参照してください。 |
| `ulimit`                           |                                                  | コンテナに渡されるUlimit値。Docker `--ulimit`フラグと同じ構文を使用します。 |
| `volumes`                          | `["/data", "/home/project/cache"]`               | マウントする必要がある追加ボリューム。Docker `-v`フラグと同じ構文。 |
| `volumes_from`                     | `["storage_container:ro"]`                       | 別のコンテナから継承するボリュームのリスト。形式は`<container name>[:<access_level>]`です。アクセスレベルはデフォルトで読み取り/書き込みですが、手動で`ro`（読み取り専用）または`rw`（読み取り/書き込み）に設定できます。 |
| `volume_driver`                    |                                                  | コンテナに使用するボリュームドライバー。 |
| `wait_for_services_timeout`        | `30`                                             | Dockerサービスを待機する時間。無効にするには`-1`に設定します。デフォルトは`30`です。 |
| `container_labels`                 |                                                  | Runnerによって作成された各コンテナに追加するラベルのセット。ラベルの値には、展開用の環境変数を含めることができます。 |
| `services_limit`                   |                                                  | ジョブごとに許可されるサービスの最大数を設定します。`-1`（デフォルト）は、制限がないことを意味します。 |
| `service_cpuset_cpus`              |                                                  | サービスに使用する`cgroups CpusetCpus`を含む文字列値。 |
| `service_cpu_shares`               |                                                  | サービスの相対CPU使用率を設定するために使用されるCPUシェア数（デフォルトは[`1024`](https://docs.docker.com/engine/containers/resource_constraints/#cpu)）。 |
| `service_cpus`                     |                                                  | サービスのCPU数を表す文字列値。Docker 1.13以降で利用可能です。 |
| `service_gpus`                     |                                                  | Dockerコンテナ用のGPUデバイス。`docker` CLIと同じ形式を使用します。詳細については、[Dockerのドキュメント](https://docs.docker.com/engine/containers/resource_constraints/#gpu)を参照してください。[GPUを有効にするための設定](gpus.md#docker-executor)が必要です。 |
| `service_memory`                   |                                                  | サービスのメモリ制限を表す文字列値。 |
| `service_memory_swap`              |                                                  | サービスの合計メモリ制限を表す文字列値。 |
| `service_memory_reservation`       |                                                  | サービスのメモリのソフト制限を表す文字列値。 |

### `[[runners.docker.services]]`セクション {#the-runnersdockerservices-section}

ジョブと実行する追加の[サービス](https://docs.gitlab.com/ci/services/)を指定します。利用可能なイメージのリストについては、[Docker Registry](https://hub.docker.com)を参照してください。各サービスは個別のコンテナで実行され、ジョブにリンクされます。

| パラメータ     | 例                            | 説明 |
|---------------|------------------------------------|-------------|
| `name`        | `"registry.example.com/svc1"`      | サービスとして実行されるイメージの名前。 |
| `alias`       | `"svc1"`                           | サービスへのアクセスに使用できる追加の[エイリアス名](https://docs.gitlab.com/ci/services/#available-settings-for-services)。 |
| `entrypoint`  | `["entrypoint.sh"]`                | コンテナのエントリポイントとして実行されるコマンドまたはスクリプト。構文は[Dockerfile ENTRYPOINT](https://docs.docker.com/reference/dockerfile/#entrypoint)ディレクティブに似ており、各Shellトークンは配列内の個別の文字列です。[GitLab Runner 13.6](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27173)で導入されました。 |
| `command`     | `["executable","param1","param2"]` | コンテナのコマンドとして使用されるコマンドまたはスクリプト。構文は[Dockerfile CMD](https://docs.docker.com/reference/dockerfile/#cmd)ディレクティブに似ており、各Shellトークンは配列内の個別の文字列です。[GitLab Runner 13.6](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27173)で導入されました。 |
| `environment` | `["ENV1=value1", "ENV2=value2"]`   | サービスコンテナの環境変数を付加または上書きします。 |

例: 

```toml
[runners.docker]
  host = ""
  hostname = ""
  tls_cert_path = "/Users/ayufan/.boot2docker/certs"
  image = "ruby:3.3"
  memory = "128m"
  memory_swap = "256m"
  memory_reservation = "64m"
  oom_kill_disable = false
  cpuset_cpus = "0,1"
  cpuset_mems = "0,1"
  cpus = "2"
  dns = ["8.8.8.8"]
  dns_search = [""]
  service_memory = "128m"
  service_memory_swap = "256m"
  service_memory_reservation = "64m"
  service_cpuset_cpus = "0,1"
  service_cpus = "2"
  services_limit = 5
  privileged = false
  group_add = ["docker"]
  cap_add = ["NET_ADMIN"]
  cap_drop = ["DAC_OVERRIDE"]
  devices = ["/dev/net/tun"]
  disable_cache = false
  wait_for_services_timeout = 30
  cache_dir = ""
  volumes = ["/data", "/home/project/cache"]
  extra_hosts = ["other-host:127.0.0.1"]
  shm_size = 300000
  volumes_from = ["storage_container:ro"]
  links = ["mysql_container:mysql"]
  allowed_images = ["ruby:*", "python:*", "php:*"]
  allowed_services = ["postgres:9", "redis:*", "mysql:*"]
  [runners.docker.ulimit]
    "rtprio" = "99"
  [[runners.docker.services]]
    name = "registry.example.com/svc1"
    alias = "svc1"
    entrypoint = ["entrypoint.sh"]
    command = ["executable","param1","param2"]
    environment = ["ENV1=value1", "ENV2=value2"]
  [[runners.docker.services]]
    name = "redis:2.8"
    alias = "cache"
  [[runners.docker.services]]
    name = "postgres:9"
    alias = "postgres-db"
  [runners.docker.sysctls]
    "net.ipv4.ip_forward" = "1"
```

### `[runners.docker]`セクションのボリューム {#volumes-in-the-runnersdocker-section}

ボリュームの詳細については、[Dockerのドキュメント](https://docs.docker.com/engine/storage/volumes/)を参照してください。

次の例は、`[runners.docker]`セクションでボリュームを指定する方法を示しています。

#### 例1: データボリュームを追加する {#example-1-add-a-data-volume}

データボリュームは、1つ以上のコンテナ内で特別に指定されたディレクトリで、Union File Systemをバイパスします。データボリュームは、コンテナのライフサイクルに依存せず、データを永続化するように設計されています。

```toml
[runners.docker]
  host = ""
  hostname = ""
  tls_cert_path = "/Users/ayufan/.boot2docker/certs"
  image = "ruby:3.3"
  privileged = false
  disable_cache = true
  volumes = ["/path/to/volume/in/container"]
```

この例では、コンテナ内の`/path/to/volume/in/container`という場所に新しいボリュームが作成されます。

#### 例2: ホストディレクトリをデータボリュームとしてマウントする {#example-2-mount-a-host-directory-as-a-data-volume}

コンテナの外部にディレクトリを保存する場合は、Dockerデーモンのホストからコンテナにディレクトリをマウントできます。

```toml
[runners.docker]
  host = ""
  hostname = ""
  tls_cert_path = "/Users/ayufan/.boot2docker/certs"
  image = "ruby:3.3"
  privileged = false
  disable_cache = true
  volumes = ["/path/to/bind/from/host:/path/to/bind/in/container:rw"]
```

この例では、CI/CDホストの`/path/to/bind/from/host`をコンテナ内の`/path/to/bind/in/container`で使用します。

GitLab Runner 11.11以降では、定義された[サービス](https://docs.gitlab.com/ci/services/)についても[同様にホストディレクトリをマウント](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/1261)します。

### プライベートコンテナレジストリを使用する {#use-a-private-container-registry}

ジョブのイメージのソースとしてプライベートレジストリを使用するには、[CI/CD変数](https://docs.gitlab.com/ci/variables/)`DOCKER_AUTH_CONFIG`を使用して認証を設定します。次のいずれかで変数を設定できます。

- プロジェクトのCI/CD設定内で[`file`タイプ](https://docs.gitlab.com/ci/variables/#use-file-type-cicd-variables)として設定
- `config.toml`ファイル内で設定

`if-not-present`プルポリシーでプライベートレジストリを使用すると、[セキュリティ上の影響](../security/_index.md#usage-of-private-docker-images-with-if-not-present-pull-policy)が生じる可能性があります。プルポリシーの仕組みの詳細については、[Runnerがイメージをプルする方法を設定する](../executors/docker.md#configure-how-runners-pull-images)を参照してください。

プライベートコンテナレジストリの使用に関する詳細については、以下を参照してください。

- [プライベートコンテナレジストリからのイメージへのアクセス](https://docs.gitlab.com/ci/docker/using_docker_images/#access-an-image-from-a-private-container-registry)
- [`.gitlab-ci.yml`キーワードリファレンス](https://docs.gitlab.com/ci/yaml/#image)

Runnerによって実行されるステップの要約を次に示します。

1. レジストリ名がイメージ名から検出されます。
1. 値が空でない場合、executorはこのレジストリに対する認証設定を検索します。
1. 最後に、指定されたレジストリに対応する認証が見つかった場合、以降のプルではその認証が使用されます。

#### GitLab統合レジストリのサポート {#support-for-gitlab-integrated-registry}

GitLabは、ジョブのデータとともに、統合レジストリの認証情報を送信します。これらの認証情報は、レジストリの認証パラメータリストに自動的に追加されます。

このステップの後、レジストリに対する認証は、`DOCKER_AUTH_CONFIG`変数で追加された設定と同様に進みます。

ジョブでは、GitLab統合レジストリのイメージがプライベートまたは保護されている場合でも、任意のイメージを使用できます。ジョブがアクセスできるイメージの詳細については、[CI/CDジョブトークンのドキュメント](https://docs.gitlab.com/ci/jobs/ci_job_token/)を参照してください。

#### Docker認証解決の優先順位 {#precedence-of-docker-authorization-resolving}

前述のように、GitLab Runnerはさまざまな方法で送信される認証情報を使用して、レジストリに対してDockerを認証できます。適切なレジストリを見つけるために、次の優先順位が考慮されます。

1. `DOCKER_AUTH_CONFIG`で設定された認証情報
1. GitLab Runnerホストでローカルに設定された認証情報（`~/.docker/config.json`または`~/.dockercfg`ファイルに保存）（例: ホストで`docker login`を実行した場合）。
1. ジョブのペイロードとともにデフォルトで送信される認証情報（例: 前述の*統合レジストリ*の認証情報）。

レジストリに対して最初に検出された認証情報が使用されます。たとえば、`DOCKER_AUTH_CONFIG`変数を使用して*統合レジストリ*の認証情報を追加すると、デフォルトの認証情報が上書きされます。

## `[runners.parallels]`セクション {#the-runnersparallels-section}

次にParallelsのパラメータを示します。

| パラメータ           | 説明 |
|---------------------|-------------|
| `base_name`         | クローンされるParallels VMの名前。 |
| `template_name`     | Parallels VMにリンクされたテンプレートのカスタム名。オプション。 |
| `disable_snapshots` | 無効にした場合、ジョブが完了するとVMは破棄されます。 |
| `allowed_images`    | 許可される`image`/`base_name`値のリスト。これらの値は正規表現として表されます。詳細については、[ベースVMイメージを上書きする](#overriding-the-base-vm-image)セクションを参照してください。 |

例: 

```toml
[runners.parallels]
  base_name = "my-parallels-image"
  template_name = ""
  disable_snapshots = false
```

## `[runners.virtualbox]`セクション {#the-runnersvirtualbox-section}

次にVirtualBoxのパラメータを示します。このexecutorは、VirtualBoxマシンを制御するために`vboxmanage`実行可能ファイルに依存しています。そのため、Windowsホストでは`PATH`環境変数を調整する必要があります（`PATH=%PATH%;C:\Program Files\Oracle\VirtualBox`）。

| パラメータ           | 説明 |
|---------------------|-------------|
| `base_name`         | クローンされるVirtualBox VMの名前。 |
| `base_snapshot`     | リンクされたクローンを作成する際の特定のVMスナップショットの名前またはUUID。この値が空であるか省略されている場合は、現在のスナップショットが使用されます。現在のスナップショットが存在しない場合は、スナップショットが作成されます。ただし、`disable_snapshots`がtrueでない場合は、ベースVMの完全なクローンが作成されます。 |
| `base_folder`       | 新しいVMを保存するフォルダー。この値が空であるか省略されている場合は、デフォルトのVMフォルダーが使用されます。 |
| `disable_snapshots` | 無効にした場合、ジョブが完了するとVMは破棄されます。 |
| `allowed_images`    | 許可される`image`/`base_name`値のリスト。これらの値は正規表現として表されます。詳細については、[ベースVMイメージを上書きする](#overriding-the-base-vm-image)セクションを参照してください。 |
| `start_type`        | VMの起動時のグラフィカルフロントエンドタイプ。 |

例: 

```toml
[runners.virtualbox]
  base_name = "my-virtualbox-image"
  base_snapshot = "my-image-snapshot"
  disable_snapshots = false
  start_type = "headless"
```

`start_type`パラメータは、仮想イメージの起動時に使用されるグラフィカルフロントエンドを決定します。有効な値は、ホストとゲストの組み合わせでサポートされている`headless`（デフォルト）、`gui`、または`separate`です。

## ベースVMイメージを上書きする {#overriding-the-base-vm-image}

Parallels executorとVirtualBox executorの両方で、`base_name`で指定されたベースVM名を上書きできます。そのためには、`.gitlab-ci.yml`ファイルの[image](https://docs.gitlab.com/ci/yaml/#image)パラメータを使用します。

下位互換性のため、デフォルトではこの値を上書きできません。`base_name`で指定されたイメージのみが許可されます。

ユーザーが`.gitlab-ci.yml`の[image](https://docs.gitlab.com/ci/yaml/#image)パラメータを使用してVMイメージを選択できるようにするには、次のようにします。

```toml
[runners.virtualbox]
  ...
  allowed_images = [".*"]
```

この例では、既存のVMイメージであればどれでも使用できます。

`allowed_images`パラメータは、正規表現のリストです。必要な精度に応じて設定を細かく指定できます。たとえば、特定のVMイメージのみを許可したい場合は、次のような正規表現を使用できます。

```toml
[runners.virtualbox]
  ...
  allowed_images = ["^allowed_vm[1-2]$"]
```

この例では、`allowed_vm1`と`allowed_vm2`のみが許可されます。その他の試行はすべてエラーになります。

## `[runners.ssh]`セクション {#the-runnersssh-section}

次のパラメータは、SSH接続を定義します。

| パラメータ                          | 説明 |
|------------------------------------|-------------|
| `host`                             | 接続先 |
| `port`                             | ポートデフォルトは`22`です。 |
| `user`                             | ユーザー名。   |
| `password`                         | パスワード。   |
| `identity_file`                    | SSH秘密キーのファイルパス（`id_rsa`、`id_dsa`、または`id_edcsa`）。ファイルは暗号化されていない状態で保存する必要があります。 |
| `disable_strict_host_key_checking` | この値は、Runnerが厳密なホストキーチェックを使用するかどうかを決定します。デフォルトは`true`です。GitLab 15.0では、デフォルト値、または指定されていない場合の値は`false`です。 |

例: 

```toml
[runners.ssh]
  host = "my-production-server"
  port = "22"
  user = "root"
  password = "production-server-password"
  identity_file = ""
```

## `[runners.machine]`セクション {#the-runnersmachine-section}

次のパラメータは、Docker Machineベースのオートスケール機能を定義します。詳細については、[Docker Machine Executorのオートスケール設定](autoscale.md)を参照してください。

| パラメータ                         | 説明 |
|-----------------------------------|-------------|
| `MaxGrowthRate`                   | Runnerに並行して追加できるマシンの最大数。デフォルトは`0`（制限なし）です。 |
| `IdleCount`                       | _アイドル_状態で作成され待機する必要があるマシンの数。 |
| `IdleScaleFactor`                 | 使用中マシンの数の係数として示される_アイドル_マシンの数。浮動小数点数形式である必要があります。詳細については、[オートスケールのドキュメント](autoscale.md#the-idlescalefactor-strategy)を参照してください。`0.0`がデフォルトです。 |
| `IdleCountMin`                    | `IdleScaleFactor`使用時に作成され_アイドル_状態で待機する必要があるマシンの最小数。デフォルトは1です。 |
| `IdleTime`                        | マシンが削除されるまでにそのマシンが_アイドル_状態を維持する時間（秒単位）。 |
| `[[runners.machine.autoscaling]]` | オートスケール設定の上書きが含まれている複数のセクション。現在の時刻に一致する式を含む最後のセクションが選択されます。 |
| `OffPeakPeriods`                  | 非推奨: スケジューラがOffPeakモードになっている時間帯。cron形式のパターンの配列（[下記](#periods-syntax)を参照）。 |
| `OffPeakTimezone`                 | 非推奨: OffPeakPeriodsで指定された時刻のタイムゾーン。`Europe/Berlin`のようなタイムゾーン文字列です。省略または空の場合、デフォルトはホストのロケールシステム設定です。GitLab Runnerは、`ZONEINFO`環境変数で指定されたディレクトリまたは解凍済みzipファイルでタイムゾーンデータベースを検索し、次にUnixシステム上の既知のインストール場所を検索し、最後に`$GOROOT/lib/time/zoneinfo.zip`内を検索します。 |
| `OffPeakIdleCount`                | 非推奨: `IdleCount`と同様ですが、_オフピーク_の時間帯を対象としています。 |
| `OffPeakIdleTime`                 | 非推奨: `IdleTime`と同様ですが、_オフピーク_の時間帯を対象としています。 |
| `MaxBuilds`                       | マシンが削除されるまでの最大ジョブ（ビルド）数。 |
| `MachineName`                     | マシンの名前。`%s`を含める**必要があります**。これは一意のマシン識別子に置き換えられます。 |
| `MachineDriver`                   | Docker Machineの`driver`。詳細については、[Docker Machine設定のクラウドプロバイダーセクション](autoscale.md#supported-cloud-providers)を参照してください。 |
| `MachineOptions`                  | MachineDriverのDocker Machineオプション。詳細については、[サポートされているクラウドプロバイダー](autoscale.md#supported-cloud-providers)を参照してください。AWSのすべてのオプションの詳細については、Docker Machineリポジトリの[AWS](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/aws.md)プロジェクトと[GCP](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/gce.md)プロジェクトを参照してください。 |

### `[[runners.machine.autoscaling]]`セクション {#the-runnersmachineautoscaling-sections}

次のパラメータは、[Instance](../executors/instance.md) executorまたは[Docker Autoscaler](../executors/docker_autoscaler.md#example-aws-autoscaling-for-1-job-per-instance) executorを使用する際に利用可能な設定を定義します。

| パラメータ         | 説明 |
|-------------------|-------------|
| `Periods`         | このスケジュールがアクティブな時間帯。cron形式のパターンの配列（[下記](#periods-syntax)を参照）。 |
| `IdleCount`       | _アイドル_状態で作成され待機する必要があるマシンの数。 |
| `IdleScaleFactor` | （実験的機能）使用中のマシン数の係数として示される_アイドル_マシンの数。浮動小数点数形式である必要があります。詳細については、[オートスケールのドキュメント](autoscale.md#the-idlescalefactor-strategy)を参照してください。`0.0`がデフォルトです。 |
| `IdleCountMin`    | `IdleScaleFactor`使用時に作成され_アイドル_状態で待機する必要があるマシンの最小数。デフォルトは1です。 |
| `IdleTime`        | マシンが削除されるまでにそのマシンが_アイドル_状態である時間（秒単位）。 |
| `Timezone`        | `Periods`で指定された時刻のタイムゾーン。`Europe/Berlin`のようなタイムゾーン文字列です。省略または空の場合、デフォルトはホストのロケールシステム設定です。GitLab Runnerは、`ZONEINFO`環境変数で指定されたディレクトリまたは解凍済みzipファイルでタイムゾーンデータベースを検索し、次にUnixシステム上の既知のインストール場所を検索し、最後に`$GOROOT/lib/time/zoneinfo.zip`内を検索します。 |

例: 

```toml
[runners.machine]
  IdleCount = 5
  IdleTime = 600
  MaxBuilds = 100
  MachineName = "auto-scale-%s"
  MachineDriver = "google" # Refer to Docker Machine docs on how to authenticate: https://docs.docker.com/machine/drivers/gce/#credentials
  MachineOptions = [
      # Additional machine options can be added using the Google Compute Engine driver.
      # If you experience problems with an unreachable host (ex. "Waiting for SSH"),
      # you should remove optional parameters to help with debugging.
      # https://docs.docker.com/machine/drivers/gce/
      "google-project=GOOGLE-PROJECT-ID",
      "google-zone=GOOGLE-ZONE", # e.g. 'us-central1-a', full list in https://cloud.google.com/compute/docs/regions-zones/
  ]
  [[runners.machine.autoscaling]]
    Periods = ["* * 9-17 * * mon-fri *"]
    IdleCount = 50
    IdleCountMin = 5
    IdleScaleFactor = 1.5 # Means that current number of Idle machines will be 1.5*in-use machines,
                          # no more than 50 (the value of IdleCount) and no less than 5 (the value of IdleCountMin)
    IdleTime = 3600
    Timezone = "UTC"
  [[runners.machine.autoscaling]]
    Periods = ["* * * * * sat,sun *"]
    IdleCount = 5
    IdleTime = 60
    Timezone = "UTC"
```

### periods構文 {#periods-syntax}

`Periods`設定は、cron形式で表される時間帯の文字列パターンを集めた配列です。行は次のフィールドで構成されます。

```plaintext
[second] [minute] [hour] [day of month] [month] [day of week] [year]
```

標準のcron設定ファイルと同様に、これらのフィールドには単一値、範囲、リスト、およびアスタリスクを含めることができます。[構文の詳細な説明](https://github.com/gorhill/cronexpr#implementation)を参照してください。

## `[runners.instance]`セクション {#the-runnersinstance-section}

| パラメータ        | 型   | 説明 |
|------------------|--------|-------------|
| `allowed_images` | 文字列 | VM分離が有効になっている場合、`allowed_images`はジョブが指定できるイメージを制御します。 |

## `[runners.autoscaler]`セクション {#the-runnersautoscaler-section}

{{< history >}}

- GitLab Runner v15.10.0で導入されました。

{{< /history >}}

次のパラメータは、オートスケーラー機能を設定します。これらのパラメータは、[インスタンス](../executors/instance.md) executorと[Docker Autoscaler](../executors/docker_autoscaler.md) executorでのみ使用できます。

| パラメータ                        | 説明 |
|----------------------------------|-------------|
| `capacity_per_instance`          | 1つのインスタンスで同時に実行できるジョブの数。 |
| `max_use_count`                  | インスタンスが削除対象としてスケジュールされる前にそのインスタンスを使用できる最大回数。 |
| `max_instances`                  | 許可されるインスタンスの最大数。これは、インスタンスの状態（保留中、実行中、削除中）に関係なく適用されます。デフォルトは`0`（無制限）です。 |
| `plugin`                         | 使用する[フリート](https://gitlab.com/gitlab-org/fleeting/fleeting)プラグイン。プラグインのインストール方法と参照方法について詳しくは、[フリートプラグインをインストールする](../fleet_scaling/fleeting.md#install-a-fleeting-plugin)を参照してください。 |
| `delete_instances_on_shutdown`   | GitLab Runnerのシャットダウン時に、プロビジョニングされたすべてのインスタンスを削除するかどうかを指定します。デフォルト: `false`。[GitLab Runner 15.11](https://gitlab.com/gitlab-org/fleeting/taskscaler/-/merge_requests/24)で導入されました。 |
| `instance_ready_command`         | オートスケーラーによってプロビジョニングされた各インスタンスでこのコマンドを実行して、インスタンスが使用できる状態になっていることを確認します。失敗すると、インスタンスが削除されます。[GitLab Runner 16.11](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/37473)で導入されました。 |
| `instance_acquire_timeout`       | Runnerがインスタンス取得を待機してタイムアウトになるまでの最大時間。デフォルト: `15m`（15分）。この値は、実際の環境に合わせて調整できます。[GitLab Runner 18.1](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5563)で導入されました。 |
| `update_interval`                | フリートプラグインでインスタンスの更新を確認する間隔。デフォルト: `1m`（1分）。[GitLab Runner 16.11](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4722)で導入されました。 |
| `update_interval_when_expecting` | 状態が変化することが予期される場合にフリートプラグインでインスタンスの更新を確認する間隔。たとえば、インスタンスがインスタンスをプロビジョニングし、Runnerが`pending`から`running`への移行を待機している場合などです。デフォルト: `2s`（2秒）。[GitLab Runner 16.11](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4722)で導入されました。 |
| `deletion_retry_interval` | 以前の削除試行が効果がなかった場合に、プラグインが削除を再試行するまで待機する間隔。デフォルト: `1m`（1分）。[GitLab Runner 18.4](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5777)で導入。 |
| `shutdown_deletion_interval`| インスタンスを削除してからシャットダウン中にそれらのステータスをチェックするまでの間で使用される、フリーティングプラグインの間隔。デフォルト: `10s`（10秒）。[GitLab Runner 18.4](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5777)で導入。 |
| `shutdown_deletion_retries` | シャットダウン前にインスタンスが削除を完了したことを確認するために、フリーティングプラグインが行う試行の最大数。デフォルト: `3`。[GitLab Runner 18.4](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5777)で導入。 |
| `failure_threshold` | フリーティングプラグインがインスタンスを置き換えるまでに発生する、連続したヘルスの失敗の最大数。ハートビート機能も参照してください。デフォルト: `3`。[GitLab Runner 18.4](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5777)で導入。 |
| `log_internal_ip`                | VMの内部IPアドレスをCI/CDの出力ログに記録するかどうかを指定します。デフォルト: `false`。[GitLab Runner 18.1](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5519)で導入されました。 |
| `log_external_ip`                | VMの外部IPアドレスをCI/CDの出力ログに記録するかどうかを指定します。デフォルト: `false`。[GitLab Runner 18.1](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5519)で導入されました。 |

{{< alert type="note" >}}

`instance_ready_command`がアイドル状態のスケールルールで頻繁に失敗する場合、Runnerがジョブを受け入れるよりも速くインスタンスが削除および作成される可能性があります。スケールスロットリングをサポートするため、[GitLab 17.0](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/37497)で指数バックオフが追加されました。

{{< /alert >}}

{{< alert type="note" >}}

オートスケーラーの設定オプションは、設定が変更されても再読み込みされません。ただし、GitLab 17.5.0以降では、設定が変更されると、`[[runners.autoscaler.policy]]`エントリが再読み込されます。

{{< /alert >}}

## `[runners.autoscaler.plugin_config]`セクション {#the-runnersautoscalerplugin_config-section}

このハッシュテーブルはJSONに再エンコードされ、設定済みのプラグインに直接渡されます。

[フリート](https://gitlab.com/gitlab-org/fleeting/fleeting)プラグインには通常、サポートされている設定に関するドキュメントが付いています。

## `[runners.autoscaler.scale_throttle]`セクション {#the-runnersautoscalerscale_throttle-section}

{{< history >}}

- GitLab Runner v17.0.0で導入されました。

{{< /history >}}

| パラメータ | 説明 |
|-----------|-------------|
| `limit`   | 1秒あたりにプロビジョニングできる新しいインスタンスのレート制限。`-1`は無制限を意味します。デフォルト（`0`）では、制限が`100`に設定されます。 |
| `burst`   | 新しいインスタンスのバースト制限。デフォルトは`max_instances`に設定されるか、`max_instances`が設定されていない場合は`limit`に設定されます。`limit`が無制限の場合、`burst`は無視されます。 |

### `limit`と`burst`の関係 {#relationship-between-limit-and-burst}

スケールスロットルは、トークンクォータシステムを使用してインスタンスを作成します。このシステムは、次の2つの値で定義されます。

- `burst`: クォータの最大サイズ。
- `limit`: 1秒あたりのクォータ更新レート。

一度に作成できるインスタンスの数は、残りのクォータによって決まります。十分なクォータがある場合は、その量までインスタンスを作成できます。クォータがなくなった場合は、1秒あたり`limit`の数のインスタンスを作成できます。インスタンスの作成が停止すると、クォータは1秒あたり`limit`ずつ、`burst`の値に達するまで増加します。

たとえば、`limit`が`1`で`burst`が`60`の場合は、次のようになります。

- 60個のインスタンスを即時に作成できますが、制限（スロットル）されます。
- 60秒待機すると、さらに60個のインスタンスを即時に作成できます。
- 待機しない場合は、1秒ごとに1つのインスタンスを作成できます。

## `[runners.autoscaler.connector_config]`セクション {#the-runnersautoscalerconnector_config-section}

[フリート](https://gitlab.com/gitlab-org/fleeting/fleeting)プラグインには通常、サポートされている接続オプションに関するドキュメントが付いています。

プラグインはコネクタ設定を自動的に更新します。`[runners.autoscaler.connector_config]`を使用して、コネクタ設定の自動更新を上書きしたり、プラグインが判断できない空の値を入力したりできます。

| パラメータ                | 説明 |
|--------------------------|-------------|
| `os`                     | インスタンスのオペレーティングシステム。 |
| `arch`                   | インスタンスのアーキテクチャ。 |
| `protocol`               | `ssh`、`winrm`、または`winrm+https`。Windowsが検出された場合、デフォルトで`winrm`が使用されます。 |
| `protocol_port`          | 指定されたプロトコルに基づいて接続を確立するために使用されるポート。デフォルトは`ssh:22`、`winrm+http:5985`、`winrm+https:5986`です。 |
| `username`               | 接続に使用するユーザー名。 |
| `password`               | 接続に使用するパスワード。 |
| `key_path`               | 接続に使用するTLSキー、または動的にプロビジョニングされた認証情報に使用するTLSキー。 |
| `use_static_credentials` | 自動認証情報プロビジョニングが無効になっています。デフォルト: `false`。 |
| `keepalive`              | 接続キープアライブ時間。 |
| `timeout`                | 接続タイムアウト時間。 |
| `use_external_addr`      | プラグインが提供する外部アドレスを使用するかどうか。プラグインが内部アドレスのみを返す場合は、この設定に関係なく内部アドレスが使用されます。デフォルト: `false`。 |

## `[runners.autoscaler.state_storage]`セクション {#the-runnersautoscalerstate_storage-section}

{{< details >}}

- ステータス: ベータ版

{{< /details >}}

{{< history >}}

- GitLab Runner 17.5.0で導入されました。

{{< /history >}}

ステートストレージが無効になっている場合（デフォルト）、GitLab Runnerが起動すると、安全上の理由から既存のフリートインスタンスは直ちに削除されます。たとえば、`max_use_count`が`1`に設定されている場合、使用状態がわからないと、すでに使用されているインスタンスに誤ってジョブを割り当ててしまう可能性があります。

ステートストレージ機能を有効にすると、インスタンスの状態をローカルディスクに保持できます。この場合、GitLab Runnerの起動時にインスタンスが存在していても、そのインスタンスは削除されません。キャッシュされた接続の詳細、使用回数、およびその他の設定が復元されます。

ステートストレージ機能を有効にする場合は、次の点を考慮してください。

- インスタンスの認証の詳細（ユーザー名、パスワード、キー）はディスクに残ります。
- インスタンスがジョブをアクティブに実行しているときにそのインスタンスが復元されると、GitLab Runnerはデフォルトでそのインスタンスを削除します。GitLab Runnerがジョブを再開できないため、この動作により安全性が確保されます。インスタンスを維持するには、`keep_instance_with_acquisitions`を`true`に設定します。

  インスタンスで進行中のジョブについて特に懸念していない場合には、`keep_instance_with_acquisitions`を`true`に設定すると役立ちます。また、`instance_ready_command`設定オプションを使用して環境をクリーンアップし、インスタンスを維持することもできます。この場合、実行中のすべてのコマンドを停止したり、Dockerコンテナを強制的に削除したりすることがあります。

| パラメータ                         | 説明 |
|-----------------------------------|-------------|
| `enabled`                         | ステートストレージを有効にするかどうか。デフォルト: `false`。 |
| `dir`                             | ステートストアディレクトリ。このディレクトリの中に、各Runner設定エントリに対応するサブディレクトリがあります。デフォルトは、Gitlab Runner設定ファイルディレクトリ内の`.taskscaler`です。 |
| `keep_instance_with_acquisitions` | アクティブなジョブがあるインスタンスを削除するかどうか。デフォルト: `false`。 |

## `[[runners.autoscaler.policy]]`セクション {#the-runnersautoscalerpolicy-sections}

**注** \- ここでの`idle_count`はジョブの数を示し、従来のオートスケール方式のようにオートスケールされたマシンの数ではありません。

| パラメータ            | 説明 |
|----------------------|-------------|
| `periods`            | このポリシーが有効になっている期間を示すunix-cron形式の文字列の配列。デフォルト: `* * * * *` |
| `timezone`           | unix-cron期間の評価時に使用されるタイムゾーン。デフォルト: システムのローカルタイムゾーン。 |
| `idle_count`         | ジョブで即時利用可能であるべき目標アイドル容量。 |
| `idle_time`          | インスタンスが終了するまでにアイドル状態でいられる時間。 |
| `scale_factor`       | `idle_count`に加えて、ジョブで即時利用可能であるべき目標アイドル容量を、現在の使用中の容量の係数として表したもの。`0.0`がデフォルトです。 |
| `scale_factor_limit` | `scale_factor`の計算から得られる最大容量。 |
| `preemptive_mode`    | プリエンプティブモードがオンになっている場合、ジョブがリクエストされるのは、インスタンスが使用可能であることが確認された場合だけです。この動作により、プロビジョニングの遅延なしに、ほぼすぐにジョブを開始できます。プリエンプティブモードがオフになっている場合、まずジョブがリクエストされた後、次にシステムが必要なキャパシティを検出したりプロビジョニングしたりしようとします。 |

アイドル状態のインスタンスを削除するかどうかを決定するために、taskscalerは`idle_time`をインスタンスのアイドル期間と比較します。各インスタンスのアイドル期間は、インスタンスが次の操作を行った時点から計算されます。

- 最後にジョブを完了した時点（インスタンスが以前に使用されていた場合）。
- プロビジョニングされた時点（未使用の場合）。

このチェックは、スケーリングイベント中に発生します。設定されている`idle_time`を超えるインスタンスは、必要な`idle_count`ジョブキャパシティを維持するために必要な場合を除き、削除されます。

`scale_factor`を設定すると、`idle_count`が最小の`idle`容量になり、`scaler_factor_limit`が最大の`idle`容量になります。

複数のポリシーを定義できます。最後に一致したポリシーが使用されます。

次の例では、アイドルカウント`1`は、月曜日から金曜日の08:00から15:59の間に使用されます。それ以外の場合、アイドルカウントは0です。

```toml
[[runners.autoscaler.policy]]
  idle_count        = 0
  idle_time         = "0s"
  periods           = ["* * * * *"]

[[runners.autoscaler.policy]]
  idle_count        = 1
  idle_time         = "30m0s"
  periods           = ["* 8-15 * * mon-fri"]
```

### periods構文 {#periods-syntax-1}

`periods`設定には、ポリシーが有効になっている期間を示す、unix-cron形式の文字列の配列が含まれています。cron形式は、次の5つのフィールドで構成されています。

```plaintext
 ┌────────── minute (0 - 59)
 │ ┌──────── hour (0 - 23)
 │ │ ┌────── day of month (1 - 31)
 │ │ │ ┌──── month (1 - 12)
 │ │ │ │ ┌── day of week (1 - 7 or MON-SUN, 0 is an alias for Sunday)
 * * * * *
```

- `-`は、2つの数値の間で範囲を指定するときに使用できます。
- `*`は、そのフィールドの有効な値の範囲全体を表すときに使用できます。
- `/`に続く数字は、範囲内でその数字ごとにスキップするときに範囲の後に使用できます。たとえば、hourフィールドに0-12/2と指定すると、00:00から00:12の間、2時間ごとに期間がアクティブになります。
- `,`は、フィールドの有効な数値または範囲のリストを区切るときに使用できます。たとえば、`1,2,6-9`などです。

このcronジョブは時間の範囲を表していることを覚えておいてください。例: 

| 期間               | 効果 |
|----------------------|--------|
| `1 * * * * *`        | 1時間ごとに1分間にわたってルールが有効になります（非常に効果的である可能性は低い） |
| `* 0-12 * * *`       | 毎日の開始時に12時間にわたってルールが有効になります |
| `0-30 13,16 * * SUN` | 毎週日曜日の午後1時に30分間、午後4時に30分間にわたってルールが有効になります |

## `[runners.autoscaler.vm_isolation]`セクション {#the-runnersautoscalervm_isolation-section}

VM分離は[`nesting`](../executors/instance.md#nested-virtualization)を使用し、これはmacOSでのみサポートされています。

| パラメータ        | 説明 |
|------------------|-------------|
| `enabled`        | VM分離を有効にするかどうかを指定します。デフォルト: `false`。 |
| `nesting_host`   | `nesting`デーモンホスト。 |
| `nesting_config` | `nesting`設定。JSONにシリアル化され、`nesting`デーモンに送信されます。 |
| `image`          | ジョブイメージが指定されていない場合に、nestingデーモンで使用されるデフォルトイメージ。 |

## `[runners.autoscaler.vm_isolation.connector_config]`セクション {#the-runnersautoscalervm_isolationconnector_config-section}

`[runners.autoscaler.vm_isolation.connector_config]`セクションのパラメータは、[`[runners.autoscaler.connector_config]`](#the-runnersautoscalerconnector_config-section)セクションと同じですが、オートスケールされたインスタンスではなく、`nesting`でプロビジョニングされた仮想マシンへの接続に使用されます。

## `[runners.custom]`セクション {#the-runnerscustom-section}

次のパラメータは、[カスタムexecutor](../executors/custom.md)の設定を定義します。

| パラメータ               | 型         | 説明 |
|-------------------------|--------------|-------------|
| `config_exec`           | 文字列       | 実行可能ファイルのパス。これにより、ユーザーはジョブ開始前に一部の設定を上書きできます。これらの値は、[`[[runners]]`](#the-runners-section)セクションで設定されている値を上書きします。一覧は[Custom executorのドキュメント](../executors/custom.md#config)にあります。 |
| `config_args`           | 文字列配列 | `config_exec`実行可能ファイルに渡される最初の引数セット。 |
| `config_exec_timeout`   | 整数      | `config_exec`の実行が完了するまでのタイムアウト（秒）。デフォルトは3600秒（1時間）。 |
| `prepare_exec`          | 文字列       | 環境を準備するための実行可能ファイルのパス。 |
| `prepare_args`          | 文字列配列 | `prepare_exec`実行可能ファイルに渡される最初の引数セット。 |
| `prepare_exec_timeout`  | 整数      | `prepare_exec`の実行が完了するまでのタイムアウト（秒）。デフォルトは3600秒（1時間）。 |
| `run_exec`              | 文字列       | **必須**。環境内でスクリプトを実行するための実行可能ファイルのパス。たとえば、クローンスクリプトやビルドスクリプトなどです。 |
| `run_args`              | 文字列配列 | `run_exec`実行可能ファイルに渡される最初の引数セット。 |
| `cleanup_exec`          | 文字列       | 環境をクリーンアップするための実行可能ファイルのパス。 |
| `cleanup_args`          | 文字列配列 | `cleanup_exec`実行可能ファイルに渡される最初の引数セット。 |
| `cleanup_exec_timeout`  | 整数      | `cleanup_exec`の実行が完了するまでのタイムアウト（秒）。デフォルトは3600秒（1時間）。 |
| `graceful_kill_timeout` | 整数      | `prepare_exec`と`cleanup_exec`が（ジョブのキャンセル中などに）終了した場合に待機する時間（秒）。このタイムアウト後に、プロセスが強制終了されます。デフォルトは600秒（10分）。 |
| `force_kill_timeout`    | 整数      | kill（強制終了）シグナルがスクリプトに送信された後に待機する時間（秒）。デフォルトは600秒（10分）。 |

## `[runners.cache]`セクション {#the-runnerscache-section}

次のパラメータは、分散キャッシュ機能を定義します。詳細については、[Runnerオートスケールに関するドキュメント](autoscale.md#distributed-runners-caching)を参照してください。

| パラメータ                | 型    | 説明 |
|--------------------------|---------|-------------|
| `Type`                   | 文字列  | `s3`、`gcs`、`azure`のいずれか。 |
| `Path`                   | 文字列  | キャッシュURLの先頭に付加するパスの名前。 |
| `Shared`                 | ブール値 | Runner間でのキャッシュ共有を有効にします。デフォルトは`false`です。 |
| `MaxUploadedArchiveSize` | int64   | クラウドストレージにアップロードされるキャッシュアーカイブの制限（バイト単位）。悪意のあるアクターはこの制限を回避できるため、GCSアダプターは署名付きURLのX-Goog-Content-Length-Rangeヘッダーによってこの制限を適用します。クラウドストレージプロバイダーにも制限を設定する必要があります。 |

以下の環境変数を使用して、キャッシュの圧縮を設定できます:

| 変数                   | 説明                           | デフォルト   | 値                                          |
|----------------------------|---------------------------------------|-----------|-------------------------------------------------|
| `CACHE_COMPRESSION_FORMAT` | キャッシュアーカイブの圧縮形式 | `zip`     | `zip`、`tarzstd`                                |
| `CACHE_COMPRESSION_LEVEL`  | キャッシュアーカイブの圧縮レベル  | `default` | `fastest`、`fast`、`default`、`slow`、`slowest` |

`tarzstd`形式は、`zip`よりも優れた圧縮率を提供する、Zstandard圧縮でTARを使用します。圧縮レベルの範囲は、`fastest`（最大速度を実現するための最小圧縮）から`slowest`（最小ファイルサイズを実現するための最大圧縮）です。`default`レベルは、圧縮率と速度のバランスの取れたトレードオフを提供します。

例: 

```yaml
job:
  variables:
    CACHE_COMPRESSION_FORMAT: tarzstd
    CACHE_COMPRESSION_LEVEL: fast
```

キャッシュメカニズムは、事前署名付きURLを使用してキャッシュをアップロードおよびダウンロードします。GitLab Runnerがそれ自体のインスタンスでURLに署名します。ジョブのスクリプト（キャッシュのアップロード/ダウンロードスクリプトを含む）がローカルマシンまたは外部マシンで実行されるかどうかは関係ありません。たとえば、`shell` executorや`docker` executorは、GitLab Runnerプロセスが実行されているマシンでスクリプトを実行します。一方で`virtualbox`や`docker+machine`は、別のVMに接続してスクリプトを実行します。このプロセスは、キャッシュアダプターの認証情報が漏洩する可能性を最小限に抑えるというセキュリティ上の理由によるものです。

[S3キャッシュアダプター](#the-runnerscaches3-section)がIAMインスタンスプロファイルを使用するように設定されている場合、このアダプターはGitLab Runnerマシンに接続されているプロファイルを使用します。[GCSキャッシュアダプター](#the-runnerscachegcs-section)が`CredentialsFile`を使用するように設定されている場合も同様です。このファイルがGitLab Runnerマシンに存在している必要があります。

次の表に、`config.toml`、`register`のCLIオプションおよび環境変数を示します。これらの環境変数を定義すると、新しいGitLab Runnerを登録した後に、値が`config.toml`に保存されます。

`config.toml`からS3の認証情報を省略し、環境変数から静的な認証情報を読み込む場合は、`AWS_ACCESS_KEY_ID`と`AWS_SECRET_ACCESS_KEY`を定義できます。詳細については、[AWS SDKデフォルト認証情報チェーンセクション](#aws-sdk-default-credential-chain)を参照してください。

| 設定                        | TOMLフィールド                                        | `register`のCLIオプション                  | `register`の環境変数 |
|--------------------------------|---------------------------------------------------|--------------------------------------------|-------------------------------------|
| `Type`                         | `[runners.cache] -> Type`                         | `--cache-type`                             | `$CACHE_TYPE`                       |
| `Path`                         | `[runners.cache] -> Path`                         | `--cache-path`                             | `$CACHE_PATH`                       |
| `Shared`                       | `[runners.cache] -> Shared`                       | `--cache-shared`                           | `$CACHE_SHARED`                     |
| `S3.ServerAddress`             | `[runners.cache.s3] -> ServerAddress`             | `--cache-s3-server-address`                | `$CACHE_S3_SERVER_ADDRESS`          |
| `S3.AccessKey`                 | `[runners.cache.s3] -> AccessKey`                 | `--cache-s3-access-key`                    | `$CACHE_S3_ACCESS_KEY`              |
| `S3.SecretKey`                 | `[runners.cache.s3] -> SecretKey`                 | `--cache-s3-secret-key`                    | `$CACHE_S3_SECRET_KEY`              |
| `S3.SessionToken`              | `[runners.cache.s3] -> SessionToken`              | `--cache-s3-session-token`                 | `$CACHE_S3_SESSION_TOKEN`           |
| `S3.BucketName`                | `[runners.cache.s3] -> BucketName`                | `--cache-s3-bucket-name`                   | `$CACHE_S3_BUCKET_NAME`             |
| `S3.BucketLocation`            | `[runners.cache.s3] -> BucketLocation`            | `--cache-s3-bucket-location`               | `$CACHE_S3_BUCKET_LOCATION`         |
| `S3.Insecure`                  | `[runners.cache.s3] -> Insecure`                  | `--cache-s3-insecure`                      | `$CACHE_S3_INSECURE`                |
| `S3.AuthenticationType`        | `[runners.cache.s3] -> AuthenticationType`        | `--cache-s3-authentication_type`           | `$CACHE_S3_AUTHENTICATION_TYPE`     |
| `S3.ServerSideEncryption`      | `[runners.cache.s3] -> ServerSideEncryption`      | `--cache-s3-server-side-encryption`        | `$CACHE_S3_SERVER_SIDE_ENCRYPTION`  |
| `S3.ServerSideEncryptionKeyID` | `[runners.cache.s3] -> ServerSideEncryptionKeyID` | `--cache-s3-server-side-encryption-key-id` | `$CACHE_S3_SERVER_SIDE_ENCRYPTION_KEY_ID` |
| `S3.DualStack`                 | `[runners.cache.s3] -> DualStack`                 | `--cache-s3-dual-stack`                    | `$CACHE_S3_DUAL_STACK`              |
| `S3.Accelerate`                | `[runners.cache.s3] -> Accelerate`                | `--cache-s3-accelerate`                    | `$CACHE_S3_ACCELERATE`              |
| `S3.PathStyle`                 | `[runners.cache.s3] -> PathStyle`                 | `--cache-s3-path-style`                    | `$CACHE_S3_PATH_STYLE`              |
| `S3.RoleARN`                   | `[runners.cache.s3] -> RoleARN`                   | `--cache-s3-role-arn`                      | `$CACHE_S3_ROLE_ARN`                |
| `S3.UploadRoleARN`             | `[runners.cache.s3] -> UploadRoleARN`             | `--cache-s3-upload-role-arn`               | `$CACHE_S3_UPLOAD_ROLE_ARN`         |
| `GCS.AccessID`                 | `[runners.cache.gcs] -> AccessID`                 | `--cache-gcs-access-id`                    | `$CACHE_GCS_ACCESS_ID`              |
| `GCS.PrivateKey`               | `[runners.cache.gcs] -> PrivateKey`               | `--cache-gcs-private-key`                  | `$CACHE_GCS_PRIVATE_KEY`            |
| `GCS.CredentialsFile`          | `[runners.cache.gcs] -> CredentialsFile`          | `--cache-gcs-credentials-file`             | `$GOOGLE_APPLICATION_CREDENTIALS`   |
| `GCS.BucketName`               | `[runners.cache.gcs] -> BucketName`               | `--cache-gcs-bucket-name`                  | `$CACHE_GCS_BUCKET_NAME`            |
| `Azure.AccountName`            | `[runners.cache.azure] -> AccountName`            | `--cache-azure-account-name`               | `$CACHE_AZURE_ACCOUNT_NAME`         |
| `Azure.AccountKey`             | `[runners.cache.azure] -> AccountKey`             | `--cache-azure-account-key`                | `$CACHE_AZURE_ACCOUNT_KEY`          |
| `Azure.ContainerName`          | `[runners.cache.azure] -> ContainerName`          | `--cache-azure-container-name`             | `$CACHE_AZURE_CONTAINER_NAME`       |
| `Azure.StorageDomain`          | `[runners.cache.azure] -> StorageDomain`          | `--cache-azure-storage-domain`             | `$CACHE_AZURE_STORAGE_DOMAIN`       |

### キャッシュキーの処理 {#cache-key-handling}

{{< history >}}

- [導入](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5751)：GitLab Runner v18.4.0。

{{< /history >}}

GitLab Runner 18.4.0以降では、`FF_HASH_CACHE_KEYS` [機能フラグ](feature-flags.md)を使用してキャッシュキーにハッシュを付けることができます。

`FF_HASH_CACHE_KEYS`がオフになっている場合（デフォルト）、GitLab Runnerはキャッシュキーをサニタイズしてから、ローカルのキャッシュファイルとストレージバケット内のオブジェクトの両方のパスをビルドするために使用します。サニタイズによってキャッシュキーが変更された場合、GitLab Runnerはこの変更をログに記録します。GitLab Runnerがキャッシュキーをサニタイズできない場合、これもログに記録し、この特定のキャッシュは使用しません。

この機能フラグをオンにすると、GitLab Runnerはキャッシュキーにハッシュを付けてから、ローカルのキャッシュアーティファクトとリモートストレージバケット内のオブジェクトのパスをビルドするために使用します。GitLab Runnerは、キャッシュキーをサニタイズしません。どのキャッシュキーが特定のキャッシュアーティファクトを作成したかを理解できるように、GitLab Runnerはメタデータを添付します:

- ローカルのキャッシュアーティファクトの場合、GitLab Runnerは、キャッシュアーティファクト`cache.zip`の横に`metadata.json`ファイルを配置し、次のコンテンツを含めます:

  ```json
  {"cachekey": "the human readable cache key"}
  ```

- 分散キャッシュのキャッシュアーティファクトの場合、GitLab Runnerはメタデータをストレージオブジェクトblobに直接添付し、キー`cachekey`を付与します。クラウドプロバイダーのメカニズムを使用してクエリできます。例については、AWS S3の[ユーザー定義オブジェクトメタデータ](https://docs.aws.amazon.com/AmazonS3/latest/userguide/UsingMetadata.html#UserMetadata)を参照してください。

{{< alert type="warning" >}}

`FF_HASH_CACHE_KEYS`を変更すると、ハッシュでキャッシュキーによってキャッシュアーティファクトの名前と場所が変更されるため、GitLab Runnerは既存のキャッシュアーティファクトを無視します。この変更は、`FF_HASH_CACHE_KEYS=true`から`FF_HASH_CACHE_KEYS=false`、およびその逆に、両方向に適用されます。

分散キャッシュを共有する複数のRunnerを実行しているが、`FF_HASH_CACHE_KEYS`の設定が異なる場合、キャッシュアーティファクトは共有されません。

したがって、ベストプラクティスは次のとおりです:

- 分散キャッシュを共有するRunner間で`FF_HASH_CACHE_KEYS`を同期した状態に保ちます。

- `FF_HASH_CACHE_KEYS`を変更した後、キャッシュミス、キャッシュアーティファクトの再ビルド、および最初のジョブの実行時間が長くなることを想定します。

{{< /alert >}}

{{< alert type="warning" >}}

`FF_HASH_CACHE_KEYS`をオンにしても、（ヘルパーイメージを以前のバージョンに固定したなどの理由で）以前のバージョンのヘルパーバイナリを実行すると、キャッシュキーへのハッシュの適用と、キャッシュのアップロードまたはダウンロードは引き続き機能します。ただし、GitLab Runnerはキャッシュアーティファクトのメタデータを保持しません。

{{< /alert >}}

### `[runners.cache.s3]`セクション {#the-runnerscaches3-section}

次のパラメータは、キャッシュ用のS3ストレージを定義します。

| パラメータ                   | 型    | 説明 |
|-----------------------------|---------|-------------|
| `ServerAddress`             | 文字列  | S3互換サーバーの`host:port`。AWS以外のサーバーを使用している場合は、ストレージ製品のドキュメントを参照して、正しいアドレスを確認してください。DigitalOceanの場合、アドレスの形式は`spacename.region.digitaloceanspaces.com`である必要があります。 |
| `AccessKey`                 | 文字列  | S3インスタンス用に指定されたアクセスキー。 |
| `SecretKey`                 | 文字列  | S3インスタンス用に指定されたシークレットキー。 |
| `SessionToken`              | 文字列  | 一時的な認証情報を使用する場合に、S3インスタンス用に指定されたセッショントークン。 |
| `BucketName`                | 文字列  | キャッシュが保存されるストレージバケットの名前。 |
| `BucketLocation`            | 文字列  | S3リージョンの名前。 |
| `Insecure`                  | ブール値 | S3サービスが`HTTP`で利用可能な場合は、`true`に設定します。デフォルトは`false`です。 |
| `AuthenticationType`        | 文字列  | `iam`または`access-key`に設定します。`ServerAddress`、`AccessKey`、および`SecretKey`がすべて指定されている場合、デフォルトは`access-key`です。`ServerAddress`、`AccessKey`、または`SecretKey`が指定されていない場合、デフォルトは`iam`です。 |
| `ServerSideEncryption`      | 文字列  | S3で使用するサーバー側の暗号化の種類。GitLab 15.3以降で使用可能な種類は、`S3`または`KMS`です。GitLab 17.5以降では、[`DSSE-KMS`](https://docs.aws.amazon.com/AmazonS3/latest/userguide/UsingDSSEncryption.html)がサポートされています。 |
| `ServerSideEncryptionKeyID` | 文字列  | KMSを使用する場合に暗号化に使用されるKMSキーのエイリアス、ID、またはAmazonリソースネーム。エイリアスを使用する場合は、`alias/`をプレフィックスとして付けます。クロスアカウントシナリオでは、ARN形式を使用します。GitLab 15.3以降で利用可能です。 |
| `DualStack`                 | ブール値 | IPv4およびIPv6エンドポイントを有効にします。デフォルトは`true`です。AWS S3 Expressを使用している場合は、この設定を無効にしてください。`ServerAddress`を設定すると、GitLabはこの設定を無視します。GitLab 17.5以降で利用可能です。 |
| `Accelerate`                | ブール値 | AWS S3 Transfer Acceleration（転送高速化）を有効にします。`ServerAddress`がAccelerated（高速化）エンドポイントとして設定されている場合、GitLabは自動的にこれを`true`に設定します。GitLab 17.5以降で利用可能です。 |
| `PathStyle`                 | ブール値 | パス形式のアクセスを有効にします。デフォルトでは、GitLabは`ServerAddress`の値に基づいてこの設定を自動的に検出します。GitLab 17.5以降で利用可能です。 |
| `UploadRoleARN`             | 文字列  | 非推奨。代わりに`RoleARN`を使用してください。時間制限付きの`PutObject` S3リクエストを生成するために`AssumeRole`で使用できるAWSロールARNを指定します。S3マルチパートアップロードを有効にします。GitLab 17.5以降で利用可能です。 |
| `RoleARN`                   | 文字列  | 時間制限付きの`GetObject`と`PutObject` S3リクエストを生成するために`AssumeRole`で使用できるAWSロールARNを指定します。S3マルチパート転送を有効にします。GitLab 17.8以降で利用可能です。 |

例: 

```toml
[runners.cache]
  Type = "s3"
  Path = "path/to/prefix"
  Shared = false
  [runners.cache.s3]
    ServerAddress = "s3.amazonaws.com"
    AccessKey = "AWS_S3_ACCESS_KEY"
    SecretKey = "AWS_S3_SECRET_KEY"
    BucketName = "runners-cache"
    BucketLocation = "eu-west-1"
    Insecure = false
    ServerSideEncryption = "KMS"
    ServerSideEncryptionKeyID = "alias/my-key"
```

## 認証 {#authentication}

GitLab Runnerは、設定に基づいてS3に異なる認証方法を使用します。

### 静的な認証情報 {#static-credentials}

Runnerは、次の場合に静的アクセスキー認証を使用します:

- `ServerAddress`、`AccessKey`、および`SecretKey`パラメータが仕様されていますが、`AuthenticationType`は提供されていません。
- `AuthenticationType = "access-key"`が明示的に設定されています。

### AWS SDKのデフォルト認証情報チェーン {#aws-sdk-default-credential-chain}

Runnerは、次の場合に[AWS SDKのデフォルト認証情報チェーン](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html#specifying-credentials)を使用します:

- `ServerAddress`、`AccessKey`、または`SecretKey`のいずれかが省略され、`AuthenticationType`が提供されていません。
- `AuthenticationType = "iam"`が明示的に設定されています。

この認証情報チェーンは、次の順序で認証を試みます:

1. 環境変数（`AWS_ACCESS_KEY_ID`、`AWS_SECRET_ACCESS_KEY`）
1. 共有認証情報ファイル（`~/.aws/credentials`）
1. IAMインスタンスプロファイル（EC2インスタンスの場合）
1. SDKでサポートされている他のAWS認証情報ソース

`RoleARN`が仕様されていない場合、デフォルトの認証情報チェーンはRunnerマネージャーによって実行されます。これは、ビルドが実行されるマシンと同じマシン上にあるとは限りません。たとえば、[オートスケールする](autoscale.md)の設定では、ジョブは別のマシンで実行されます。同様に、Kubernetesエグゼキューターを使用すると、ビルドポッドもRunnerマネージャーとは異なるノードで実行できます。この動作により、Runnerマネージャーにのみバケットレベルのアクセス権を付与できます。

`RoleARN`が仕様されている場合、認証情報はヘルパーイメージの実行コンテキスト内で解決されます。詳細については、[RoleARN](#enable-multipart-transfers-with-rolearn)を参照してください。

Helmチャートを使用してGitLab Runnerをインストールし、`rbac.create`が`values.yaml`ファイルで`true`に設定されている場合、サービスアカウントが作成されます。サービスアカウントの注釈は、`rbac.serviceAccountAnnotations`セクションから取得されます。

Amazon EKSのRunnerの場合、サービスアカウントに割り当てるIAMロールを指定できます。必要な特定のアノテーションは`eks.amazonaws.com/role-arn: arn:aws:iam::<ACCOUNT_ID>:role/<IAM_ROLE_NAME>`です。

このロールのIAMポリシーには、指定されたバケットに対して次のアクションを実行する権限が必要です。

- `s3:PutObject`
- `s3:GetObjectVersion`
- `s3:GetObject`
- `s3:DeleteObject`
- `s3:ListBucket`

`KMS`タイプの`ServerSideEncryption`を使用する場合、このロールには、指定されたAWS KMSキーに対して次のアクションを実行する権限も必要です。

- `kms:Encrypt`
- `kms:Decrypt`
- `kms:ReEncrypt*`
- `kms:GenerateDataKey*`
- `kms:DescribeKey`

`SSE-C`タイプの`ServerSideEncryption`はサポートされていません。`SSE-C`では、事前署名付きURLに加えて、ユーザー提供のキーを含むヘッダーをダウンロードリクエストに対して指定する必要があります。これは、ジョブにキーマテリアルを渡すことになり、キーの安全を保証できません。これにより、復号化キーが漏洩する可能性があります。この問題に関するディスカッションについては、[このマージリクエスト](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/3295)を参照してください。

{{< alert type="note" >}}

AWS S3キャッシュにアップロードできる単一ファイルの最大サイズは5 GBです。この動作に対する潜在的な回避策についてのディスカッションについては、[このイシュー](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/26921)を参照してください。

{{< /alert >}}

#### Runnerキャッシュ用のS3バケットでKMSキー暗号化を使用する {#use-kms-key-encryption-in-s3-bucket-for-runner-cache}

`GenerateDataKey` APIはKMS対称キーを使用して、クライアント側の暗号化（<https://docs.aws.amazon.com/kms/latest/APIReference/API_GenerateDataKey.html>）用のデータキーを作成します。KMSキーの正しい設定は次のとおりです。

| 属性 | 説明 |
|-----------|-------------|
| キータイプ  | 対称   |
| 生成元    | `AWS_KMS`   |
| キー仕様  | `SYMMETRIC_DEFAULT` |
| キーの用途 | 暗号化と復号化 |

`rbac.serviceAccountName`で定義されたServiceAccountに割り当てられたロールのIAMポリシーには、KMSキーに対して次のアクションを実行する権限が必要です。

- `kms:GetPublicKey`
- `kms:Decrypt`
- `kms:Encrypt`
- `kms:DescribeKey`
- `kms:GenerateDataKey`

#### `RoleARN`でマルチパート転送を有効にする {#enable-multipart-transfers-with-rolearn}

キャッシュへのアクセスを制限するために、Runnerマネージャーは時間制限のある[事前署名付きURL](https://docs.aws.amazon.com/AmazonS3/latest/userguide/using-presigned-url.html)を生成し、ジョブがキャッシュからのダウンロードやキャッシュへアップロードを行えるようにします。ただし、AWS S3では[1つのPUTリクエストが5 GBに制限されています](https://docs.aws.amazon.com/AmazonS3/latest/userguide/upload-objects.html)。5 GBを超えるファイルの場合は、マルチパートアップロードAPIを使用する必要があります。

マルチパート転送は、AWS S3でのみサポートされており、他のS3プロバイダーではサポートされていません。Runnerマネージャーはさまざまなプロジェクトのジョブを処理することから、バケット全体の権限を含むS3認証情報を渡すことができません。代わりに、Runnerマネージャーは時間制限のある事前署名付きURLと範囲が限定された認証情報を使用して、特定のオブジェクトへのアクセスを制限します。

AWSでS3マルチパート転送を使用するには、`RoleARN`に`arn:aws:iam:::<ACCOUNT ID>:<YOUR ROLE NAME>`形式でIAMロールを指定します。このロールは、バケット内の特定のblobへの書き込みに限定された、時間制限のあるAWS認証情報を生成します。元のS3認証情報が、指定された`RoleARN`の`AssumeRole`にアクセスできることを確認してください。

`RoleARN`で指定されたIAMロールには、次の権限が必要です。

- `BucketName`で指定されたバケットへの`s3:GetObject`アクセス権。
- `BucketName`で指定されたバケットへの`s3:PutObject`アクセス権。
- `BucketName`で指定されたバケットへの`s3:ListBucket`アクセス権。
- KMSまたはDSSE-KMSを使用したサーバー側の暗号化が有効になっている場合は、`kms:Decrypt`と`kms:GenerateDataKey`権限。

たとえば、ARN `arn:aws:iam::1234567890123:role/my-instance-role`を持つEC2インスタンスに`my-instance-role`という名前のIAMロールが添付されているとします。

この場合、`BucketName`に対して`s3:PutObject`権限のみを持つ新しいロール`arn:aws:iam::1234567890123:role/my-upload-role`を作成できます。`my-instance-role`のAWS設定では、`Trust relationships`は次のようになります。

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": {
                "AWS": "arn:aws:iam::1234567890123:role/my-upload-role"
            },
            "Action": "sts:AssumeRole"
        }
    ]
}
```

`my-instance-role`を`RoleARN`として再利用して、新しいロールの作成を回避することもできます。その場合は、`my-instance-role`に`AssumeRole`権限があることを確認してください。たとえば、EC2インスタンスに関連付けられているIAMプロファイルの`Trust relationships`は次のようになります。

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": {
                "Service": "ec2.amazonaws.com",
                "AWS": "arn:aws:iam::1234567890123:role/my-instance-role"
            },
            "Action": "sts:AssumeRole"
        }
    ]
}
```

AWSコマンドラインインターフェースを使用して、インスタンスに`AssumeRole`権限があることを確認できます。例: 

```shell
aws sts assume-role --role-arn arn:aws:iam::1234567890123:role/my-upload-role --role-session-name gitlab-runner-test1
```

##### `RoleARN`によるアップロードの仕組み {#how-uploads-work-with-rolearn}

`RoleARN`が設定されている場合、Runnerがキャッシュにアップロードするたびに次の処理が行われます。

1. Runnerマネージャーは、（`AuthenticationType`、`AccessKey`、`SecretKey`で指定された）元のS3認証情報を取得します。
1. RunnerマネージャーはこのS3認証情報を使用して、Amazon Security Token Service（STS）に`RoleARN`を使った[`AssumeRole`](https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRole.html)のリクエストを送信します。ポリシーリクエストは次のようになります。

   ```json
   {
       "Version": "2012-10-17",
       "Statement": [
           {
               "Effect": "Allow",
               "Action": ["s3:PutObject"],
               "Resource": "arn:aws:s3:::<YOUR-BUCKET-NAME>/<CACHE-FILENAME>"
           }
       ]
   }
   ```

1. リクエストが成功した場合、Runnerマネージャーは制限付きセッションで一時的なAWS認証情報を取得します。
1. Runnerマネージャーは、これらの認証情報とURLを`s3://<bucket name>/<filename>`形式でキャッシュアーカイバーに渡し、キャッシュアーカイバーがファイルをアップロードします。

#### Kubernetes ServiceAccountリソース用のIAMロールを有効にする {#enable-iam-roles-for-kubernetes-serviceaccount-resources}

サービスアカウントにIAMロールを使用するには、IAM OIDCプロバイダーが[クラスター用に存在する必要があります](https://docs.aws.amazon.com/eks/latest/userguide/enable-iam-roles-for-service-accounts.html)。IAM OIDCプロバイダーがクラスターに関連付けられたら、IAMロールを作成してRunnerのサービスアカウントに関連付けることができます。

1. **Create Role**（ロール作成）画面の**Select type of trusted entity**（信頼されたエンティティのタイプを選択）で、**Web Identity**（Web ID）を選択します。
1. ロールの**Trusted Relationships**（信頼関係）タブで次のようにします。

   - **Trusted entities**（信頼されたエンティティ）セクションの形式は`arn:aws:iam::<ACCOUNT_ID>:oidc-provider/oidc.eks.<AWS_REGION>.amazonaws.com/id/<OIDC_ID>`である必要があります。**OIDC ID**は、Amazon EKSクラスターの**Configuration**（設定）タブにあります。

   - **Condition**（条件）セクションには、`rbac.serviceAccountName`で定義されたGitLab Runnerサービスアカウント、または`rbac.create`が`true`に設定されている場合に作成されるデフォルトのサービスアカウントが必要です。

     | 条件      | キー                                                    | 値 |
     |----------------|--------------------------------------------------------|-------|
     | `StringEquals` | `oidc.eks.<AWS_REGION>.amazonaws.com/id/<OIDC_ID>:sub` | `system:serviceaccount:<GITLAB_RUNNER_NAMESPACE>:<GITLAB_RUNNER_SERVICE_ACCOUNT>` |

#### S3 Express One Zoneバケットを使用する {#use-s3-express-one-zone-buckets}

{{< history >}}

- GitLab Runner 17.5.0で導入されました。

{{< /history >}}

{{< alert type="note" >}}

Runnerマネージャーが1つの特定のオブジェクトに対するアクセスを制限できないため、[S3 Express One Zoneディレクトリバケットは`RoleARN`では機能しません](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/38484#note_2313111840)。

{{< /alert >}}

1. [Amazonのチュートリアル](https://docs.aws.amazon.com/AmazonS3/latest/userguide/s3-express-getting-started.html)に従って、S3 Express One Zoneバケットを設定します。
1. `BucketName`と`BucketLocation`を使用して`config.toml`を設定します。
1. S3 Expressはデュアルスタックエンドポイントをサポートしていないため、`DualStack`を`false`に設定します。

`config.toml`の例

```toml
[runners.cache]
  Type = "s3"
  [runners.cache.s3]
    BucketName = "example-express--usw2-az1--x-s3"
    BucketLocation = "us-west-2"
    DualStack = false
```

### `[runners.cache.gcs]`セクション {#the-runnerscachegcs-section}

次のパラメータは、Google Cloud Storageのネイティブサポートを定義します。これらの値の詳細については、[Google Cloud Storage（GCS）の認証に関するドキュメント](https://docs.cloud.google.com/storage/docs/authentication#service_accounts)を参照してください。

| パラメータ         | 型   | 説明 |
|-------------------|--------|-------------|
| `CredentialsFile` | 文字列 | Google JSONキーファイルのパス。`service_account`タイプのみがサポートされています。設定されている場合、この値は`config.toml`で直接設定された`AccessID`と`PrivateKey`よりも優先されます。 |
| `AccessID`        | 文字列 | ストレージへのアクセスに使用されるGCPサービスアカウントのID。 |
| `PrivateKey`      | 文字列 | GCSリクエストの署名に使用される秘密キー。 |
| `BucketName`      | 文字列 | キャッシュが保存されるストレージバケットの名前。 |

例:

**`config.toml`ファイルで直接設定された認証情報**

```toml
[runners.cache]
  Type = "gcs"
  Path = "path/to/prefix"
  Shared = false
  [runners.cache.gcs]
    AccessID = "cache-access-account@test-project-123456.iam.gserviceaccount.com"
    PrivateKey = "-----BEGIN PRIVATE KEY-----\nXXXXXX\n-----END PRIVATE KEY-----\n"
    BucketName = "runners-cache"
```

**GCPからダウンロードしたJSONファイル内の認証情報**

```toml
[runners.cache]
  Type = "gcs"
  Path = "path/to/prefix"
  Shared = false
  [runners.cache.gcs]
    CredentialsFile = "/etc/gitlab-runner/service-account.json"
    BucketName = "runners-cache"
```

**GCPのメタデータサーバーからのアプリケーションデフォルト認証情報（ADC）**

GitLab RunnerとGoogle Cloud ADCを使用する場合、通常はデフォルトのサービスアカウントを使用します。その場合、インスタンスの認証情報を提供する必要はありません。

```toml
[runners.cache]
  Type = "gcs"
  Path = "path/to/prefix"
  Shared = false
  [runners.cache.gcs]
    BucketName = "runners-cache"
```

ADCを使用する場合は、使用するサービスアカウントに`iam.serviceAccounts.signBlob`権限があることを確認してください。通常、これは[サービスアカウントトークン作成者のロール](https://docs.cloud.google.com/iam/docs/service-account-permissions#token-creator-role)をサービスアカウントに付与することで行われます。

#### GKEのワークロードアイデンティティフェデレーション {#workload-identity-federation-for-gke}

GKEのワークロードアイデンティティフェデレーションは、アプリケーションデフォルト認証情報（ADC）でサポートされています。ワークロードアイデンティティが機能しないイシューが発生した場合:

- `ERROR: generating signed URL`メッセージについては、Runnerポッドログ（ビルドログではなく）を確認してください。このエラーは、次のようなパーミッションのイシューを示している可能性があります:

  ```plaintext
  IAM returned 403 Forbidden: Permission 'iam.serviceAccounts.getAccessToken' denied on resource (or it may not exist).
  ```

- Runnerポッド内から次の`curl`コマンドを試してください:

  ```shell
  curl -H "Metadata-Flavor: Google" http://169.254.169.254/computeMetadata/v1/instance/service-accounts/default/email
  ```

   このコマンドは、正しいKubernetesサービスアカウントを返すはずです。次に、アクセストークンを取得してみてください:

  ```shell
  curl -H "Metadata-Flavor: Google" http://169.254.169.254/computeMetadata/v1/instance/service-accounts/default/token?scopes=https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fcloud-platform
  ```

   コマンドが成功すると、結果はアクセストークンを含むJSONペイロードを返します。失敗した場合は、サービスアカウントの権限を確認してください。

### `[runners.cache.azure]`セクション {#the-runnerscacheazure-section}

次のパラメータは、Azure Blob Storageのネイティブサポートを定義します。詳細については、[Azure Blob Storageのドキュメント](https://learn.microsoft.com/en-us/azure/storage/blobs/storage-blobs-introduction)を参照してください。S3やGCSではオブジェクトの集合に`bucket`という用語が使用されていますが、Azureではblobの集合に`container`が使用されています。

| パラメータ       | 型   | 説明 |
|-----------------|--------|-------------|
| `AccountName`   | 文字列 | ストレージへのアクセスに使用するAzure Blob Storageアカウントの名前。 |
| `AccountKey`    | 文字列 | コンテナへのアクセスに使用するストレージアカウントのアクセスキー。設定から`AccountKey`を省略するには、[AzureワークロードまたはマネージドID](#azure-workload-and-managed-identities)を使用します。 |
| `ContainerName` | 文字列 | キャッシュデータを保存する[ストレージコンテナ](https://learn.microsoft.com/en-us/azure/storage/blobs/storage-blobs-introduction#containers)の名前。 |
| `StorageDomain` | 文字列 | [Azureストレージエンドポイントのサービスに使用される](https://learn.microsoft.com/en-us/azure/china/resources-developer-guide#check-endpoints-in-azure)ドメイン名（オプション）。デフォルトは`blob.core.windows.net`です。 |

例: 

```toml
[runners.cache]
  Type = "azure"
  Path = "path/to/prefix"
  Shared = false
  [runners.cache.azure]
    AccountName = "<AZURE STORAGE ACCOUNT NAME>"
    AccountKey = "<AZURE STORAGE ACCOUNT KEY>"
    ContainerName = "runners-cache"
    StorageDomain = "blob.core.windows.net"
```

#### AzureワークロードIDとマネージドID {#azure-workload-and-managed-identities}

{{< history >}}

- GitLab Runner v17.5.0で[導入](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27303)されました。

{{< /history >}}

AzureワークロードまたはマネージドIDを使用するには、設定から`AccountKey`を省略します。`AccountKey`が空白の場合、Runnerは次の処理を試みます。

1. [`DefaultAzureCredential`を使用](https://github.com/Azure/azure-sdk-for-go/blob/main/sdk/azidentity/README.md#defaultazurecredential)して一時的な認証情報を取得します。
1. [ユーザー委任キー](https://learn.microsoft.com/en-us/rest/api/storageservices/get-user-delegation-key)を取得します。
1. そのキーを使用して、ストレージアカウントのblobにアクセスするためのSASトークンを生成します。

インスタンスに`Storage Blob Data Contributor`ロールが割り当てられていることを確認します。上記のアクションを実行するためのアクセス権がインスタンスにない場合、GitLab Runnerは`AuthorizationPermissionMismatch`エラーを報告します。

AzureワークロードIDを使用するには、IDに関連付けられている`service_account`を追加し、ポッドラベル`azure.workload.identity/use`を`runner.kubernetes`セクションに追加します。たとえば、`service_account`が`gitlab-runner`の場合は次のようになります。

```toml
  [runners.kubernetes]
    service_account = "gitlab-runner"
    [runners.kubernetes.pod_labels]
      "azure.workload.identity/use" = "true"
```

`service_account`に、`azure.workload.identity/client-id`アノテーションが関連付けられていることを確認します。

```yaml
serviceAccount:
  annotations:
    azure.workload.identity/client-id: <YOUR CLIENT ID HERE>
```

GitLab 17.7以降では、ワークロードIDのセットアップにはこの設定で十分です。

ただし、GitLab Runner 17.5および17.6では、Runnerマネージャーにも以下の設定が必要です。

- `azure.workload.identity/use`ポッドラベル
- ワークロードIDで使用するサービスアカウント

たとえば、GitLab Runner Helmチャートを使用する場合は次のようになります。

```yaml
serviceAccount:
  name: "gitlab-runner"
podLabels:
  azure.workload.identity/use: "true"
```

認証情報は異なるソースから取得されるため、このラベルが必要です。キャッシュのダウンロードの場合、認証情報はRunnerマネージャーから取得されます。キャッシュのアップロードの場合、認証情報は[ヘルパーイメージ](#helper-image)を実行するポッドから取得されます。

詳細については、[イシュー38330](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/38330)を参照してください。

## `[runners.kubernetes]`セクション {#the-runnerskubernetes-section}

次の表に、Kubernetes executorで使用できる設定パラメータを示します。その他のパラメータについては、[Kubernetes executorのドキュメント](../executors/kubernetes/_index.md)を参照してください。

| パラメータ                    | 型    | 説明 |
|------------------------------|---------|-------------|
| `host`                       | 文字列  | オプション。KubernetesホストのURL。指定されていない場合、Runnerは自動検出を試みます。 |
| `cert_file`                  | 文字列  | オプション。Kubernetes認証証明書。 |
| `key_file`                   | 文字列  | オプション。Kubernetes認証秘密キー。 |
| `ca_file`                    | 文字列  | オプション。Kubernetes認証CA証明書。 |
| `image`                      | 文字列  | ジョブでコンテナイメージが指定されていない場合に使用するデフォルトのコンテナイメージ。 |
| `allowed_images`             | 配列   | `.gitlab-ci.yml`で許可されるコンテナイメージのワイルドカードリスト。この設定が存在しない場合は、すべてのイメージが許可されます（`["*/*:*"]`と同等）。[Docker](../executors/docker.md#restrict-docker-images-and-services) executorまたは[Kubernetes](../executors/kubernetes/_index.md#restrict-docker-images-and-services) executorで使用します。 |
| `allowed_services`           | 配列   | `.gitlab-ci.yml`で許可されるサービスのワイルドカードリスト。この設定が存在しない場合は、すべてのイメージが許可されます（`["*/*:*"]`と同等）。[Docker](../executors/docker.md#restrict-docker-images-and-services) executorまたは[Kubernetes](../executors/kubernetes/_index.md#restrict-docker-images-and-services) executorで使用します。 |
| `namespace`                  | 文字列  | Kubernetesジョブを実行するネームスペース。 |
| `privileged`                 | ブール値 | 特権フラグを有効にしてすべてのコンテナを実行します。 |
| `allow_privilege_escalation` | ブール値 | オプション。`allowPrivilegeEscalation`フラグを有効にしてすべてのコンテナを実行します。 |
| `node_selector`              | テーブル   | `string=string`の`key=value`ペアの`table`。ポッドの作成が、すべての`key=value`ペアに一致するKubernetesノードに制限されます。 |
| `image_pull_secrets`         | 配列   | プライベートレジストリからのコンテナイメージのプル認証に使用されるKubernetesの`docker-registry`シークレット名を含む項目の配列。 |
| `logs_base_dir`              | 文字列  | ビルドログを保存するために生成されたパスの前に付加されるベースディレクトリ。GitLab Runner 17.2で[導入](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/37760)されました。 |
| `scripts_base_dir`           | 文字列  | ビルドスクリプトを保存するために生成されたパスの前に付加されるベースディレクトリ。GitLab Runner 17.2で[導入](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/37760)されました。 |
| `service_account`            | 文字列  | ジョブ/executorポッドがKubernetes APIと通信するために使用するデフォルトのサービスアカウント。 |

例: 

```toml
[runners.kubernetes]
  host = "https://45.67.34.123:4892"
  cert_file = "/etc/ssl/kubernetes/api.crt"
  key_file = "/etc/ssl/kubernetes/api.key"
  ca_file = "/etc/ssl/kubernetes/ca.crt"
  image = "golang:1.8"
  privileged = true
  allow_privilege_escalation = true
  image_pull_secrets = ["docker-registry-credentials", "optional-additional-credentials"]
  allowed_images = ["ruby:*", "python:*", "php:*"]
  allowed_services = ["postgres:9.4", "postgres:latest"]
  logs_base_dir = "/tmp"
  scripts_base_dir = "/tmp"
  [runners.kubernetes.node_selector]
    gitlab = "true"
```

## ヘルパーイメージ {#helper-image}

`docker`、`docker+machine`、または`kubernetes` executorを使用すると、GitLab RunnerはGit、アーティファクト、およびキャッシュ操作の処理に特定のコンテナを使用します。このコンテナは、`helper image`という名前のイメージから作成されます。

ヘルパーイメージは、amd64、ARM、arm64、s390x、ppc64le、およびriscv64アーキテクチャで使用できます。これには、GitLab Runnerバイナリの特別なコンパイルである`gitlab-runner-helper`バイナリが含まれています。これには、利用可能なコマンドのサブセットと、Git、Git LFS、およびSSL証明書ストアのみが含まれています。

ヘルパーイメージには、`alpine`、`alpine3.21`、`alpine-latest`、`ubi-fips`、`ubuntu`のようないくつかの種類があります。`alpine`イメージはフットプリントが小さいため、デフォルトです。`helper_image_flavor = "ubuntu"`を使用すると、ヘルパーイメージの`ubuntu`フレーバーが選択されます。

GitLab Runner 16.1から17.1では、`alpine`フレーバーは`alpine3.18`のエイリアスです。GitLab Runner 17.2から17.6では、`alpine3.19`のエイリアスです。GitLab Runner 17.7以降では、`alpine3.21`のエイリアスとなっています。GitLab Runner 18.4以降では、`alpine-latest`のエイリアスです。

`alpine-latest`フレーバーは、`alpine:latest`をベースイメージとして使用し、新しいアップストリームのバージョンがリリースされると、自動的にバージョンが上がります。

GitLab Runnerが`DEB`パッケージまたは`RPM`パッケージからインストールされると、サポートされているアーキテクチャ用のイメージがホストにインストールされます。Docker Engineが指定されたイメージバージョンを見つけられない場合、Runnerはジョブを実行する前に自動的にダウンロードします。`docker` executorと`docker+machine` executorの両方がこのように動作します。

`alpine`フレーバーの場合、デフォルトの`alpine`フレーバーイメージのみがパッケージに含まれています。その他すべてのフレーバーは、レジストリからダウンロードされます。

GitLab Runnerの手動インストールと`kubernetes` executorは異なる動作をします。

- 手動インストールの場合は、`gitlab-runner-helper`バイナリは含まれていません。
- `kubernetes` executorの場合、Kubernetes APIは`gitlab-runner-helper`イメージをローカルアーカイブから読み込むことを許可しません。

いずれの場合も、GitLab Runnerは[ヘルパーイメージをダウンロード](#helper-image-registry)します。GitLab Runnerのリビジョンとアーキテクチャによって、ダウンロードするタグが決まります。

### Arm上のKubernetes用ヘルパーイメージ設定 {#helper-image-configuration-for-kubernetes-on-arm}

既定では、アーキテクチャに適した[ヘルパーイメージ](../executors/kubernetes/_index.md#operating-system-architecture-and-windows-kernel-version)が選択されます。`arm64` Kubernetesクラスターで`arm64`ヘルパーイメージを使用するためにカスタム`helper_image`パスを設定する必要がある場合は、[設定ファイル](../executors/kubernetes/_index.md#configuration-settings)で次の値を設定します:

```toml
[runners.kubernetes]
  helper_image = "my.registry.local/gitlab/gitlab-runner-helper:arm64-v${CI_RUNNER_VERSION}"
```

### 古いバージョンのAlpine Linuxを使用するRunnerイメージ {#runner-images-that-use-an-old-version-of-alpine-linux}

{{< history >}}

- GitLab Runner 14.5で[導入](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/3122)されました。

{{< /history >}}

イメージは、複数のAlpine Linuxバージョンでビルドされています。新しいバージョンのAlpineを使用できますが、同時に古いバージョンも使用できます。

ヘルパーイメージの場合は、`helper_image_flavor`を変更するか、[ヘルパーイメージ](#helper-image)セクションを参照してください。

GitLab Runnerイメージの場合は、`alpine`、`alpine3.19`、`alpine3.21`、または`alpine-latest`がバージョンの前にイメージのプレフィックスとして使用されるように、同じロジックに従ってください:

```shell
docker pull gitlab/gitlab-runner:alpine3.19-v16.1.0
```

### Alpine `pwsh`イメージ {#alpine-pwsh-images}

GitLab Runner 16.1以降、すべての`alpine`ヘルパーイメージには`pwsh`バリアントがあります。唯一の例外は`alpine-latest`です。これは、GitLab Runnerヘルパーイメージのベースとなる[`powershell` Dockerイメージ](https://learn.microsoft.com/en-us/powershell/scripting/install/powershell-in-docker?view=powershell-7.4)が`alpine:latest`をサポートしていないためです。

例: 

```shell
docker pull registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:alpine3.21-x86_64-v17.7.0-pwsh
```

### ヘルパーイメージレジストリ {#helper-image-registry}

GitLab 15.0以前では、Docker Hubのイメージを使用するようにヘルパーイメージを設定します。

GitLab 15.1以降では、ヘルパーイメージは、GitLab.com上のGitLab Containerレジストリから`registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:x86_64-v${CI_RUNNER_VERSION}`でプルされます。GitLab Self-Managedインスタンスも、既定でGitLab.com上のGitLab Containerレジストリからヘルパーイメージをプルします。GitLab.com上のGitLab Containerレジストリのステータスを確認するには、[GitLabシステムのステータス](https://status.gitlab.com/)を参照してください。

### ヘルパーイメージを上書きする {#override-the-helper-image}

場合によっては、次の理由でヘルパーイメージを上書きする必要があります。

1. **ジョブ実行の高速化**: インターネット接続の速度が遅い環境では、同じイメージを複数回ダウンロードすると、ジョブの実行に時間がかかる可能性があります。`registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:XYZ`の正確なコピーが保存されているローカルレジストリからヘルパーイメージをダウンロードすることで、処理を高速化できます。

1. **セキュリティに関する懸念**: 事前にチェックされていない外部依存関係をダウンロードしたくない場合があります。レビューが完了し、ローカルリポジトリに保存されている依存関係のみを使用するというビジネスルールが存在する可能性があります。

1. **インターネットにアクセスできないビルド環境**: [オフライン環境にKubernetesクラスターをインストールしている](../install/operator.md#install-gitlab-runner-operator-on-kubernetes-clusters-in-offline-environments)場合は、ローカルイメージレジストリまたはパッケージリポジトリを使用して、CI/CDジョブで使用されるイメージをプルできます。

1. **追加のソフトウェア**: `git+http`の代わりに`git+ssh`を使用してアクセス可能なサブモジュールをサポートするために、`openssh`のような追加のソフトウェアをヘルパーイメージにインストールしたい場合があります。

このような場合は、`docker`、`docker+machine`、および`kubernetes` executorで利用可能な`helper_image`設定フィールドを使用して、カスタムイメージを設定できます。

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    helper_image = "my.registry.local/gitlab/gitlab-runner-helper:tag"
```

ヘルパーイメージのバージョンは、GitLab Runnerのバージョンと緊密に結合されていると考えてください。これらのイメージを提供する主な理由の1つは、GitLab Runnerが`gitlab-runner-helper`バイナリを使用していることです。このバイナリは、GitLab Runnerソースの一部からコンパイルされます。このバイナリは、両方のバイナリで同じであることが期待される内部APIを使用しています。

デフォルトでは、GitLab Runnerは`registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:XYZ`イメージを参照します。ここで、`XYZ`はGitLab RunnerのアーキテクチャとGitリビジョンに基づいています。[バージョン変数](https://gitlab.com/gitlab-org/gitlab-runner/blob/main/common/version.go#L60-61)のいずれかを使用することによって、イメージバージョンを定義することができます。

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    helper_image = "my.registry.local/gitlab/gitlab-runner-helper:x86_64-v${CI_RUNNER_VERSION}"
```

この設定により、GitLab Runnerはexecutorに対し、コンパイルデータに基づくバージョン`x86_64-v${CI_RUNNER_VERSION}`のイメージを使用するように指示します。GitLab Runnerが新しいバージョンに更新された後で、GitLab Runnerは適切なイメージをダウンロードしようとします。GitLab Runnerをアップグレードする前に、イメージをレジストリにアップロードする必要があります。そうしないと、ジョブが「No such image」（指定されたイメージが見つかりません）エラーで失敗し始めます。

ヘルパーイメージは、`$CI_RUNNER_REVISION`に加えて`$CI_RUNNER_VERSION`によってタグ付けされます。どちらのタグも有効であり、同じイメージを指しています。

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    helper_image = "my.registry.local/gitlab/gitlab-runner-helper:x86_64-v${CI_RUNNER_VERSION}"
```

#### PowerShell Coreを使用する場合 {#when-using-powershell-core}

PowerShell Coreを含むLinux用のヘルパーイメージの追加バージョンは、`registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:XYZ-pwsh`タグを使用して公開されます。

## `[runners.custom_build_dir]`セクション {#the-runnerscustom_build_dir-section}

{{< history >}}

- GitLab Runner 11.10で[導入](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/1267)されました。

{{< /history >}}

このセクションでは、[カスタムビルドディレクトリ](https://docs.gitlab.com/ci/runners/configure_runners/#custom-build-directories)パラメータを定義します。

この機能は、明示的に設定されていない場合でも、`kubernetes`、`docker`、`docker+machine`、`docker autoscaler`、および`instance` executorで、デフォルトで有効になっています。他のすべてのexecutorでは、デフォルトで無効になっています。

この機能を使用するには、`runners.builds_dir`で定義されたパスに`GIT_CLONE_PATH`が含まれている必要があります。`builds_dir`を使用するには、`$CI_BUILDS_DIR`変数を使用します。

デフォルトでは、この機能は`docker` executorと`kubernetes` executorでのみ有効になっています。これは、これらのexecutorがリソースを分離するのに適した方法を提供するためです。この機能はどのexecutorでも明示的に有効にできますが、`builds_dir`を共有し、`concurrent > 1`が設定されたexecutorで使用する場合は注意が必要です。

| パラメータ | 型    | 説明 |
|-----------|---------|-------------|
| `enabled` | ブール値 | ユーザーがジョブのカスタムビルドディレクトリを定義できるようにします。 |

例: 

```toml
[runners.custom_build_dir]
  enabled = true
```

### デフォルトのビルドディレクトリ {#default-build-directory}

GitLab Runnerは、_ビルドディレクトリ_と呼ばれるベースパスの下に存在するパスにリポジトリをクローンします。このベースディレクトリのデフォルトの場所は、executorによって異なります。詳細は以下の説明を参照してください。

- [Kubernetes](../executors/kubernetes/_index.md)、[Docker](../executors/docker.md)、[Docker Machine](../executors/docker_machine.md) executorの場合は、コンテナ内の`/builds`です。
- [Instance](../executors/instance.md)の場合は、ターゲットマシンへのSSH接続またはWinRM接続を処理するように設定されているユーザーのホームディレクトリにある`~/builds`です。
- [Docker Autoscaler](../executors/docker_autoscaler.md)の場合は、コンテナ内の`/builds`です。
- [Shell](../executors/shell.md) executorの場合は、`$PWD/builds`です。
- [SSH](../executors/ssh.md)、[VirtualBox](../executors/virtualbox.md)、[Parallels](../executors/parallels.md) executorの場合は、ターゲットマシンへのSSH接続を処理するように設定されているユーザーのホームディレクトリにある`~/builds`です。
- [Custom](../executors/custom.md) executorの場合はデフォルトが提供されていないため、明示的に設定する必要があります。設定されていない場合、ジョブが失敗します。

使用される_ビルドディレクトリ_は、ユーザーが[`builds_dir`](#the-runners-section)設定で明示的に定義できます。

{{< alert type="note" >}}

カスタムディレクトリにクローンする場合は、[`GIT_CLONE_PATH`](https://docs.gitlab.com/ci/runners/configure_runners/#custom-build-directories)を指定することもできます。その場合は以下のガイドラインは適用されません。

{{< /alert >}}

GitLab Runnerは、実行するすべてのジョブに_ビルドディレクトリ_を使用しますが、特定のパターン`{builds_dir}/$RUNNER_TOKEN_KEY/$CONCURRENT_PROJECT_ID/$NAMESPACE/$PROJECT_NAME`を使用してそれらをネストします。例: `/builds/2mn-ncv-/0/user/playground`。

GitLab Runnerは、ユーザーが_ビルドディレクトリ_に保存することを妨げません。たとえば、CI実行中に使用できるツールを`/builds/tools`内に保存できます。この操作は**極力**控えてください。_ビルドディレクトリ_には何も保存しないでください。GitLab Runnerはこの動作を完全に制御する必要があり、そのような場合には安定性が保証されません。CIに必要な依存関係がある場合は、他の場所にインストールする必要があります。

## Git設定をクリーンアップする {#cleaning-git-configuration}

{{< history >}}

- GitLab Runner 17.10で[導入](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5438)されました。

{{< /history >}}

すべてのビルドの開始時と終了時に、GitLab Runnerはリポジトリとそのサブモジュールから次のファイルを削除します。

- Gitロックファイル（`{index,shallow,HEAD,config}.lock`）
- post-checkoutフック（`hooks/post-checkout`）

`clean_git_config`を有効にすると、リポジトリ、そのサブモジュール、およびGitテンプレートディレクトリから、次の追加ファイルまたはディレクトリが削除されます。

- `.git/config`ファイル
- `.git/hooks`ディレクトリ

このクリーンアップにより、カスタムGit設定、一時的なGit設定、または潜在的に悪意のあるGit設定がジョブ間でキャッシュされることを防ぎます。

GitLab Runner 17.10より前では、クリーンアップの動作が異なっていました。

- Gitロックファイルとpost-checkoutフックのクリーンアップは、ジョブの開始時にのみ行われ、終了時には行われませんでした。
- 他のGit設定（現在は`clean_git_config`で制御されるようになった設定）は、`FF_ENABLE_JOB_CLEANUP`が設定されていない場合には削除されませんでした。このフラグを設定すると、メインリポジトリの`.git/config`のみが削除されますが、サブモジュールの設定は削除されませんでした。

`clean_git_config`設定はデフォルトで`true`です。ただし、次の場合はデフォルトで`false`です。

- [Shell executor](../executors/shell.md)が使用されている。
- [Git戦略](https://docs.gitlab.com/ci/runners/configure_runners/#git-strategy)が`none`に設定されている。

明示的な`clean_git_config`設定は、デフォルト設定よりも優先されます。

## `[runners.referees]`セクション {#the-runnersreferees-section}

GitLab Runnerレフェリーを使用して、追加のジョブモニタリングデータをGitLabに渡します。レフェリーは、ジョブに関連する追加データの照会と収集を行うRunnerマネージャーのワーカーです。結果は、ジョブアーティファクトとしてGitLabにアップロードされます。

### Metrics Runnerレフェリーを使用する {#use-the-metrics-runner-referee}

ジョブを実行しているマシンまたはコンテナが[Prometheus](https://prometheus.io)メトリクスを公開している場合、GitLab Runnerはジョブ期間全体にわたってPrometheusサーバーに照会できます。受信したメトリクスはジョブアーティファクトとしてアップロードされ、後で分析に使用できます。

[`docker-machine` executor](../executors/docker_machine.md)のみがレフェリーをサポートしています。

### GitLab Runner用のMetrics Runnerレフェリーを設定する {#configure-the-metrics-runner-referee-for-gitlab-runner}

`config.toml`ファイルの`[[runner]]`セクションで`[runner.referees]`と`[runner.referees.metrics]`を定義し、次のフィールドを追加します。

| 設定              | 説明 |
|----------------------|-------------|
| `prometheus_address` | GitLab Runnerインスタンスからメトリクスを収集するサーバー。ジョブの完了時にRunnerマネージャーからアクセスできる必要があります。 |
| `query_interval`     | ジョブに関連付けられているPrometheusインスタンスに対し、時系列データが照会を受ける頻度。間隔（秒単位）として定義されます。 |
| `queries`            | 各間隔で実行される[PromQL](https://prometheus.io/docs/prometheus/latest/querying/basics/)クエリの配列。 |

`node_exporter`メトリクスの構成を網羅した設定例を次に示します。

```toml
[[runners]]
  [runners.referees]
    [runners.referees.metrics]
      prometheus_address = "http://localhost:9090"
      query_interval = 10
      metric_queries = [
        "arp_entries:rate(node_arp_entries{{selector}}[{interval}])",
        "context_switches:rate(node_context_switches_total{{selector}}[{interval}])",
        "cpu_seconds:rate(node_cpu_seconds_total{{selector}}[{interval}])",
        "disk_read_bytes:rate(node_disk_read_bytes_total{{selector}}[{interval}])",
        "disk_written_bytes:rate(node_disk_written_bytes_total{{selector}}[{interval}])",
        "memory_bytes:rate(node_memory_MemTotal_bytes{{selector}}[{interval}])",
        "memory_swap_bytes:rate(node_memory_SwapTotal_bytes{{selector}}[{interval}])",
        "network_tcp_active_opens:rate(node_netstat_Tcp_ActiveOpens{{selector}}[{interval}])",
        "network_tcp_passive_opens:rate(node_netstat_Tcp_PassiveOpens{{selector}}[{interval}])",
        "network_receive_bytes:rate(node_network_receive_bytes_total{{selector}}[{interval}])",
        "network_receive_drops:rate(node_network_receive_drop_total{{selector}}[{interval}])",
        "network_receive_errors:rate(node_network_receive_errs_total{{selector}}[{interval}])",
        "network_receive_packets:rate(node_network_receive_packets_total{{selector}}[{interval}])",
        "network_transmit_bytes:rate(node_network_transmit_bytes_total{{selector}}[{interval}])",
        "network_transmit_drops:rate(node_network_transmit_drop_total{{selector}}[{interval}])",
        "network_transmit_errors:rate(node_network_transmit_errs_total{{selector}}[{interval}])",
        "network_transmit_packets:rate(node_network_transmit_packets_total{{selector}}[{interval}])"
      ]
```

メトリクスクエリの形式は`canonical_name:query_string`です。クエリ文字列は、実行中に置き換えられる2つの変数をサポートしています。

| 設定      | 説明 |
|--------------|-------------|
| `{selector}` | 特定のGitLab RunnerインスタンスによってPrometheusで生成されたメトリクスを選択する`label_name=label_value`ペアに置き換えられます。 |
| `{interval}` | このレフェリーの`[runners.referees.metrics]`設定の`query_interval`パラメータに置き換えられます。 |

たとえば、`docker-machine` executorを使用する共有GitLab Runner環境では、`{selector}`が`node=shared-runner-123`のようになります。
