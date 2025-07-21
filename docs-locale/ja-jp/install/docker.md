---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: コンテナ内でGitLab Runnerを実行する
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

DockerコンテナでGitLab Runnerを実行して、CI/CDジョブを実行できます。GitLab Runner Dockerイメージには、以下の実行に必要なすべての依存関係が含まれています。

- GitLab Runnerを実行する。
- コンテナ内でCI/CDジョブを実行する。

GitLab Runner Dockerイメージは、[UbuntuまたはAlpine Linux](#docker-images)をベースとして使用しています。ホストにGitLab Runnerを直接インストールする場合と同様に、標準の`gitlab-runner`コマンドをラップします。

`gitlab-runner`コマンドはDockerコンテナで実行されます。このセットアップでは、Dockerデーモンに対する完全な制御が各GitLab Runnerコンテナに委譲されます。このため、他のペイロードも実行するDockerデーモン内部でGitLab Runnerを実行すると、分離の保証が損なわれます。

このセットアップでは、以下に示すように、実行するどのGitLab Runnerコマンドにも、それに相当する`docker run`のコマンドがあります。

- Runnerコマンド: `gitlab-runner <runner command and options...>`
- Dockerコマンド: `docker run <chosen docker options...> gitlab/gitlab-runner <runner command and options...>`

たとえば、GitLab Runnerのトップレベルのヘルプ情報を取得するには、コマンドの`gitlab-runner`の部分を`docker run [docker options] gitlab/gitlab-runner`に置き換えます。次に例を示します。

```shell
docker run --rm -t -i gitlab/gitlab-runner --help

NAME:
   gitlab-runner - a GitLab Runner

USAGE:
   gitlab-runner [global options] command [command options] [arguments...]

VERSION:
   17.9.1 (bbf75488)

(...)
```

## Docker Engineのバージョンの互換性 {#docker-engine-version-compatibility}

Docker EngineとGitLab Runnerコンテナイメージのバージョンが一致している必要はありません。GitLab Runnerイメージには下位互換性と上位互換性があります。最新の機能とセキュリティ更新を確実に入手するには、常に最新の安定版[Docker Engineバージョン](https://docs.docker.com/engine/install/)を使用する必要があります。

## Dockerイメージをインストールしてコンテナを起動する {#install-the-docker-image-and-start-the-container}

前提要件:

- [Dockerをインストール](https://docs.docker.com/get-started/get-docker/)していること。
- [FAQ](../faq/_index.md)を読んで、GitLab Runnerの一般的な問題を理解していること。

1. `docker pull gitlab/gitlab-runner:<version-tag>`コマンドを使用して、`gitlab-runner` Dockerイメージをダウンロードします。

   利用可能なバージョンタグのリストについては、[GitLab Runnerのタグ](https://hub.docker.com/r/gitlab/gitlab-runner/tags)を参照してください。
1. `docker run -d [options] <image-uri> <runner-command>`コマンドを使用して、`gitlab-runner` Dockerイメージを実行します。
1. Dockerコンテナで`gitlab-runner`を実行する場合は、コンテナの再起動時に設定が失われないようにしてください。永続ボリュームをマウントして設定を保存します。ボリュームは次のいずれかにマウントできます。

   - [ローカルシステムボリューム](#from-a-local-system-volume)
   - [Dockerボリューム](#from-a-docker-volume)

1. （オプション）[`session_server`](../configuration/advanced-configuration.md)を使用している場合は、`docker run`コマンドに`-p 8093:8093`を追加して、ポート`8093`を公開します。
1. （オプション）オートスケールにDocker Machine Executorを使用するには、`docker run`コマンドにボリュームマウントを追加して、Docker Machineストレージパス（`/root/.docker/machine`）をマウントします。

   - システムボリュームマウントの場合は、`-v /srv/gitlab-runner/docker-machine-config:/root/.docker/machine`を追加
   - Dockerの名前付きボリュームの場合は、`-v docker-machine-config:/root/.docker/machine`を追加

1. [新しいRunnerを登録します](../register/_index.md)。ジョブを取得するには、GitLab Runnerコンテナを登録する必要があります。

利用可能な設定オプションには次のものがあります。

- コンテナのタイムゾーンを設定するには、フラグ`--env TZ=<TIMEZONE>`を使用します。[利用可能なタイムゾーンの一覧](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones)を参照してください。
- [FIPS準拠のGitLab Runner](_index.md#fips-compliant-gitlab-runner)イメージを使用する場合は、`redhat/ubi9-micro`ベースの`gitlab/gitlab-runner:ubi-fips`タグを使用します。
- [信頼できるSSLサーバー証明書をインストールします](#install-trusted-ssl-server-certificates)。

### ローカルシステムボリュームを使用する場合 {#from-a-local-system-volume}

`gitlab-runner`コンテナにマウントされた設定ボリュームやその他のリソースとしてローカルシステムを使用するには、次のようにします。

1. （オプション）MacOSシステムでは、デフォルトの場合、`/srv`は存在しません。セットアップ用に`/private/srv`を作成するか、または別のプライベートディレクトリを作成します。
1. 次のコマンドを実行します（必要に応じて修正）。

   ```shell
   docker run -d --name gitlab-runner --restart always \
     -v /srv/gitlab-runner/config:/etc/gitlab-runner \
     -v /var/run/docker.sock:/var/run/docker.sock \
     gitlab/gitlab-runner:latest
   ```

### Dockerボリュームを使用する場合 {#from-a-docker-volume}

設定コンテナを使用してカスタムデータボリュームをマウントするには、次の手順に従います。

1. Dockerボリュームを作成します。

   ```shell
   docker volume create gitlab-runner-config
   ```

1. 作成したボリュームを使用してGitLab Runnerコンテナを起動します。

   ```shell
   docker run -d --name gitlab-runner --restart always \
     -v /var/run/docker.sock:/var/run/docker.sock \
     -v gitlab-runner-config:/etc/gitlab-runner \
     gitlab/gitlab-runner:latest
   ```

## Runnerの設定を更新する {#update-runner-configuration}

`config.toml`で[Runnerの設定を変更](../configuration/advanced-configuration.md)したら、`docker stop`と`docker run`でコンテナを再起動して、変更を適用します。

## Runnerのバージョンをアップグレードする {#upgrade-runner-version}

前提要件:

- 最初に使用した方法（`-v /srv/gitlab-runner/config:/etc/gitlab-runner`または`-v gitlab-runner-config:/etc/gitlab-runner`）でデータボリュームをマウントする必要があります。

1. 最新バージョン（または特定のタグ）をプルします。

   ```shell
   docker pull gitlab/gitlab-runner:latest
   ```

1. 既存のコンテナを停止して削除します。

   ```shell
   docker stop gitlab-runner && docker rm gitlab-runner
   ```

1. 最初に使用した方法でコンテナを起動します。

   ```shell
   docker run -d --name gitlab-runner --restart always \
     -v /var/run/docker.sock:/var/run/docker.sock \
     -v /srv/gitlab-runner/config:/etc/gitlab-runner \
     gitlab/gitlab-runner:latest
   ```

## Runnerのログを表示する {#view-runner-logs}

ログファイルの場所は、Runnerの起動方法によって異なります。次のようになります。

- **フォアグラウンドタスク**として（ローカルにインストールされたバイナリとして、またはDockerコンテナ内で）起動する場合は、ログは`stdout`に出力されます。
- `systemd`などを使用して**システムサービス**として起動する場合は、Syslogなどのシステムログ生成メカニズムでログが使用可能になります。
- **Dockerベースのサービス**として起動する場合は、`docker logs`コマンドを使用します。これは、`gitlab-runner ...`コマンドがコンテナのメインプロセスであるためです。

たとえば、次のコマンドでコンテナを起動すると、その名前は`gitlab-runner`に設定されます。

```shell
docker run -d --name gitlab-runner --restart always \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /srv/gitlab-runner/config:/etc/gitlab-runner \
  gitlab/gitlab-runner:latest
```

ログを表示するには、`gitlab-runner`をコンテナ名に置き換えて次のコマンドを実行します。

```shell
docker logs gitlab-runner
```

コンテナログの処理の詳細については、Dockerドキュメントの[`docker container logs`](https://docs.docker.com/reference/cli/docker/container/logs/)を参照してください。

## 信頼できるSSLサーバー証明書をインストールする {#install-trusted-ssl-server-certificates}

GitLab CI/CDサーバーが自己署名SSL証明書を使用している場合は、RunnerコンテナがGitLab CIサーバー証明書を信頼していることを確認してください。これにより、通信障害の発生を防止できます。

前提要件:

- `ca.crt`ファイルには、GitLab Runnerに信頼させたいすべてのサーバーのルート証明書が含まれている必要があります。

1. （オプション）`gitlab/gitlab-runner`イメージは、`/etc/gitlab-runner/certs/ca.crt`で信頼できるSSL証明書を探します。この動作を変更するには、`-e "CA_CERTIFICATES_PATH=/DIR/CERT"`設定オプションを使用します。
1. `ca.crt`ファイルをデータボリューム（またはコンテナ）の`certs`ディレクトリにコピーします。
1. （オプション）コンテナがすでに実行されている場合は、再起動して起動時に`ca.crt`ファイルをインポートします。

## Dockerイメージ {#docker-images}

GitLab Runner 17.10.0では、AlpineベースのDockerイメージはAlpine 3.19を使用します。次のマルチプラットフォームDockerイメージが利用可能です。

- `gitlab/gitlab-runner:latest` - Ubuntuベース、約800 MB
- `gitlab/gitlab-runner:alpine` - Alpineベース、約460 MB

UbuntuイメージとAlpineイメージの両方で利用可能なビルド手順については、[GitLab Runner](https://gitlab.com/gitlab-org/gitlab-runner/tree/main/dockerfiles)のソースを参照してください。

### Runner Dockerイメージを作成する {#create-a-runner-docker-image}

GitLabリポジトリで更新が利用可能になる前に、イメージのオペレーティングシステムをアップグレードできます。

前提要件:

- IBM Zイメージを使用していないこと（`docker-machine`依存関係が含まれていないため）。このイメージは、Linux s390xまたはLinux ppc64leプラットフォーム向けにはメンテナンスされていません。現状については、[イシュー26551](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/26551)を参照してください。

最新のAlpineバージョン用の`gitlab-runner` Dockerイメージをビルドするには、次の手順に従います。

1. `alpine-upgrade/Dockerfile`を作成します。

   ```dockerfile
   ARG GITLAB_RUNNER_IMAGE_TYPE
   ARG GITLAB_RUNNER_IMAGE_TAG
   FROM gitlab/${GITLAB_RUNNER_IMAGE_TYPE}:${GITLAB_RUNNER_IMAGE_TAG}

   RUN apk update
   RUN apk upgrade
   ```

1. アップグレードされた`gitlab-runner`イメージを作成します。

   ```shell
   GITLAB_RUNNER_IMAGE_TYPE=gitlab-runner \
   GITLAB_RUNNER_IMAGE_TAG=alpine-v17.9.1 \
   docker build -t $GITLAB_RUNNER_IMAGE_TYPE:$GITLAB_RUNNER_IMAGE_TAG \
     --build-arg GITLAB_RUNNER_IMAGE_TYPE=$GITLAB_RUNNER_IMAGE_TYPE \
     --build-arg GITLAB_RUNNER_IMAGE_TAG=$GITLAB_RUNNER_IMAGE_TAG \
     -f alpine-upgrade/Dockerfile alpine-upgrade
   ```

1. アップグレードされた`gitlab-runner-helper`イメージを作成します。

   ```shell
   GITLAB_RUNNER_IMAGE_TYPE=gitlab-runner-helper \
   GITLAB_RUNNER_IMAGE_TAG=x86_64-v17.9.1 \
   docker build -t $GITLAB_RUNNER_IMAGE_TYPE:$GITLAB_RUNNER_IMAGE_TAG \
     --build-arg GITLAB_RUNNER_IMAGE_TYPE=$GITLAB_RUNNER_IMAGE_TYPE \
     --build-arg GITLAB_RUNNER_IMAGE_TAG=$GITLAB_RUNNER_IMAGE_TAG \
     -f alpine-upgrade/Dockerfile alpine-upgrade
   ```

## コンテナでSELinuxを使用する {#use-selinux-in-your-container}

CentOS、Red Hat、Fedoraなどの一部のディストリビューションでは、基盤となるシステムのセキュリティを強化するために、デフォルトでSELinux（Security-Enhanced Linux）が使用されています。

この設定には注意が必要です。

前提要件:

- [Docker executor](../executors/docker.md)を使用してコンテナでビルドを実行するには、Runnerが`/var/run/docker.sock`にアクセスできる必要があります。
- 強制モードでSELinuxを使用する場合は、Runnerが`/var/run/docker.sock`にアクセスするときに`Permission denied`エラーが発生しないようにするため、[`selinux-dockersock`](https://github.com/dpw/selinux-dockersock)をインストールします。

1. ホストに永続ディレクトリを作成します（`mkdir -p /srv/gitlab-runner/config`）。
1. ボリュームで`:Z`を使用してDockerを実行します。

   ```shell
   docker run -d --name gitlab-runner --restart always \
     -v /var/run/docker.sock:/var/run/docker.sock \
     -v /srv/gitlab-runner/config:/etc/gitlab-runner:Z \
     gitlab/gitlab-runner:latest
   ```
