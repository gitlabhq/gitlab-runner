---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: プロキシの背後でRunnerを実行
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

このガイドは、プロキシの背後でDocker executorを使用するGitLab Runnerを機能させることを特に目的としています。

続ける前に、同じマシンにすでに[Dockerがインストールされている](https://docs.docker.com/get-started/get-docker/)こと、および[GitLab Runnerがインストールされている](../install/_index.md)ことを確認してください。

## `cntlm`の設定 {#configuring-cntlm}

> [!note]
> 認証なしでプロキシをすでに使用している場合、このセクションはオプションであり、[Dockerの設定](#configuring-docker-for-downloading-images)に直接スキップできます。`cntlm`の設定は、認証付きプロキシの背後にいる場合にのみ必要ですが、どの場合でも使用することをお勧めします。

[`cntlm`](https://github.com/versat/cntlm)はローカルプロキシとして使用できるLinuxプロキシであり、プロキシの詳細をどこでも手動で追加する場合と比較して、2つの主な利点があります:

- 認証情報を変更する必要がある単一のソース
- 認証情報はDocker Runnerからアクセスできません

[`cntlm`がインストール済み](https://www.howtoforge.com/linux-ntlm-authentication-proxy-isa-server-with-cntlm)であることを前提として、まず設定する必要があります。

### `cntlm`を`docker0`インターフェースでリッスンさせる {#make-cntlm-listen-to-the-docker0-interface}

セキュリティとインターネットからの保護を強化するために、`cntlm`を`docker0`インターフェースにバインドしてリッスンさせます。このインターフェースは、コンテナがアクセスできるIPアドレスを持っています。Dockerホスト上の`cntlm`にこのアドレスのみにバインドするように指示すると、Dockerコンテナはアクセスできますが、外部の世界からはアクセスできません。

1. Dockerが使用しているIPを見つける:

   ```shell
   ip -4 -oneline addr show dev docker0
   ```

   IPアドレスは通常`172.17.0.1`です。これを`docker0_interface_ip`と呼びましょう。

1. `cntlm`の設定ファイル (`/etc/cntlm.conf`) を開きます。前のステップで見つけたユーザー名、パスワード、ドメイン、プロキシホストを入力し、`Listen` IPアドレスを設定します。次のようになるはずです:

   ```plaintext
   Username     testuser
   Domain       corp-uk
   Password     password
   Proxy        10.0.0.41:8080
   Proxy        10.0.0.42:8080
   Listen       172.17.0.1:3128 # Change to your docker0 interface IP
   ```

1. 変更を保存し、サービスを再起動します:

   ```shell
   sudo systemctl restart cntlm
   ```

## Dockerのイメージダウンロードの設定 {#configuring-docker-for-downloading-images}

> [!note]
> systemdをサポートするOSに以下の内容が適用されます。

プロキシの使用方法については、[Dockerのドキュメント](https://docs.docker.com/engine/daemon/proxy/)を参照してください。

サービスファイルは次のようになります:

```ini
[Service]
Environment="HTTP_PROXY=http://docker0_interface_ip:3128/"
Environment="HTTPS_PROXY=http://docker0_interface_ip:3128/"
```

## GitLab Runnerの設定にプロキシ変数を追加する {#adding-proxy-variables-to-the-gitlab-runner-configuration}

プロキシ変数は、GitLab Runnerの設定にも追加する必要があります。これにより、プロキシの背後からGitLab.comに接続できるようになります。

このアクションは、上記のDockerサービスにプロキシを追加するのと同じです:

1. `gitlab-runner`サービス用のsystemdドロップインディレクトリを作成します:

   ```shell
   mkdir /etc/systemd/system/gitlab-runner.service.d
   ```

1. `HTTP_PROXY`環境変数を追加する`/etc/systemd/system/gitlab-runner.service.d/http-proxy.conf`というファイルを作成します:

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

1. ファイルを保存し、変更をフラッシュします:

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

   次のように表示されます:

   ```ini
   Environment=HTTP_PROXY=http://docker0_interface_ip:3128/ HTTPS_PROXY=http://docker0_interface_ip:3128/
   ```

## Dockerコンテナにプロキシを追加する {#adding-the-proxy-to-the-docker-containers}

[Runnerを登録](../register/_index.md)した後、プロキシ設定をDockerコンテナに伝播させたい場合があります (たとえば、`git clone`の場合)。

これを行うには、`/etc/gitlab-runner/config.toml`を編集し、`[[runners]]`セクションに次を追加する必要があります:

```toml
pre_get_sources_script = "git config --global http.proxy $HTTP_PROXY; git config --global https.proxy $HTTPS_PROXY"
environment = ["https_proxy=http://docker0_interface_ip:3128", "http_proxy=http://docker0_interface_ip:3128", "HTTPS_PROXY=docker0_interface_ip:3128", "HTTP_PROXY=docker0_interface_ip:3128"]
```

ここで`docker0_interface_ip`は、`docker0`インターフェースのIPアドレスです。

> [!note]
> 例では、特定のプログラムが`HTTP_PROXY`を、別のプログラムが`http_proxy`を想定しているため、小文字と大文字の両方の変数を設定しています。残念ながら、これらの種類の環境変数に関する[標準](https://unix.stackexchange.com/questions/212894/whats-the-right-format-for-the-http-proxy-environment-variable-caps-or-no-ca#212972)はありません。

## `dind`サービスを使用する場合のプロキシ設定 {#proxy-settings-when-using-dind-service}

[Docker-in-Docker executor](https://docs.gitlab.com/ci/docker/using_docker_build/#use-docker-in-docker) (`dind`) を使用する場合、`docker:2375,docker:2376`を`NO_PROXY`環境変数に指定する必要がある場合があります。ポートは必須です。そうしないと`docker push`がブロックされます。

`dind`からの`dockerd`とローカル`docker`クライアント (こちらで説明されています: <https://hub.docker.com/_/docker/>) との間の通信には、rootのDocker設定で保持されているプロキシ変数が使用されます。

これを設定するには、完全なプロキシ設定を含めるように`/root/.docker/config.json`を編集する必要があります。例:

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

設定をDocker executorのコンテナに渡すには、`$HOME/.docker/config.json`もコンテナ内に作成する必要があります。これは、たとえば`.gitlab-ci.yml`の`before_script`としてスクリプト化できます:

```yaml
before_script:
  - mkdir -p $HOME/.docker/
  - 'echo "{ \"proxies\": { \"default\": { \"httpProxy\": \"$HTTP_PROXY\", \"httpsProxy\": \"$HTTPS_PROXY\", \"noProxy\": \"$NO_PROXY\" } } }" > $HOME/.docker/config.json'
```

または、影響を受ける`gitlab-runner` (`/etc/gitlab-runner/config.toml`) の設定で、次のようにします:

```toml
[[runners]]
  pre_build_script = "mkdir -p $HOME/.docker/ && echo \"{ \\\"proxies\\\": { \\\"default\\\": { \\\"httpProxy\\\": \\\"$HTTP_PROXY\\\", \\\"httpsProxy\\\": \\\"$HTTPS_PROXY\\\", \\\"noProxy\\\": \\\"$NO_PROXY\\\" } } }\" > $HOME/.docker/config.json"
```

> [!note]
> これは、TOMLファイル内で単一の文字列として指定されたシェルを持つJSONファイルを作成するため、`"`を追加でエスケープする必要があります。これはYAMLではないため、`:`をエスケープしないでください。

`NO_PROXY`リストを拡張する必要がある場合、ワイルドカード`*`はサフィックスにのみ機能し、プレフィックスやCIDR表記には機能しません。詳細については、<https://github.com/moby/moby/issues/9145>および<https://unix.stackexchange.com/questions/23452/set-a-network-range-in-the-no-proxy-environment-variable>を参照してください。

## レート制限されたリクエストの処理 {#handling-rate-limited-requests}

GitLabインスタンスは、乱用を防ぐためにAPIリクエストに対してレート制限のあるリバースプロキシの背後にある場合があります。GitLab RunnerはAPIに複数のリクエストを送信するため、これらのレート制限を超える可能性があります。

結果として、GitLab Runnerは以下の[再試行ロジック](#retry-logic)を使用して、レート制限されたシナリオを処理します:

### 再試行ロジック {#retry-logic}

GitLab Runnerが`429 Too Many Requests`応答を受け取ると、この再試行シーケンスに従います:

1. Runnerは`RateLimit-ResetTime`ヘッダーを応答のヘッダーで確認します。
   - `RateLimit-ResetTime`ヘッダーは、`Wed, 21 Oct 2015 07:28:00 GMT`のような有効なHTTP日付 (RFC1123) の値を持つ必要があります。
   - ヘッダーが存在し、有効な値を持つ場合、Runnerは指定された時間まで待機し、別のリクエストを発行します。
1. `RateLimit-ResetTime`ヘッダーが無効または欠落している場合、Runnerは応答のヘッダーで`Retry-After`ヘッダーを確認します。
   - `Retry-After`ヘッダーは、`Retry-After: 30`のような秒単位の値を持つ必要があります。
   - ヘッダー形式が存在し、有効な値を持つ場合、Runnerは指定された時間まで待機し、別のリクエストを発行します。
1. 両方のヘッダーが欠落しているか無効な場合、Runnerはデフォルトの間隔まで待機し、別のリクエストを発行します。

Runnerは失敗したリクエストを最大5回再試行します。すべての再試行が失敗した場合、Runnerは最終応答からのエラーをログに記録します。

### サポートされるヘッダー形式 {#supported-header-formats}

| ヘッダー                | 形式              | 例                         |
|-----------------------|---------------------|---------------------------------|
| `RateLimit-ResetTime` | HTTP日付 (RFC1123) | `Wed, 21 Oct 2015 07:28:00 GMT` |
| `Retry-After`         | 秒             | `30`                            |

> [!note]
> `RateLimit-ResetTime`ヘッダーは、すべてのヘッダーキーが[`http.CanonicalHeaderKey`](https://pkg.go.dev/net/http#CanonicalHeaderKey)関数によって実行されるため、大文字と小文字を区別しません。
