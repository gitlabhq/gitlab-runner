---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: SSH
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

{{< alert type="note" >}}

SSH executorは、Bashで生成されたスクリプトのみをサポートしており、キャッシュ機能はサポートされていません。

{{< /alert >}}

このexecutorでは、SSH経由でコマンドを実行して、リモートマシンでビルドを実行できます。

{{< alert type="note" >}}

GitLab RunnerがSSH executorを使用するすべてのリモートシステムで、[一般的な前提要件](_index.md#prerequisites-for-non-docker-executors)を満たしていることを確認してください。

{{< /alert >}}

## SSH executorを使用する {#use-the-ssh-executor}

SSH executorを使用するには、[`[runners.ssh]`](../configuration/advanced-configuration.md#the-runnersssh-section)セクションで`executor = "ssh"`を指定します。次に例を示します。

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
- `<concurrent-id>`は、プロジェクトのコンテキストで特定のRunner上のローカルジョブIDを識別する一意の番号です。
- `<namespace>`は、GitLabでプロジェクトが保存されているネームスペースです。
- `<project-name>`は、GitLabに保存されているプロジェクトの名前です。

`~/builds`ディレクトリを上書きするには、[`config.toml`](../configuration/advanced-configuration.md)の`[[runners]]`セクションで`builds_dir`オプションを指定します。

ジョブアーティファクトをアップロードする場合は、SSH経由で接続するホストに`gitlab-runner`をインストールします。

## 厳密なホストキーチェックを設定する {#configure-strict-host-key-checking}

SSHの`StrictHostKeyChecking`を有効にするには、`[runners.ssh.disable_strict_host_key_checking]`が`false`に設定されていることを確認してください。現在のデフォルトは`true`です。

[GitLab 15.0以降](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28192)のデフォルト値は`false`です。つまり、ホストキーチェックは必須です。
