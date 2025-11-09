---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: ジョブの実行を高速化する
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

イメージと依存関係をキャッシュすることで、ジョブのパフォーマンスを向上させることができます。

## コンテナのプロキシの使用 {#use-a-proxy-for-containers}

以下を使用すると、Dockerイメージをダウンロードする時間を短縮できます:

- GitLab依存プロキシ、または
- DockerHubレジストリのミラー
- その他のオープンソースソリューション

### GitLab Dependency Proxy {#gitlab-dependency-proxy}

コンテナイメージへのアクセスをより迅速に行うために、[依存プロキシを使用](https://docs.gitlab.com/user/packages/dependency_proxy/)して、コンテナイメージをプロキシできます。

### Docker Hubレジストリミラー {#docker-hub-registry-mirror}

Docker Hubをミラーリングすることで、ジョブがコンテナイメージにアクセスする時間を短縮することもできます。これにより、[Registry as a pull through cache](https://docs.docker.com/docker-hub/image-library/mirror/)になります。ジョブの実行速度が向上するだけでなく、ミラーを使用すると、Docker Hub停止やDocker Hubレート制限に対するインフラストラクチャの耐性を高めることができます。

Dockerデーモンが[mirrorを使用するように設定されている](https://docs.docker.com/docker-hub/image-library/mirror/#configure-the-docker-daemon)場合、ミラーの実行中のインスタンスでイメージが自動的に確認されます。利用できない場合、パブリックDockerレジストリからイメージをプルし、ローカルに保存してから、ユーザーに返します。

同じイメージに対する次のリクエストは、ローカルレジストリからプルされます。

その仕組みの詳細については、[Dockerデーモンの設定ドキュメント](https://docs.docker.com/docker-hub/image-library/mirror/#configure-the-docker-daemon)を参照してください。

#### Docker Hubレジストリミラーを使用 {#use-a-docker-hub-registry-mirror}

Docker Hubレジストリミラーを作成するには、次の手順に従います:

1. プロキシコンテナレジストリが実行される専用マシンにログインします。
1. [Docker Engine](https://docs.docker.com/get-started/get-docker/)がそのマシンにインストールされていることを確認してください。
1. 新しいコンテナレジストリを作成します:

   ```shell
   docker run -d -p 6000:5000 \
       -e REGISTRY_PROXY_REMOTEURL=https://registry-1.docker.io \
       --restart always \
       --name registry registry:2
   ```

   レジストリを別のポートで公開するには、ポート番号（`6000`）を変更できます。これにより、`http`でサーバーが起動します。TLS（`https`）を有効にする場合は、[公式ドキュメント](https://distribution.github.io/distribution/about/configuration/#tls)に従ってください。

1. サーバーのIPアドレスを確認します:

   ```shell
   hostname --ip-address
   ```

   プライベートネットワークのIPアドレスを選択する必要があります。通常、プライベートネットワークは、DigitalOcean、AWS、またはAzureのような単一プロバイダーのマシン間の内部通信に最適なソリューションです。通常、プライベートネットワークで転送されるデータは、月間帯域幅の制限には適用されません。

Docker Hubレジストリは、`MY_REGISTRY_IP:6000`でアクセスできます。

新しいレジストリサーバーを使用するように[`config.toml`設定](autoscale.md#distributed-container-registry-mirroring)できるようになりました。

### その他のオープンソースソリューション {#other-open-source-solutions}

- [`rpardini/docker-registry-proxy`](https://github.com/rpardini/docker-registry-proxy)は、GitLabコンテナレジストリを含む、ほとんどのコンテナレジストリをローカルでプロキシできます。

## 分散キャッシュを使用する {#use-a-distributed-cache}

分散[キャッシュ](https://docs.gitlab.com/ci/yaml/#cache)を使用すると、言語の依存関係をダウンロードする時間を短縮できます。

分散キャッシュを指定するには、キャッシュサーバーをセットアップしてから、[Runnerがそのキャッシュサーバーを使用するように設定します](advanced-configuration.md#the-runnerscache-section)。

オートスケールを使用している場合は、分散Runnerの[キャッシュ機能](autoscale.md#distributed-runners-caching)の詳細をご覧ください。

以下のキャッシュサーバーがサポートされています:

- [AWS S3](#use-aws-s3)
- [MinIO](#use-minio)またはその他のS3互換キャッシュサーバー
- [Google Cloud Storage](#use-google-cloud-storage)
- [Azure Blob Storage](#use-azure-blob-storage)

GitLab CI/CDの[キャッシュの依存関係とベストプラクティス](https://docs.gitlab.com/ci/caching/)をご覧ください。

### AWS S3を使用 {#use-aws-s3}

分散キャッシュとしてAWS S3を使用するには、[Runnerの`config.toml`設定ファイルを編集](advanced-configuration.md#the-runnerscaches3-section)してS3の場所を指定し、接続用の認証情報を提供します。RunnerにS3エンドポイントへのネットワークパスがあることを確認してください。

S3 VPCエンドポイントを有効にすると、NATゲートウェイを備えたプライベートサブネットを使用している場合、データ転送のコストを節約できます。

### MinIOを使用 {#use-minio}

AWS S3を使用する代わりに、独自のキャッシュストレージを作成できます。

1. キャッシュサーバーが実行される専用マシンにログインします。
1. [Docker Engine](https://docs.docker.com/get-started/get-docker/)がそのマシンにインストールされていることを確認してください。
1. Goで記述されたシンプルなS3互換サーバーである[MinIO](https://www.min.io)を起動します:

   ```shell
   docker run -d --restart always -p 9005:9000 \
           -v /.minio:/root/.minio -v /export:/export \
           -e "MINIO_ROOT_USER=<minio_root_username>" \
           -e "MINIO_ROOT_PASSWORD=<minio_root_password>" \
           --name minio \
           minio/minio:latest server /export
   ```

   別のポートでキャッシュサーバーを公開するには、ポート`9005`を変更できます。

1. サーバーのIPアドレスを確認します:

   ```shell
   hostname --ip-address
   ```

1. キャッシュサーバーは`MY_CACHE_IP:9005`で利用可能になります。
1. Runnerで使用されるバケットを作成します:

   ```shell
   sudo mkdir /export/runner
   ```

   `runner`はその場合のバケットの名前です。別のバケットを選択した場合、それは異なります。すべてのキャッシュは`/export`ディレクトリに保存されます。

1. Runnerを設定するときに、（上記から）`MINIO_ROOT_USER`値と`MINIO_ROOT_PASSWORD`値をアクセスキーとシークレットキーとして使用します。

新しいキャッシュサーバーを使用するように[`config.toml`設定](autoscale.md#distributed-runners-caching)できるようになりました。

### Google Cloud Storage {#use-google-cloud-storage}

分散キャッシュとしてGoogle Cloud Platformを使用するには、[Runnerの`config.toml`設定ファイルを編集](advanced-configuration.md#the-runnerscachegcs-section)してGCPの場所を指定し、接続用の認証情報を提供します。RunnerにGCSエンドポイントへのネットワークパスがあることを確認してください。

### Azure Blob Storageを使用する {#use-azure-blob-storage}

分散キャッシュとしてAzure Blobストレージを使用するには、[Runnerの`config.toml`設定ファイルを編集](advanced-configuration.md#the-runnerscacheazure-section)してAzureの場所を指定し、接続用の認証情報を提供します。RunnerにAzureエンドポイントへのネットワークパスがあることを確認してください。
