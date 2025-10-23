---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: Shell executor
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

Shell executorを使用すると、GitLab Runnerがインストールされているマシン上でローカルにビルドを実行できます。Shell executorは、Runnerをインストールできるすべてのシステムをサポートしています。つまり、Bash、PowerShell Core、Windows PowerShell、およびWindows Batch（非推奨）向けに生成されたスクリプトを使用できます。

{{< alert type="note" >}}

GitLab RunnerがShell executorを使用するマシンで、[一般的な前提要件](_index.md#prerequisites-for-non-docker-executors)を満たしていることを確認してください。

{{< /alert >}}

## 特権ユーザーとしてスクリプトを実行する {#run-scripts-as-a-privileged-user}

`--user`を[`gitlab-runner run`コマンド](../commands/_index.md#gitlab-runner-run)に追加すると、スクリプトを非特権ユーザーとして実行できます。この機能はBashでのみサポートされています。

ソースプロジェクトは`<working-directory>/builds/<short-token>/<concurrent-id>/<namespace>/<project-name>`にチェックアウトされます。

プロジェクトのキャッシュは`<working-directory>/cache/<namespace>/<project-name>`に保存されます。

各要素の内容は次のとおりです。

- `<working-directory>`は、`gitlab-runner run`コマンドに渡された`--working-directory`の値、またはRunnerが実行されている現在のディレクトリです。
- `<short-token>`は、Runnerのトークンの短縮バージョンです（最初の8文字）。
- `<concurrent-id>`は、プロジェクトのコンテキストで特定のRunnerでローカルジョブIDを識別する一意の番号です（[定義済み変数](https://docs.gitlab.com/ci/variables/predefined_variables/)`CI_CONCURRENT_PROJECT_ID`を使用してアクセスできます）。
- `<namespace>`は、GitLabでプロジェクトが保存されているネームスペースです。
- `<project-name>`は、GitLabに保存されているプロジェクトの名前です。

`<working-directory>/builds`と`<working-directory/cache`を上書きするには、[`config.toml`](../configuration/advanced-configuration.md)の`[[runners]]`セクションで`builds_dir`オプションと`cache_dir`オプションを指定します。

## 非特権ユーザーとしてスクリプトを実行する {#run-scripts-as-an-unprivileged-user}

GitLab Runnerが[公式`.deb`パッケージまたは`.rpm`パッケージ](https://packages.gitlab.com/runner/gitlab-runner)からLinuxにインストールされる場合、インストーラーは、`gitlab_ci_multi_runner`ユーザーが検出された場合にはそのユーザーを使用しようとします。`gitlab_ci_multi_runner`ユーザーが見つからない場合には、インストーラーは代わりに`gitlab-runner`ユーザーを作成して使用します。

すべてのShellビルドは、`gitlab-runner`ユーザーと`gitlab_ci_multi_runner`ユーザーのいずれかとして実行されます。

一部のテストシナリオでは、ビルドがDocker EngineやVirtualBoxなどの特権リソースにアクセスすることが必要な場合があります。その場合は、`gitlab-runner`ユーザーをそれぞれのグループに追加する必要があります。

```shell
usermod -aG docker gitlab-runner
usermod -aG vboxusers gitlab-runner
```

## Shellを選択する {#selecting-your-shell}

GitLab Runnerは[特定のShellをサポートしています](../shells/_index.md)。Shellを選択するには、`config.toml`ファイルでそのShellを指定します。次に例を示します。

```toml
...
[[runners]]
  name = "shell executor runner"
  executor = "shell"
  shell = "powershell"
...
```

## セキュリティ {#security}

一般に、Shell executorでジョブを実行することは安全ではありません。ジョブがユーザーの権限（`gitlab-runner`）で実行され、このサーバーで実行されている他のプロジェクトからコードを「盗む」可能性があります。設定によっては、ジョブがサーバー上で高度な特権ユーザーとして任意のコマンドを実行する可能性があります。自分自身が責任を持つ信頼できるサーバー上で、信頼できるユーザーからのビルドを実行する場合にのみ、この方法を使用してください。

## プロセスの終了と強制終了 {#terminating-and-killing-processes}

Shell executorは各ジョブのスクリプトを、新しいプロセスで開始します。UNIXシステムでは、メインプロセスをプロセスグループとして設定します。

GitLab Runnerは、次の場合にプロセスを終了します。

- ジョブが[タイムアウトになった](https://docs.gitlab.com/ci/pipelines/settings/#set-a-limit-for-how-long-jobs-can-run)。
- ジョブがキャンセルされた。

UNIXシステムでは、`gitlab-runner`はプロセスとその子プロセスに`SIGTERM`を送信し、10分後に`SIGKILL`を送信します。これにより、プロセスを正常に終了できます。Windowsには`SIGTERM`と同等の機能がないため、kill（強制終了）シグナルが2回送信されます。2回目のシグナルは10分後に送信されます。
