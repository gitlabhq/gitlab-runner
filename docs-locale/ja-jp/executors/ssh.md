---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: SSH
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

> [!note]
> このexecutorはメンテナンスモードです。重要なセキュリティアップデートは受け取りますが、新しい機能は予定されていません。新しいプロジェクトには、[積極的に開発されているexecutor](_index.md#selecting-the-executor)の使用を検討してください。

SSH executorは完全性のために含まれていますが、サポートされているexecutorの中で最も少ないものの1つです。GitLab Runnerは外部サーバーに接続し、SSHを介してそこでビルドを実行します。一部の組織はこのexecutorをうまく使用していますが、一般的には別のexecutorタイプを使用する方が良いでしょう。

> [!note]
> SSH executorはBashで生成されたスクリプトのみをサポートしており、キャッシュ機能はサポートされていません。

このexecutorでは、SSH経由でコマンドを実行して、リモートマシンでビルドを実行できます。

> [!note]
> GitLab RunnerがSSH executorを使用するリモートシステムで、[一般的な前提条件](_index.md#git-requirements-for-non-docker-executors)を満たしていることを確認してください。

## SSH executorを使用する {#use-the-ssh-executor}

SSH executorを使用するには、[`[runners.ssh]`](../configuration/advanced-configuration.md#the-runnersssh-section)セクションで`executor = "ssh"`を指定します。例: 

```toml
[[runners]]
  executor = "ssh"
  [runners.ssh]
    host = "example.com"
    port = "22"
    user = "root"
    password = "password"
    identity_file = "/path/to/identity/file"
```

サーバーに対して認証するには、`password`または`identity_file`、あるいはその両方を使用できます。GitLab Runnerは、`/home/user/.ssh/id_(rsa|dsa|ecdsa)`から`identity_file`を暗黙的に読み取りません。`identity_file`は明示的に指定する必要があります。

プロジェクトのソースは`~/builds/<short-token>/<concurrent-id>/<namespace>/<project-name>`にチェックアウトされます。

各要素の内容は次のとおりです。

- `<short-token>`は、Runnerのトークンの短縮バージョンです（最初の8文字）。
- `<concurrent-id>`は、同じプロジェクトのビルドを同時に実行するすべてのrunnerのリストからのインデックスです（`CI_CONCURRENT_PROJECT_ID`の[事前定義変数](https://docs.gitlab.com/ci/variables/predefined_variables/)を介してアクセス可能）。
- `<namespace>`は、GitLabでプロジェクトが保存されているネームスペースです。
- `<project-name>`は、GitLabに保存されているプロジェクトの名前です。

`~/builds`ディレクトリを上書きするには、[`config.toml`](../configuration/advanced-configuration.md)の`[[runners]]`セクションで`builds_dir`オプションを指定します。

ジョブアーティファクトをアップロードする場合は、SSH経由で接続するホストに`gitlab-runner`をインストールします。

## 厳密なホストキーチェックを設定する {#configure-strict-host-key-checking}

SSH `StrictHostKeyChecking`はデフォルトで[有効](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28192)です。SSH `StrictHostKeyChecking`を無効にするには、`[runners.ssh.disable_strict_host_key_checking]`を`true`に設定します。現在のデフォルト値は`false`です。
