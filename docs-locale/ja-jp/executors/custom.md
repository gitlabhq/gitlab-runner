---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: カスタムexecutor
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

GitLab Runnerは、ネイティブでサポートされていない環境向けに、Custom executorを提供します。例: `LXD`、`Libvirt`。

GitLab Runnerを設定して、プロビジョニング、実行、および環境のクリーンアップを行う実行可能ファイルを指定することで、独自のexecutorを作成できます。

カスタムexecutor用に設定したスクリプトは、`Drivers`と呼ばれます。たとえば、[`LXD`ドライバー](custom_examples/lxd.md)や[`Libvirt`ドライバー](custom_examples/libvirt.md)を作成できます。

## 設定 {#configuration}

いくつかの設定キーから選択できます。そのうちのいくつかはオプションです。

以下に、使用可能なすべての設定キーを使用した、カスタムexecutorの設定の例を示します:

```toml
[[runners]]
  name = "custom"
  url = "https://gitlab.com"
  token = "TOKEN"
  executor = "custom"
  builds_dir = "/builds"
  cache_dir = "/cache"
  shell = "bash"
  [runners.custom]
    config_exec = "/path/to/config.sh"
    config_args = [ "SomeArg" ]
    config_exec_timeout = 200

    prepare_exec = "/path/to/script.sh"
    prepare_args = [ "SomeArg" ]
    prepare_exec_timeout = 200

    run_exec = "/path/to/binary"
    run_args = [ "SomeArg" ]

    cleanup_exec = "/path/to/executable"
    cleanup_args = [ "SomeArg" ]
    cleanup_exec_timeout = 200

    graceful_kill_timeout = 200
    force_kill_timeout = 200
```

フィールドの定義と必要なフィールドについては、[`[runners.custom]`セクション](../configuration/advanced-configuration.md#the-runnerscustom-section)の設定を参照してください。

さらに、[`[[runners]]`](../configuration/advanced-configuration.md#the-runners-section)内の`builds_dir`と`cache_dir`の両方が必須フィールドです。

## ジョブを実行するための前提条件となるソフトウェア {#prerequisite-software-for-running-a-job}

ユーザーは、`PATH`に存在する必要がある以下を含む環境をセットアップする必要があります:

- [Git](https://git-scm.com/download)と[Git LFS](https://git-lfs.com/) ：[共通の前提条件](_index.md#prerequisites-for-non-docker-executors)を参照してください。
- [GitLab Runner](../install/_index.md): アーティファクトとキャッシュをダウンロード/更新するために使用されます。

## ステージ {#stages}

Custom executorは、ジョブの詳細を設定し、環境を準備およびクリーンアップし、ジョブスクリプトを実行するためのステージを提供します。各ステージは特定のことを担当し、留意すべき点が異なります。

Custom executorによって実行される各ステージは、組み込みのGitLab Runner executorが実行するタイミングで実行されます。

実行される各ステップは、実行中のジョブに関する情報を提供する特定の環境変数にアクセスできます。すべてのステージで、次の環境変数を使用できます:

- 標準のCI/CD [環境変数](https://docs.gitlab.com/ci/variables/) （[定義済み変数](https://docs.gitlab.com/ci/variables/predefined_variables/)を含む）。
- Custom executor Runnerホストシステムによって提供されるすべての環境変数。
- すべてのサービスとそれらの[利用可能な設定](https://docs.gitlab.com/ci/services/#available-settings-for-services)。`CUSTOM_ENV_CI_JOB_SERVICES`としてJSON形式で公開されます。

CI/CD環境変数と定義済み変数の両方に、システムの環境変数との競合を防ぐために`CUSTOM_ENV_`というプレフィックスが付きます。たとえば、`CI_BUILDS_DIR`は`CUSTOM_ENV_CI_BUILDS_DIR`として利用できます。

ステージは次の順序で実行されます:

1. `config_exec`
1. `prepare_exec`
1. `run_exec`
1. `cleanup_exec`

### サービス {#services}

[サービス](https://docs.gitlab.com/ci/services/)は、`CUSTOM_ENV_CI_JOB_SERVICES`としてJSON配列で公開されます。

次に例を示します: 

```yaml
custom:
  script:
    - echo $CUSTOM_ENV_CI_JOB_SERVICES
  services:
    - redis:latest
    - name: my-postgres:9.4
      alias: pg
      entrypoint: ["path", "to", "entrypoint"]
      command: ["path", "to", "cmd"]
```

上記の例では、`CUSTOM_ENV_CI_JOB_SERVICES`環境変数に次の値を設定します:

```json
[{"name":"redis:latest","alias":"","entrypoint":null,"command":null},{"name":"my-postgres:9.4","alias":"pg","entrypoint":["path","to","entrypoint"],"command":["path","to","cmd"]}]
```

### 設定 {#config}

設定ステージは、`config_exec`によって実行されます。

実行時にいくつかの設定を設定したい場合があります。たとえば、プロジェクトIDに基づいてビルドディレクトリを設定します。`config_exec`は、STDOUTから読み取り、特定のキーを持つ有効なJSON文字列を予期します。

次に例を示します:

```shell
#!/usr/bin/env bash

cat << EOS
{
  "builds_dir": "/builds/${CUSTOM_ENV_CI_CONCURRENT_PROJECT_ID}/${CUSTOM_ENV_CI_PROJECT_PATH_SLUG}",
  "cache_dir": "/cache/${CUSTOM_ENV_CI_CONCURRENT_PROJECT_ID}/${CUSTOM_ENV_CI_PROJECT_PATH_SLUG}",
  "builds_dir_is_shared": true,
  "hostname": "custom-hostname",
  "driver": {
    "name": "test driver",
    "version": "v0.0.1"
  },
  "job_env" : {
    "CUSTOM_ENVIRONMENT": "example"
  },
  "shell": "bash"
}
EOS
```

JSON文字列内の追加のキーはすべて無視されます。有効なJSON文字列でない場合、ステージは失敗し、さらに2回再試行されます。

| パラメータ              | 型    | 必須 | 空にすることが許可されています  | 説明 |
|------------------------|---------|----------|----------------|-------------|
| `builds_dir`           | 文字列  | ✗        | ✗              | ジョブの作業ディレクトリが作成されるベースディレクトリ。 |
| `cache_dir`            | 文字列  | ✗        | ✗              | ローカルキャッシュが格納されるベースディレクトリ。 |
| `builds_dir_is_shared` | ブール値 | ✗        | 該当なし | 同時ジョブ間で環境が共有されるかどうかを定義します。 |
| `hostname`             | 文字列  | ✗        | ✓              | Runnerによって格納されるジョブの「メタデータ」に関連付けるホスト名。未定義の場合、ホスト名は設定されません。 |
| `driver.name`          | 文字列  | ✗        | ✓              | ドライバーのユーザー定義名。`Using custom executor...`行と一緒に出力されます。未定義の場合、ドライバーに関する情報は出力されません。 |
| `driver.version`       | 文字列  | ✗        | ✓              | ドライバーのユーザー定義バージョン。`Using custom executor...`行と一緒に出力されます。未定義の場合、名前情報のみが出力されます。 |
| `job_env`              | オブジェクト  | ✗        | ✓              | ジョブ実行の後続のすべてのステージで、環境変数を介して使用できる名前と値のペア。それらは、ジョブではなく、ドライバーで使用できます。詳細については、[`job_env`の使用方法](#job_env-usage)を参照してください。 |
| `shell`                | 文字列  | ✗        | ✓              | ジョブスクリプトの実行に使用されるシェル。 |

実行可能ファイルの`STDERR`は、ジョブログに出力されます。

[`config_exec_timeout`](../configuration/advanced-configuration.md#the-runnerscustom-section)を設定して、プロセスを終了する前に、GitLab RunnerがJSON文字列の読み取りを待機する時間の上限を設定できます。

[`config_args`](../configuration/advanced-configuration.md#the-runnerscustom-section)を定義すると、定義した順序で`config_exec`実行可能ファイルに追加されます。たとえば、次の`config.toml`コンテンツがあるとします:

```toml
...
[runners.custom]
  ...
  config_exec = "/path/to/config"
  config_args = [ "Arg1", "Arg2" ]
  ...
```

GitLab Runnerは、`/path/to/config Arg1 Arg2`として実行します。

#### `job_env`の使用法 {#job_env-usage}

`job_env`設定の主な目的は、ジョブ実行の後続のステージのために、**カスタムexecutorドライバー呼び出しのコンテキストに**変数を渡すことです。

たとえば、ジョブ実行環境との接続で、いくつかの認証情報の準備が必要なドライバー。この操作はコストがかかります。ドライバーは、環境に接続する前に、ローカルプロバイダーから一時的なSSH認証情報をリクエストする必要があります。

カスタムexecutor実行フローでは、各ジョブ実行[ステージ](#stages) (`prepare`、複数の`run`呼び出し、および`cleanup`) は、独自のコンテキストを持つ個別の実行として実行されます。認証情報を解決する例では、認証情報プロバイダーへの接続を毎回行う必要があります。

この操作にコストがかかる場合は、ジョブの実行全体に対して1回実行し、すべてのジョブ実行ステージに対して認証情報を再利用します。`job_env`はここで役立ちます。これにより、`config_exec`呼び出し中にプロバイダーと1回接続し、`job_env`で受信した認証情報を渡すことができます。次に、カスタムexecutorが[`prepare_exec`](#prepare) 、[`run_exec`](#run) 、および[`cleanup_exec`](#cleanup)に呼び出しを行う変数のリストに追加されます。これにより、認証情報プロバイダーに毎回接続する代わりに、ドライバーは変数を読み取り、存在する認証情報を使用するだけです。

理解しておくべき重要なことは、**変数はジョブ自体では自動的に利用できない**ということです。これは、カスタムexecutorドライバーがどのように実装されているかに完全に依存し、多くの場合、そこには存在しません。

`job_env`設定を使用して、特定のRunnerによって実行されるすべてのジョブに変数のセットを渡す方法については、[`environment`設定（`[[runners]]`から）](../configuration/advanced-configuration.md#the-runners-section)を参照してください。

変数が動的で、ジョブ間で値が変化する可能性がある場合は、ドライバーの実装で、`job_env`によって渡される変数を実行呼び出しに追加するようにしてください。

### 準備 {#prepare}

準備ステージは、`prepare_exec`によって実行されます。

この時点で、GitLab Runnerはジョブ（どこでどのように実行されるか）に関するすべてを認識しています。残っているのは、ジョブを実行できるように、環境をセットアップすることだけです。GitLab Runnerは、`prepare_exec`で指定された実行可能ファイルを実行します。

このアクションは、環境（たとえば、仮想マシンまたはコンテナ、サービスなどを作成する）のセットアップを担当します。これが完了すると、環境はジョブを実行する準備ができていると予想されます。

このステージは、ジョブの実行で1回だけ実行されます。

[`prepare_exec_timeout`](../configuration/advanced-configuration.md#the-runnerscustom-section)を設定して、GitLab Runnerがプロセスを終了する前に環境の準備を待機する時間の上限を設定できます。

この実行可能ファイルから返された`STDOUT`と`STDERR`は、ジョブログに出力されます。

[`prepare_exec_args`](../configuration/advanced-configuration.md#the-runnerscustom-section)を定義すると、定義した順序で`prepare_exec`実行可能ファイルに追加されます。たとえば、次の`config.toml`コンテンツがあるとします:

```toml
...
[runners.custom]
  ...
  prepare_exec = "/path/to/bin"
  prepare_args = [ "Arg1", "Arg2" ]
  ...
```

GitLab Runnerは、`/path/to/bin Arg1 Arg2`として実行します。

### 実行 {#run}

実行ステージは`run_exec`によって実行されます。

この実行可能ファイルから返された`STDOUT`と`STDERR`は、ジョブログに出力されます。

他のステージとは異なり、`run_exec`ステージは複数回実行されます。これは、以下のサブステージに分割され、順番にリストされているためです:

1. `prepare_script`
1. `get_sources`
1. `restore_cache`
1. `download_artifacts`
1. `step_*`
1. `build_script`
1. `step_*`
1. `after_script`
1. `archive_cache`または`archive_cache_on_failure`
1. `upload_artifacts_on_success`または`upload_artifacts_on_failure`
1. `cleanup_file_variables`

上記の各ステージでは、`run_exec`実行可能ファイルは以下で実行されます:

- 通常の環境変数。
- 2つの引数:
  - GitLab Runnerがカスタムexecutorの実行用に作成するスクリプトへのパス。
  - ステージの名前。

次に例を示します:

```shell
/path/to/run_exec.sh /path/to/tmp/script1 prepare_executor
/path/to/run_exec.sh /path/to/tmp/script1 prepare_script
/path/to/run_exec.sh /path/to/tmp/script1 get_sources
```

`run_args`が定義されている場合、これらは`run_exec`実行可能ファイルに渡される最初の引数のセットであり、GitLab Runnerがその他を追加します。たとえば、次の`config.toml`があるとします:

```toml
...
[runners.custom]
  ...
  run_exec = "/path/to/run_exec.sh"
  run_args = [ "Arg1", "Arg2" ]
  ...
```

GitLab Runnerは、次の引数で実行可能ファイルを実行します:

```shell
/path/to/run_exec.sh Arg1 Arg2 /path/to/tmp/script1 prepare_executor
/path/to/run_exec.sh Arg1 Arg2 /path/to/tmp/script1 prepare_script
/path/to/run_exec.sh Arg1 Arg2 /path/to/tmp/script1 get_sources
```

この実行可能ファイルは、最初の引数で指定されたスクリプトを実行する役割を担う必要があります。これらには、クローン作成、アーティファクトのダウンロード、ユーザースクリプトの実行、および以下に説明するその他すべてのステップを実行するために、GitLab Runner executorが実行するすべてのスクリプトが含まれています。スクリプトは、次のシェルにすることができます:

- Bash
- PowerShell Desktop
- PowerShell Core
- バッチ処理（非推奨）

スクリプトは、[`[[runners]]`](../configuration/advanced-configuration.md#the-runners-section)内の`shell`によって設定されたシェルを使用して生成します。何も指定されていない場合は、OSプラットフォームのデフォルトが使用されます。

下の表は、各スクリプトが何を行い、そのスクリプトの主な目的が何かを詳細に説明したものです。

| スクリプト名                   | スクリプトの内容 |
|-------------------------------|-----------------|
| `prepare_script`              | ジョブが実行されているマシンに関するデバッグ情報。 |
| `get_sources`                 | Git設定を準備し、リポジトリをクローン/フェッチします。GitLabが提供するGit戦略のすべてのメリットが得られるため、これをそのままにしておくことをお勧めします。 |
| `restore_cache`               | キャッシュが定義されている場合は、展開します。これには、`gitlab-runner`バイナリが`$PATH`で使用可能であることが必要です。 |
| `download_artifacts`          | アーティファクトが定義されている場合は、ダウンロードします。これには、`gitlab-runner`バイナリが`$PATH`で使用可能であることが必要です。 |
| `step_*`                      | GitLabによって生成されます。実行するスクリプトのセット。カスタムexecutorに送信されない場合があります。`step_release`や`step_accessibility`など、複数のステップがある場合があります。これは、`.gitlab-ci.yml`ファイルの機能である可能性があります。 |
| `after_script`                | ジョブから定義された[`after_script`](https://docs.gitlab.com/ci/yaml/#before_script-and-after_script)。このスクリプトは、以前のステップのいずれかが失敗した場合でも、常に呼び出しされます。 |
| `archive_cache`               | キャッシュが定義されている場合は、すべてのキャッシュのアーカイブを作成します。`build_script`が成功した場合にのみ実行されます。 |
| `archive_cache_on_failure`    | キャッシュが定義されている場合は、すべてのキャッシュのアーカイブを作成します。`build_script`が失敗した場合にのみ実行されます。 |
| `upload_artifacts_on_success` | アーティファクトが定義されている場合は、アップロードします。`build_script`が成功した場合にのみ実行されます。 |
| `upload_artifacts_on_failure` | アーティファクトが定義されている場合は、アップロードします。`build_script`が失敗した場合にのみ実行されます。 |
| `cleanup_file_variables`      | ディスクからすべての[ファイルベース](https://docs.gitlab.com/ci/variables/#custom-environment-variables-of-type-file)変数を削除します。 |

### クリーンアップ {#cleanup}

クリーンアップステージは`cleanup_exec`によって実行されます。

この最後のステージは、以前のステージのいずれかが失敗した場合でも実行されます。このステージの主な目標は、セットアップされた可能性のある環境をクリーンアップすることです。たとえば、VMをオフにするか、コンテナを削除します。

`cleanup_exec`の結果は、ジョブのステータスに影響を与えません。たとえば、次のことが発生した場合でも、ジョブは成功としてマークされます:

- `prepare_exec`と`run_exec`の両方が成功します。
- `cleanup_exec`が失敗します。

[`cleanup_exec_timeout`](../configuration/advanced-configuration.md#the-runnerscustom-section)を設定して、GitLab Runnerがプロセスを終了する前に環境のクリーンアップを待機する時間の上限を設定できます。

この実行可能ファイルの`STDOUT`は、`DEBUG`レベルでGitLab Runnerログに出力されます。`STDERR`は、`WARN`レベルでログに出力されます。

[`cleanup_exec_args`](../configuration/advanced-configuration.md#the-runnerscustom-section)を定義すると、定義した順序で`cleanup_exec`実行可能ファイルに追加されます。たとえば、次の`config.toml`コンテンツがあるとします:

```toml
...
[runners.custom]
  ...
  cleanup_exec = "/path/to/bin"
  cleanup_args = [ "Arg1", "Arg2" ]
  ...
```

GitLab Runnerは、`/path/to/bin Arg1 Arg2`として実行します。

## 実行可能ファイルの終了と強制終了 {#terminating-and-killing-executables}

GitLab Runnerは、次のいずれかの条件で、実行可能ファイルを正常に終了しようとします:

- `config_exec_timeout`、`prepare_exec_timeout`、または`cleanup_exec_timeout`が満たされた場合。
- ジョブが[タイムアウト](https://docs.gitlab.com/ci/pipelines/settings/#set-a-limit-for-how-long-jobs-can-run)します。
- ジョブがキャンセルされました。

タイムアウトに達すると、`SIGTERM`が実行可能ファイルに送信され、[`exec_terminate_timeout`](../configuration/advanced-configuration.md#the-runnerscustom-section)のカウントダウンが開始されます。実行可能ファイルは、このシグナルをリッスンして、リソースをクリーンアップするようにする必要があります。`exec_terminate_timeout`が経過してもプロセスが実行中の場合は、`SIGKILL`がプロセスを強制終了するために送信され、[`exec_force_kill_timeout`](../configuration/advanced-configuration.md#the-runnerscustom-section)が開始されます。`exec_force_kill_timeout`が完了した後もプロセスが実行中の場合、GitLab Runnerはプロセスを中断し、停止または強制終了を試行しなくなります。これらのタイムアウトの両方が`config_exec`、`prepare_exec`、または`run_exec`中に発生した場合、ビルドは失敗としてマークされます。

ドライバーによって起動された子プロセスも、上記のUNIXベースのシステムで説明されている正常終了プロセスを受け取ります。これは、メインプロセスを、すべての子プロセスが属する[プロセスグループ](https://man7.org/linux/man-pages/man2/setpgid.2.html)として設定することで実現されます。

## Error handling {#error-handling}

GitLab Runnerは、2種類のエラーを異なる方法で処理できます。これらのエラーは、`config_exec`、`prepare_exec`、`run_exec`、および`cleanup_exec`内の実行可能ファイルがこれらのコードで終了した場合にのみ処理されます。ユーザーがゼロ以外の終了コードで終了した場合、以下のエラーコードのいずれかとして伝播される必要があります。

ユーザースクリプトがこれらのコードの1つで終了した場合、実行可能ファイルの終了コードに伝播される必要があります。

### ビルドの失敗 {#build-failure}

GitLab Runnerは、ジョブの失敗を示す終了コードとして実行可能ファイルが使用する必要がある`BUILD_FAILURE_EXIT_CODE`環境変数を提供します。実行可能ファイルが`BUILD_FAILURE_EXIT_CODE`のコードで終了した場合、ビルドはGitLab CIで適切に失敗としてマークされます。

ユーザーが`.gitlab-ci.yml`ファイル内で定義するスクリプトがゼロ以外のコードで終了した場合、`run_exec`は`BUILD_FAILURE_EXIT_CODE`値で終了する必要があります。

{{< alert type="note" >}}

ハードコードされた値の代わりに`BUILD_FAILURE_EXIT_CODE`を使用することを強く推奨します。これは、すべてのリリースで変更される可能性があり、バイナリ/スクリプトの将来性を保証するためです。

{{< /alert >}}

### ビルド失敗の終了コード {#build-failure-exit-code}

ビルドが失敗した場合に終了コードを含むファイルをオプションで指定できます。ファイルの予期されるパスは、`BUILD_EXIT_CODE_FILE`環境変数を介して提供されます。次に例を示します:

```shell
if [ $exit_code -ne 0 ]; then
  echo $exit_code > ${BUILD_EXIT_CODE_FILE}
  exit ${BUILD_FAILURE_EXIT_CODE}
fi
```

CI/CDジョブは、[`allow_failure`](https://docs.gitlab.com/ci/yaml/#allow_failure)構文を利用するために、このメソッドを必要とします。

{{< alert type="note" >}}

このファイルには、整数の終了コードのみを保存してください。追加情報があると、`unknown Custom executor executable exit code`エラーが発生する可能性があります。

{{< /alert >}}

### システム失敗 {#system-failure}

`SYSTEM_FAILURE_EXIT_CODE`で指定されたエラーコードでプロセスを終了することにより、システム失敗をRunnerに送信できます。このエラーコードが返された場合、Runnerは特定のステージングを再試行します。再試行が成功しない場合、ジョブは失敗としてマークされます。

以下は、どのステージングが再試行されるか、および再試行回数を示す表です。

| ステージング名           | 試行回数                                          | 各再試行の間隔 |
|----------------------|-------------------------------------------------------------|-------------------------------------|
| `prepare_exec`       | 3                                                           | 3秒                           |
| `get_sources`        | `GET_SOURCES_ATTEMPTS`変数の値。（デフォルトは1です）。       | 0秒                           |
| `restore_cache`      | `RESTORE_CACHE_ATTEMPTS`変数の値。（デフォルトは1です）。     | 0秒                           |
| `download_artifacts` | `ARTIFACT_DOWNLOAD_ATTEMPTS`変数の値。（デフォルトは1です）。 | 0秒                           |

{{< alert type="note" >}}

ハードコードされた値の代わりに`SYSTEM_FAILURE_EXIT_CODE`を使用することを強く推奨します。これは、すべてのリリースで変更される可能性があり、バイナリ/スクリプトの将来性を保証するためです。

{{< /alert >}}

## ジョブの応答 {#job-response}

`CUSTOM_ENV_`変数は、ドキュメント化された[CI/CD変数の優先順位](https://docs.gitlab.com/ci/variables/#cicd-variable-precedence)を監視するため、ジョブレベルで変更できます。この機能は望ましい場合がありますが、信頼できるジョブコンテキストが必要な場合は、完全なJSONジョブ応答が自動的に提供されます。Runnerは一時ファイルを生成します。これは、`JOB_RESPONSE_FILE`環境変数で参照されます。このファイルはすべてのステージングに存在し、クリーンアップ中に自動的に削除されます。

```shell
$ cat ${JOB_RESPONSE_FILE}
{"id": 123456, "token": "jobT0ken",...}
```
