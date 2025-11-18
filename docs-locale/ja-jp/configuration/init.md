---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: GitLab Runnerのシステムサービス
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

GitLab Runnerは、基盤となるOSを検出し、最終的に初期化システムに基づいてサービスファイルをインストールするために、[Go言語の`service`ライブラリ](https://github.com/kardianos/service)を使用します。

{{< alert type="note" >}}

パッケージ`service`は、プログラムをサービス（デーモン）としてインストール、アンインストール、起動、停止、および実行します。Windows XP +、Linux（systemd、Upstart、およびSystem V）、およびmacOS（`launchd`）がサポートされています。

{{< /alert >}}

GitLab Runnerが[インストールされる](../install/_index.md)と、サービスファイルが自動的に作成されます:

- **systemd**：`/etc/systemd/system/gitlab-runner.service`
- **Upstart**：`/etc/init/gitlab-runner`

## カスタム環境変数 {#setting-custom-environment-variables}

カスタム環境変数を使用してGitLab Runnerを実行できます。たとえば、Runnerの環境変数に`GOOGLE_APPLICATION_CREDENTIALS`を定義するとします。このアクションは、[`environment`設定](advanced-configuration.md#the-runners-section)とは異なります。これは、Runnerによって実行されるすべてのジョブに自動的に追加される変数を定義します。

### systemdのカスタマイズ {#customizing-systemd}

systemdを使用するRunnerの場合は、エクスポートする変数ごとに1つの`Environment=key=value`行を使用して、`/etc/systemd/system/gitlab-runner.service.d/env.conf`を作成します。

次に例を示します: 

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

Upstartを使用するRunnerの場合は、`/etc/init/gitlab-runner.override`を作成し、目的の変数をエクスポートします。

次に例を示します: 

```shell
export GOOGLE_APPLICATION_CREDENTIALS="/etc/gitlab-runner/gce-credentials.json"
```

これを有効にするには、Runnerを再起動します。

## デフォルトの停止動作のオーバーライド {#overriding-default-stopping-behavior}

場合によっては、サービスのデフォルトの動作をオーバーライドすることが必要な場合があります。

たとえば、GitLab Runnerをアップグレードするときは、実行中のすべてのジョブが完了するまで、正常に停止する必要があります。ただし、systemd、Upstart、またはその他のサービスは、気付かなくてもすぐにプロセスを再起動する可能性があります。

そのため、GitLab Runnerをアップグレードすると、インストールスクリプトは、当時新しいジョブを処理していた可能性のあるRunnerプロセスを強制終了して再起動します。

### systemdのオーバーライド {#overriding-systemd}

systemdを使用するRunnerの場合は、次のコンテンツを含む`/etc/systemd/system/gitlab-runner.service.d/kill.conf`を作成します:

```toml
[Service]
TimeoutStopSec=7200
KillSignal=SIGQUIT
```

これらの2つの設定をsystemdユニット設定に追加すると、Runnerを停止できます。Runnerが停止した後、systemdは`SIGQUIT`を強制終了シグナルとして使用して、プロセスを停止します。さらに、停止コマンドに2時間のタイムアウトが設定されています。このタイムアウトの前にジョブが正常に終了しない場合、systemdは`SIGKILL`を使用してプロセスを強制終了します。

### Upstartのオーバーライド {#overriding-upstart}

Upstartを使用するRunnerの場合は、次のコンテンツを含む`/etc/init/gitlab-runner.override`を作成します:

```shell
kill signal SIGQUIT
kill timeout 7200
```

これらの2つの設定をUpstartユニット設定に追加すると、Runnerを停止できます。Upstartは上記のsystemdと同じことを行います。
