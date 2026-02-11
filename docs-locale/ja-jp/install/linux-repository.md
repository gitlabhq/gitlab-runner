---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
description: パッケージマネージャーを使用して、GitLabリポジトリからGitLab Runnerをインストールします。
title: 公式のGitLabリポジトリを使用してGitLab Runnerをインストールする
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

GitLab Runnerをインストールするには、[GitLabリポジトリ](https://packages.gitlab.com/runner/gitlab-runner)のパッケージを使用できます。

## サポートされているディストリビューション {#supported-distributions}

GitLabでは、[Packagecloud](https://packages.gitlab.com/runner/gitlab-runner/)でサポートされている以下のバージョンのLinuxディストリビューションのパッケージを提供しています。新しいOSディストリビューションリリースに対応する新しいRunner `deb`または`rpm`パッケージは、Packagecloudでサポートされている場合、自動的に追加されます。

<!-- supported_os_versions_list_start -->

### Debベースのディストリビューション {#deb-based-distributions}

| ディストリビューション | サポート対象バージョン |
|--------------|--------------------|
| Debian | 15 Duke、14 Forky、13 Trixie、12 Bookworm、11 Bullseye |
| LinuxMint | 22.1 Xia、22 Wilma、21.3 Virginia、21.2 Victoria、21.1 Vera、21 Vanessa |
| Raspbian | 15 Duke、14 Forky、13 Trixie、12 Bookworm、11 Bullseye |
| Ubuntu | 25.04 Plucky Puffin、24.04 Lts Noble Numbat、22.04 Jammy Jellyfish、20.04 Focal Fossa、18.04 Lts Bionic Beaver、16.04 Lts Xenial Xerus |

### RPMベースのディストリビューション {#rpm-based-distributions}

| ディストリビューション | サポート対象バージョン |
|--------------|--------------------|
| Amazon Linux | 2025、2023、2022、2 |
| Red Hat Enterprise Linux | 10、9、8、7 |
| Fedora | 43, 42 |
| Oracle Linux | 10、9、8、7 |
| openSUSE | 16.0、15.6 |
| SUSE Linux Enterprise Server | 15.7、15.6、15.5、15.4、12.5 |

<!-- supported_os_versions_list_end -->

セットアップによっては、他のDebianまたはRPMベースのディストリビューションもサポートされている場合があります。これは、サポートされているGitLab Runnerディストリビューションからの派生であり、互換性のあるパッケージリポジトリを持つディストリビューションを指します。たとえば、DeepinはDebianの派生ディストリビューションです。そのため、Runnerの`deb`パッケージはDeepinにインストールして実行できるはずです。他のLinuxディストリビューションでも[GitLab Runnerをバイナリとしてインストール](linux-manually.md#using-binary-file)できる場合があります。

{{< alert type="note" >}}

リストにないディストリビューションのパッケージは、パッケージリポジトリから入手できません。これらは、S3バケットからRPMまたはDEBパッケージをダウンロードして、手動で[インストール](linux-manually.md#using-debrpm-package)できます。

{{< /alert >}}

## GitLab Runnerをインストールする {#install-gitlab-runner}

GitLab Runnerをインストールするには、次の手順に従います。

1. 公式GitLabリポジトリを追加します。

   {{< tabs >}}

   {{< tab title="Debian/Ubuntu/Mint" >}}

   1. リポジトリ設定スクリプトをダウンロードします:

      ```shell
      curl -L "https://packages.gitlab.com/install/repositories/runner/gitlab-runner/script.deb.sh" -o script.deb.sh
      ```

   1. 実行する前にスクリプトを検査します:

      ```shell
      less script.deb.sh
      ```

   1. スクリプトを実行します:

      ```shell
      sudo bash script.deb.sh
      ```

   {{< /tab >}}

   {{< tab title="RHEL/CentOS/Fedora/Amazon Linux" >}}

   1. リポジトリ設定スクリプトをダウンロードします:

      ```shell
      curl -L "https://packages.gitlab.com/install/repositories/runner/gitlab-runner/script.rpm.sh" -o script.rpm.sh
      ```

   1. 実行する前にスクリプトを検査します:

      ```shell
      less script.rpm.sh
      ```

   1. スクリプトを実行します:

      ```shell
      sudo bash script.rpm.sh
      ```

   {{< /tab >}}

   {{< /tabs >}}

1. 最新バージョンのGitLab Runnerをインストールするか、次のステップに進んで特定のバージョンをインストールします。

   {{< alert type="note" >}}

   [`No such file or directory`ジョブの失敗](#error-no-such-file-or-directory-job-failures)を防ぐために、`skel`ディレクトリの使用はデフォルトで無効になっています。

   {{< /alert >}}

   {{< tabs >}}

   {{< tab title="Debian/Ubuntu/Mint" >}}

   ```shell
   sudo apt install gitlab-runner
   ```

   {{< /tab >}}

   {{< tab title="RHEL/CentOS/Fedora/Amazon Linux" >}}

   ```shell
   sudo yum install gitlab-runner

   or

   sudo dnf install gitlab-runner
   ```

   {{< /tab >}}

   {{< /tabs >}}

   {{< alert type="note" >}}

   RHELディストリビューション向けに、FIPS 140-2準拠バージョンのGitLab Runnerが利用可能です。このバージョンをインストールするには、パッケージ名として`gitlab-runner`の代わりに`gitlab-runner-fips`を使用します。

   {{< /alert >}}

1. 特定のバージョンのGitLab Runnerをインストールするには、次のようにします。

   {{< tabs >}}

   {{< tab title="Debian/Ubuntu/Mint" >}}

   {{< alert type="note" >}}

   `gitlab-runner`バージョン`v17.7.1`の時点では、最新バージョンではない特定のバージョンの`gitlab-runner`をインストールする場合、そのバージョンに必要な`gitlab-runner-helper-packages`を明示的にインストールする必要があります。この要件は、`apt`/`apt-get`の制限により存在しています。

   {{< /alert >}}

   ```shell
   apt-cache madison gitlab-runner
   sudo apt install gitlab-runner=17.7.1-1 gitlab-runner-helper-images=17.7.1-1
   ```

   特定バージョンの`gitlab-runner`をインストールするときに、同じバージョンの`gitlab-runner-helper-images`をインストールしないと、次のようなエラーが発生する可能性があります。

   ```shell
   sudo apt install gitlab-runner=17.7.1-1
   ...
   The following packages have unmet dependencies:
    gitlab-runner : Depends: gitlab-runner-helper-images (= 17.7.1-1) but 17.8.3-1 is to be installed
   E: Unable to correct problems, you have held broken packages.
   ```

   {{< /tab >}}

   {{< tab title="RHEL/CentOS/Fedora/Amazon Linux" >}}

   ```shell
   yum list gitlab-runner --showduplicates | sort -r
   sudo yum install gitlab-runner-17.2.0-1
   ```

   {{< /tab >}}

   {{< /tabs >}}

1. [Runnerを登録します](../register/_index.md)。

上記の手順を完了すると、Runnerを起動してプロジェクトで使用できるようになります。

[FAQ](../faq/_index.md)セクションを参照してください。このセクションでは、GitLab Runnerに関する最も一般的な問題について説明しています。

## ヘルパーイメージパッケージ {#helper-images-package}

`gitlab-runner-helper-images`パッケージには、GitLab Runnerがジョブの実行中に使用する、構築済みのヘルパーコンテナイメージが含まれています。これらのイメージは、リポジトリのクローンを作成し、アーティファクトをアップロードし、キャッシュを管理するために必要なツールとユーティリティを提供します。

`gitlab-runner-helper-images`パッケージには、次のオペレーティングシステムとアーキテクチャ用のヘルパーイメージが含まれています:

Alpineベースのイメージ（最新）:

- `alpine-arm`
- `alpine-arm64`
- `alpine-riscv64`
- `alpine-s390x`
- `alpine-x86_64`
- `alpine-x86_64-pwsh`

Ubuntuベースのイメージ（24.04）:

- `ubuntu-arm`
- `ubuntu-arm64`
- `ubuntu-ppc64le`
- `ubuntu-s390x`
- `ubuntu-x86_64`
- `ubuntu-x86_64-pwsh`

### ヘルパーイメージの自動ダウンロード {#automatic-helper-image-download}

特定のオペレーティングシステムとアーキテクチャの組み合わせ用のヘルパーイメージがホストシステムで使用できない場合、GitLab Runnerは必要に応じて必要なイメージを自動的にダウンロードします。`gitlab-runner-helper-images package`に含まれていないアーキテクチャの場合、手動インストールは必要ありません。この自動ダウンロードにより、runnerは、手動での介入や個別のパッケージインストールを必要とせずに、追加のアーキテクチャ（`loong64`など）をサポートできます。

## GitLab Runnerをアップグレードする {#upgrade-gitlab-runner}

最新バージョンのGitLab Runnerをインストールするには、次のようにします。

{{< tabs >}}

{{< tab title="Debian/Ubuntu/Mint" >}}

```shell
sudo apt update
sudo apt install gitlab-runner
```

{{< /tab >}}

{{< tab title="RHEL/CentOS/Fedora/Amazon Linux" >}}

```shell
sudo yum update
sudo yum install gitlab-runner
```

{{< /tab >}}

{{< /tabs >}}

## PackageインストールのGPG署名 {#gpg-signatures-for-package-installation}

GitLab Runnerプロジェクトは、パッケージインストール方法に対して2種類のGPG署名を提供しています。

- [リポジトリメタデータの署名](#repository-metadata-signing)
- [パッケージの署名](#package-signing)

### リポジトリメタデータの署名 {#repository-metadata-signing}

リモートリポジトリからダウンロードしたパッケージ情報が信頼できるものであることを検証するために、パッケージマネージャーはリポジトリメタデータの署名を使用します。

この署名は、`apt-get update`などのコマンドを使用するときに検証されます。このため、**パッケージのダウンロードとインストールが行われる前に**、利用可能なパッケージに関する情報が更新されます。検証に失敗した場合、パッケージマネージャーはメタデータを拒否します。つまり、署名の不一致の原因となった問題が見つかって解決されるまで、リポジトリからパッケージをダウンロードしてインストールすることはできません。

パッケージメタデータ署名の検証に使用されるGPG公開キーは、上記の手順で最初に行われたインストール時に自動的にインストールされます。今後のキーの更新では、既存のユーザーが新しいキーを手動でダウンロードしてインストールする必要があります。

<https://packages.gitlab.com>でホストされているすべてのプロジェクトに対して1つのキーを使用します。使用されているキーの詳細は、[Linuxパッケージのドキュメント](https://docs.gitlab.com/omnibus/update/package_signatures/#package-repository-metadata-signing-keys)で確認できます。このドキュメントページには、[過去に使用されたすべてのキー](https://docs.gitlab.com/omnibus/update/package_signatures/#previous-keys)も記載されています。

### パッケージの署名 {#package-signing}

リポジトリメタデータの署名は、ダウンロードされたバージョン情報が<https://packages.gitlab.com>からのものであることを証明します。パッケージ自体の整合性を証明するものではありません。リポジトリからユーザーへのメタデータ転送が影響を受けていない限り、<https://packages.gitlab.com>にアップロードされたものはすべて、承認されているかどうかにかかわらず、適切に検証されます。

パッケージ署名では、各パッケージがそのビルド時に署名されます。ビルド環境と使用されているGPGキーの機密性を信頼できるようになるまで、パッケージの信頼性を検証できません。パッケージの有効な署名は、その出所が認証されており、その整合性が侵害されていないことを証明します。

パッケージ署名検証は、Debian/RPMベースのディストリビューションの一部でのみデフォルトで有効になっています。このタイプの検証を使用するには、設定の調整が必要になる場合があります。

<https://packages.gitlab.com>でホストされているリポジトリごとに、パッケージ署名検証に使用されるGPGキーが異なる場合があります。GitLab Runnerプロジェクトでは、このタイプの署名に独自のキーペアを使用します。

#### RPMベースのディストリビューション {#rpm-based-distributions-1}

RPM形式には、GPG署名機能の完全な実装が含まれており、この形式に基づくパッケージマネージャーと完全に統合されています。

[Linuxパッケージのドキュメント](https://docs.gitlab.com/omnibus/update/package_signatures/#rpm-based-distributions)に、RPMベースのディストリビューションのパッケージ署名検証を設定する方法に関する技術的な説明があります。GitLab Runnerでの違いは次のとおりです。

- インストールする必要がある公開キーパッケージの名前は`gpg-pubkey-35dfa027-60ba0235`です。
- RPMベースのディストリビューションのリポジトリファイルの名前は、`/etc/yum.repos.d/runner_gitlab-runner.repo`（安定版リリースの場合）または`/etc/yum.repos.d/runner_unstable.repo`（不安定版リリースの場合）です。
- [パッケージ署名公開キー](#current-gpg-public-key)は、`https://packages.gitlab.com/runner/gitlab-runner/gpgkey/runner-gitlab-runner-49F16C5CC3A0F81F.pub.gpg`からインポートできます。

#### Debianベースのディストリビューション {#debian-based-distributions}

`deb`形式は、公式にはパッケージ署名機能をデフォルトで備えていません。GitLab Runnerプロジェクトでは、パッケージの署名と検証に`dpkg-sig`ツールを使用します。この方法では、パッケージの手動検証のみがサポートされています。

`deb`パッケージを検証するには、次の手順に従います。

1. `dpkg-sig`をインストールします。

   ```shell
   apt update && apt install dpkg-sig
   ```

1. [パッケージ署名公開キー](#current-gpg-public-key)をダウンロードしてインポートします。

   ```shell
   curl -JLO "https://packages.gitlab.com/runner/gitlab-runner/gpgkey/runner-gitlab-runner-49F16C5CC3A0F81F.pub.gpg"
   gpg --import runner-gitlab-runner-49F16C5CC3A0F81F.pub.gpg
   ```

1. `dpkg-sig`でダウンロードしたパッケージを検証します。

   ```shell
   dpkg-sig --verify gitlab-runner_amd64.deb
   Processing gitlab-runner_amd64.deb...
   GOODSIG _gpgbuilder 931DA69CFA3AFEBBC97DAA8C6C57C29C6BA75A4E 1623755049
   ```

   パッケージの署名が無効であるか、無効なキー（失効したキーなど）で署名されている場合、出力は次のようになります。

   ```shell
   dpkg-sig --verify gitlab-runner_amd64.deb
   Processing gitlab-runner_amd64.deb...
   BADSIG _gpgbuilder
   ```

   キーがユーザーのキーリングに存在しない場合、出力は次のようになります。

   ```shell
   dpkg-sig --verify gitlab-runner_amd64.v13.1.0.deb
   Processing gitlab-runner_amd64.v13.1.0.deb...
   UNKNOWNSIG _gpgbuilder 880721D4
   ```

#### 現在のGPG公開キー {#current-gpg-public-key}

`https://packages.gitlab.com/runner/gitlab-runner/gpgkey/runner-gitlab-runner-49F16C5CC3A0F81F.pub.gpg`からパッケージ署名に使用される現在の公開GPGキーをダウンロードできます。

| キーの属性 | 値 |
|---------------|-------|
| 名前          | `GitLab, Inc.` |
| メール         | `support@gitlab.com` |
| フィンガープリント   | `931D A69C FA3A FEBB C97D  AA8C 6C57 C29C 6BA7 5A4E` |
| 有効期限        | `2026-04-28` |

{{< alert type="note" >}}

GitLab Runnerプロジェクトは、`<https://gitlab-runner-downloads.s3.dualstack.us-east-1.amazonaws.com>`バケットで利用可能なS3リリースの`release.sha256`ファイルに署名するために、同じキーを使用します。

{{< /alert >}}

#### 過去のGPG公開キー {#previous-gpg-public-keys}

過去に使用されたキーを以下の表に示します。

失効したキーは、パッケージ署名検証設定から削除することを強くお勧めします。

次のキーによって作成された署名は、信頼すべきではありません。

| シリアル番号 | キーのフィンガープリント                                      | 状態    | 有効期限  | ダウンロード（失効したキーのみ） |
|---------|------------------------------------------------------|-----------|--------------|------------------------------|
| 1       | `3018 3AC2 C4E2 3A40 9EFB  E705 9CE4 5ABC 8807 21D4` | `revoked` | `2021-06-08` | [失効したキー](https://gitlab.com/gitlab-org/gitlab-runner/-/blob/main/docs/install/gpg-keys/9CE45ABC880721D4.pub.gpg) |
| 2       | `09E5 7083 F34C CA94 D541  BC58 A674 BF81 35DF A027` | `revoked` | `2023-04-26` | [失効したキー](https://gitlab.com/gitlab-org/gitlab-runner/-/blob/main/docs/install/gpg-keys/A674BF8135DFA027.pub.gpg) |

## トラブルシューティング {#troubleshooting}

GitLab Runnerのインストール時に発生する問題のトラブルシューティングと解決のためのヒントを以下に示します。

### エラー: `No such file or directory`ジョブの失敗 {#error-no-such-file-or-directory-job-failures}

デフォルトのスケルトン（`skel`）ディレクトリが原因でGitLab Runnerで問題が発生し、ジョブの実行に失敗することがあります。[イシュー4449](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4449)と[イシュー1379](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/1379)を参照してください。

これを回避するために、GitLab Runnerをインストールすると、`gitlab-runner`ユーザーが作成され、デフォルトでは、ホームディレクトリはスケルトンなしで作成されます。`skel`の使用によってホームディレクトリに追加されるShell設定は、ジョブの実行を妨げる可能性があります。この設定は、前述のような予期しない問題を引き起こす可能性があります。

`skel`の回避がデフォルトの動作になる前にRunnerを作成していた場合は、次のドットファイルを削除してみてください。

```shell
sudo rm /home/gitlab-runner/.profile
sudo rm /home/gitlab-runner/.bashrc
sudo rm /home/gitlab-runner/.bash_logout
```

`skel`ディレクトリを使用して、新しく作成された`$HOME`ディレクトリにデータを入力する必要がある場合は、Runnerをインストールする前に、`GITLAB_RUNNER_DISABLE_SKEL`変数を明示的に`false`に設定する必要があります。

{{< tabs >}}

{{< tab title="Debian/Ubuntu/Mint" >}}

```shell
export GITLAB_RUNNER_DISABLE_SKEL=false; sudo -E apt-get install gitlab-runner
```

{{< /tab >}}

{{< tab title="RHEL/CentOS/Fedora/Amazon Linux" >}}

```shell
export GITLAB_RUNNER_DISABLE_SKEL=false; sudo -E yum install gitlab-runner
```

{{< /tab >}}

{{< /tabs >}}
