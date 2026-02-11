---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: GitLab Runnerの機能フラグ
---

{{< alert type="warning" >}}

デフォルトで無効になっている機能を有効にすると、データの破損、安定性の低下、パフォーマンスの低下、およびセキュリティの問題が発生する可能性があります。機能フラグを有効にする前に、有効化に伴うリスクを認識しておく必要があります。詳細については、[開発中の機能を有効にする際のリスク](https://docs.gitlab.com/administration/feature_flags/#risks-when-enabling-features-still-in-development)を参照してください。

{{< /alert >}}

機能フラグは、特定の機能を有効または無効を切り替えることができる仕組みです。機能フラグは通常、次の機能に対して使用されます。

- ボランティアがテストできるベータ機能のうち、すべてのユーザーに対して有効にできる状態ではない機能。

  ベータ機能は、不完全であるか、さらにテストが必要な場合があります。ベータ機能の使用を希望するユーザーは、リスクを受け入れて、機能フラグで機能を明示的に有効にすることを選択できます。機能はデフォルトで無効になっているため、機能を必要としないユーザー、またはシステムのリスクを受け入れたくないユーザーはバグやリグレッションの影響を受けません。

- 近い将来に機能の非推奨化または機能の削除につながる破壊的な変更。

  製品の進化に伴い、機能が変更されたり、完全に削除されたりします。多くの場合既知のバグは修正されますが、ユーザーに対して影響しているバグに対する回避策がすでに判明していることがあります。ユーザーに標準化されたバグ修正を採用することを強制すると、カスタマイズされた設定で他の問題が発生する可能性があります。

  そのような場合、機能フラグを使用して、オンデマンドで古い動作から新しい動作に切り替えることができます。これにより、ユーザーは製品の新しいバージョンを採用し、古い動作から新しい動作へのスムーズで永続的な移行を計画するための時間を確保できます。

機能フラグは、環境変数を使用して切り替えます。次のように設定します。

- 機能フラグを有効にするには、対応する環境変数を`"true"`または`1`に設定します。
- 機能フラグを無効にするには、対応する環境変数を`"false"`または`0`に設定します。

## 利用可能な機能フラグ {#available-feature-flags}

<!--
The list of feature flags is created automatically.
If you need to update it, call `make update_feature_flags_docs` in the
root directory of this project.
The flags are defined in `./helpers/featureflags/flags.go` file.
-->

<!-- feature_flags_list_start -->

| 機能フラグ | デフォルト値 | 非推奨 | 削除予定 | 説明 |
|--------------|---------------|------------|--------------------|-------------|
| `FF_NETWORK_PER_BUILD` | `false` | {{< icon name="dotted-circle" >}} いいえ |  | `docker` executorを使用したDockerの[ビルドごとのネットワーク](../executors/docker.md#network-configurations)の作成を有効にします。 |
| `FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY` | `false` | {{< icon name="dotted-circle" >}} いいえ |  | `false`に設定すると、[#4119](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4119)などのイシューを解決するために、`exec`によるリモートKubernetesコマンドの実行を無効にし、代わりに`attach`を使用します。 |
| `FF_USE_DIRECT_DOWNLOAD` | `true` | {{< icon name="dotted-circle" >}} いいえ |  | `true`に設定すると、Runnerは最初にGitLabを介してプロキシする代わりに、すべてアーティファクトを直接ダウンロードしようとします。有効にすると、GitLabでオブジェクトストレージが有効になっている場合に、オブジェクトストレージのTLS証明書の検証で発生する問題が原因で、ダウンロードが失敗する可能性があります。[自己署名証明書またはカスタム認証局](tls-self-signed.md)を参照してください |
| `FF_SKIP_NOOP_BUILD_STAGES` | `true` | {{< icon name="dotted-circle" >}} いいえ |  | `false`に設定すると、実行しても効果がない場合でも、すべてのビルドステージが実行されます |
| `FF_USE_FASTZIP` | `false` | {{< icon name="dotted-circle" >}} いいえ |  | Fastzipは、キャッシュ/アーティファクトのアーカイブと解凍を行うための高性能アーカイバーです |
| `FF_DISABLE_UMASK_FOR_DOCKER_EXECUTOR` | `false` | {{< icon name="dotted-circle" >}} いいえ |  | 有効にすると、`docker` executorで実行されるジョブに対する`umask 0000`呼び出しの使用が削除されます。代わりに、Runnerはビルドコンテナで使用されるイメージに対して設定されたユーザーのUIDとGIDの検出を試み、（ソースの更新、キャッシュの復元、およびアーティファクトのダウンロード後に）定義済みのコンテナで`chmod`コマンドを実行して、作業ディレクトリとファイルの所有権を変更します。この機能フラグを使用するには、POSIXユーティリティ`id`がビルドイメージにインストールされ、動作可能である必要があります。RunnerはUIDとGIDを取得するために、オプション`-u`と`-g`を指定して`id`を実行します。 |
| `FF_ENABLE_BASH_EXIT_CODE_CHECK` | `false` | {{< icon name="dotted-circle" >}} いいえ |  | 有効にすると、bashスクリプトは`set -e`のみに依存しませんが、各スクリプトコマンドの実行後にゼロ以外の終了コードを確認します。 |
| `FF_USE_WINDOWS_LEGACY_PROCESS_STRATEGY` | `false` | {{< icon name="dotted-circle" >}} いいえ |  | GitLab Runner 16.10以降では、デフォルトは`false`です。GitLab Runner 16.9以前では、デフォルトは`true`です。無効にすると、WindowsでRunnerが作成するプロセス（Shell executorとカスタムexecutor）が、追加のセットアップを使用して作成され、これによりプロセスの終了が改善されます。`true`に設定すると、従来のプロセスセットアップが使用されます。Windows Runnerを正常にドレインするには、この機能フラグを`false`に設定する必要があります。 |
| `FF_USE_NEW_BASH_EVAL_STRATEGY` | `false` | {{< icon name="dotted-circle" >}} いいえ |  | `true`に設定すると、実行されたスクリプトの終了コードを適切に検出できるように、Bash `eval`呼び出しがサブShellで実行されます。 |
| `FF_USE_POWERSHELL_PATH_RESOLVER` | `false` | {{< icon name="dotted-circle" >}} いいえ |  | 有効にすると、RunnerではなくPowerShellが、Runnerがホストされている場所に固有のOS特有のファイルパス関数を使用して、パス名を解決します。 |
| `FF_USE_DYNAMIC_TRACE_FORCE_SEND_INTERVAL` | `false` | {{< icon name="dotted-circle" >}} いいえ |  | 有効にすると、ログのトレース強制送信間隔は、トレース更新間隔に基づいて動的に調整されます。 |
| `FF_SCRIPT_SECTIONS` | `false` | {{< icon name="dotted-circle" >}} いいえ |  | 有効にすると、`.gitlab-ci.yml`ファイルの各スクリプト行がジョブ出力で折りたたみ可能なセクションにまとめられ、各行の期間が表示されます。コマンドが複数行にわたる場合、完全なコマンドがジョブログ出力ターミナルに表示されます。 |
| `FF_ENABLE_JOB_CLEANUP` | `false` | {{< icon name="dotted-circle" >}} いいえ |  | 有効にすると、プロジェクトディレクトリがビルドの最後にクリーンアップされます。`GIT_CLONE`を使用すると、プロジェクトディレクトリ全体が削除されます。`GIT_FETCH`を使用すると、一連のGit `clean`コマンドが発行されます。 |
| `FF_KUBERNETES_HONOR_ENTRYPOINT` | `false` | {{< icon name="dotted-circle" >}} いいえ |  | 有効にすると、`FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY`がtrueに設定されていない場合、イメージのDockerエントリポイントが実行されます。 |
| `FF_POSIXLY_CORRECT_ESCAPES` | `false` | {{< icon name="dotted-circle" >}} いいえ |  | 有効にすると、[`bash`スタイルのANSI-Cの引用符の使い方](https://www.gnu.org/software/bash/manual/html_node/Quoting.html)ではなく[POSIX Shellエスケープ](https://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html#tag_18_02)が使用されます。ジョブ環境がPOSIX準拠のShellを使用している場合は、これを有効にする必要があります。 |
| `FF_RESOLVE_FULL_TLS_CHAIN` | `false` | {{< icon name="dotted-circle" >}} いいえ |  | GitLab Runner 16.4以降では、デフォルトは`false`です。GitLab Runner 16.3以前では、デフォルトは`true`です。有効にすると、Runnerは`CI_SERVER_TLS_CA_FILE`の自己署名ルート証明書までのTLSチェーン全体を解決します。これは以前、v7.68.0以前のlibcurlとOpenSSLを使用してビルドされたGitクライアントで[Git HTTPSクローンを機能させる](tls-self-signed.md#git-cloning)ために必要でした。ただし、古い署名アルゴリズムで署名されたルート証明書を拒否するmacOSなどの一部のオペレーティングシステムでは、証明書解決のプロセスが失敗する可能性があります。証明書の解決が失敗する場合は、この機能を無効にする必要があることがあります。この機能フラグは、[`[runners.feature_flags]`設定](#enable-feature-flag-in-runner-configuration)でのみ無効にできます。 |
| `FF_DISABLE_POWERSHELL_STDIN` | `false` | {{< icon name="dotted-circle" >}} いいえ |  | 有効にすると、Shell executorとカスタムxecutorのPowerShellスクリプトは、stdinを介して渡されて実行されるのではなく、ファイルによって渡されます。これは、ジョブの`allow_failure:exit_codes`キーワードが正しく機能するために必要です。 |
| `FF_USE_POD_ACTIVE_DEADLINE_SECONDS` | `true` | {{< icon name="dotted-circle" >}} いいえ |  | 有効にすると、[ポッドの`activeDeadlineSeconds`](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#lifecycle)がCI/CDジョブタイムアウトに設定されます。このフラグは、[ポッドのライフサイクル](../executors/kubernetes/_index.md#pod-lifecycle)に影響します。 |
| `FF_USE_ADVANCED_POD_SPEC_CONFIGURATION` | `false` | {{< icon name="dotted-circle" >}} いいえ |  | 有効にすると、ユーザーは`config.toml`ファイルでポッド仕様全体を設定できます。詳細については、[生成されたポッド仕様を上書きする（実験的機能）](../executors/kubernetes/_index.md#overwrite-generated-pod-specifications)を参照してください。 |
| `FF_SET_PERMISSIONS_BEFORE_CLEANUP` | `true` | {{< icon name="dotted-circle" >}} いいえ |  | 有効にすると、クリーンアップ中の削除が確実に成功するように、最初にプロジェクトディレクトリ内のディレクトリとファイルに対する権限が設定されます。 |
| `FF_SECRET_RESOLVING_FAILS_IF_MISSING` | `true` | {{< icon name="dotted-circle" >}} いいえ |  | 有効にすると、値が見つからない場合にシークレットの解決が失敗します。 |
| `FF_PRINT_POD_EVENTS` | `false` | {{< icon name="dotted-circle" >}} いいえ |  | 有効にすると、ビルドポッドが開始するまで、ビルドポッドに関連付けられているすべてのイベントが出力されます。 |
| `FF_USE_GIT_BUNDLE_URIS` | `true` | {{< icon name="dotted-circle" >}} いいえ |  | 有効にすると、Git `transfer.bundleURI`設定オプションが`true`に設定されます。このFFはデフォルトで有効になっています。Gitバンドルのサポートを無効にするには、`false`に設定します。 |
| `FF_USE_GIT_NATIVE_CLONE` | `false` | {{< icon name="dotted-circle" >}} いいえ |  | これが有効になっていて、かつ`GIT_STRATEGY=clone`の場合、プロジェクトのクローンを作成するには、`git-init(1)` + `git-fetch(1)`ではなく`git-clone(1)`コマンドを使用します。これにはGitバージョン2.49以降が必要であり、それが利用できない場合は`init` + `fetch`にフォールバックします。 |
| `FF_USE_DUMB_INIT_WITH_KUBERNETES_EXECUTOR` | `false` | {{< icon name="dotted-circle" >}} いいえ |  | 有効にすると、すべてのスクリプトの実行に`dumb-init`が使用されます。これにより、`dumb-init`をヘルパーコンテナとビルドコンテナの最初のプロセスとして実行できるようになります。 |
| `FF_USE_INIT_WITH_DOCKER_EXECUTOR` | `false` | {{< icon name="dotted-circle" >}} いいえ |  | 有効にすると、Docker executorは`--init`オプション（`tini-init`をPID 1として実行）を使用して、サービスコンテナとビルドコンテナを起動します。 |
| `FF_LOG_IMAGES_CONFIGURED_FOR_JOB` | `false` | {{< icon name="dotted-circle" >}} いいえ |  | 有効にすると、Runnerは受信した各ジョブに定義されているイメージとサービスイメージの名前をログに記録します。 |
| `FF_USE_DOCKER_AUTOSCALER_DIAL_STDIO` | `true` | {{< icon name="dotted-circle" >}} いいえ |  | 有効にすると（デフォルト）、リモートDockerデーモンへのトンネル接続に`docker system stdio`が使用されます。無効にすると、SSH接続ではネイティブSSHトンネルが使用され、WinRM接続では最初に「fleeting-proxy」ヘルパーバイナリがデプロイされます。 |
| `FF_CLEAN_UP_FAILED_CACHE_EXTRACT` | `false` | {{< icon name="dotted-circle" >}} いいえ |  | 有効にすると、キャッシュ抽出の失敗を検出し、残された部分的なキャッシュコンテンツをクリーンアップするためのコマンドがビルドスクリプトに挿入されます。 |
| `FF_USE_WINDOWS_JOB_OBJECT` | `false` | {{< icon name="dotted-circle" >}} いいえ |  | 有効にすると、RunnerがShell executorとカスタムexecutorを使用してWindows上に作成するプロセスごとに、ジョブオブジェクトが作成されます。プロセスを強制終了するために、Runnerはジョブオブジェクトを閉じます。これにより、強制終了が困難なプロセスの終了が改善されます。 |
| `FF_TIMESTAMPS` | `true` | {{< icon name="dotted-circle" >}} いいえ |  | 無効にすると、各ログトレース行の先頭にタイムスタンプは追加されません。 |
| `FF_DISABLE_AUTOMATIC_TOKEN_ROTATION` | `false` | {{< icon name="dotted-circle" >}} いいえ |  | 有効にすると、自動トークンローテーションが制限され、トークンの有効期限が近づくと警告がログに記録されます。 |
| `FF_USE_LEGACY_GCS_CACHE_ADAPTER` | `false` | {{< icon name="dotted-circle" >}} いいえ |  | 有効にすると、従来のGCSキャッシュアダプターが使用されます。無効にすると（デフォルト）、認証にGoogle Cloud StorageのSDKを使用する新しいGCSキャッシュアダプターが使用されます。これにより、GKEのワークロードID設定など、従来のアダプターでは解決が困難だった環境での認証の問題が解決されます。 |
| `FF_DISABLE_UMASK_FOR_KUBERNETES_EXECUTOR` | `false` | {{< icon name="dotted-circle" >}} いいえ |  | 有効にすると、Kubernetes executorで実行されるジョブに対する`umask 0000`呼び出しが削除されます。代わりに、Runnerはビルドコンテナの実行ユーザーのユーザーID（UID）とグループID（GID）を検出します。またRunnerは、（ソースの更新、キャッシュの復元、およびアーティファクトのダウンロード後に）定義済みのコンテナで`chown`コマンドを実行することにより、作業ディレクトリとファイルの所有権を変更します。 |
| `FF_USE_LEGACY_S3_CACHE_ADAPTER` | `false` | {{< icon name="dotted-circle" >}} いいえ |  | 有効にすると、従来のS3キャッシュアダプターが使用されます。無効にすると（デフォルト）、認証にAmazonのS3 SDKを使用する新しいS3キャッシュアダプターが使用されます。これにより、カスタムSTSエンドポイントなど、従来のアダプターでは解決が困難だった環境での認証の問題が解決されます。 |
| `FF_GIT_URLS_WITHOUT_TOKENS` | `false` | {{< icon name="dotted-circle" >}} いいえ |  | 有効にすると、Git設定またはコマンドの実行中にGitLab Runnerはジョブトークンをどこにも埋め込みません。代わりに、環境変数を使用してジョブトークンを取得するGit認証情報ヘルパーをセットアップします。このアプローチではトークンの保存が制限され、トークンリークのリスクが軽減されます。 |
| `FF_WAIT_FOR_POD_TO_BE_REACHABLE` | `false` | {{< icon name="dotted-circle" >}} いいえ |  | 有効にすると、Runnerはポッド状態が「Running」になるまで、およびポッドに証明書がアタッチされた状態で準備が整うまで待機します。 |
| `FF_MASK_ALL_DEFAULT_TOKENS` | `true` | {{< icon name="dotted-circle" >}} いいえ |  | 有効にすると、GitLab Runnerはすべてのデフォルトトークンパターンを自動的にマスクします。 |
| `FF_EXPORT_HIGH_CARDINALITY_METRICS` | `false` | {{< icon name="dotted-circle" >}} いいえ |  | 有効にすると、Runnerはカーディナリティが高いメトリクスをエクスポートします。大量のデータをインジェストすることを避けるために、この機能フラグを有効にする場合は特に注意する必要があります。詳細については、[フリートスケーリング](../fleet_scaling/_index.md)を参照してください。 |
| `FF_USE_FLEETING_ACQUIRE_HEARTBEATS` | `false` | {{< icon name="dotted-circle" >}} いいえ |  | 有効にすると、ジョブがインスタンスに割り当てられる前に、フリートインスタンスの接続が確認されます。 |
| `FF_USE_EXPONENTIAL_BACKOFF_STAGE_RETRY` | `true` | {{< icon name="dotted-circle" >}} いいえ |  | これが有効の場合、`GET_SOURCES_ATTEMPTS`、`ARTIFACT_DOWNLOAD_ATTEMPTS`、`RESTORE_CACHE_ATTEMPTS`、`EXECUTOR_JOB_SECTION_ATTEMPTS`の再試行では、指数バックオフ（5秒～5分）が使用されます。 |
| `FF_USE_ADAPTIVE_REQUEST_CONCURRENCY` | `true` | {{< icon name="dotted-circle" >}} いいえ |  | これが有効の場合、`request_concurrency`の設定が最大並行処理値になり、同時リクエスト数はジョブリクエストの成功率に基づいて調整されます。 |
| `FF_USE_GITALY_CORRELATION_ID` | `true` | {{< icon name="dotted-circle" >}} いいえ |  | 有効にすると、すべてのGit HTTPリクエストに`X-Gitaly-Correlation-ID`ヘッダーが追加されます。無効にすると、Git操作はGitaly Correlation IDヘッダーなしで実行されます。 |
| `FF_HASH_CACHE_KEYS` | `false` | {{< icon name="dotted-circle" >}} いいえ |  | GitLab Runnerがキャッシュを作成または抽出する際に、ローカルと分散キャッシュ（S3など）の両方に対して、使用前にキャッシュキーをハッシュします（SHA256）。詳細については、[キャッシュキーの処理](advanced-configuration.md#cache-key-handling)を参照してください。 |
| `FF_ENABLE_JOB_INPUTS_INTERPOLATION` | `false` | {{< icon name="dotted-circle" >}} いいえ |  | 有効にすると、ジョブの入力が補間されます。詳細については、[&17833](https://gitlab.com/groups/gitlab-org/-/epics/17833)を参照してください。 |
| `FF_USE_JOB_ROUTER` | `false` | {{< icon name="dotted-circle" >}} いいえ |  | GitLab RunnerがGitLabに直接接続するのではなく、ジョブRouterに接続してジョブをフェッチするようにします。 |
| `FF_SCRIPT_TO_STEP_MIGRATION` | `false` | {{< icon name="dotted-circle" >}} いいえ |  | 有効にすると、ユーザースクリプトはステップに移行され、ステップランナーで実行されます。 |

<!-- feature_flags_list_end -->

## パイプライン設定で機能フラグを有効にする {#enable-feature-flag-in-pipeline-configuration}

[CI/CD変数](https://docs.gitlab.com/ci/variables/)を使用して、機能フラグを有効にできます:

- パイプライン内のすべてのジョブ（グローバル）:

  ```yaml
  variables:
    FEATURE_FLAG_NAME: 1
  ```

- 単一ジョブ:

  ```yaml
  job:
    stage: test
    variables:
      FEATURE_FLAG_NAME: 1
    script:
    - echo "Hello"
  ```

## Runner環境変数で機能フラグを有効にする {#enable-feature-flag-in-runner-environment-variables}

Runnerが実行するすべてのジョブで機能を有効にするには、[Runner設定](advanced-configuration.md)で機能フラグを[`environment`](advanced-configuration.md#the-runners-section)変数として指定します。

```toml
[[runners]]
  name = "example-runner"
  url = "https://gitlab.com/"
  token = "TOKEN"
  limit = 0
  executor = "docker"
  builds_dir = ""
  shell = ""
  environment = ["FEATURE_FLAG_NAME=1"]
```

## Runner設定で機能フラグを有効にする {#enable-feature-flag-in-runner-configuration}

機能フラグを有効にするには、`[runners.feature_flags]`に機能フラグを指定します。この設定では、ジョブが機能フラグの値を上書きすることを防止できます。

一部の機能フラグは、ジョブの実行方法に対処しないため、この設定を行うときにのみ使用できます。

```toml
[[runners]]
  name = "example-runner"
  url = "https://gitlab.com/"
  token = "TOKEN"
  executor = "docker"
  [runners.feature_flags]
    FF_USE_DIRECT_DOWNLOAD = true
```
