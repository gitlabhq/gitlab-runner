---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: Fleeting
---

[Fleeting](https://gitlab.com/gitlab-org/fleeting/fleeting)は、クラウドプロバイダーのインスタンスグループに対して、プラグインベースの抽象化を提供する目的でRunnerが使用するライブラリです。

以下のexecutorは、RunnerをスケールするためにFleetingを使用します:

- [Docker Autoscaler](../executors/docker_autoscaler.md)
- [インスタンス](../executors/instance.md)

## Fleetingプラグインを検索 {#find-a-fleeting-plugin}

GitLabは、以下の公式プラグインを管理しています:

| クラウドプロバイダー                                                             | 備考 |
|----------------------------------------------------------------------------|-------|
| [Google Cloud](https://gitlab.com/gitlab-org/fleeting/plugins/googlecloud) | [Google Cloudインスタンスグループ](https://docs.cloud.google.com/compute/docs/instance-groups)を使用 |
| [AWS](https://gitlab.com/gitlab-org/fleeting/plugins/aws)                  | [AWS Auto Scaling groups](https://docs.aws.amazon.com/autoscaling/ec2/userguide/auto-scaling-groups.html)を使用 |
| [Azure](https://gitlab.com/gitlab-org/fleeting/plugins/azure)              | Azure [Virtual Machine Scale Sets](https://learn.microsoft.com/en-us/azure/virtual-machine-scale-sets/overview)を使用します。[Uniform orchestration](https://learn.microsoft.com/en-us/azure/virtual-machine-scale-sets/virtual-machine-scale-sets-orchestration-modes#scale-sets-with-uniform-orchestration)モードのみがサポートされています。 |

以下のプラグインは、コミュニティによって管理されています:

| クラウドプロバイダー | OCI参照 | 備考 |
|----------------|---------------|-------|
| [VMware vSphere](https://gitlab.com/santhanuv/fleeting-plugin-vmware-vsphere) | `registry.gitlab.com/santhanuv/fleeting-plugin-vmware-vsphere:latest` | VMware vSphereを使用して、既存のテンプレートからクローンを作成して仮想マシンを作成および管理します。[`govmomi vcsim`](https://github.com/vmware/govmomi/tree/main/vcsim)シミュレーターでテストされ、基本的なユースケースに対してコミュニティメンバーによって検証されています。制限されたvSphere権限では制限がある場合があります。[Fleeting Plugin VMware vSphere project](https://gitlab.com/santhanuv/fleeting-plugin-vmware-vsphere/-/issues)で関連するイシューを作成できます。|

コミュニティで管理されているプラグインは、GitLab（コミュニティ）外のコントリビューターが所有、構築、ホスト、および管理しています。GitLabは、FleetingライブラリとAPIを所有および管理して、静的なコードレビューを提供します。GitLabは、必要なコンピューティング環境すべてにアクセスできないため、コミュニティのプラグインをテストできません。コミュニティメンバーは、OCIリポジトリにプラグインをビルド、テスト、および公開し、このページでマージリクエストを介して参照を提供する必要があります。OCI参照には、イシューのレポート先、プラグインのサポートと安定性のレベル、およびドキュメントの場所に関する注記を添付する必要があります。

## Fleetingプラグインを構成 {#configure-a-fleeting-plugin}

Fleetingを構成するには、`config.toml`で、[`[runners.autoscaler]`](../configuration/advanced-configuration.md#the-runnersautoscaler-section)構成セクションを使用します。

{{< alert type="note" >}}

各プラグインのREADME.mdファイルには、インストールと設定に関する重要な情報が含まれています。

{{< /alert >}}

## フリートプラグインをインストールする {#install-a-fleeting-plugin}

Fleetingプラグインをインストールするには、次のいずれかを使用します:

- OCIレジストリ配信（推奨）
- 手動バイナリインストール

## OCIレジストリ配信でインストール {#install-with-the-oci-registry-distribution}

{{< history >}}

- [導入](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4690)OCIレジストリの配信（GitLab Runner 16.11内）

{{< /history >}}

プラグインは、UNIXシステムでは`~/.config/fleeting/plugins`に、Windowsでは`%APPDATA%/fleeting/plugins`にインストールされます。プラグインのインストール場所をオーバーライドするには、環境変数`FLEETING_PLUGIN_PATH`を更新します。

fleetingプラグインをインストールするには:

1. `config.toml`の`[runners.autoscaler]`セクションで、fleetingプラグインを追加します:

   {{< tabs >}}

   {{< tab title="AWS" >}}

   ```toml
   [[runners]]
     name = "my runner"
     url = "https://gitlab.com"
     token = "<token>"
     shell = "sh"

   executor = "instance"

   [runners.autoscaler]
     plugin = "aws:latest"
   ```

   {{< /tab >}}

   {{< tab title="Google Cloud" >}}

   ```toml
   [[runners]]
     name = "my runner"
     url = "https://gitlab.com"
     token = "<token>"
     shell = "sh"

   executor = "instance"

   [runners.autoscaler]
     plugin = "googlecloud:latest"
   ```

   {{< /tab >}}

   {{< tab title="Azure" >}}

   ```toml
   [[runners]]
     name = "my runner"
     url = "https://gitlab.com"
     token = "<token>"
     shell = "sh"

   executor = "instance"

   [runners.autoscaler]
     plugin = "azure:latest"
   ```

   {{< /tab >}}

   {{< /tabs >}}

1. `gitlab-runner fleeting install`を実行します。

### `plugin`形式 {#plugin-formats}

`plugin`パラメータは、次の形式をサポートします:

- `<name>`
- `<name>:<version constraint>`
- `<repository>/<name>`
- `<repository>/<name>:<version constraint>`
- `<registry>/<repository>/<name>`
- `<registry>/<repository>/<name>:<version constraint>`

各項目の説明: 

- `registry.gitlab.com`はデフォルトレジストリです。
- `gitlab-org/fleeting/plugins`はデフォルトリポジトリです。
- `latest`はデフォルトバージョンです。

### バージョン制約の形式 {#version-constraint-formats}

`gitlab-runner fleeting install`コマンドは、リモートリポジトリで最新の一致するバージョンを見つけるために、バージョン制約を使用します。

Runnerを実行すると、バージョン制約を使用して、ローカルにインストールされている最新の一致するバージョンが検索されます。

次のバージョン制約形式を使用します:

| 形式                    | 説明 |
|---------------------------|-------------|
| `latest`                  | 最新バージョン。 |
| `<MAJOR>`                 | メジャーバージョンを選択します。たとえば、`1`は、`1.*.*`と一致するバージョンを選択します。 |
| `<MAJOR>.<MINOR>`         | メジャーおよびマイナーバージョンを選択します。たとえば、`1.5`は、`1.5.*`と一致する最新バージョンを選択します。 |
| `<MAJOR>.<MINOR>.<PATCH>` | メジャー、マイナーバージョン、およびパッチを選択します。たとえば、`1.5.1`は、バージョン`1.5.1`を選択します。 |

## バイナリを手動でインストール {#install-binary-manually}

fleetingプラグインを手動でインストールするには:

1. システム用のfleetingプラグインバイナリをダウンロードします:
   - [AWS](https://gitlab.com/gitlab-org/fleeting/plugins/aws/-/releases)。
   - [Google Cloud](https://gitlab.com/gitlab-org/fleeting/plugins/googlecloud/-/releases)
   - [Azure](https://gitlab.com/gitlab-org/fleeting/plugins/azure/-/releases)
1. バイナリの名前が`fleeting-plugin-<name>`の形式であることを確認します。たとえば、`fleeting-plugin-aws`などです。
1. バイナリが`$PATH`から検出できることを確認します。たとえば、`/usr/local/bin`に移動します。
1. `config.toml`の`[runners.autoscaler]`セクションで、fleetingプラグインを追加します。例: 

   {{< tabs >}}

   {{< tab title="AWS" >}}

   ```toml
   [[runners]]
     name = "my runner"
     url = "https://gitlab.com"
     token = "<token>"
     shell = "sh"

   executor = "instance"

   [runners.autoscaler]
     plugin = "fleeting-plugin-aws"
   ```

   {{< /tab >}}

   {{< tab title="Google Cloud" >}}

   ```toml
   [[runners]]
     name = "my runner"
     url = "https://gitlab.com"
     token = "<token>"
     shell = "sh"

   executor = "instance"

   [runners.autoscaler]
     plugin = "fleeting-plugin-googlecloud"
   ```

   {{< /tab >}}

   {{< tab title="Azure" >}}

   ```toml
   [[runners]]
     name = "my runner"
     url = "https://gitlab.com"
     token = "<token>"
     shell = "sh"

   executor = "instance"

   [runners.autoscaler]
     plugin = "fleeting-plugin-azure"
   ```

   {{< /tab >}}

   {{< /tabs >}}

## Fleetingプラグインの管理 {#fleeting-plugin-management}

次の`fleeting`サブコマンドを使用して、fleetingプラグインを管理します:

| コマンド                          | 説明 |
|----------------------------------|-------------|
| `gitlab-runner fleeting install` | OCIレジストリ配信からfleetingプラグインをインストールします。 |
| `gitlab-runner fleeting list`    | 参照されているプラグインと使用されているバージョンを一覧表示します。 |
| `gitlab-runner fleeting login`   | プライベートレジストリにサインインします。 |
