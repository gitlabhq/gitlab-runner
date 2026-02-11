---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: Docker executor
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

GitLab Runnerは、Docker executorを使用してDockerイメージでジョブを実行します。

Docker executorを使用すると、次のことが可能になります。

- 各ジョブで同じビルド環境を維持する。
- イメージを使用してコマンドをローカルでテストする（CIサーバーでジョブを実行する必要はない）。

Docker executorは[Docker Engine](https://www.docker.com/products/container-runtime/)を使用して、個別の隔離されたコンテナ内で各ジョブを実行します。Docker Engineに接続するために、executorは以下を使用します。

- [`.gitlab-ci.yml`](https://docs.gitlab.com/ci/yaml/)で定義するイメージとサービス。
- [`config.toml`](../commands/_index.md#configuration-file)で定義する設定。

`config.toml`でデフォルトのイメージを定義していないなら、RunnerとそのDocker executorを登録することはできません。`.gitlab-ci.yml`で何も定義されていない場合、`config.toml`で定義されているイメージを使用できます。`.gitlab-ci.yml`でイメージが定義されている場合、それは`config.toml`で定義されているイメージをオーバーライドします。

前提条件: 

- [Dockerをインストールします](https://docs.docker.com/engine/install/)。

## Docker executorのワークフロー {#docker-executor-workflow}

Docker executorは、[Alpine Linux](https://alpinelinux.org/)をベースとするDockerイメージを使用します。このイメージには、準備、ジョブ実行前、およびジョブ実行後のステップを実行するためのツールが含まれています。特別なDockerイメージの定義を確認するには、[GitLab Runnerリポジトリ](https://gitlab.com/gitlab-org/gitlab-runner/-/tree/v13.4.1/dockerfiles/runner-helper)を参照してください。

Docker executorは、ジョブを複数のステップに分割します。

1. **準備**: [サービス](https://docs.gitlab.com/ci/yaml/#services)を作成して開始します。
1. **ジョブ実行前**: クローン、[キャッシュ](https://docs.gitlab.com/ci/yaml/#cache)の復元、および前のステージからの[アーティファクト](https://docs.gitlab.com/ci/yaml/#artifacts)のダウンロードを行います。特別なDockerイメージで実行されます。
1. **ジョブ**: Runner用に設定したDockerイメージでビルドを実行します。
1. **ジョブ実行後**: キャッシュの作成、GitLabへのアーティファクトのアップロードを実行します。特別なDockerイメージで実行されます。

## サポートされている設定 {#supported-configurations}

Docker executorは以下の設定をサポートしています。

Windows設定に関する既知のイシューと追加の要件については、[Windowsコンテナを使用する](#use-windows-containers)を参照してください。

| Runnerがインストールされている場所: | executor:     | コンテナの実行: |
|-------------------------|------------------|-----------------------|
| Windows                 | `docker-windows` | Windows               |
| Windows                 | `docker`         | Linux                 |
| Linux                   | `docker`         | Linux                 |
| macOS                   | `docker`         | Linux                 |

以下の設定はサポート**されていません**。

| Runnerがインストールされている場所: | executor:     | コンテナの実行: |
|-------------------------|------------------|-----------------------|
| Linux                   | `docker-windows` | Linux                 |
| Linux                   | `docker`         | Windows               |
| Linux                   | `docker-windows` | Windows               |
| Windows                 | `docker`         | Windows               |
| Windows                 | `docker-windows` | Linux                 |

{{< alert type="note" >}}

GitLab Runnerは、Docker Engine API [v1.25](https://docs.docker.com/reference/api/engine/version/v1.25/)を使用してDocker Engineと通信します。つまり、Linuxサーバーで[サポートされる最小バージョン](https://docs.docker.com/reference/api/engine/#api-version-matrix)のDockerは`1.13.0`です。Windows Serverでは、Windows Serverのバージョンを識別するために、[これよりも新しいバージョンが必要です](#supported-docker-versions)。

{{< /alert >}}

## Docker executorを使用する {#use-the-docker-executor}

Docker executorを使用するには、`config.toml`でDockerをexecutorとして手動で定義するか、[`gitlab-runner register --executor "docker"`](../register/_index.md#register-with-a-runner-authentication-token)コマンドを使用して自動的に定義します。

次に示すのは、Dockerをexecutorとして定義している設定例です。これらの値の詳細については、[高度な設定](../configuration/advanced-configuration.md)を参照してください

```toml
concurrent = 4

[[runners]]
name = "myRunner"
url = "https://gitlab.com/ci"
token = "......"
executor = "docker"
[runners.docker]
  tls_verify = true
  image = "my.registry.tld:5000/alpine:latest"
  privileged = false
  disable_entrypoint_overwrite = false
  oom_kill_disable = false
  disable_cache = false
  volumes = [
    "/cache",
  ]
  shm_size = 0
  allowed_pull_policies = ["always", "if-not-present"]
  allowed_images = ["my.registry.tld:5000/*:*"]
  allowed_services = ["my.registry.tld:5000/*:*"]
  [runners.docker.volume_driver_ops]
    "size" = "50G"
```

## イメージとサービスを設定する {#configure-images-and-services}

前提条件: 

- ジョブが実行されるイメージには、オペレーティングシステムの`PATH`に動作するShellが必要です。サポートされているShellは次のとおりです。
  - Linux:
    - `sh`
    - `bash`
    - PowerShell Core（`pwsh`）。[13.9で導入されました](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4021)。
  - Windows:
    - PowerShell（`powershell`）
    - PowerShell Core（`pwsh`）。[13.6で導入されました](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/13139)。

Docker executorを設定するには、[`.gitlab-ci.yml`](https://docs.gitlab.com/ci/yaml/)と[`config.toml`](../commands/_index.md#configuration-file)でDockerイメージとサービスを定義します。

次のキーワードを使用します。

- `image`: Runnerがジョブを実行するために使用するDockerイメージの名前。
  - ローカルDocker Engineのイメージ、またはDocker Hubの任意のイメージを入力します。詳細については、[Dockerのドキュメント](https://docs.docker.com/get-started/introduction/)を参照してください。
  - イメージのバージョンを定義するには、コロン（`:`）を使用してタグを追加します。タグを指定しない場合、Dockerはこのバージョンとして`latest`を使用します。
- `services`: 別のコンテナを作成し、`image`にリンクする追加のイメージ。サービスの種類に関する詳細については、[サービス](https://docs.gitlab.com/ci/services/)を参照してください。

### `.gitlab-ci.yml`でイメージとサービスを定義する {#define-images-and-services-in-gitlab-ciyml}

Runnerがすべてのジョブに使用するイメージと、ビルド時に使用する一連のサービスを定義します。

例: 

```yaml
image: ruby:3.3

services:
  - postgres:9.3

before_script:
  - bundle install

test:
  script:
  - bundle exec rake spec
```

ジョブごとに異なるイメージとサービスを定義するには、次のようにします。

```yaml
before_script:
  - bundle install

test:3.3:
  image: ruby:3.3
  services:
  - postgres:9.3
  script:
  - bundle exec rake spec

test:3.4:
  image: ruby:3.4
  services:
  - postgres:9.4
  script:
  - bundle exec rake spec
```

`.gitlab-ci.yml`で`image`を定義しない場合、Runnerは`config.toml`で定義された`image`を使用します。

### `config.toml`でイメージとサービスを定義する {#define-images-and-services-in-configtoml}

Runnerが実行するすべてのジョブにイメージとサービスを追加するには、`config.toml`の`[runners.docker]`を更新します。

デフォルトの場合、`.gitlab-ci.yml`で定義されている`image`がDocker executorで使用されます。`.gitlab-ci.yml`で定義していない場合、Runnerは`config.toml`で定義されているイメージを使用します。

例: 

```toml
[runners.docker]
  image = "ruby:3.3"

[[runners.docker.services]]
  name = "mysql:latest"
  alias = "db"

[[runners.docker.services]]
  name = "redis:latest"
  alias = "cache"
```

この例では、[テーブル構文の配列](https://toml.io/en/v0.4.0#array-of-tables)を使用しています。

### プライベートレジストリのイメージを定義する {#define-an-image-from-a-private-registry}

前提条件: 

- プライベートレジストリのイメージにアクセスするには、[GitLab Runnerを認証する](https://docs.gitlab.com/ci/docker/using_docker_images/#access-an-image-from-a-private-container-registry)必要があります。

プライベートレジストリのイメージを定義するには、`.gitlab-ci.yml`でレジストリ名とイメージを指定します。

例: 

```yaml
image: my.registry.tld:5000/namespace/image:tag
```

この例では、GitLab Runnerはレジストリ`my.registry.tld:5000`でイメージ`namespace/image:tag`を検索します。

## ネットワーク設定 {#network-configurations}

サービスをCI/CDジョブに接続するには、ネットワークを設定する必要があります。

ネットワークを設定するには、次のいずれかを実行します。

- 推奨。ジョブごとにネットワークを作成するようにRunnerを設定します。
- コンテナリンクを定義します。コンテナリンクは、Dockerのレガシー機能です。

### ジョブごとにネットワークを作成する {#create-a-network-for-each-job}

ジョブごとにネットワークを作成するようにRunnerを設定できます。

このネットワーキングモードを有効にすると、Runnerはジョブごとにユーザー定義のDockerブリッジネットワークを作成して使用します。Docker環境変数は、コンテナ間で共有されません。ユーザー定義のブリッジネットワークの詳細については、[Dockerのドキュメント](https://docs.docker.com/engine/network/drivers/bridge/)を参照してください。

このネットワーキングモードを使用するには、`config.toml`の機能フラグまたは環境変数で`FF_NETWORK_PER_BUILD`を有効にします。

`network_mode`は設定しないでください。

例: 

```toml
[[runners]]
  (...)
  executor = "docker"
  environment = ["FF_NETWORK_PER_BUILD=1"]
```

または:

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.feature_flags]
    FF_NETWORK_PER_BUILD = true
```

デフォルトのDockerアドレスプールを設定するには、[`dockerd`](https://docs.docker.com/reference/cli/dockerd/)で`default-address-pool`を使用します。CIDR範囲がネットワークですでに使用されている場合、Dockerネットワークは、ホスト上の他のネットワーク（他のDockerネットワークを含む）と競合する可能性があります。

この機能は、IPv6を有効にしてDockerデーモンが設定されている場合にのみ機能します。IPv6サポートを有効にするには、Docker設定で`enable_ipv6`を`true`に設定します。詳細については、[Dockerのドキュメント](https://docs.docker.com/engine/daemon/ipv6/)を参照してください。

Runnerは、ジョブコンテナを解決するために`build`エイリアスを使用します。

{{< alert type="note" >}}

この機能を使用すると、Docker-in-Docker（`dind`）サービスでDNSが正しく機能しない場合があります。

この動作は、ネットワークを指定した場合に`dind`コンテナがカスタムDNSエントリを継承しないという、[Docker/Moby](https://github.com/moby/moby/issues/20037#issuecomment-181659049)の問題によるものです。

回避策として、`dind`サービスに対して、カスタムDNS設定を手動で指定してください。たとえば、カスタムDNSサーバーが`1.1.1.1`の場合、Dockerの内部DNSサービスである`127.0.0.11`を使用できます。

```yaml
  services:
    - name: docker:dind
      command: [--dns=127.0.0.11, --dns=1.1.1.1]
```

このアプローチでは、コンテナが同じネットワーク上のサービスを解決できるようになります。

{{< /alert >}}

#### Runnerがジョブごとにネットワークを作成する仕組み {#how-the-runner-creates-a-network-for-each-job}

ジョブが開始されると、Runnerは次の処理を行います。

1. Dockerコマンド`docker network create <network>`と同様に、ブリッジネットワークを作成します。
1. サービスとコンテナをブリッジネットワークに接続します。
1. ジョブの最後にネットワークを削除します。

ジョブを実行しているコンテナと、サービスを実行しているコンテナが、互いのホスト名とエイリアスを解決します。この機能は[Dockerによって提供](https://docs.docker.com/engine/network/drivers/bridge/#differences-between-user-defined-bridges-and-the-default-bridge)されます。

### コンテナリンクを使用してネットワークを設定する {#configure-a-network-with-container-links}

GitLab Runner 18.7.0以前は、デフォルトのDocker `bridge`と[レガシーコンテナリンク](https://docs.docker.com/engine/network/links/)を使用して、ジョブコンテナとサービスをリンクしていました。Dockerはリンク機能を非推奨にしたため、GitLab Runner 18.7.0以降では、サービスのエイリアスがDockerの`extra_hosts`機能を使用して解決されるようにすることで、レガシーコンテナリンクの動作がエミュレートされます。このネットワークモードは、[`FF_NETWORK_PER_BUILD`](#create-a-network-for-each-job)が無効になっている場合のデフォルトです。

{{< alert type="note" >}}

GitLab Runnerのエミュレートされたリンクの動作は、[レガシーコンテナリンク](https://docs.docker.com/engine/network/links/)とはわずかに異なります:

- `icc`を無効にすると、コンテナ間通信が無効になり、コンテナが相互に通信できなくなります。
- リンクされたコンテナの環境変数は存在しなくなりました（`<name>_PORT_<port>_<protocol>`）。

{{< /alert >}}

ネットワークを設定するには、`config.toml`ファイルで[ネットワーキング](https://docs.docker.com/engine/containers/run/#network-settings)モードを指定します。

- `bridge`: ブリッジネットワークを使用します。デフォルト。
- `host`: コンテナ内でホストのネットワークスタックを使用します。
- `none`: ネットワーキングなし。推奨されません。

例: 

```toml
[[runners]]
  (...)
  executor = "docker"
[runners.docker]
  network_mode = "bridge"
```

他の`network_mode`値を使用すると、ビルドコンテナが接続する既存のDockerネットワークの名前として扱われます。

Dockerは名前の解決中にサービスコンテナのホスト名とエイリアスを使用して、コンテナ内の`/etc/hosts`ファイルを更新します。ただし、サービスコンテナはコンテナ名を解決**できません**。コンテナ名を解決するには、ジョブごとにネットワークを作成する必要があります。

リンクされたコンテナは、その環境変数を共有します。

#### 作成されたネットワークのMTUを上書きする {#overriding-the-mtu-of-the-created-network}

OpenStackの仮想マシンなどの一部の環境では、カスタムMTUが必要です。Dockerデーモンは、`docker.json`のMTUに従いません（[Mobyイシュー34981](https://github.com/moby/moby/issues/34981)を参照）。Dockerデーモンが新しく作成されたネットワークに正しいMTUを使用できるようにするために、`config.toml`で`network_mtu`を有効な値に設定できます。上書きを有効にするには、[`FF_NETWORK_PER_BUILD`](#create-a-network-for-each-job)も有効にする必要があります。

次の設定では、各ジョブ用に作成されたネットワークのMTUが`1402`に設定されます。この値は、特定の環境要件に合わせて調整してください。

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    network_mtu = 1402
    [runners.feature_flags]
      FF_NETWORK_PER_BUILD = true
```

## Dockerイメージとサービスを制限する {#restrict-docker-images-and-services}

Dockerイメージとサービスを制限するには、`allowed_images`および`allowed_services`パラメータでワイルドカードパターンを指定します。構文の詳細については、[doublestarのドキュメント](https://github.com/bmatcuk/doublestar)を参照してください。

たとえば、プライベートDockerレジストリのイメージのみを許可するには、次のようにします。

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    allowed_images = ["my.registry.tld:5000/*:*"]
    allowed_services = ["my.registry.tld:5000/*:*"]
```

プライベートDockerレジストリのイメージのリストに制限するには、次のようにします。

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    allowed_images = ["my.registry.tld:5000/ruby:*", "my.registry.tld:5000/node:*"]
    allowed_services = ["postgres:9.4", "postgres:latest"]
```

Kaliなどの特定のイメージを除外するには、次のようにします。

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    allowed_images = ["**", "!*/kali*"]
```

## サービスホスト名にアクセスする {#access-services-hostnames}

サービスホスト名にアクセスするには、`.gitlab-ci.yml`で`services`にサービスを追加します。

たとえば、Wordpressインスタンスを使用してアプリケーションとのAPIインテグレーションをテストするには、[tutum/wordpress](https://hub.docker.com/r/tutum/wordpress/)をサービスイメージとして使用します。

```yaml
services:
- tutum/wordpress:latest
```

ジョブの実行時に`tutum/wordpress`サービスが開始されます。ホスト名`tutum__wordpress`および`tutum-wordpress`の下のビルドコンテナからこのサービスにアクセスできます。

指定されたサービスエイリアスの他に、Runnerはサービスイメージの名前をエイリアスとしてサービスコンテナに割り当てます。これらのエイリアスはどれでも使用できます。

Runnerは以下のルールに従って、イメージ名に基づいてエイリアスを作成します。

- `:`より後のすべての文字が削除されます。
- 1番目のエイリアスでは、スラッシュ（`/`）が2つのアンダースコア（`__`）に置き換えられます。
- 2番目のエイリアスでは、スラッシュ（`/`）が1つのダッシュ（`-`）に置き換えられます。

プライベートサービスイメージを使用する場合、Runnerは指定されたポートをすべて削除し、ルールを適用します。サービス`registry.gitlab-wp.com:4999/tutum/wordpress`の場合、ホスト名は`registry.gitlab-wp.com__tutum__wordpress`および`registry.gitlab-wp.com-tutum-wordpress`になります。

## サービスを設定する {#configuring-services}

データベース名を変更する場合、またはアカウント名を設定する場合には、サービスに環境変数を定義します。

Runnerが変数を渡すときには、次のように渡されます。

- 変数はすべてのコンテナに渡されます。Runnerは、特定のコンテナに変数を渡すことができません。
- セキュア変数はビルドコンテナに渡されます。

設定変数の詳細については、対応するDocker Hubページで提供される各イメージのドキュメントを参照してください。

### RAMにディレクトリをマウントする {#mount-a-directory-in-ram}

`tmpfs`オプションを使用して、RAMにディレクトリをマウントできます。これにより、データベースなどのI/O関連の処理が多い場合にテストに必要な時間を短縮できます。

Runner設定で`tmpfs`オプションと`services_tmpfs`オプションを使用する場合は、複数のパスをそれぞれ専用のオプションで指定できます。詳細については、[Dockerのドキュメント](https://docs.docker.com/reference/cli/docker/container/run/#tmpfs)を参照してください。

たとえば、公式のMySQLコンテナのデータディレクトリをRAMにマウントするには、`config.toml`を設定します。

```toml
[runners.docker]
  # For the main container
  [runners.docker.tmpfs]
      "/var/lib/mysql" = "rw,noexec"

  # For services
  [runners.docker.services_tmpfs]
      "/var/lib/mysql" = "rw,noexec"
```

### サービスでディレクトリをビルドする {#building-a-directory-in-a-service}

GitLab Runnerは、すべての共有サービスに`/builds`ディレクトリをマウントします。

さまざまなサービスの使用法の詳細については、以下を参照してください。

- [PostgreSQLを使用する](https://docs.gitlab.com/ci/services/postgres/)
- [MySQLを使用する](https://docs.gitlab.com/ci/services/mysql/)

### GitLab Runnerがサービスのヘルスチェックを実行する仕組み {#how-gitlab-runner-performs-the-services-health-check}

{{< history >}}

- GitLab 16.0で複数のポートチェックが[導入](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4079)されました。

{{< /history >}}

サービスの開始後、GitLab Runnerはサービスが応答するまで待機します。Docker executorは、サービスコンテナで公開されているサービスポートへのTCP接続を開こうとします。

- GitLab 15.11以前では、最初に公開されたポートのみがチェックされます。
- GitLab 16.0以降では、最初に公開された20個のポートがチェックされます。

特定のポートでヘルスチェックを実行するには、`HEALTHCHECK_TCP_PORT`サービス変数を使用できます。

```yaml
job:
  services:
    - name: mongo
      variables:
        HEALTHCHECK_TCP_PORT: "27017"
```

これがどのように実装されているかを確認するには、ヘルスチェックの[Goコマンド](https://gitlab.com/gitlab-org/gitlab-runner/blob/main/commands/helpers/health_check.go)を使用します。

## Dockerドライバーオペレーションを指定する {#specify-docker-driver-operations}

ビルドのボリュームを作成するときにDockerボリュームドライバーに渡す引数を指定します。たとえば、他のすべてのドライバー固有のオプションに加えて、これらの引数を使用して、各ビルドが実行されるスペースを制限できます。次の例は、各ビルドが消費できるスペースの制限が50 GBに設定されている`config.toml`を示しています。

```toml
[runners.docker]
  [runners.docker.volume_driver_ops]
      "size" = "50G"
```

## ホストデバイスを使用する {#using-host-devices}

{{< history >}}

- GitLab 17.10で[導入](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/6208)されました。

{{< /history >}}

GitLab Runnerホスト上のハードウェアデバイスを、ジョブを実行するコンテナに対して公開できます。このためには、Runnerの`devices`オプションと`services_devices`オプションを設定します。

- デバイスを`build`コンテナと[ヘルパー](../configuration/advanced-configuration.md#helper-image)コンテナに公開するには、`devices`オプションを使用します。
- デバイスをサービスコンテナに公開するには、`services_devices`オプションを使用します。サービスコンテナのデバイスアクセスを特定のイメージに制限するには、正確なイメージ名またはglobパターンを使用します。このアクションにより、ホストシステムデバイスへの直接アクセスが防止されます。

デバイスアクセスの詳細については、[Dockerのドキュメント](https://docs.docker.com/reference/cli/docker/container/run/#device)を参照してください。

### ビルドコンテナの例 {#build-container-example}

この例では、`config.toml`セクションで`/dev/bus/usb`をビルドコンテナに公開します。この設定により、パイプラインはホストマシンに接続されたUSBデバイス（[Android Debug Bridge（`adb`）](https://developer.android.com/tools/adb)を介して制御されるAndroidスマートフォンなど）にアクセスできます。

ビルドジョブコンテナがホストUSBデバイスに直接アクセスできるため、同じハードウェアにアクセスすると、同時パイプライン実行が互いに競合する可能性があります。このような競合を防ぐには、[`resource_group`](https://docs.gitlab.com/ci/yaml/#resource_group)を使用します。

```toml
[[runners]]
  name = "hardware-runner"
  url = "https://gitlab.com"
  token = "__REDACTED__"
  executor = "docker"
  [runners.docker]
    # All job containers may access the host device
    devices = ["/dev/bus/usb"]
```

### プライベートレジストリの例 {#private-registry-example}

この例は、プライベートDockerレジストリから`/dev/kvm`デバイスと`/dev/dri`デバイスをコンテナイメージに公開する方法を示します。これらのデバイスは通常、ハードウェアアクセラレーションによる仮想化とレンダリングに使用されます。ハードウェアリソースへの直接アクセスをユーザーに付与することに伴うリスクを軽減するには、デバイスアクセスを、`myregistry:5000/emulator/*`ネームスペース内の信頼できるイメージに制限します。

```toml
[runners.docker]
  [runners.docker.services_devices]
    # Only images from an internal registry may access the host devices
    "myregistry:5000/emulator/*" = ["/dev/kvm", "/dev/dri"]
```

{{< alert type="warning" >}}

イメージ名`**/*`は、任意のイメージにデバイスを公開する可能性があります。

{{< /alert >}}

## コンテナのビルドとキャッシュ用のディレクトリを設定する {#configure-directories-for-the-container-build-and-cache}

コンテナ内でデータが保存される場所を定義するには、`config.toml`の`[[runners]]`セクションで`/builds`ディレクトリと`/cache`ディレクトリを設定します。

`/cache`ストレージパスを変更する場合は、パスを永続としてマークするために、`config.toml`の`[runners.docker]`セクションで`volumes = ["/my/cache/"]`にこのパスを定義する必要があります。

デフォルトでは、Docker executorは次のディレクトリにビルドとキャッシュを保存します。

- ビルド: `/builds/<namespace>/<project-name>`
- キャッシュ: コンテナ内の`/cache`

## Dockerキャッシュをクリアする {#clear-the-docker-cache}

Runnerによって作成された未使用のコンテナとボリュームを削除するには、[`clear-docker-cache`](https://gitlab.com/gitlab-org/gitlab-runner/blob/main/packaging/root/usr/share/gitlab-runner/clear-docker-cache)を使用します。

オプションのリストを確認するには、`help`オプションを指定してスクリプトを実行します。

```shell
clear-docker-cache help
```

デフォルトのオプションは`prune-volumes`です。これにより、未使用のコンテナ（ダングリングおよび未参照）とボリュームがすべて削除されます。

キャッシュストレージを効率的に管理するには、次の操作を行う必要があります。

- `cron`を使用して`clear-docker-cache`を定期的に実行します（たとえば週に1回）。
- ディスクスペースを回収する際に、パフォーマンスのためにキャッシュに最近のコンテナをいくつか保持します。

どのオブジェクトが削除されるかは`FILTER_FLAG`環境変数によって制御されます。その使用例については、[Docker imageプルーニング](https://docs.docker.com/reference/cli/docker/image/prune/#filter)のドキュメントを参照してください。

## Dockerビルドイメージをクリアする {#clear-docker-build-images}

DockerイメージはGitLab Runnerによってタグ付けされていないため、[`clear-docker-cache`](https://gitlab.com/gitlab-org/gitlab-runner/blob/main/packaging/root/usr/share/gitlab-runner/clear-docker-cache)スクリプトはDockerイメージを削除しません。

Dockerビルドイメージをクリアするには、次の手順に従います。

1. 回収できるディスクスペースを確認します。

   ```shell
   clear-docker-cache space

   Show docker disk usage
   ----------------------

   TYPE            TOTAL     ACTIVE    SIZE      RECLAIMABLE
   Images          14        9         1.306GB   545.8MB (41%)
   Containers      19        18        115kB     0B (0%)
   Local Volumes   0         0         0B        0B
   Build Cache     0         0         0B        0B
   ```

1. 未使用のコンテナ、ネットワーク、イメージ（ダングリングおよび未参照）、およびタグ付けされていないボリュームをすべて削除するには、[`docker system prune`](https://docs.docker.com/reference/cli/docker/system/prune/)を実行します。

## 永続ストレージ {#persistent-storage}

Docker executorは、コンテナの実行時に永続ストレージを提供します。`volumes =`で定義されているすべてのディレクトリは、ビルド間で維持されます。

`volumes`ディレクティブは、次の種類のストレージをサポートしています。

- 動的ストレージの場合は`<path>`を使用します。`<path>`は、そのプロジェクトで同じ同時実行ジョブの後続の実行間で維持されます。`runners.docker.cache_dir`を設定しない場合、データはDockerボリュームに永続的に保存されます。そうでない場合は、ホスト上の設定されたディレクトリに永続的に保存されます（ビルドコンテナにマウントされます）。

  ボリュームベースの永続ストレージのボリューム名:

  - 18.4.0より以前のGitLab Runnerの場合: `runner-<short-token>-project-<project-id>-concurrent-<concurrency-id>-cache-<md5-of-path>`
  - GitLab Runner 18.4.0以降の場合: `runner-<runner-id-hash>-cache-<md5-of-path><protection>`

    ボリューム名で人間が読めなくなったデータは、ボリュームのラベルに移動されます。

  ホストベースの永続ストレージのホストディレクトリ:

  - 18.4.0より以前のGitLab Runnerの場合: `<cache-dir>/runner-<short-token>-project-<project-id>-concurrent-<concurrency-id>/<md5-of-path>`
  - GitLab Runner 18.4.0以降の場合: `<cache-dir>/runner-<runner-id-hash>/<md5-of-path><protection>`

  変数部分の説明:

  - `<short-token>`: Runnerのトークンの短縮バージョン（最初の8文字）
  - `<project-id>`: GitLabプロジェクトのID
  - `<concurrency-id>`: Runnerのインデックス（同じプロジェクトのビルドを同時に実行しているすべてのRunnerのリストから）
  - `<md5-of-path>`: コンテナ内のパスのMD5サム
  - `<runner-id-hash>`: 次のデータのハッシュ:
    - Runnerのトークン
    - RunnerのシステムID
    - `<project-id>`
    - `<concurrency-id>`
  - `<protection>`: 値は、保護されていないブランチのビルドの場合は空で、保護されたブランチのビルドの場合は`-protected`です
  - `<cache-dir>`: `runners.docker.cache_dir`の設定

- ホストにバインドされたストレージの場合は、`<host-path>:<path>[:<mode>]`を使用します。GitLab Runnerは、ホストシステムの`<host-path>`に`<path>`をバインドします。オプションの`<mode>`は、このストレージが読み取り専用か読み取り/書き込み（デフォルト）かを指定します。

{{< alert type="warning" >}}

GitLab Runner 18.4.0では、動的ストレージのソースの命名（上記参照）が、Dockerボリュームベースおよびホストディレクトリベースの永続ストレージの両方で変更されました。18.4.0にアップグレードすると、GitLab Runnerは以前のRunnerバージョンのキャッシュされたデータを無視し、新しいDockerボリュームまたは新しいホストディレクトリを介して、オンデマンドで新しい動的ストレージを作成します。

動的ストレージとは対照的に、ホストバインドストレージ（`<host-path>`設定を使用）は影響を受けません。

{{< /alert >}}

### ビルド用の永続ストレージ {#persistent-storage-for-builds}

`/builds`ディレクトリをホストにバインドされたストレージにすると、ビルドは`/builds/<short-token>/<concurrent-id>/<namespace>/<project-name>`に保存されます。

- `<short-token>`は、Runnerのトークンの短縮バージョンです（最初の8文字）。
- `<concurrent-id>`は、プロジェクトのコンテキストで特定のRunnerのローカルジョブIDを識別する一意の番号です。

## IPCモード {#ipc-mode}

Docker executorでは、コンテナのIPCネームスペースを他の場所と共有できます。これは`docker run --ipc`フラグにマップされます。IPC設定の詳細については、[Dockerのドキュメント](https://docs.docker.com/engine/containers/run/#ipc-settings---ipc)を参照してください。

## 特権モード {#privileged-mode}

Docker executorは、ビルドコンテナのファインチューニングを可能にするさまざまなオプションをサポートしています。このようなオプションの1つが[`privileged`モード](https://docs.docker.com/engine/containers/run/#runtime-privilege-and-linux-capabilities)です。

### 特権モードでDocker-in-Dockerを使用する {#use-docker-in-docker-with-privileged-mode}

設定された`privileged`フラグがビルドコンテナとすべてのサービスに渡されます。このフラグを使用すると、Docker-in-Dockerアプローチを使用できます。

まず、`privileged`モードで実行するようにRunner（`config.toml`）を設定します。

```toml
[[runners]]
  executor = "docker"
  [runners.docker]
    privileged = true
```

次に、Docker-in-Dockerコンテナを使用するためのビルドスクリプト（`.gitlab-ci.yml`）を作成します。

```yaml
image: docker:git
services:
- docker:dind

build:
  script:
  - docker build -t my-image .
  - docker push my-image
```

{{< alert type="warning" >}}

特権モードで実行されるコンテナには、セキュリティ上のリスクがあります。コンテナが特権モードで実行されている場合、コンテナセキュリティメカニズムを無効にし、ホストを特権エスカレーションに公開します。特権モードでコンテナを実行すると、コンテナのブレイクアウトが発生する可能性があります。詳細については、[ランタイム特権とLinux機能](https://docs.docker.com/engine/containers/run/#runtime-privilege-and-linux-capabilities)に関するDockerドキュメントを参照してください。

{{< /alert >}}

次のようなエラーを回避するには、[TLSを使用してDocker-in-Dockerを設定するか、またはTLSを無効にする](https://docs.gitlab.com/ci/docker/using_docker_build/#use-the-docker-executor-with-docker-in-docker)必要があります。

```plaintext
Cannot connect to the Docker daemon at tcp://docker:2375. Is the docker daemon running?
```

### 制限付き特権モードでルートレスDocker-in-Dockerを使用する {#use-rootless-docker-in-docker-with-restricted-privileged-mode}

このバージョンではDocker-in-Dockerルートレスイメージのみを特権モードでサービスとして実行できます。

`services_privileged`および`allowed_privileged_services`設定パラメータは、特権モードで実行できるコンテナを制限します。

制限付き特権モードでルートレスDocker-in-Dockerを使用するには、次の手順に従います。

1. `config.toml`で、`services_privileged`と`allowed_privileged_services`を使用するようにRunnerを設定します。

   ```toml
   [[runners]]
     executor = "docker"
     [runners.docker]
       services_privileged = true
       allowed_privileged_services = ["docker.io/library/docker:*-dind-rootless", "docker.io/library/docker:dind-rootless", "docker:*-dind-rootless", "docker:dind-rootless"]
   ```

1. `.gitlab-ci.yml`で、Docker-in-Dockerルートなしコンテナを使用するようにビルドスクリプトを編集します。

   ```yaml
   image: docker:git
   services:
   - docker:dind-rootless

   build:
     script:
     - docker build -t my-image .
     - docker push my-image
   ```

特権モードで実行できるのは、`allowed_privileged_services`にリストされているDocker-in-Dockerルートレスイメージのみです。ジョブとサービスのその他のコンテナはすべて、非特権モードで実行されます。

これらは非ルートとして実行されるため、Docker-in-DockerルートレスやBuildKitルートレスなどの特権モードのイメージとともに使用することは_ほぼ安全です_。

セキュリティの問題の詳細については、[Docker executorのセキュリティリスク](../security/_index.md#usage-of-docker-executor)を参照してください。

## Docker ENTRYPOINTを設定する {#configure-a-docker-entrypoint}

デフォルトの場合、Docker executorは[Dockerイメージの`ENTRYPOINT`](https://docs.docker.com/engine/containers/run/#entrypoint-default-command-to-execute-at-runtime)をオーバーライドしません。ジョブスクリプトを実行するコンテナを起動するために、`sh`または`bash`を[`COMMAND`](https://docs.docker.com/engine/containers/run/#cmd-default-command-or-options)として渡します。

ジョブを実行できるようにするには、そのDockerイメージが次の処理を行う必要があります。

- `sh`または`bash`と`grep`を提供する。
- 引数として`sh`/`bash`が渡されるとShellを起動する`ENTRYPOINT`を定義する。

Docker Executorは、次のコマンドと同等のコマンドでジョブのコンテナを実行します。

```shell
docker run <image> sh -c "echo 'It works!'" # or bash
```

Dockerイメージがこのメカニズムをサポートしていない場合は、プロジェクト設定で次のように[イメージのENTRYPOINTをオーバーライドできます](https://docs.gitlab.com/ci/yaml/#imageentrypoint)。

```yaml
# Equivalent of
# docker run --entrypoint "" <image> sh -c "echo 'It works!'"
image:
  name: my-image
  entrypoint: [""]
```

詳細については、[イメージのエントリポイントをオーバーライドする](https://docs.gitlab.com/ci/docker/using_docker_images/#override-the-entrypoint-of-an-image)と[Dockerでの`CMD`と`ENTRYPOINT`の相互作用の仕組み](https://docs.docker.com/reference/dockerfile/#understand-how-cmd-and-entrypoint-interact)を参照してください。

### ENTRYPOINTとしてのジョブスクリプト {#job-script-as-entrypoint}

`ENTRYPOINT`を使用して、カスタム環境またはセキュアモードでビルドスクリプトを実行するDockerイメージを作成できます。

たとえば、ビルドスクリプトを実行しない`ENTRYPOINT`を使用するDockerイメージを作成できます。代わりにDockerイメージは、定義済みの一連のコマンドを実行して、ディレクトリからDockerイメージをビルドします。[特権モード](#privileged-mode)でビルドコンテナを実行し、Runnerのビルド環境を保護します。

1. 新しいDockerfileを作成します。

   ```dockerfile
   FROM docker:dind
   ADD / /entrypoint.sh
   ENTRYPOINT ["/bin/sh", "/entrypoint.sh"]
   ```

1. `ENTRYPOINT`として使用されるbashスクリプト（`entrypoint.sh`）を作成します。

   ```shell
   #!/bin/sh

   dind docker daemon
       --host=unix:///var/run/docker.sock \
       --host=tcp://0.0.0.0:2375 \
       --storage-driver=vf &

   docker build -t "$BUILD_IMAGE" .
   docker push "$BUILD_IMAGE"
   ```

1. イメージをDockerレジストリにプッシュします。

1. `privileged`モードでDocker executorを実行します。`config.toml`で次のように定義します。

   ```toml
   [[runners]]
     executor = "docker"
     [runners.docker]
       privileged = true
   ```

1. プロジェクトで次の`.gitlab-ci.yml`を使用します。

   ```yaml
   variables:
     BUILD_IMAGE: my.image
   build:
     image: my/docker-build:image
     script:
     - Dummy Script
   ```

## Podmanを使用してDockerコマンドを実行する {#use-podman-to-run-docker-commands}

{{< history >}}

- GitLab 15.3で[導入](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27119)されました。

{{< /history >}}

LinuxにGitLab Runnerがインストールされている場合、ジョブはPodmanを使用して、DockerをDocker executorのコンテナランタイムに置き換えることができます。

前提条件: 

- [Podman](https://podman.io/) v4.2.0以降。
- Podmanをexecutorとして使用して[サービス](#services)を実行するには、[`FF_NETWORK_PER_BUILD`機能フラグ](#create-a-network-for-each-job)を有効にします。[Dockerコンテナリンク](https://docs.docker.com/engine/network/links/)はレガシー機能であり、[Podman](https://podman.io/)ではサポートされていません。ネットワークエイリアスを作成するサービスの場合、`podman-plugins`パッケージをインストールする必要があります。

{{< alert type="note" >}}

Podmanは、コンテナのDNSサーバーとして`aardvark-dns`を使用します。`aardvark-dns`バージョン1.10.0以前では、CI/CDジョブで散発的なDNS解決の失敗が発生します。新しいバージョンがインストールされていることを確認してください。詳細については、[GitHubイシュー389](https://github.com/containers/aardvark-dns/issues/389)を参照してください。

{{< /alert >}}

1. LinuxホストにGitLab Runnerをインストールします。システムのパッケージマネージャーを使用してGitLab Runnerをインストールした場合、`gitlab-runner`ユーザーが自動的に作成されます。
1. GitLab Runnerを実行するユーザーとしてサインインします。これは、[`pam_systemd`](https://www.freedesktop.org/software/systemd/man/latest/pam_systemd.html)を回避しない方法で行う必要があります。正しいユーザーでSSHを使用できます。これにより、このユーザーとして`systemctl`を実行できるようになります。
1. システムが、[ルートレスPodmanセットアップ](https://github.com/containers/podman/blob/main/docs/tutorials/rootless_tutorial.md)の前提条件を満たしていることを確認します。具体的には、[`/etc/subuid`および`/etc/subgid`にユーザーの正しいエントリがあること](https://github.com/containers/podman/blob/main/docs/tutorials/rootless_tutorial.md#etcsubuid-and-etcsubgid-configuration)を確認します。
1. Linuxホストに[Podmanをインストール](https://podman.io/getting-started/installation)します。
1. Podmanソケットを有効にして起動します。

   ```shell
   systemctl --user --now enable podman.socket
   ```

1. Podmanソケットがリッスンしていることを検証します。

   ```shell
   systemctl status --user podman.socket
   ```

1. Podman APIへのアクセスに使用されている`Listen`キーのソケット文字列をコピーします。
1. GitLab Runnerユーザーがログアウトした後も、Podmanソケットが利用可能な状態であることを確認します。

   ```shell
   sudo loginctl enable-linger gitlab-runner
   ```

1. GitLab Runnerの`config.toml`ファイルを編集し、`[runners.docker]`セクションのhostエントリにソケット値を追加します。例: 

   ```toml
   [[runners]]
     name = "podman-test-runner-2025-06-07"
     url = "https://gitlab.com"
     token = "TOKEN"
     executor = "docker"
     [runners.docker]
       host = "unix:///run/user/1012/podman/podman.sock"
       tls_verify = false
       image = "quay.io/podman/stable"
       privileged = false
   ```

   {{< alert type="note" >}}

   標準のPodmanを使用するには、`privileged = false`を設定します。ジョブ内で[Docker-in-Dockerサービス](#use-docker-in-docker-with-privileged-mode)を実行する必要がある場合にのみ、`privileged = true`を設定してください。

   {{< /alert >}}

### Podmanを使用してDockerfileからコンテナイメージをビルドする {#use-podman-to-build-container-images-from-a-dockerfile}

次の例では、Podmanを使用してコンテナイメージをビルドし、このイメージをGitLabコンテナレジストリにプッシュします。

Runnerの`config.toml`でデフォルトコンテナイメージが`quay.io/podman/stable`に設定されているため、CIジョブはそのイメージを使用して、含まれているコマンドを実行します。

```yaml
variables:
  IMAGE_TAG: $CI_REGISTRY_IMAGE:$CI_COMMIT_REF_SLUG

before_script:
  - podman login -u "$CI_REGISTRY_USER" -p "$CI_REGISTRY_PASSWORD" $CI_REGISTRY

oci-container-build:
  stage: build
  script:
    - podman build -t $IMAGE_TAG .
    - podman push $IMAGE_TAG
  when: manual
```

### Buildahを使用してDockerfileからコンテナイメージをビルドする {#use-buildah-to-build-container-images-from-a-dockerfile}

次の例は、Buildahを使用してコンテナイメージをビルドし、このイメージをGitLabコンテナレジストリにプッシュする方法を示しています。

```yaml
image: quay.io/buildah/stable

variables:
  IMAGE_TAG: $CI_REGISTRY_IMAGE:$CI_COMMIT_REF_SLUG

before_script:
  - buildah login -u "$CI_REGISTRY_USER" -p "$CI_REGISTRY_PASSWORD" $CI_REGISTRY

oci-container-build:
  stage: build
  script:
    - buildah bud -t $IMAGE_TAG .
    - buildah push $IMAGE_TAG
  when: manual
```

### 既知の問題 {#known-issues}

Dockerとは異なり、PodmanはデフォルトでSELinuxポリシーを適用します。多くのパイプラインは問題なく実行されますが、ツールが一時ディレクトリを使用すると、SELinuxコンテキストの継承により失敗する場合があります。

たとえば、次のパイプラインはPodmanでは失敗します:

```yaml
testing:
  image: alpine:3.20
  script:
    - apk add --no-cache python3 py3-pip
    - pip3 install --target $CI_PROJECT_DIR requests==2.28.2
```

pipが作業ディレクトリとして`/tmp`を使用するため、失敗が発生します。`/tmp`で作成されたファイルは、そのSELinuxコンテキストを継承します。これにより、コンテナは、これらのファイルが`$CI_PROJECT_DIR`に移動されたときに変更できなくなります。

**Solution:**`runners.docker`セクションの下のRunnerの`config.toml`のボリュームに`/tmp`を追加します:

```toml
[[runners]]
  [runners.docker]
    volumes = ["/cache", "/tmp"]
```

この追加により、マウントされたディレクトリ全体で一貫したSELinuxコンテキストが確保されます。

#### SELinuxのトラブルシューティング {#troubleshooting-selinux-issues}

その他のPodman/SELinuxの問題では、必要な設定の変更を特定するために、追加のトラブルシューティングが必要になる場合があります。

Podman Runnerの問題がSELinuxに関連しているかどうかをテストするには、`runners.docker`セクションの下のRunnerの`config.toml`に、次のディレクティブを一時的に追加します:

```toml
[[runners]]
  [runners.docker]
    security_opt = ["label:disable"]
```

{{< alert type="warning" >}}

この追加により、コンテナ内のSELinuxの適用がオフになります（これはDockerのデフォルトの動作です）。この設定はテスト目的でのみ使用し、セキュリティに影響を与える可能性があるため、永続的なソリューションとしては使用しないでください。

{{< /alert >}}

#### SELinux MCSの設定 {#configure-selinux-mcs}

SELinuxが一部の書き込み操作（既存のGitリポジトリの再初期化など）をブロックする場合は、Runnerによって起動されたすべてのコンテナでマルチカテゴリセキュリティ（MCS）を強制できます:

```toml
[[runners]]
  [runners.docker]
    security_opt = ["label=level:s0:c1000"]
```

このオプションではSELinuxは無効になりませんが、コンテナのMCSサービスレベル指標を設定します。このアプローチは、`label:disable`を使用するよりも安全です。

{{< alert type="warning" >}}

同じMCSカテゴリを使用する複数のコンテナは、そのカテゴリでタグ付けされた同じファイルにアクセスできます。

{{< /alert >}}

## ジョブを実行するユーザーを指定する {#specify-which-user-runs-the-job}

デフォルトでは、Runnerはコンテナ内の`root`ユーザーとしてジョブを実行します。ジョブを実行する別の非rootユーザーを指定するには、DockerイメージのDockerfileで`USER`ディレクティブを使用します。

```dockerfile
FROM amazonlinux
RUN ["yum", "install", "-y", "nginx"]
RUN ["useradd", "www"]
USER "www"
CMD ["/bin/bash"]
```

そのDockerイメージを使用してジョブを実行すると、指定されたユーザーとして実行されます。

```yaml
build:
  image: my/docker-build:image
  script:
  - whoami   # www
```

## Runnerがイメージをプルする方法を設定する {#configure-how-runners-pull-images}

RunnerがレジストリからDockerイメージをプルする方法を定義するには、`config.toml`でプルポリシーを設定します。1つのポリシー、[ポリシーのリスト](#set-multiple-pull-policies)、または[特定のプルポリシーを許可](#allow-docker-pull-policies)できます。

`pull_policy`には次の値を使用します。

- [`always`](#set-the-always-pull-policy): デフォルト。ローカルイメージが存在する場合でもイメージをプルします。このプルポリシーは、ディスクに既に存在する`SHA256`で指定されたイメージには適用されません。
- [`if-not-present`](#set-the-if-not-present-pull-policy): ローカルバージョンが存在しない場合にのみ、イメージをプルします。
- [`never`](#set-the-never-pull-policy): イメージをプルせずに、ローカルイメージのみを使用します。

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    pull_policy = "always" # available: always, if-not-present, never
```

### `always`プルポリシーを設定する {#set-the-always-pull-policy}

`always`オプションはデフォルトで有効になっており、常にコンテナの作成前にプルを開始します。このオプションにより、イメージが最新の状態になり、ローカルイメージが存在する場合でも古いイメージの使用を回避できます。

このプルポリシーは、次の場合に使用します。

- Runnerが常に最新のイメージをプルする必要がある。
- Runnerが公開されており、[オートスケール](../configuration/autoscale.md)向けに設定されているか、またはGitLabインスタンスのインスタンスRunnerとして設定されている。

Runnerがローカルに保存されているイメージを使用する必要がある場合は、このポリシーを**使用しないでください**。

`config.toml`で`always`を`pull policy`として設定します。

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    pull_policy = "always"
```

### `if-not-present`プルポリシーを設定する {#set-the-if-not-present-pull-policy}

プルポリシーを`if-not-present`に設定すると、Runnerは最初にローカルイメージが存在するかどうかを確認します。ローカルイメージがない場合、Runnerはレジストリからイメージをプルします。

`if-not-present`ポリシーは、次の場合に使用します。

- ローカルイメージを使用するが、ローカルイメージが存在しない場合はイメージをプルする。
- 負荷が高いイメージやほとんど更新されないイメージのイメージレイヤの差分をRunnerが分析する時間を短縮する。この場合、イメージの更新を強制的に実行するために、ローカルのDocker Engineストアから定期的に手動でイメージを削除する必要があります。

次の場合にはこのポリシーを**使用しないでください**。

- Runnerを使用するさまざまなユーザーがプライベートイメージにアクセスできるインスタンスRunnerの場合。セキュリティの問題の詳細については、[if-not-presentプルポリシーでのプライベートDockerイメージの使用](../security/_index.md#usage-of-private-docker-images-with-if-not-present-pull-policy)をご覧ください。
- ジョブが頻繁に更新され、最新のイメージバージョンでジョブを実行する必要がある場合。これにより実現するネットワーク負荷の軽減の価値は、ローカルイメージを頻繁に削除する価値を上回る可能性があります。

`config.toml`で`if-not-present`ポリシーを設定します。

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    pull_policy = "if-not-present"
```

### `never`プルポリシーを設定する {#set-the-never-pull-policy}

前提条件: 

- ローカルイメージには、インストール済みのDocker Engineと、使用されているイメージのローカルコピーが含まれている必要があります。

プルポリシーを`never`に設定すると、イメージのプルが無効になります。ユーザーはRunnerが実行されているDockerホストで、手動でプルされたイメージのみを使用できます。

次の場合に`never`プルポリシーを使用します。

- Runnerユーザーが使用するイメージを制御する場合。
- レジストリで公開されていない特定のイメージのみを使用できるプロジェクト専用のプライベートRunnerの場合。

[オートスケールされた](../configuration/autoscale.md)Docker executorには、`never`プルポリシーを**使用しないでください**。`never`プルポリシーは、選択したクラウドプロバイダーに定義済みのクラウドインスタンスイメージを使用する場合にのみ使用できます。

`config.toml`で`never`ポリシーを設定します。

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    pull_policy = "never"
```

### 複数のプルポリシーを設定する {#set-multiple-pull-policies}

プルが失敗した場合に実行する複数のプルポリシーをリストできます。Runnerは、プルが成功するか、リストされたポリシーがすべて処理されるまで、リストされた順にプルポリシーを処理します。たとえば、Runnerが`always`プルポリシーを使用している場合にレジストリが利用できない場合は、2番目のプルポリシーとして`if-not-present`を追加できます。この設定により、RunnerはローカルにキャッシュされているDockerイメージを使用できます。

このプルポリシーのセキュリティへの影響について詳しくは、[if-not-presentプルポリシーでのプライベートDockerイメージの使用](../security/_index.md#usage-of-private-docker-images-with-if-not-present-pull-policy)を参照してください。

複数のプルポリシーを設定するには、`config.toml`でプルポリシーをリストとして追加します。

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    pull_policy = ["always", "if-not-present"]
```

### Dockerプルポリシーを許可する {#allow-docker-pull-policies}

{{< history >}}

- GitLab 15.1で[導入](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/26753)されました。

{{< /history >}}

`.gitlab-ci.yml`ファイルでプルポリシーを指定できます。このポリシーは、CI/CDジョブがイメージをフェッチする方法を決定します。

`.gitlab-ci.yml`ファイルで指定されているものの中から使用できるプルポリシーを制限するには、`allowed_pull_policies`を使用します。

たとえば、`always`および`if-not-present`プルポリシーのみを許可するには、それらを`config.toml`に追加します。

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    allowed_pull_policies = ["always", "if-not-present"]
```

- `allowed_pull_policies`を指定しない場合、リストは`pull_policy`キーワードで指定された値と一致します。
- `pull_policy`を指定しない場合、デフォルトは`always`です。
- `pull_policy`と`allowed_pull_policies`の両方に含まれているプルポリシーだけがジョブによって使用されます。有効なプルポリシーは、[`pull_policy`キーワード](#configure-how-runners-pull-images)で指定されているポリシーを`allowed_pull_policies`と比較することによって決定されます。GitLabでは、これら2つのポリシーリストの[共通部分](https://en.wikipedia.org/wiki/Intersection_(set_theory))が使用されます。たとえば、`pull_policy`が`["always", "if-not-present"]`、`allowed_pull_policies`が`["if-not-present"]`の場合、ジョブでは、両方のリストで定義されている唯一のプルポリシーである`if-not-present`だけが使用されます。
- 既存の`pull_policy`キーワードには、`allowed_pull_policies`で指定されているプルポリシーが少なくとも1つ含まれている必要があります。`pull_policy`の値の中に`allowed_pull_policies`と一致するものがない場合、ジョブは失敗します。

### イメージのプルエラーメッセージ {#image-pull-error-messages}

| エラーメッセージ                                                                                                                                                                                                                                                               | 説明 |
|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-------------|
| `Pulling docker image registry.tld/my/image:latest ... ERROR: Build failed: Error: image registry.tld/my/image:latest not found`                                                                                                                                            | Runnerはイメージを見つけることができません。`always`プルポリシーが設定されている場合に表示されます。 |
| `Pulling docker image local_image:latest ... ERROR: Build failed: Error: image local_image:latest not found`                                                                                                                                                                | イメージがローカルでビルドされており、パブリックまたはデフォルトのDockerレジストリに存在していません。`always`プルポリシーが設定されている場合に表示されます。 |
| `Pulling docker image registry.tld/my/image:latest ... WARNING: Cannot pull the latest version of image registry.tld/my/image:latest : Error: image registry.tld/my/image:latest not found WARNING: Locally found image will be used instead.`                              | Runnerは、イメージをプルする代わりに、ローカルイメージを使用しました。 |
| `Pulling docker image local_image:latest ... ERROR: Build failed: Error: image local_image:latest not found`                                                                                                                                                                | イメージをローカルで見つけることができません。`never`プルポリシーが設定されている場合に表示されます。 |
| `WARNING: Failed to pull image with policy "always": Error response from daemon: received unexpected HTTP status: 502 Bad Gateway (docker.go:143:0s) Attempt #2: Trying "if-not-present" pull policy Using locally found image version due to "if-not-present" pull policy` | Runnerはイメージのプルに失敗し、次にリストされているプルポリシーを使用してイメージのプルを試行します。複数のプルポリシーが設定されている場合に表示されます。 |

## 失敗したプルを再試行する {#retry-a-failed-pull}

失敗したイメージのプルを再試行するようにRunnerを設定するには、`config.toml`で同じポリシーを複数回指定します。

たとえば次の設定では、プルを1回再試行します。

```toml
[runners.docker]
  pull_policy = ["always", "always"]
```

この設定は、個々のプロジェクトの`.gitlab-ci.yml`ファイルの[`retry`ディレクティブ](https://docs.gitlab.com/ci/yaml/#retry)と似ていますが、Dockerのプルが最初に失敗した場合にのみ有効になります。

## Windowsコンテナを使用する {#use-windows-containers}

Docker executorでWindowsコンテナを使用するには、制限事項、サポートされているWindowsバージョン、およびWindows Docker executorの設定に関する次の情報に注意してください。

### Nanoserverのサポート {#nanoserver-support}

Windowsヘルパーイメージで導入されたPowerShell Coreのサポートにより、ヘルパーイメージの`nanoserver`バリアントを利用できるようになりました。

### Windows上のDocker executorに関する既知のイシュー {#known-issues-with-docker-executor-on-windows}

以下は、Docker executorでWindowsコンテナを使用する場合の制限事項の一部です。

- Docker-in-DockerはDocker自体で[サポートされていない](https://github.com/docker-library/docker/issues/49)ため、サポートされていません。
- インタラクティブWebターミナルはサポートされていません。
- ホストデバイスのマウントはサポートされていません。
- ボリュームディレクトリをマウントする場合、ディレクトリが存在している必要があります。そうでない場合、Dockerはコンテナを起動できません。詳細については、[\#3754](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/3754)を参照してください。
- `docker-windows` executorは、Windowsで実行されているGitLab Runnerのみを使用して実行できます。
- [Windows上のLinuxコンテナ](https://learn.microsoft.com/en-us/virtualization/windowscontainers/deploy-containers/set-up-linux-containers)はまだ実験的機能であるため、サポートされていません。詳細については、[関連するイシュー](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4373)を確認してください。
- [Dockerでの制限](https://github.com/MicrosoftDocs/Virtualization-Documentation/pull/331)により、宛先パスのドライブ文字が`c:`ではない場合、以下ではパスがサポートされません。

  - [`builds_dir`](../configuration/advanced-configuration.md#the-runners-section)
  - [`cache_dir`](../configuration/advanced-configuration.md#the-runners-section)
  - [`volumes`](../configuration/advanced-configuration.md#volumes-in-the-runnersdocker-section)

  つまり、`f:\\cache_dir`などの値はサポートされていませんが、`f:`はサポートされています。ただし、宛先パスが`c:`ドライブ上にある場合は、パスもサポートされます（`c:\\cache_dir`など）。

  Dockerデーモンがイメージとコンテナを保持する場所を設定するには、Dockerデーモンの`daemon.json`ファイルで`data-root`パラメータを更新します。

  詳細については、[設定ファイルを使用してDockerを設定する](https://learn.microsoft.com/en-us/virtualization/windowscontainers/manage-docker/configure-docker-daemon#configure-docker-with-a-configuration-file)を参照してください。

### サポートされているWindowsバージョン {#supported-windows-versions}

GitLab Runnerは、[Windowsのサポートライフサイクル](../install/support-policy.md#windows-version-support)に従う次のバージョンのWindowsのみをサポートします。

- Windows Server 2022 LTSC（21H2）
- Windows Server 2019 LTSC（1809）

将来のWindows Serverバージョンについては、[将来のバージョンサポートポリシー](../install/support-policy.md#windows-version-support)があります。

Dockerデーモンが実行されているOSバージョンに基づいたコンテナのみを実行できます。たとえば、次の[`Windows Server Core`](https://hub.docker.com/r/microsoft/windows-servercore)イメージを使用できます。

- `mcr.microsoft.com/windows/servercore:ltsc2022`
- `mcr.microsoft.com/windows/servercore:ltsc2022-amd64`
- `mcr.microsoft.com/windows/servercore:1809`
- `mcr.microsoft.com/windows/servercore:1809-amd64`
- `mcr.microsoft.com/windows/servercore:ltsc2019`

### サポートされているDockerのバージョン {#supported-docker-versions}

GitLab RunnerはDockerを使用して、実行されているWindows Serverのバージョンを検出します。したがって、GitLab Runnerを実行しているWindows Serverで、最新バージョンのDockerが実行されている必要があります。

GitLab Runnerで機能しない既知のDockerのバージョンは`Docker 17.06`です。DockerはWindows Serverのバージョンを識別しないため、次のエラーが発生します。

```plaintext
unsupported Windows Version: Windows Server Datacenter
```

[この問題のトラブルシューティングの詳細については、こちらを参照してください](../install/windows.md#docker-executor-unsupported-windows-version)。

### Windows Docker executorを設定する {#configure-a-windows-docker-executor}

{{< alert type="note" >}}

ソースディレクトリとして`c:\\cache`を指定してRunnerが登録されている場合に`--docker-volumes`または`DOCKER_VOLUMES`環境変数を渡すときの[既知のイシュー](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4312)があります。

{{< /alert >}}

Windowsを実行しているDocker executorの設定の例を次に示します。

```toml
[[runners]]
  name = "windows-docker-2019"
  url = "https://gitlab.com/"
  token = "xxxxxxx"
  executor = "docker-windows"
  [runners.docker]
    image = "mcr.microsoft.com/windows/servercore:1809_amd64"
    volumes = ["c:\\cache"]
```

Docker executorのその他の設定オプションについては、[高度な設定](../configuration/advanced-configuration.md#the-runnersdocker-section)セクションを参照してください。

### サービス {#services}

[ジョブごとにネットワークを](#create-a-network-for-each-job)有効にすることによって、[サービス](https://docs.gitlab.com/ci/services/)を使用することができます。

## ネイティブステップRunnerインテグレーション {#native-step-runner-integration}

{{< history >}}

- GitLab 17.6.0で、機能フラグ`FF_USE_NATIVE_STEPS`により隠されている状態で[導入](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5069)されました。デフォルトでは無効になっています。
- GitLab 17.9.0で[更新](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5322)されました。GitLab Runnerは、`step-runner`バイナリをビルドコンテナに挿入し、それに合わせて`$PATH`環境変数を調整します。この拡張機能により、任意のイメージをビルドイメージとして使用できます。

{{< /history >}}

Docker executorは、[`step-runner`](https://gitlab.com/gitlab-org/step-runner)が提供する`gRPC` APIを使用して[CI/CDステップ](https://docs.gitlab.com/ci/steps/)をネイティブに実行することをサポートしています。

この実行モードを有効にするには、従来の`script`キーワードの代わりに`run`キーワードを使用してCI/CDジョブを指定する必要があります。さらに、`FF_USE_NATIVE_STEPS`機能フラグを有効にする必要があります。この機能フラグは、ジョブレベルまたはパイプラインレベルで有効にできます。

```yaml
step job:
  stage: test
  variables:
    FF_USE_NATIVE_STEPS: true
  image:
    name: alpine:latest
  run:
    - name: step1
      script: pwd
    - name: step2
      script: env
    - name: step3
      script: ls -Rlah ../
```

### 既知の問題 {#known-issues-1}

- GitLab 17.9以降では、ビルドイメージで`ca-certificates`パッケージがインストールされている必要があります。インストールされていないと、`step-runner`がジョブで定義されているステップのプルに失敗します。たとえば、DebianベースのLinuxディストリビューションは、デフォルトでは`ca-certificates`をインストールしません。

- 17.9より前のGitLabバージョンでは、ビルドイメージで`$PATH`に`step-runner`バイナリが含まれている必要があります。これを実現するには、次のいずれかを実行します。

  - 独自のカスタムビルドイメージを作成し、`step-runner`バイナリを含めます。
  - `registry.gitlab.com/gitlab-org/step-runner:v0`イメージに、ジョブの実行に必要な依存関係が含まれている場合は、このイメージを使用します。

- Dockerコンテナを実行するステップの実行は、従来の`scripts`と同じ設定パラメータと制約に従う必要があります。たとえば、[Docker-in-Docker](#use-docker-in-docker-with-privileged-mode)を使用する必要があります。
- この実行モードでは、[`Github Actions`](https://gitlab.com/components/action-runner)の実行はサポートされていません。
