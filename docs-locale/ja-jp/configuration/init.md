---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: GitLab Runnerのシステムサービス
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

GitLab Runnerは、基盤となるOSを検出し、initシステムに基づいてサービスファイルをインストールするために、[Go `service`ライブラリ](https://github.com/kardianos/service)を使用します。

> [!note]
> パッケージ`service`は、プログラムをサービス（デーモン）としてインストール、アンインストール、起動、停止、実行します。Windows XP+、Linux（systemd、Upstart、System V）、macOS（`launchd`）がサポートされています。

GitLab Runnerが[インストールされる](../install/_index.md)と、サービスファイルが自動的に作成されます:

- **systemd**: `/etc/systemd/system/gitlab-runner.service`
- **Upstart**: `/etc/init/gitlab-runner`

## カスタム環境変数の設定 {#setting-custom-environment-variables}

カスタム環境変数を使用してGitLab Runnerを実行できます。例えば、`GOOGLE_APPLICATION_CREDENTIALS`をRunnerの環境で定義したい場合などです。このアクションは、Runnerによって実行されるすべてのジョブに自動的に追加される変数を定義する、[`environment`設定](advanced-configuration.md#the-runners-section)とは異なります。

### systemdのカスタマイズ {#customizing-systemd}

systemdを使用するRunnerの場合、`/etc/systemd/system/gitlab-runner.service.d/env.conf`を作成し、エクスポートする変数ごとに1つの`Environment=key=value`行を使用します。

例: 

```toml
[Service]
Environment=GOOGLE_APPLICATION_CREDENTIALS=/etc/gitlab-runner/gce-credentials.json
```

次に、設定をリロードします:

```shell
systemctl daemon-reload
systemctl restart gitlab-runner.service
```

### Upstartのカスタマイズ {#customizing-upstart}

Upstartを使用するRunnerの場合、`/etc/init/gitlab-runner.override`を作成し、目的の変数をエクスポートします。

例: 

```shell
export GOOGLE_APPLICATION_CREDENTIALS="/etc/gitlab-runner/gce-credentials.json"
```

これを有効にするには、Runnerを再起動してください。

## デフォルトの停止動作をオーバーライドする {#overriding-default-stopping-behavior}

場合によっては、サービスのデフォルトの動作をオーバーライドしたい場合があります。

例えば、GitLab Runnerをアップグレードする際、すべての実行中のジョブが終了するまで、安全に停止する必要があります。しかし、systemd、Upstart、またはその他のサービスは、気づかないうちにプロセスをすぐに再起動する可能性があります。

したがって、GitLab Runnerをアップグレードすると、インストールスクリプトは、当時新しいジョブを処理していたであろうRunnerプロセスを強制終了し、再起動します。

### systemdのオーバーライド {#overriding-systemd}

systemdを使用するRunnerの場合、`/etc/systemd/system/gitlab-runner.service.d/kill.conf`を以下の内容で作成します:

```toml
[Service]
TimeoutStopSec=7200
KillSignal=SIGQUIT
```

これら2つの設定をsystemdユニット設定に追加した後、Runnerを停止できます。Runnerが停止すると、systemdはプロセスを停止するためのキルシグナルとして`SIGQUIT`を使用します。さらに、停止コマンドには2時間のタイムアウトが設定されます。このタイムアウトまでにいずれかのジョブが正常に終了しない場合、systemdは`SIGKILL`を使用してプロセスを強制終了します。

### Upstartのオーバーライド {#overriding-upstart}

Upstartを使用するRunnerの場合、`/etc/init/gitlab-runner.override`を以下の内容で作成します:

```shell
kill signal SIGQUIT
kill timeout 7200
```

これら2つの設定をUpstartユニット設定に追加した後、Runnerを停止できます。Upstartは、上記のsystemdと同じ処理を行います。
