---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: ジョブの実行を高速化する
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

ジョブのパフォーマンスは、イメージと依存関係をキャッシュすることで改善できます。

## コンテナにプロキシを使用する {#use-a-proxy-for-containers}

Dockerイメージのダウンロードにかかる時間を短縮するには、次を使用します:

- GitLab依存プロキシ、または
- Docker Hubレジストリのミラー
- その他のオープンソースソリューション

### GitLab依存プロキシ {#gitlab-dependency-proxy}

コンテナイメージにすばやくアクセスするには、[依存プロキシを使用](https://docs.gitlab.com/user/packages/dependency_proxy/)してコンテナイメージをプロキシできます。

### Docker Hubレジストリミラー {#docker-hub-registry-mirror}

Docker Hubをミラーリングすることで、ジョブがコンテナイメージにアクセスするのにかかる時間を短縮することもできます。これにより、[プルスルーキャッシュ](https://docs.docker.com/docker-hub/image-library/mirror/)としてのレジストリが実現します。ジョブの実行を高速化するだけでなく、ミラーはDocker Hubの停止やDocker Hubのレート制限に対するインフラストラクチャの回復力を高めることができます。

Dockerデーモンが[ミラーを使用するように設定されている](https://docs.docker.com/docker-hub/image-library/mirror/#configure-the-docker-daemon)場合、実行中のミラーのインスタンスでイメージが自動的にチェックされます。利用できない場合は、パブリックなDockerレジストリからイメージをプルし、ローカルに保存してから返されます。

同じイメージに対する次回のリクエストは、ローカルレジストリからプルされます。

動作の詳細については、[Dockerデーモンの設定ドキュメント](https://docs.docker.com/docker-hub/image-library/mirror/#configure-the-docker-daemon)を参照してください。

#### Docker Hubレジストリミラーを使用する {#use-a-docker-hub-registry-mirror}

Docker Hubレジストリミラーを作成するには:

1. プロキシコンテナレジストリが実行される専用マシンにログインします。
1. そのマシンに[Docker Engine](https://docs.docker.com/get-started/get-docker/)がインストールされていることを確認してください。
1. 新しいコンテナレジストリを作成します:

   ```shell
   docker run -d -p 6000:5000 \
       -e REGISTRY_PROXY_REMOTEURL=https://registry-1.docker.io \
       --restart always \
       --name registry registry:2
   ```

   レジストリを別のポートで公開するために、ポート番号（`6000`）を変更できます。これにより、`http`でサーバーが起動します。TLS（`https`）をオンにする場合は、[公式ドキュメント](https://distribution.github.io/distribution/about/configuration/#tls)に従ってください。

1. サーバーのIPアドレスを確認します:

   ```shell
   hostname --ip-address
   ```

   プライベートネットワークのIPアドレスを選択する必要があります。プライベートネットワークは、DigitalOcean、AWS、Azureなどの単一のプロバイダー上のマシン間の内部通信にとって、通常は最速のソリューションです。通常、プライベートネットワークで転送されるデータは、月間帯域幅制限に適用されません。

Docker Hubレジストリは`MY_REGISTRY_IP:6000`でアクセスできます。

これで、新しいレジストリサーバーを使用するように[`config.toml`](autoscale.md#distributed-container-registry-mirroring)を設定できます。

### その他のオープンソースソリューション {#other-open-source-solutions}

- [`rpardini/docker-registry-proxy`](https://github.com/rpardini/docker-registry-proxy)は、GitLabコンテナレジストリを含むほとんどのコンテナレジストリをローカルでプロキシできます。

## 分散キャッシュを使用する {#use-a-distributed-cache}

分散[キャッシュ](https://docs.gitlab.com/ci/yaml/#cache)を使用することで、言語の依存関係をダウンロードするのにかかる時間を短縮できます。

分散キャッシュを指定するには、キャッシュサーバーをセットアップし、次にそのキャッシュサーバーを使用するように[Runnerを設定](advanced-configuration.md#the-runnerscache-section)します。

オートスケールを使用している場合は、分散Runnerの[キャッシュ](autoscale.md#distributed-runners-caching)機能について詳しく学んでください。

次のキャッシュサーバーがサポートされています:

- [AWS S3](#use-aws-s3)
- [MinIO](#use-minio)またはその他のS3互換キャッシュサーバー
- [Google Cloud Storage](#use-google-cloud-storage)
- [Azure Blob Storage](#use-azure-blob-storage)

GitLab CI/CDの[キャッシュ](https://docs.gitlab.com/ci/caching/)の依存関係とベストプラクティスについて詳しく学んでください。

### AWS S3を使用する {#use-aws-s3}

分散キャッシュとしてAWS S3を使用するには、[Runnerの`config.toml`設定ファイルを編集](advanced-configuration.md#the-runnerscaches3-section)してS3の場所を指定し、接続用の認証情報を提供します。RunnerがS3エンドポイントへのネットワークパスを持っていることを確認してください。

NATゲートウェイを持つプライベートサブネットを使用している場合、データ転送コストを削減するためにS3 VPCエンドポイントを有効にできます。

### MinIOを使用する {#use-minio}

AWS S3を使用する代わりに、独自のキャッシュストレージを作成できます。

1. キャッシュサーバーが実行される専用マシンにログインします。
1. そのマシンに[Docker Engine](https://docs.docker.com/get-started/get-docker/)がインストールされていることを確認してください。
1. Goで書かれたシンプルなS3互換サーバーである[MinIO](https://www.min.io)を起動します:

   ```shell
   docker run -d --restart always -p 9005:9000 \
           -v /.minio:/root/.minio -v /export:/export \
           -e "MINIO_ROOT_USER=<minio_root_username>" \
           -e "MINIO_ROOT_PASSWORD=<minio_root_password>" \
           --name minio \
           minio/minio:latest server /export
   ```

   キャッシュサーバーを別のポートで公開するために、ポート`9005`を変更できます。

1. サーバーのIPアドレスを確認します:

   ```shell
   hostname --ip-address
   ```

1. キャッシュサーバーは`MY_CACHE_IP:9005`で利用可能です。
1. Runnerが使用するバケットを作成します:

   ```shell
   sudo mkdir /export/runner
   ```

   この場合、`runner`がバケットの名前です。別のバケットを選択すると、その名前は異なります。すべてのキャッシュは`/export`ディレクトリに保存されます。

1. Runnerを設定する際に、上記の`MINIO_ROOT_USER`と`MINIO_ROOT_PASSWORD`の値をアクセスキーとシークレットキーとして使用します。

これで、新しいキャッシュサーバーを使用するように[`config.toml`](autoscale.md#distributed-runners-caching)を設定できます。

### Google Cloud Storageを使用する {#use-google-cloud-storage}

分散キャッシュとしてGoogle Cloud Platformを使用するには、[Runnerの`config.toml`設定ファイルを編集](advanced-configuration.md#the-runnerscachegcs-section)してGCPの場所を指定し、接続用の認証情報を提供します。RunnerがGCSエンドポイントへのネットワークパスを持っていることを確認してください。

### Azure Blobストレージを使用する {#use-azure-blob-storage}

分散キャッシュとしてAzure Blobストレージを使用するには、[Runnerの`config.toml`設定ファイルを編集](advanced-configuration.md#the-runnerscacheazure-section)してAzureの場所を指定し、接続用の認証情報を提供します。RunnerがAzureエンドポイントへのネットワークパスを持っていることを確認してください。

### キャッシュとアーティファクトの転送を高速化する {#speed-up-cache-and-artifact-transfers}

次のオプションを使用すると、キャッシュとアーティファクトのアップロードおよびダウンロードのパフォーマンスを向上させることができます。

#### バックエンド固有のRunner設定 {#backend-specific-runner-config}

各キャッシュバックエンドには独自の`config.toml`セクションがあります。バックエンドを最適化します:

- [S3設定](advanced-configuration.md#the-runnerscaches3-section)): `BucketLocation`をRunnerと同じリージョンに設定します。5 GBを超えるアーカイブには`RoleARN`を使用して、[マルチパートアップロードを有効](advanced-configuration.md#enable-multipart-transfers-with-rolearn)にします。デフォルトのS3 v2アダプターを使用します（`FF_USE_LEGACY_S3_CACHE_ADAPTER=true`は設定しないでください）。Runnerがバケットリージョンから離れている場合に、[AWS S3 Transfer Acceleration](https://docs.aws.amazon.com/AmazonS3/latest/userguide/transfer-acceleration.html)のためにオプションで`Accelerate = true`を有効にできます。同じリージョンにある[S3 VPCエンドポイント](https://docs.aws.amazon.com/vpc/latest/privatelink/vpc-endpoints-s3.html)は、レイテンシーとコストを削減できます。
- [Google Cloud Storage設定](advanced-configuration.md#the-runnerscachegcs-section)): Runnerと同じか最も近いリージョンにあるバケットを使用します。
- [Azure Blob設定](advanced-configuration.md#the-runnerscacheazure-section)): Runnerと同じか最も近いリージョンにあるストレージアカウントを使用します。

#### キャッシュ圧縮 {#cache-compression}

より高速な圧縮を使用して、キャッシュのアーカイブとダウンロードを高速化します。これにより、より大きなアーカイブが作成されます。ジョブまたは[CI/CD変数](https://docs.gitlab.com/ci/variables/)で圧縮オプションを設定します:

| 変数 | 速度が推奨される場合 | 説明 |
|----------|------------------------|-------------|
| `CACHE_COMPRESSION_LEVEL` | `fastest`または`fast` | CPU使用率が低く、アップロードまたはダウンロードが高速になります。アーカイブは大きくなります。デフォルトは`default`です。 |
| `CACHE_COMPRESSION_FORMAT` | `zip` | `zip`は作成が高速なことが多いです。`tarzstd`はより良い圧縮率を提供しますが、遅くなる可能性があります。 |

`.gitlab-ci.yml`での設定例:

```yaml
variables:
  CACHE_COMPRESSION_LEVEL: fastest
  CACHE_COMPRESSION_FORMAT: zip
```

#### キャッシュリクエストタイムアウト {#cache-request-timeout}

大量のキャッシュがタイムアウトになる場合は、`CACHE_REQUEST_TIMEOUT` [CI/CD変数](https://docs.gitlab.com/ci/variables/)で制限（分単位）を増やしてください。デフォルトは`10`です。この設定は転送を高速化しませんが、遅いまたは大容量のアップロードおよびダウンロードでの失敗を防ぎます。

#### キャッシュ転送バッファサイズ（スループット） {#cache-transfer-buffer-size-throughput}

キャッシュのダウンロードとアップロードには、単一のストリーミングバッファを使用します。バッファが大きいほどシステムコールが減少し、特に転送が20～30 MB/秒付近で頭打ちになる場合は、スループットが向上することがよくあります。

`CACHE_TRANSFER_BUFFER_SIZE`（バイト単位）をジョブ環境または[CI/CD変数](https://docs.gitlab.com/ci/variables/)で設定します。デフォルトは4 MiB（4194304）です。

8 MiBの設定例:

```yaml
variables:
  CACHE_TRANSFER_BUFFER_SIZE: "8388608"
```

#### キャッシュチャンクサイズと並行処理 {#cache-chunk-size-and-concurrency}

チャンクサイズとは、並列アップロード（GoCloud）または並列ダウンロード（プリサイン済みまたはGoCloud）の各部分またはチャンクのバイト単位のサイズです。並行処理とは、並列で実行されるチャンクの数です。メモリ使用量は、約チャンクサイズ x 並行処理です。

| 変数 | 説明 | デフォルト |
|----------|-------------|---------|
| `CACHE_CHUNK_SIZE` | チャンクサイズ（バイト単位）。アップロード（GoCloudバックエンド）の場合: 制限はバックエンドに依存します（例えば、S3ではパーツごとに5 MiBから5 GiB、最大10,000パーツ。AzureとGCSには独自の制限があります）。ダウンロードの場合: 0 = レガシー順次。並行処理 > 1の場合、設定されていなければ16 MiBが使用されます。 | アップロード: 16 MiB（16777216）。ダウンロード: 0（レガシー） |
| `CACHE_CONCURRENCY` | 並行処理チャンクの数。アップロード: GoCloudバックエンドのみ（RoleARN付きS3、Azure、GCS）。ダウンロード: 0または1 = レガシー順次モード。1より大きい値 = 並列モード（プリサイン済みまたはGoCloud）。 | アップロード: 16。ダウンロード: 0（レガシー） |

カスタムチューニングの設定例（例えば、32 MiBチャンク、32並行処理）:

```yaml
variables:
  CACHE_CHUNK_SIZE: "33554432"
  CACHE_CONCURRENCY: "32"
```

#### GitLabへのアーティファクトのアップロード {#artifact-uploads-to-gitlab}

GitLabはアーティファクトをGitLabコーディネーターに送信し、それはオブジェクトストレージに保存される可能性があります。Runnerからのアップロードを高速化するには:

| 変数 | 速度が推奨される場合 | 説明 |
|----------|------------------------|-------------|
| `ARTIFACT_COMPRESSION_LEVEL` | `fastest`または`fast` | アップロード前のCPU使用率と圧縮にかかる時間を削減します。 |

ジョブまたはCI/CD変数で圧縮オプションを設定します。例:

```yaml
variables:
  ARTIFACT_COMPRESSION_LEVEL: fastest
```

#### オブジェクトストレージからのアーティファクトのダウンロード {#artifact-downloads-from-object-storage}

コーディネーターがアーティファクトのダウンロードをオブジェクトストレージ（`direct_download`）にリダイレクトする場合、`FF_USE_PARALLEL_ARTIFACT_TRANSFER` [機能フラグ](feature-flags.md)を使用して並列範囲ダウンロードを有効にできます。これは並列キャッシュ転送（`FF_USE_PARALLEL_CACHE_TRANSFER`）とは別です。[並列アーティファクトダウンロード（直接ダウンロード）](advanced-configuration.md#parallel-artifact-downloads-direct-download)を参照してください。
