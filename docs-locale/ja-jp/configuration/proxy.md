---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: プロキシの背後でGitLab Runnerを実行する
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

このガイドは、Docker executorでGitLab Runnerをプロキシの背後で動作させることに特化しています。

続行する前に、[Dockerがインストール](https://docs.docker.com/get-started/get-docker/)され、同じマシンに[GitLab Runner](../install/_index.md)がインストールされていることを確認してください。

## `cntlm`の設定 {#configuring-cntlm}

{{< alert type="note" >}}

すでに認証なしでプロキシを使用している場合は、このセクションはオプションであり、[Dockerの設定](#configuring-docker-for-downloading-images)に直接スキップできます。`cntlm`の設定は、認証付きのプロキシの背後にいる場合にのみ必要ですが、いずれにしても使用することをお勧めします。

{{< /alert >}}

[`cntlm`](https://github.com/versat/cntlm)はローカルプロキシとして使用できるLinuxプロキシであり、プロキシの詳細を手動で追加するのに比べて、次の2つの大きな利点があります:

- 変更する必要がある認証情報は1つのソースのみ
- 認証情報はDocker Runnerからアクセスできません

[`cntlm`をインストール](https://www.howtoforge.com/linux-ntlm-authentication-proxy-isa-server-with-cntlm)したと仮定して、最初に設定する必要があります。

### `cntlm`が`docker0`インターフェースをリッスンするようにする {#make-cntlm-listen-to-the-docker0-interface}

セキュリティを強化し、インターネットから保護するために、`cntlm`をバインドして、コンテナが到達できるIPアドレスを持つ`docker0`インターフェースでリッスンします。Dockerホスト上の`cntlm`にこのアドレスのみにバインドするように指示すると、Dockerコンテナはそれに到達できますが、外部には到達できません。

1. Dockerが使用しているIPを見つけます:

   ```shell
   ip -4 -oneline addr show dev docker0
   ```

   IPアドレスは通常`172.17.0.1`です。これを`docker0_interface_ip`と呼びましょう。

1. `cntlm` (`/etc/cntlm.conf`) の設定ファイルを開きます。ユーザー名、パスワード、ドメイン、プロキシホストを入力し、前の手順で見つけた`Listen` IPアドレスを設定します。次のようになります:

   ```plaintext
   Username     testuser
   Domain       corp-uk
   Password     password
   Proxy        10.0.0.41:8080
   Proxy        10.0.0.42:8080
   Listen       172.17.0.1:3128 # Change to your docker0 interface IP
   ```

1. 変更を保存して、サービスを再起動します:

   ```shell
   sudo systemctl restart cntlm
   ```

## イメージをダウンロードするためのDockerの設定 {#configuring-docker-for-downloading-images}

{{< alert type="note" >}}

以下は、systemdをサポートするOSに適用されます。

{{< /alert >}}

プロキシの使用方法については、[Dockerドキュメント](https://docs.docker.com/engine/daemon/proxy/)を参照してください。

サービスファイルは次のようになります:

```ini
[Service]
Environment="HTTP_PROXY=http://docker0_interface_ip:3128/"
Environment="HTTPS_PROXY=http://docker0_interface_ip:3128/"
```

## GitLab Runner設定へのプロキシ変数の追加 {#adding-proxy-variables-to-the-gitlab-runner-configuration}

プロキシ変数は、プロキシの背後からGitLab.comに接続できるように、GitLab Runner設定にも追加する必要があります。

このアクションは、上記のプロキシをDockerサービスに追加するのと同じです:

1. `gitlab-runner`サービスのsystemdドロップインディレクトリを作成します:

   ```shell
   mkdir /etc/systemd/system/gitlab-runner.service.d
   ```

1. `/etc/systemd/system/gitlab-runner.service.d/http-proxy.conf`というファイルを作成して、`HTTP_PROXY`環境変数を追加します:

   ```ini
   [Service]
   Environment="HTTP_PROXY=http://docker0_interface_ip:3128/"
   Environment="HTTPS_PROXY=http://docker0_interface_ip:3128/"
   ```

   GitLab RunnerをGitLab Self-Managedインスタンスのような内部URLに接続するには、`NO_PROXY`環境変数の値を設定します。

   ```ini
   [Service]
   Environment="HTTP_PROXY=http://docker0_interface_ip:3128/"
   Environment="HTTPS_PROXY=http://docker0_interface_ip:3128/"
   Environment="NO_PROXY=gitlab.example.com"
   ```

1. ファイルを保存して、変更をフラッシュします:

   ```shell
   systemctl daemon-reload
   ```

1. GitLab Runnerを再起動します:

   ```shell
   sudo systemctl restart gitlab-runner
   ```

1. 設定が読み込まれたことを確認します:

   ```shell
   systemctl show --property=Environment gitlab-runner
   ```

   以下が表示されるはずです:

   ```ini
   Environment=HTTP_PROXY=http://docker0_interface_ip:3128/ HTTPS_PROXY=http://docker0_interface_ip:3128/
   ```

## Dockerコンテナへのプロキシの追加 {#adding-the-proxy-to-the-docker-containers}

[Runnerを登録](../register/_index.md)した後、プロキシ設定をDockerコンテナに伝播させることができます（たとえば、`git clone`など）。

これを行うには、`/etc/gitlab-runner/config.toml`を編集し、次の内容を`[[runners]]`セクションに追加する必要があります:

```toml
pre_get_sources_script = "git config --global http.proxy $HTTP_PROXY; git config --global https.proxy $HTTPS_PROXY"
environment = ["https_proxy=http://docker0_interface_ip:3128", "http_proxy=http://docker0_interface_ip:3128", "HTTPS_PROXY=docker0_interface_ip:3128", "HTTP_PROXY=docker0_interface_ip:3128"]
```

ここで、`docker0_interface_ip`は`docker0`インターフェースのIPアドレスです。

{{< alert type="note" >}}

この例では、特定のプログラムが`HTTP_PROXY`を予期し、他のプログラムが`http_proxy`を予期するため、小文字と大文字の両方の変数を設定しています。残念ながら、この種の環境変数には[標準](https://unix.stackexchange.com/questions/212894/whats-the-right-format-for-the-http-proxy-environment-variable-caps-or-no-ca#212972)がありません。

{{< /alert >}}

## `dind`サービス使用時のプロキシ設定 {#proxy-settings-when-using-dind-service}

[Docker-in-Docker executor](https://docs.gitlab.com/ci/docker/using_docker_build/#use-docker-in-docker)（`dind`）を使用する場合、`docker:2375,docker:2376`を`NO_PROXY`環境変数で指定する必要がある場合があります。ポートは必須です。そうしないと、`docker push`がブロックされます。

`dind`の`dockerd`とローカル`docker`クライアント間の通信（こちらで説明：<https://hub.docker.com/_/docker/>）は、ルートのDocker設定に保持されているプロキシ変数を使用します。

これを設定するには、`/root/.docker/config.json`を編集して、完全なプロキシ設定を含める必要があります（例：）:

```json
{
    "proxies": {
        "default": {
            "httpProxy": "http://proxy:8080",
            "httpsProxy": "http://proxy:8080",
            "noProxy": "docker:2375,docker:2376"
        }
    }
}
```

Docker executorのコンテナに設定を渡すには、`$HOME/.docker/config.json`もコンテナ内に作成する必要があります。これは、たとえば、`.gitlab-ci.yml`の`before_script`としてスクリプト化できます:

```yaml
before_script:
  - mkdir -p $HOME/.docker/
  - 'echo "{ \"proxies\": { \"default\": { \"httpProxy\": \"$HTTP_PROXY\", \"httpsProxy\": \"$HTTPS_PROXY\", \"noProxy\": \"$NO_PROXY\" } } }" > $HOME/.docker/config.json'
```

または、影響を受ける`gitlab-runner`（`/etc/gitlab-runner/config.toml`）の設定で、:

```toml
[[runners]]
  pre_build_script = "mkdir -p $HOME/.docker/ && echo \"{ \\\"proxies\\\": { \\\"default\\\": { \\\"httpProxy\\\": \\\"$HTTP_PROXY\\\", \\\"httpsProxy\\\": \\\"$HTTPS_PROXY\\\", \\\"noProxy\\\": \\\"$NO_PROXY\\\" } } }\" > $HOME/.docker/config.json"
```

{{< alert type="note" >}}

TOMLファイル内で単一の文字列として指定されたシェルを使用してJSONファイルが作成されるため、追加レベルのエスケープ`"`が必要です。これはYAMLではないため、`:`をエスケープしないでください。

{{< /alert >}}

`NO_PROXY`リストを拡張する必要がある場合、ワイルドカード`*`はサフィックスに対してのみ機能し、プレフィックスまたはCIDR表記では機能しません。詳細については、<https://github.com/moby/moby/issues/9145>および<https://unix.stackexchange.com/questions/23452/set-a-network-range-in-the-no-proxy-environment-variable>を参照してください。

## レート制限されたリクエストの処理 {#handling-rate-limited-requests}

GitLabインスタンスは、悪用を防ぐためにAPIリクエストに対するレート制限があるリバースプロキシの背後にある可能性があります。GitLab RunnerはAPIに複数のリクエストを送信し、これらのレート制限を超える可能性があります。

その結果、GitLab Runnerは、次の[再試行ロジック](#retry-logic)を使用して、レート制限されたシナリオを処理します:

### 再試行ロジック {#retry-logic}

GitLab Runnerが`429 Too Many Requests`応答を受信すると、この再試行シーケンスに従います:

1. Runnerは、応答ヘッダーで`RateLimit-ResetTime`ヘッダーを確認します。
   - `RateLimit-ResetTime`ヘッダーには、`Wed, 21 Oct 2015 07:28:00 GMT`のような有効なHTTP日付（RFC1123）である値が必要です。
   - ヘッダーが存在し、有効な値がある場合、Runnerは指定された時間まで待機し、別のリクエストを発行します。
1. `RateLimit-ResetTime`ヘッダーが無効または欠落している場合、Runnerは応答ヘッダーで`Retry-After`ヘッダーを確認します。
   - `Retry-After`ヘッダーには、`Retry-After: 30`のような秒形式の値が必要です。
   - ヘッダー形式が存在し、有効な値がある場合、Runnerは指定された時間まで待機し、別のリクエストを発行します。
1. 両方のヘッダーがないか無効な場合、Runnerはデフォルトの間隔を待機し、別のリクエストを発行します。

Runnerは、失敗したリクエストを最大5回再試行します。すべての再試行が失敗した場合、Runnerは最終応答からのエラーをログに記録します。

### サポートされているヘッダー形式 {#supported-header-formats}

| ヘッダー                | 形式              | 例                         |
|-----------------------|---------------------|---------------------------------|
| `RateLimit-ResetTime` | HTTP日付（RFC1123） | `Wed, 21 Oct 2015 07:28:00 GMT` |
| `Retry-After`         | 秒             | `30`                            |

{{< alert type="note" >}}

ヘッダー`RateLimit-ResetTime`は、すべてのヘッダーキーが[`http.CanonicalHeaderKey`](https://pkg.go.dev/net/http#CanonicalHeaderKey)関数を介して実行されるため、大文字と小文字が区別されません。

{{< /alert >}}
