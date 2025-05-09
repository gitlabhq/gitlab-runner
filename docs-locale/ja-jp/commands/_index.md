---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: GitLab Runnerのコマンド
---

{{< details >}}

- プラン:Free、Premium、Ultimate
- 製品:GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

GitLab Runnerには、ビルドの登録、管理、実行に使用する一連のコマンドがあります。

コマンドのリストは、以下を実行して確認できます。

```shell
gitlab-runner --help
```

コマンドの後に`--help`を付加すると、そのコマンドに固有のヘルプページが表示されます。

```shell
gitlab-runner <command> --help
```

## 環境変数を使用する

ほとんどのコマンドは、コマンドへ設定を渡す方法として環境変数をサポートしています。

特定のコマンドに対して`--help`を呼び出すと、環境変数の名前を確認できます。たとえば、`run`コマンドのヘルプメッセージは次のようになります。

```shell
gitlab-runner run --help
```

出力は次のようになります。

```plaintext
NAME:
   gitlab-runner run - run multi runner service

USAGE:
   gitlab-runner run [command options] [arguments...]

OPTIONS:
   -c, --config "/Users/ayufan/.gitlab-runner/config.toml"      Config file [$CONFIG_FILE]
```

## デバッグモードで実行する

未定義の動作またはエラーの原因を調べる場合は、デバッグモードを使用します。

コマンドをデバッグモードで実行するには、コマンドの先頭に`--debug`を追加します。

```shell
gitlab-runner --debug <command>
```

## スーパーユーザー権限

GitLab Runnerの設定にアクセスするコマンドは、スーパーユーザー（`root`）として実行する場合には動作が異なります。ファイルの場所は、コマンドを実行するユーザーに応じて異なります。

`gitlab-runner`コマンドを実行すると、実行中のモードが表示されます。

```shell
$ gitlab-runner run

INFO[0000] Starting multi-runner from /Users/ayufan/.gitlab-runner/config.toml ...  builds=0
WARN[0000] Running in user-mode.
WARN[0000] Use sudo for system-mode:
WARN[0000] $ sudo gitlab-runner...
```

`user-mode`が使用するモードであると確信できる場合は、このモードを使用してください。それ以外の場合は、コマンドの先頭に`sudo`を付加します。

```shell
$ sudo gitlab-runner run

INFO[0000] Starting multi-runner from /etc/gitlab-runner/config.toml ...  builds=0
INFO[0000] Running in system-mode.
```

Windowsの場合、コマンドプロンプトを管理者として実行する必要がある場合があります。

## 設定ファイル

GitLab Runnerの設定では[TOML](https://github.com/toml-lang/toml)形式が使用されます。

編集するファイルは次の場所にあります。

1. \*nixシステムでGitLab Runnerがスーパーユーザー（`root`）として実行されている場合は`/etc/gitlab-runner/config.toml`
1. \*nix システムでGitLab Runnerが非rootユーザーとして実行されている場合は`~/.gitlab-runner/config.toml`
1. その他のシステムでは`./config.toml`

ほとんどのコマンドは、カスタム設定ファイルを指定する引数を受け入れるため、1つのマシンで複数の異なる設定を持つことができます。カスタム設定ファイルを指定するには、`-c`または`--config`フラグを使用するか、`CONFIG_FILE`環境変数を使用します。

## シグナル

システムシグナルを使用してGitLab Runnerを操作できます。以下のコマンドは、以下のシグナルをサポートしています。

| コマンド             | シグナル                  | アクション                                                                                                |
|---------------------|-------------------------|-------------------------------------------------------------------------------------------------------|
| `register`          | **SIGINT**              | Runnerの登録をキャンセルし、すでに登録されている場合は削除します。                                   |
| `run`、`run-single` | **SIGINT**、**SIGTERM** | 実行中のすべてのビルドを中断し、できるだけ早く終了します。すぐに終了するには2回使用します（**強制シャットダウン**）。 |
| `run`、`run-single` | **SIGQUIT**             | 新しいビルドの受け入れを停止します。実行中のビルドが完了したらすぐに終了します（**正常なシャットダウン**）。       |
| `run`               | **SIGHUP**              | 設定ファイルを強制的に再読み込みします。                                                                   |

たとえばRunnerの設定ファイルを強制的に再読み込みするには、次のように実行します。

```shell
sudo kill -SIGHUP <main_runner_pid>
```

[正常なシャットダウン](#gitlab-runner-stop-doesnt-shut-down-gracefully)の場合は次のようになります。

```shell
sudo kill -SIGQUIT <main_runner_pid>
```

{{< alert type="warning" >}}

`shell`または`docker` executorを使用している場合は、正常なシャットダウンのために`killall`または`pkill`を**使用しないでください**。これによりサブプロセスも強制終了されるため、シグナルが不適切に処理される可能性があります。ジョブを処理するメインプロセスでのみ使用してください。

{{< /alert >}}

一部のオペレーティングシステムは、サービスが失敗すると自動的に再起動するように設定されています（一部のプラットフォームではデフォルトです）。ご使用のオペレーティングシステムでこのように設定されている、上記のシグナルによってRunnerがシャットダウンされると、自動的にRunnerが再起動される可能性があります。

## コマンドの概要

引数を指定せずに`gitlab-runner`を実行すると、次のように表示されます。

```plaintext
NAME:
   gitlab-runner - a GitLab Runner

USAGE:
   gitlab-runner [global options] command [command options] [arguments...]

VERSION:
   16.5.0 (853330f9)

AUTHOR:
   GitLab Inc. <support@gitlab.com>

COMMANDS:
   list                  List all configured runners
   run                   run multi runner service
   register              register a new runner
   reset-token           reset a runner's token
   install               install service
   uninstall             uninstall service
   start                 start service
   stop                  stop service
   restart               restart service
   status                get status of a service
   run-single            start single runner
   unregister            unregister specific runner
   verify                verify all registered runners
   artifacts-downloader  download and extract build artifacts (internal)
   artifacts-uploader    create and upload build artifacts (internal)
   cache-archiver        create and upload cache artifacts (internal)
   cache-extractor       download and extract cache artifacts (internal)
   cache-init            changed permissions for cache paths (internal)
   health-check          check health for a specific address
   read-logs             reads job logs from a file, used by kubernetes executor (internal)
   help, h               Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --cpuprofile value           write cpu profile to file [$CPU_PROFILE]
   --debug                      debug mode [$RUNNER_DEBUG]
   --log-format value           Choose log format (options: runner, text, json) [$LOG_FORMAT]
   --log-level value, -l value  Log level (options: debug, info, warn, error, fatal, panic) [$LOG_LEVEL]
   --help, -h                   show help
   --version, -v                print the version
```

以下で各コマンドの動作を詳しく説明します。

## 登録関連コマンド

新しいRunnerを登録するか、Runnerが登録されている場合にリストして検証するには、次のコマンドを使用します。

- [`gitlab-runner register`](#gitlab-runner-register)
  - [インタラクティブ登録](#interactive-registration)
  - [非インタラクティブ登録](#non-interactive-registration)
- [`gitlab-runner list`](#gitlab-runner-list)
- [`gitlab-runner verify`](#gitlab-runner-verify)
- [`gitlab-runner unregister`](#gitlab-runner-unregister)

これらのコマンドでは次の引数がサポートされています。

| パラメーター  | デフォルト                                                   | 説明                                    |
| ---------- | --------------------------------------------------------- | ---------------------------------------------- |
| `--config` | [設定ファイルセクション](#configuration-file)を参照 | 使用するカスタム設定ファイルを指定します |

### `gitlab-runner register`

このコマンドは、GitLab [Runners API](https://docs.gitlab.com/api/runners/#register-a-new-runner)を使用して、GitLabにRunnerを登録します。

登録されたRunnerは[設定ファイル](#configuration-file)に追加されます。1つのGitLab Runnerインストールで複数の設定を使用できます。`gitlab-runner register`を実行すると、新しい設定エントリが追加されます。以前のエントリは削除されません。

Runnerは次のいずれかの方法で登録できます。

- インタラクティブ
- 非インタラクティブ

{{< alert type="note" >}}

RunnerはGitLab [Runners API](https://docs.gitlab.com/api/runners/#register-a-new-runner)を使用して直接登録できますが、設定は自動的に生成されません。

{{< /alert >}}

#### インタラクティブ登録

このコマンドは通常、インタラクティブモード（**デフォルト**）で使用されます。Runnerの登録中に複数の質問が表示されます。

この質問に対する回答を事前に入力するには、登録コマンドの呼び出し時に引数を追加します。

```shell
gitlab-runner register --name my-runner --url "http://gitlab.example.com" --token my-authentication-token
```

あるいは`register`コマンドよりも前に環境変数を設定します。

```shell
export CI_SERVER_URL=http://gitlab.example.com
export RUNNER_NAME=my-runner
export CI_SERVER_TOKEN=my-authentication-token
gitlab-runner register
```

設定可能なすべての引数と環境を確認するには、以下を実行します。

```shell
gitlab-runner register --help
```

#### 非インタラクティブ登録

非インタラクティブ/無人モードで登録を使用することができます。

登録コマンドの呼び出し時に引数を指定できます。

```shell
gitlab-runner register --non-interactive <other-arguments>
```

あるいは`register`コマンドよりも前に環境変数を設定します。

```shell
<other-environment-variables>
export REGISTER_NON_INTERACTIVE=true
gitlab-runner register
```

{{< alert type="note" >}}

ブール値パラメーターは、コマンドラインで`--key={true|false}`を使用して渡す必要があります。

{{< /alert >}}

#### `[[runners]]`設定テンプレートファイル

{{< history >}}

- GitLab Runner 12.2で[導入](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4228)されました。

{{< /history >}}

[設定テンプレートファイル](../register/_index.md#register-with-a-configuration-template)機能を使用して、Runnerの登録中に追加のオプションを設定できます。

### `gitlab-runner list`

このコマンドは、[設定ファイル](#configuration-file)に保存されているすべてのRunnerをリストします。

### `gitlab-runner verify`

このコマンドは、登録されたRunnerがGitLabに接続できることを確認します。ただし、RunnerがGitLab Runner サービスで使用されているかどうかは検証しません。出力例を次に示します。

```plaintext
Verifying runner... is alive                        runner=fee9938e
Verifying runner... is alive                        runner=0db52b31
Verifying runner... is alive                        runner=826f687f
Verifying runner... is alive                        runner=32773c0f
```

GitLabから削除された古いRunnerを削除するには、次のコマンドを実行します。

{{< alert type="warning" >}}

この操作は元に戻すことができません。この操作では設定ファイルが更新されます。このため、実行する前に`config.toml`のバックアップがあることを確認してください。

{{< /alert >}}

```shell
gitlab-runner verify --delete
```

### `gitlab-runner unregister`

このコマンドは、GitLab [Runners API](https://docs.gitlab.com/api/runners/#delete-a-runner)を使用して、登録されているRunnerを登録解除します。

次のいずれかを指定する必要があります。

- 完全なURLとRunnerのトークン。
- Runnerの名前。

`--all-runners`オプションを使用すると、アタッチされているすべてのRunnerの登録が解除されます。

{{< alert type="note" >}}

RunnerはGitLab [Runners API](https://docs.gitlab.com/api/runners/#delete-a-runner)で登録解除できますが、ユーザーに対して設定は変更されません。

{{< /alert >}}

- Runner登録トークンを使用してRunnerが作成された場合、Runner認証トークンを指定した`gitlab-runner unregister`を実行すると、Runnerが削除されます。
- RunnerがGitLab UIまたはRunners APIで作成された場合、Runner認証トークンを指定して`gitlab-runner unregister`を実行すると、Runnerマネージャーが削除されますが、Runnerは削除されません。Runnerを完全に削除するには、[Runner管理ページでRunnerを削除する](https://docs.gitlab.com/ci/runners/runners_scope/#delete-instance-runners)か、[`DELETE /runners`](https://docs.gitlab.com/api/runners/#delete-a-runner) REST APIエンドポイントを使用します。

1つのRunnerを登録解除するには、まず`gitlab-runner list`を実行してRunnerの詳細を取得します。

```plaintext
test-runner     Executor=shell Token=t0k3n URL=http://gitlab.example.com
```

次にこの情報を使用して、次のいずれかのコマンドで登録を解除します。

{{< alert type="warning" >}}

この操作は元に戻すことができません。この操作では設定ファイルが更新されます。このため、実行する前に`config.toml`のバックアップがあることを確認してください。

{{< /alert >}}

#### URLおよびトークンを指定

```shell
gitlab-runner unregister --url "http://gitlab.example.com/" --token t0k3n
```

#### 名前を指定

```shell
gitlab-runner unregister --name test-runner
```

{{< alert type="note" >}}

指定された名前のRunnerが複数ある場合、最初のRunnerのみが削除されます。

{{< /alert >}}

#### すべてのRunner

```shell
gitlab-runner unregister --all-runners
```

### `gitlab-runner reset-token`

このコマンドはGitLab Runners APIを使用して、[Runner ID](https://docs.gitlab.com/api/runners/#reset-runners-authentication-token-by-using-the-runner-id)または[現在のトークン](https://docs.gitlab.com/api/runners/#reset-runners-authentication-token-by-using-the-current-token)のいずれかでRunnerのトークンをリセットします。

Runnerの名前（またはURLとID）が必要です。Runner IDでリセットする場合はオプションのパーソナルアクセストークン（PAT）が必要です。パーソナルアクセストークン（PAT）とRunner IDは、トークンがすでに期限切れになっている場合に使用することを目的としています。

`--all-runners`オプションを使用すると、アタッチされているRunnerのすべてのトークンがリセットされます。

#### Runnerの現在のトークンを使用

```shell
gitlab-runner reset-token --name test-runner
```

#### パーソナルアクセストークン（PAT）とRunner名を使用

```shell
gitlab-runner reset-token --name test-runner --pat PaT
```

#### パーソナルアクセストークン（PAT）、GitLab URL、およびRunner IDを使用

```shell
gitlab-runner reset-token --url "https://gitlab.example.com/" --id 12345 --pat PaT
```

#### すべてのRunner

```shell
gitlab-runners reset-token --all-runners
```

## サービス関連コマンド

次のコマンドを使用すると、Runnerをシステムサービスまたはユーザーサービスとして管理できます。Runnerサービスをインストール、アンインストール、開始、および停止するために使用します。

- [`gitlab-runner install`](#gitlab-runner-install)
- [`gitlab-runner uninstall`](#gitlab-runner-uninstall)
- [`gitlab-runner start`](#gitlab-runner-start)
- [`gitlab-runner stop`](#gitlab-runner-stop)
- [`gitlab-runner restart`](#gitlab-runner-restart)
- [`gitlab-runner status`](#gitlab-runner-status)
- [複数のサービス](#multiple-services)
- サービス関連コマンドの実行時に[**アクセスが拒否される**](#access-denied-when-running-the-service-related-commands)

すべてのサービス関連コマンドは、次の引数を受け入れます。

| パラメーター   | デフォルト                                           | 説明                                |
| ----------- | ------------------------------------------------- | ------------------------------------------ |
| `--service` | `gitlab-runner`                                   | カスタムサービス名を指定します                |
| `--config`  | [設定ファイル](#configuration-file)を参照 | 使用するカスタム設定ファイルを指定します |

### `gitlab-runner install`

このコマンドは、GitLab Runnerをサービスとしてインストールします。受け入れられる引数のセットは、実行するシステムに応じて異なります。

**Windows**で実行する場合、またはスーパーユーザーとして実行する場合は、`--user`フラグが受け入れられます。このフラグにより、**Shell** executorで実行されるビルドの権限を削除できます。

| パラメーター             | デフォルト                                           | 説明                                                                                         |
| --------------------- | ------------------------------------------------- | --------------------------------------------------------------------------------------------------- |
| `--service`           | `gitlab-runner`                                   | 使用するサービス名を指定します                                                                         |
| `--config`            | [設定ファイル](#configuration-file)を参照 | 使用するカスタム設定ファイルを指定します                                                          |
| `--syslog`            | `true`（systemd以外のシステムの場合）                  | サービスをシステムログ生成サービスと統合するかどうかを指定します                                 |
| `--working-directory` | 現在のディレクトリ                             | **Shell** executorを使用してビルドを実行するときにすべてのデータを保存するルートディレクトリを指定します |
| `--user`              | `root`                                            | ビルドを実行するユーザーを指定します                                                           |
| `--password`          | なし                                              | ビルドを実行するユーザーのパスワードを指定します                                          |

### `gitlab-runner uninstall`

このコマンドは、GitLab Runnerがサービスとして実行されないようにするため、GitLab Runnerを停止してアンインストールします。

### `gitlab-runner start`

このコマンドは、GitLab Runnerサービスを開始します。

### `gitlab-runner stop`

このコマンドは、GitLab Runnerサービスを停止します。

### `gitlab-runner restart`

このコマンドは、GitLab Runnerサービスを停止してから開始します。

### `gitlab-runner status`

このコマンドは、GitLab Runnerサービスの状態を出力します。サービスが実行中の場合の終了コードは0で、サービスが実行されていない場合は0以外です。

### 複数のサービス

`--service`フラグを指定することで、複数の個別の設定を使用して複数のGitLab Runnerサービスをインストールできます。

## 実行関連コマンド

このコマンドを使用すると、GitLabからビルドをフェッチして処理できます。

### `gitlab-runner run`

`gitlab-runner run`コマンドは、GitLab Runnerがサービスとして開始されたときに実行されるメインコマンドです。`config.toml`から定義されているすべてのRunnerを読み取り、それらすべてを実行しようとします。

コマンドは実行され、[シグナルを受信する](#signals)まで動作します。

次のパラメーターを受け入れます。

| パラメーター             | デフォルト                                       | 説明                                                                                     |
| --------------------- | --------------------------------------------- | ----------------------------------------------------------------------------------------------- |
| `--config`            | [設定ファイル](#configuration-file)を参照 | 使用するカスタム設定ファイルを指定します                                                  |
| `--working-directory` | 現在のディレクトリ                         | **Shell** executorを使用してビルドを実行するときにすべてのデータを保存するルートディレクトリを指定します |
| `--user`              | 現在のユーザー                              | ビルドを実行するユーザーを指定します                                                           |
| `--syslog`            | `false`                                       | すべてのログをSysLog（Unix）またはEventLog（Windows）に送信します                                            |
| `--listen-address`    | 空                                         | PrometheusメトリクスHTTPサーバーがリッスンするアドレス（`<host>:<port>`）       |

### `gitlab-runner run-single`

{{< history >}}

- GitLab Runner 17.1で設定ファイルを使用する機能が[導入](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/37670)されました。

{{< /history >}}

1つのGitLabインスタンスから1つのビルドを実行するには、この補助コマンドを使用します。このコマンドでは次のことができます。

- GitLab URLやRunnerトークンなど、すべてのオプションをCLIパラメーターまたは環境変数として取ります。たとえば、すべてのパラメーターが明示的に指定されたシングルジョブの場合は次のようになります。

  ```shell
  gitlab-runner run-single -u http://gitlab.example.com -t my-runner-token --executor docker --docker-image ruby:2.7
  ```

- 設定ファイルを読み取って、特定のRunnerの設定を使用します。たとえば、設定ファイルが指定されたシングルジョブの場合は次のようになります。

  ```shell
  gitlab-runner run-single -c ~/.gitlab-runner/config.toml -r runner-name
  ```

`--help`フラグを使用すると、使用可能なすべての設定オプションを確認できます。

```shell
gitlab-runner run-single --help
```

`--max-builds`オプションを使用して、Runnerが終了するまでに実行するビルドの数を制御できます。デフォルトの`0`は、Runnerにビルド制限がなく、ジョブが永久に実行されることを意味します。

`--wait-timeout`オプションを使用して、Runnerが終了するまでにジョブを待機する時間を制御することもできます。デフォルトの`0`は、Runnerにタイムアウトがなく、ジョブ間で永久に待機することを意味します。

## 内部コマンド

GitLab Runnerは単一バイナリとして配布され、ビルド中に使用されるいくつかの内部コマンドが含まれています。

### `gitlab-runner artifacts-downloader`

GitLabからアーティファクトアーカイブをダウンロードします。

### `gitlab-runner artifacts-uploader`

アーティファクトアーカイブをGitLabにアップロードします。

### `gitlab-runner cache-archiver`

キャッシュアーカイブを作成し、ローカルに保存するか、外部サーバーにアップロードします。

### `gitlab-runner cache-extractor`

ローカルまたは外部に保存されたファイルからキャッシュアーカイブを復元します。

## トラブルシューティング

よくある落とし穴のいくつかについて説明します。

### サービス関連コマンドの実行時に**アクセスが拒否される**

通常、[サービス関連コマンド](#service-related-commands)を実行するには管理者権限が必要です。

- Unix（Linux、macOS、FreeBSD）システムでは、`gitlab-runner`の前に`sudo`を付加します
- Windowsシステムでは、管理者権限でのコマンドプロンプトを使用します。`Administrator`コマンドプロンプトを実行します。Windowsの検索フィールドに`Command Prompt`を書き込むには、右クリックして`Run as administrator`を選択します。管理者権限でのコマンドプロンプトを実行することを確認します。

## `gitlab-runner stop`が正常にシャットダウンしない

GitLab Runnerがホストにインストールされており、ローカルexecutorを実行すると、アーティファクトのダウンロードやアップロード、キャッシュの処理などの操作のために追加のプロセスが開始されます。これらのプロセスは`gitlab-runner`コマンドとして実行されます。つまり、`pkill -QUIT gitlab-runner`または`killall QUIT gitlab-runner`を使用してプロセスを強制終了できます。プロセスを強制終了すると、プロセスが担当するオペレーションが失敗します。

これを防ぐには、次の2つの方法があります。

- 強制終了シグナルとして`SIGQUIT`を使用して、Runnerをローカルサービス（`systemd`など）として登録し、`gitlab-runner stop`または`systemctl stop gitlab-runner.service`を使用します。この動作を有効にするための設定例を次に示します。

  ```ini
  ; /etc/systemd/system/gitlab-runner.service.d/kill.conf
  [Service]
  KillSignal=SIGQUIT
  TimeoutStopSec=infinity
  ```

  - 設定の変更を適用するには、このファイルを作成した後、`systemctl daemon-reload`を使用して`systemd`を再読み込みします。

- `kill -SIGQUIT <pid>`を使用してプロセスを手動で強制終了します。メインの`gitlab-runner`プロセスの`pid`を確認する必要があります。これを確認するには、起動時に表示されるログを調べます。

  ```shell
  $ gitlab-runner run
  Runtime platform                                    arch=arm64 os=linux pid=8 revision=853330f9 version=16.5.0
  ```

### システムIDステートファイルの保存: アクセスが拒否される

GitLab Runner 15.7および15.8は、`config.toml`ファイルを含むディレクトリに対する書き込み権限がない場合、起動しない可能性があります。

GitLab Runnerは起動時に、`config.toml`を含むディレクトリにある`.runner_system_id`ファイルを検索します。`.runner_system_id`ファイルが見つからない場合、新しいファイルを作成します。GitLab Runnerに書き込み権限がない場合、起動が失敗します。

この問題を解決するには、一時的にファイル書き込み権限を許可して`gitlab-runner run`を実行します。`.runner_system_id`ファイルが作成されたら、権限を読み取り専用にリセットできます。
